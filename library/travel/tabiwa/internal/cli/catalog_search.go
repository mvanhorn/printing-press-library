// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import "github.com/spf13/cobra"

func newNovelCatalogSearchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "search [query]"}
	var region string
	cmd.Flags().StringVar(&region, "region", "10", "Regional catalog: 10 せとうち, 20 北陸, 30 山陰, 40 九州; a public display preference")
	bindCatalogSearch(cmd, flags, false)
	return cmd
}
