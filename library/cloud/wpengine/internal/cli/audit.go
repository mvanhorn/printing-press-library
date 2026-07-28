// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelAuditCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "audit",
		Short:       "audit subcommands: backups, certs, domains, usage, versions",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelAuditBackupsCmd(flags))
	cmd.AddCommand(newNovelAuditCertsCmd(flags))
	cmd.AddCommand(newNovelAuditDomainsCmd(flags))
	cmd.AddCommand(newNovelAuditUsageCmd(flags))
	cmd.AddCommand(newNovelAuditVersionsCmd(flags))
	return cmd
}
