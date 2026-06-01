// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command: `drives diff <path>`. Shows which local files
// differ from a Drive (added, changed, deleted, unchanged) without uploading
// anything. Read-only. Header is a plain copyright line so regen-merge
// preserves this file.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelDriveDiffCmd(flags *rootFlags) *cobra.Command {
	var (
		flagDrive string
		flagDB    string
	)

	cmd := &cobra.Command{
		Use:   "diff <path>",
		Short: "Show which local files differ from a Drive without uploading anything",
		Example: strings.Trim(`
  here-now-pp-cli drives diff ./assets --drive drv_abc123
  here-now-pp-cli drives diff ./assets --drive drv_abc123 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			dir := args[0]
			if strings.TrimSpace(flagDrive) == "" {
				return usageErr(fmt.Errorf("--drive <id> is required (e.g. --drive drv_abc123); see 'here-now-pp-cli drives get-default')"))
			}
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			remote, err := listDriveFiles(cmd.Context(), c, flagDrive)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			diff, err := computeDriveDiff(dir, remote, true)
			if err != nil {
				return err
			}

			added := []string{}
			changed := []string{}
			localSeen := map[string]bool{}
			for _, f := range diff.Upload {
				localSeen[f.RelPath] = true
				if _, ok := remote[f.RelPath]; ok {
					changed = append(changed, f.RelPath)
				} else {
					added = append(added, f.RelPath)
				}
			}
			unchanged := make([]string, 0, len(diff.Unchanged))
			for _, f := range diff.Unchanged {
				unchanged = append(unchanged, f.RelPath)
			}

			out := map[string]any{
				"added":     added,
				"changed":   changed,
				"deleted":   diff.Delete,
				"unchanged": unchanged,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added: %d  changed: %d  deleted: %d  unchanged: %d\n",
				len(added), len(changed), len(diff.Delete), len(unchanged))
			for _, p := range added {
				fmt.Fprintf(cmd.OutOrStdout(), "  + %s\n", p)
			}
			for _, p := range changed {
				fmt.Fprintf(cmd.OutOrStdout(), "  ~ %s\n", p)
			}
			for _, p := range diff.Delete {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", p)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagDrive, "drive", "", "Drive ID to diff against (required)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path (default: ~/.local/share/here-now-pp-cli/data.db)")
	return cmd
}
