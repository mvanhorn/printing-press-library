// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built transcendence command for clickup-2-pp-cli.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/project-management/clickup-2/internal/store"
	"github.com/spf13/cobra"
)

func newNovelChangedSinceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var noSave bool

	cmd := &cobra.Command{
		Use:   "changed-since [last|<duration>]",
		Short: "See exactly what moved on your tasks: status changes, new/removed assignees, and due-date shifts",
		Long: `Diffs the current synced task state against the last snapshot stored
locally and reports what changed per task: status transitions, assignee
changes, and due-date shifts. After reporting, it rewrites the snapshot so
the next run reports only newer changes.

The optional argument scopes which tasks to consider by their date_updated:
  last        (default) every task updated since the previous snapshot
  24h, 7d, 2w only tasks whose date_updated is within that window

The first run has no prior snapshot, so it records a baseline and reports
zero changes. Use --no-save to report without advancing the baseline.

This reads (and writes the snapshot to) the local store only; no network.`,
		Example: `  # What changed since the last run
  clickup-2-pp-cli changed-since

  # Changes among tasks touched in the last 2 days, JSON, don't move baseline
  clickup-2-pp-cli changed-since 2d --no-save --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate the scope argument up front, before any store access or
			// dry-run/empty-store short-circuit, so a bad arg is always rejected.
			scope := "last"
			if len(args) > 0 {
				scope = strings.ToLower(strings.TrimSpace(args[0]))
			}
			var sinceMS int64
			if scope != "last" && scope != "" {
				d, err := parseDurationWindow(scope)
				if err != nil {
					return fmt.Errorf("invalid scope %q: %v (use 'last' or a window like 24h, 7d)", scope, err)
				}
				sinceMS = time.Now().Add(-d).UnixMilli()
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("clickup-2-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'clickup-2-pp-cli sync' first.", err)
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "task", flags.maxAge)

			tasks, err := loadPMTasks(db)
			if err != nil {
				return err
			}
			if len(tasks) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: "+noTasksHint)
				if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"baseline": true, "changes": []any{}}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No tasks in the local store yet.")
				return nil
			}

			prior, err := readSnapshot(cmd.Context(), db)
			if err != nil {
				return err
			}
			baseline := len(prior) == 0

			type change struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				Kind      string `json:"change"` // new | status | assignees | due
				From      string `json:"from,omitempty"`
				To        string `json:"to,omitempty"`
				UpdatedAt string `json:"date_updated,omitempty"`
			}
			changes := []change{}
			for _, t := range tasks {
				if sinceMS != 0 && t.DateUpdated < sinceMS {
					continue
				}
				cur := fingerprintOf(t)
				old, seen := prior[t.ID]
				if !seen {
					if !baseline {
						changes = append(changes, change{ID: t.ID, Name: t.Name, Kind: "new", To: t.Status, UpdatedAt: msToString(t.DateUpdated)})
					}
					continue
				}
				if old.status != cur.status {
					changes = append(changes, change{ID: t.ID, Name: t.Name, Kind: "status", From: old.status, To: cur.status, UpdatedAt: msToString(t.DateUpdated)})
				}
				if old.assignees != cur.assignees {
					changes = append(changes, change{ID: t.ID, Name: t.Name, Kind: "assignees", From: old.assignees, To: cur.assignees, UpdatedAt: msToString(t.DateUpdated)})
				}
				if old.due != cur.due {
					changes = append(changes, change{ID: t.ID, Name: t.Name, Kind: "due", From: msToString(old.due), To: msToString(cur.due), UpdatedAt: msToString(t.DateUpdated)})
				}
			}
			sort.SliceStable(changes, func(i, j int) bool { return changes[i].UpdatedAt > changes[j].UpdatedAt })

			if !noSave {
				if err := writeSnapshot(cmd.Context(), db, tasks); err != nil {
					return fmt.Errorf("updating snapshot: %w", err)
				}
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				payload := map[string]any{"baseline": baseline, "changes": changes}
				return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
			}
			w := cmd.OutOrStdout()
			if baseline {
				fmt.Fprintf(w, "Baseline recorded for %d tasks. Re-run after your next sync to see what changed.\n", len(tasks))
				return nil
			}
			if len(changes) == 0 {
				fmt.Fprintln(w, "No task changes since the last snapshot.")
				return nil
			}
			fmt.Fprintf(w, "%-12s %-10s %s\n", "ID", "CHANGE", "DETAIL")
			for _, c := range changes {
				detail := c.Name
				switch c.Kind {
				case "status", "due":
					detail = fmt.Sprintf("%s: %s -> %s", c.Name, dash(c.From), dash(c.To))
				case "assignees":
					detail = fmt.Sprintf("%s: assignees changed", c.Name)
				case "new":
					detail = fmt.Sprintf("%s (new in %s)", c.Name, dash(c.To))
				}
				fmt.Fprintf(w, "%-12s %-10s %s\n", c.ID, c.Kind, detail)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&noSave, "no-save", false, "Report changes without advancing the snapshot baseline")
	return cmd
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

type pmFingerprint struct {
	status    string
	assignees string
	due       int64
	updated   int64
}

func fingerprintOf(t pmTask) pmFingerprint {
	ids := make([]string, 0, len(t.Assignees))
	for _, a := range t.Assignees {
		ids = append(ids, strconv.FormatInt(a.ID, 10))
	}
	sort.Strings(ids)
	return pmFingerprint{
		status:    t.Status,
		assignees: strings.Join(ids, ","),
		due:       t.DueDate,
		updated:   t.DateUpdated,
	}
}

func readSnapshot(ctx context.Context, db *store.Store) (map[string]pmFingerprint, error) {
	out := map[string]pmFingerprint{}
	rows, err := db.Query(`SELECT task_id, status, assignee_ids, due_date FROM pm_snapshot`)
	if err != nil {
		return nil, fmt.Errorf("reading snapshot: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, status, assignees string
		var due int64
		if err := rows.Scan(&id, &status, &assignees, &due); err != nil {
			continue
		}
		out[id] = pmFingerprint{status: status, assignees: assignees, due: due}
	}
	return out, rows.Err()
}

func writeSnapshot(ctx context.Context, db *store.Store, tasks []pmTask) error {
	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM pm_snapshot`); err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO pm_snapshot(task_id, status, assignee_ids, due_date, date_updated, snapshot_at) VALUES (?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	now := time.Now().UnixMilli()
	for _, t := range tasks {
		fp := fingerprintOf(t)
		if _, err := stmt.ExecContext(ctx, t.ID, fp.status, fp.assignees, fp.due, fp.updated, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}
