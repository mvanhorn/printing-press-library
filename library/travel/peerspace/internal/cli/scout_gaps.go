// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: scout gaps — tech-event amenity checklist vs listings/shortlist.

package cli

// pp:data-source local

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/venuex"
	"github.com/spf13/cobra"
)

func newNovelScoutGapsCmd(flags *rootFlags) *cobra.Command {
	var flagChecklist string
	var flagDB string

	cmd := &cobra.Command{
		Use:         "gaps",
		Short:       "Surface missing tech-event must-haves (WiFi, AV/projector, late access, flexible seating, transit) on any shortlist.",
		Example:     "  peerspace-pp-cli scout gaps --checklist tech-meetup --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagChecklist == "" {
				flagChecklist = "tech-meetup"
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			s, err := openNovelStoreRO(ctx, flagDB)
			if err != nil {
				return err
			}
			if s == nil {
				missingDBHint(flagDB)
				return printJSONFiltered(cmd.OutOrStdout(), make([]any, 0), flags)
			}
			defer s.Close()

			listings, err := loadListings(ctx, s)
			if err != nil {
				return err
			}
			favIDs, err := loadFavoriteIDs(ctx, s)
			if err != nil {
				return err
			}
			// Prefer shortlist when available; otherwise scan all listings.
			target := listings
			source := "listings"
			if len(favIDs) > 0 {
				target = venuex.JoinFavorites(favIDs, listings)
				source = "shortlist"
			}

			rows := make([]map[string]any, 0, len(target))
			for _, l := range target {
				gaps := venuex.GapChecklist(l, flagChecklist)
				if len(gaps) == 0 {
					continue
				}
				rows = append(rows, map[string]any{
					"id":           l.ID,
					"title":        l.Title,
					"city":         l.City,
					"gaps":         gaps,
					"gap_count":    len(gaps),
					"price_hourly": l.PriceHourly,
					"guests":       l.Guests,
				})
			}
			result := map[string]any{
				"checklist": flagChecklist,
				"source":    source,
				"scanned":   len(target),
				"rows":      rows,
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "checklist=%s source=%s scanned=%d with_gaps=%d\n", flagChecklist, source, len(target), len(rows))
				for _, r := range rows {
					fmt.Fprintf(cmd.OutOrStdout(), "  %v  %v  gaps=%v\n", r["id"], r["title"], r["gaps"])
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagChecklist, "checklist", "tech-meetup", "Checklist name (tech-meetup) or comma-separated gap keys")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database path")
	return cmd
}
