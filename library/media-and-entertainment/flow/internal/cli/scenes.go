// Copyright 2026 github-actionsbot and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelScenesCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "scenes",
		Short:       "Local sanity checks",
		Example: "  flow-pp-cli scenes gaps --project a1b2c3d4-e5f6-47a8-9b0c-1d2e3f4a5b6c",
		// pp:typed-exit-codes: this parent's own Example is independently
		// live-dogfood-tested (see drive.go); same placeholder-project-ID
		// caveat as scenes_gaps.go applies here too.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,5"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelScenesGapsCmd(flags))
	return cmd
}
