// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

// watch persists named catalog searches and reports templates newly matching
// since the last run — a time-windowed delta no single Placeit call provides.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/placeit/internal/algolia"
	"github.com/spf13/cobra"
)

type watchEntry struct {
	Name     string   `json:"name"`
	Query    string   `json:"query"`
	Category string   `json:"category,omitempty"`
	LastSeen []string `json:"last_seen"`
	LastRun  string   `json:"last_run,omitempty"`
}

func watchFilePath() string {
	return filepath.Join(filepath.Dir(defaultDBPath("placeit-pp-cli")), "watches.json")
}

func loadWatches() ([]watchEntry, error) {
	data, err := os.ReadFile(watchFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return []watchEntry{}, nil
		}
		return nil, err
	}
	var out []watchEntry
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func saveWatches(entries []watchEntry) error {
	path := watchFilePath()
	// User-private watchlist (search queries); match config/cache 0o700/0o600.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Persist named catalog searches and report newly matching templates.",
		Long: strings.Trim(`
Save named catalog searches and re-run them to see which templates are new
since the last run — a fresh-templates watchlist for a niche. 'watch run'
queries Placeit live and diffs against the ids seen on the previous run.`, "\n"),
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWatchAddCmd(flags))
	cmd.AddCommand(newWatchListCmd(flags))
	cmd.AddCommand(newWatchRunCmd(flags))
	cmd.AddCommand(newWatchRemoveCmd(flags))
	return cmd
}

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	var name, category string
	cmd := &cobra.Command{
		Use:     "add <query>",
		Short:   "Save a named catalog search to watch",
		Example: "  placeit-pp-cli watch add \"halloween instagram\" --name halloween",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query is required"))
			}
			query := strings.Join(args, " ")
			if name == "" {
				name = strings.ReplaceAll(strings.ToLower(query), " ", "-")
			}
			entries, err := loadWatches()
			if err != nil {
				return apiErr(err)
			}
			for _, e := range entries {
				if e.Name == name {
					return usageErr(fmt.Errorf("a watch named %q already exists; remove it first or pick --name", name))
				}
			}
			entries = append(entries, watchEntry{Name: name, Query: query, Category: category, LastSeen: []string{}})
			if err := saveWatches(entries); err != nil {
				return apiErr(err)
			}
			return flags.printJSON(cmd, map[string]any{"added": name, "query": query, "category": category})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Name for the saved search (defaults to a slug of the query)")
	cmd.Flags().StringVar(&category, "category", "", "Restrict the search to a category")
	return cmd
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List saved catalog searches",
		Example:     "  placeit-pp-cli watch list --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			entries, err := loadWatches()
			if err != nil {
				return apiErr(err)
			}
			return flags.printJSON(cmd, entries)
		},
	}
}

func newWatchRemoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "remove <name>",
		Short:   "Remove a saved catalog search",
		Example: "  placeit-pp-cli watch remove halloween",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a watch name is required"))
			}
			name := args[0]
			entries, err := loadWatches()
			if err != nil {
				return apiErr(err)
			}
			kept := make([]watchEntry, 0, len(entries))
			found := false
			for _, e := range entries {
				if e.Name == name {
					found = true
					continue
				}
				kept = append(kept, e)
			}
			if !found {
				return notFoundErr(fmt.Errorf("no watch named %q", name))
			}
			if err := saveWatches(kept); err != nil {
				return apiErr(err)
			}
			return flags.printJSON(cmd, map[string]any{"removed": name})
		},
	}
}

func newWatchRunCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:     "run [name]",
		Short:   "Run saved searches live and report templates new since the last run",
		Example: "  placeit-pp-cli watch run --agent",
		// Not mcp:read-only: each run persists LastSeen/LastRun back to the
		// watches file, so it mutates local state and should prompt in MCP hosts.
		Annotations: map[string]string{"pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			entries, err := loadWatches()
			if err != nil {
				return apiErr(err)
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "no saved searches; add one with: placeit-pp-cli watch add <query>")
				return flags.printJSON(cmd, []any{})
			}
			only := ""
			if len(args) > 0 {
				only = args[0]
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			ac := algolia.New(flags.timeout)

			results := make([]map[string]any, 0)
			changed := false
			for i := range entries {
				e := &entries[i]
				if only != "" && e.Name != only {
					continue
				}
				ff, _, ferr := buildFacetFilters(e.Category, "", "", nil)
				if ferr != nil {
					return usageErr(ferr)
				}
				res, serr := ac.Search(ctx, algolia.SearchParams{
					Index:        algolia.IndexNewest,
					Query:        e.Query,
					HitsPerPage:  limit,
					FacetFilters: ff,
				})
				if serr != nil {
					return apiErr(serr)
				}
				seen := map[string]struct{}{}
				for _, id := range e.LastSeen {
					seen[id] = struct{}{}
				}
				fresh := make([]map[string]any, 0)
				newIDs := make([]string, 0, len(res.Hits))
				for _, h := range res.Hits {
					m, cerr := cleanStage(h)
					if cerr != nil {
						continue
					}
					id := stageID(m)
					if id == "" {
						continue
					}
					newIDs = append(newIDs, id)
					if _, ok := seen[id]; !ok && len(e.LastSeen) > 0 {
						fresh = append(fresh, projectStage(m))
					}
				}
				results = append(results, map[string]any{
					"name":      e.Name,
					"query":     e.Query,
					"new_count": len(fresh),
					"new":       fresh,
					"first_run": len(e.LastSeen) == 0,
				})
				for _, id := range newIDs {
					seen[id] = struct{}{}
				}
				// Union the prior seen-set with this run, so a template that drops out
				// of and re-enters the scan window is not re-reported as new later.
				mergedSeen := make([]string, 0, len(seen))
				for id := range seen {
					mergedSeen = append(mergedSeen, id)
				}
				sort.Strings(mergedSeen)
				e.LastSeen = mergedSeen
				e.LastRun = time.Now().UTC().Format(time.RFC3339)
				changed = true
			}
			if only != "" && len(results) == 0 {
				return notFoundErr(fmt.Errorf("no watch named %q", only))
			}
			if changed {
				if err := saveWatches(entries); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not persist watch state: %v\n", err)
				}
			}
			return flags.printJSON(cmd, results)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Templates to scan per watch")
	return cmd
}
