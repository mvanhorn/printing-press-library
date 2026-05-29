package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/ollama-cloud/internal/advisor"
	"github.com/spf13/cobra"
)

func newAdviseCmd(flags *rootFlags) *cobra.Command {
	var (
		promptFile        string
		sessionFile       string
		taskHint          string
		budget            float64
		maxLatencyMs      int
		requireTools      bool
		exclude           []string
		expectedOutputTok int
		modelsOverlay     string
		providerOverlays  []string
		explain           bool
		validateCatalog   bool
		enableTiebreak    bool
		tiebreakModel     string
		logPath           string
		format            string
	)

	cmd := &cobra.Command{
		Use:   "advise",
		Short: "Pick the right Ollama Cloud model for a prompt",
		Long: strings.TrimSpace(`
Heuristic-first model advisor. Extracts prompt features (token count, code-fence
density, language signals, reasoning depth, tool-use mentions, attachments)
without calling any LLM, scores every model in the live /api/tags catalog
against the curated metadata overlay, and emits a stable JSON envelope:

  { recommended, why, alternatives[], est_input_tokens, est_output_tokens,
    est_cost_usd, est_latency_ms, fallback, schema_version }

When the top-2 scores are within 5%, an optional cheap meta-LLM tiebreak
(--enable-tiebreak) calls --tiebreak-model and falls back deterministically
if that call fails or 429s.

Logs every invocation to ~/.local/state/ollama-cloud-pp-cli/advisor-log.jsonl
(override with OLLAMA_CLOUD_PP_CLI_ADVISOR_LOG or --log).
`),
		Example: strings.Trim(`
  ollama-cloud-pp-cli advise --validate-catalog --json
  ollama-cloud-pp-cli advise --prompt-file ./prompt.txt --task-hint coding --json
  ollama-cloud-pp-cli advise --prompt-file ./prompt.txt --explain --format md
  ollama-cloud-pp-cli advise --prompt-file ./prompt.txt --enable-tiebreak --tiebreak-model gpt-oss:20b
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"command":"advise","note":"would extract features, fetch /api/tags, score, log to JSONL"}`)
				return nil
			}
			ctx := cmd.Context()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Catalog must be fresh — verify-mode mock responses + agent-mode
			// caches can poison /api/tags shape ([] vs {"models":[...]}) and
			// silently mis-score every prompt thereafter.
			c.NoCache = true

			overlay := advisor.BundledModelsJSON()
			if modelsOverlay != "" {
				b, rerr := os.ReadFile(modelsOverlay)
				if rerr != nil {
					return configErr(fmt.Errorf("--models-overlay: %w", rerr))
				}
				overlay = b
			}

			if validateCatalog {
				tagsRaw, terr := c.Get(ctx, "/api/tags", nil)
				if terr != nil {
					captureDialDiag(terr.Error())
					return classifyAPIError(terr, flags)
				}
				drift, derr := advisor.ValidateCatalog(tagsRaw, overlay)
				if derr != nil {
					return apiErr(derr)
				}
				out, _ := json.MarshalIndent(drift, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}

			prompt, err := readPromptInput(promptFile)
			if err != nil {
				return usageErr(err)
			}
			if strings.TrimSpace(prompt) == "" {
				return usageErr(fmt.Errorf("advise: --prompt-file required (or pipe via stdin); pass --validate-catalog to skip"))
			}

			var sess *advisor.Session
			if sessionFile != "" {
				b, serr := os.ReadFile(sessionFile)
				if serr != nil {
					return usageErr(fmt.Errorf("--session-file: %w", serr))
				}
				sess = &advisor.Session{}
				if jerr := json.Unmarshal(b, sess); jerr != nil {
					return usageErr(fmt.Errorf("--session-file: parse: %w", jerr))
				}
			}

			tagsRaw, err := c.Get(ctx, "/api/tags", nil)
			if err != nil {
				captureDialDiag(err.Error())
				return classifyAPIError(err, flags)
			}
			catalog, err := advisor.LoadCatalog(tagsRaw, overlay)
			if err != nil {
				return apiErr(err)
			}
			// Merge sibling provider overlays (e.g. local-llama, openrouter)
			// so advise can recommend across providers. Each overlay must have
			// schema_version=1 + provider="<name>" + a models[] array.
			for _, povPath := range providerOverlays {
				povPath = strings.TrimSpace(povPath)
				if povPath == "" {
					continue
				}
				b, perr := os.ReadFile(povPath)
				if perr != nil {
					return configErr(fmt.Errorf("--provider-overlay %s: %w", povPath, perr))
				}
				sibling, perr := advisor.LoadProviderOverlay(b)
				if perr != nil {
					return configErr(fmt.Errorf("--provider-overlay %s: %w", povPath, perr))
				}
				catalog = append(catalog, sibling...)
			}

			req := advisor.Request{
				Prompt:               prompt,
				Session:              sess,
				TaskHint:             taskHint,
				BudgetRemainingUSD:   budget,
				MaxLatencyMs:         maxLatencyMs,
				RequireTools:         requireTools,
				Exclude:              exclude,
				ExpectedOutputTokens: expectedOutputTok,
				EnableTiebreak:       enableTiebreak,
			}
			if enableTiebreak && tiebreakModel != "" {
				req.Tiebreaker = makeTiebreaker(c, tiebreakModel, 8*time.Second)
			}

			rec, err := advisor.Advise(ctx, req, catalog, explain || format == "md")
			if err != nil {
				return apiErr(err)
			}

			if err := writeAdviseLog(logPath, prompt, taskHint, rec); err != nil {
				fmt.Fprintf(os.Stderr, "warning: advisor log write failed: %v\n", err)
			}

			switch format {
			case "md":
				return renderAdviseMD(cmd.OutOrStdout(), rec)
			default:
				out, _ := json.MarshalIndent(rec, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
		},
	}

	cmd.Flags().StringVar(&promptFile, "prompt-file", "", "Path to prompt text (or '-' for stdin)")
	cmd.Flags().StringVar(&sessionFile, "session-file", "", "Path to JSON session context (messages[], budget_remaining_usd, prior_model_choices[])")
	cmd.Flags().StringVar(&taskHint, "task-hint", "", "coding|reasoning|long-context|cheap|multilingual|vision")
	cmd.Flags().Float64Var(&budget, "budget-remaining-usd", 0, "Filter out models whose estimated cost exceeds this budget")
	cmd.Flags().IntVar(&maxLatencyMs, "max-latency-ms", 0, "Filter out models whose p50 latency exceeds this ms target")
	cmd.Flags().BoolVar(&requireTools, "require-tools", false, "Only consider models that support tool/function calling")
	cmd.Flags().StringSliceVar(&exclude, "exclude", nil, "Comma-separated model IDs to exclude")
	cmd.Flags().IntVar(&expectedOutputTok, "expected-output-tokens", 1024, "Output-token planning hint (for cost+latency estimation)")
	cmd.Flags().StringVar(&modelsOverlay, "models-overlay", "", "Override the bundled models.json with a custom overlay file")
	cmd.Flags().StringSliceVar(&providerOverlays, "provider-overlay", nil, "Comma-separated sibling-provider overlay files (e.g. workspace/scripts/local-llama-models.json). Each adds its models to the candidate set; provider-qualified IDs (model@provider) appear in the envelope when non-ollama-cloud wins.")
	cmd.Flags().BoolVar(&explain, "explain", false, "Emit full scoring trace + features + filtered candidates")
	cmd.Flags().BoolVar(&validateCatalog, "validate-catalog", false, "Diff /api/tags against curated models.json overlay and exit")
	cmd.Flags().BoolVar(&enableTiebreak, "enable-tiebreak", false, "When top-2 scores within 5%, call a cheap LLM to break the tie")
	cmd.Flags().StringVar(&tiebreakModel, "tiebreak-model", "gpt-oss:20b", "Model for tiebreak calls (must be cheap + fast)")
	cmd.Flags().StringVar(&logPath, "log", "", "Override the advisor log path (default ~/.local/state/ollama-cloud-pp-cli/advisor-log.jsonl)")
	cmd.Flags().StringVar(&format, "format", "json", "Output format: json|md")
	return cmd
}

func readPromptInput(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if path == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func writeAdviseLog(override, prompt, taskHint string, rec *advisor.Recommendation) error {
	path := override
	if path == "" {
		path = advisor.DefaultLogPath()
	}
	if path == "" {
		return nil
	}
	hash := sha256.Sum256([]byte(prompt))
	entry := advisor.LogEntry{
		AdvisedAt:    rec.AdvisedAt,
		PromptHash:   hex.EncodeToString(hash[:8]),
		PromptBytes:  len(prompt),
		TaskHint:     taskHint,
		Recommended:  rec.Recommended,
		EstCostUSD:   rec.EstCostUSD,
		EstLatencyMs: rec.EstLatencyMs,
		TiebreakUsed: rec.TiebreakUsed,
	}
	if rec.Features != nil {
		entry.Features = *rec.Features
	}
	// Trim alternatives to top-2 for compactness (atomic-append safety).
	if len(rec.Alternatives) > 2 {
		entry.Alternatives = rec.Alternatives[:2]
	} else {
		entry.Alternatives = rec.Alternatives
	}
	return advisor.AppendLog(path, entry)
}

func renderAdviseMD(w io.Writer, rec *advisor.Recommendation) error {
	fmt.Fprintf(w, "# Advisor recommendation\n\n")
	fmt.Fprintf(w, "**Recommended:** `%s`\n\n", rec.Recommended)
	fmt.Fprintf(w, "**Why:** %s\n\n", rec.Why)
	fmt.Fprintf(w, "**Est cost:** $%.6f &nbsp; **Est latency:** %dms &nbsp; **Tokens:** %d in / %d out\n\n",
		rec.EstCostUSD, rec.EstLatencyMs, rec.EstInputTokens, rec.EstOutputTokens)
	if rec.Fallback != "" {
		fmt.Fprintf(w, "**Fallback:** `%s`\n\n", rec.Fallback)
	}
	if len(rec.Alternatives) > 0 {
		fmt.Fprintf(w, "## Alternatives\n\n")
		for _, c := range rec.Alternatives {
			fmt.Fprintf(w, "- `%s` (score=%.3f) — %s\n", c.ModelID, c.Score, c.Why)
		}
		fmt.Fprintln(w)
	}
	if rec.Features != nil {
		fb, _ := json.MarshalIndent(rec.Features, "", "  ")
		fmt.Fprintf(w, "## Features\n\n```json\n%s\n```\n\n", string(fb))
	}
	if len(rec.Filtered) > 0 {
		fmt.Fprintf(w, "## Filtered (hard constraints)\n\n")
		for _, c := range rec.Filtered {
			fmt.Fprintf(w, "- `%s` — %s\n", c.ModelID, c.FilterReason)
		}
	}
	if rec.TiebreakAttempted {
		fmt.Fprintf(w, "\n_Tiebreak attempted (used=%v, error=%q)_\n", rec.TiebreakUsed, rec.TiebreakError)
	}
	return nil
}

// makeTiebreaker returns a TiebreakFunc that posts a single-shot decision
// prompt to /api/chat against `model`, expects a JSON reply with the picked
// model ID, and returns the picked ID. 429/timeout/parse-error → returns
// error so Advise() deterministically falls back to the heuristic pick.
func makeTiebreaker(c clientLike, model string, timeout time.Duration) advisor.TiebreakFunc {
	return func(ctx context.Context, prompt string, top []advisor.Candidate) (string, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		body := map[string]any{
			"model": model,
			"messages": []map[string]any{
				{"role": "system", "content": "You break ties between two LLM candidates. Respond with ONLY the chosen model ID. No prose."},
				{"role": "user", "content": fmt.Sprintf("Prompt:\n%s\n\nCandidates:\n1. %s — %s\n2. %s — %s\n\nReply with the chosen model ID only.",
					truncate(prompt, 1500), top[0].ModelID, top[0].Why, top[1].ModelID, top[1].Why)},
			},
			"stream":  false,
			"options": map[string]any{"num_predict": 32, "temperature": 0.1},
		}
		raw, status, err := c.Post(ctx, "/api/chat", body)
		if err != nil {
			return "", err
		}
		if status == 429 {
			return "", fmt.Errorf("tiebreak: 429 rate limit")
		}
		if status >= 400 {
			return "", fmt.Errorf("tiebreak: HTTP %d: %s", status, string(raw))
		}
		var resp struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return "", err
		}
		guess := strings.TrimSpace(resp.Message.Content)
		// Accept any case-insensitive substring match against one of the top IDs.
		for _, c := range top {
			if strings.EqualFold(guess, c.ModelID) || strings.Contains(guess, c.ModelID) {
				return c.ModelID, nil
			}
		}
		return "", fmt.Errorf("tiebreak: %q matched no candidate", guess)
	}
}

// clientLike is the subset of *client.Client used by the tiebreaker, kept tiny
// so it stays testable.
type clientLike interface {
	Post(ctx context.Context, path string, body any) (json.RawMessage, int, error)
}
