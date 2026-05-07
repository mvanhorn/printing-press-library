// Hand-authored novel feature: LLM span cost rollup grouped by model and task.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type spanBucket struct {
	Key              string  `json:"key"`
	Model            string  `json:"model,omitempty"`
	Task             string  `json:"task,omitempty"`
	Runs             int     `json:"runs"`
	TotalCostCents   float64 `json:"total_cost_cents"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
}

func newRunsSpanCostCmd(flags *rootFlags) *cobra.Command {
	var period string
	var groupBy string
	var topN int

	cmd := &cobra.Command{
		Use:         "span-cost",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Rank LLM span cost grouped by model + task across recent runs",
		Long: `Walks recent runs, fetches per-run span data via /api/v1/runs/{id}/trace,
extracts LLM spans, and groups by (model, task). Surfaces top spenders so
finance/CTO questions answer in one shell call.

When the trace endpoint doesn't expose per-span model+token+cost, the row is
reported under model='unknown' using the per-run cost.`,
		Example: `  trigger-dev-pp-cli runs span-cost --since 7d --top 20
  trigger-dev-pp-cli runs span-cost --since 30d --by model --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would aggregate span costs over %s grouped by %s\n", period, groupBy)
				return nil
			}
			runResp, err := c.Get("/api/v1/runs", map[string]string{
				"page[size]":                "100",
				"filter[createdAt][period]": period,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			type runSummary struct {
				ID             string  `json:"id"`
				TaskIdentifier string  `json:"taskIdentifier"`
				CostInCents    float64 `json:"costInCents"`
			}
			var runs []runSummary
			for _, raw := range unwrapEnvelope(runResp) {
				var r runSummary
				if err := json.Unmarshal(raw, &r); err == nil {
					runs = append(runs, r)
				}
			}
			buckets := map[string]*spanBucket{}
			var totalCost float64
			for _, r := range runs {
				totalCost += r.CostInCents
				tracePath := replacePathParam("/api/v1/runs/{runId}/trace", "runId", r.ID)
				traceResp, err := c.Get(tracePath, nil)
				if err != nil {
					addSpanBucket(buckets, groupBy, r.TaskIdentifier, "unknown", 1, r.CostInCents, 0, 0)
					continue
				}
				addSpansToBuckets(buckets, groupBy, r.TaskIdentifier, traceResp, r.CostInCents)
			}
			var rows []*spanBucket
			for _, b := range buckets {
				rows = append(rows, b)
			}
			sort.Slice(rows, func(i, j int) bool {
				return rows[i].TotalCostCents > rows[j].TotalCostCents
			})
			if topN > 0 && topN < len(rows) {
				rows = rows[:topN]
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"period":           period,
					"group_by":         groupBy,
					"total_cost_cents": totalCost,
					"rows":             rows,
				}, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No runs in the last %s — nothing to roll up.\n", period)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "LLM span cost (last %s) — total $%.4f\n\n", period, totalCost/100)
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-25s %6s %12s\n", groupBy, "model", "runs", "cost")
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s %s\n",
				strings.Repeat("-", 30), strings.Repeat("-", 25), strings.Repeat("-", 6), strings.Repeat("-", 12))
			for _, r := range rows {
				key := r.Task
				if groupBy == "model" {
					key = r.Model
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-25s %6d $%10.4f\n",
					truncate(key, 30), truncate(r.Model, 25), r.Runs, r.TotalCostCents/100)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&period, "since", "7d", "Time window (1d, 7d, 30d)")
	cmd.Flags().StringVar(&groupBy, "by", "model,task", "Group key: model, task, or model,task")
	cmd.Flags().IntVar(&topN, "top", 20, "Show only the top N rows (0 for all)")
	return cmd
}

func addSpansToBuckets(buckets map[string]*spanBucket, groupBy, task string, trace json.RawMessage, fallbackCost float64) {
	var env struct {
		Spans []struct {
			Model            string  `json:"model"`
			GenAIModel       string  `json:"gen_ai.request.model"`
			Cost             float64 `json:"costInCents"`
			PromptTokens     int64   `json:"promptTokens"`
			CompletionTokens int64   `json:"completionTokens"`
		} `json:"spans"`
	}
	if err := json.Unmarshal(trace, &env); err != nil || len(env.Spans) == 0 {
		addSpanBucket(buckets, groupBy, task, "unknown", 1, fallbackCost, 0, 0)
		return
	}
	for _, s := range env.Spans {
		model := s.Model
		if model == "" {
			model = s.GenAIModel
		}
		if model == "" {
			continue
		}
		addSpanBucket(buckets, groupBy, task, model, 1, s.Cost, s.PromptTokens, s.CompletionTokens)
	}
}

func addSpanBucket(buckets map[string]*spanBucket, groupBy, task, model string, runDelta int, cost float64, prompt, completion int64) {
	var key string
	switch groupBy {
	case "model":
		key = model
	case "task":
		key = task
	default:
		key = task + "|" + model
	}
	b, ok := buckets[key]
	if !ok {
		b = &spanBucket{Key: key, Model: model, Task: task}
		buckets[key] = b
	}
	b.Runs += runDelta
	b.TotalCostCents += cost
	b.PromptTokens += prompt
	b.CompletionTokens += completion
}
