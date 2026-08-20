// Copyright 2026 Greg Cole and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: compound workflows. `archive` syncs the current week's menu
// (delegates to the CookUnity SDUI sync); `status` reports local archive state.
// Preserved on regen.
package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookunity/internal/store"

	"github.com/spf13/cobra"
)

func newWorkflowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "workflow",
		Short:       "Compound workflows that combine multiple API operations",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWorkflowArchiveCmd(flags))
	cmd.AddCommand(newWorkflowStatusCmd(flags))
	return cmd
}

// pp:data-source live
func newWorkflowArchiveCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var full bool
	var dateFlag string

	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Sync the current week's menu to the local store for offline access and search",
		Long: `Archive fetches the CookUnity weekly menu and stores it in a local SQLite
database. After archiving, use 'search', 'meals', or 'plan' offline.`,
		Example: `  # Archive the next delivery week's menu
  cookunity-pp-cli workflow archive

  # Archive a specific delivery date
  cookunity-pp-cli workflow archive --date 2026-08-04`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would archive the CookUnity menu into the local store")
				return nil
			}
			date := resolveSyncDate(dateFlag, nil)
			stored, resolvedDB, err := runCookunitySync(ctx, flags, date, dbPath)
			if err != nil {
				return err
			}
			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"resources_synced": 1,
					"total_items":      stored,
					"delivery_date":    date,
					"store_path":       resolvedDB,
					"timestamp":        time.Now().UTC().Format(time.RFC3339),
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Archived %d meals for %s to %s\n", stored, date, resolvedDB)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: standard local path)")
	cmd.Flags().BoolVar(&full, "full", false, "Full re-archive (the menu is always fetched in full)")
	cmd.Flags().StringVar(&dateFlag, "date", "", "Delivery date to archive, YYYY-MM-DD (default: next Monday)")
	return cmd
}

func newWorkflowStatusCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Show local archive status and sync state for all resources",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Show archive status
  cookunity-pp-cli workflow status

  # Show status as JSON
  cookunity-pp-cli workflow status --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("cookunity-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			status, err := s.Status()
			if err != nil {
				return err
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(status)
			}

			if len(status) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No archived data. Run 'workflow archive' to sync.")
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Archive Status:")
			total := 0
			for resource, count := range status {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-30s %d items\n", resource, count)
				total += count
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n  Total: %d items\n", total)
			fmt.Fprintf(cmd.OutOrStdout(), "  Store: %s\n", dbPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: standard local path)")
	return cmd
}
