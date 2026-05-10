package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// newPublishersCmd returns the `publishers` command tree. Today there is one
// subcommand: `publishers top` for cross-publisher gravitas ranking.
func newPublishersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "publishers",
		Short:       "Compare publishers across the public API network",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newPublishersTopCmd(flags))
	return cmd
}

// newPublishersTopCmd ranks publishers by aggregate fork count across all
// their synced entities. Optional category narrows to a single category id.
func newPublishersTopCmd(flags *rootFlags) *cobra.Command {
	var categoryRef string
	var entityType string
	var limit int
	var minEntities int
	cmd := &cobra.Command{
		Use:         "top",
		Short:       "Rank publishers by aggregate fork count across their entities",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Aggregate forkCount across every entity owned by each publisher in the
local store, then rank publishers by total forks. Useful for picking a vendor
within a category or finding the most-forked teams overall.

Run 'sync' first; this command reads the local store.`,
		Example: strings.Trim(`
  # Top 10 publishers by aggregate fork count
  postman-explore-pp-cli publishers top --limit 10

  # Top publishers within a single category
  postman-explore-pp-cli publishers top --category 7 --limit 5 --json

  # Restrict to publishers with at least 3 entities (drops one-collection users)
  postman-explore-pp-cli publishers top --min-entities 3`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if entityType != "" && !validEntityType(entityType) {
				return usageErr(fmt.Errorf("invalid --type %q (use one of collection, workspace, api, flow, or omit for all)", entityType))
			}
			db, err := openLocalStore(flags)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			extraWhere := ""
			if categoryRef != "" {
				resolver, err := newCategoryResolverFromFlags(flags)
				if err != nil {
					return err
				}
				categoryID, err := resolveCategoryID(resolver, categoryRef)
				if err != nil {
					return usageErr(err)
				}
				if categoryID > 0 {
					extraWhere = `data LIKE '%"id":` + fmt.Sprintf("%d", categoryID) + `,%'`
				}
			}
			var items []json.RawMessage
			if entityType != "" {
				items, err = queryNetworkEntities(db, entityType, extraWhere)
			} else {
				items, err = queryAllNetworkEntities(db, extraWhere)
			}
			if err != nil {
				return err
			}

			type pub struct {
				ID       string `json:"publisherId"`
				Forks    int64  `json:"totalForkCount"`
				Entities int    `json:"entities"`
			}
			byID := map[string]*pub{}
			for _, raw := range items {
				pid := entityPublisherID(raw)
				if pid == "" {
					continue
				}
				p, ok := byID[pid]
				if !ok {
					p = &pub{ID: pid}
					byID[pid] = p
				}
				p.Entities++
				p.Forks += extractMetricValue(raw, "forkCount")
			}
			out := make([]pub, 0, len(byID))
			for _, p := range byID {
				if p.Entities < minEntities {
					continue
				}
				out = append(out, *p)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Forks > out[j].Forks })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}

			handles := lookupPublisherHandles(db.DB())
			type rich struct {
				PublisherID    string `json:"publisherId"`
				PublicHandle   string `json:"publicHandle,omitempty"`
				Entities       int    `json:"entities"`
				TotalForkCount int64  `json:"totalForkCount"`
			}
			res := make([]rich, 0, len(out))
			for _, p := range out {
				res = append(res, rich{
					PublisherID:    p.ID,
					PublicHandle:   handles[p.ID],
					Entities:       p.Entities,
					TotalForkCount: p.Forks,
				})
			}
			if len(res) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd, []any{}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), emptyMessage("no publishers in synced data — run 'sync' first"))
				return nil
			}
			if flags.asJSON {
				return printJSONFiltered(cmd, res, flags)
			}
			tbl := make([][]string, 0, len(res))
			for i, r := range res {
				tbl = append(tbl, []string{
					fmt.Sprintf("%d", i+1),
					r.PublicHandle,
					r.PublisherID,
					fmt.Sprintf("%d", r.Entities),
					fmt.Sprintf("%d", r.TotalForkCount),
				})
			}
			return flags.printTable(cmd, []string{"#", "Handle", "PublisherID", "Entities", "TotalForks"}, tbl)
		},
	}
	cmd.Flags().StringVar(&categoryRef, "category", "", "Category to narrow results — accepts a slug (developer-productivity) or a numeric id (4)")
	cmd.Flags().StringVar(&entityType, "type", "", "Restrict aggregation to a single entity type (default: all)")
	cmd.Flags().IntVar(&limit, "limit", 10, "How many publishers to return")
	cmd.Flags().IntVar(&minEntities, "min-entities", 1, "Drop publishers with fewer than this many synced entities")
	return cmd
}

// lookupPublisherHandles returns publisherId → publicHandle from synced team
// rows in the generic resources table. Best-effort: returns an empty map if
// no team rows are synced. The fallback to scanning entity payloads catches
// publishers whose team profile wasn't synced — many entity responses
// include the publisher's `publicHandle` directly under `meta`.
func lookupPublisherHandles(db *sql.DB) map[string]string {
	out := map[string]string{}
	rows, err := db.Query(`SELECT id, data FROM resources WHERE resource_type = 'team'`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, dataText string
			if err := rows.Scan(&id, &dataText); err != nil {
				continue
			}
			var doc struct {
				PublicHandle string `json:"publicHandle"`
			}
			if err := json.Unmarshal([]byte(dataText), &doc); err == nil && doc.PublicHandle != "" {
				out[id] = doc.PublicHandle
			}
		}
	}
	// Fallback: scan entity rows for embedded publisher handles
	entityRows, err := db.Query(`SELECT data FROM resources WHERE resource_type IN ('collection','workspace','api','flow')`)
	if err != nil {
		return out
	}
	defer entityRows.Close()
	for entityRows.Next() {
		var dataText string
		if err := entityRows.Scan(&dataText); err != nil {
			continue
		}
		var doc struct {
			Meta struct {
				PublisherID   string `json:"publisherId"`
				PublicHandle  string `json:"workspaceSlug"`
				PublisherType string `json:"publisherType"`
			} `json:"meta"`
		}
		if err := json.Unmarshal([]byte(dataText), &doc); err != nil {
			continue
		}
		if doc.Meta.PublisherID != "" && doc.Meta.PublicHandle != "" && out[doc.Meta.PublisherID] == "" {
			out[doc.Meta.PublisherID] = doc.Meta.PublicHandle
		}
	}
	return out
}
