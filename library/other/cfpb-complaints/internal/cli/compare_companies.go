// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelCompareCompaniesCmd(flags *rootFlags) *cobra.Command {
	var product, state, window string

	cmd := &cobra.Command{
		Use:         "companies COMPANY [COMPANY...]",
		Short:       "Compare companies inside one explicit cohort while labeling counts as non-market-adjusted complaint volume.",
		Example:     "cfpb-complaints-pp-cli compare companies 'CAPITAL ONE FINANCIAL CORPORATION' 'DISCOVER BANK' --product 'Credit card' --window 90d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			duration, err := parseWindow(window)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			start, end := currentCalendarRange(duration)
			rows := make([]map[string]any, 0, len(args))
			for _, company := range args {
				cohort := complaintCohort{Company: company, Product: product, State: state, Window: window, Size: 0}
				response, fetchErr := fetchCFPB(ctx, flags, cohortParams(cohort, start, end))
				if fetchErr != nil {
					return fetchErr
				}
				summary := cohortSummary(response)
				summary["company"] = company
				rows = append(rows, summary)
			}
			return emitCFPB(cmd, flags, map[string]any{"cohort": map[string]any{"product": product, "state": state, "window": window, "dates": rangeMetadata(start, end)}, "companies": rows, "interpretation": "Counts are published complaint volume inside one shared cohort, not market-adjusted rates or quality rankings.", "caveats": standardCaveats()})
		},
	}
	cmd.Flags().StringVar(&product, "product", "", "Optional exact product label shared by every company")
	cmd.Flags().StringVar(&state, "state", "", "Optional two-letter state code shared by every company")
	cmd.Flags().StringVar(&window, "window", "90d", "Shared cohort window")
	return cmd
}
