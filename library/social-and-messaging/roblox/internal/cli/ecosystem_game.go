// Copyright 2026 Kieran Maynard and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

func newNovelEcosystemGameCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "game <universe-id>", Short: "Build a public game-centered view of details, media, badges, and thumbnails.", Example: "  roblox-pp-cli ecosystem game 1534453623 --agent", Args: cobra.ExactArgs(1), Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateRobloxID(args[0], "universe-id"); err != nil {
			return err
		}
		if flags.dataSource == "local" {
			return fmt.Errorf("ecosystem game has no local data source; use --data-source live")
		}
		if dryRunOK(flags) {
			return nil
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		id := args[0]
		out := map[string]any{"universe_id": id}
		fetchBundle(cmd, c, out, "game", "https://games.roblox.com/v1/games", map[string]string{"universeIds": id})
		fetchBundle(cmd, c, out, "media", "https://games.roblox.com/v2/games/"+id+"/media", nil)
		fetchBundle(cmd, c, out, "badges", "https://badges.roblox.com/v1/universes/"+id+"/badges", map[string]string{"limit": "10", "sortOrder": "Asc"})
		fetchBundle(cmd, c, out, "thumbnail", "https://thumbnails.roblox.com/v1/games/icons", map[string]string{"universeIds": id, "size": "512x512", "format": "Png", "isCircular": "false"})
		return printBundle(cmd, flags, out)
	}}
	return cmd
}
