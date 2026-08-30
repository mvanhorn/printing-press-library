// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/groq/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/groq/internal/store"
)

// builtinModelPrices is a fallback per-token price map (USD) for well-known
// Groq models, used when the models catalog has not been synced locally.
// Synced pricing (models table, data.pricing.prompt/completion) always wins.
var builtinModelPrices = map[string][2]float64{
	"openai/gpt-oss-120b":                 {0.00000015, 0.00000060},
	"openai/gpt-oss-20b":                  {0.000000075, 0.00000030},
	"openai/gpt-oss-safeguard-20b":        {0.000000075, 0.00000030},
	"qwen/qwen3.6-27b":                    {0.00000060, 0.00000300},
	"qwen/qwen3.8-27b":                    {0.00000080, 0.00000400},
	"meta-llama/llama-prompt-guard-2-22m": {0.00000003, 0.00000003},
	"meta-llama/llama-prompt-guard-2-86m": {0.00000004, 0.00000004},
}

type costRow struct {
	Model            string  `json:"model"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	CostUSD          float64 `json:"cost_usd"`
	Runs             int     `json:"runs"`
}

type costsReport struct {
	GroupBy         string           `json:"group_by"`
	Rows            []costRow        `json:"rows"`
	TotalTokens     int64            `json:"total_tokens"`
	TotalCostUSD    float64          `json:"total_cost_usd"`
	TotalRuns       int              `json:"total_runs"`
	UnknownPricing  map[string]int64 `json:"unknown_pricing_models"`
	ExcludedNoUsage int              `json:"excluded_no_usage_rows"`
	Note            string           `json:"note,omitempty"`
}

func newNovelCostsCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagGroupBy string

	cmd := &cobra.Command{
		Use:         "costs",
		Short:       "Aggregate token and dollar spend from your local completion history, grouped by model or day.",
		Long:        "Aggregate token usage and estimated dollar spend from locally stored chat completions, priced with the synced models catalog (or a built-in price map). History accumulates as you run 'chat completions' or 'compare'; run 'sync --resources models' to keep catalog pricing fresh.",
		Example:     "  groq-pp-cli costs --since 48h --group-by model --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "costs")
			}
			groupBy := flagGroupBy
			if groupBy == "" {
				groupBy = "model"
			}
			if groupBy != "model" && groupBy != "day" {
				return usageErr(fmt.Errorf("--group-by must be \"model\" or \"day\", got %q", groupBy))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			var since time.Time
			if flagSince != "" {
				d, err := cliutil.ParseDurationLoose(flagSince)
				if err != nil {
					return usageErr(fmt.Errorf("parsing --since %q: %w", flagSince, err))
				}
				since = time.Now().Add(-d)
			}

			dbPath := defaultDBPath("groq-pp-cli")
			if _, statErr := os.Stat(dbPath); statErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local history at %s\nrun chat completions or compare to accumulate usage, then retry\n", dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]costRow, 0), flags)
				}
				return nil
			}
			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local history: %w", err)
			}
			defer db.Close()

			prices, err := loadModelPrices(ctx, db)
			if err != nil {
				return fmt.Errorf("loading model pricing: %w", err)
			}
			if len(prices) == 0 {
				_ = hintIfUnsynced(cmd, db, "models")
			}

			report, err := computeCosts(ctx, db, since, groupBy, prices)
			if err != nil {
				return err
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			if len(report.Rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No completion history found in the local ledger.")
				return nil
			}
			rows := make([]map[string]any, 0, len(report.Rows))
			for _, r := range report.Rows {
				rows = append(rows, map[string]any{
					"model": r.Model, "prompt_tokens": r.PromptTokens, "completion_tokens": r.CompletionTokens,
					"total_tokens": r.TotalTokens, "cost_usd": fmt.Sprintf("$%.6f", r.CostUSD), "runs": r.Runs,
				})
			}
			if err := printAutoTable(cmd.OutOrStdout(), rows); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\ntotal: %d tokens, $%.6f across %d runs\n", report.TotalTokens, report.TotalCostUSD, report.TotalRuns)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Only include completions newer than this duration (e.g. 24h, 7d, 1w)")
	cmd.Flags().StringVar(&flagGroupBy, "group-by", "model", "Group rows by \"model\" or \"day\"")
	return cmd
}

type modelPrice struct {
	Prompt     float64
	Completion float64
}

// loadModelPrices reads per-model pricing from the synced models catalog.
func loadModelPrices(ctx context.Context, db *store.Store) (map[string]modelPrice, error) {
	rows, err := db.DB().QueryContext(ctx, `SELECT id, data FROM models`)
	if err != nil {
		return nil, fmt.Errorf("querying models catalog: %w", err)
	}
	defer rows.Close()
	prices := make(map[string]modelPrice)
	for rows.Next() {
		var id string
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			continue
		}
		var obj struct {
			Pricing struct {
				Prompt     json.RawMessage `json:"prompt"`
				Completion json.RawMessage `json:"completion"`
			} `json:"pricing"`
		}
		if err := json.Unmarshal(data, &obj); err != nil {
			continue
		}
		p, ok := parsePrice(obj.Pricing.Prompt)
		c, ok2 := parsePrice(obj.Pricing.Completion)
		if !ok || !ok2 {
			continue
		}
		prices[id] = modelPrice{Prompt: p, Completion: c}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating models catalog: %w", err)
	}
	return prices, nil
}

func parsePrice(raw json.RawMessage) (float64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.TrimSpace(s)
		if s == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(s, 64)
		return f, err == nil
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f, true
	}
	return 0, false
}

func (m modelPrice) orBuiltin(model string) modelPrice {
	if m.Prompt > 0 || m.Completion > 0 {
		return m
	}
	if b, ok := builtinModelPrices[model]; ok {
		return modelPrice{Prompt: b[0], Completion: b[1]}
	}
	return m
}

func computeCosts(ctx context.Context, db *store.Store, since time.Time, groupBy string, prices map[string]modelPrice) (*costsReport, error) {
	rows, err := db.DB().QueryContext(ctx, `SELECT id, data, COALESCE(model,'') AS model, COALESCE(created,0) AS created FROM chat`)
	if err != nil {
		return nil, fmt.Errorf("querying chat history: %w", err)
	}
	type chatRow struct {
		id      string
		data    []byte
		model   string
		created int64
	}
	var raw []chatRow
	for rows.Next() {
		var r chatRow
		if err := rows.Scan(&r.id, &r.data, &r.model, &r.created); err != nil {
			continue
		}
		raw = append(raw, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating chat history: %w", err)
	}
	_ = rows.Close()

	report := &costsReport{
		GroupBy:        groupBy,
		Rows:           make([]costRow, 0),
		UnknownPricing: make(map[string]int64),
	}
	grouped := map[string]*costRow{}
	var excluded int

	for _, r := range raw {
		if since.After(time.Unix(0, 0)) && r.created > 0 && time.Unix(r.created, 0).Before(since) {
			continue
		}
		var obj struct {
			Model string `json:"model"`
			Usage struct {
				PromptTokens     int64 `json:"prompt_tokens"`
				CompletionTokens int64 `json:"completion_tokens"`
				TotalTokens      int64 `json:"total_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(r.data, &obj); err != nil {
			continue
		}
		if obj.Usage.TotalTokens == 0 && obj.Usage.PromptTokens == 0 {
			excluded++
			continue
		}
		model := obj.Model
		if model == "" {
			model = r.model
		}
		mp := modelPrice{}.orBuiltin(model)
		if p, ok := prices[model]; ok {
			mp = p
		} else if mp.Prompt == 0 && mp.Completion == 0 {
			report.UnknownPricing[model]++
		}
		cost := float64(obj.Usage.PromptTokens)*mp.Prompt + float64(obj.Usage.CompletionTokens)*mp.Completion

		key := model
		if groupBy == "day" {
			day := time.Unix(r.created, 0).UTC().Format("2006-01-02")
			if r.created == 0 {
				day = time.Now().UTC().Format("2006-01-02")
			}
			key = day
		}
		row := grouped[key]
		if row == nil {
			row = &costRow{Model: key}
			grouped[key] = row
		}
		row.PromptTokens += obj.Usage.PromptTokens
		row.CompletionTokens += obj.Usage.CompletionTokens
		row.TotalTokens += obj.Usage.TotalTokens
		row.CostUSD += cost
		row.Runs++
	}

	keys := make([]string, 0, len(grouped))
	for k := range grouped {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		report.Rows = append(report.Rows, *grouped[k])
	}
	for _, r := range report.Rows {
		report.TotalTokens += r.TotalTokens
		report.TotalCostUSD += r.CostUSD
		report.TotalRuns += r.Runs
	}
	report.ExcludedNoUsage = excluded
	if len(report.UnknownPricing) > 0 {
		report.Note = "some models had no known pricing; their cost is counted as $0. Run 'groq-pp-cli sync --resources models' to populate catalog prices."
	}
	return report, nil
}
