// Copyright 2026 bernard-maltais. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/ai/artyfacts/internal/store"
	"github.com/spf13/cobra"
)

func newArtifactsTreeCmd(flags *rootFlags) *cobra.Command {
	var flagDBPath string
	var flagDepth int
	var flagFolderID string

	cmd := &cobra.Command{
		Use:   "tree [folder-id]",
		Short: "Display the artifact hierarchy as an ASCII tree",
		Long: `Recursively renders the folder hierarchy from the local store.
Shows artifact type, status, and section count for each node.

Requires a prior 'sync' to populate the local store.`,
		Example:     "  artyfacts-pp-cli artifacts tree\n  artyfacts-pp-cli artifacts tree <folder-id> --depth 3",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				flagFolderID = args[0]
			}

			dbPath := flagDBPath
			if dbPath == "" {
				dbPath = store.DefaultPath()
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w\nRun 'sync' first.", err)
			}
			defer db.Close()

			if flags.asJSON {
				return printTreeJSON(cmd, db, flagFolderID, flagDepth, flags)
			}

			// Get root or folder children
			var roots []store.ArtifactRow
			if flagFolderID == "" {
				roots, err = db.ListArtifacts("", "", "", true)
			} else {
				roots, err = db.ChildArtifacts(flagFolderID)
			}
			if err != nil {
				return err
			}

			if len(roots) == 0 {
				if flagFolderID == "" {
					fmt.Fprintln(cmd.OutOrStdout(), "(empty workspace — run 'sync' to populate)")
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "(no children for folder %s)\n", flagFolderID)
				}
				return nil
			}

			maxDepth := flagDepth
			if maxDepth <= 0 {
				maxDepth = 5
			}

			printTreeNodes(cmd, db, roots, "", 0, maxDepth)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagDBPath, "db", "", "Custom SQLite database path")
	cmd.Flags().IntVar(&flagDepth, "depth", 5, "Maximum nesting depth")

	return cmd
}

func printTreeNodes(cmd *cobra.Command, db *store.DB, rows []store.ArtifactRow, prefix string, depth, maxDepth int) {
	for i, r := range rows {
		isLast := i == len(rows)-1
		connector := "├── "
		childPrefix := prefix + "│   "
		if isLast {
			connector = "└── "
			childPrefix = prefix + "    "
		}

		typeTag := r.ArtifactType
		if typeTag == "" {
			typeTag = "?"
		}
		statusTag := ""
		if r.Status != "" && r.Status != "active" {
			statusTag = " [" + r.Status + "]"
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%s%s%s  (%s)%s  %s\n",
			prefix, connector, r.Title, typeTag, statusTag, truncateStr(r.ID, 8))

		if depth+1 < maxDepth && r.ArtifactType == "folder" {
			children, err := db.ChildArtifacts(r.ID)
			if err == nil && len(children) > 0 {
				printTreeNodes(cmd, db, children, childPrefix, depth+1, maxDepth)
			}
		}
	}
}

func printTreeJSON(cmd *cobra.Command, db *store.DB, folderID string, maxDepth int, flags *rootFlags) error {
	if maxDepth <= 0 {
		maxDepth = 5
	}
	var roots []store.ArtifactRow
	var err error
	if folderID == "" {
		roots, err = db.ListArtifacts("", "", "", true)
	} else {
		roots, err = db.ChildArtifacts(folderID)
	}
	if err != nil {
		return err
	}
	nodes := buildTreeJSON(db, roots, 0, maxDepth)
	b, _ := json.Marshal(nodes)
	return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
}

func buildTreeJSON(db *store.DB, rows []store.ArtifactRow, depth, maxDepth int) []map[string]any {
	var out []map[string]any
	for _, r := range rows {
		node := map[string]any{
			"id":    r.ID,
			"title": r.Title,
			"type":  r.ArtifactType,
		}
		if r.Status != "" {
			node["status"] = r.Status
		}
		if depth+1 < maxDepth && r.ArtifactType == "folder" {
			children, err := db.ChildArtifacts(r.ID)
			if err == nil && len(children) > 0 {
				node["children"] = buildTreeJSON(db, children, depth+1, maxDepth)
			}
		}
		out = append(out, node)
	}
	return out
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

