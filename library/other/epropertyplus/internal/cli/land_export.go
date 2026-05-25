// Copyright 2026 startos00. Licensed under Apache-2.0. See LICENSE.

// Hand-built novel command: `land-export` — export vacant lots projecting the
// fields the Land Designer needs (parcel geometry, zoning, potential use).

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/epropertyplus/internal/landbank"

	"github.com/spf13/cobra"
)

// landDesignerProps projects a lot Property into the Land-Designer field set.
func landDesignerProps(p landbank.Property) map[string]any {
	m := map[string]any{
		"parcelNumber":     p.ParcelNumber,
		"propertyAddress1": p.PropertyAddress1,
		"zonedAs":          p.ZonedAs,
		"potentialUse":     p.PotentialUse,
		"propertyClass":    p.PropertyClass,
	}
	if p.Latitude != nil {
		m["latitude"] = *p.Latitude
	}
	if p.Longitude != nil {
		m["longitude"] = *p.Longitude
	}
	if p.ParcelSquareFootage != nil {
		m["parcelSquareFootage"] = *p.ParcelSquareFootage
	}
	if p.ParcelLength != nil {
		m["parcelLength"] = *p.ParcelLength
	}
	if p.ParcelWidth != nil {
		m["parcelWidth"] = *p.ParcelWidth
	}
	if p.AskingPrice != nil {
		m["askingPrice"] = *p.AskingPrice
	}
	return m
}

func newLandExportCmd(flags *rootFlags) *cobra.Command {
	var asGeoJSON bool
	var limit int

	cmd := &cobra.Command{
		Use:   "land-export",
		Short: "Export vacant-land parcels with geometry, zoning, and potential use",
		Long: strings.Trim(`
Export only the vacant lots (the buildable land) from the active instance,
projecting the fields the Land Designer needs:

  parcelNumber, propertyAddress1, latitude, longitude, parcelSquareFootage,
  parcelLength, parcelWidth, zonedAs, potentialUse, propertyClass, askingPrice

Prefers the synced local store; falls back to hydrating live (up to --limit).
Use --json, --csv, or --geojson to pick the format (default: table for a TTY,
JSON when piped).
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  epropertyplus-pp-cli land-export --json --limit 20
  epropertyplus-pp-cli land-export --geojson
  epropertyplus-pp-cli --instance kclb land-export --csv
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			// --json / --csv are inherited persistent root flags and already
			// drive the output pipeline; only --geojson is local here.

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			loaded, source, err := loadProperties(cmd.Context(), c, flags, limit, "lot")
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if asGeoJSON {
				sel := selectedFieldSet(flags.selectFields)
				inputs := make([]landbank.FeatureInput, 0, len(loaded))
				for _, lp := range loaded {
					props := landDesignerProps(lp.Prop)
					if sel != nil {
						for k := range props {
							if !sel[k] {
								delete(props, k)
							}
						}
					}
					in := landbank.FeatureInput{Props: props}
					if lp.Prop.Latitude != nil && lp.Prop.Longitude != nil {
						in.Lat = *lp.Prop.Latitude
						in.Lng = *lp.Prop.Longitude
						in.HasCoords = true
					}
					inputs = append(inputs, in)
				}
				fc := landbank.BuildFeatureCollection(inputs)
				b, err := landbank.MarshalFeatureCollection(fc)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}

			rows := make([]map[string]any, 0, len(loaded))
			for _, lp := range loaded {
				rows = append(rows, landDesignerProps(lp.Prop))
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.ErrOrStderr(), "%d lot(s) (%s)\n", len(rows), source)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().BoolVar(&asGeoJSON, "geojson", false, "Output as a GeoJSON FeatureCollection (use the inherited --json/--csv for those formats)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max properties to hydrate live (0 = default cap; ignored when reading the synced store)")
	return cmd
}
