// Copyright 2026 David Bryson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// `visited` — track wonders you've seen, in the local SQLite store (hand-authored).
package cli

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelVisitedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "visited",
		Short: "Record which wonders you've seen, with optional date and note.",
		Long: "Track which Atlas Obscura places you've visited. Visited state lives in the local\n" +
			"SQLite store and feeds 'gaps' and 'surprise --exclude-visited'.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newVisitedMarkCmd(flags))
	cmd.AddCommand(newVisitedListCmd(flags))
	return cmd
}

func newVisitedMarkCmd(flags *rootFlags) *cobra.Command {
	var note string
	var date string
	cmd := &cobra.Command{
		Use:     "mark <id-or-slug>",
		Short:   "Mark a place as visited",
		Example: "  atlas-obscura-pp-cli visited mark salvation-mountain --note \"worth the desert drive\"",
		// Writes to the local SQLite store (ao_visited), so it is NOT mcp:read-only.
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would mark a place visited")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a place id or slug is required"))
			}
			if date == "" {
				date = nowDate()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			place, err := aoFetchPlaceShort(cmd.Context(), c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			s, err := aoDB(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			if err := ensureAOTables(s); err != nil {
				return err
			}
			cachePlace(s, place)
			_, err = s.DB().Exec(
				`INSERT OR REPLACE INTO ao_visited (place_id, slug, title, visited_on, note) VALUES (?,?,?,?,?)`,
				place.ID, place.Slug, place.Title, date, note)
			if err != nil {
				return fmt.Errorf("saving visited: %w", err)
			}
			return aoEmit(cmd, flags, map[string]any{
				"visited": place.Title,
				"slug":    place.Slug,
				"id":      place.ID,
				"date":    date,
				"message": fmt.Sprintf("marked %q (%s) visited on %s", place.Title, place.Slug, date),
			})
		},
	}
	cmd.Flags().StringVar(&note, "note", "", "Optional note about the visit")
	cmd.Flags().StringVar(&date, "date", "", "Visit date (YYYY-MM-DD; default today)")
	return cmd
}

func newVisitedListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List places you've marked visited",
		Example:     "  atlas-obscura-pp-cli visited list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list visited places")
				return nil
			}
			s, err := aoDB(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			if err := ensureAOTables(s); err != nil {
				return err
			}
			rows, err := s.DB().Query(`SELECT place_id, slug, title, visited_on, note FROM ao_visited ORDER BY visited_on DESC, title`)
			if err != nil {
				return err
			}
			defer rows.Close()
			type visitRow struct {
				ID        int    `json:"id"`
				Slug      string `json:"slug"`
				Title     string `json:"title"`
				VisitedOn string `json:"visited_on"`
				Note      string `json:"note,omitempty"`
				URL       string `json:"url"`
			}
			visits := make([]visitRow, 0)
			for rows.Next() {
				var v visitRow
				var slug, title, on, note sql.NullString
				if err := rows.Scan(&v.ID, &slug, &title, &on, &note); err != nil {
					continue
				}
				v.Slug, v.Title, v.VisitedOn, v.Note = slug.String, title.String, on.String, note.String
				v.URL = absoluteAOURL("/places/" + v.Slug)
				visits = append(visits, v)
			}
			return aoEmit(cmd, flags, map[string]any{"source": aoSourceNote, "visited": visits, "count": len(visits)})
		},
	}
	return cmd
}

// visitedIDs returns the set of visited place ids for join-based commands.
func visitedIDs(s interface{ DB() *sql.DB }) (map[int]bool, error) {
	rows, err := s.DB().Query(`SELECT place_id FROM ao_visited`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	set := map[int]bool{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		set[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return set, nil
}
