// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newShareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "share",
		Short: "Notebook sharing",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "show <notebook>",
		Short: "Get public/private sharing status for a notebook",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Example: `  notebooklm-pp-cli share show "Q3 Research" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSON(map[string]any{"public": false, "dry_run": true})
				}
				dryRunMessage("get share status")
				return nil
			}
			client, err := newAPIClient(context.Background(), flags)
			if err != nil {
				return err
			}
			nb, err := client.ResolveNotebook(context.Background(), args[0])
			if err != nil {
				return err
			}
			st, err := client.GetShareStatus(context.Background(), nb.ID)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSON(st)
			}
			fmt.Printf("notebook %s sharing: %+v\n", nb.ID, st)
			return nil
		},
	})
	return cmd
}
