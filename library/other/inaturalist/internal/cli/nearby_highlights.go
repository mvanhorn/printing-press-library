// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// pp:data-source live
// pp:client-call
// runNearbyHighlights reaches the generated client through the shared
// privacy-redacting novelGet helper.
func newNovelNearbyHighlightsCmd(flags *rootFlags) *cobra.Command {
	var lat, lng, radius string
	cmd := &cobra.Command{
		Use:         "highlights",
		Short:       "Get a transparent recent wildlife briefing without observation coordinates.",
		Example:     "  inaturalist-pp-cli nearby highlights --lat 37.7749 --lng -122.4194 --radius 5 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runNearbyHighlights(cmd, flags, lat, lng, radius)
		},
	}
	cmd.Flags().StringVar(&lat, "lat", "", "Center latitude for the supplied search area")
	cmd.Flags().StringVar(&lng, "lng", "", "Center longitude for the supplied search area")
	cmd.Flags().StringVar(&radius, "radius", "10", "Search radius in kilometers (maximum 200)")
	return cmd
}
