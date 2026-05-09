package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/huggingface/internal/hfx"
)

type backendCheckResponse struct {
	hfx.Envelope
	ID            string                `json:"id"`
	Architectures []string              `json:"architectures,omitempty"`
	Features      []string              `json:"detected_features"`
	Backends      []backendVerdictGroup `json:"backends"`
	Summary       string                `json:"summary"`
	MatrixSource  string                `json:"matrix_source"`
	Explain       string                `json:"explain,omitempty"`
}

type backendVerdictGroup struct {
	Backend  string             `json:"backend"`
	Verdicts []hfx.BackendEntry `json:"verdicts"`
	Verdict  string             `json:"verdict"` // overall: yes|partial|no|unknown
}

func newHFBackendCheckCmd(flags *rootFlags) *cobra.Command {
	var backends string
	cmd := &cobra.Command{
		Use:   "backend-check <id>",
		Short: "Verdict whether a model's architecture loads on target backends.",
		Long: `backend-check reads the model's config.json + card and the bundled
backend-support matrix to return a per-backend, per-feature verdict
("yes/no/partial/unknown") with citations and source_checked dates.

Catches the "downloaded 30GB then can't load" failure class.`,
		Example: `  huggingface-pp-cli backend-check Qwen/Qwen3-MoE-A14B --backend llama.cpp,mlx
  huggingface-pp-cli backend-check deepseek-ai/DeepSeek-V2 --backend llama.cpp/turboquant --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			ctx := cmd.Context()
			model, status, err := hfFetchModel(ctx, id, hfTokenForRequests())
			if err != nil {
				if status == 429 {
					return hfRateLimited("rate limited (HTTP 429); set HF_TOKEN for higher quota")
				}
				if status == 404 {
					return hfNotFound("model %q not found", id)
				}
				return err
			}

			// Parse config.json (nested in card response)
			cfg, _ := decodeConfigFromMap(model.Config)

			// Fetch raw README for low-confidence card-text matches if config is sparse.
			var cardText string
			if cfg.NumExperts == 0 && cfg.NumNextnPredictLayers == 0 && cfg.MultiTokenPredictionLayers == 0 {
				if raw, _, rerr := hfFetchRaw(ctx, id, "main", "README.md", hfTokenForRequests()); rerr == nil {
					cardText = string(raw)
					if len(cardText) > 16000 {
						cardText = cardText[:16000]
					}
				}
			}

			// Detect features that matter for backend verdicts
			featuresToCheck := []string{"moe", "mtp", "mla", "gqa", "sliding-window-attn", "rope-yarn"}
			detected := []string{}
			for _, f := range featuresToCheck {
				if c := hfx.ClassifyFeature(f, cfg, cardText); c.Detected {
					detected = append(detected, f)
				}
			}

			// Resolve target backends
			stateDir, _ := hfx.StateDir(flags.stateDir)
			matrix, src, err := hfx.LoadBackendMatrix(stateDir, flags.backendSupport)
			if err != nil {
				return fmt.Errorf("loading backend matrix: %w", err)
			}

			targets := []string{}
			if backends == "" {
				targets = []string{"llama.cpp", "llama.cpp/turboquant", "mlx"}
			} else {
				for _, b := range strings.Split(backends, ",") {
					if t := strings.TrimSpace(b); t != "" {
						targets = append(targets, t)
					}
				}
			}

			resp := backendCheckResponse{
				Envelope:     hfx.NewEnvelope("backend-check"),
				ID:           model.ID,
				Features:     detected,
				MatrixSource: src,
			}
			if archs, ok := model.Config["architectures"].([]any); ok {
				for _, a := range archs {
					if s, ok := a.(string); ok {
						resp.Architectures = append(resp.Architectures, s)
					}
				}
			}

			summaries := []string{}
			anyBlocking := false
			for _, backend := range targets {
				grp := backendVerdictGroup{Backend: backend}
				worst := "yes"
				for _, f := range detected {
					entry, found := hfx.LookupVerdict(matrix, f, backend)
					grp.Verdicts = append(grp.Verdicts, entry)
					if !found {
						if worst != "no" && worst != "partial" {
							worst = "unknown"
						}
						continue
					}
					switch strings.ToLower(entry.Supported) {
					case "no":
						worst = "no"
					case "training-only":
						if worst != "no" {
							worst = "partial"
						}
					case "partial":
						if worst != "no" {
							worst = "partial"
						}
					case "unknown":
						if worst == "yes" {
							worst = "unknown"
						}
					}
				}
				if len(detected) == 0 {
					worst = "no-features-of-interest"
				}
				grp.Verdict = worst
				resp.Backends = append(resp.Backends, grp)
				summaries = append(summaries, backend+":"+worst)
				if worst == "no" {
					anyBlocking = true
				}
			}
			resp.Summary = strings.Join(summaries, " | ")
			if flags.explain {
				resp.Explain = explainBackendCheck(resp)
			}

			// Output
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				err := printJSONFiltered(cmd.OutOrStdout(), resp, flags)
				if err != nil {
					return err
				}
				if anyBlocking {
					return hfBackendUnsupported("at least one target backend reports 'no'")
				}
				return nil
			}

			// Human path
			fmt.Fprintf(cmd.OutOrStdout(), "%s — backend-check (matrix: %s)\n", resp.ID, resp.MatrixSource)
			fmt.Fprintf(cmd.OutOrStdout(), "  arch: %s   detected features: %s\n",
				strings.Join(resp.Architectures, ","), strings.Join(resp.Features, ","))
			for _, g := range resp.Backends {
				fmt.Fprintf(cmd.OutOrStdout(), "\n  Backend: %s — %s\n", g.Backend, g.Verdict)
				for _, v := range g.Verdicts {
					fmt.Fprintf(cmd.OutOrStdout(), "    %-22s %-10s since=%s checked=%s\n",
						v.Feature, v.Supported, v.Since, v.SourceChecked)
					if v.Notes != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "      note: %s\n", v.Notes)
					}
					if v.WikiPointer != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "      wiki: %s\n", v.WikiPointer)
					}
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nSummary: %s\n", resp.Summary)
			if resp.Explain != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", resp.Explain)
			}
			if anyBlocking {
				return hfBackendUnsupported("at least one target backend reports 'no'")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&backends, "backend", "", "Comma-separated backends to check (default: llama.cpp,llama.cpp/turboquant,mlx)")
	return cmd
}

func explainBackendCheck(r backendCheckResponse) string {
	var b strings.Builder
	b.WriteString("explain: ")
	if len(r.Features) == 0 {
		b.WriteString("No backend-relevant features detected; default-load expected. Verify with a tiny inference probe before relying.")
		return b.String()
	}
	b.WriteString(fmt.Sprintf("detected %s. ", strings.Join(r.Features, "+")))
	for _, g := range r.Backends {
		switch g.Verdict {
		case "no":
			b.WriteString(fmt.Sprintf("%s: BLOCKED (exit 3 verdict). ", g.Backend))
		case "partial":
			b.WriteString(fmt.Sprintf("%s: partial — load works but verify performance with bench-history. ", g.Backend))
		case "unknown":
			b.WriteString(fmt.Sprintf("%s: matrix has no entry — refresh `backend-support.json` or assume risk. ", g.Backend))
		case "yes":
			b.WriteString(fmt.Sprintf("%s: ok. ", g.Backend))
		}
	}
	return strings.TrimSpace(b.String())
}
