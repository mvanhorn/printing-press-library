// Copyright 2026 JimPresting and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelBasketCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "basket",
		Short:       "Work with basket",
		Example:     "  edeka-pp-cli basket add hafermilch --qty 2 --dry-run",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:parent-group": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelBasketAddCmd(flags))
	return cmd
}
