// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call through clGet in courtlistener_novel_support.go

package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/other/courtlistener/internal/store"
	"github.com/spf13/cobra"
)

func newNovelNewFilingsCmd(flags *rootFlags) *cobra.Command {
	var query, searchType string

	cmd := &cobra.Command{
		Use:         "new-filings",
		Short:       "Persist a bounded newest-first search observation and report newly observed CourtListener result IDs.",
		Annotations: map[string]string{"mcp:local-write": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if query == "" {
				return errors.New("--query is required")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			response, err := clGet(ctx, flags, "/search/", url.Values{"q": {query}, "type": {searchType}, "page_size": {"100"}, "order_by": {"dateFiled desc"}}, false)
			if err != nil {
				return err
			}
			current := map[string]bool{}
			results := clResults(response)
			for _, row := range results {
				id := ""
				for _, field := range []string{"id", "cluster_id", "docket_id", "absolute_url"} {
					candidate := fmt.Sprint(row[field])
					if candidate != "" && candidate != "<nil>" {
						id = field + ":" + candidate
						break
					}
				}
				if id != "" {
					current[id] = true
				}
			}
			db, err := store.OpenWithContext(ctx, defaultDBPath("courtlistener-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			key := searchType + "|" + query
			previous := map[string]bool{}
			raw, getErr := db.Get("courtlistener-search-watch", key)
			baseline := errors.Is(getErr, sql.ErrNoRows)
			if getErr != nil && !baseline {
				return getErr
			}
			if getErr == nil {
				if err := json.Unmarshal(raw, &previous); err != nil {
					return err
				}
			}
			var added []string
			if !baseline {
				for id := range current {
					if !previous[id] {
						added = append(added, id)
					}
				}
			}
			sort.Strings(added)
			next, _ := json.Marshal(current)
			if err := db.Upsert("courtlistener-search-watch", key, next); err != nil {
				return err
			}
			return emitCL(cmd, flags, "mixed", map[string]any{"query": query, "type": searchType, "baseline_created": baseline, "observed_results": len(results), "identified_results": len(current), "new_result_ids": added, "total_matching": response["count"], "next": response["next"], "complete_observation": response["next"] == nil, "caveats": clCaveats()})
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "CourtListener search query")
	cmd.Flags().StringVar(&searchType, "type", "r", "CourtListener search type, such as r for case law or d for RECAP")
	return cmd
}
