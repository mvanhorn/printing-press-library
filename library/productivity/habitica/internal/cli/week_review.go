// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/habitica/internal/store"
	"github.com/spf13/cobra"
)

type habiticaWeekSnapshot struct {
	CapturedAt string `json:"captured_at"`
	Open       int    `json:"open"`
	Completed  int    `json:"completed"`
	Overdue    int    `json:"overdue"`
}

func readHabiticaWeekSnapshot(ctx context.Context, db *sql.DB, now time.Time) (habiticaWeekSnapshot, error) {
	snapshot := habiticaWeekSnapshot{CapturedAt: now.UTC().Format(time.RFC3339)}
	cutoff := now.UTC().Format(time.RFC3339)
	err := db.QueryRowContext(ctx, `SELECT
		COALESCE(SUM(CASE WHEN COALESCE(completed, 0) = 0 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN COALESCE(completed, 0) = 1 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN COALESCE(completed, 0) = 0 AND date != '' AND date < ? THEN 1 ELSE 0 END), 0)
		FROM tasks`, cutoff).Scan(&snapshot.Open, &snapshot.Completed, &snapshot.Overdue)
	return snapshot, err
}

func newNovelWeekReviewCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "review",
		Short:       "Review seven-day overdue, stalled, and completed-task trends from real local snapshots.",
		Example:     "  habitica-pp-cli week review --agent --select overdue_delta,completed_delta",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{"snapshots": []any{}, "action": "would compare local task snapshots from the last seven days"})
			}
			if dbPath == "" {
				dbPath = defaultDBPath("habitica-pp-cli")
			}
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: habitica-pp-cli sync --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if _, err := db.DB().ExecContext(ctx, `CREATE TABLE IF NOT EXISTS habitica_week_snapshots (
				captured_at TEXT PRIMARY KEY, open_count INTEGER NOT NULL, completed_count INTEGER NOT NULL, overdue_count INTEGER NOT NULL
			)`); err != nil {
				return fmt.Errorf("creating snapshot table: %w", err)
			}
			snapshot, err := readHabiticaWeekSnapshot(ctx, db.DB(), time.Now())
			if err != nil {
				return fmt.Errorf("reading local task mirror: %w", err)
			}
			if _, err := db.DB().ExecContext(ctx, `INSERT OR REPLACE INTO habitica_week_snapshots (captured_at, open_count, completed_count, overdue_count) VALUES (?, ?, ?, ?)`, snapshot.CapturedAt, snapshot.Open, snapshot.Completed, snapshot.Overdue); err != nil {
				return fmt.Errorf("recording local snapshot: %w", err)
			}
			rows, err := db.DB().QueryContext(ctx, `SELECT captured_at, open_count, completed_count, overdue_count FROM habitica_week_snapshots WHERE captured_at >= ? ORDER BY captured_at`, time.Now().AddDate(0, 0, -7).UTC().Format(time.RFC3339))
			if err != nil {
				return fmt.Errorf("reading weekly snapshots: %w", err)
			}
			defer rows.Close()
			snapshots := []habiticaWeekSnapshot{}
			for rows.Next() {
				var item habiticaWeekSnapshot
				if err := rows.Scan(&item.CapturedAt, &item.Open, &item.Completed, &item.Overdue); err != nil {
					return err
				}
				snapshots = append(snapshots, item)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			result := map[string]any{"snapshots": snapshots, "current": snapshot}
			if len(snapshots) > 1 {
				first := snapshots[0]
				result["open_delta"] = snapshot.Open - first.Open
				result["completed_delta"] = snapshot.Completed - first.Completed
				result["overdue_delta"] = snapshot.Overdue - first.Overdue
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror database path (default: resolved data directory data.db)")
	return cmd
}
