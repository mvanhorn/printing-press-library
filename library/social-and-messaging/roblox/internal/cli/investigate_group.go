// Copyright 2026 Kieran Maynard and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelInvestigateGroupCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use: "group <group-id>", Short: "Resolve a Roblox group, its owner, roles, and public context.", Example: "  roblox-pp-cli investigate group 1 --agent",
		Args: cobra.ExactArgs(1), Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateRobloxID(args[0], "group-id"); err != nil {
				return err
			}
			if flags.dataSource == "local" {
				return fmt.Errorf("investigate group has no local data source; use --data-source live")
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			id := args[0]
			out := map[string]any{"group_id": id}
			fetchBundle(cmd, c, out, "group", "https://groups.roblox.com/v1/groups/"+id, nil)
			fetchBundle(cmd, c, out, "roles", "https://groups.roblox.com/v1/groups/"+id+"/roles", nil)
			if g, ok := out["group"].(map[string]any); ok {
				if owner, ok := g["owner"].(map[string]any); ok {
					if n, ok := owner["userId"].(json.Number); ok {
						fetchBundle(cmd, c, out, "owner", "https://users.roblox.com/v1/users/"+n.String(), nil)
					}
				}
			}
			return printBundle(cmd, flags, out)
		},
	}
	return cmd
}
