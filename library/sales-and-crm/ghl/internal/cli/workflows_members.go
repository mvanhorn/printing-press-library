// Copyright 2026 alex-puckhaber. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/ghl/internal/store"

	"github.com/spf13/cobra"
)

// newWorkflowsMembersCmd derives "contacts currently enrolled in this workflow"
// from the locally synced contacts table. GHL has no public list-membership
// endpoint; we look for the `workflowId` (or `workflows[]` membership array)
// on each cached contact.
func newWorkflowsMembersCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:         "members <workflow-id>",
		Short:       "List contacts whose synced record references the given workflow",
		Long:        "GHL has no public membership endpoint. This command searches the locally synced contacts for those whose `workflowId` field or `workflows[]` array references the given workflow id. Run 'ghl-pp-cli sync --full' first for accurate results.",
		Example:     "  ghl-pp-cli workflows members wf_abc123 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			workflowID := strings.TrimSpace(args[0])
			if dbPath == "" {
				dbPath = defaultDBPath("ghl-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'ghl-pp-cli sync' first", err)
			}
			defer db.Close()

			rows, err := db.Query(`SELECT id, data FROM "contacts"`)
			if err != nil {
				return fmt.Errorf("querying contacts: %w", err)
			}
			defer rows.Close()

			type member struct {
				ID    string `json:"id"`
				Name  string `json:"name,omitempty"`
				Phone string `json:"phone,omitempty"`
			}
			var hits []member
			for rows.Next() {
				if limit > 0 && len(hits) >= limit {
					break
				}
				var id string
				var data []byte
				if err := rows.Scan(&id, &data); err != nil {
					continue
				}
				if !contactReferencesWorkflow(data, workflowID) {
					continue
				}
				hits = append(hits, member{
					ID:    id,
					Name:  strings.TrimSpace(extractStr(data, "firstName") + " " + extractStr(data, "lastName")),
					Phone: extractStr(data, "phone"),
				})
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"workflow_id": workflowID, "count": len(hits), "members": hits}, flags)
			}
			if len(hits) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No synced contacts reference workflow %s.\n", workflowID)
				fmt.Fprintln(cmd.OutOrStdout(), "Hint: GHL has no public membership endpoint. Make sure 'sync --full' has run.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d contact(s) enrolled in workflow %s:\n\n", len(hits), workflowID)
			for _, m := range hits {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %-22s  %s\n", m.ID, m.Name, m.Phone)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/ghl-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 500, "Max rows to return")
	return cmd
}

// contactReferencesWorkflow returns true if the contact's JSON references the
// given workflow id under any of the documented shapes (single workflowId,
// `workflows` array of strings, or `workflows` array of objects with id/wfId).
func contactReferencesWorkflow(data []byte, wfID string) bool {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return false
	}
	if s, ok := obj["workflowId"].(string); ok && s == wfID {
		return true
	}
	if arr, ok := obj["workflows"].([]any); ok {
		for _, w := range arr {
			switch v := w.(type) {
			case string:
				if v == wfID {
					return true
				}
			case map[string]any:
				if id, ok := v["id"].(string); ok && id == wfID {
					return true
				}
				if id, ok := v["wfId"].(string); ok && id == wfID {
					return true
				}
				if id, ok := v["workflowId"].(string); ok && id == wfID {
					return true
				}
			}
		}
	}
	return false
}
