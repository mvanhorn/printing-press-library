// PATCH: hand-authored workflow `search` — local substring search over synced
// menu items and stores. Uses the local SQLite store; no live API call.
// See .printing-press-patches.json patch id "workflow-search".

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/jimmy-johns/internal/store"
)

type searchHit struct {
	Type string                 `json:"type"`
	ID   string                 `json:"id"`
	Name string                 `json:"name,omitempty"`
	Data map[string]any         `json:"data,omitempty"`
}

type searchResult struct {
	Query string      `json:"query"`
	Hits  []searchHit `json:"hits"`
	Notes []string    `json:"notes,omitempty"`
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search locally synced menu items and stores by substring match",
		Long: `Substring-search across the local SQLite store of synced menu items and
stores. Returns matching rows tagged by type ("menu" | "stores") with the
underlying JSON payload.

This is pure-local — runs against the store populated by 'sync'. Useful for
agents composing carts without a round-trip to the live API.`,
		Example: `  jimmy-johns-pp-cli search turkey --json
  jimmy-johns-pp-cli search "veggie" --limit 5
  jimmy-johns-pp-cli search "Capitol Hill"`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.TrimSpace(args[0])
			if query == "" {
				return fmt.Errorf("search query is empty")
			}
			if dbPath == "" {
				dbPath = defaultDBPath("jimmy-johns-pp-cli")
			}
			db, err := store.OpenWithContext(context.Background(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			result := searchResult{Query: query}
			like := "%" + strings.ToLower(query) + "%"

			// Domain-typed search via the store's SearchMenu method (FTS-backed).
			// Falls back to substring LIKE for stores below since there's no
			// generated SearchStores helper.
			menuResults, mErr := db.SearchMenu(query, limit)
			if mErr == nil {
				for _, raw := range menuResults {
					var d map[string]any
					_ = json.Unmarshal(raw, &d)
					hit := searchHit{Type: "menu", Data: d}
					if d != nil {
						if id, ok := d["id"].(string); ok {
							hit.ID = id
						}
						if name, ok := d["name"].(string); ok {
							hit.Name = name
						}
					}
					result.Hits = append(result.Hits, hit)
				}
			} else {
				// Fallback: FTS may not be populated yet — try substring match.
				menuRows, fErr := db.DB().QueryContext(cmd.Context(),
					`SELECT id, data FROM menu
					 WHERE LOWER(COALESCE(json_extract(data,'$.name'),'')) LIKE ?
					    OR LOWER(COALESCE(json_extract(data,'$.description'),'')) LIKE ?
					 LIMIT ?`, like, like, limit)
				if fErr == nil {
					defer menuRows.Close()
					for menuRows.Next() {
						var id, raw string
						if err := menuRows.Scan(&id, &raw); err == nil {
							var d map[string]any
							_ = json.Unmarshal([]byte(raw), &d)
							hit := searchHit{Type: "menu", ID: id, Data: d}
							if d != nil {
								if name, ok := d["name"].(string); ok {
									hit.Name = name
								}
							}
							result.Hits = append(result.Hits, hit)
						}
					}
				}
			}

			// Search stores by name/city.
			storeRows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, data FROM stores
				 WHERE LOWER(COALESCE(json_extract(data,'$.name'),'')) LIKE ?
				    OR LOWER(COALESCE(json_extract(data,'$.city'),'')) LIKE ?
				    OR LOWER(COALESCE(json_extract(data,'$.address'),'')) LIKE ?
				 LIMIT ?`, like, like, like, limit)
			if err == nil {
				defer storeRows.Close()
				for storeRows.Next() {
					var id, raw string
					if err := storeRows.Scan(&id, &raw); err == nil {
						var d map[string]any
						_ = json.Unmarshal([]byte(raw), &d)
						hit := searchHit{Type: "stores", ID: id, Data: d}
						if d != nil {
							if name, ok := d["name"].(string); ok {
								hit.Name = name
							}
						}
						result.Hits = append(result.Hits, hit)
					}
				}
			}

			if len(result.Hits) == 0 {
				result.Notes = append(result.Notes,
					"no matches — has 'sync' been run to populate the local store?")
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Search %q: %d hit(s)\n", query, len(result.Hits))
			for _, h := range result.Hits {
				fmt.Fprintf(w, "  [%s] %s — %s\n", h.Type, h.ID, h.Name)
			}
			for _, n := range result.Notes {
				fmt.Fprintf(w, "Note: %s\n", n)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite store (defaults to the user cache dir)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum number of search hits to return per resource type")
	return cmd
}
