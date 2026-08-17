// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelRulesCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "rules",
		Short:       "rules subcommands: stale",
		Example:     "  algolia-pp-cli rules stale --index algolia_movie_sample_dataset",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelRulesStaleCmd(flags))
	return cmd
}
