// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/passive-indices/internal/indiapassivefunds"
)

// fundDetailView is the flattened, agent-friendly shape of a fund detail
// response. Field codes are resolved to names in the sibling client.
type fundDetailView struct {
	SchemeID      string             `json:"scheme_id"`
	SchemeName    string             `json:"scheme_name"`
	CategoryName  string             `json:"category_name"`
	SchemeType    string             `json:"scheme_type"`
	Riskometer    string             `json:"riskometer,omitempty"`
	BenchmarkText string             `json:"benchmark_index,omitempty"`
	NAV           string             `json:"nav,omitempty"`
	NAVDate       string             `json:"nav_date,omitempty"`
	AUM           string             `json:"aum,omitempty"`
	AUMDate       string             `json:"aum_date,omitempty"`
	ExpenseRatio  float64            `json:"expense_ratio,omitempty"`
	TrackingError float64            `json:"tracking_error,omitempty"`
	TrackingDiff  float64            `json:"tracking_difference,omitempty"`
	RatiosAsOf    string             `json:"ratios_as_of,omitempty"`
	SectorWeights []namedPercentView `json:"sector_weights,omitempty"`
	TopHoldings   []namedPercentView `json:"top_holdings,omitempty"`
}

type namedPercentView struct {
	Name    string  `json:"name"`
	Percent float64 `json:"percent"`
}

func newFundGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <schemeId>",
		Short:   "Get a fund's full detail (NAV, AUM, expense ratio, tracking error, benchmark index)",
		Example: "  passive-indices-pp-cli fund get 1150 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch fund detail")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("schemeId is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newIndiaPassiveFundsClient(flags)
			fd, err := c.FundDetail(ctx, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			view := fundDetailToView(fd)

			if db, _, openErr := openStoreForCache(flags); openErr == nil {
				if data, err := json.Marshal(view); err == nil {
					_ = db.Upsert(resourceTypeFund, fd.SchemeID, data)
				}
				db.Close()
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !wantsHumanTable(cmd.OutOrStdout(), flags)) {
				return flags.printJSON(cmd, view)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s (%s)\n", view.SchemeName, view.SchemeType)
			fmt.Fprintf(cmd.OutOrStdout(), "  Benchmark: %s\n", view.BenchmarkText)
			fmt.Fprintf(cmd.OutOrStdout(), "  NAV: %s (%s)   AUM: %s (%s)\n", view.NAV, view.NAVDate, view.AUM, view.AUMDate)
			fmt.Fprintf(cmd.OutOrStdout(), "  Expense Ratio: %.2f   Tracking Error: %.2f   Tracking Diff: %.2f (as of %s)\n",
				view.ExpenseRatio, view.TrackingError, view.TrackingDiff, view.RatiosAsOf)
			return nil
		},
	}
	return cmd
}

func fundDetailToView(fd *indiapassivefunds.FundDetail) *fundDetailView {
	v := &fundDetailView{
		SchemeID:      fd.SchemeID,
		SchemeName:    fd.SchemeName,
		CategoryName:  fd.CategoryName,
		SchemeType:    fd.SchemeType,
		Riskometer:    fd.Riskometer,
		BenchmarkText: fd.BenchmarkText,
		NAV:           fd.NAV,
		NAVDate:       fd.NAVDate,
		AUM:           fd.AUM,
		AUMDate:       fd.AUMDate,
		ExpenseRatio:  fd.ExpenseRatio,
		TrackingError: fd.TrackingError,
		TrackingDiff:  fd.TrackingDiff,
		RatiosAsOf:    fd.RatiosAsOf,
	}
	for _, sw := range fd.SectorWeights {
		v.SectorWeights = append(v.SectorWeights, namedPercentView{Name: sw.Name, Percent: sw.Percent})
	}
	for _, th := range fd.TopHoldings {
		v.TopHoldings = append(v.TopHoldings, namedPercentView{Name: th.Name, Percent: th.Percent})
	}
	return v
}
