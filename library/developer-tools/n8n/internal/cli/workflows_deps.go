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

type workflowDep struct {
	WorkflowID   string   `json:"workflow_id"`
	WorkflowName string   `json:"workflow_name"`
	CallsIDs     []string `json:"calls_workflow_ids"`
	CallsNames   []string `json:"calls_workflow_names,omitempty"`
	CalledByIDs  []string `json:"called_by_workflow_ids,omitempty"`
}

func newWorkflowsDepsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var workflowID string

	cmd := &cobra.Command{
		Use:   "deps [workflow-id]",
		Short: "Show workflow dependency map via executeWorkflow nodes",
		Long: `Map which workflows call other workflows via the n8n executeWorkflow node.
Use this to understand execution chains before modifying or deleting a workflow,
or to find orphaned workflows that no other workflow calls.
Requires a local sync (n8n-pp-cli sync) first.`,
		Example: strings.Trim(`
  # Show full dependency map
  n8n-pp-cli workflows deps

  # Show deps for a specific workflow
  n8n-pp-cli workflows deps abc123

  # JSON for downstream tooling
  n8n-pp-cli workflows deps --json --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				workflowID = args[0]
			}
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

			// id->name index
			idToName := map[string]string{}
			// id->[]calledIDs
			type wfRecord struct {
				id   string
				name string
				data []byte
			}
			var allWFs []wfRecord
			for rows.Next() {
				var r wfRecord
				if err := rows.Scan(&r.id, &r.name, &r.data); err != nil {
					continue
				}
				idToName[r.id] = r.name
				allWFs = append(allWFs, r)
			}
			if rows.Err() != nil {
				return fmt.Errorf("scanning rows: %w", rows.Err())
			}

			// calledBy[targetID] = []callerIDs
			calledBy := map[string][]string{}
			deps := map[string]*workflowDep{}

			for _, r := range allWFs {
				var wfData map[string]json.RawMessage
				if err := json.Unmarshal(r.data, &wfData); err != nil {
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
				var calledIDs []string
				for _, node := range nodes {
					typeRaw, ok := node["type"]
					if !ok {
						continue
					}
					var nodeType string
					if err := json.Unmarshal(typeRaw, &nodeType); err != nil {
						continue
					}
					if nodeType != "n8n-nodes-base.executeWorkflow" {
						continue
					}
					// Look for workflowId in parameters
					paramsRaw, ok := node["parameters"]
					if !ok {
						continue
					}
					var params map[string]json.RawMessage
					if err := json.Unmarshal(paramsRaw, &params); err != nil {
						continue
					}
					if idRaw, ok := params["workflowId"]; ok {
						var targetID string
						if err := json.Unmarshal(idRaw, &targetID); err == nil && targetID != "" {
							calledIDs = append(calledIDs, targetID)
							calledBy[targetID] = append(calledBy[targetID], r.id)
						}
					}
				}
				if len(calledIDs) > 0 || workflowID == "" {
					deps[r.id] = &workflowDep{
						WorkflowID:   r.id,
						WorkflowName: r.name,
						CallsIDs:     calledIDs,
					}
				}
			}

			// Resolve names for called IDs
			for _, d := range deps {
				for _, cid := range d.CallsIDs {
					if n, ok := idToName[cid]; ok {
						d.CallsNames = append(d.CallsNames, n)
					} else {
						d.CallsNames = append(d.CallsNames, cid)
					}
				}
			}

			// Attach calledBy
			for id, callers := range calledBy {
				if d, ok := deps[id]; ok {
					d.CalledByIDs = callers
				} else {
					deps[id] = &workflowDep{
						WorkflowID:   id,
						WorkflowName: idToName[id],
						CalledByIDs:  callers,
					}
				}
			}

			result := make([]workflowDep, 0, len(deps))
			for _, d := range deps {
				if workflowID != "" && d.WorkflowID != workflowID {
					continue
				}
				result = append(result, *d)
			}
			sort.Slice(result, func(i, j int) bool {
				return result[i].WorkflowName < result[j].WorkflowName
			})

			if len(result) == 0 {
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
					return nil
				}
				if workflowID != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "No executeWorkflow dependencies found for %s\n", workflowID)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "No executeWorkflow nodes found. Run 'n8n-pp-cli sync' first.")
				}
				return nil
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	return cmd
}
