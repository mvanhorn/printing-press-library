// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// Hand-written: auth bootstrap (derive L2 credentials from L1 EOA PK).

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/polymarket/internal/config"
)

func newAuthDeriveCmd(flags *rootFlags) *cobra.Command {
	var funder, sigType string

	cmd := &cobra.Command{
		Use:   "derive",
		Short: "Derive L2 HMAC credentials (api_key/secret/passphrase) from your L1 EOA private key.",
		Long: `Bootstrap Polymarket trading credentials. Reads POLYMARKET_PRIVATE_KEY
from env, signs an EIP-712 challenge, calls /auth/api-key, and prints the
returned api_key/secret/passphrase trio you should export to
POLYMARKET_API_KEY/_SECRET/_PASSPHRASE.

NOTE: This build ships a documented HONEST stub. Real EIP-712 signing
requires go-ethereum (~300 LOC). See output for the exact request shape
and a path to live derivation via the official Rust CLI.`,
		Example: `  polymarket-pp-cli auth derive
  polymarket-pp-cli auth derive --funder 0xPOLYMARKET_PROXY --signature-type 2`,
		Annotations: map[string]string{"pp:onchain": "auth.derive"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			pk := os.Getenv("POLYMARKET_PRIVATE_KEY")
			cfg, _ := config.Load(flags.configPath)
			if pk == "" && cfg != nil {
				pk = cfg.PolymarketPrivateKey
			}
			if pk == "" {
				return authErr(fmt.Errorf("no POLYMARKET_PRIVATE_KEY in env or config. Run 'wallet create' or 'wallet import' first, or export POLYMARKET_PRIVATE_KEY=0x..."))
			}
			if !validatePKHex(pk) {
				return authErr(fmt.Errorf("invalid POLYMARKET_PRIVATE_KEY: expected 0x-prefixed 64-char hex"))
			}

			pkPreview := strings.TrimPrefix(pk, "0x")
			if len(pkPreview) > 8 {
				pkPreview = pkPreview[:4] + "..." + pkPreview[len(pkPreview)-4:]
			}

			out := map[string]any{
				"flow":           "POLYMARKET_PRIVATE_KEY detected; L2 derivation walkthrough",
				"pk_preview":     pkPreview,
				"pk_format":      "valid hex 0x-prefixed 64-char",
				"address":        "<derive_requires_secp256k1>",
				"funder":         funder,
				"signature_type": sigType,
				"expected_post": map[string]any{
					"url":    "https://clob.polymarket.com/auth/api-key",
					"method": "POST",
					"headers": map[string]string{
						"Content-Type":   "application/json",
						"POLY_ADDRESS":   "<derived EOA address>",
						"POLY_SIGNATURE": "<EIP-712 signature over create-api-key challenge>",
						"POLY_TIMESTAMP": "<unix seconds>",
						"POLY_NONCE":     "0",
					},
					"body": map[string]any{
						"nonce": 0,
					},
				},
				"expected_response": map[string]any{
					"apiKey":     "<uuid>",
					"secret":     "<base64>",
					"passphrase": "<string>",
				},
				"status":    "NOT_IMPLEMENTED",
				"next_step": "Run the official Polymarket Rust CLI: `polymarket clob create-api-key`. Paste the returned trio into POLYMARKET_API_KEY/_SECRET/_PASSPHRASE env vars (or `polymarket-pp-cli auth set-token` to persist). This command will sign natively in v0.2.",
				"why_stub":  "EIP-712 signing requires the secp256k1 curve + EIP-712 typed-data hashing (~300 LOC + go-ethereum dep). Honest stub keeps the build dep-light while mapping the function end-to-end.",
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&funder, "funder", "", "Optional funder address (Polymarket proxy wallet)")
	cmd.Flags().StringVar(&sigType, "signature-type", "0", "Signature type: 0 (EOA), 1 (Polymarket proxy), 2 (Polymarket gnosis-safe)")
	return cmd
}
