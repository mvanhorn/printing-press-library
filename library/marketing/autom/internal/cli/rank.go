// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelRankCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "rank",
		Short:       "Work with rank",
		Example:     "  autom-pp-cli rank movers --since 7d --domain autom.dev --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:parent-group": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelRankMoversCmd(flags))
	return cmd
}
