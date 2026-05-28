// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/config"
)

// newAuthUseCmd registers `superhuman-pp-cli auth use <email>`. The command
// writes Config.ActiveEmail to the on-disk config file so subsequent CLI
// invocations resolve the active account deterministically (step 2 of the
// four-step priority order in plan R2b). `--clear` removes the pin so the
// resolver falls back to most-recently-used.
//
// MCP-hidden: setting an active account is part of interactive setup; an
// agent that needs a specific account should pass --account per-command
// instead of persisting a default.
func newAuthUseCmd(flags *rootFlags) *cobra.Command {
	var clear bool
	cmd := &cobra.Command{
		Use:   "use <email>",
		Short: "Set the default active account (or --clear to remove the pin)",
		Example: "  superhuman-pp-cli auth use user@example.com\n" +
			"  superhuman-pp-cli auth use --clear",
		Args: func(cmd *cobra.Command, args []string) error {
			if clear {
				if len(args) > 0 {
					return usageErr(fmt.Errorf("auth use: --clear takes no positional args"))
				}
				return nil
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("auth use: requires <email> arg or --clear flag"))
			}
			return nil
		},
		Annotations: map[string]string{
			"pp:typed-exit-codes": "0,2,4",
			// MCP hidden: agents should use --account per-call rather than
			// mutating persistent CLI state.
			"mcp:hidden": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			if clear {
				if err := cfg.SaveActiveEmail(""); err != nil {
					return configErr(fmt.Errorf("auth use --clear: %w", err))
				}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"cleared":     true,
						"config_path": cfg.Path,
					}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Active account cleared. Resolution falls back to most-recently-used.")
				return nil
			}

			email := args[0]
			store := auth.NewStoreAt(cfg.TokenStorePath())
			_, exists, err := store.Get(email)
			if err != nil {
				return configErr(fmt.Errorf("auth use: load token store: %w", err))
			}
			if !exists {
				accounts := availableAccounts(store)
				return authErr(fmt.Errorf("auth use: account %q not in token store (available: %s); run 'auth login --disk' first",
					email, formatAccountList(accounts)))
			}
			if err := cfg.SaveActiveEmail(email); err != nil {
				return configErr(fmt.Errorf("auth use: save active email: %w", err))
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"active":      email,
					"config_path": cfg.Path,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Active account: %s\n", email)
			return nil
		},
	}
	cmd.Flags().BoolVar(&clear, "clear", false, "Clear the active account pin (fallback to most-recently-used)")
	return cmd
}

// availableAccounts returns the sorted list of emails in the token store.
// Returns nil if the store can't be loaded — callers render a "[none]" hint
// rather than surfacing the I/O error, which would obscure the actual
// "account not in store" message.
func availableAccounts(store *auth.Store) []string {
	p, err := store.Load()
	if err != nil || p == nil {
		return nil
	}
	out := make([]string, 0, len(p.Accounts))
	for email := range p.Accounts {
		out = append(out, email)
	}
	sort.Strings(out)
	return out
}

// formatAccountList renders a list of emails for inline display in an error
// message. Empty list renders as "[none]" so the user sees "available: [none]"
// rather than a dangling colon.
func formatAccountList(emails []string) string {
	if len(emails) == 0 {
		return "[none]"
	}
	out := ""
	for i, e := range emails {
		if i > 0 {
			out += ", "
		}
		out += e
	}
	return out
}
