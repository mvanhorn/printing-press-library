// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelDeliveryCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "delivery",
		Short:       "Safe automation",
		Example:     "  agentmail-pp-cli delivery reconcile agentmail delivery reconcile --db /tmp/agentmail.db --since 7d --json --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:parent-group": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelDeliveryReconcileCmd(flags))
	return cmd
}
