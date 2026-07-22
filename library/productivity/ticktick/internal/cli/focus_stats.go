// Copyright 2026 Harvey The AI Guy and contributors. Licensed under Apache-2.0. See LICENSE.
// Focus stats: focus/pomodoro minutes aggregated per day from the local mirror.

package cli

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/ticktick/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/ticktick/internal/store"
)

// pp:data-source local
func newNovelFocusStatsCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagBy string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Focus/pomodoro minutes aggregated per day over a window",
		Long: "Aggregates synced focus and pomodoro records from the local mirror into per-day duration totals.\n" +
			"Run 'sync --resources focus' first to hydrate the mirror.",
		Example:     "  ticktick-pp-cli focus stats --since 7d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would aggregate local focus records")
				return nil
			}
			if flagSince == "" {
				flagSince = "7d"
			}
			window, err := cliutil.ParseDurationLoose(flagSince)
			if err != nil {
				return usageErr(fmt.Errorf("--since: %w", err))
			}
			cutoffStr := time.Now().Add(-window).Format("2006-01-02")
			if dbPath == "" {
				dbPath = defaultDBPath("ticktick-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: ticktick-pp-cli sync --resources focus --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					// Keep the documented object shape on an empty/unsynced
					// cache so JSON consumers keyed on object fields don't
					// see the type flip to a bare array.
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"since":         cutoffStr,
						"window":        flagSince,
						"by":            flagBy,
						"total_minutes": 0,
						"by_" + flagBy:  []any{},
					}, flags)
				}
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "focus") {
				hintIfStale(cmd, db, "focus", 24*time.Hour)
			}

			focusRows, err := queryResourceMaps(cmd, db, "focus")
			if err != nil {
				return err
			}
			if flagBy != "day" && flagBy != "project" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--by must be 'day' or 'project' (got %q)", flagBy))
			}
			byKey := map[string]float64{}
			sessions := map[string]int{}
			var total float64
			for _, f := range focusRows {
				start, _ := f["startTime"].(string)
				if len(start) < 10 || start[:10] < cutoffStr {
					continue
				}
				key := start[:10]
				if flagBy == "project" {
					key = focusProjectName(f)
				}
				m := focusDurationMinutes(f)
				byKey[key] += m
				sessions[key]++
				total += m
			}
			keys := make([]string, 0, len(byKey))
			for k := range byKey {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			label := "date"
			if flagBy == "project" {
				label = "project"
			}
			rows := make([]map[string]any, 0, len(keys))
			for _, k := range keys {
				rows = append(rows, map[string]any{
					label:      k,
					"minutes":  int(byKey[k]),
					"sessions": sessions[k],
				})
			}
			view := map[string]any{
				"since":         cutoffStr,
				"window":        flagSince,
				"by":            flagBy,
				"total_minutes": int(total),
				"by_" + flagBy:  rows,
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Focus since %s: %d minutes\n", cutoffStr, int(total))
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "  %v  %4d min  (%d sessions)\n", r[label], r["minutes"], r["sessions"])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Aggregation window (e.g. 7d, 2w, 24h)")
	cmd.Flags().StringVar(&flagBy, "by", "day", "Aggregation dimension: day or project")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// focusProjectName extracts a display key for by-project grouping from a
// focus record's tasks array; records without task links group under "(none)".
func focusProjectName(f map[string]any) string {
	tasks, _ := f["tasks"].([]any)
	for _, t := range tasks {
		tm, _ := t.(map[string]any)
		if name, _ := tm["projectName"].(string); name != "" {
			return name
		}
		if title, _ := tm["title"].(string); title != "" {
			return title
		}
	}
	return "(none)"
}
