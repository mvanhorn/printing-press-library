// Copyright 2026 Vincent Lauriat and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/notion/internal/store"
)

func newNovelBulkCmd(flags *rootFlags) *cobra.Command {
	var flagDb string
	var flagFilter []string
	var flagSet string

	cmd := &cobra.Command{
		Use:   "bulk <database-id>",
		Short: "Update hundreds of Notion records matching a filter in seconds — no clicking.",
		Long: `Batch-update Notion pages in a database. Finds matching records via the local
store, then patches each via the live Notion API.

--filter key=value        Filter by property (repeatable). Matches select.name, status.name, checkbox.
--set '{"Key":{...}}'     Raw Notion properties JSON to apply to each matched page.

Requires NOTION_TOKEN. Run 'notion-pp-cli sync' first to find matching records.

Examples:
  notion-pp-cli bulk <db-id> --filter "Status=In Progress" --set '{"Status":{"select":{"name":"Done"}}}'
  notion-pp-cli bulk <db-id> --filter "Priority=High" --set '{"Priority":{"select":{"name":"Medium"}}}' --dry-run`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if flagSet == "" {
				return usageErr(fmt.Errorf("--set is required (Notion properties JSON, e.g. '{\"Status\":{\"select\":{\"name\":\"Done\"}}}')"))
			}

			var propsUpdate map[string]any
			if err := json.Unmarshal([]byte(flagSet), &propsUpdate); err != nil {
				return usageErr(fmt.Errorf("--set must be valid JSON: %w", err))
			}

			dbID := args[0]
			if flagDb == "" {
				flagDb = defaultDBPath("notion-pp-cli")
			}
			localDB, err := store.OpenReadOnly(flagDb)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'notion-pp-cli sync' first.", err)
			}
			defer localDB.Close()

			// Query: pages whose parent is the given database.
			// Filter is applied in Go to avoid SQL injection from property names.
			rows, err := localDB.DB().QueryContext(cmd.Context(),
				`SELECT id, data FROM resources
				 WHERE resource_type = 'pages'
				 AND (json_extract(data, '$.parent.database_id') = ?
				      OR json_extract(data, '$.parent.database_id') = ?)`,
				dbID, strings.ReplaceAll(dbID, "-", ""),
			)
			if err != nil {
				return fmt.Errorf("query failed: %w", err)
			}
			defer rows.Close()

			type candidate struct {
				id   string
				data []byte
			}
			var candidates []candidate
			for rows.Next() {
				var id string
				var data []byte
				if err := rows.Scan(&id, &data); err != nil {
					continue
				}
				candidates = append(candidates, candidate{id: id, data: data})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("querying candidates: %w", err)
			}

			// Apply Go-side filters against property values.
			var ids []string
			for _, c := range candidates {
				if bulkMatchesFilters(c.data, flagFilter) {
					ids = append(ids, c.id)
				}
			}

			if len(ids) == 0 {
				if flags.asJSON {
					return flags.printJSON(cmd, map[string]any{"updated": 0, "errors": 0, "ids": []string{}})
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "No matching records found.\n")
				return nil
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Found %d record(s) to update.\n", len(ids))

			if !flags.yes && !flags.noInput {
				return usageErr(fmt.Errorf("pass --yes to confirm updating %d records", len(ids)))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type patchResult struct {
				ID    string `json:"id"`
				Error string `json:"error,omitempty"`
			}
			var updated, errCount int
			var results []patchResult

			for _, id := range ids {
				body := map[string]any{"properties": propsUpdate}
				_, _, patchErr := c.Patch("/v1/pages/"+id, body)
				if patchErr != nil {
					errCount++
					results = append(results, patchResult{ID: id, Error: patchErr.Error()})
				} else {
					updated++
					results = append(results, patchResult{ID: id})
				}
			}

			if !flags.asJSON {
				fmt.Fprintf(cmd.ErrOrStderr(), "Updated %d record(s), %d error(s).\n", updated, errCount)
			}
			return flags.printJSON(cmd, map[string]any{
				"updated": updated,
				"errors":  errCount,
				"results": results,
			})
		},
	}
	cmd.Flags().StringVar(&flagDb, "db", "", "Local database path (default: ~/.local/share/github.com/mvanhorn/printing-press-library/library/productivity/notion/data.db)")
	cmd.Flags().StringSliceVar(&flagFilter, "filter", nil, "Filter by property (key=value, repeatable)")
	cmd.Flags().StringVar(&flagSet, "set", "", "Notion properties JSON to apply (e.g. '{\"Status\":{\"select\":{\"name\":\"Done\"}}}')")
	return cmd
}

// bulkMatchesFilters checks whether a page's JSON data satisfies all key=value filter pairs.
// Matches against common Notion property shapes: select.name, status.name, checkbox.
func bulkMatchesFilters(data []byte, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return false
	}
	propsRaw, ok := obj["properties"]
	if !ok {
		return false
	}
	var props map[string]json.RawMessage
	if err := json.Unmarshal(propsRaw, &props); err != nil {
		return false
	}

	for _, f := range filters {
		k, v, ok := strings.Cut(f, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)

		propRaw, ok := props[k]
		if !ok {
			return false
		}
		var prop map[string]json.RawMessage
		if err := json.Unmarshal(propRaw, &prop); err != nil {
			return false
		}

		matched := false
		// select / status
		for _, typeKey := range []string{"select", "status"} {
			if typeRaw, ok := prop[typeKey]; ok {
				var inner map[string]string
				if json.Unmarshal(typeRaw, &inner) == nil {
					if inner["name"] == v {
						matched = true
						break
					}
				}
			}
		}
		// checkbox
		if !matched {
			if cbRaw, ok := prop["checkbox"]; ok {
				var cb bool
				if json.Unmarshal(cbRaw, &cb) == nil {
					cbStr := "false"
					if cb {
						cbStr = "true"
					}
					if cbStr == v || v == "1" && cb || v == "0" && !cb {
						matched = true
					}
				}
			}
		}
		// rich_text plain_text
		if !matched {
			if rtRaw, ok := prop["rich_text"]; ok {
				text := extractRichText(rtRaw)
				if text == v {
					matched = true
				}
			}
		}
		if !matched {
			return false
		}
	}
	return true
}
