// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelFloatCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "float",
		Short:       "Cross-source float comparison: custody penetration vs exchange vs margin float",
		Example:     "  cdc-pakistan-pp-cli float triangulate --symbol OGDC --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelFloatTriangulateCmd(flags))
	return cmd
}
