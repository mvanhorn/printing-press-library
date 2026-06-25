// Copyright 2026 Luke J and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for the FRED CLI. Carried across regen via the
// novel-command merge path; the generated stub was replaced.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// watchlistEntry is the last-known observation for a tracked series.
type watchlistEntry struct {
	SeriesID string `json:"series_id"`
	Date     string `json:"date"`
	Value    string `json:"value"`
}

// watchlistState is the persisted local watchlist.
type watchlistState struct {
	Series map[string]watchlistEntry `json:"series"`
}

// watchlistPath returns the JSON file backing the watchlist, beside the local DB.
func watchlistPath() string {
	dbPath := defaultDBPath("fred-pp-cli")
	return filepath.Join(filepath.Dir(dbPath), "watchlist.json")
}

func loadWatchlist() (*watchlistState, error) {
	st := &watchlistState{Series: map[string]watchlistEntry{}}
	data, err := os.ReadFile(watchlistPath())
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, st); err != nil {
		return nil, fmt.Errorf("parsing watchlist at %s: %w", watchlistPath(), err)
	}
	if st.Series == nil {
		st.Series = map[string]watchlistEntry{}
	}
	return st, nil
}

func saveWatchlist(st *watchlistState) error {
	p := watchlistPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

func sortedSeriesIDs(st *watchlistState) []string {
	ids := make([]string, 0, len(st.Series))
	for id := range st.Series {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func newNovelWatchlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "watchlist",
		Short:       "Track a personal set of FRED series and see what changed",
		Long:        "Persist a set of series locally, then sync their latest observations and surface only the ones that moved since the last sync. Detecting change between runs requires local state that no single FRED API call provides.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelWatchlistAddCmd(flags))
	cmd.AddCommand(newNovelWatchlistListCmd(flags))
	cmd.AddCommand(newNovelWatchlistSyncCmd(flags))
	return cmd
}

func newNovelWatchlistAddCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "add <series_id> [series_id...]",
		Short:       "Add one or more series to the watchlist",
		Example:     "  fred-pp-cli watchlist add UNRATE CPIAUCSL",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least one series id is required"))
			}
			st, err := loadWatchlist()
			if err != nil {
				return err
			}
			added := make([]string, 0, len(args))
			for _, id := range args {
				id = strings.TrimSpace(id)
				if id == "" {
					continue
				}
				if _, ok := st.Series[id]; !ok {
					st.Series[id] = watchlistEntry{SeriesID: id}
					added = append(added, id)
				}
			}
			if err := saveWatchlist(st); err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{
				"added":   added,
				"watched": sortedSeriesIDs(st),
			})
		},
	}
	return cmd
}

func newNovelWatchlistListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List the series currently on the watchlist",
		Example:     "  fred-pp-cli watchlist list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			st, err := loadWatchlist()
			if err != nil {
				return err
			}
			rows := make([]watchlistEntry, 0, len(st.Series))
			for _, id := range sortedSeriesIDs(st) {
				rows = append(rows, st.Series[id])
			}
			return flags.printJSON(cmd, rows)
		},
	}
	return cmd
}
