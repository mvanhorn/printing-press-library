// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelCommentsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "comments",
		Short:       "comments subcommands: coverage, search, sweep, thread",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelCommentsCoverageCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelCommentsSearchCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelCommentsSweepCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelCommentsThreadCmd(flags))
	return cmd
}
