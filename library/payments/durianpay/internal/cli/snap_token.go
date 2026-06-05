// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: SNAP B2B access-token lifecycle (mint, cache, status).
package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelSnapTokenCmd(flags *rootFlags) *cobra.Command {
	var status bool
	var mint bool

	cmd := &cobra.Command{
		Use:   "token",
		Short: "Mint or inspect the SNAP B2B access token (900s TTL, cached on disk)",
		Long: strings.Trim(`
Mints a SNAP B2B access token by signing clientKey|timestamp with your RSA
private key (RSA-SHA256), then caches it on disk and reuses it until ~30s
before expiry. Use --status to inspect the cache without any network call.
`, "\n"),
		Example: strings.Trim(`
  durianpay-pp-cli snap token --status
  durianpay-pp-cli snap token --mint --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--status",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would mint or inspect the SNAP B2B access token")
				return nil
			}
			c, err := snapClientFromFlags(flags)
			if err != nil {
				return err
			}
			if status && !mint {
				tok, valid := c.CachedToken()
				out := map[string]any{"cached": tok != nil, "valid": valid}
				if tok != nil {
					out["minted_at"] = tok.MintedAt
					out["expires_at"] = tok.ExpiresAt
					out["expires_in_seconds"] = int(time.Until(tok.ExpiresAt).Seconds())
					out["environment"] = tok.Env
				} else {
					out["note"] = "no cached token; run 'snap token --mint' or any snap command to mint one"
				}
				return flags.printJSON(cmd, out)
			}
			tok, err := c.MintToken(cmd.Context())
			if err != nil {
				return authErr(err)
			}
			return flags.printJSON(cmd, map[string]any{
				"minted":             true,
				"minted_at":          tok.MintedAt,
				"expires_at":         tok.ExpiresAt,
				"expires_in_seconds": int(time.Until(tok.ExpiresAt).Seconds()),
				"environment":        tok.Env,
			})
		},
	}
	cmd.Flags().BoolVar(&status, "status", false, "Inspect the cached token without minting (offline)")
	cmd.Flags().BoolVar(&mint, "mint", false, "Force-mint a fresh token even if the cache is valid")
	return cmd
}
