// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/myanimelist/internal/malhtml"
)

type airingRow struct {
	Day       string `json:"day"`
	Date      string `json:"date,omitempty"`
	ID        int    `json:"id"`
	Title     string `json:"title"`
	MediaType string `json:"media_type,omitempty"`
	Members   int    `json:"members,omitempty"`
}

// newNovelAiringCmd reads MyAnimeList's own weekly schedule page and groups it
// by weekday. The site renders this view for Japan; this command keeps the
// day/time text the site already resolved for the caller's timezone parameter.
func newNovelAiringCmd(flags *rootFlags) *cobra.Command {
	var days int
	var tz string
	cmd := &cobra.Command{
		Use:   "airing",
		Short: "List what airs this week across the current season",
		Long: "Use this command for a flat list of everything airing in the coming days.\n" +
			"Do NOT use this command for a timezone-correct grid of only the shows you track; use 'week' instead.",
		Example: "  myanimelist-pp-cli airing --days 7 --json",
		Annotations: map[string]string{
			"mcp:read-only":     "true",
			"pp:data-source":    "live",
			"pp:happy-args":     "--days=1",
			"pp:novel-scaffold": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "airing")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			params := map[string]string{}
			if tz != "" {
				params["tz"] = tz
			}
			body, err := malGet(ctx, flags, "/anime/season/schedule", params, true)
			if err != nil {
				return fmt.Errorf("fetching the weekly schedule: %w", err)
			}
			entries, err := malhtml.ParseSeason(body)
			if err != nil {
				return err
			}
			cutoff := time.Now().AddDate(0, 0, days)
			rows := make([]airingRow, 0, len(entries))
			for _, e := range entries {
				row := airingRow{Day: e.Category, ID: e.ID, Title: e.Title, MediaType: e.MediaType, Members: e.Members}
				if e.StartDate != "" {
					row.Date = e.StartDate
					// The schedule page lists each show once with its next
					// airing date, so keep only dates inside [today, today+days].
					if t, perr := time.Parse("2006-01-02", e.StartDate); perr == nil && days > 0 {
						now := time.Now()
						if t.Before(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)) || t.After(cutoff) {
							continue
						}
					}
				}
				rows = append(rows, row)
			}
			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].Day != rows[j].Day {
					return dayOrder(rows[i].Day) < dayOrder(rows[j].Day)
				}
				return rows[i].Title < rows[j].Title
			})
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Nothing scheduled in that window.")
				return nil
			}
			table := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				table = append(table, map[string]any{"day": r.Day, "date": r.Date, "title": r.Title, "type": r.MediaType})
			}
			return printAutoTable(cmd.OutOrStdout(), table)
		},
	}
	cmd.Flags().IntVar(&days, "days", 7, "Limit to entries starting within this many days (0 for the whole season)")
	cmd.Flags().StringVar(&tz, "tz", "", "IANA timezone for the site to render times in (e.g. America/Phoenix)")
	return cmd
}
