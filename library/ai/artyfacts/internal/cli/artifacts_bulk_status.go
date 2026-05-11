// Copyright 2026 bernard-maltais. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/artyfacts/internal/store"
	"github.com/spf13/cobra"
)

func newArtifactsBulkStatusCmd(flags *rootFlags) *cobra.Command {
	var flagStatus string
	var flagFilterType string
	var flagFilterStatus string
	var flagDryRun bool
	var flagDBPath string

	cmd := &cobra.Command{
		Use:   "bulk-status",
		Short: "Batch-transition artifact statuses",
		Long: `Set the status on all artifacts matching the filter criteria.
Both the local store and the live API are updated.

Use --dry-run to preview changes before applying.

Valid statuses: draft, active, final, superseded, archived`,
		Example: `  # Archive all superseded docs
  artyfacts-pp-cli artifacts bulk-status --status archived --filter-status superseded --filter-type doc

  # Preview promoting all draft specs to active
  artyfacts-pp-cli artifacts bulk-status --status active --filter-status draft --filter-type spec --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagStatus == "" {
				return fmt.Errorf("--status is required (draft, active, final, superseded, archived)")
			}
			validStatuses := map[string]bool{
				"draft": true, "active": true, "final": true,
				"superseded": true, "archived": true,
			}
			if !validStatuses[strings.ToLower(flagStatus)] {
				return fmt.Errorf("invalid status %q; valid: draft, active, final, superseded, archived", flagStatus)
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

			ids, err := db.ListByStatus(flagFilterType, flagFilterStatus)
			if err != nil {
				return err
			}

			if len(ids) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching artifacts found.")
				return nil
			}

			if flagDryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Dry run: would set status=%s on %d artifact(s):\n", flagStatus, len(ids))
				for _, id := range ids {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", id)
				}
				return nil
			}

			if !flags.yes && !flags.noInput {
				fmt.Fprintf(cmd.OutOrStdout(), "Set status=%s on %d artifact(s)? [y/N] ", flagStatus, len(ids))
				var resp string
				fmt.Scanln(&resp)
				if strings.ToLower(strings.TrimSpace(resp)) != "y" {
					fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
					return nil
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Pre-flight: verify the API accepts status-only PATCH on the first artifact.
			// The Artyfacts PATCH /artifacts/{id} endpoint requires at least one non-status
			// field to be present; a status-only body returns "No valid fields to update".
			_, _, preErr := c.Patch(fmt.Sprintf("/artifacts/%s", ids[0]), map[string]any{"status": flagStatus})
			if preErr != nil {
				if isNoValidFieldsError(preErr) {
					return fmt.Errorf("the Artyfacts API does not currently support status-only PATCH updates.\nTo change an artifact's status, use `artifacts update <id> --status <status> --title <current-title>`.")
				}
				return preErr
			}
			_ = db.UpdateArtifactStatus(ids[0], flagStatus)

			succeeded, failed := 1, 0
			for _, id := range ids[1:] {
				_, _, apiErr := c.Patch(fmt.Sprintf("/artifacts/%s", id), map[string]any{"status": flagStatus})
				if apiErr != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  FAIL %s: %v\n", id, apiErr)
					failed++
					continue
				}
				if storeErr := db.UpdateArtifactStatus(id, flagStatus); storeErr != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  WARN %s: store update failed: %v\n", id, storeErr)
				}
				succeeded++
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Done: %d updated, %d failed.\n", succeeded, failed)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagStatus, "status", "", "New status to set (required)")
	cmd.Flags().StringVar(&flagFilterType, "filter-type", "", "Only act on artifacts of this type")
	cmd.Flags().StringVar(&flagFilterStatus, "filter-status", "", "Only act on artifacts with this current status")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Preview changes without applying")
	cmd.Flags().StringVar(&flagDBPath, "db", "", "Custom SQLite database path")

	return cmd
}
