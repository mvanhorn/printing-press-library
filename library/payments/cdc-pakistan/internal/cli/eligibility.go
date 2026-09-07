// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelEligibilityCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "eligibility",
		Short:       "CDS eligibility lifecycle folded from twenty years of notices",
		Example:     "  cdc-pakistan-pp-cli eligibility state --isin PK0069501016 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelEligibilityStateCmd(flags))
	return cmd
}
