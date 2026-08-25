// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Multi-account profile commands (gauth layer). Hand-written; part of the
// Multi-account auth surface: per-profile scope roles, verified consents.

package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/google-calendar/internal/gauth"
	"github.com/spf13/cobra"
)

func newAccountsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accounts",
		Short: "List gauth profiles and their token status (roles decide OAuth scopes)",
		Example: `  google-calendar-pp-cli accounts
  google-calendar-pp-cli accounts --json
  google-calendar-pp-cli accounts auth --account personal
  google-calendar-pp-cli accounts auth --all`,
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
				Default  bool      `json:"default_write"`
				HasToken bool      `json:"has_token"`
				Expiry   time.Time `json:"expiry,omitempty"`
			}
			rows := make([]row, 0, len(sts))
			for _, s := range sts {
				rows = append(rows, row{s.Profile.Name, s.Profile.Email, s.Profile.Role, s.Profile.Default, s.HasTok, s.Expiry})
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			hdr := []string{"NAME", "EMAIL", "ROLE", "DEFAULT_WRITE", "TOKEN"}
			var out [][]string
			for _, r := range rows {
				tok := "missing — run: accounts auth --account " + r.Name
				if r.HasToken {
					tok = "ok"
				}
				def := ""
				if r.Default {
					def = "yes"
				}
				out = append(out, []string{r.Name, r.Email, r.Role, def, tok})
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
		Short: "Run the browser OAuth flow for one profile (or --all), requesting only the profile role's scopes",
		Example: `  google-calendar-pp-cli accounts auth --account personal
  google-calendar-pp-cli accounts auth --all`,
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
				return fmt.Errorf("pass --account <name> or --all")
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
