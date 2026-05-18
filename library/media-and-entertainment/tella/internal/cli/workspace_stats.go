// Copyright 2026 gregce. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/store"

	"github.com/spf13/cobra"
)

// newWorkspaceCmd is the parent for workspace-wide aggregations against the
// local SQLite store.
func newWorkspaceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "workspace",
		Short:       "Workspace-wide aggregations against the local store",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        rejectUnknownSubcommand,
	}
	cmd.AddCommand(newWorkspaceStatsCmd(flags))
	return cmd
}

func newWorkspaceStatsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "stats",
		Short:       "Aggregate counts and totals across the cached workspace",
		Example:     "  tella-pp-cli workspace stats --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("tella-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			counts := map[string]int{}
			for _, rt := range []string{"videos", "clips", "exports", "webhooks", "playlists"} {
				n, _ := db.Count(rt)
				counts[rt] = n
			}
			tCount, _ := db.CountTranscripts()
			counts["transcripts"] = tCount
			words, _ := db.SumTranscriptWords()
			views, viewsErr := totalVideoViews(db)
			lastSync, _ := db.LatestSyncedAt()
			totals := map[string]any{
				"video_views":           views,
				"transcript_word_count": words,
			}
			// Surface a partial-sum signal so consumers can distinguish a
			// legitimate zero from a truncated scan caused by a row-level
			// or iteration error. Keep the numeric `video_views` field
			// for backward compatibility; agents reading the envelope
			// can branch on the optional `video_views_error` field.
			if viewsErr != nil {
				totals["video_views_error"] = viewsErr.Error()
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: video_views may be a partial sum: %v\n", viewsErr)
			}
			out := map[string]any{
				"counts":    counts,
				"totals":    totals,
				"last_sync": formatTimeRFC3339(lastSync),
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database")
	return cmd
}

// totalVideoViews sums the `views` field across every cached video record.
// Returns (sum, error) so callers can distinguish a partial sum caused by a
// mid-iteration database error from a legitimate zero. Without the
// rows.Err() check, a transient SQLite read failure halfway through the
// scan would silently truncate the total — and the workspace stats
// envelope would report a confidently-wrong number.
func totalVideoViews(db *store.Store) (int, error) {
	rows, err := db.Query(`SELECT data FROM resources WHERE resource_type = 'videos'`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	total := 0
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return total, fmt.Errorf("scanning video row: %w", err)
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(raw), &obj); err != nil {
			// Best-effort: an unparseable row doesn't poison the
			// scan, but the row-level error stays local rather than
			// bubbling out (the row likely predates a schema migration).
			continue
		}
		if v, ok := obj["views"].(float64); ok {
			total += int(v)
		}
	}
	if err := rows.Err(); err != nil {
		return total, fmt.Errorf("iterating video rows: %w", err)
	}
	return total, nil
}
