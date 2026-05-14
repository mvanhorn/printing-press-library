// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/n8n/internal/store"
)

type nodeInventoryEntry struct {
	NodeType  string   `json:"node_type"`
	Count     int      `json:"count"`
	Workflows []string `json:"workflows,omitempty"`
}

func newWorkflowsNodeInventoryCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var showWorkflows bool
	var filterType string

	cmd := &cobra.Command{
		Use:   "node-inventory",
		Short: "Aggregate node type usage across all synced workflows",
		Long: `Count how many times each n8n node type appears across your workflows.
Useful for auditing community node dependencies before upgrades, finding
workflows that use a specific node, or mapping your automation footprint.
Requires a local sync (n8n-pp-cli sync) first.`,
		Example: strings.Trim(`
  # Full node type inventory
  n8n-pp-cli workflows node-inventory

  # Show which workflows use each node type
  n8n-pp-cli workflows node-inventory --show-workflows

  # Filter to a specific node type
  n8n-pp-cli workflows node-inventory --node n8n-nodes-base.httpRequest

  # JSON output
  n8n-pp-cli workflows node-inventory --json --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("n8n-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'n8n-pp-cli sync' first.", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, name, data FROM workflows ORDER BY name`)
			if err != nil {
				return fmt.Errorf("querying workflows: %w\nRun 'n8n-pp-cli sync' first.", err)
			}
			defer rows.Close()

			// nodeType -> {count, []workflowNames}
			inventory := map[string]*nodeInventoryEntry{}
			for rows.Next() {
				var id, name string
				var dataBlob []byte
				if err := rows.Scan(&id, &name, &dataBlob); err != nil {
					continue
				}
				var wfData map[string]json.RawMessage
				if err := json.Unmarshal(dataBlob, &wfData); err != nil {
					continue
				}
				nodesRaw, ok := wfData["nodes"]
				if !ok {
					continue
				}
				var nodes []map[string]json.RawMessage
				if err := json.Unmarshal(nodesRaw, &nodes); err != nil {
					continue
				}
				seen := map[string]bool{}
				for _, node := range nodes {
					typeRaw, ok := node["type"]
					if !ok {
						continue
					}
					var nodeType string
					if err := json.Unmarshal(typeRaw, &nodeType); err != nil {
						continue
					}
					if filterType != "" && !strings.Contains(nodeType, filterType) {
						continue
					}
					if _, exists := inventory[nodeType]; !exists {
						inventory[nodeType] = &nodeInventoryEntry{NodeType: nodeType}
					}
					inventory[nodeType].Count++
					if !seen[name] {
						seen[name] = true
						if showWorkflows {
							inventory[nodeType].Workflows = append(inventory[nodeType].Workflows, name)
						}
					}
				}
			}
			if rows.Err() != nil {
				return fmt.Errorf("scanning rows: %w", rows.Err())
			}

			result := make([]nodeInventoryEntry, 0, len(inventory))
			for _, e := range inventory {
				result = append(result, *e)
			}
			sort.Slice(result, func(i, j int) bool {
				return result[i].Count > result[j].Count
			})

			if len(result) == 0 {
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
					return nil
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No workflow data found. Run 'n8n-pp-cli sync' first.")
				return nil
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	cmd.Flags().BoolVar(&showWorkflows, "show-workflows", false, "Include workflow names for each node type")
	cmd.Flags().StringVar(&filterType, "node", "", "Filter to node types containing this string (e.g. httpRequest)")
	return cmd
}
