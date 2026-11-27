// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
	"github.com/spf13/cobra"
)

func newAccountCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Inspect Granola account profile, subscriptions, and active sessions",
	}
	cmd.AddCommand(newAccountInfoCmd(flags))
	cmd.AddCommand(newAccountSubscriptionsCmd(flags))
	cmd.AddCommand(newAccountSessionsCmd(flags))
	return cmd
}

func newAccountInfoCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Fetch current user profile and workspace IDs via the internal API",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flags.dataSource == "local" {
				return notFoundErr(fmt.Errorf("account info requires live internal API access"))
			}
			ic, err := granola.NewInternalClient()
			if err != nil {
				return authErr(err)
			}
			raw, err := ic.GetUserInfo()
			if err != nil {
				return apiErr(err)
			}
			return emitJSON(cmd, flags, raw)
		},
	}
}

func newAccountSubscriptionsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "subscriptions",
		Short: "Fetch active subscription and plan metadata via the internal API",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flags.dataSource == "local" {
				return notFoundErr(fmt.Errorf("account subscriptions requires live internal API access"))
			}
			ic, err := granola.NewInternalClient()
			if err != nil {
				return authErr(err)
			}
			raw, err := ic.GetSubscriptions()
			if err != nil {
				return apiErr(err)
			}
			return emitJSON(cmd, flags, raw)
		},
	}
}

func newAccountSessionsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "sessions",
		Short: "List active Granola user sessions via the internal API",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flags.dataSource == "local" {
				return notFoundErr(fmt.Errorf("account sessions requires live internal API access"))
			}
			ic, err := granola.NewInternalClient()
			if err != nil {
				return authErr(err)
			}
			raw, err := ic.ListUserSessions()
			if err != nil {
				return apiErr(err)
			}
			return emitJSON(cmd, flags, raw)
		},
	}
}
