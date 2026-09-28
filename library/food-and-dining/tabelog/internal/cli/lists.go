// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelListsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "lists",
		Short:       "Work with lists",
		Example:     "  tabelog-pp-cli lists add tokyo-bars 13005012 --note 'Ginza bar option' --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelListsAddCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelListsAlternativesCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelListsAuditCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelListsCompareCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelListsRefreshCmd(flags))
	return cmd
}
