// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/algolia/internal/store"
	"github.com/spf13/cobra"
)

type findHit struct {
	Index    string `json:"index"`
	ObjectID string `json:"objectID"`
	Title    string `json:"title,omitempty"`
}

type findResult struct {
	Query        string    `json:"query"`
	Hits         []findHit `json:"hits"`
	Scanned      int       `json:"scanned"`
	MaxScanPages int       `json:"max_scan_pages"`
	Note         string    `json:"note,omitempty"`
}

// nonRecordResourceTypes are synced resource kinds whose blobs are not user
// records (logs, keys, clusters, dictionaries, security) — searching them
// returns noise instead of records.
var nonRecordResourceTypes = map[string]bool{
	"logs":                 true,
	"keys":                 true,
	"clusters":             true,
	"clusters-mapping":     true,
	"clusters-mapping-top": true,
	"dictionaries":         true,
	"security":             true,
	"wait-for-api-key":     true,
}

func newNovelFindCmd(flags *rootFlags) *cobra.Command {
	var flagQuery string
	var flagLimit int
	var flagDB string
	var flagMaxScanPages int

	cmd := &cobra.Command{
		Use:         "find",
		Short:       "Search across every synced index in one shot, with each hit labeled by its source index.",
		Example:     "  algolia-pp-cli find --query dune --limit 20",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "find")
			}
			query := flagQuery
			if query == "" && len(args) > 0 {
				query = args[0]
			}
			if query == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--query is required"))
			}
			if flagLimit <= 0 {
				flagLimit = 20
			}
			if flagDB == "" {
				flagDB = defaultDBPath("algolia-pp-cli")
			}
			if _, statErr := os.Stat(flagDB); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: algolia-pp-cli sync --resources indexes to populate the local database.\n", flagDB)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), findResult{Query: query, Hits: make([]findHit, 0)}, flags)
				}
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), flagDB)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "") {
				hintIfStale(cmd, db, "", flags.maxAge)
			}

			// Discover every synced resource type and search each with FTS,
			// labeling hits by resource_type. Skip non-record resources
			// (logs, keys, clusters, etc.) whose blobs are not user records.
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT DISTINCT r.resource_type FROM resources r
				WHERE r.resource_type IN (SELECT DISTINCT resource_type FROM resources_fts)
				ORDER BY r.resource_type`)
			if err != nil {
				return fmt.Errorf("listing synced resource types: %w", err)
			}
			var types []string
			for rows.Next() {
				var t string
				if err := rows.Scan(&t); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan resource type: %w", err)
				}
				if nonRecordResourceTypes[t] {
					continue
				}
				types = append(types, t)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate resource types: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("close resource types: %w", err)
			}

			if flagMaxScanPages <= 0 {
				flagMaxScanPages = 5
			}
			hits := make([]findHit, 0)
			scanned := 0
			scanCapped := false
			typesScanned := 0
			for _, t := range types {
				if typesScanned >= flagMaxScanPages {
					scanCapped = true
					break
				}
				typesScanned++
				partial, searchErr := db.Search(query, flagLimit, t)
				if searchErr != nil {
					continue
				}
				for _, raw := range partial {
					var obj map[string]any
					if json.Unmarshal(raw, &obj) != nil {
						continue
					}
					objectID, _ := obj["objectID"].(string)
					if objectID == "" {
						objectID, _ = obj["id"].(string)
					}
					title, _ := obj["title"].(string)
					if title == "" {
						title, _ = obj["name"].(string)
					}
					scanned++
					hits = append(hits, findHit{
						Index:    t,
						ObjectID: objectID,
						Title:    title,
					})
					if len(hits) >= flagLimit {
						break
					}
				}
				if len(hits) >= flagLimit {
					break
				}
			}

			res := findResult{Query: query, Hits: hits, Scanned: scanned, MaxScanPages: flagMaxScanPages}
			if scanCapped {
				res.Note = fmt.Sprintf("scan cap reached at %d resource types; raise --max-scan-pages to search more", flagMaxScanPages)
			}
			if len(hits) == 0 {
				res.Note = "no hits found in any synced index; run 'algolia-pp-cli sync' to refresh the local mirror"
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if len(hits) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No hits found in any synced index.")
				return nil
			}
			for _, h := range hits {
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s (%s)\n", h.Index, h.Title, h.ObjectID)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "%d hits across %d synced index types\n", len(hits), len(types))
			return nil
		},
	}
	cmd.Flags().StringVar(&flagQuery, "query", "", "Search query to match across all synced indices")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum hits to return (default 20)")
	cmd.Flags().IntVar(&flagMaxScanPages, "max-scan-pages", 5, "Maximum resource types to scan before returning partial or empty results")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}
