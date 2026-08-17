// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Builds a first-seen / last-seen / mention-count timeline for
// a named company or person across synced webset items and monitor-run results.
// pp:data-source local

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/exa/internal/store"
)

// mentionHits carries where an entity name was found and when.
type mentionHits struct {
	First time.Time
	Last  time.Time
	Count int
}

func (h *mentionHits) note(ts time.Time) {
	if h.Count == 0 || ts.Before(h.First) {
		h.First = ts
	}
	if ts.After(h.Last) {
		h.Last = ts
	}
	h.Count++
}

// scanItemsForEntity scans webset items whose data JSON mentions the entity
// name. The items table stores each webset item with its entity properties
// (companies, people, articles, papers) in the data column.
func scanItemsForEntity(ctx context.Context, db *store.Store, name string, cutoff time.Time) (mentionHits, []string, error) {
	var hits mentionHits
	var sampleIDs []string
	rows, err := db.DB().QueryContext(ctx,
		`SELECT data, synced_at FROM items ORDER BY synced_at`)
	if err != nil {
		return hits, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var data string
		var syncedAt string
		if err := rows.Scan(&data, &syncedAt); err != nil {
			return hits, nil, err
		}
		ts := parseSyncedAt(syncedAt)
		if !cutoff.IsZero() && (ts.IsZero() || ts.Before(cutoff)) {
			continue
		}
		lower := strings.ToLower(data)
		if !strings.Contains(lower, strings.ToLower(name)) {
			continue
		}
		hits.note(ts)
		if len(sampleIDs) < 5 {
			var obj map[string]any
			if json.Unmarshal([]byte(data), &obj) == nil {
				if id, _ := obj["id"].(string); id != "" {
					sampleIDs = append(sampleIDs, id)
				}
			}
		}
	}
	return hits, sampleIDs, rows.Err()
}

// scanRunsForEntity scans synced monitor runs whose output results mention the
// entity, grouped by how many runs mention it.
func scanRunsForEntity(ctx context.Context, db *store.Store, name string, cutoff time.Time) (mentionHits, error) {
	var hits mentionHits
	rows, err := db.DB().QueryContext(ctx,
		`SELECT data, synced_at FROM runs ORDER BY synced_at`)
	if err != nil {
		return hits, err
	}
	defer rows.Close()
	lower := strings.ToLower(name)
	for rows.Next() {
		var data string
		var syncedAt string
		if err := rows.Scan(&data, &syncedAt); err != nil {
			return hits, err
		}
		ts := parseSyncedAt(syncedAt)
		if !cutoff.IsZero() && (ts.IsZero() || ts.Before(cutoff)) {
			continue
		}
		if !strings.Contains(strings.ToLower(data), lower) {
			continue
		}
		hits.note(ts)
	}
	return hits, rows.Err()
}

func parseSyncedAt(v string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z07:00"} {
		if t, err := time.Parse(layout, v); err == nil {
			return t
		}
	}
	return time.Time{}
}

// parseHumanDuration parses durations like "7d", "24h", "90m", "30d" — Go's
// time.ParseDuration does not accept day units, and day windows are the
// natural unit for research sweeps. Trailing garbage and non-positive
// values are rejected so a typo cannot silently shift the window.
func parseHumanDuration(s string) (time.Duration, error) {
	lower := strings.ToLower(strings.TrimSpace(s))
	if lower == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if strings.HasSuffix(lower, "d") {
		n := strings.TrimSuffix(lower, "d")
		var days float64
		if _, err := fmt.Sscanf(n, "%g", &days); err != nil {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		// Reject trailing garbage: "1d2h" must not parse as 1 day.
		if fmt.Sprintf("%g", days) != strings.TrimSpace(n) {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		if days <= 0 {
			return 0, fmt.Errorf("duration must be positive: %q", s)
		}
		return time.Duration(days * float64(24*time.Hour)), nil
	}
	d, err := time.ParseDuration(lower)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q", s)
	}
	if d <= 0 {
		return 0, fmt.Errorf("duration must be positive: %q", s)
	}
	return d, nil
}

func newNovelEntityReportCmd(flags *rootFlags) *cobra.Command {
	var flagType string
	var flagSince string

	cmd := &cobra.Command{
		Use:   "report [name]",
		Short: "Build a first-seen / last-seen / mention-count timeline for any company or person across your synced searches and webset items.",
		Long: `Use this command for a first-seen / last-seen / mention-count timeline of a named company or person across synced webset items and search results.
Do NOT use it to compare two scheduled monitor runs; use 'monitor diff'.
Do NOT use it for new items in a live webset; use 'webset new'.
Do NOT use it to run a fresh search; use 'search'.`,
		Example:     "  exa-pp-cli entity report 'Acme Corp' --type company --since 30d",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3", "pp:happy-args": "<name>=Acme Corp"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "entity report")
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if len(args) != 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("entity name is required"))
			}
			name := strings.TrimSpace(args[0])
			if name == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("entity name is required"))
			}
			if flagType != "" && flagType != "company" && flagType != "person" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--type must be 'company' or 'person', got %q", flagType))
			}

			dbPath := defaultDBPath("exa-pp-cli")
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no results for %s \u2014 no local mirror yet\nrun: exa-pp-cli sync --resources websets,monitors --db %s\n", name, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"entity": name, "error": "no-local-mirror",
						"syncHint": "exa-pp-cli sync --resources websets,monitors",
					}, flags)
				}
				return fmt.Errorf("no results for %s: run 'exa-pp-cli sync --resources websets,monitors' first", name)
			}
			db, err := store.OpenReadOnlyContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			if !hintIfUnsynced(cmd, db, "items") {
				hintIfStale(cmd, db, "items", flags.maxAge)
			}

			var cutoff time.Time
			if flagSince != "" {
				d, perr := parseHumanDuration(flagSince)
				if perr != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--since must be a duration like 7d or 24h, got %q", flagSince))
				}
				cutoff = time.Now().Add(-d)
			}
			items, itemIDs, err := scanItemsForEntity(cmd.Context(), db, name, cutoff)
			if err != nil {
				return fmt.Errorf("scanning webset items: %w", err)
			}
			runs, err := scanRunsForEntity(cmd.Context(), db, name, cutoff)
			if err != nil {
				return fmt.Errorf("scanning monitor runs: %w", err)
			}

			first, last, count := items.First, items.Last, items.Count
			if runs.Count > 0 {
				if count == 0 || runs.First.Before(first) {
					first = runs.First
				}
				if runs.Last.After(last) {
					last = runs.Last
				}
				count += runs.Count
			}

			view := struct {
				Entity      string   `json:"entity"`
				Type        string   `json:"type,omitempty"`
				FirstSeen   string   `json:"firstSeen,omitempty"`
				LastSeen    string   `json:"lastSeen,omitempty"`
				Mentions    int      `json:"mentionCount"`
				WebsetItems int      `json:"websetItemMentions"`
				MonitorRuns int      `json:"monitorRunMentions"`
				SampleIDs   []string `json:"sampleItemIds,omitempty"`
				Source      string   `json:"source"`
			}{
				Entity:      name,
				Type:        flagType,
				Mentions:    count,
				WebsetItems: items.Count,
				MonitorRuns: runs.Count,
				SampleIDs:   itemIDs,
				Source:      "local",
			}
			if !first.IsZero() {
				view.FirstSeen = first.UTC().Format(time.RFC3339)
			}
			if !last.IsZero() {
				view.LastSeen = last.UTC().Format(time.RFC3339)
			}

			if count == 0 {
				if flags.asJSON || flags.agent {
					_ = printJSONFiltered(cmd.OutOrStdout(), view, flags)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "No match for entity %q in synced webset items or monitor runs.\n", name)
				}
				return notFoundErr(fmt.Errorf("no match for entity %q in synced data", name))
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Entity: %s\n", name)
			if flagType != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Type:   %s\n", flagType)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "First seen:  %s\n", view.FirstSeen)
			fmt.Fprintf(cmd.OutOrStdout(), "Last seen:   %s\n", view.LastSeen)
			fmt.Fprintf(cmd.OutOrStdout(), "Mentions:    %d (webset items: %d, monitor runs: %d)\n", count, items.Count, runs.Count)
			if len(itemIDs) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Sample items:\n")
				for _, id := range itemIDs {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", id)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagType, "type", "", "Entity kind filter: company or person")
	cmd.Flags().StringVar(&flagSince, "since", "", "Only consider mentions synced within this window (e.g. 7d, 30d)")
	return cmd
}

// ensure sort is used when no items exist
var _ = sort.Strings
