// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: rank designs by the largest positive delta in a
// chosen metric between the two most recent synced snapshots.

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/makerworld/internal/store"

	"github.com/spf13/cobra"
)

// pp:data-source local

type moverItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Creator  string `json:"creator"`
	Metric   string `json:"metric"`
	Current  int    `json:"current"`
	Previous int    `json:"previous"`
	Delta    int    `json:"delta"`
	URL      string `json:"url"`
}

func newNovelMoversCmd(flags *rootFlags) *cobra.Command {
	var flagMetric string
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "movers",
		Short: "Rank models by the biggest jump in a metric between your two most recent syncs",
		Long: "Compares the two most recent synced snapshots of your local mirror and ranks " +
			"designs by the largest positive change in a metric. Records a snapshot on each " +
			"run keyed by sync time, so it needs at least two distinct syncs to report deltas.\n\n" +
			"Use this for what is rising between syncs. For absolute current popularity use " +
			"'designs list --nav Trending'; for the quality blend use 'discover'.",
		Example: strings.Trim(`
  makerworld-pp-cli movers --metric downloads --limit 25 --agent
  makerworld-pp-cli movers --metric likes --limit 10`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare the two most recent synced snapshots")
				return nil
			}
			metric := strings.ToLower(strings.TrimSpace(flagMetric))
			if metric == "" {
				metric = "downloads"
			}
			switch metric {
			case "downloads", "likes", "prints", "collections":
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--metric must be one of: downloads, likes, prints, collections"))
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("movers compares local snapshots and has no live mode; run 'sync' then 'movers'"))
			}
			if flagLimit < 0 {
				return usageErr(fmt.Errorf("--limit must be >= 0"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("makerworld-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: makerworld-pp-cli sync --resources designs --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			rows, err := loadDesignRows(ctx, db.DB())
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "local mirror is empty; run: makerworld-pp-cli sync --resources designs")
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			syncAt := db.GetLastSyncedAt("designs")
			if syncAt == "" {
				// No sync timestamp (e.g. data loaded via `import`, not `sync`).
				// movers is a between-syncs feature, so snapshotting against a
				// fabricated timestamp would grow the table unbounded and yield
				// always-zero deltas. Require a real sync instead.
				fmt.Fprintln(cmd.ErrOrStderr(), "movers needs sync-based snapshots; run 'sync' first (imported data has no sync timeline to diff against)")
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			if err := db.RecordDesignSnapshots(ctx, syncAt, toSnapshotRows(rows)); err != nil {
				return err
			}

			current, previous, err := latestTwoSnapshots(ctx, db.DB())
			if err != nil {
				return err
			}
			if previous == "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "only one synced snapshot recorded so far; re-sync later and re-run to compute movers")
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			curSnap, err := loadSnapshot(ctx, db.DB(), current)
			if err != nil {
				return err
			}
			prevSnap, err := loadSnapshot(ctx, db.DB(), previous)
			if err != nil {
				return err
			}

			movers := computeMovers(curSnap, prevSnap, metric, flagLimit)

			if len(movers) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no positive %s movement between the two most recent snapshots\n", metric)
			}
			if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), movers, flags)
			}
			items := make([]map[string]any, 0, len(movers))
			for _, m := range movers {
				items = append(items, map[string]any{
					"id": m.ID, "title": m.Title, "creator": m.Creator,
					"delta": m.Delta, "current": m.Current,
				})
			}
			if len(items) > 0 {
				return printAutoTable(cmd.OutOrStdout(), items)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagMetric, "metric", "downloads", "Metric to rank by: downloads, likes, prints, collections")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum movers to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
