// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// Stub for GET /recommendations. Deprecated for new apps per the
// 2024-11-27 Spotify Web API change. See deprecated_stubs.go.

package cli

import (
	"github.com/spf13/cobra"
)

func newRecommendationsGetCmd(flags *rootFlags) *cobra.Command {
	// Preserve the full seed/target flag surface so `--legacy-app` callers
	// on grandfathered apps still get the documented interface. Values
	// captured here are dropped — the stub never reaches the API path.
	var (
		flagLimit              int
		flagMarket             string
		flagSeedArtists        string
		flagSeedGenres         string
		flagSeedTracks         string
		flagMinAcousticness    float64
		flagMaxAcousticness    float64
		flagTargetAcousticness float64
		flagMinDanceability    float64
		flagMaxDanceability    float64
		flagTargetDanceability float64
		flagMinDurationMs      int
		flagMaxDurationMs      int
		flagTargetDurationMs   int
		flagMinEnergy          float64
		flagMaxEnergy          float64
		flagTargetEnergy       float64
		flagMinPopularity      int
		flagMaxPopularity      int
		flagTargetPopularity   int
		flagMinValence         float64
		flagMaxValence         float64
		flagTargetValence      float64
		legacyApp              bool
	)

	cmd := &cobra.Command{
		Use:         "get",
		Aliases:     []string{"list"},
		Short:       "Get track recommendations (DEPRECATED — stub by default for new apps; --legacy-app attempts the real call)",
		Example:     "  spotify-pp-cli recommendations get --seed-artists 4NHQUGzhtTLFvgF5SZesLK",
		Annotations: map[string]string{"pp:endpoint": "recommendations.get", "pp:method": "GET", "pp:path": "/recommendations", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := deprecatedStubPayload("GET /recommendations",
				"Spotify's recommendations endpoint is unavailable to new apps. Use 'spotify-pp-cli discover artists' or 'spotify-pp-cli discover new-releases' instead — these use search + genres which still work post-deprecation. Or retry with --legacy-app on a grandfathered app.")
			if legacyApp {
				payload["next_action"] = "live call (legacy-app mode) — endpoint not retried via stub; call /v1/recommendations directly with your client"
			}
			return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Number of recommendations (1-100)")
	cmd.Flags().StringVar(&flagMarket, "market", "", "ISO 3166-1 alpha-2 country code")
	cmd.Flags().StringVar(&flagSeedArtists, "seed-artists", "", "Comma-separated artist IDs (max 5 total seeds)")
	cmd.Flags().StringVar(&flagSeedGenres, "seed-genres", "", "Comma-separated genre seeds")
	cmd.Flags().StringVar(&flagSeedTracks, "seed-tracks", "", "Comma-separated track IDs")
	cmd.Flags().Float64Var(&flagMinAcousticness, "min-acousticness", 0, "")
	cmd.Flags().Float64Var(&flagMaxAcousticness, "max-acousticness", 0, "")
	cmd.Flags().Float64Var(&flagTargetAcousticness, "target-acousticness", 0, "")
	cmd.Flags().Float64Var(&flagMinDanceability, "min-danceability", 0, "")
	cmd.Flags().Float64Var(&flagMaxDanceability, "max-danceability", 0, "")
	cmd.Flags().Float64Var(&flagTargetDanceability, "target-danceability", 0, "")
	cmd.Flags().IntVar(&flagMinDurationMs, "min-duration-ms", 0, "")
	cmd.Flags().IntVar(&flagMaxDurationMs, "max-duration-ms", 0, "")
	cmd.Flags().IntVar(&flagTargetDurationMs, "target-duration-ms", 0, "")
	cmd.Flags().Float64Var(&flagMinEnergy, "min-energy", 0, "")
	cmd.Flags().Float64Var(&flagMaxEnergy, "max-energy", 0, "")
	cmd.Flags().Float64Var(&flagTargetEnergy, "target-energy", 0, "")
	cmd.Flags().IntVar(&flagMinPopularity, "min-popularity", 0, "")
	cmd.Flags().IntVar(&flagMaxPopularity, "max-popularity", 0, "")
	cmd.Flags().IntVar(&flagTargetPopularity, "target-popularity", 0, "")
	cmd.Flags().Float64Var(&flagMinValence, "min-valence", 0, "")
	cmd.Flags().Float64Var(&flagMaxValence, "max-valence", 0, "")
	cmd.Flags().Float64Var(&flagTargetValence, "target-valence", 0, "")
	cmd.Flags().BoolVar(&legacyApp, "legacy-app", false, "Document a real call (only works for apps grandfathered before 2024-11-27)")
	return cmd
}
