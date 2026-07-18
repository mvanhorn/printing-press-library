// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

func newNovelEmergingThemesCmd(flags *rootFlags) *cobra.Command {
	var company, product, state, window, baseline string

	cmd := &cobra.Command{
		Use:         "themes --company COMPANY",
		Short:       "Rank mechanical product and issue count changes between current and baseline windows without semantic or causal claims.",
		Example:     "cfpb-complaints-pp-cli emerging themes --company 'TRANSUNION INTERMEDIATE HOLDINGS, INC.' --window 7d --baseline 7d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			cohort, currentDuration, err := validateCohort(company, product, state, window, 0)
			if err != nil {
				return err
			}
			baselineDuration, err := parseWindow(baseline)
			if err != nil {
				return err
			}
			if currentDuration != baselineDuration {
				return errors.New("--window and --baseline must use equal durations so raw count deltas have comparable exposure")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			start, end := currentCalendarRange(currentDuration)
			current, err := fetchCFPB(ctx, flags, cohortParams(cohort, start, end))
			if err != nil {
				return err
			}
			priorStart, priorEnd := start.Add(-baselineDuration), start
			prior, err := fetchCFPB(ctx, flags, cohortParams(cohort, priorStart, priorEnd))
			if err != nil {
				return err
			}
			return emitCFPB(cmd, flags, map[string]any{"cohort": map[string]any{"company": company, "product": product, "state": state, "current_window": window, "baseline_window": baseline, "current_dates": rangeMetadata(start, end), "baseline_dates": rangeMetadata(priorStart, priorEnd)}, "products": deltaBuckets(current, prior, "product"), "issues": deltaBuckets(current, prior, "issue"), "interpretation": "Only categories returned in both potentially truncated bucket sets receive a raw count delta; absent buckets remain unknown.", "caveats": standardCaveats()})
		},
	}
	cmd.Flags().StringVar(&company, "company", "", "Exact CFPB company label")
	cmd.Flags().StringVar(&product, "product", "", "Optional exact product label")
	cmd.Flags().StringVar(&state, "state", "", "Optional two-letter state code")
	cmd.Flags().StringVar(&window, "window", "7d", "Current window")
	cmd.Flags().StringVar(&baseline, "baseline", "7d", "Immediately preceding comparison window (use the same duration as --window)")
	return cmd
}
