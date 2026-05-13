// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// Root `top` subtree that owns `top drift` (T4). The endpoint-mirror top-tracks
// and top-artists commands live under `me get-users-top-tracks` and friends;
// this `top` tree is a cleaner agent surface for the same surface plus the
// transcendence command.

package cli

import (
	"github.com/spf13/cobra"
)

func newTopCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "top",
		Short: "Top tracks and artists (with snapshot drift)",
	}
	cmd.AddCommand(newTopDriftCmd(flags))
	return cmd
}
