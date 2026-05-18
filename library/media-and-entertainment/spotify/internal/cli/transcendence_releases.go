// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// Root `releases` subtree owning `releases since` (T5). Distinct from the
// generator-emitted `browse new-releases` endpoint mirror — `releases since`
// is a hand-built deterministic feed sourced from followed_artists.

package cli

import (
	"github.com/spf13/cobra"
)

func newReleasesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "releases",
		Short: "Release-Radar replacement: new releases from your followed artists",
	}
	cmd.AddCommand(newReleasesSinceCmd(flags))
	return cmd
}
