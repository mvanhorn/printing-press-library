// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelSettingsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "settings",
		Short:       "settings subcommands: diff",
		Example:     "  algolia-pp-cli settings diff algolia_movie_sample_dataset staging_movies",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelSettingsDiffCmd(flags))
	return cmd
}
