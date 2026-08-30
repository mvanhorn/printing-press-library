// Copyright 2026 RyanGravetteIDLA and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelAuditCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "audit",
		Short:       "audit subcommands: app-risk, email-exposure, external-shares, logins, reconstruct, user360",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelAuditAppRiskCmd(flags))
	cmd.AddCommand(newNovelAuditEmailExposureCmd(flags))
	cmd.AddCommand(newNovelAuditExternalSharesCmd(flags))
	cmd.AddCommand(newNovelAuditLoginsCmd(flags))
	cmd.AddCommand(newNovelAuditReconstructCmd(flags))
	cmd.AddCommand(newNovelAuditUser360Cmd(flags))
	return cmd
}
