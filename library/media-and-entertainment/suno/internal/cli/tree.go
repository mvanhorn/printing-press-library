// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// PATCH: Add local clip lineage tree rendering.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type clipTreeNode struct {
	ID       string         `json:"id"`
	Title    string         `json:"title,omitempty"`
	ParentID string         `json:"parent_id,omitempty"`
	Children []clipTreeNode `json:"children,omitempty"`
}

func newTreeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "tree <clip-id>",
		Short:       "Render a local clip lineage tree",
		Example:     "  suno-pp-cli tree 9baa5d3c-02fb-466d-80f9-a4edfc9f0a65",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := openExistingStore(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if s == nil {
				return notFoundErr(fmt.Errorf("clip %q not found; run 'suno-pp-cli sync' first", args[0]))
			}
			defer s.Close()
			rows, err := s.DB().QueryContext(cmd.Context(), `SELECT id, data FROM resources WHERE resource_type IN ('clip','clips')`)
			if err != nil {
				return fmt.Errorf("querying clips: %w", err)
			}
			defer rows.Close()
			nodes := map[string]*clipTreeNode{}
			children := map[string][]string{}
			for rows.Next() {
				var id, raw string
				if err := rows.Scan(&id, &raw); err != nil {
					return fmt.Errorf("scanning clip: %w", err)
				}
				obj := unmarshalObject(json.RawMessage(raw))
				node := &clipTreeNode{ID: id, Title: clipTitle(obj), ParentID: clipParentID(obj)}
				nodes[id] = node
				if node.ParentID != "" {
					children[node.ParentID] = append(children[node.ParentID], id)
				}
			}
			root := nodes[args[0]]
			if root == nil {
				return notFoundErr(fmt.Errorf("clip %q not found", args[0]))
			}
			var build func(*clipTreeNode, map[string]bool)
			build = func(n *clipTreeNode, seen map[string]bool) {
				if seen[n.ID] {
					return
				}
				seen[n.ID] = true
				for _, childID := range children[n.ID] {
					if child := nodes[childID]; child != nil {
						copied := *child
						build(&copied, seen)
						n.Children = append(n.Children, copied)
					}
				}
			}
			build(root, map[string]bool{})
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), root, flags)
			}
			var render func(clipTreeNode, int)
			render = func(n clipTreeNode, depth int) {
				label := n.ID
				if n.Title != "" && n.Title != n.ID {
					label += "  " + n.Title
				}
				fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("  ", depth)+label)
				for _, child := range n.Children {
					render(child, depth+1)
				}
			}
			render(*root, 0)
			return nil
		},
	}
	return cmd
}
