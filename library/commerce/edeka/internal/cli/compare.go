// Copyright 2026 JimPresting and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "compare",
		Short:       "Work with compare",
		Example:     "  edeka-pp-cli compare markets milch --market doerflinger-ulm --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:parent-group": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelCompareMarketsCmd(flags))
	return cmd
}
