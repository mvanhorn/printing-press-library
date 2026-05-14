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

type credentialAuditEntry struct {
	CredentialID   string   `json:"credential_id"`
	CredentialName string   `json:"credential_name"`
	CredentialType string   `json:"credential_type"`
	UsedByCount    int      `json:"used_by_count"`
	UsedByNames    []string `json:"used_by_workflow_names,omitempty"`
	IsResolvable   bool     `json:"is_resolvable"`
}

func newCredentialsAuditCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var showUnused bool
	var showWorkflows bool

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit which credentials are used across workflows",
		Long: `Cross-reference every credential in the store with the workflows that reference it.
Surfaces unused credentials (safe to delete), credentials shared across many
workflows (high-impact on rotation), and which workflows would break if a
credential were removed. Requires a local sync (n8n-pp-cli sync) first.`,
		Example: strings.Trim(`
  # Full credential audit
  n8n-pp-cli credentials audit

  # Only show unused credentials
  n8n-pp-cli credentials audit --unused

  # Include workflow names per credential
  n8n-pp-cli credentials audit --show-workflows --json --agent`, "\n"),
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

			// Load all credentials
			credRows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, name, type, is_resolvable FROM credentials ORDER BY name`)
			if err != nil {
				return fmt.Errorf("querying credentials: %w\nRun 'n8n-pp-cli sync' first.", err)
			}
			defer credRows.Close()

			index := map[string]*credentialAuditEntry{}
			for credRows.Next() {
				var e credentialAuditEntry
				var resolvable int
				if err := credRows.Scan(&e.CredentialID, &e.CredentialName, &e.CredentialType, &resolvable); err != nil {
					continue
				}
				e.IsResolvable = resolvable != 0
				index[e.CredentialID] = &e
			}
			if credRows.Err() != nil {
				return fmt.Errorf("scanning credentials: %w", credRows.Err())
			}

			// Load workflows and scan credential references in node parameters
			wfRows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, name, data FROM workflows ORDER BY name`)
			if err != nil {
				return fmt.Errorf("querying workflows: %w", err)
			}
			defer wfRows.Close()

			for wfRows.Next() {
				var wfID, wfName string
				var dataBlob []byte
				if err := wfRows.Scan(&wfID, &wfName, &dataBlob); err != nil {
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
					// Credential references appear in "credentials" field of a node
					credRaw, ok := node["credentials"]
					if !ok {
						continue
					}
					var credMap map[string]map[string]json.RawMessage
					if err := json.Unmarshal(credRaw, &credMap); err != nil {
						continue
					}
					for _, credDetail := range credMap {
						idRaw, ok := credDetail["id"]
						if !ok {
							continue
						}
						var credID string
						if err := json.Unmarshal(idRaw, &credID); err != nil {
							continue
						}
						if seen[credID] {
							continue
						}
						seen[credID] = true
						if e, ok := index[credID]; ok {
							e.UsedByCount++
							if showWorkflows {
								e.UsedByNames = append(e.UsedByNames, wfName)
							}
						}
					}
				}
			}
			if wfRows.Err() != nil {
				return fmt.Errorf("scanning workflows: %w", wfRows.Err())
			}

			result := make([]credentialAuditEntry, 0, len(index))
			for _, e := range index {
				if showUnused && e.UsedByCount > 0 {
					continue
				}
				result = append(result, *e)
			}
			sort.Slice(result, func(i, j int) bool {
				if result[i].UsedByCount != result[j].UsedByCount {
					return result[i].UsedByCount > result[j].UsedByCount
				}
				return result[i].CredentialName < result[j].CredentialName
			})

			if len(result) == 0 {
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
					return nil
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No credentials found. Run 'n8n-pp-cli sync' first.")
				return nil
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	cmd.Flags().BoolVar(&showUnused, "unused", false, "Only show credentials not referenced by any workflow")
	cmd.Flags().BoolVar(&showWorkflows, "show-workflows", false, "Include workflow names for each credential")
	return cmd
}
