// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// Reads the local mirror only. Run `tiimo-pp-cli sync` to refresh it.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// backupResult summarizes a written snapshot.
type backupResult struct {
	Path      string         `json:"path"`
	Bytes     int64          `json:"bytes"`
	Resources map[string]int `json:"resource_counts"`
	Total     int            `json:"total_records"`
	TakenAt   string         `json:"taken_at"`
}

// backupEnvelope is the on-disk snapshot format: a single JSON document with
// every mirrored resource under its own key. Deliberately not the raw SQLite
// file -- a JSON snapshot survives schema changes in this CLI and can be read
// by anything, which is the entire point of owning a backup.
type backupEnvelope struct {
	Tool      string                       `json:"tool"`
	TakenAt   string                       `json:"taken_at"`
	Resources map[string][]json.RawMessage `json:"resources"`
}

// backupResources is the set mirrored by `sync`. Kept explicit rather than
// discovered so a snapshot has a stable, reviewable shape.
var backupResources = []string{
	"activities", "todo_tasks", "todo_lists", "tags", "routines", "calendars", "profiles",
}

func newNovelBackupCmd(flags *rootFlags) *cobra.Command {
	var flagOut, flagDB string

	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Take a full local snapshot of your planner that you can restore from.",
		Long: `Write every mirrored record to a single timestamped JSON file.

Tiimo has no export path, so the local mirror is the only copy of your
planner you actually control. This turns it into a portable file that does
not depend on this CLI's database schema, or on Tiimo continuing to exist.

Run 'sync' first: backup snapshots what has been mirrored, and cannot
retrieve anything the mirror has never seen.`,
		Example: "  tiimo-pp-cli backup --out ~/tiimo-backups",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--out=-",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "backup")
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			st, ok, err := openLocalMirror(ctx, cmd, flags, flagDB)
			if err != nil {
				return err
			}
			if !ok {
				return writeNoMirror(cmd, flags, flagDB, make([]backupResult, 0))
			}
			defer st.Close()

			env := backupEnvelope{
				Tool:      "tiimo-pp-cli",
				TakenAt:   time.Now().UTC().Format(time.RFC3339),
				Resources: map[string][]json.RawMessage{},
			}
			counts := map[string]int{}
			total := 0
			for _, res := range backupResources {
				// List with a generous cap; a personal planner is small, and
				// silently truncating a backup would be the worst possible
				// failure mode for this particular command.
				items, err := st.List(res, 100000)
				if err != nil {
					// Do NOT treat this as an empty section. The earlier
					// reasoning here -- "a resource that was never synced has
					// no rows" -- confused two different things: List reads the
					// generic resources table, which always exists once the
					// store has migrated, so a never-synced resource returns an
					// empty slice and a nil error. A non-nil error is therefore
					// always a real failure (database locked or corrupt, schema
					// drift, cancelled context), and continuing here would write
					// an exit-zero snapshot that silently omits planner data.
					//
					// A backup that quietly loses a resource is worse than no
					// backup, because it is trusted later, when the original is
					// gone.
					return fmt.Errorf("reading %s for snapshot: %w", res, err)
				}
				rows := make([]json.RawMessage, 0, len(items))
				rows = append(rows, items...)
				env.Resources[res] = rows
				counts[res] = len(rows)
				total += len(rows)
			}

			payload, err := json.MarshalIndent(env, "", " ")
			if err != nil {
				return fmt.Errorf("encoding snapshot: %w", err)
			}

			if flagOut == "-" {
				if _, err := cmd.OutOrStdout().Write(payload); err != nil {
					return fmt.Errorf("writing snapshot to stdout: %w", err)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "wrote %d record(s) to stdout\n", total)
				return nil
			}

			dir := flagOut
			if dir == "" {
				dir = "."
			}
			resolvedDir, err := expandHome(dir)
			if err != nil {
				return err
			}
			// 0700 to match the 0600 snapshot below: a backup directory holds
			// nothing but personal planner data. Existing directories keep
			// their own mode; this only constrains ones we create.
			if err := os.MkdirAll(resolvedDir, 0o700); err != nil {
				return fmt.Errorf("creating %s: %w", resolvedDir, err)
			}
			name := fmt.Sprintf("tiimo-backup-%s.json", time.Now().Format("20060102-150405"))
			path := filepath.Join(resolvedDir, name)
			// 0600: a planner snapshot is personal data.
			if err := os.WriteFile(path, payload, 0o600); err != nil {
				return fmt.Errorf("writing %s: %w", path, err)
			}

			res := backupResult{
				Path:      path,
				Bytes:     int64(len(payload)),
				Resources: counts,
				Total:     total,
				TakenAt:   env.TakenAt,
			}
			// Announce an empty snapshot on stderr, not in the text renderer:
			// --json is the default for non-TTY stdout, so a hint that only
			// fires in the human callback is invisible to exactly the callers
			// most likely to script against a silently-empty backup.
			if total == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: the local mirror is empty, so this snapshot has 0 records. Run 'tiimo-pp-cli sync' first.")
			}
			return writeTiimoResult(cmd, flags, []backupResult{res}, func(w io.Writer) {
				fmt.Fprintf(w, "Wrote %d record(s) to %s (%d bytes)\n", total, path, len(payload))
				keys := make([]string, 0, len(counts))
				for k := range counts {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					fmt.Fprintf(w, "  %-12s %d\n", k, counts[k])
				}
			})
		},
	}

	cmd.Flags().StringVar(&flagOut, "out", "", `Directory to write the snapshot into, or "-" for stdout`)
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror (defaults to the standard cache location)")
	return cmd
}
