// Copyright 2026 startos00. Licensed under Apache-2.0. See LICENSE.

// Hand-built novel command: `export` — export the synced (or live) inventory
// as json, csv, or GeoJSON.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/epropertyplus/internal/landbank"

	"github.com/spf13/cobra"
)

// selectedFieldSet parses a --select value into a set, or nil when no
// selection was requested.
func selectedFieldSet(sel string) map[string]bool {
	sel = strings.TrimSpace(sel)
	if sel == "" {
		return nil
	}
	set := map[string]bool{}
	for _, f := range strings.Split(sel, ",") {
		if f = strings.TrimSpace(f); f != "" {
			set[f] = true
		}
	}
	return set
}

func newExportCmd(flags *rootFlags) *cobra.Command {
	var format string
	var kind string
	var limit int

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export an instance's inventory as json, csv, or GeoJSON",
		Long: strings.Trim(`
Export the active instance's inventory. Prefers the synced local store; falls
back to hydrating live (up to --limit) when nothing is synced.

  --format json     array of property objects (default)
  --format csv      flat CSV
  --format geojson  FeatureCollection of Point features ([lng, lat]); rows
                    with no coordinates are skipped

--kind filters to structure | lot | all (default all). --select restricts the
projected fields (applies to json/csv and to GeoJSON feature properties).
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  epropertyplus-pp-cli export --format geojson --kind lot --limit 20
  epropertyplus-pp-cli export --format csv --kind structure
  epropertyplus-pp-cli --instance kclb export --format json --select id,parcelNumber,zonedAs
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch strings.ToLower(format) {
			case "", "json", "csv", "geojson":
			default:
				return usageErr(fmt.Errorf("invalid --format %q: must be json, csv, or geojson", format))
			}
			switch strings.ToLower(kind) {
			case "", "all", "structure", "lot":
			default:
				return usageErr(fmt.Errorf("invalid --kind %q: must be all, structure, or lot", kind))
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			loaded, _, err := loadProperties(cmd.Context(), c, flags, limit, kind)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if strings.EqualFold(format, "geojson") {
				sel := selectedFieldSet(flags.selectFields)
				inputs := make([]landbank.FeatureInput, 0, len(loaded))
				for _, lp := range loaded {
					inputs = append(inputs, landbank.FeatureInputFromProperty(lp.Prop, sel))
				}
				fc := landbank.BuildFeatureCollection(inputs)
				b, err := landbank.MarshalFeatureCollection(fc)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}

			// json / csv go through the standard pipeline (csv handled by --csv,
			// but --format csv should also work directly).
			if strings.EqualFold(format, "csv") && !flags.csv {
				flags.csv = true
			}
			return emitProperties(cmd, flags, loaded)
		},
	}
	cmd.Flags().StringVar(&format, "format", "json", "Output format: json, csv, or geojson")
	cmd.Flags().StringVar(&kind, "kind", "all", "Filter by classification: all, structure, or lot")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max properties to hydrate live (0 = default cap; ignored when reading the synced store)")
	return cmd
}
