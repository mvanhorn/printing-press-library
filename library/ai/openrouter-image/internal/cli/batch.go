// Copyright 2026 neal-kyle and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: budget-gated batch run. Reads a CSV of generations (prompt,
// model, optional params), estimates every row from local pricing before the
// first API call, gates execution on a hard USD budget, then runs each
// generation and appends real cost to the local ledger.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/openrouter-image/internal/store"
)

// pp:data-source auto

type batchRow struct {
	Prompt     string `json:"prompt"`
	Model      string `json:"model"`
	N          int    `json:"n,omitempty"`
	Resolution string `json:"resolution,omitempty"`
	Quality    string `json:"quality,omitempty"`
	Output     string `json:"output,omitempty"`
}

type batchItemResult struct {
	Row      int     `json:"row"`
	Model    string  `json:"model"`
	Prompt   string  `json:"prompt"`
	Estimate float64 `json:"estimate_usd"`
	Cost     float64 `json:"cost_usd,omitempty"`
	Status   string  `json:"status"` // estimated | generated | error
	Error    string  `json:"error,omitempty"`
	SavedTo  string  `json:"saved_to,omitempty"`
	LedgerID string  `json:"ledger_id,omitempty"`
}

type batchView struct {
	Action        string            `json:"action"`
	TotalEstimate float64           `json:"total_estimate_usd"`
	Budget        float64           `json:"budget_usd"`
	UnderBudget   bool              `json:"under_budget"`
	TotalCost     float64           `json:"total_cost_usd,omitempty"`
	Rows          []batchItemResult `json:"rows"`
	Note          string            `json:"note,omitempty"`
}

func newNovelBatchCmd(flags *rootFlags) *cobra.Command {
	var (
		flagSpec   string
		flagBudget string
		dbPath     string
	)

	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Run many generations from a CSV with a hard USD budget: estimate first, abort before any spend if over",
		Long: `Run a batch of image generations from a CSV with a hard USD budget gate.

CSV format (header row required):
  prompt,model,n,resolution,quality,output
  "a red panda astronaut",openai/gpt-image-1,1,2K,high,panda.png

Flow: every row is estimated from local per-endpoint pricing BEFORE the first
API call. If the total estimate exceeds --budget, the batch aborts with no
spend. Otherwise each row is generated, saved, and recorded in the local
ledger.

Use --dry-run to preview estimates and the budget decision without spending.

Use this command to plan and run a budgeted batch of generations from a CSV.
Do NOT use it for a single ad-hoc image; use 'generate' instead.
Do NOT use it to estimate one model's cost interactively; use 'cost-estimate' instead.`,
		Example: strings.Trim(`
  openrouter-image-pp-cli batch --spec batch.csv --budget 2.00 --dry-run
  openrouter-image-pp-cli batch --spec batch.csv --budget 5.00 --json --agent
`, "\n"),
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would batch-generate from %s with budget %s\n", flagSpec, flagBudget)
				return nil
			}
			if flagSpec == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--spec is required (path to a CSV file)"))
			}
			budget, err := strconv.ParseFloat(orDefault(flagBudget, "0"), 64)
			if err != nil || budget < 0 {
				return usageErr(fmt.Errorf("--budget must be a non-negative USD amount, got %q", flagBudget))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			rows, err := parseBatchCSV(flagSpec)
			if err != nil {
				return usageErr(fmt.Errorf("parsing --spec %s: %w", flagSpec, err))
			}
			if len(rows) == 0 {
				return usageErr(fmt.Errorf("no rows in %s (header required: prompt,model,n,resolution,quality,output)", flagSpec))
			}

			// Load pricing from the local store for estimation.
			if dbPath == "" {
				dbPath = defaultDBPath("openrouter-image-pp-cli")
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			_ = db.EnsureOpenRouterImageTables(ctx)

			view := batchView{Action: "batch", Budget: budget, Rows: make([]batchItemResult, 0, len(rows))}
			for i, r := range rows {
				item := batchItemResult{Row: i + 1, Model: r.Model, Prompt: r.Prompt, Status: "estimated"}
				if cached, err := db.GetEndpointCache(ctx, r.Model); err == nil && cached != nil {
					if unit, _, _ := cheapestOutputUnit(cached.Data); unit > 0 {
						n := r.N
						if n < 1 {
							n = 1
						}
						item.Estimate = unit * float64(n)
					}
				}
				if item.Estimate == 0 {
					item.Status = "error"
					item.Error = "no cached pricing for model; run sync first or drop the budget gate"
				}
				view.TotalEstimate += item.Estimate
				view.Rows = append(view.Rows, item)
			}

			// Budget gate: if any row lacks pricing when a budget is set, or
			// the total exceeds the budget, abort before any spend. A row
			// without cached pricing estimates at $0, so it must not silently
			// keep the run under budget and then spend credits the cap cannot
			// bound.
			missingPricing := false
			for _, item := range view.Rows {
				if item.Status == "error" {
					missingPricing = true
					break
				}
			}
			view.UnderBudget = view.TotalEstimate <= budget && !missingPricing
			if budget > 0 && !view.UnderBudget {
				switch {
				case missingPricing:
					view.Note = "one or more rows lack cached pricing; run sync first so the budget gate can enforce the cap"
				default:
					view.Note = fmt.Sprintf("total estimate $%.2f exceeds budget $%.2f; no images were generated", view.TotalEstimate, budget)
				}
				if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				for _, item := range view.Rows {
					fmt.Fprintf(cmd.OutOrStdout(), "row %d: %s $%.4f\n", item.Row, item.Model, item.Estimate)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", view.Note)
				return nil
			}

			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			// Execute: generate each row.
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// A row that was billed but could not be recorded in the ledger
			// fails the whole run: surface a run-level error (non-zero exit)
			// so automation cannot treat a batch with missing cost history
			// as clean, and regenerate cannot silently lose it.
			var runErr error
			for i := range view.Rows {
				item := &view.Rows[i]
				r := rows[i]
				body := map[string]any{"model": r.Model, "prompt": r.Prompt}
				if r.N > 0 {
					body["n"] = r.N
				}
				if r.Resolution != "" {
					body["resolution"] = r.Resolution
				}
				if r.Quality != "" {
					body["quality"] = r.Quality
				}
				data, statusCode, err := c.PostWithParams(ctx, "/images", nil, body)
				if err != nil {
					item.Status = "error"
					item.Error = err.Error()
					continue
				}
				if statusCode < 200 || statusCode >= 300 {
					item.Status = "error"
					item.Error = fmt.Sprintf("HTTP %d", statusCode)
					continue
				}
				// Use the shared response parser so streaming and plain-JSON
				// responses (and every documented event shape) behave the
				// same here as in generate/regenerate.
				resp, err := parseImagesResponse(data, false)
				if err != nil {
					item.Status = "error"
					item.Error = fmt.Sprintf("parse: %v", err)
					continue
				}
				item.Status = "generated"
				if resp.Usage != nil {
					item.Cost = resp.Usage.Cost
					view.TotalCost += resp.Usage.Cost
				}
				// Save every image the row paid for. With a file --output and
				// multiple images, suffix each so later writes do not
				// overwrite earlier ones.
				if r.Output != "" {
					saved := make([]string, 0, len(resp.Data))
					for i, img := range resp.Data {
						if img.B64JSON == "" {
							continue
						}
						raw, err := decodeB64(img.B64JSON)
						if err != nil {
							item.Status = "error"
							item.Error = fmt.Sprintf("decode image %d: %v", i, err)
							break
						}
						outPath := r.Output
						if len(resp.Data) > 1 {
							fileExt := filepath.Ext(r.Output)
							base := strings.TrimSuffix(r.Output, fileExt)
							outPath = fmt.Sprintf("%s-%d%s", base, i, fileExt)
						}
						// #nosec G306 -- output images need read permission for the user's tools
						if err := os.WriteFile(outPath, raw, 0o644); err != nil {
							item.Status = "error"
							item.Error = fmt.Sprintf("write image %d: %v", i, err)
							break
						}
						saved = append(saved, outPath)
					}
					item.SavedTo = strings.Join(saved, ", ")
				}
				// Ledger write. A failed ledger write after a billed
				// generation must surface, not silently vanish, or the cost
				// never lands in the ledger and regenerate cannot find it.
				paramsJSON, _ := json.Marshal(body)
				ledgerID := newLedgerID(r.Model)
				entry := store.GenerationEntry{
					ID:         ledgerID,
					Model:      r.Model,
					Prompt:     r.Prompt,
					Params:     string(paramsJSON),
					OutputPath: item.SavedTo,
				}
				if resp.Usage != nil {
					entry.CostUSD = resp.Usage.Cost
				}
				if err := db.LedgerGeneration(ctx, entry); err != nil {
					item.Status = "error"
					item.Error = fmt.Sprintf("ledger: %v", err)
					if runErr == nil {
						runErr = fmt.Errorf("row %d (%s) was billed but its ledger write failed: %w", item.Row, r.Model, err)
					}
					continue
				}
				item.LedgerID = ledgerID
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
				return runErr
			}
			for _, item := range view.Rows {
				fmt.Fprintf(cmd.OutOrStdout(), "row %d: %-10s %-40s $%.4f\n", item.Row, item.Status, item.Model, item.Estimate)
				if item.Error != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  error: %s\n", item.Error)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "total estimate $%.2f | spent $%.2f | budget $%.2f\n", view.TotalEstimate, view.TotalCost, view.Budget)
			return runErr
		},
	}
	cmd.Flags().StringVar(&flagSpec, "spec", "", "Path to a CSV file (header: prompt,model,n,resolution,quality,output)")
	cmd.Flags().StringVar(&flagBudget, "budget", "", "Hard USD budget; abort before any spend if the total estimate exceeds it")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (default: platform data dir)")
	return cmd
}

func parseBatchCSV(path string) ([]batchRow, error) {
	// #nosec G304 -- user-named CSV spec path, explicit CLI input
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("empty or header-only CSV")
	}
	header := make([]string, len(records[0]))
	for i, h := range records[0] {
		header[i] = strings.ToLower(strings.TrimSpace(h))
	}
	col := func(name string) int {
		for i, h := range header {
			if h == name {
				return i
			}
		}
		return -1
	}
	cPrompt := col("prompt")
	cModel := col("model")
	if cPrompt < 0 || cModel < 0 {
		return nil, fmt.Errorf("CSV must have prompt and model columns")
	}
	get := func(rec []string, idx int) string {
		if idx < 0 || idx >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[idx])
	}
	out := make([]batchRow, 0, len(records)-1)
	for _, rec := range records[1:] {
		if len(rec) == 0 || (len(rec) == 1 && strings.TrimSpace(rec[0]) == "") {
			continue
		}
		r := batchRow{Prompt: get(rec, cPrompt), Model: get(rec, cModel)}
		if v := get(rec, col("n")); v != "" {
			r.N, _ = strconv.Atoi(v)
		}
		r.Resolution = get(rec, col("resolution"))
		r.Quality = get(rec, col("quality"))
		r.Output = get(rec, col("output"))
		if r.Prompt == "" || r.Model == "" {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}
