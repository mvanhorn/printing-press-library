// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// newAuthCmd groups the optional Tier-1 Medium session-cookie commands. v2 is
// $0/no-key: there is no API key to manage. The only "auth" concept is the
// user's OWN Medium session cookie — always optional (every command works
// anonymously at Tier 0) and used only to unlock member full bodies on the
// read path. See newAuthLoginCmd (auth_login.go) for the import paths.
func newAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage the optional Medium session cookie (Tier 1)",
		RunE:  parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newAuthLoginCmd(flags)) // Tier-1 Medium session cookie import + status

	return cmd
}
