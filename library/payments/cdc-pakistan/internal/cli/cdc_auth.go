// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Custom auth surface, in its own file so `generate --force` preserves it.
//
// CDC needs a Cloudflare clearance cookie, which cannot be minted by HTTP: it
// requires a real browser to execute the challenge. Rather than ship a browser
// dependency, these commands accept a cookie the operator captured themselves
// and store it WITH the User-Agent that minted it and the mint time. All three
// are required -- see the Clearance doc comment for why.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		authCmd, _, err := root.Find([]string{"auth"})
		if err != nil || authCmd == nil {
			return
		}
		addNovelCommandIfAbsent(authCmd, newCDCClearanceCmd(flags))
	})
}

func newCDCClearanceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clearance",
		Short: "Manage the Cloudflare clearance cookie CDC requires",
		Long: strings.Trim(`
Manage the Cloudflare clearance cookie.

Every path on cdcpakistan.com is behind a JS challenge, so a cookie minted by a
real browser is required. Once minted it replays over ordinary HTTP -- no
browser stays running -- but its server-side lifetime is a HARD ~30 MINUTES from
mint, regardless of traffic. The cookie's own expiry attribute claims a year and
is not to be trusted.

To capture one:

  1. Open https://www.cdcpakistan.com/ in Chrome and let the challenge clear.
  2. DevTools -> Application -> Cookies -> copy the cf_clearance value.
  3. DevTools console: copy(navigator.userAgent)
  4. printf '%s' '<cf_clearance value>' | cdc-pakistan-pp-cli auth clearance set --user-agent '<the UA>'

The cookie is bound to that exact User-Agent; a mismatch fails closed.
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCDCClearanceSetCmd(flags), newCDCClearanceStatusCmd(flags))
	return cmd
}

func newCDCClearanceSetCmd(flags *rootFlags) *cobra.Command {
	var userAgent, cookie, mintedAt string
	cmd := &cobra.Command{
		Use:     "set",
		Short:   "Store a cf_clearance cookie together with its User-Agent and mint time",
		Example: "  printf '%s' \"$CF\" | cdc-pakistan-pp-cli auth clearance set --user-agent \"$UA\"",
		Annotations: map[string]string{
			"mcp:read-only":       "false",
			"mcp:local-write":     "true",
			"pp:happy-args":       "--user-agent=Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36;--cookie=example-clearance-value",
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "auth clearance set")
			}
			if userAgent == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--user-agent is required: cf_clearance is bound to the User-Agent that minted it and is unusable without it"))
			}
			val := strings.TrimSpace(cookie)
			if val == "" {
				b, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 1<<16))
				if err != nil {
					return usageErr(fmt.Errorf("reading cookie from stdin: %w", err))
				}
				val = strings.TrimSpace(string(b))
			}
			if val == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("no cookie provided: pass --cookie or pipe the value on stdin"))
			}
			val = strings.TrimPrefix(val, "cf_clearance=")

			cl := &Clearance{Cookie: val, UserAgent: userAgent, MintedAt: time.Now()}
			if mintedAt != "" {
				t, err := time.Parse(time.RFC3339, mintedAt)
				if err != nil {
					return usageErr(fmt.Errorf("--minted-at must be RFC3339, got %q: %w", mintedAt, err))
				}
				cl.MintedAt = t
			}

			p := clearancePath(flags)
			if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
				return configErr(fmt.Errorf("creating config dir: %w", err))
			}
			b, err := json.MarshalIndent(cl, "", "  ")
			if err != nil {
				return err
			}
			// 0600: this is credential material.
			if err := os.WriteFile(p, b, 0o600); err != nil {
				return configErr(fmt.Errorf("writing clearance: %w", err))
			}

			out := map[string]any{
				"stored_at":         p,
				"user_agent":        cl.UserAgent,
				"minted_at":         cl.MintedAt.Format(time.RFC3339),
				"expires_at":        cl.MintedAt.Add(clearanceLifetime).Format(time.RFC3339),
				"remaining_seconds": int(cl.Remaining().Seconds()),
				"cookie_length":     len(cl.Cookie),
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "clearance stored at %s\n", p)
			fmt.Fprintf(cmd.OutOrStdout(), "  usable for about %s (hard ~30 min from mint)\n", cl.Remaining().Round(time.Second))
			return nil
		},
	}
	cmd.Flags().StringVar(&userAgent, "user-agent", "", "User-Agent that minted the cookie (required; the cookie is bound to it)")
	cmd.Flags().StringVar(&cookie, "cookie", "", "cf_clearance value (omit to read it from stdin, which keeps it out of your shell history)")
	cmd.Flags().StringVar(&mintedAt, "minted-at", "", "RFC3339 mint time (default: now)")
	return cmd
}

func newCDCClearanceStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Show whether the stored clearance is still inside its measured window",
		Example:     "  cdc-pakistan-pp-cli auth clearance status --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,4"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "auth clearance status")
			}
			cl, err := LoadClearance(flags)
			if err != nil {
				// Absent or unusable clearance is a real state, reported as JSON
				// for agents rather than only as an error string.
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"configured": false,
						"usable":     false,
						"reason":     err.Error(),
					}, flags)
				}
				return err
			}
			rem := cl.Remaining()
			out := map[string]any{
				"configured":        true,
				"usable":            rem > clearanceMargin,
				"minted_at":         cl.MintedAt.Format(time.RFC3339),
				"expires_at":        cl.MintedAt.Add(clearanceLifetime).Format(time.RFC3339),
				"remaining_seconds": int(rem.Seconds()),
				"user_agent":        cl.UserAgent,
				"note":              "measured hard lifetime is ~30 minutes from mint; the cookie's own expires attribute claims 365 days and is not trustworthy",
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if rem <= 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "clearance EXPIRED %s ago -- re-mint it\n", (-rem).Round(time.Second))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "clearance usable, about %s left\n", rem.Round(time.Second))
			return nil
		},
	}
	return cmd
}
