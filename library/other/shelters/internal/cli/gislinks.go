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
		Short:       "Authoritative FEMA service URLs and the full-NSS access path (link-out only)",
		Long:        "Print the stable FEMA OpenShelters layer URL this CLI reads, plus the broader National Shelter System program page (full access requires an MOU) and the free geocoder used by 'near'. Link-out only; never ingested.",
		Example:     "  shelters-pp-cli gis-links",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			links := map[string]any{
				"openshelters_feature_layer": featureServerURL,
				"openshelters_query":         openSheltersBase + openSheltersQuery,
				"national_shelter_system":    fullNSSInfoURL,
				"census_geocoder":            censusGeocoderBase,
				"note":                       "This CLI reads the OpenShelters attributes feed; the GIS layers are referenced, not ingested. Full NSS (beyond public OpenShelters) requires an MOU with FEMA.",
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
