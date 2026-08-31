// Copyright 2026 github-actionsbot and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelEpisodeCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "episode",
		Short: "Audio-drama pipeline",
		// See drive.go for why this mirrors the child's real pp:happy-args
		// fixture (with --dry-run) rather than an illustrative, non-existent
		// path or a bare --help invocation (which fails --json fidelity).
		Example:     "  flow-pp-cli episode import --scribe-folder testdata/dogfood-fixtures/scribe --images-folder testdata/dogfood-fixtures/images --dry-run",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelEpisodeImportCmd(flags))
	return cmd
}
