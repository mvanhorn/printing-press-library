// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
)

const transactionPatchPath = "/budgets/{budgetSyncId}/transactions/{transactionId}"

type categorizeApplyRow struct {
	TransactionID       string         `json:"transaction_id"`
	SuggestedCategoryID string         `json:"suggested_category_id"`
	SuggestedCategory   string         `json:"suggested_category"`
	Method              string         `json:"method"`
	Path                string         `json:"path"`
	Body                map[string]any `json:"body"`
	Status              string         `json:"status"` // planned | applied
	HTTPStatus          int            `json:"http_status,omitempty"`
}

type categorizeFailure struct {
	TransactionID string `json:"transaction_id"`
	Error         string `json:"error"`
}

type categorizeApplyResult struct {
	DryRun        bool                 `json:"dry_run"`
	Applied       []categorizeApplyRow `json:"applied"`
	FetchFailures []categorizeFailure  `json:"fetch_failures"`
}

// defaultMinConfidence is the suggestion threshold when --min-confidence is unset.
const defaultMinConfidence = 0.6

// suggestOpts holds the thresholds shared by 'categorize suggest' and
// 'categorize apply', so both commands pick exactly the same transactions.
type suggestOpts struct {
	minConfidence float64
	minHistory    int
	limit         int
}

func (o *suggestOpts) register(cmd *cobra.Command, limitHelp string) {
	cmd.Flags().Float64Var(&o.minConfidence, "min-confidence", defaultMinConfidence, "Minimum share (0..1) of the payee's history the suggested category must have")
	cmd.Flags().IntVar(&o.minHistory, "min-history", 2, "Minimum number of categorized past transactions for the payee")
	cmd.Flags().IntVar(&o.limit, "limit", 50, limitHelp)
}

func (o *suggestOpts) validate() error {
	if math.IsNaN(o.minConfidence) || o.minConfidence < 0 || o.minConfidence > 1 {
		return usageErr(fmt.Errorf("--min-confidence must be a number between 0 and 1, got %v", o.minConfidence))
	}
	if o.minHistory < 1 {
		return usageErr(errors.New("--min-history must be >= 1"))
	}
	if o.limit < 0 {
		return usageErr(errors.New("--limit must be >= 0"))
	}
	return nil
}

// loadSuggestions validates the thresholds and reads suggestions from the
// mirror. found is false when no mirror exists (openMirror already printed
// the hint), in which case the caller emits an empty result.
func loadSuggestions(cmd *cobra.Command, flags *rootFlags, o *suggestOpts) (sugg []actual.CategorySuggestion, found bool, err error) {
	if err := rejectLiveDataSource(flags); err != nil {
		return nil, false, err
	}
	if err := o.validate(); err != nil {
		return nil, false, err
	}
	h, err := openMirror(cmd, flags)
	if err != nil || h == nil {
		return nil, false, err
	}
	defer h.Close()
	sctx, scancel := boundCtx(cmd.Context(), flags)
	defer scancel()
	sugg, err = h.Ledger.SuggestCategories(sctx, o.minHistory, o.minConfidence)
	if err != nil {
		return nil, false, err
	}
	if o.limit > 0 && len(sugg) > o.limit {
		sugg = sugg[:o.limit]
	}
	return sugg, true, nil
}

func newNovelCategorizeSuggestCmd(flags *rootFlags) *cobra.Command {
	var opts suggestOpts

	cmd := &cobra.Command{
		Use:   "suggest",
		Short: "Get a category suggestion for every uncategorized transaction based on how you categorized that payee before",
		Long: `Use this command to get category suggestions for uncategorized transactions. Do NOT use this command just to list uncategorized transactions; use 'ledger uncategorized' instead.

For each uncategorized transaction in the local mirror, the suggestion is the
category used most often with the same (merge-resolved) payee across live,
categorized, non-transfer transactions. confidence is that category's share of
the payee's history; suggestions need at least --min-history history rows and
--min-confidence confidence. Starting-balance rows are never suggested. Amounts
are integer minor units.

Read-only. To write the suggestions, run 'categorize apply' with the same
thresholds.`,
		Example: strings.Trim(`
  actual-budget-pp-cli categorize suggest
  actual-budget-pp-cli categorize suggest --min-confidence 0.7 --agent
  actual-budget-pp-cli categorize suggest --min-history 3 --limit 0 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "categorize suggest")
			}
			sugg, found, err := loadSuggestions(cmd, flags, &opts)
			if err != nil {
				return err
			}
			if !found {
				sugg = make([]actual.CategorySuggestion, 0)
			}
			return emitRows(cmd, flags, sugg, func() error {
				w := cmd.OutOrStdout()
				if len(sugg) == 0 {
					fmt.Fprintln(w, "No confident category suggestions for uncategorized transactions.")
					return nil
				}
				table := make([][]string, 0, len(sugg))
				for _, s := range sugg {
					table = append(table, []string{s.Date, s.Payee, amt(s.Amount), s.SuggestedCategory, fmt.Sprintf("%.0f%%", s.Confidence*100), fmt.Sprint(s.HistoryCount), s.TransactionID})
				}
				return printOrderedTable(w, []string{"DATE", "PAYEE", "AMOUNT", "SUGGESTED", "CONFIDENCE", "HISTORY", "TRANSACTION"}, table)
			})
		},
	}
	opts.register(cmd, "Maximum suggestions to return (0 = all)")
	return cmd
}

func newNovelCategorizeApplyCmd(flags *rootFlags) *cobra.Command {
	var opts suggestOpts

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Write the categories 'categorize suggest' would suggest, through the actual-http-api sidecar",
		Long: `Use this command to write category suggestions to the budget. Run 'categorize suggest' first with the same thresholds to review them.

Computes the same suggestions as 'categorize suggest' from the local mirror and
PATCHes each one to the actual-http-api sidecar (ACTUAL_BUDGET_BASE_URL,
ACTUAL_HTTP_API_KEY) at /budgets/{budgetSyncId}/transactions/{id} with
{"transaction":{"category":id}}. --dry-run prints the planned requests without
sending them. Failed rows are reported in fetch_failures without stopping the
others. Run 'mirror pull' afterwards to refresh the mirror.`,
		Example: strings.Trim(`
  actual-budget-pp-cli categorize apply --dry-run --json
  actual-budget-pp-cli categorize apply --min-confidence 0.8 --min-history 3 --dry-run
  actual-budget-pp-cli categorize apply --limit 10 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			sugg, found, err := loadSuggestions(cmd, flags, &opts)
			if err != nil {
				return err
			}
			if !found && dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "categorize apply")
			}
			res := categorizeApplyResult{DryRun: flags.dryRun, Applied: make([]categorizeApplyRow, 0), FetchFailures: make([]categorizeFailure, 0)}
			plan := make([]categorizeApplyRow, 0, len(sugg))
			for _, s := range sugg {
				plan = append(plan, categorizeApplyRow{
					TransactionID: s.TransactionID, SuggestedCategoryID: s.SuggestedCategoryID, SuggestedCategory: s.SuggestedCategory,
					Method: "PATCH", Path: replacePathParam(transactionPatchPath, "transactionId", s.TransactionID),
					Body: map[string]any{"transaction": map[string]any{"category": s.SuggestedCategoryID}}, Status: "planned",
				})
			}
			if flags.dryRun {
				res.Applied = plan
			} else if len(plan) > 0 {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				for i, p := range plan {
					if cmd.Context().Err() != nil {
						for _, rest := range plan[i:] {
							res.FetchFailures = append(res.FetchFailures, categorizeFailure{TransactionID: rest.TransactionID, Error: "not attempted: interrupted"})
						}
						break
					}
					// Each PATCH gets its own --timeout so a long batch does not
					// starve the later rows of a shared deadline.
					ctx, cancel := boundCtx(cmd.Context(), flags)
					_, status, err := c.PatchWithHeaders(ctx, p.Path, p.Body, sidecarHeaders())
					cancel()
					if err == nil && (status < 200 || status >= 300) {
						err = fmt.Errorf("HTTP %d", status)
					}
					if err != nil {
						res.FetchFailures = append(res.FetchFailures, categorizeFailure{TransactionID: p.TransactionID, Error: err.Error()})
						continue
					}
					p.Status, p.HTTPStatus = "applied", status
					res.Applied = append(res.Applied, p)
				}
				if n := len(res.FetchFailures); n > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d category updates failed; see fetch_failures\n", n, len(plan))
				}
			}
			if err := emitRows(cmd, flags, res, func() error {
				w := cmd.OutOrStdout()
				verb := "Applied"
				if res.DryRun {
					verb = "Would apply"
				}
				fmt.Fprintf(w, "%s %d category update(s).\n", verb, len(res.Applied))
				for _, a := range res.Applied {
					fmt.Fprintf(w, "  %s %s -> %s\n", a.Method, a.Path, a.SuggestedCategory)
				}
				for _, f := range res.FetchFailures {
					fmt.Fprintf(w, "  FAILED %s: %s\n", f.TransactionID, f.Error)
				}
				return nil
			}); err != nil {
				return err
			}
			if err := cmd.Context().Err(); err != nil && !res.DryRun {
				return fmt.Errorf("interrupted after %d of %d category updates: %w", len(res.Applied), len(res.Applied)+len(res.FetchFailures), err)
			}
			if len(res.FetchFailures) > 0 && len(res.Applied) == 0 {
				return apiErr(fmt.Errorf("all %d category updates failed", len(res.FetchFailures)))
			}
			return nil
		},
	}
	opts.register(cmd, "Maximum suggestions to apply (0 = all)")
	return cmd
}
