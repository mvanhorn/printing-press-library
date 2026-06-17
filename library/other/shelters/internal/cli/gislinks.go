// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.
//
// The `gis-links` command: stable link-outs to the authoritative FEMA services
// and the broader NSS program. Reference only; this CLI never ingests the GIS
// layers, it parses the OpenShelters attributes feed.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelGisLinksCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "gis-links",
		Short:       "Authoritative FEMA and American Red Cross service URLs and the full-NSS access path (link-out only)",
		Long:        "Print the stable layer URLs this CLI reads: the FEMA OpenShelters spine, the American Red Cross Emergency-Action view it unions in, the FEMA_NSS/0 enrichment layer, and the American Red Cross Open_Shelters layer it reads live occupancy (current population and capacity) from, plus the broader National Shelter System program page (full access requires an MOU) and the free geocoder used by 'near'. Link-out only; never ingested.",
		Example:     "  shelters-pp-cli gis-links",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			links := map[string]any{
				"openshelters_feature_layer": featureServerURL,
				"openshelters_query":         openSheltersBase + openSheltersQuery,
				"fema_nss_enrichment_layer":  femaNSSLayerURL,
				"red_cross_ea_view":          redCrossLayerURL,
				"open_shelters_occupancy":    openSheltersOccLayerURL,
				"national_shelter_system":    fullNSSInfoURL,
				"census_geocoder":            censusGeocoderBase,
				"note":                       "This CLI unions the FEMA OpenShelters feed (spine) with the American Red Cross Emergency-Action view (the redcross.org map feed, deduped by shelter), best-effort enriches FEMA rows from the richer FEMA_NSS/0 layer (county, incident, generator, populations), and overlays live occupancy (current population and capacity) from the American Red Cross Open_Shelters layer, matched by name+state+ZIP. The occupancy overlay is fill-only: it adds population onto shelters already in the public feeds and never adds a site the Red Cross keeps off the public map, so the CLI shows only publicly listed shelters. It also fetches the Red Cross hidden set (hide_from_public other than 'No') and suppresses any matching shelter, even one FEMA's public feed lists, so a site the Red Cross has hidden from the public is never surfaced. FEMA is synchronized downstream of Red Cross, so a freshly opened shelter can appear in Red Cross first. The Red Cross EA endpoint requires a Referer header; Open_Shelters does not. The GIS layers are referenced, not ingested. Full NSS (beyond public OpenShelters) requires an MOU with FEMA.",
			}
			// Prose only for an interactive terminal; piped / --json / --agent /
			// --quiet consumers get the machine path (pipe-default contract).
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return emitData(cmd, flags, links)
			}
			var b strings.Builder
			fmt.Fprintln(&b, "FEMA National Shelter System -- link-outs (not ingested):")
			fmt.Fprintf(&b, "  OpenShelters layer : %s\n", featureServerURL)
			fmt.Fprintf(&b, "  OpenShelters query : %s\n", openSheltersBase+openSheltersQuery)
			fmt.Fprintf(&b, "  FEMA_NSS enrich    : %s\n", femaNSSLayerURL)
			fmt.Fprintf(&b, "  Red Cross EA view  : %s\n", redCrossLayerURL)
			fmt.Fprintf(&b, "  Open_Shelters occ  : %s\n", openSheltersOccLayerURL)
			fmt.Fprintf(&b, "  NSS program        : %s\n", fullNSSInfoURL)
			fmt.Fprintf(&b, "  Census geocoder    : %s\n", censusGeocoderBase)
			fmt.Fprintln(&b)
			fmt.Fprintln(&b, "Full NSS (beyond public OpenShelters) requires an MOU with FEMA.")
			fmt.Fprint(cmd.OutOrStdout(), b.String())
			return nil
		},
	}
	return cmd
}
