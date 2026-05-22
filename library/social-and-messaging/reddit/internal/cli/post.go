// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// newPostCmd is a compact alias parent for novel post-level features.
// The spec-derived `submissions` resource handles CRUD; `post` exposes the
// novel analytics that ride on top.
func newPostCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "post",
		Short: "Post-level analytics: velocity (comments/min vs sub baseline)",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newPostVelocityCmd(flags))
	return cmd
}
