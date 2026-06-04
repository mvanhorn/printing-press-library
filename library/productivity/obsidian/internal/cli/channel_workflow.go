// Copyright 2026 Angelo Pullen and contributors. Licensed under Apache-2.0. See LICENSE.

// Workflow subcommands. The generic Printing Press emits these for SaaS
// APIs that benefit from "archive everything then search offline"
// flows; the obsidian-pp-cli mirror is purpose-built for that, so the
// workflow commands here are thin obsidian-aware wrappers — `archive`
// delegates to the obsidian `sync`, `status` reports vault-mirror state.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/store"
	"github.com/spf13/cobra"
)

func newWorkflowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Compound workflows (archive + status report)",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWorkflowArchiveCmd(flags))
	cmd.AddCommand(newWorkflowStatusCmd(flags))
	return cmd
}

func newWorkflowArchiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Archive the active vault into the local mirror (alias for `sync`)",
		Long: `Walk the open Obsidian vault and populate the local SQLite mirror with
notes, tags, links, and frontmatter. Equivalent to running ` + "`obsidian-pp-cli sync`" + ` —
both share the same implementation; this command remains for compatibility
with the Press's archive/search workflow vocabulary.`,
		Example:     "  obsidian-pp-cli workflow archive",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,4,5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			sync := newSyncCmd(flags)
			sync.SetArgs(args)
			sync.SetOut(cmd.OutOrStdout())
			sync.SetErr(cmd.ErrOrStderr())
			return sync.Execute()
		},
	}
	return cmd
}

func newWorkflowStatusCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Show obsidian mirror coverage (notes, tags, links, last sync)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  obsidian-pp-cli workflow status
  obsidian-pp-cli workflow status --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("obsidian-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()
			if err := s.EnsureObsidianSchema(); err != nil {
				return err
			}

			dbi := s.DB()
			summary := map[string]any{}
			var n int
			_ = dbi.QueryRow(`SELECT COUNT(*) FROM notes`).Scan(&n)
			summary["notes"] = n
			_ = dbi.QueryRow(`SELECT COUNT(DISTINCT tag) FROM obsidian_tags`).Scan(&n)
			summary["distinct_tags"] = n
			_ = dbi.QueryRow(`SELECT COUNT(*) FROM obsidian_links`).Scan(&n)
			summary["links"] = n
			var vaultPath, lastSync string
			_ = dbi.QueryRow(`SELECT vault_path, COALESCE(last_sync_at,'') FROM vault_sync_state WHERE id=1`).Scan(&vaultPath, &lastSync)
			summary["vault_path"] = vaultPath
			summary["last_sync_at"] = lastSync
			summary["store_path"] = dbPath

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(summary)
			}
			if summary["notes"].(int) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Mirror empty. Run 'obsidian-pp-cli sync' to populate.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"Mirror coverage:\n  vault:         %s\n  last sync:     %s\n  notes:         %d\n  distinct tags: %d\n  links:         %d\n  store:         %s\n",
				vaultPath, lastSync, summary["notes"], summary["distinct_tags"], summary["links"], dbPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
