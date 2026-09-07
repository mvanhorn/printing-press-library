// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelGopCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "gop",
		Short:       "Government of Pakistan shareholding derived from CDC's paired capital columns",
		Example:     "  cdc-pakistan-pp-cli gop stake --min-pct 25 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelGopStakeCmd(flags))
	return cmd
}
