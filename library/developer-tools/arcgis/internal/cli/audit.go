// Copyright 2026 togorashi45 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: walk every layer in a service and report which high-value
// fields each exposes. The "what are we dropping" schema audit.

package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/arcgis/internal/arcgis"

	"github.com/spf13/cobra"
)

// highValueSignals maps a signal label to regexes that match likely field names.
// Tuned for real-estate lead-gen (owner, absentee, homestead, distress, sale).
var highValueSignals = []struct {
	Label string
	Re    *regexp.Regexp
}{
	{"apn/parcel", regexp.MustCompile(`(?i)\b(apn|parcel|pin|ain|gpin|folio|account)\b|parcelid|parcel_?num`)},
	{"owner", regexp.MustCompile(`(?i)owner|own_?name|taxpayer|grantee`)},
	{"mailing address", regexp.MustCompile(`(?i)mail(ing)?_?(addr|address|city|zip|state)|owner_?addr`)},
	{"situs address", regexp.MustCompile(`(?i)situs|site_?addr|prop(erty)?_?addr|phys_?addr`)},
	{"homestead/exemption", regexp.MustCompile(`(?i)homestead|exempt|hmstd|hs_?ex`)},
	{"land use/zoning", regexp.MustCompile(`(?i)land_?use|luc|zoning|zone|use_?code|class`)},
	{"assessed/market value", regexp.MustCompile(`(?i)assess|market_?val|appraised|totval|land_?val|imp_?val|tax_?val`)},
	{"last sale", regexp.MustCompile(`(?i)sale_?(date|price|amt)|last_?sale|deed|saleprice|saledt`)},
	{"acreage/area", regexp.MustCompile(`(?i)acre|acreage|area|sqft|shape_?area|gis_?acre`)},
	{"year built", regexp.MustCompile(`(?i)year_?built|yr_?blt|yearbuilt|actyrblt`)},
}

// pp:data-source live
func newNovelAuditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit <service-url>",
		Short: "Walk every layer in a service and report which high-value fields each exposes",
		Long: `Walk all layers in an ArcGIS service and report, per layer, which real-estate
signal fields it exposes: parcel id, owner, mailing vs situs address, homestead
exemption, land use, assessed value, last sale, acreage, year built.

Use this before building a county load to see which distress-signal fields a
county's layers actually carry, so you don't leave motivation signals on the table.`,
		Example:     "  arcgis-pp-cli audit https://services.arcgis.com/P3ePLMYs2RVChkkx/arcgis/rest/services/USA_Counties/FeatureServer --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "url=https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would audit high-value fields across the service's layers")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a service URL is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newArcClient(flags)
			if err != nil {
				return err
			}
			svc, err := c.ServiceMeta(ctx, args[0])
			if err != nil {
				return fmt.Errorf("reading service metadata: %w", err)
			}
			base := arcgis.NormalizeLayerURL(args[0])

			type layerAudit struct {
				ID      int               `json:"id"`
				Name    string            `json:"name"`
				URL     string            `json:"url"`
				Signals map[string]string `json:"signals"` // label -> matched field name ("" if absent)
				Present int               `json:"signals_present"`
				Error   string            `json:"error,omitempty"`
			}
			results := make([]layerAudit, 0, len(svc.Layers))
			for _, lref := range svc.Layers {
				la := layerAudit{ID: lref.ID, Name: lref.Name, URL: fmt.Sprintf("%s/%d", base, lref.ID), Signals: map[string]string{}}
				m, err := c.LayerMeta(ctx, la.URL)
				if err != nil {
					la.Error = err.Error()
					results = append(results, la)
					continue
				}
				for _, sig := range highValueSignals {
					matched := ""
					for _, fld := range m.Fields {
						if sig.Re.MatchString(fld.Name) {
							matched = fld.Name
							break
						}
					}
					la.Signals[sig.Label] = matched
					if matched != "" {
						la.Present++
					}
				}
				results = append(results, la)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			for _, la := range results {
				if la.Error != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "[%d] %s  ERROR: %s\n", la.ID, la.Name, la.Error)
					continue
				}
				var present []string
				for _, sig := range highValueSignals {
					if f := la.Signals[sig.Label]; f != "" {
						present = append(present, fmt.Sprintf("%s=%s", sig.Label, f))
					}
				}
				fmt.Fprintf(cmd.OutOrStdout(), "[%d] %s  (%d/%d signals)\n", la.ID, la.Name, la.Present, len(highValueSignals))
				if len(present) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "     %s\n", strings.Join(present, "  "))
				}
			}
			return nil
		},
	}
	return cmd
}
