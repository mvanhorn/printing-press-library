// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelVipCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:     "vip",
		Short:   "Measure what an AgodaVIP session is actually worth",
		Example: "  agoda-pp-cli vip delta Tokyo --checkin 2026-10-15 --nights 2 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			// Mirrors the delta subcommand: the live matrix attributes results to
			// this parent, so the typed auth-required exit must be declared here
			// too or an honest "no session" result is scored as a failure.
			"pp:typed-exit-codes": "0,4",
		},
		RunE: parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelVipDeltaCmd(flags))
	return cmd
}
