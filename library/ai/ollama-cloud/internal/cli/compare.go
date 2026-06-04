package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

type compareResult struct {
	Model     string `json:"model"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
	Content   string `json:"content,omitempty"`
	LatencyMs int    `json:"latency_ms"`
	PromptTok int    `json:"prompt_tokens"`
	OutputTok int    `json:"output_tokens"`
	TotalTok  int    `json:"total_tokens"`
	Status    int    `json:"http_status"`
	RateLimit bool   `json:"rate_limited,omitempty"`
}

func newCompareCmd(flags *rootFlags) *cobra.Command {
	var (
		promptFile string
		models     []string
		system     string
		maxTokens  int
		timeoutSec int
	)
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Run the same prompt against N models in parallel; side-by-side latency/tokens",
		Long: strings.TrimSpace(`
Posts the prompt to /api/chat against each --model in parallel and reports
latency, token counts, HTTP status, and any rate-limit/error per row. Useful
for calibrating advisor recommendations when two models score within 5%.

Output is always JSON-shaped {prompt_hash, prompt_bytes, results[], compared_at};
--json is implicit. prompt_hash matches the advisor log's hash so compare runs
correlate with advise-replay entries. Cost is not reported here (compare does not
load the cost overlay); use advise for per-model cost estimates.
`),
		Example: strings.Trim(`
  ollama-cloud-pp-cli compare --prompt-file ./p.txt --models qwen3-coder:480b,gpt-oss:120b
  ollama-cloud-pp-cli compare --prompt-file - --models gpt-oss:20b,gemma3:4b --max-tokens 200
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"command":"compare","note":"would post prompt to N models in parallel and emit side-by-side latency/tokens"}`)
				return nil
			}
			if len(models) < 2 {
				return usageErr(fmt.Errorf("compare: need at least 2 --models"))
			}
			prompt, err := readPromptInput(promptFile)
			if err != nil {
				return usageErr(fmt.Errorf("compare: reading prompt: %w", err))
			}
			if strings.TrimSpace(prompt) == "" {
				return usageErr(fmt.Errorf("compare: --prompt-file required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			messages := []map[string]any{}
			if system != "" {
				messages = append(messages, map[string]any{"role": "system", "content": system})
			}
			messages = append(messages, map[string]any{"role": "user", "content": prompt})

			parentCtx := cmd.Context()
			results := make([]compareResult, len(models))
			var wg sync.WaitGroup
			for i, m := range models {
				wg.Add(1)
				go func(idx int, model string) {
					defer wg.Done()
					results[idx] = runCompareOne(parentCtx, c, model, messages, maxTokens, time.Duration(timeoutSec)*time.Second)
				}(i, strings.TrimSpace(m))
			}
			wg.Wait()

			promptHash := sha256.Sum256([]byte(prompt))
			envelope := map[string]any{
				"prompt_hash":  hex.EncodeToString(promptHash[:8]),
				"prompt_bytes": len(prompt),
				"results":      results,
				"compared_at":  time.Now().UTC(),
			}
			out, _ := json.MarshalIndent(envelope, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&promptFile, "prompt-file", "", "Path to prompt text (or '-' for stdin)")
	cmd.Flags().StringSliceVar(&models, "models", nil, "Comma-separated model IDs to compare (need >=2)")
	cmd.Flags().StringVar(&system, "system", "", "Optional system message prepended to messages")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 256, "num_predict cap per model (low default keeps free-tier quota usage modest)")
	cmd.Flags().IntVar(&timeoutSec, "timeout", 60, "Per-model timeout in seconds")
	return cmd
}

func runCompareOne(parentCtx context.Context, c clientLike, model string, messages []map[string]any, maxTok int, timeout time.Duration) compareResult {
	r := compareResult{Model: model}
	body := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   false,
		"options":  map[string]any{"num_predict": maxTok},
	}
	// Root the per-model deadline at the command context so OS cancellation
	// (Ctrl+C) aborts in-flight requests instead of waiting out --timeout.
	ctx := parentCtx
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	start := time.Now()
	raw, status, err := c.Post(ctx, "/api/chat", body)
	r.LatencyMs = int(time.Since(start) / time.Millisecond)
	r.Status = status
	if err != nil {
		r.Error = err.Error()
		return r
	}
	if status == 429 {
		r.RateLimit = true
		r.Error = string(raw)
		return r
	}
	if status >= 400 {
		r.Error = fmt.Sprintf("HTTP %d: %s", status, truncate(string(raw), 200))
		return r
	}
	var resp struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		PromptEvalCount int `json:"prompt_eval_count"`
		EvalCount       int `json:"eval_count"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		r.Error = "parse: " + err.Error()
		return r
	}
	r.OK = true
	r.Content = resp.Message.Content
	r.PromptTok = resp.PromptEvalCount
	r.OutputTok = resp.EvalCount
	r.TotalTok = r.PromptTok + r.OutputTok
	return r
}
