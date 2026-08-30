// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelThreadCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "thread",
		Short:       "Local operational memory",
		Example:     "  agentmail-pp-cli thread rollup agentmail thread rollup thread_demo --db /tmp/agentmail.db --json --agent --select thread_id,latest_direction,message_count,pending_draft",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:parent-group": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelThreadRollupCmd(flags))
	return cmd
}
