// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for the SRAM AXS CLI. Not generated.
// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/axs/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/axs/internal/store"

	"github.com/spf13/cobra"
)

type sinceItem struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Title     string `json:"title"`
	Timestamp string `json:"timestamp"`
}

type sinceView struct {
	Window string      `json:"window"`
	Since  string      `json:"since"`
	Items  []sinceItem `json:"items"`
}

func newNovelSinceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "since [window]",
		Short:       "Show new ride activities and notifications since your last sync.",
		Long:        "Time-windowed diff of new activities and notifications from your local mirror. Defaults to the last 7 days. Run `axs-pp-cli sync` first to populate the mirror.",
		Example:     "  axs-pp-cli since 7d",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			window := "7d"
			if len(args) > 0 {
				window = args[0]
			}
			dur, err := cliutil.ParseDurationLoose(window)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid window %q: %w", window, err))
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			cutoff := time.Now().Add(-dur).UTC()

			if dbPath == "" {
				dbPath = defaultDBPath("axs-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: axs-pp-cli sync --resources activities,notifications --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), sinceView{Window: window, Since: cutoff.Format(time.RFC3339), Items: []sinceItem{}}, flags)
				}
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			view := sinceView{Window: window, Since: cutoff.Format(time.RFC3339), Items: []sinceItem{}}
			rows, err := db.DB().QueryContext(ctx, `
				SELECT id, resource_type, data FROM resources
				WHERE resource_type IN ('activities','notifications')`)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()
			for rows.Next() {
				var id, rtype string
				var raw sql.NullString
				if err := rows.Scan(&id, &rtype, &raw); err != nil {
					continue
				}
				var m map[string]any
				if json.Unmarshal([]byte(raw.String), &m) != nil {
					continue
				}
				tsStr := gstr(m, "start_ts", "create_ts", "created_at", "timestamp", "start_time")
				if tsStr == "" {
					continue
				}
				ts, ok := parseAXSTime(tsStr)
				if !ok || ts.Before(cutoff) {
					continue
				}
				title := gstr(m, "name", "title", "body")
				view.Items = append(view.Items, sinceItem{
					Type:      rtype,
					ID:        id,
					Title:     title,
					Timestamp: tsStr,
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate resources: %w", err)
			}
			sortSinceItems(view.Items)

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			if len(view.Items) == 0 {
				fmt.Fprintf(out, "nothing new in the last %s\n", window)
				return nil
			}
			headers := []string{"WHEN", "TYPE", "TITLE"}
			var tableRows [][]string
			for _, it := range view.Items {
				tableRows = append(tableRows, []string{it.Timestamp, it.Type, it.Title})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// parseAXSTime accepts the common timestamp shapes AXS emits: RFC3339, a plain
// date, and unix-epoch seconds rendered as a string.
func parseAXSTime(s string) (time.Time, bool) {
	if epoch, err := strconv.ParseInt(s, 10, 64); err == nil {
		if epoch > 1_000_000_000_000 {
			return time.UnixMilli(epoch).UTC(), true
		}
		return time.Unix(epoch, 0).UTC(), true
	}
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

func sortSinceItems(items []sinceItem) {
	sort.SliceStable(items, func(i, j int) bool {
		left, leftOK := parseAXSTime(items[i].Timestamp)
		right, rightOK := parseAXSTime(items[j].Timestamp)
		if leftOK && rightOK {
			return left.After(right)
		}
		return items[i].Timestamp > items[j].Timestamp
	})
}
