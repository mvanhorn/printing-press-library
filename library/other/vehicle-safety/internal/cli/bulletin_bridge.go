// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelBulletinBridgeCmd(flags *rootFlags) *cobra.Command {
	var flagYear int
	var flagMake string
	var flagModel string
	var flagCommunicationsFile string

	cmd := &cobra.Command{
		Use:         "bulletin-bridge --year YEAR --make MAKE --model MODEL",
		Short:       "Show structured complaint and manufacturer-communication co-occurrence candidates.",
		Example:     "  vehicle-safety-pp-cli bulletin-bridge --year 2020 --make Honda --model Civic --agent",
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
			complaints, err := nhtsaGet(ctx, flags, nhtsaBaseURL, "/complaints/complaintsByVehicle", vehicleParams(vehicle))
			if err != nil {
				return err
			}
			result := map[string]any{"vehicle": vehicle, "complaint_components": componentCounts(complaints.Results), "caveat": "Shared component words are review candidates, not evidence that a communication addresses a complaint or that a defect exists."}
			if strings.TrimSpace(flagCommunicationsFile) == "" {
				result["manufacturer_communications"] = map[string]any{"status": "file_required", "reason": "Manufacturer communications are published as NHTSA flat files, not by the public JSON API.", "documentation": "https://static.nhtsa.gov/odi/ffdd/tsbs/TSBS.txt", "next_step": "Download the matching TSV archive and pass its extracted file with --communications-file."}
				return emitLive(cmd, flags, result)
			}
			records, err := readCommunicationFile(flagCommunicationsFile, vehicle)
			if err != nil {
				return err
			}
			result["manufacturer_communications"] = records
			result["communication_count"] = len(records)
			result["co_occurrence_candidates"] = bridgeCandidates(complaints.Results, records)
			return emitNHTSA(cmd, flags, "mixed", result)
		},
	}
	cmd.Flags().IntVar(&flagYear, "year", 0, "Vehicle model year")
	cmd.Flags().StringVar(&flagMake, "make", "", "Vehicle manufacturer")
	cmd.Flags().StringVar(&flagModel, "model", "", "Vehicle model")
	cmd.Flags().StringVar(&flagCommunicationsFile, "communications-file", "", "Extracted NHTSA manufacturer-communications TSV flat file")
	return cmd
}

func bridgeCandidates(complaints []map[string]any, records []communicationRecord) []map[string]any {
	counts := componentCounts(complaints)
	var candidates []map[string]any
	for _, record := range records {
		haystack := strings.ToUpper(record.Components + " " + record.System + " " + record.Subsystem + " " + record.Summary)
		var overlaps []string
		for _, item := range counts {
			component := strings.TrimSpace(strings.Split(strings.ToUpper(fmt.Sprint(item["component"])), ":")[0])
			if len(component) >= 4 && strings.Contains(haystack, component) {
				overlaps = append(overlaps, component)
			}
		}
		if len(overlaps) > 0 {
			candidates = append(candidates, map[string]any{"nhtsa_id": record.ID, "document_id": record.DocumentID, "date": record.Date, "type": record.Type, "components": record.Components, "summary": record.Summary, "overlapping_component_terms": overlaps})
		}
	}
	return candidates
}
