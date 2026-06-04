// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source local
//
// `top` — ranked flat list of best clips by a numeric field. Reads the local
// SQLite store only; no network and no auth. Read-only.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/store"
	"github.com/spf13/cobra"
)

// topByFields maps the --by value to its clips column. All are numeric and
// NULL-safe via COALESCE in the query.
var topByFields = map[string]bool{
	"play_count":   true,
	"upvote_count": true,
	"duration":     true,
}

func newSunoTopCmd(flags *rootFlags) *cobra.Command {
	var (
		by     string
		limit  int
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "top",
		Short: "Ranked flat list of best local clips by a numeric field",
		Long: "Ranked flat list of best clips. Orders your synced clips by play_count, " +
			"upvote_count, or duration (descending) and returns the top N.\n\n" +
			"Do NOT use for grouped aggregates; use 'analytics'.",
		Example:     "  suno-pp-cli top --by upvote_count --limit 5\n  suno-pp-cli top --by play_count --json",
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if !topByFields[by] {
				return usageErr(fmt.Errorf("invalid --by %q: must be one of play_count, upvote_count, duration", by))
			}
			if limit <= 0 {
				limit = 10
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'suno-pp-cli sync' first.", err)
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "clips")
			hintIfStale(cmd, db, "clips", flags.maxAge)

			results, err := topClips(db, by, limit)
			if err != nil {
				return fmt.Errorf("ranking local clips: %w", err)
			}
			attachWorkspaceColumn(db, topClipIDs(results), func(i int, label string) {
				results[i].Workspace = label
			}, len(results))
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&by, "by", "play_count", "Numeric field to rank by: play_count, upvote_count, duration")
	cmd.Flags().IntVar(&limit, "limit", 10, "Number of clips to return")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("suno-pp-cli"), "Path to local SQLite store")
	return cmd
}

// topClip is one ranked row.
type topClip struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Tags        string  `json:"tags"`
	ModelName   string  `json:"model_name"`
	PlayCount   int64   `json:"play_count"`
	UpvoteCount int64   `json:"upvote_count"`
	Duration    float64 `json:"duration"`
	Workspace   string  `json:"workspace,omitempty"`
}

// topClipIDs returns the clip ids of a ranked result set, preserving order.
func topClipIDs(rows []topClip) []string {
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	return ids
}

// topClips returns the top N clips ordered by the chosen numeric field desc.
// by is validated against topByFields before reaching here, so interpolating
// it into ORDER BY is safe (it can only ever be a known column literal).
func topClips(db *store.Store, by string, limit int) ([]topClip, error) {
	out := make([]topClip, 0)
	query := fmt.Sprintf(
		`SELECT id,
		        COALESCE(title, ''),
		        COALESCE(tags, ''),
		        COALESCE(model_name, ''),
		        COALESCE(play_count, 0),
		        COALESCE(upvote_count, 0),
		        COALESCE(duration, 0)
		   FROM clips
		  ORDER BY COALESCE(%s, 0) DESC
		  LIMIT ?`, by)
	rows, err := db.DB().Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var c topClip
		var title, tags, model sql.NullString
		if err := rows.Scan(&c.ID, &title, &tags, &model, &c.PlayCount, &c.UpvoteCount, &c.Duration); err != nil {
			return nil, err
		}
		c.Title = title.String
		c.Tags = tags.String
		c.ModelName = model.String
		out = append(out, c)
	}
	return out, rows.Err()
}
