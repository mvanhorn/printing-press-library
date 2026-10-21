// Copyright 2026 Elizabeth Stone and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelPerfCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "perf",
		Short:       "Work with perf",
		Example:     "  pinterest-pp-cli perf compare --days 28 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelPerfCompareCmd(flags))
	return cmd
}
