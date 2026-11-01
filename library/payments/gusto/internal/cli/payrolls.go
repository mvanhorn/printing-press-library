// Copyright 2026 david and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelPayrollsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "payrolls",
		Short:       "Work with payrolls",
		Example:     "  gusto-pp-cli payrolls compare --period-end 2026-11-01 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelPayrollsCompareCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelPayrollsMileageCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelPayrollsPrepCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelPayrollsVerifyCmd(flags))
	return cmd
}
