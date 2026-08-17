// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call through echoGet in echo_novel_support.go

package cli

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"
)

func newNovelNearbyExplainCmd(flags *rootFlags) *cobra.Command {
	var latitude, longitude, radius float64
	var detailLimit, limit int

	cmd := &cobra.Command{
		Use:         "explain",
		Short:       "List facilities within an EPA-supported radius with selected compliance and enforcement summary fields.",
		Example:     "epa-echo-pp-cli nearby explain --latitude 41.83 --longitude -71.42 --radius 5 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			if !cmd.Flags().Changed("latitude") || !cmd.Flags().Changed("longitude") {
				return errors.New("both --latitude and --longitude are required")
			}
			if latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
				return errors.New("valid --latitude and --longitude are required")
			}
			if radius <= 0 || radius > 100 {
				return errors.New("--radius must be greater than 0 and at most 100 miles")
			}
			if detailLimit < 0 || detailLimit > 25 {
				return errors.New("--detail-limit must be between 0 and 25")
			}
			if limit < 1 || limit > 100 {
				return errors.New("--limit must be between 1 and 100")
			}
			params := url.Values{"p_lat": {strconv.FormatFloat(latitude, 'f', 6, 64)}, "p_long": {strconv.FormatFloat(longitude, 'f', 6, 64)}, "p_radius": {strconv.FormatFloat(radius, 'f', 2, 64)}}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			summary, facilities, err := searchEcho(ctx, flags, params)
			if err != nil {
				return err
			}
			if len(facilities) > limit {
				facilities = facilities[:limit]
			}
			rows := make([]map[string]any, 0, len(facilities))
			detailed := 0
			detailFailures := 0
			for i, facility := range facilities {
				row := facilityConcern(facility)
				if i < detailLimit {
					id := fmt.Sprint(facility["RegistryID"])
					if id != "" && id != "<nil>" {
						detail, detailErr := getDFR(ctx, flags, id)
						if detailErr != nil {
							row["evidence_error"] = detailErr.Error()
							detailFailures++
						} else {
							row["evidence"] = selectedSections(detail, "InspectionEnforcementSummary", "ComplianceHistory", "ViolationsEnforcementActions", "FormalActions", "ICISFormalActions", "CaseFormalActions", "EnforcementComplianceSummaries")
							detailed++
						}
					}
				}
				rows = append(rows, row)
			}
			return emitECHO(cmd, flags, "live", map[string]any{"location": map[string]any{"latitude": latitude, "longitude": longitude, "radius_miles": radius}, "summary": summary, "coverage": map[string]any{"total_matches": queryRowCount(summary), "returned": len(rows), "detail_attempted": min(detailLimit, len(rows)), "detailed": detailed, "detail_failures": detailFailures, "truncated": queryRowCount(summary) > len(rows)}, "facilities": rows, "explanation": "Facilities retain EPA order. Bounded detailed-report failures are attached to their facility instead of hiding the successful nearby scan; no composite score or inferred ranking is added.", "caveats": echoCaveats()})
		},
	}
	cmd.Flags().Float64Var(&latitude, "latitude", 0, "Latitude in decimal degrees")
	cmd.Flags().Float64Var(&longitude, "longitude", 0, "Longitude in decimal degrees")
	cmd.Flags().Float64Var(&radius, "radius", 5, "Search radius in miles (maximum 100)")
	cmd.Flags().IntVar(&detailLimit, "detail-limit", 10, "Fetch detailed evidence for the first N facilities (0-25)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum facilities to return (1-100)")
	return cmd
}
