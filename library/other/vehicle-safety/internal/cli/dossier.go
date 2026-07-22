// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelDossierCmd(flags *rootFlags) *cobra.Command {
	var flagYear int
	var flagMake string
	var flagModel string
	var flagIncludeRecords bool

	cmd := &cobra.Command{
		Use:         "dossier --year YEAR --make MAKE --model MODEL",
		Short:       "See identity, recalls, complaints, ratings, investigations, and communications in one source-attributed report.",
		Example:     "  vehicle-safety-pp-cli dossier --year 2020 --make Honda --model Civic --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			vehicle, err := validateVehicle(flagYear, flagMake, flagModel)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dossier, err := fetchDossier(ctx, flags, vehicle)
			if err != nil {
				return err
			}
			result := map[string]any{"vehicle": dossier.Vehicle, "retrieved_at": dossier.RetrievedAt, "recall_count": len(dossier.Recalls), "complaint_count": len(dossier.Complaints), "complaint_components": componentCounts(dossier.Complaints), "rating_variants": dossier.RatingVariants, "ratings": dossier.Ratings, "source_attribution": dossier.SourceAttribution, "caveats": dossier.Caveats}
			if flagIncludeRecords {
				result["recalls"] = dossier.Recalls
				result["complaints"] = dossier.Complaints
			}
			return emitLive(cmd, flags, result)
		},
	}
	cmd.Flags().IntVar(&flagYear, "year", 0, "Vehicle model year")
	cmd.Flags().StringVar(&flagMake, "make", "", "Vehicle manufacturer")
	cmd.Flags().StringVar(&flagModel, "model", "", "Vehicle model")
	cmd.Flags().BoolVar(&flagIncludeRecords, "include-records", false, "Include complete recall and complaint source records")
	return cmd
}
