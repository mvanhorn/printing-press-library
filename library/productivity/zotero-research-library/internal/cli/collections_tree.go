// Copyright 2026 Eric Rash and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: cached collection hierarchy in one call.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type collectionNode struct {
	Key      string            `json:"key"`
	Name     string            `json:"name"`
	Items    int               `json:"items,omitempty"`
	Children []*collectionNode `json:"children,omitempty"`
}

func newNovelCollectionsTreeCmd(flags *rootFlags) *cobra.Command {
	var flagCounts bool

	cmd := &cobra.Command{
		Use:         "tree",
		Short:       "Render the full collection hierarchy with per-node item counts in one command",
		Long:        "Assembles the whole collection tree from the local cache — a shape the flat API can only produce with N+1 requests. --counts adds per-collection item counts from the item-collection membership in the cache.",
		Example:     "  zotero-research-library-pp-cli collections tree --counts --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := requireLocalCache(ctx, "zotero-research-library-pp-cli")
			if err != nil {
				return err
			}
			defer db.Close()

			// Dedicated columns are empty on this envelope-nested API; the
			// authoritative fields live in the stored JSON.
			rows, err := db.DB().QueryContext(ctx,
				`SELECT "id",
				        COALESCE(json_extract("data", '$.data.name'), ''),
				        COALESCE(NULLIF(json_extract("data", '$.data.parentCollection'), 'false'), '')
				 FROM "collections"`)
			if err != nil {
				return fmt.Errorf("reading collections: %w", err)
			}
			defer rows.Close()

			nodes := map[string]*collectionNode{}
			parentOf := map[string]string{}
			for rows.Next() {
				var key, name, parent string
				if err := rows.Scan(&key, &name, &parent); err != nil {
					return err
				}
				if key == "" {
					continue
				}
				nodes[key] = &collectionNode{Key: key, Name: name}
				parentOf[key] = parent
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if len(nodes) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No collections in the local cache — run 'sync' first.")
				return nil
			}

			if flagCounts {
				// One aggregate over the JSON membership arrays covers every
				// collection at once.
				crows, err := db.DB().QueryContext(ctx,
					`SELECT je.value, COUNT(*)
					 FROM "items" i, json_each(COALESCE(json_extract(i."data", '$.data.collections'), '[]')) je
					 GROUP BY je.value`)
				if err != nil {
					return fmt.Errorf("counting collection items: %w", err)
				}
				defer crows.Close()
				for crows.Next() {
					var key string
					var n int
					if err := crows.Scan(&key, &n); err != nil {
						return err
					}
					if node, ok := nodes[key]; ok {
						node.Items = n
					}
				}
				if err := crows.Err(); err != nil {
					return err
				}
			}

			// "false"/"" both mean top-level in the Zotero API's
			// parentCollection field.
			var roots []*collectionNode
			for key, node := range nodes {
				parent := parentOf[key]
				if p, ok := nodes[parent]; ok && parent != "" && parent != "false" {
					p.Children = append(p.Children, node)
				} else {
					roots = append(roots, node)
				}
			}
			sortCollectionNodes(roots)

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"collections": len(nodes),
					"tree":        roots,
				}, flags)
			}
			out := cmd.OutOrStdout()
			var render func(ns []*collectionNode, depth int)
			render = func(ns []*collectionNode, depth int) {
				for _, n := range ns {
					for i := 0; i < depth; i++ {
						fmt.Fprint(out, "  ")
					}
					fmt.Fprintf(out, "%s", n.Name)
					if flagCounts {
						fmt.Fprintf(out, " (%d)", n.Items)
					}
					fmt.Fprintf(out, "  [%s]\n", n.Key)
					render(n.Children, depth+1)
				}
			}
			render(roots, 0)
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagCounts, "counts", false, "Include per-collection item counts")
	return cmd
}

func sortCollectionNodes(ns []*collectionNode) {
	sort.Slice(ns, func(a, b int) bool { return ns[a].Name < ns[b].Name })
	for _, n := range ns {
		sortCollectionNodes(n.Children)
	}
}
