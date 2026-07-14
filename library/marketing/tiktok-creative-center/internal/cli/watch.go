// Copyright 2026 Jon and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-implemented transcendence command for the TikTok Creative Center CLI.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// watchEntry is one tracked hashtag in the watchlist. HasBaseline
// distinguishes "never checked yet" from "checked and measured zero" so a
// freshly-added entry that's already above threshold doesn't report a false
// crossing on its first check (LastPopularity == 0 is ambiguous between the
// two without this flag).
type watchEntry struct {
	Hashtag        string  `json:"hashtag"`
	Threshold      float64 `json:"threshold"`
	LastPopularity float64 `json:"lastPopularity"`
	HasBaseline    bool    `json:"hasBaseline"`
}

// watchReport is the output of a watch check.
type watchReport struct {
	Crossed []watchCrossed `json:"crossed"`
	Stable  []watchEntry   `json:"stable"`
}

type watchCrossed struct {
	Hashtag        string  `json:"hashtag"`
	Threshold      float64 `json:"threshold"`
	LastPopularity float64 `json:"lastPopularity"`
	Current        float64 `json:"current"`
}

// pp:data-source local
func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var flagThreshold string
	var flagRegion string

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Track hashtags and report which crossed a popularity threshold since the last snapshot.",
		Long: "Tracks hashtags in a local watchlist and reports which crossed their popularity threshold " +
			"since the last check. Subcommands: 'watch add <hashtag> --threshold N', 'watch list', " +
			"'watch rm <hashtag>'. Running 'watch' with no subcommand checks all entries against the " +
			"local store. Reads synced popularity; run 'sync' first.",
		Example:     "  tiktok-creative-center-pp-cli watch add \"gaming\" --threshold 80",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}

	addCmd := &cobra.Command{
		Use:     "add <hashtag>",
		Short:   "Add a hashtag to the watchlist with a popularity threshold.",
		Example: "  tiktok-creative-center-pp-cli watch add \"gaming\" --threshold 80",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			thr, err := strconv.ParseFloat(strings.TrimSpace(flagThreshold), 64)
			if err != nil {
				return fmt.Errorf("invalid --threshold %q: expected a number", flagThreshold)
			}
			list, err := loadWatchlist()
			if err != nil {
				return err
			}
			list = upsertWatchEntry(list, watchEntry{Hashtag: args[0], Threshold: thr})
			if err := saveWatchlist(list); err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{"added": args[0], "threshold": thr, "watching": len(list)})
		},
	}

	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List tracked hashtags and their thresholds.",
		Example: "  tiktok-creative-center-pp-cli watch list",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			list, err := loadWatchlist()
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, list)
		},
	}

	rmCmd := &cobra.Command{
		Use:     "rm <hashtag>",
		Short:   "Remove a hashtag from the watchlist.",
		Example: "  tiktok-creative-center-pp-cli watch rm \"gaming\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			list, err := loadWatchlist()
			if err != nil {
				return err
			}
			if !watchlistContains(list, args[0]) {
				return fmt.Errorf("hashtag %q is not in the watchlist", args[0])
			}
			list = removeWatchEntry(list, args[0])
			if err := saveWatchlist(list); err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{"removed": args[0], "watching": len(list)})
		},
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return nil
		}
		ctx := cmd.Context()
		list, err := loadWatchlist()
		if err != nil {
			return err
		}
		if len(list) == 0 {
			return fmt.Errorf("watchlist is empty; add entries with 'watch add <hashtag> --threshold N'")
		}
		db, err := novelOpenStore(ctx)
		if err != nil {
			return err
		}
		defer db.Close()
		rows, err := loadHashtagRows(ctx, db, flagRegion)
		if err != nil {
			return err
		}
		report, updated := checkWatchlist(list, rows)
		if err := saveWatchlist(updated); err != nil {
			return err
		}
		return flags.printJSON(cmd, report)
	}

	addCmd.Flags().StringVar(&flagThreshold, "threshold", "", "Popularity threshold to alert at (required)")
	_ = addCmd.MarkFlagRequired("threshold")
	cmd.Flags().StringVar(&flagRegion, "region", "", "ISO country code to filter synced hashtags (empty = all)")
	cmd.AddCommand(addCmd)
	cmd.AddCommand(listCmd)
	cmd.AddCommand(rmCmd)
	return cmd
}

// checkWatchlist compares each entry's current synced popularity to its
// threshold, returning the report and the updated list (with lastPopularity
// refreshed). An entry crosses when current >= threshold AND a prior baseline
// check measured it below threshold. A first check (no baseline yet) only
// establishes the baseline and never reports a crossing — an entry that's
// already above threshold when first watched was never "crossed" while being
// watched, so alerting on it would be a false positive.
func checkWatchlist(list []watchEntry, rows []hashtagRow) (watchReport, []watchEntry) {
	byName := map[string]hashtagRow{}
	for _, r := range rows {
		byName[strings.ToLower(r.Name)] = r
	}
	report := watchReport{}
	updated := make([]watchEntry, 0, len(list))
	for _, e := range list {
		row, ok := byName[strings.ToLower(e.Hashtag)]
		current := row.Popularity
		entry := e
		entry.LastPopularity = current
		entry.HasBaseline = true
		updated = append(updated, entry)
		if !ok {
			// Not in local store; leave in stable with current 0.
			report.Stable = append(report.Stable, e)
			continue
		}
		if e.HasBaseline && current >= e.Threshold && e.LastPopularity < e.Threshold {
			report.Crossed = append(report.Crossed, watchCrossed{
				Hashtag:        e.Hashtag,
				Threshold:      e.Threshold,
				LastPopularity: e.LastPopularity,
				Current:        current,
			})
		} else {
			report.Stable = append(report.Stable, e)
		}
	}
	return report, updated
}

// upsertWatchEntry adds or replaces a watch entry by hashtag name.
func upsertWatchEntry(list []watchEntry, entry watchEntry) []watchEntry {
	for i, e := range list {
		if strings.EqualFold(e.Hashtag, entry.Hashtag) {
			list[i] = entry
			return list
		}
	}
	return append(list, entry)
}

// watchlistContains reports whether name is already tracked (case-insensitive).
func watchlistContains(list []watchEntry, name string) bool {
	for _, e := range list {
		if strings.EqualFold(e.Hashtag, name) {
			return true
		}
	}
	return false
}

// removeWatchEntry removes a watch entry by hashtag name.
func removeWatchEntry(list []watchEntry, name string) []watchEntry {
	out := make([]watchEntry, 0, len(list))
	for _, e := range list {
		if !strings.EqualFold(e.Hashtag, name) {
			out = append(out, e)
		}
	}
	return out
}

// watchlistPath returns the JSON watchlist file path.
func watchlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "tiktok-creative-center-pp-cli", "watchlist.json"), nil
}

// loadWatchlist reads the watchlist, returning an empty list if absent.
func loadWatchlist() ([]watchEntry, error) {
	path, err := watchlistPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []watchEntry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading watchlist: %w", err)
	}
	var list []watchEntry
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing watchlist %s: %w", path, err)
	}
	return list, nil
}

// saveWatchlist writes the watchlist with 0600 permissions.
func saveWatchlist(list []watchEntry) error {
	path, err := watchlistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating watchlist dir: %w", err)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling watchlist: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing watchlist: %w", err)
	}
	return nil
}

// ensure context import retained for the open-store call path.
var _ = context.Background
