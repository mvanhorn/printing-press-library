// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command: `drives sync <path>`. Rsync-style sync of a
// local directory to a Drive: compares local sha256 against the Drive's file
// checksums and uploads only what is new or changed (and, with --delete,
// removes remote-only files). Header is a plain copyright line so regen-merge
// preserves this file.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelDriveSyncCmd(flags *rootFlags) *cobra.Command {
	var (
		flagDrive  string
		flagDelete bool
		flagDB     string
	)

	cmd := &cobra.Command{
		Use:   "sync <path>",
		Short: "Rsync-style sync of a local directory to a Drive (uploads only changed files)",
		Example: strings.Trim(`
  here-now-pp-cli drives sync ./assets --drive drv_abc123
  here-now-pp-cli drives sync ./assets --drive drv_abc123 --delete
  here-now-pp-cli drives sync ./assets --drive drv_abc123 --dry-run --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			dir := args[0]
			if strings.TrimSpace(flagDrive) == "" {
				return usageErr(fmt.Errorf("--drive <id> is required (e.g. --drive drv_abc123); see 'here-now-pp-cli drives get-default')"))
			}

			if dryRunOK(flags) {
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"dry_run": true,
						"dir":     dir,
						"drive":   flagDrive,
						"delete":  flagDelete,
					}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] would sync %s to drive %s (delete=%v)\n", dir, flagDrive, flagDelete)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			res, err := runDriveSync(cmd.Context(), c, flagDrive, dir, flagDelete, flags.timeout)
			if err != nil {
				if res != nil {
					return err
				}
				return classifyAPIError(err, flags)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "uploaded %d, skipped %d, deleted %d\n", res.Uploaded, res.Skipped, res.Deleted)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagDrive, "drive", "", "Drive ID to sync into (required)")
	cmd.Flags().BoolVar(&flagDelete, "delete", false, "Delete remote files that no longer exist locally")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path (default: ~/.local/share/here-now-pp-cli/data.db)")
	return cmd
}
