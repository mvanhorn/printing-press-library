// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "compare 'YEAR MAKE MODEL' 'YEAR MAKE MODEL'",
		Short:       "Compare two model years while keeping raw complaint counts and missing denominators explicit.",
		Example:     "  vehicle-safety-pp-cli compare '2020 Honda Civic' '2020 Toyota Corolla' --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			left, err := parseVehicleArg(args[0])
			if err != nil {
				return err
			}
			right, err := parseVehicleArg(args[1])
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			leftDossier, err := fetchDossier(ctx, flags, left)
			if err != nil {
				return err
			}
			rightDossier, err := fetchDossier(ctx, flags, right)
			if err != nil {
				return err
			}
			result := map[string]any{
				"vehicles":           []any{summarizeDossier(leftDossier), summarizeDossier(rightDossier)},
				"interpretation":     "Counts are descriptive only. They are not normalized by registrations, mileage, trim mix, or time on road.",
				"source_attribution": leftDossier.SourceAttribution,
			}
			return emitLive(cmd, flags, result)
		},
	}
	return cmd
}

func summarizeDossier(d vehicleDossier) map[string]any {
	return map[string]any{"vehicle": d.Vehicle, "recall_count": len(d.Recalls), "complaint_count": len(d.Complaints), "rating_variant_count": len(d.RatingVariants), "complaint_components": componentCounts(d.Complaints), "ratings": d.Ratings}
}
