// Copyright 2026 rai. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// watchlistEntry tracks one series the user is following.
type watchlistEntry struct {
	Alias    string `json:"alias"`
	SeriesID string `json:"series_id"`
	Added    string `json:"added"`
}

type watchlistFile struct {
	Entries []watchlistEntry `json:"entries"`
}

func watchlistPath() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cfg, "cricapi-pp-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "watchlist.json"), nil
}

func loadWatchlist() (*watchlistFile, error) {
	path, err := watchlistPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &watchlistFile{Entries: []watchlistEntry{}}, nil
		}
		return nil, err
	}
	var f watchlistFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

func saveWatchlist(f *watchlistFile) error {
	path, err := watchlistPath()
	if err != nil {
		return err
	}
	sort.SliceStable(f.Entries, func(i, j int) bool {
		return f.Entries[i].Alias < f.Entries[j].Alias
	})
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func newWatchlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watchlist",
		Short: "Track favourite series locally for quick refresh",
		Long: `Maintain a local list of series IDs under friendly aliases so you can
refresh fixtures and standings for all of them with a single command.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newWatchlistAddCmd(flags))
	cmd.AddCommand(newWatchlistListCmd(flags))
	cmd.AddCommand(newWatchlistRemoveCmd(flags))
	cmd.AddCommand(newWatchlistRefreshCmd(flags))
	return cmd
}

func newWatchlistAddCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "add [alias] [series-id]",
		Short:   "Add a series to the local watchlist",
		Example: "  cricapi-pp-cli watchlist add ipl abc-series-id-here",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			alias := strings.TrimSpace(args[0])
			seriesID := strings.TrimSpace(args[1])
			if alias == "" || seriesID == "" {
				return cmd.Help()
			}
			f, err := loadWatchlist()
			if err != nil {
				return err
			}
			for i, e := range f.Entries {
				if e.Alias == alias {
					f.Entries[i].SeriesID = seriesID
					f.Entries[i].Added = time.Now().UTC().Format(time.RFC3339)
					if err := saveWatchlist(f); err != nil {
						return err
					}
					fmt.Fprintf(cmd.OutOrStdout(), "Updated %q -> %s\n", alias, seriesID)
					return nil
				}
			}
			f.Entries = append(f.Entries, watchlistEntry{
				Alias:    alias,
				SeriesID: seriesID,
				Added:    time.Now().UTC().Format(time.RFC3339),
			})
			if err := saveWatchlist(f); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added %q -> %s\n", alias, seriesID)
			return nil
		},
	}
}

func newWatchlistListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List entries in the local watchlist",
		Example: "  cricapi-pp-cli watchlist list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			f, err := loadWatchlist()
			if err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), f.Entries, flags)
			}
			if len(f.Entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Watchlist empty. Add an entry with: cricapi-pp-cli watchlist add <alias> <series-id>")
				return nil
			}
			rows := make([]map[string]any, 0, len(f.Entries))
			for _, e := range f.Entries {
				rows = append(rows, map[string]any{
					"alias":     e.Alias,
					"series_id": e.SeriesID,
					"added":     e.Added,
				})
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		},
	}
}

func newWatchlistRemoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "remove [alias]",
		Short:   "Remove an entry from the local watchlist",
		Example: "  cricapi-pp-cli watchlist remove ipl",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			alias := strings.TrimSpace(args[0])
			if alias == "" {
				return cmd.Help()
			}
			f, err := loadWatchlist()
			if err != nil {
				return err
			}
			out := make([]watchlistEntry, 0, len(f.Entries))
			removed := false
			for _, e := range f.Entries {
				if e.Alias == alias {
					removed = true
					continue
				}
				out = append(out, e)
			}
			if !removed {
				fmt.Fprintf(cmd.OutOrStdout(), "No entry with alias %q\n", alias)
				return nil
			}
			f.Entries = out
			if err := saveWatchlist(f); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %q\n", alias)
			return nil
		},
	}
}

func newWatchlistRefreshCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "refresh",
		Short:   "Fetch latest info for every series in the watchlist",
		Example: "  cricapi-pp-cli watchlist refresh",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			f, err := loadWatchlist()
			if err != nil {
				return err
			}
			if len(f.Entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Watchlist empty.")
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type refreshResult struct {
				Alias    string          `json:"alias"`
				SeriesID string          `json:"series_id"`
				Info     json.RawMessage `json:"info,omitempty"`
				Error    string          `json:"error,omitempty"`
			}
			results := make([]refreshResult, 0, len(f.Entries))

			for _, e := range f.Entries {
				path := "/series_info"
				params := map[string]string{"id": e.SeriesID, "offset": "0"}
				data, _, ferr := resolveRead(cmd.Context(), c, flags, "series", false, path, params, nil)
				r := refreshResult{Alias: e.Alias, SeriesID: e.SeriesID}
				if ferr != nil {
					r.Error = ferr.Error()
				} else {
					r.Info = data
				}
				results = append(results, r)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			rows := make([]map[string]any, 0, len(results))
			for _, r := range results {
				row := map[string]any{
					"alias":     r.Alias,
					"series_id": r.SeriesID,
					"status":    "ok",
				}
				if r.Error != "" {
					row["status"] = "error: " + r.Error
				}
				rows = append(rows, row)
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		},
	}
}
