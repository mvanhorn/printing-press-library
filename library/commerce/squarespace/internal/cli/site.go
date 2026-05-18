// Copyright 2026 zayd. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

func newSiteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "site",
		Aliases: []string{"website"},
		Short:   "Inspect the Squarespace website attached to the current token",
	}

	profile := newV10GetWebsiteProfileCmd(flags)
	profile.Use = "profile"
	profile.Aliases = []string{"info", "website-profile"}
	profile.Example = "  squarespace-pp-cli site profile"

	storePages := newV10GetStorePagesCmd(flags)
	storePages.Use = "store-pages"
	storePages.Aliases = []string{"pages"}
	storePages.Example = "  squarespace-pp-cli site store-pages --all"

	cmd.AddCommand(profile)
	cmd.AddCommand(storePages)
	return cmd
}

func newStorePagesAliasCmd(flags *rootFlags) *cobra.Command {
	cmd := newV10GetStorePagesCmd(flags)
	cmd.Use = "store-pages"
	cmd.Aliases = []string{"pages"}
	cmd.Short = "List commerce store pages for the current Squarespace website"
	cmd.Example = "  squarespace-pp-cli store-pages --all\n  squarespace-pp-cli pages --json"
	return cmd
}
