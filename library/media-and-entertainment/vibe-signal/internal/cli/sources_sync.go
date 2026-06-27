// Copyright 2026 not0xjarvis and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: sync sources into the local store for a topic (hand-authored).

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/vibe-signal/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/vibe-signal/internal/source"
)

type sourcesSyncResult struct {
	Query    string          `json:"query"`
	RunID    string          `json:"run_id"`
	Window   string          `json:"window"`
	Coverage []coverageEntry `json:"coverage"`
	Stored   int             `json:"stored"`
}

func newSourcesSyncCmd(flags *rootFlags) *cobra.Command {
	var flagQuery string
	var flagSource string
	var flagWindow string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync sources into the local store for a topic (populates 'evidence')",
		Long: strings.Trim(`
Fetch the topic across the wired sources and write a snapshot to the local store
without rendering a report. Use this to pre-populate the store so 'evidence' can
list cited items. 'report' performs the same sync-and-store as a side effect.`, "\n"),
		Example: strings.Trim(`
  vibe-signal-pp-cli sources sync --query "AI browser agents" --window 30d
  vibe-signal-pp-cli sources sync --query "Postgres" --source hackernews`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would sync sources into the local store")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if strings.TrimSpace(flagQuery) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--query is required"))
			}
			since, windowDays, err := parseWindow(flagWindow)
			if err != nil {
				return err
			}
			limit := flagLimit
			if limit <= 0 {
				limit = 20
			}
			if cliutil.IsDogfoodEnv() && limit > 5 {
				limit = 5
			}
			sources, err := selectedSources(flagSource)
			if err != nil {
				return err
			}

			signals, coverage := syncSources(ctx, sources, source.SyncOptions{
				Query: flagQuery, Since: since, Limit: limit,
			})

			runID := newRunID(flagQuery)
			dbPath := defaultDBPath("vibe-signal-pp-cli")
			db, err := openSignalStore(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()
			coverageJSON, _ := json.Marshal(coverage)
			if err := db.RecordRun(ctx, runID, flagQuery, windowDays, string(coverageJSON)); err != nil {
				return err
			}
			rows := signalsToRows(flagQuery, signals)
			if err := db.UpsertSignals(ctx, runID, rows); err != nil {
				return err
			}

			failed := 0
			for _, c := range coverage {
				if c.Status == "failed" {
					failed++
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: source %q failed: %s\n", c.Source, c.Error)
				}
			}

			res := sourcesSyncResult{Query: flagQuery, RunID: runID, Window: flagWindow, Coverage: coverage, Stored: len(rows)}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Synced %q (window %s): stored %d items, run %s\n", flagQuery, flagWindow, len(rows), runID)
			for _, c := range coverage {
				line := fmt.Sprintf("  - %-12s %s, %d items", c.Source, c.Status, c.Count)
				if c.Error != "" {
					line += " (" + c.Error + ")"
				}
				fmt.Fprintln(out, line)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagQuery, "query", "", "Topic to sync (required)")
	cmd.Flags().StringVar(&flagSource, "source", "", "Restrict to one source (default: all)")
	cmd.Flags().StringVar(&flagWindow, "window", "30d", "Recency window (e.g. 7d, 30d, 48h)")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Max items to pull per source")
	return cmd
}
