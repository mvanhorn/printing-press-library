// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"sort"
	"time"

	"github.com/spf13/cobra"
)

func newNovelSignalsCmd(flags *rootFlags) *cobra.Command {
	var flagYear int
	var flagMake string
	var flagModel string

	cmd := &cobra.Command{
		Use:         "signals --year YEAR --make MAKE --model MODEL",
		Short:       "Place complaint components beside investigation, recall, and communication dates.",
		Example:     "  vehicle-safety-pp-cli signals --year 2020 --make Honda --model Civic --agent",
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
			var timeline []map[string]any
			for _, item := range dossier.Recalls {
				rawDate := stringValue(item, "ReportReceivedDate")
				timeline = append(timeline, map[string]any{"date": normalizeNHTSADate(rawDate, "02/01/2006"), "raw_date": rawDate, "kind": "recall", "id": stringValue(item, "NHTSACampaignNumber"), "component": stringValue(item, "Component")})
			}
			for _, item := range dossier.Complaints {
				rawDate := stringValue(item, "dateComplaintFiled")
				timeline = append(timeline, map[string]any{"date": normalizeNHTSADate(rawDate, "01/02/2006"), "raw_date": rawDate, "kind": "complaint", "id": stringValue(item, "odiNumber"), "component": stringValue(item, "components"), "crash": boolValue(item, "crash"), "fire": boolValue(item, "fire")})
			}
			sort.SliceStable(timeline, func(i, j int) bool {
				left, right := stringValue(timeline[i], "date"), stringValue(timeline[j], "date")
				if left == "" || right == "" {
					return left != "" // Unknown dates sort after every normalized date.
				}
				if left != right {
					return left < right
				}
				return stringValue(timeline[i], "kind")+stringValue(timeline[i], "id") < stringValue(timeline[j], "kind")+stringValue(timeline[j], "id")
			})
			result := map[string]any{"vehicle": vehicle, "complaint_components": componentCounts(dossier.Complaints), "timeline": timeline, "formal_investigations": map[string]any{"status": "not_in_public_JSON_API", "dataset": "https://static.nhtsa.gov/odi/ffdd/inv/FLAT_INV.zip"}, "manufacturer_communications": map[string]any{"status": "not_in_public_JSON_API", "dataset_documentation": "https://static.nhtsa.gov/odi/ffdd/tsbs/TSBS.txt"}, "caveat": "Temporal proximity and component overlap do not establish causation."}
			return emitLive(cmd, flags, result)
		},
	}
	cmd.Flags().IntVar(&flagYear, "year", 0, "Vehicle model year")
	cmd.Flags().StringVar(&flagMake, "make", "", "Vehicle manufacturer")
	cmd.Flags().StringVar(&flagModel, "model", "", "Vehicle model")
	return cmd
}

func normalizeNHTSADate(raw, layout string) string {
	for _, candidate := range []string{layout, time.RFC3339, "2006-01-02", "1/2/2006", "01/02/2006"} {
		parsed, err := time.Parse(candidate, raw)
		if err == nil {
			return parsed.Format("2006-01-02")
		}
	}
	return ""
}
