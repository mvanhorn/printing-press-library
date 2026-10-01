// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelPolicyCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "policy",
		Short:       "Add a policy-file entry or roll back to a local backup, with validation and a diff",
		Example:     "  tailscale-pp-cli policy restore --list",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelPolicyAddEntryCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelPolicyRestoreCmd(flags))
	return cmd
}
