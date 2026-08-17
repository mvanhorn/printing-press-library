// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelNarrativesPacketCmd(flags *rootFlags) *cobra.Command {
	var company, product, state, window string
	var limit int

	cmd := &cobra.Command{
		Use:         "packet --company COMPANY",
		Short:       "Select deterministic published narratives with complaint IDs, cohort metadata, and availability caveats.",
		Example:     "cfpb-complaints-pp-cli narratives packet --company 'CAPITAL ONE FINANCIAL CORPORATION' --limit 10 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			cohort, duration, err := validateCohort(company, product, state, window, limit)
			if err != nil {
				return err
			}
			cohort.NarrativeOnly = true
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			start, end := currentCalendarRange(duration)
			response, err := fetchCFPB(ctx, flags, cohortParams(cohort, start, end))
			if err != nil {
				return err
			}
			records := make([]map[string]any, 0, len(response.Hits.Hits))
			for _, hit := range response.Hits.Hits {
				records = append(records, hit.Source)
			}
			return emitCFPB(cmd, flags, map[string]any{"cohort": map[string]any{"company": company, "product": product, "state": state, "window": window, "dates": rangeMetadata(start, end)}, "published_narrative_count": response.Hits.Total.Value, "records": records, "selection": "Newest-first API order requested explicitly and bounded by --limit; every record retains its complaint_id.", "api_meta": response.Meta, "caveats": standardCaveats()})
		},
	}
	cmd.Flags().StringVar(&company, "company", "", "Exact CFPB company label")
	cmd.Flags().StringVar(&product, "product", "", "Optional exact product label")
	cmd.Flags().StringVar(&state, "state", "", "Optional two-letter state code")
	cmd.Flags().StringVar(&window, "window", "365d", "Narrative cohort window")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum published narratives (1-100)")
	return cmd
}
