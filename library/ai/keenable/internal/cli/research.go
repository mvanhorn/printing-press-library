// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelResearchCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "research",
		Short:       "Work with research",
		Example:     "  keenable-pp-cli research citations --snapshot latest --format markdown",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelResearchCitationsCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelResearchCoverageCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelResearchDiffCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelResearchFetchManyCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelResearchLocalSearchCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelResearchReplayCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelResearchSnapshotCmd(flags))
	return cmd
}
