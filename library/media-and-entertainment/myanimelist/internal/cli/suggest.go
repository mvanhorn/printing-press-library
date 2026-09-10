// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/myanimelist/internal/malhtml"
)

type suggestRow struct {
	ID        int      `json:"id"`
	Title     string   `json:"title"`
	Score     float64  `json:"score,omitempty"`
	Members   int      `json:"members,omitempty"`
	MediaType string   `json:"media_type,omitempty"`
	StartDate string   `json:"start_date,omitempty"`
	Genres    []string `json:"genres,omitempty"`
}

type suggestView struct {
	Candidates []suggestRow `json:"candidates"`
	Season     string       `json:"season,omitempty"`
	Excluded   int          `json:"excluded_library_entries"`
	Note       string       `json:"note,omitempty"`
}

// newNovelSuggestCmd proposes titles from the current season chart that your
// local library has not already recorded. The eligibility filter is local; the
// candidate pool and the quality signal are public MyAnimeList data.
func newNovelSuggestCmd(flags *rootFlags) *cobra.Command {
	var dbPath, seasonPath string
	var limit, minScore int
	cmd := &cobra.Command{
		Use:   "suggest",
		Short: "Suggest season titles you have not started yet",
		Long: "Use this command for a shortlist of what to start next, drawn from the current season and filtered against your local library.\n" +
			"Do NOT use this command to search for a specific title; use 'instant' or 'anime list' instead.",
		Example: "  myanimelist-pp-cli suggest --min-score 8 --limit 5 --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "auto",
			"pp:happy-args":       "--min-score=0;--limit=1",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "suggest")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			path := seasonPath
			seasonLabel := "current season"
			if path == "" {
				path = "/anime/season"
			}
			body, err := malGet(ctx, flags, path, nil, true)
			if err != nil {
				return fmt.Errorf("fetching the season chart: %w", err)
			}
			entries, err := malhtml.ParseSeason(body)
			if err != nil {
				return err
			}
			excluded := 0
			tracked := map[string]bool{}
			dbPath = malDBPath(flags, dbPath)
			if malStoreExists(dbPath) {
				db, derr := malOpenStore(ctx, dbPath)
				if derr == nil {
					ids, ierr := malLibraryIDs(ctx, db)
					_ = db.Close()
					if ierr == nil {
						tracked = ids
					}
				}
			}
			rows := make([]suggestRow, 0, len(entries))
			for _, e := range entries {
				if tracked[fmt.Sprintf("anime:%d", e.ID)] {
					excluded++
					continue
				}
				if minScore > 0 && e.Score < float64(minScore) {
					continue
				}
				rows = append(rows, suggestRow{ID: e.ID, Title: e.Title, Score: e.Score, Members: e.Members, MediaType: e.MediaType, StartDate: e.StartDate, Genres: e.Genres})
			}
			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].Score != rows[j].Score {
					return rows[i].Score > rows[j].Score
				}
				return rows[i].Members > rows[j].Members
			})
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			view := suggestView{Candidates: rows, Season: seasonLabel, Excluded: excluded}
			if len(rows) == 0 {
				view.Note = "every candidate in the current season is either already in your local library or below --min-score"
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			table := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				table = append(table, map[string]any{"title": r.Title, "score": r.Score, "type": r.MediaType, "start": r.StartDate})
			}
			return printAutoTable(cmd.OutOrStdout(), table)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (used to exclude what you already track)")
	cmd.Flags().StringVar(&seasonPath, "season-path", "", "Override the season page path (default: the current season)")
	cmd.Flags().IntVar(&limit, "limit", 10, "How many suggestions to return")
	cmd.Flags().IntVar(&minScore, "min-score", 0, "Minimum MyAnimeList score (0 disables the filter)")
	return cmd
}
