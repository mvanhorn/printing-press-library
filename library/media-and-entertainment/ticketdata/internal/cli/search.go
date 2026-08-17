// pp:data-source local
// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketdata/internal/store"
	"github.com/spf13/cobra"
)

type searchView struct {
	Query   string            `json:"query"`
	Type    string            `json:"type"`
	Count   int               `json:"count"`
	Results []json.RawMessage `json:"results"`
}

func newNovelSearchCmd(flags *rootFlags) *cobra.Command {
	var resourceType string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "search <query>",
		Short:       "Local full-text search across synced events, performers, and venues, returning multiple matches.",
		Long:        "Use `search` to browse multiple offline matches. For the single canonical resolve of a name to its stats use `performers search` / `venues search`.",
		Example:     "  ticketdata-pp-cli search \"ariana\" --type performers --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "ariana"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && commandNFlag(cmd) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search local synced resources")
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("search query is required"))
			}
			if limit <= 0 {
				return usageErr(fmt.Errorf("--limit must be positive"))
			}
			types, typeLabel, err := searchTypes(resourceType)
			if err != nil {
				return usageErr(err)
			}
			if !cmd.Flags().Changed("db") {
				dbPath = defaultDBPath("ticketdata-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			query := strings.TrimSpace(strings.Join(args, " "))
			results, err := runOfflineSearch(db, query, limit, types)
			if err != nil {
				return err
			}
			if len(results) == 0 {
				hintIfUnsynced(cmd, db, "")
			}
			view := searchView{Query: query, Type: typeLabel, Count: len(results), Results: results}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return printSearchTable(cmd, view)
		},
	}
	cmd.Flags().StringVar(&resourceType, "type", "", "Restrict search to events, performers, or venues")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of results")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("ticketdata-pp-cli"), "SQLite database file path")
	return cmd
}

func searchTypes(resourceType string) ([]string, string, error) {
	switch resourceType {
	case "":
		return []string{"events", "performers", "venues"}, "all", nil
	case "events", "performers", "venues":
		return []string{resourceType}, resourceType, nil
	default:
		return nil, "", fmt.Errorf("--type must be one of events, performers, venues")
	}
}

func runOfflineSearch(db *store.Store, query string, limit int, types []string) ([]json.RawMessage, error) {
	results := make([]json.RawMessage, 0)
	for _, resourceType := range types {
		docs, err := db.Search(query, limit, resourceType)
		if err != nil {
			return nil, err
		}
		for _, doc := range docs {
			if len(results) >= limit {
				return results, nil
			}
			results = append(results, doc)
		}
	}
	return results, nil
}

func printSearchTable(cmd *cobra.Command, view searchView) error {
	if len(view.Results) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no local matches found")
		return nil
	}
	tw := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(tw, "RESULT")
	for _, raw := range view.Results {
		fmt.Fprintf(tw, "%s\n", truncate(string(raw), 120))
	}
	return tw.Flush()
}
