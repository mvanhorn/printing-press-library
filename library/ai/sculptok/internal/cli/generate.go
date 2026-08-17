// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-edited: added the restore subcommand and corrected the summary. The
// `generate` group is the headline workflow surface (upload -> submit -> poll
// -> persist) for SculptOK draws.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelGenerateCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate from a local image: depthmap, stl, threed, restore",
		Long:  "One-command SculptOK generation: upload a local image, submit the draw, poll to completion, record the job locally, and download the results.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelGenerateDepthmapCmd(flags))
	cmd.AddCommand(newNovelGenerateStlCmd(flags))
	cmd.AddCommand(newNovelGenerateThreedCmd(flags))
	cmd.AddCommand(newNovelGenerateRestoreCmd(flags))
	return cmd
}
