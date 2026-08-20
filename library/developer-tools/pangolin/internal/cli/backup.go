// Copyright 2026 cfinney. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature for pangolin-pp-cli.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/pangolin/internal/store"
)

type backupFile struct {
	Schema       string                       `json:"schema"`
	Version      int                          `json:"version"`
	GeneratedAt  string                       `json:"generated_at"`
	GeneratedBy  string                       `json:"generated_by"`
	ResourceSets map[string][]json.RawMessage `json:"resource_sets"`
	Counts       map[string]int               `json:"counts"`
}

// backupResourceTypes lists every resource_type the backup command exports.
// Order matters for the restore command: parents first, dependents last.
var backupResourceTypes = []string{
	"orgs", "idp", "users",
	"sites", "resources", "site_resources",
	"target", "client", "role", "certificate", "domains",
	"org_users", "org_roles", "org_idp", "org_domains",
	"org_access_tokens", "openapi-json", "openapi-yaml",
}

func newBackupCmd(flags *rootFlags) *cobra.Command {
	var outPath string
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Export full Pangolin configuration as a version-controllable JSON snapshot.",
		Long: `backup walks every resource_type in the local store, serialises each row's
JSON to a single backup file, and writes the result to stdout (or --out).

Run 'sync --full' first to ensure the store is current. The output is stable
enough to commit to git and diff between dates for change tracking.`,
		Example: "  pangolin-pp-cli backup --out pangolin-backup.json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("pangolin-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			snap := backupFile{
				Schema:       "pangolin-pp-cli/backup",
				Version:      1,
				GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
				GeneratedBy:  "pangolin-pp-cli backup",
				ResourceSets: map[string][]json.RawMessage{},
				Counts:       map[string]int{},
			}
			for _, rt := range backupResourceTypes {
				rows, qerr := db.DB().QueryContext(cmd.Context(),
					`SELECT data FROM resources WHERE resource_type = ?`, rt)
				if qerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warn: skipping %s: query failed: %v\n", rt, qerr)
					continue
				}
				items := []json.RawMessage{}
				for rows.Next() {
					var data sql.NullString
					if rows.Scan(&data) == nil && data.String != "" {
						items = append(items, json.RawMessage(data.String))
					}
				}
				if rerr := rows.Err(); rerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warn: cursor error reading %s: %v\n", rt, rerr)
				}
				rows.Close()
				if len(items) > 0 {
					snap.ResourceSets[rt] = items
					snap.Counts[rt] = len(items)
				}
			}

			out, err := json.MarshalIndent(snap, "", "  ")
			if err != nil {
				return fmt.Errorf("marshalling backup: %w", err)
			}
			if outPath == "" || outPath == "-" {
				_, werr := cmd.OutOrStdout().Write(append(out, '\n'))
				return werr
			}
			if err := os.WriteFile(outPath, append(out, '\n'), 0o600); err != nil {
				return fmt.Errorf("writing %s: %w", outPath, err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Wrote backup: %s (%d resource types, %d bytes)\n",
				outPath, len(snap.ResourceSets), len(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&outPath, "out", "", "Output path (default stdout)")
	return cmd
}
