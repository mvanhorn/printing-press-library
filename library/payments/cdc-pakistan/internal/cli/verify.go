// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelVerifyCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "verify",
		Short:       "Extraction and schema audits over the penetration reports",
		Example:     "  cdc-pakistan-pp-cli verify rows --vintage 2025-11-30 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelVerifyRowsCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelVerifySchemaCmd(flags))
	return cmd
}
