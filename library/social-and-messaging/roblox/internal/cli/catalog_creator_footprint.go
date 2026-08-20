// Copyright 2026 Kieran Maynard and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

func newNovelCatalogCreatorFootprintCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "creator-footprint <creator-id>", Short: "Summarize a creator's public games, bundles, and identity.", Example: "  roblox-pp-cli catalog creator-footprint 1 --agent", Args: cobra.ExactArgs(1), Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateRobloxID(args[0], "creator-id"); err != nil {
			return err
		}
		if flags.dataSource == "local" {
			return fmt.Errorf("catalog creator-footprint has no local data source; use --data-source live")
		}
		if dryRunOK(flags) {
			return nil
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		id := args[0]
		out := map[string]any{"creator_id": id}
		fetchBundle(cmd, c, out, "creator", "https://users.roblox.com/v1/users/"+id, nil)
		fetchBundle(cmd, c, out, "games", "https://games.roblox.com/v2/users/"+id+"/games", map[string]string{"accessFilter": "Public", "limit": "10", "sortOrder": "Asc"})
		fetchBundle(cmd, c, out, "bundles", "https://catalog.roblox.com/v1/users/"+id+"/bundles", map[string]string{"limit": "10", "sortOrder": "Asc"})
		return printBundle(cmd, flags, out)
	}}
}
