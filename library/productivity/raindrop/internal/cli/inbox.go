// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelInboxCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "inbox",
		Short:       "inbox subcommands: review",
		Example:     "  raindrop-pp-cli inbox review --limit 10 --resume",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelInboxReviewCmd(flags))
	addNovelCommandIfAbsent(cmd, newInboxApplyCmd(flags))
	return cmd
}
