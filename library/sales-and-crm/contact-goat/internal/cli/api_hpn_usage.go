// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// api_hpn_usage.go: `api hpn usage` and `api hpn user`. Both wrap free
// probes — no credit cost, no budget gate, no cost preview.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newAPIHpnUsageCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "usage",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "Show live Happenstance public-API credit balance and usage history (free)",
		Long:    `Calls GET /v1/usage. Returns the live credit balance, purchase history, recent usage events, and auto-reload settings. Free probe — no credits spent.`,
		Example: `  contact-goat-pp-cli api hpn usage --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newHappenstanceAPIClient()
			if err != nil {
				return err
			}
			u, err := c.Usage(cmd.Context())
			if err != nil {
				return classifyHpnError(err)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{
					"source":          "api",
					"balance_credits": u.BalanceCredits,
					"has_credits":     u.HasCredits,
					"purchases":       u.Purchases,
					"usage":           u.Usage,
					"auto_reload":     u.AutoReload,
				})
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "balance: %d credits\n", u.BalanceCredits)
			fmt.Fprintf(w, "has_credits: %v\n", u.HasCredits)
			if u.AutoReload != nil && u.AutoReload.Enabled {
				fmt.Fprintf(w, "auto_reload: enabled (threshold: %d, top_up: %d)\n",
					u.AutoReload.ThresholdCred, u.AutoReload.TopUpCredits)
			}
			if !u.HasCredits {
				fmt.Fprintln(w, "")
				fmt.Fprintln(w, yellow("warning: balance is empty. Top up at https://happenstance.ai/settings/api-keys"))
			}
			return nil
		},
	}
}

func newAPIHpnUserCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "user",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "Show the current Happenstance public-API user (email, name, friends) (free)",
		Long:    `Calls GET /v1/users/me. Returns the email, name, and friends list. The canonical liveness probe — every doctor invocation hits this endpoint first to confirm the bearer key is valid. Free probe — no credits spent.`,
		Example: `  contact-goat-pp-cli api hpn user --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newHappenstanceAPIClient()
			if err != nil {
				return err
			}
			u, err := c.Me(cmd.Context())
			if err != nil {
				return classifyHpnError(err)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{
					"source":  "api",
					"email":   u.Email,
					"name":    u.Name,
					"friends": u.Friends,
				})
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "email: %s\n", u.Email)
			fmt.Fprintf(w, "name:  %s\n", u.Name)
			fmt.Fprintf(w, "friends: %d\n", len(u.Friends))
			return nil
		},
	}
}
