// Copyright 2026 giuliano-giacaglia. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/figma/internal/store"
	"github.com/spf13/cobra"
)

func newVariablesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "variables",
		Short: "Variable analysis (use 'files variables' for raw API mirrors).",
	}
	cmd.AddCommand(newVariablesExplainCmd(flags))
	return cmd
}

func newVariablesExplainCmd(flags *rootFlags) *cobra.Command {
	var name, dbPath string

	cmd := &cobra.Command{
		Use:   "explain <key>",
		Short: "List every node and component that references a given variable.",
		Long: `Resolve the variable id by name, then scan the cached node tree and component
table to surface every binding. Requires the file's nodes to be cached locally.`,
		Example: strings.Trim(`
  # All references to color/brand/primary
  figma-pp-cli variables explain abc123XyZ --variable color/brand/primary

  # JSON output
  figma-pp-cli variables explain abc123XyZ --variable spacing/md --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run": true, "command": "variables explain"}`)
				return nil
			}
			if strings.TrimSpace(name) == "" {
				return usageErr(fmt.Errorf("--variable is required"))
			}
			key := args[0]

			if dbPath == "" {
				dbPath = defaultDBPath("figma-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nlocal cache empty — run 'figma-pp-cli sync' first", err)
			}
			defer db.Close()

			// 1. Look up variable id by name.
			var varID string
			var varData []byte
			row := db.DB().QueryRowContext(cmd.Context(),
				`SELECT id, data FROM variables WHERE files_id = ? AND json_extract(data, '$.name') = ? LIMIT 1`,
				key, name)
			if err := row.Scan(&varID, &varData); err != nil {
				return fmt.Errorf("variable %q not found in cache for file %s — run 'figma-pp-cli sync files %s' first", name, key, key)
			}
			var varObj map[string]any
			_ = json.Unmarshal(varData, &varObj)

			// 2. Scan nodes for boundVariables that reference varID.
			nodeCount := 0
			_ = db.DB().QueryRowContext(cmd.Context(),
				`SELECT COUNT(*) FROM nodes WHERE files_id = ?`, key).Scan(&nodeCount)
			if nodeCount == 0 {
				return fmt.Errorf("no node tree cached for this file — run 'figma-pp-cli sync files %s' first", key)
			}

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, data FROM nodes WHERE files_id = ?`, key)
			if err != nil {
				return fmt.Errorf("scanning nodes: %w", err)
			}
			defer rows.Close()

			type ref struct {
				NodeID          string `json:"node_id"`
				NodeName        string `json:"node_name,omitempty"`
				BindingProperty string `json:"binding_property"`
			}
			var refs []ref
			for rows.Next() {
				var nid string
				var data []byte
				if err := rows.Scan(&nid, &data); err != nil {
					continue
				}
				var node map[string]any
				_ = json.Unmarshal(data, &node)
				bindings, ok := node["boundVariables"].(map[string]any)
				if !ok {
					continue
				}
				name, _ := node["name"].(string)
				for prop, b := range bindings {
					if matchesVariableID(b, varID) {
						refs = append(refs, ref{NodeID: nid, NodeName: name, BindingProperty: prop})
					}
				}
			}

			// 3. Also scan files_components.
			cRows, cerr := db.DB().QueryContext(cmd.Context(),
				`SELECT id, data FROM files_components WHERE files_id = ?`, key)
			if cerr == nil {
				defer cRows.Close()
				for cRows.Next() {
					var cid string
					var data []byte
					if err := cRows.Scan(&cid, &data); err != nil {
						continue
					}
					var co map[string]any
					_ = json.Unmarshal(data, &co)
					bindings, ok := co["boundVariables"].(map[string]any)
					if !ok {
						continue
					}
					cname, _ := co["name"].(string)
					for prop, b := range bindings {
						if matchesVariableID(b, varID) {
							refs = append(refs, ref{NodeID: cid, NodeName: "component:" + cname, BindingProperty: prop})
						}
					}
				}
			}

			out := map[string]any{
				"variable":        varObj,
				"file_key":        key,
				"variable_id":     varID,
				"referenced_by":   refs,
				"reference_count": len(refs),
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().StringVar(&name, "variable", "", "Variable name (e.g. color/brand/primary)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/figma-pp-cli/data.db)")

	return cmd
}

// matchesVariableID checks any of the common boundVariables shapes for a
// reference to the target variable id. Figma represents bindings as either
// a single VARIABLE_ALIAS object {"type":"VARIABLE_ALIAS","id":"VariableID:..."}
// or a list of them, depending on the property.
func matchesVariableID(v any, target string) bool {
	switch x := v.(type) {
	case map[string]any:
		if id, ok := x["id"].(string); ok && id == target {
			return true
		}
	case []any:
		for _, el := range x {
			if matchesVariableID(el, target) {
				return true
			}
		}
	}
	return false
}
