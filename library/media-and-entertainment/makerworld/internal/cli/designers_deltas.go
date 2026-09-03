// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: roll up per-designer changes (new uploads and
// engagement deltas) between the two most recent synced snapshots.

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/makerworld/internal/store"

	"github.com/spf13/cobra"
)

// pp:data-source local

type designerDelta struct {
	CreatorID     string   `json:"creator_id"`
	Creator       string   `json:"creator"`
	NewDesigns    int      `json:"new_designs"`
	NewDesignIDs  []string `json:"new_design_ids,omitempty"`
	LikeDelta     int      `json:"like_delta"`
	DownloadDelta int      `json:"download_delta"`
}

func newNovelDesignersDeltasCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "deltas",
		Short: "See which designers posted new models or gained engagement since your last sync",
		Long: "Rolls up, per designer, the new designs and the like/download gains between " +
			"the two most recent synced snapshots of your local mirror. Needs at least two " +
			"distinct syncs (run 'movers' or 'designers deltas' after each sync to record one).\n\n" +
			"Use this for the all-designers roll-up of activity. For one designer's current " +
			"catalog use 'designers models --creator-id <uid>'.",
		Example: strings.Trim(`
  makerworld-pp-cli designers deltas --limit 30 --agent
  makerworld-pp-cli designers deltas --limit 10`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would roll up per-designer changes between the two most recent snapshots")
				return nil
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("designers deltas compares local snapshots and has no live mode; run 'sync' then re-run"))
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
				// deltas is a between-syncs feature, so snapshotting against a
				// fabricated timestamp would grow the table unbounded and yield
				// always-zero deltas. Require a real sync instead.
				fmt.Fprintln(cmd.ErrOrStderr(), "designers deltas needs sync-based snapshots; run 'sync' first (imported data has no sync timeline to diff against)")
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
				fmt.Fprintln(cmd.ErrOrStderr(), "only one synced snapshot recorded so far; re-sync later and re-run to compute deltas")
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

			deltas := aggregateDesignerDeltas(curSnap, prevSnap, flagLimit)

			if len(deltas) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "no designer activity between the two most recent snapshots")
			}
			if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), deltas, flags)
			}
			items := make([]map[string]any, 0, len(deltas))
			for _, d := range deltas {
				items = append(items, map[string]any{
					"creator": d.Creator, "new_designs": d.NewDesigns,
					"like_delta": d.LikeDelta, "download_delta": d.DownloadDelta,
				})
			}
			if len(items) > 0 {
				return printAutoTable(cmd.OutOrStdout(), items)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum designers to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
