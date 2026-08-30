// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/groq/internal/store"
)

type compareModelResult struct {
	Model            string  `json:"model"`
	LatencyMs        int64   `json:"latency_ms"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	TokensPerSec     float64 `json:"tokens_per_sec"`
	CostUSD          float64 `json:"cost_usd"`
	Output           string  `json:"output,omitempty"`
	Error            string  `json:"error,omitempty"`
}

type compareReport struct {
	Prompt        string               `json:"prompt"`
	Models        []compareModelResult `json:"models"`
	FetchFailures int                  `json:"fetch_failures"`
	Note          string               `json:"note,omitempty"`
}

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var flagModels string

	cmd := &cobra.Command{
		Use:         "compare <prompt>",
		Short:       "Run one prompt across several models and rank them by latency, tokens/sec, usage, and cost.",
		Long:        "Run one prompt through several Groq models in parallel and rank the results by latency, tokens/sec, usage, and estimated cost. Pricing comes from the synced models catalog when available, else a built-in price map.",
		Example:     "  groq-pp-cli compare \"Explain transformers in one line\" --models openai/gpt-oss-120b,openai/gpt-oss-20b --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "prompt=Explain fast inference in one sentence;--models=openai/gpt-oss-20b"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "compare")
			}
			if flagModels == "" {
				return usageErr(fmt.Errorf("--models is required (comma-separated model IDs)"))
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("a prompt argument is required"))
			}
			prompt := strings.Join(args, " ")
			modelIDs := splitTrim(flagModels)
			if len(modelIDs) == 0 {
				return usageErr(fmt.Errorf("--models must name at least one model (comma-separated model IDs)"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			prices := map[string]modelPrice{}
			if db, err := store.OpenReadOnlyContext(ctx, defaultDBPath("groq-pp-cli")); err == nil {
				prices, _ = loadModelPrices(ctx, db)
				db.Close()
			}

			type fetchResult struct {
				idx   int
				id    string
				entry compareModelResult
				err   error
			}
			results := make(chan fetchResult, len(modelIDs))
			var wg sync.WaitGroup
			// Serialize local-ledger writes so parallel completions cannot
			// contend on SQLite's single writer (SQLITE_BUSY).
			var storeMu sync.Mutex
			for i, id := range modelIDs {
				wg.Add(1)
				go func(idx int, model string) {
					defer wg.Done()
					start := time.Now()
					body := map[string]any{
						"model":      model,
						"messages":   []map[string]string{{"role": "user", "content": prompt}},
						"max_tokens": 512,
					}
					data, _, callErr := c.PostWithParams(ctx, "/openai/v1/chat/completions", nil, body)
					entry := compareModelResult{Model: model}
					if callErr != nil {
						entry.Error = callErr.Error()
						results <- fetchResult{idx: idx, id: model, entry: entry, err: callErr}
						return
					}
					var obj struct {
						Model string `json:"model"`
						Usage struct {
							PromptTokens     int64 `json:"prompt_tokens"`
							CompletionTokens int64 `json:"completion_tokens"`
							TotalTokens      int64 `json:"total_tokens"`
						} `json:"usage"`
						Choices []struct {
							Message struct {
								Content string `json:"content"`
							} `json:"message"`
						} `json:"choices"`
					}
					if err := json.Unmarshal(data, &obj); err != nil {
						entry.Error = "parsing response: " + err.Error()
						results <- fetchResult{idx: idx, id: model, entry: entry, err: err}
						return
					}
					elapsed := time.Since(start)
					mp := modelPrice{}.orBuiltin(obj.Model)
					if p, ok := prices[obj.Model]; ok {
						mp = p
					}
					entry.LatencyMs = elapsed.Milliseconds()
					entry.PromptTokens = obj.Usage.PromptTokens
					entry.CompletionTokens = obj.Usage.CompletionTokens
					entry.TotalTokens = obj.Usage.TotalTokens
					if elapsed.Seconds() > 0 {
						entry.TokensPerSec = float64(entry.TotalTokens) / elapsed.Seconds()
					}
					entry.CostUSD = float64(obj.Usage.PromptTokens)*mp.Prompt + float64(obj.Usage.CompletionTokens)*mp.Completion
					if len(obj.Choices) > 0 {
						entry.Output = obj.Choices[0].Message.Content
					}
					// Persist the completion so it shows up in 'costs'.
					if raw, err := json.Marshal(obj); err == nil {
						storeMu.Lock()
						writeMutationResponseToStore(ctx, "chat", raw, "")
						storeMu.Unlock()
					}
					results <- fetchResult{idx: idx, id: model, entry: entry}
				}(i, id)
			}
			go func() {
				wg.Wait()
				close(results)
			}()

			ordered := make([]compareModelResult, len(modelIDs))
			fetchErrors := make([]error, len(modelIDs))
			for r := range results {
				ordered[r.idx] = r.entry
				if r.err != nil {
					fetchErrors[r.idx] = r.err
				}
			}

			survivors := make([]compareModelResult, 0, len(ordered))
			var failures int
			for idx, e := range ordered {
				if fetchErrors[idx] != nil {
					failures++
				}
				survivors = append(survivors, e)
			}
			sort.SliceStable(survivors, func(i, j int) bool {
				if survivors[i].Error != "" && survivors[j].Error == "" {
					return false
				}
				if survivors[i].Error == "" && survivors[j].Error != "" {
					return true
				}
				return survivors[i].TokensPerSec > survivors[j].TokensPerSec
			})

			report := &compareReport{Prompt: prompt, Models: survivors, FetchFailures: failures}
			if failures > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d model runs failed\n", failures, len(modelIDs))
				report.Note = fmt.Sprintf("%d of %d model runs failed; failed entries are listed with their error", failures, len(modelIDs))
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			rows := make([]map[string]any, 0, len(survivors))
			for _, e := range survivors {
				if e.Error != "" {
					rows = append(rows, map[string]any{"model": e.Model, "status": "error", "error": e.Error})
					continue
				}
				rows = append(rows, map[string]any{
					"model": e.Model, "latency_ms": e.LatencyMs, "tokens": e.TotalTokens,
					"tokens_per_sec": round2(e.TokensPerSec), "cost_usd": fmt.Sprintf("$%.6f", e.CostUSD),
				})
			}
			if err := printAutoTable(cmd.OutOrStdout(), rows); err != nil {
				return err
			}
			if failures > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%d model run(s) failed; use --json to inspect errors\n", failures)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagModels, "models", "", "Comma-separated model IDs to run the prompt against (required)")
	return cmd
}

func splitTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func round2(f float64) float64 {
	return float64(int64(f*100+0.5)) / 100
}
