// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Multi-account profile commands (gauth layer). Hand-written; ported from the
// google-calendar-pp-cli gauth surface. Every profile holds the single
// gmail.modify scope — see internal/gauth's package doc for why the role
// field exists but does not widen or narrow the grant.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/gauth"
)

// resolveGauthAccount resolves which gauth profile a command targets and
// validates it against profiles.yaml, so a typo'd --account fails with the
// available names instead of a downstream 401. When --account is empty and
// exactly one profile exists, that profile is the implicit default; with
// several profiles the flag is required. On success flags.account holds the
// resolved name so newClient() injects that profile's token.
func resolveGauthAccount(flags *rootFlags) (string, error) {
	dir := gauth.ConfigDir(flags.authDir)
	if flags.account != "" {
		p, err := gauth.Get(dir, flags.account)
		if err != nil {
			return "", err
		}
		return p.Name, nil
	}
	p, err := gauth.DefaultAccount(dir)
	if err != nil {
		return "", err
	}
	flags.account = p.Name
	return p.Name, nil
}

func newAccountsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accounts",
		Short: "List gauth profiles and their token status (every profile holds the single gmail.modify scope)",
		Example: `  gmail-pp-cli accounts
  gmail-pp-cli accounts --json
  gmail-pp-cli accounts auth --account personal
  gmail-pp-cli accounts auth --all`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			dir := gauth.ConfigDir(flags.authDir)
			sts, err := gauth.Statuses(dir)
			if err != nil {
				return err
			}
			type row struct {
				Name     string    `json:"name"`
				Email    string    `json:"email"`
				Role     string    `json:"role"`
				HasToken bool      `json:"has_token"`
				Expiry   time.Time `json:"expiry,omitempty"`
			}
			rows := make([]row, 0, len(sts))
			for _, s := range sts {
				rows = append(rows, row{s.Profile.Name, s.Profile.Email, s.Profile.Role, s.HasTok, s.Expiry})
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			hdr := []string{"NAME", "EMAIL", "ROLE", "TOKEN"}
			var out [][]string
			for _, r := range rows {
				tok := "missing — run: accounts auth --account " + r.Name
				if r.HasToken {
					tok = "ok"
				}
				out = append(out, []string{r.Name, r.Email, r.Role, tok})
			}
			return flags.printTable(cmd, hdr, out)
		},
	}
	cmd.AddCommand(newAccountsAuthCmd(flags))
	return cmd
}

func newAccountsAuthCmd(flags *rootFlags) *cobra.Command {
	var all bool
	var account string
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Run the browser OAuth flow for one profile (or --all); the consented account is verified against the profile's email",
		Example: `  gmail-pp-cli accounts auth --account personal
  gmail-pp-cli accounts auth --all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			dir := gauth.ConfigDir(flags.authDir)
			var targets []gauth.Profile
			switch {
			case all:
				ps, err := gauth.LoadProfiles(dir)
				if err != nil {
					return err
				}
				targets = ps
			case account != "":
				p, err := gauth.Get(dir, account)
				if err != nil {
					return err
				}
				targets = []gauth.Profile{p}
			default:
				return usageErr(fmt.Errorf("pass --account <name> or --all"))
			}
			for _, p := range targets {
				if err := gauth.Authenticate(cmd.Context(), dir, p, func(s string) { cmd.Println(s) }); err != nil {
					return fmt.Errorf("profile %q: %w", p.Name, err)
				}
			}
			cmd.Println("All requested profiles authorized.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Authorize every profile in profiles.yaml, one browser round each")
	cmd.Flags().StringVar(&account, "account", "", "Profile name to authorize")
	return cmd
}
