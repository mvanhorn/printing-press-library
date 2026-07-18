// Copyright 2026 Kieran Maynard and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelInvestigateUserCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "user <user-id>",
		Short:       "Assemble public identity, avatar, groups, badges, inventory visibility, and thumbnail data.",
		Long:        "Use this command for a broad live investigation of one Roblox user. Do NOT use it to compare two users; use 'network overlap' instead.",
		Example:     "  roblox-pp-cli investigate user 1 --agent",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateRobloxID(args[0], "user-id"); err != nil {
				return err
			}
			if flags.dataSource == "local" {
				return fmt.Errorf("investigate user has no local data source; use --data-source live")
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			id := args[0]
			out := map[string]any{"user_id": id}
			fetchBundle(cmd, c, out, "user", "https://users.roblox.com/v1/users/"+id, nil)
			fetchBundle(cmd, c, out, "avatar", "https://avatar.roblox.com/v2/avatar/users/"+id+"/avatar", nil)
			fetchBundle(cmd, c, out, "groups", "https://groups.roblox.com/v2/users/"+id+"/groups/roles", nil)
			fetchBundle(cmd, c, out, "badges", "https://badges.roblox.com/v1/users/"+id+"/badges", map[string]string{"limit": "10", "sortOrder": "Asc"})
			fetchBundle(cmd, c, out, "inventory_visibility", "https://inventory.roblox.com/v1/users/"+id+"/can-view-inventory", nil)
			fetchBundle(cmd, c, out, "thumbnail", "https://thumbnails.roblox.com/v1/users/avatar-headshot", map[string]string{"userIds": id, "size": "150x150", "format": "Png", "isCircular": "false"})
			return printBundle(cmd, flags, out)
		},
	}
	return cmd
}
