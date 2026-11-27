// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
	"github.com/spf13/cobra"
)

// classifyInternalAPIErr maps InternalClient errors to documented exit codes:
// 4 for missing/rejected credentials, 7 for rate limiting, 5 for other API errors.
func classifyInternalAPIErr(err error) error {
	if err == nil {
		return nil
	}
	var rlErr *cliutil.RateLimitError
	if errors.As(err, &rlErr) {
		return rateLimitErr(err)
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "no granola token") ||
		strings.Contains(lower, "sign into the granola") ||
		strings.Contains(lower, "granola-pp-cli auth login") ||
		strings.Contains(lower, "no refresh token") ||
		strings.Contains(lower, "refresh failed") ||
		strings.Contains(lower, "status 401") ||
		strings.Contains(lower, "status 403") {
		return authErr(err)
	}
	return apiErr(err)
}

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
				return classifyInternalAPIErr(err)
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
				return classifyInternalAPIErr(err)
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
				return classifyInternalAPIErr(err)
			}
			return emitJSON(cmd, flags, raw)
		},
	}
}
