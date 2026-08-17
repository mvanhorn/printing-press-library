// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: projects scout — project metadata + fitting listings.

package cli

// pp:data-source local

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/venuex"
	"github.com/spf13/cobra"
)

func newNovelProjectsScoutCmd(flags *rootFlags) *cobra.Command {
	var flagProjectID string
	var flagDB string

	cmd := &cobra.Command{
		Use:         "scout",
		Short:       "Combine project location metadata with fitting listings from the local store.",
		Example:     "  peerspace-pp-cli projects scout --project-id demo-project --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			s, err := openNovelStoreRO(ctx, flagDB)
			if err != nil {
				return err
			}
			if s == nil {
				missingDBHint(flagDB)
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"projects": make([]any, 0),
					"listings": make([]any, 0),
				}, flags)
			}
			defer s.Close()

			projects, err := loadProjects(ctx, s)
			if err != nil {
				return err
			}
			if flagProjectID != "" {
				filtered := make([]map[string]any, 0)
				for _, p := range projects {
					if fmt.Sprint(p["id"]) == flagProjectID {
						filtered = append(filtered, p)
					}
				}
				projects = filtered
			}

			listings, err := loadListings(ctx, s)
			if err != nil {
				return err
			}
			// Derive city hints from project data when present.
			cityHints := make([]string, 0)
			for _, p := range projects {
				if data, ok := p["data"].(map[string]any); ok {
					for _, key := range []string{"city", "location", "viewport_location", "place"} {
						if v, ok := data[key].(string); ok && strings.TrimSpace(v) != "" {
							cityHints = append(cityHints, v)
						}
					}
				}
			}

			fitting := listings
			if len(cityHints) > 0 {
				fitting = make([]venuex.Listing, 0)
				for _, l := range listings {
					for _, c := range cityHints {
						if venuex.MatchCity(l, c) {
							fitting = append(fitting, l)
							break
						}
					}
				}
			}
			// Cap and score lightly for tech events.
			ranked := venuex.RankListings(fitting, 0, 0, nil, 25)
			listingRows := make([]map[string]any, 0, len(ranked))
			for _, r := range ranked {
				row := listingToRow(r.Listing)
				row["score"] = r.Score
				listingRows = append(listingRows, row)
			}

			result := map[string]any{
				"projects":   projects,
				"listings":   listingRows,
				"city_hints": cityHints,
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "projects scout projects=%d listings=%d\n", len(projects), len(listingRows))
				for _, p := range projects {
					fmt.Fprintf(cmd.OutOrStdout(), "  project %v\n", p["id"])
				}
				for _, r := range listingRows {
					fmt.Fprintf(cmd.OutOrStdout(), "  listing %v score=%v %v\n", r["id"], r["score"], r["title"])
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagProjectID, "project-id", "", "Optional project id filter")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database path")
	return cmd
}
