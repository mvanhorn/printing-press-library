// Copyright 2026 Kieran Maynard and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

func newNovelNetworkOverlapCmd(flags *rootFlags) *cobra.Command {
	var a, b string
	cmd := &cobra.Command{Use: "overlap", Short: "Find shared public friends and groups between two Roblox users.", Example: "  roblox-pp-cli network overlap --user-a 1 --user-b 156 --agent", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if a == "" || b == "" {
			return usageErr(fmt.Errorf("--user-a and --user-b are required"))
		}
		if err := validateRobloxID(a, "user-a"); err != nil {
			return err
		}
		if err := validateRobloxID(b, "user-b"); err != nil {
			return err
		}
		if flags.dataSource == "local" {
			return fmt.Errorf("network overlap has no local data source; use --data-source live")
		}
		if dryRunOK(flags) {
			return nil
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		out := map[string]any{"user_a": a, "user_b": b}
		fa, err := fetchArray(cmd, c, "https://friends.roblox.com/v1/users/"+a+"/friends", nil)
		if err != nil {
			return fmt.Errorf("fetching friends for --user-a: %w", err)
		}
		fb, err := fetchArray(cmd, c, "https://friends.roblox.com/v1/users/"+b+"/friends", nil)
		if err != nil {
			return fmt.Errorf("fetching friends for --user-b: %w", err)
		}
		ga, err := fetchArray(cmd, c, "https://groups.roblox.com/v2/users/"+a+"/groups/roles", nil)
		if err != nil {
			return fmt.Errorf("fetching groups for --user-a: %w", err)
		}
		gb, err := fetchArray(cmd, c, "https://groups.roblox.com/v2/users/"+b+"/groups/roles", nil)
		if err != nil {
			return fmt.Errorf("fetching groups for --user-b: %w", err)
		}
		out["shared_friends"] = intersectByNestedID(fa, fb, "id")
		out["shared_groups"] = intersectByNestedID(ga, gb, "group.id")
		return printBundle(cmd, flags, out)
	}}
	cmd.Flags().StringVar(&a, "user-a", "", "first Roblox user ID")
	cmd.Flags().StringVar(&b, "user-b", "", "second Roblox user ID")
	return cmd
}
