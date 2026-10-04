// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source auto
package cli

import "github.com/spf13/cobra"

func newSnowResortsSearchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "search"}
	configureSnowResortsList(cmd, flags, true)
	return cmd
}
