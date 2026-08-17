// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Diffs two synced monitor runs (the two most recent by
// default, or explicit --from/--to run ids) and reports which result URLs
// appeared, disappeared, or stayed the same.
// pp:data-source local

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/exa/internal/store"
)

// extractRunResultURLs pulls the result URL set out of a stored monitor run's
// data JSON. The stored run object is the SearchMonitorRun shape: data.output
// carries results[] where each result has a url field.
func extractRunResultURLs(data json.RawMessage) ([]string, error) {
	var run struct {
		Output *struct {
			Results []struct {
				URL string `json:"url"`
			} `json:"results"`
		} `json:"output"`
	}
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, err
	}
	if run.Output == nil {
		return nil, nil
	}
	seen := map[string]bool{}
	var urls []string
	for _, r := range run.Output.Results {
		u := strings.TrimSpace(r.URL)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		urls = append(urls, u)
	}
	sort.Strings(urls)
	return urls, nil
}

// runDataID extracts the canonical run id from the stored run payload.
func runDataID(data json.RawMessage) string {
	var obj struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return ""
	}
	return obj.ID
}

// loadMonitorRunRows loads the stored run rows for a monitor, newest first,
// so the first two rows are the two most recent runs.
func loadMonitorRunRows(ctx context.Context, db *store.Store, monitorID string) ([]struct {
	ID    string
	Data  json.RawMessage
	Syncd string
}, error) {
	rows, err := db.DB().QueryContext(ctx,
		`SELECT id, data, synced_at FROM runs WHERE monitors_id = ? ORDER BY synced_at DESC, rowid DESC`, monitorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct {
		ID    string
		Data  json.RawMessage
		Syncd string
	}
	for rows.Next() {
		var r struct {
			ID    string
			Data  json.RawMessage
			Syncd string
		}
		var data string
		if err := rows.Scan(&r.ID, &data, &r.Syncd); err != nil {
			return nil, err
		}
		r.Data = json.RawMessage(data)
		// Storage IDs carry a parent suffix (run-a\u241fmon-2); the real run id
		// lives in the payload and is what --from/--to should match.
		if realID := runDataID(r.Data); realID != "" {
			r.ID = realID
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func newNovelMonitorDiffCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagTo string

	cmd := &cobra.Command{
		Use:   "diff [monitor-id]",
		Short: "Compare two synced monitor runs and see exactly which URLs are new, gone, or unchanged.",
		Long: `Use this command to see what changed between two runs of a scheduled monitor.
Do NOT use it for new items in a live webset; use 'webset new'.
Do NOT use it for a named entity's timeline; use 'entity report'.`,
		Example:     "  exa-pp-cli monitor diff '<monitor-id>'",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3", "pp:happy-args": "<monitor-id>=mon-1"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "monitor diff")
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if len(args) != 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("monitor-id is required"))
			}
			monitorID := strings.TrimSpace(args[0])
			if monitorID == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("monitor-id is required"))
			}

			dbPath := defaultDBPath("exa-pp-cli")
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no results for monitor %s \u2014 no local mirror yet\nrun: exa-pp-cli sync --resources monitors --db %s\n", monitorID, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"monitorId": monitorID, "error": "no-local-mirror", "syncHint": "exa-pp-cli sync --resources monitors",
					}, flags)
				}
				return notFoundErr(fmt.Errorf("no results for monitor %s: run 'exa-pp-cli sync --resources monitors' first", monitorID))
			}
			db, err := store.OpenReadOnlyContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			if !hintIfUnsynced(cmd, db, "monitors") {
				hintIfStale(cmd, db, "monitors", flags.maxAge)
			}

			runs, err := loadMonitorRunRows(cmd.Context(), db, monitorID)
			if err != nil {
				return fmt.Errorf("loading monitor runs: %w", err)
			}
			if len(runs) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no results for monitor %s \u2014 no synced runs yet\nrun: exa-pp-cli sync --resources monitors\n", monitorID)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"monitorId": monitorID, "error": "no-runs", "syncHint": "exa-pp-cli sync --resources monitors",
					}, flags)
				}
				return notFoundErr(fmt.Errorf("no results for monitor %s: run 'exa-pp-cli sync --resources monitors' first", monitorID))
			}

			// Pick the two runs to compare: explicit ids win, else two newest.
			toIdx, fromIdx := 0, 1
			if flagTo != "" {
				found := -1
				for i, r := range runs {
					if r.ID == flagTo {
						found = i
						break
					}
				}
				if found < 0 {
					return usageErr(fmt.Errorf("run %q not found in synced runs for monitor %s", flagTo, monitorID))
				}
				toIdx = found
			}
			if flagFrom != "" {
				found := -1
				for i, r := range runs {
					if r.ID == flagFrom {
						found = i
						break
					}
				}
				if found < 0 {
					return usageErr(fmt.Errorf("run %q not found in synced runs for monitor %s", flagFrom, monitorID))
				}
				fromIdx = found
			}
			if fromIdx == toIdx {
				if len(runs) < 2 {
					fmt.Fprintf(cmd.ErrOrStderr(), "only one synced run for monitor %s; sync again to capture a second run\n", monitorID)
					_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"monitorId": monitorID, "error": "need-two-runs",
					}, flags)
					return nil
				}
				if toIdx == len(runs)-1 {
					fromIdx = toIdx - 1
				} else {
					fromIdx = toIdx + 1
				}
			}

			toData := runs[toIdx].Data
			fromData := runs[fromIdx].Data
			toURLs, err := extractRunResultURLs(toData)
			if err != nil {
				return fmt.Errorf("parsing run %s output: %w", runs[toIdx].ID, err)
			}
			fromURLs, err := extractRunResultURLs(fromData)
			if err != nil {
				return fmt.Errorf("parsing run %s output: %w", runs[fromIdx].ID, err)
			}

			toSet := map[string]bool{}
			for _, u := range toURLs {
				toSet[u] = true
			}
			fromSet := map[string]bool{}
			for _, u := range fromURLs {
				fromSet[u] = true
			}
			var added, removed, kept []string
			for u := range toSet {
				if fromSet[u] {
					kept = append(kept, u)
				} else {
					added = append(added, u)
				}
			}
			for u := range fromSet {
				if !toSet[u] {
					removed = append(removed, u)
				}
			}
			sort.Strings(added)
			sort.Strings(removed)
			sort.Strings(kept)

			view := struct {
				MonitorID    string   `json:"monitorId"`
				FromRunID    string   `json:"fromRunId"`
				ToRunID      string   `json:"toRunId"`
				Added        []string `json:"added"`
				Removed      []string `json:"removed"`
				Unchanged    []string `json:"unchanged"`
				AddedCount   int      `json:"addedCount"`
				RemovedCount int      `json:"removedCount"`
				KeptCount    int      `json:"keptCount"`
				TotalTo      int      `json:"totalTo"`
				TotalFrom    int      `json:"totalFrom"`
				Source       string   `json:"source"`
			}{
				MonitorID:    monitorID,
				FromRunID:    runs[fromIdx].ID,
				ToRunID:      runs[toIdx].ID,
				Added:        added,
				Removed:      removed,
				Unchanged:    kept,
				AddedCount:   len(added),
				RemovedCount: len(removed),
				KeptCount:    len(kept),
				TotalTo:      len(toURLs),
				TotalFrom:    len(fromURLs),
				Source:       "local",
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Monitor %s\n", monitorID)
			fmt.Fprintf(cmd.OutOrStdout(), "  from run %s (%d urls)\n", view.FromRunID, view.TotalFrom)
			fmt.Fprintf(cmd.OutOrStdout(), "  to   run %s (%d urls)\n", view.ToRunID, view.TotalTo)
			fmt.Fprintf(cmd.OutOrStdout(), "  added:   %d\n", view.AddedCount)
			for _, u := range view.Added {
				fmt.Fprintf(cmd.OutOrStdout(), "    + %s\n", u)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  removed: %d\n", view.RemovedCount)
			for _, u := range view.Removed {
				fmt.Fprintf(cmd.OutOrStdout(), "    - %s\n", u)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  unchanged: %d\n", view.KeptCount)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "Earlier run id to diff from (default: second-most-recent synced run)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Later run id to diff against (default: most recent synced run)")
	return cmd
}
