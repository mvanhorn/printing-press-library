// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"github.com/spf13/cobra"
	"strings"
)

func newNovelCompanyPulseCmd(flags *rootFlags) *cobra.Command {
	var company, product, state, window string

	cmd := &cobra.Command{
		Use:         "pulse --company COMPANY",
		Short:       "Summarize complaint volume, products, issues, responses, timeliness, narrative availability",
		Example:     "cfpb-complaints-pp-cli company pulse --company 'TRANSUNION INTERMEDIATE HOLDINGS, INC.' --window 30d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			cohort, duration, err := validateCohort(company, product, state, window, 0)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			start, end := currentCalendarRange(duration)
			current, err := fetchCFPB(ctx, flags, cohortParams(cohort, start, end))
			if err != nil {
				return err
			}
			priorStart, priorEnd := start.Add(-duration), start
			prior, err := fetchCFPB(ctx, flags, cohortParams(cohort, priorStart, priorEnd))
			if err != nil {
				return err
			}
			return emitCFPB(cmd, flags, map[string]any{"cohort": map[string]any{"company": strings.TrimSpace(company), "product": product, "state": state, "window": window, "current_dates": rangeMetadata(start, end), "prior_dates": rangeMetadata(priorStart, priorEnd)}, "current": cohortSummary(current), "prior_complaint_count": prior.Hits.Total.Value, "complaint_count_delta": current.Hits.Total.Value - prior.Hits.Total.Value, "api_meta": current.Meta, "caveats": standardCaveats()})
		},
	}
	cmd.Flags().StringVar(&company, "company", "", "Exact CFPB company label")
	cmd.Flags().StringVar(&product, "product", "", "Optional exact product label")
	cmd.Flags().StringVar(&state, "state", "", "Optional two-letter state code")
	cmd.Flags().StringVar(&window, "window", "30d", "Current and prior comparison window")
	return cmd
}
