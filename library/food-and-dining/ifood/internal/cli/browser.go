// Copyright 2026 Matheus Coêlho and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelBrowserCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "browser",
		Short:       "browser subcommands: cart-plan, plan, validate-quote",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelBrowserCartPlanCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelBrowserPlanCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelBrowserValidateQuoteCmd(flags))
	return cmd
}
