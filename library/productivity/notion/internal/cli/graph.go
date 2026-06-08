// Copyright 2026 Vincent Lauriat and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/notion/internal/store"
)

type graphNode struct {
	ID       string       `json:"id"`
	Title    string       `json:"title,omitempty"`
	Type     string       `json:"type,omitempty"`
	Children []*graphNode `json:"children,omitempty"`
}

func newNovelGraphCmd(flags *rootFlags) *cobra.Command {
	var flagDepth int
	var flagFormat string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "graph <page-or-database-id>",
		Short: "Follow Notion relation properties across databases to any depth — output as tree, JSON, or Graphviz DOT.",
		Long: `Traverse Notion relation properties starting from a page or database ID.
Reads entirely from the local sync store — no API calls required.
Run 'notion-pp-cli sync' first to populate the local database.

Output formats:
  tree   Indented text tree (default)
  json   JSON tree with nested children arrays
  dot    Graphviz DOT format for use with dot/graphviz

Examples:
  notion-pp-cli graph <page-id>
  notion-pp-cli graph <page-id> --depth 5 --format dot
  notion-pp-cli graph <page-id> --format json --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("notion-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'notion-pp-cli sync' first to populate the local database.", err)
			}
			defer db.Close()

			root, err := buildGraphNode(cmd.Context(), db.DB(), args[0], flagDepth, map[string]bool{})
			if err != nil {
				return fmt.Errorf("building graph: %w", err)
			}
			if root == nil {
				return notFoundErr(fmt.Errorf("page or database %q not found in local store\nRun 'notion-pp-cli sync' to populate.", args[0]))
			}

			switch flagFormat {
			case "json":
				return flags.printJSON(cmd, root)
			case "dot":
				fmt.Fprintln(cmd.OutOrStdout(), "digraph notion {")
				emitDOT(cmd, root)
				fmt.Fprintln(cmd.OutOrStdout(), "}")
			default:
				printGraphTree(cmd, root, 0)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&flagDepth, "depth", 3, "Maximum relation traversal depth")
	cmd.Flags().StringVar(&flagFormat, "format", "tree", "Output format: tree, json, or dot")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/github.com/mvanhorn/printing-press-library/library/productivity/notion/data.db)")
	return cmd
}

func buildGraphNode(ctx context.Context, rawDB *sql.DB, id string, depth int, visited map[string]bool) (*graphNode, error) {
	if depth < 0 || visited[id] {
		return nil, nil
	}
	visited[id] = true

	var data string
	err := rawDB.QueryRowContext(ctx, `SELECT data FROM resources WHERE id = ? LIMIT 1`, id).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return nil, err
	}

	node := &graphNode{ID: id}
	if t, ok := obj["object"]; ok {
		var s string
		if json.Unmarshal(t, &s) == nil {
			node.Type = s
		}
	}
	node.Title = extractNotionTitle(obj)

	if depth == 0 {
		return node, nil
	}

	// Traverse relation properties.
	if propsRaw, ok := obj["properties"]; ok {
		var props map[string]json.RawMessage
		if json.Unmarshal(propsRaw, &props) == nil {
			for _, propVal := range props {
				var prop map[string]json.RawMessage
				if json.Unmarshal(propVal, &prop) != nil {
					continue
				}
				typeRaw, ok := prop["type"]
				if !ok {
					continue
				}
				var propType string
				if json.Unmarshal(typeRaw, &propType) != nil || propType != "relation" {
					continue
				}
				relRaw, ok := prop["relation"]
				if !ok {
					continue
				}
				var relations []map[string]json.RawMessage
				if json.Unmarshal(relRaw, &relations) != nil {
					continue
				}
				for _, rel := range relations {
					idRaw, ok := rel["id"]
					if !ok {
						continue
					}
					var relID string
					if json.Unmarshal(idRaw, &relID) != nil || relID == "" {
						continue
					}
					child, err := buildGraphNode(ctx, rawDB, relID, depth-1, visited)
					if err != nil || child == nil {
						continue
					}
					node.Children = append(node.Children, child)
				}
			}
		}
	}
	return node, nil
}

func extractNotionTitle(obj map[string]json.RawMessage) string {
	if propsRaw, ok := obj["properties"]; ok {
		var props map[string]json.RawMessage
		if json.Unmarshal(propsRaw, &props) == nil {
			for _, k := range []string{"title", "Title", "Name", "name"} {
				if propRaw, ok := props[k]; ok {
					if title := extractTitleFromProp(propRaw); title != "" {
						return title
					}
				}
			}
		}
	}
	if titleRaw, ok := obj["title"]; ok {
		return extractRichText(titleRaw)
	}
	return ""
}

func extractTitleFromProp(raw json.RawMessage) string {
	var prop map[string]json.RawMessage
	if json.Unmarshal(raw, &prop) != nil {
		return ""
	}
	if titleArr, ok := prop["title"]; ok {
		return extractRichText(titleArr)
	}
	return ""
}

func extractRichText(raw json.RawMessage) string {
	var items []map[string]json.RawMessage
	if json.Unmarshal(raw, &items) != nil {
		return ""
	}
	var parts []string
	for _, item := range items {
		if pt, ok := item["plain_text"]; ok {
			var s string
			if json.Unmarshal(pt, &s) == nil && s != "" {
				parts = append(parts, s)
			}
		}
	}
	return strings.Join(parts, "")
}

func printGraphTree(cmd *cobra.Command, node *graphNode, indent int) {
	prefix := strings.Repeat("  ", indent)
	title := node.Title
	if title == "" {
		title = "(untitled)"
	}
	typeLabel := ""
	if node.Type != "" {
		typeLabel = " [" + node.Type + "]"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s%s%s  %s\n", prefix, title, typeLabel, node.ID)
	for _, child := range node.Children {
		printGraphTree(cmd, child, indent+1)
	}
}

func emitDOT(cmd *cobra.Command, node *graphNode) {
	label := node.Title
	if label == "" {
		label = node.ID
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  %q [label=%q];\n", node.ID, label)
	for _, child := range node.Children {
		childLabel := child.Title
		if childLabel == "" {
			childLabel = child.ID
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %q [label=%q];\n", child.ID, childLabel)
		fmt.Fprintf(cmd.OutOrStdout(), "  %q -> %q;\n", node.ID, child.ID)
		emitDOT(cmd, child)
	}
}
