// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/store"
	"github.com/spf13/cobra"
)

func newWorkflowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Compound workflows that combine multiple API operations",
	}
	cmd.AddCommand(newWorkflowArchiveCmd(flags))
	cmd.AddCommand(newWorkflowStatusCmd(flags))

	return cmd
}
func newWorkflowArchiveCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var full bool

	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Sync all resources to local store for offline access and search",
		Example: `  # Archive all resources
  gorgias-pp-cli workflow archive

  # Full re-archive (ignore previous sync state)
  gorgias-pp-cli workflow archive --full`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true

			if dbPath == "" {
				dbPath = defaultDBPath("gorgias-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			resources := []string{"account", "customers", "events", "gorgias-jobs", "integrations", "macros", "messages", "phone", "phone-voice-call-events", "phone-voice-call-recordings", "rules", "satisfaction-surveys", "tags", "teams", "tickets", "users", "views", "widgets"}
			totalSynced := 0

			// --full clears the cursor here because syncResource reads
			// existingCursor unconditionally; its full param only gates the
			// since filter, not cursor reset. Mirrors newSyncCmd's pattern.
			if full {
				for _, resource := range resources {
					_ = s.SaveSyncState(resource, "", 0)
				}
			}

			for _, resource := range resources {
				res := syncResource(c, s, resource, "", full, 100, false, nil)
				if res.Err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "  %s: error: %v\n", resource, res.Err)
					continue
				}
				if res.Warn != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "  %s: warning: %v\n", resource, res.Warn)
					continue
				}
				totalSynced += res.Count
				fmt.Fprintf(cmd.ErrOrStderr(), "  %s: %d synced\n", resource, res.Count)
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"resources_synced": len(resources),
					"total_items":      totalSynced,
					"store_path":       dbPath,
					"timestamp":        time.Now().UTC().Format(time.RFC3339),
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Archived %d items across %d resources to %s\n", totalSynced, len(resources), dbPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/gorgias-pp-cli/data.db)")
	cmd.Flags().BoolVar(&full, "full", false, "Full re-archive (ignore previous sync state)")

	return cmd
}

func newWorkflowStatusCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Show local archive status and sync state for all resources",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Show archive status
  gorgias-pp-cli workflow status

  # Show status as JSON
  gorgias-pp-cli workflow status --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("gorgias-pp-cli")
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

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}

// defaultDBPath is defined in helpers.go
