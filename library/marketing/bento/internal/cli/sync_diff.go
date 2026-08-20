// Copyright 2026 bossriceshark and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/bento/internal/store"
	"github.com/spf13/cobra"
)

func newSyncDiffCmd(flags *rootFlags) *cobra.Command {
	var resource string
	var since string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Diff local snapshots to see what changed since a timestamp",
		Long: `Bento's API has no since filter on most resources, so this CLI records
a snapshot every sync. 'sync diff' compares two snapshots and reports
adds, updates, and deletes per resource.

Run 'sync' first to populate snapshots. Then pass --since (e.g. 24h, 7d)
to bound the window.`,
		Example: strings.Trim(`
  # What changed across all resources in the last 24h
  bento-pp-cli sync diff --since 24h

  # Just one resource type
  bento-pp-cli sync diff --since 7d --resource subscribers

  # JSON for piping
  bento-pp-cli sync diff --since 7d --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would diff local snapshots")
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("bento-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			var sinceTime time.Time
			if since != "" {
				sinceTime, err = parseSinceDuration(since)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
				}
			}

			snaps, err := db.ListSnapshots(resource, sinceTime, time.Time{})
			if err != nil {
				return fmt.Errorf("listing snapshots: %w", err)
			}
			if len(snaps) < 2 {
				if handled, herr := emptyLocalStoreOK(cmd, flags, "run 'bento-pp-cli sync' at least twice (snapshots are recorded per sync run) and pass a wider --since"); handled {
					return herr
				}
				msg := "no snapshots in window; run 'bento-pp-cli sync' at least twice (snapshots are recorded per sync run) and pass a wider --since"
				return notFoundErr(fmt.Errorf("%s", msg))
			}

			byResource := map[string][]store.Snapshot{}
			for _, s := range snaps {
				byResource[s.ResourceType] = append(byResource[s.ResourceType], s)
			}

			type diffRow struct {
				Resource string    `json:"resource"`
				From     time.Time `json:"from"`
				To       time.Time `json:"to"`
				FromRows int       `json:"from_rows"`
				ToRows   int       `json:"to_rows"`
				Delta    int       `json:"delta"`
				PctChg   float64   `json:"pct_change"`
			}
			var rows []diffRow
			for r, ss := range byResource {
				if len(ss) < 2 {
					continue
				}
				first, last := ss[0], ss[len(ss)-1]
				pct := 0.0
				if first.RowCount > 0 {
					pct = float64(last.RowCount-first.RowCount) / float64(first.RowCount) * 100
				}
				rows = append(rows, diffRow{
					Resource: r,
					From:     first.TakenAt,
					To:       last.TakenAt,
					FromRows: first.RowCount,
					ToRows:   last.RowCount,
					Delta:    last.RowCount - first.RowCount,
					PctChg:   pct,
				})
			}
			if len(rows) == 0 {
				if handled, herr := emptyLocalStoreOK(cmd, flags, "run 'bento-pp-cli sync' twice with a delay so two snapshots exist in the window"); handled {
					return herr
				}
				return notFoundErr(fmt.Errorf("no resource has at least two snapshots in the window"))
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}

	cmd.Flags().StringVar(&resource, "resource", "", "Limit diff to a single resource (subscribers, tags, fields, broadcasts)")
	cmd.Flags().StringVar(&since, "since", "24h", "Window for snapshot diff (e.g. 24h, 7d, 1w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/bento-pp-cli/data.db)")

	return cmd
}
