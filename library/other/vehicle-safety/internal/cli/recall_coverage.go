// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelRecallCoverageCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "recall-coverage VIN",
		Short:       "Contrast VIN-specific unrepaired recalls with model-wide campaigns.",
		Example:     "  vehicle-safety-pp-cli recall-coverage 1HGCV1F34LA000001 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			vin := strings.ToUpper(strings.TrimSpace(args[0]))
			if !regexp.MustCompile(`^[A-HJ-NPR-Z0-9]{17}$`).MatchString(vin) {
				return errors.New("VIN must contain exactly 17 valid characters (I, O, and Q are not allowed)")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			decoded, err := nhtsaGet(ctx, flags, "https://vpic.nhtsa.dot.gov/api", "/vehicles/DecodeVinValues/"+url.PathEscape(vin), url.Values{"format": {"json"}})
			if err != nil {
				return err
			}
			if len(decoded.Results) == 0 {
				return errors.New("vPIC returned no VIN decode result")
			}
			row := decoded.Results[0]
			yearText := stringValue(row, "ModelYear")
			year, err := strconv.Atoi(yearText)
			if err != nil {
				return errors.New("VIN did not decode to a model year")
			}
			vehicle, err := validateVehicle(year, stringValue(row, "Make"), stringValue(row, "Model"))
			if err != nil {
				return err
			}
			recalls, err := nhtsaGet(ctx, flags, nhtsaBaseURL, "/recalls/recallsByVehicle", vehicleParams(vehicle))
			if err != nil {
				return err
			}
			result := map[string]any{"vin": vin, "decoded_vehicle": vehicle, "model_level_campaigns": recalls.Results, "vin_specific_unrepaired_status": map[string]any{"status": "not_queried", "reason": "NHTSA offers its public VIN repair-status lookup as a consumer web flow and warns against bulk VIN API use.", "official_lookup": "https://www.nhtsa.gov/recalls?vin=" + vin}, "coverage_interpretation": "These campaigns match the decoded year/make/model only. They are not proof that this VIN is included or unrepaired."}
			return emitLive(cmd, flags, result)
		},
	}
	return cmd
}
