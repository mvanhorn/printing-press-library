// Copyright 2026 jacobprice. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

// newFleetCmd is the parent for fleet-wide read-only audits over the local
// synced store. Subcommands answer cross-server questions ("every SSL cert",
// "every blocked IP", "every PHP version below X") that the upstream API
// only exposes per-server.
func newFleetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fleet",
		Short: "Fleet-wide audits across every server",
		Long: `Fleet-wide audits across every synced server.

Most fleet subcommands read from the local SQLite store populated by 'sync'.
Run 'runcloud-pp-cli sync' first so the store has data to query.`,
		RunE: parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newFleetSSLAuditCmd(flags))
	cmd.AddCommand(newFleetPHPAuditCmd(flags))
	cmd.AddCommand(newFleetHealthCmd(flags))
	cmd.AddCommand(newFleetBlockedIPsCmd(flags))
	cmd.AddCommand(newFleetInstallersCmd(flags))
	cmd.AddCommand(newFleetSSHKeysCmd(flags))
	cmd.AddCommand(newFleetServicesCmd(flags))

	return cmd
}

// newAgencyCmd is the parent for hand-authored agency workflow commands that
// chain multiple agency-api endpoints. The auto-generated agency-clients,
// agency-packages, agency-teams, and agency-client-servers commands stay
// where they are; this parent groups higher-level workflow commands like
// 'onboard'.
func newAgencyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agency",
		Short: "Agency workflows (multi-step end-to-end flows)",
		RunE:  parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newAgencyOnboardCmd(flags))

	return cmd
}
