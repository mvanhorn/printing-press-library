// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelSpecsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "specs",
		Short:       "Show and compare Crestron product specification tables",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelSpecsCompareCmd(flags))
	addNovelCommandIfAbsent(cmd, newCrestronSpecsShowCmd(flags))
	return cmd
}
