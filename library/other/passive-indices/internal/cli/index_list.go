// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newIndexListCmd(flags *rootFlags) *cobra.Command {
	var filter string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List every published NSE index (name + category), optionally filtered by name substring",
		Example: "  passive-indices-pp-cli index list --filter bank --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list indices")
				return nil
			}

			if flags.dataSource == "local" {
				return listIndicesFromStore(cmd, flags, filter)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newNiftyIndicesClient(flags)
			quotes, err := c.LiveWatch(ctx)
			if err != nil {
				if flags.dataSource == "auto" {
					if storeErr := listIndicesFromStore(cmd, flags, filter); storeErr == nil {
						return nil
					}
				}
				return classifyAPIError(err, flags)
			}

			if db, _, openErr := openStoreForCache(flags); openErr == nil {
				for _, q := range quotes {
					if data, err := json.Marshal(q); err == nil {
						_ = db.Upsert(resourceTypeIndexQuote, q.IndexName, data)
					}
				}
				db.Close()
			}

			names := make([]map[string]string, 0, len(quotes))
			for _, q := range quotes {
				if filter != "" && !strings.Contains(strings.ToLower(q.IndexName), strings.ToLower(filter)) {
					continue
				}
				names = append(names, map[string]string{
					"index_name": q.IndexName,
					"category":   q.Category,
				})
			}
			return flags.printJSON(cmd, names)
		},
	}
	cmd.Flags().StringVar(&filter, "filter", "", "substring filter on index name")
	return cmd
}

func listIndicesFromStore(cmd *cobra.Command, flags *rootFlags, filter string) error {
	dbPath := defaultDBPath("passive-indices-pp-cli")
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: passive-indices-pp-cli index list (live) to populate the cache\n", dbPath)
		if flags.asJSON || flags.agent {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
		}
		return nil
	}
	db, _, err := openStoreForCache(flags)
	if err != nil {
		return err
	}
	defer db.Close()
	rows, err := db.List(resourceTypeIndexQuote, 0)
	if err != nil {
		return err
	}
	names := make([]map[string]string, 0, len(rows))
	for _, raw := range rows {
		var q struct {
			IndexName string `json:"indexName"`
			Category  string `json:"category"`
		}
		if err := json.Unmarshal(raw, &q); err != nil {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(q.IndexName), strings.ToLower(filter)) {
			continue
		}
		names = append(names, map[string]string{
			"index_name": q.IndexName,
			"category":   q.Category,
		})
	}
	return flags.printJSON(cmd, names)
}
