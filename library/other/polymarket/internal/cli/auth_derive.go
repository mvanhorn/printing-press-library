// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// auth derive — live EIP-712 derivation of L2 HMAC credentials.
//
// Flow:
//  1. Load EOA private key from env (PK_FOR_POLYMARKET_LOGIN or
//     POLYMARKET_PRIVATE_KEY) or config.toml.
//  2. Build the ClobAuth typed-data with current unix timestamp.
//  3. Compute the EIP-712 v4 digest.
//  4. Sign with secp256k1 (V normalized to 27/28).
//  5. POST /auth/api-key with POLY_* headers; the body stays empty —
//     Polymarket carries every input in headers for this endpoint.
//  6. Persist apiKey / secret / passphrase to config.toml and print a
//     PREVIEW envelope (first/last 4 chars only) so the trio never
//     hits stdout / shell history in full.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"polymarket-pp-cli/internal/config"
	"polymarket-pp-cli/internal/eip712"
	"polymarket-pp-cli/internal/wallet"
)

func newAuthDeriveCmd(flags *rootFlags) *cobra.Command {
	var funder, sigType string
	var nonce int64

	cmd := &cobra.Command{
		Use:   "derive",
		Short: "Derive L2 HMAC credentials (api_key/secret/passphrase) from your L1 EOA private key.",
		Long: `Mint Polymarket L2 trading credentials. Reads PK_FOR_POLYMARKET_LOGIN
or POLYMARKET_PRIVATE_KEY from env (or config.toml), signs an EIP-712
ClobAuth challenge, POSTs to /auth/api-key, and persists the returned
apiKey / secret / passphrase to your config.

Output prints PREVIEWS only (first/last 4 chars of each credential) —
the full trio is written to ~/.config/polymarket-pp-cli/config.toml
with mode 0600. Use 'polymarket-pp-cli config show --reveal' to
display the raw values when needed.`,
		Example: `  polymarket-pp-cli auth derive
  polymarket-pp-cli auth derive --funder 0xPOLYMARKET_PROXY --signature-type 2`,
		Annotations: map[string]string{"pp:onchain": "auth.derive"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			signer, err := loadSigner(flags)
			if err != nil {
				return authErr(err)
			}

			ts := fmt.Sprintf("%d", time.Now().Unix())
			td := eip712.ClobAuthTypedData(&eip712.ClobAuth{
				Address:   signer.AddressHex(),
				Timestamp: ts,
				Nonce:     big.NewInt(nonce),
				Message:   eip712.DefaultClobAuthMessage,
			})
			digest, err := eip712.Digest(td)
			if err != nil {
				return fmt.Errorf("EIP-712 digest: %w", err)
			}
			sigHex, err := signer.SignToHex(digest)
			if err != nil {
				return fmt.Errorf("sign auth challenge: %w", err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			headers := map[string]string{
				"POLY_ADDRESS":   signer.AddressHex(),
				"POLY_SIGNATURE": sigHex,
				"POLY_TIMESTAMP": ts,
				"POLY_NONCE":     fmt.Sprintf("%d", nonce),
				"Content-Type":   "application/json",
			}

			// Polymarket exposes two endpoints with the same headers:
			//   GET /auth/derive-api-key   → returns existing creds for the
			//     signer address (deterministic; idempotent across calls).
			//   POST /auth/api-key         → mints a NEW key; returns 400
			//     once any key already exists for the address.
			// Try derive first (safe + idempotent); fall back to create only
			// when the address has no existing credentials. This matches
			// py-clob-client's standard bootstrap flow.
			var raw json.RawMessage
			var derived bool
			deriveRaw, deriveErr := c.GetWithHeaders(cmd.Context(),
				"https://clob.polymarket.com/auth/derive-api-key", nil, headers)
			if deriveErr == nil && len(deriveRaw) > 0 && !bytes.Equal(bytes.TrimSpace(deriveRaw), []byte("{}")) {
				// Polymarket sometimes wraps errors in 200 envelopes — parse to confirm.
				var maybeErr struct {
					Error string `json:"error"`
				}
				_ = json.Unmarshal(deriveRaw, &maybeErr)
				if maybeErr.Error == "" {
					raw = deriveRaw
					derived = true
				}
			}
			if !derived {
				var postStatus int
				var postErr error
				raw, postStatus, postErr = c.PostWithHeaders(cmd.Context(),
					"https://clob.polymarket.com/auth/api-key", map[string]any{}, headers)
				if postErr != nil {
					return classifyAPIError(postErr, flags)
				}
				if postStatus >= 400 {
					return fmt.Errorf("auth/api-key returned HTTP %d: %s (derive fallback also failed: %v)", postStatus, string(raw), deriveErr)
				}
			}

			var resp struct {
				APIKey     string `json:"apiKey"`
				Secret     string `json:"secret"`
				Passphrase string `json:"passphrase"`
			}
			if err := json.Unmarshal(raw, &resp); err != nil {
				return fmt.Errorf("parse /auth/api-key response: %w (body=%s)", err, string(raw))
			}
			if resp.APIKey == "" || resp.Secret == "" || resp.Passphrase == "" {
				return fmt.Errorf("auth/api-key response missing fields (apiKey/secret/passphrase): %s", string(raw))
			}

			cfg, _ := config.Load(flags.configPath)
			if cfg == nil {
				cfg = &config.Config{}
			}
			if funder != "" {
				cfg.PolymarketFunder = funder
			}
			if sigType != "" {
				cfg.PolymarketSignatureType = sigType
			}
			if err := cfg.SaveL2Credentials(resp.APIKey, resp.Secret, resp.Passphrase); err != nil {
				return fmt.Errorf("persist L2 creds: %w", err)
			}

			outStatus := "L2_CREDENTIALS_PERSISTED"
			if derived {
				outStatus = "L2_CREDENTIALS_DERIVED_AND_PERSISTED (existing key)"
			}
			out := map[string]any{
				"status":              outStatus,
				"address":             signer.AddressHex(),
				"funder":              cfg.PolymarketFunder,
				"signature_type":      cfg.PolymarketSignatureType,
				"api_key_preview":     preview(resp.APIKey),
				"secret_preview":      preview(resp.Secret),
				"passphrase_preview":  preview(resp.Passphrase),
				"persisted_to":        cfg.Path,
				"chain_id":            eip712.ChainID,
				"auth_message_signed": eip712.DefaultClobAuthMessage,
				"signed_at_unix":      ts,
				"next_step":           "L2 creds are now available for `orders create --broadcast`, `orders cancel`, and other authenticated CLOB endpoints. No action needed — the CLI reads them from config automatically.",
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&funder, "funder", "", "Optional funder address (Polymarket proxy wallet, signature-type 1 or 2)")
	cmd.Flags().StringVar(&sigType, "signature-type", "0", "Signature type: 0 (EOA), 1 (Polymarket proxy), 2 (Polymarket gnosis-safe)")
	cmd.Flags().Int64Var(&nonce, "nonce", 0, "Nonce in the ClobAuth typed data (server accepts 0 for first derive)")
	return cmd
}

// loadSigner pulls a *wallet.Signer from env first, falling back to the
// PolymarketPrivateKey field on the on-disk config. Returns a typed error
// so the auth subsystem can render a user-friendly message.
func loadSigner(flags *rootFlags) (*wallet.Signer, error) {
	if s, err := wallet.LoadFromEnv(); err == nil {
		return s, nil
	}
	cfg, _ := config.Load(flags.configPath)
	if cfg != nil && cfg.PolymarketPrivateKey != "" {
		return wallet.LoadFromString(cfg.PolymarketPrivateKey)
	}
	return nil, fmt.Errorf("no private key in env (PK_FOR_POLYMARKET_LOGIN or POLYMARKET_PRIVATE_KEY) or config.toml")
}

// preview returns a credential-safe display string: first/last 4 chars with
// the middle masked. Inputs shorter than 12 chars collapse to "***" so
// short test values (verify-mode, mocks) don't leak in their entirety.
func preview(s string) string {
	if len(s) < 12 {
		return "***"
	}
	return s[:4] + "..." + s[len(s)-4:] + fmt.Sprintf(" (len=%d)", len(s))
}

// keep strings import alive across re-orderings
var _ = strings.HasPrefix
