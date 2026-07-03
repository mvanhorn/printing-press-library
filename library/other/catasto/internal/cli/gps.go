// Copyright 2026 roberto-bissanti. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func newGpsCmd(flags *rootFlags) *cobra.Command {
	var stdin bool
	var strict bool

	cmd := &cobra.Command{
		Use:   "gps [lon] [lat]",
		Short: "Resolve a WGS84 lon/lat point to its Italian cadastral parcel (province, comune, foglio, particella).",
		Long: "Calls the Agenzia delle Entrate public ajax endpoint. Accepts a single point as two positional args (lon then lat), or a stream of \"lon,lat\" lines on stdin with --stdin.\n\n" +
			"Italy spans roughly 6.6–18.5 longitude and 35.5–47.1 latitude. Points outside cadastral coverage return an empty response unless --strict is set.",
		Example: "  catasto-pp-cli gps 12.4924 41.8902 --json\n" +
			"  echo '12.4924,41.8902' | catasto-pp-cli gps --stdin --json\n" +
			"  catasto-pp-cli gps 9.19 45.4642 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdin && len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			lookupOne := func(lon, lat float64) (json.RawMessage, error) {
				params := map[string]string{
					"op":  "getDatiOggetto",
					"lon": strconv.FormatFloat(lon, 'f', -1, 64),
					"lat": strconv.FormatFloat(lat, 'f', -1, 64),
				}
				data, _, err := resolveRead(cmd.Context(), c, flags, "lookup", false, "/inspire/ajax/ajax.php", params, nil)
				if err != nil {
					return nil, classifyAPIError(err, flags)
				}
				return extractResponseData(data), nil
			}

			if stdin {
				return streamGpsStdin(cmd, lookupOne, strict, flags)
			}

			if len(args) != 2 {
				return usageErr(fmt.Errorf("gps takes two positional args (lon lat), got %d. Try: catasto-pp-cli gps <lon> <lat>", len(args)))
			}
			lon, err := strconv.ParseFloat(args[0], 64)
			if err != nil {
				return usageErr(fmt.Errorf("invalid longitude %q: %w", args[0], err))
			}
			lat, err := strconv.ParseFloat(args[1], 64)
			if err != nil {
				return usageErr(fmt.Errorf("invalid latitude %q: %w", args[1], err))
			}
			if !pointInItalyBBox(lon, lat) {
				return usageErr(fmt.Errorf("point (lon=%v, lat=%v) is outside Italy's bounding box (lon 6.6–18.5, lat 35.5–47.1)", lon, lat))
			}
			data, err := lookupOne(lon, lat)
			if err != nil {
				return err
			}
			if strict && isEmptyAjaxResult(data) {
				return notFoundErr(fmt.Errorf("no cadastral parcel found at lon=%v, lat=%v", lon, lat))
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().BoolVar(&stdin, "stdin", false, "Read 'lon,lat' pairs from stdin (one per line, CSV-compatible). Outputs one JSON line per input row.")
	cmd.Flags().BoolVar(&strict, "strict", false, "Return a NotFound error when the upstream returns an empty result (default: emit the empty result).")
	return cmd
}

func pointInItalyBBox(lon, lat float64) bool {
	return lon >= 6.6 && lon <= 18.6 && lat >= 35.4 && lat <= 47.2
}

func isEmptyAjaxResult(data json.RawMessage) bool {
	s := string(data)
	if len(s) == 0 || s == "null" || s == "{}" || s == "[]" || s == `""` {
		return true
	}
	// Heuristic: a real result always has a COD_COMUNE field.
	var probe map[string]any
	if err := json.Unmarshal(data, &probe); err == nil {
		if v, ok := probe["COD_COMUNE"]; ok {
			if s, isStr := v.(string); isStr && s != "" && s != "null" {
				return false
			}
		}
		return true
	}
	return false
}
