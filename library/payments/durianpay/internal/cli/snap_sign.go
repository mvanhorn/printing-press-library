// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: SNAP signature debugger — the #1 Durianpay integration pain.
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/durianpay/internal/snap"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelSnapSignCmd(flags *rootFlags) *cobra.Command {
	var method, path, bodyArg, token, externalID string
	var debug, tokenMode, showToken bool

	cmd := &cobra.Command{
		Use:   "sign",
		Short: "Build and inspect SNAP request signatures locally to diagnose 401/403 mismatches",
		Long: strings.Trim(`
Use this command to construct and inspect SNAP request signatures and diagnose
403 signature mismatches. It computes everything locally: the minified body,
its SHA-256 hex, the full string-to-sign
(METHOD:path:token:bodyHash:timestamp), and the HMAC-SHA512 X-SIGNATURE.
With --token-mode it instead builds the asymmetric RSA-SHA256 access-token
signature over clientKey|timestamp.
Do NOT use this command to verify an inbound webhook signature; use
'webhook verify' instead.
`, "\n"),
		Example: strings.Trim(`
  durianpay-pp-cli snap sign --method POST --path /v1.0/balance-inquiry --body '{"partnerReferenceNo":"ref-1"}' --debug
  durianpay-pp-cli snap sign --method POST --path /v1.0/transfer-interbank --body @req.json
  durianpay-pp-cli snap sign --token-mode --debug
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--method=POST;--path=/v1.0/balance-inquiry;--body={\"a\":1}",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute SNAP signatures locally (no API call)")
				return nil
			}
			c, err := snapClientFromFlags(flags)
			if err != nil {
				return err
			}
			cfg := c.Config()

			if tokenMode {
				if cfg.ClientKey == "" || cfg.PrivateKey == "" {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--token-mode needs DURIANPAY_SNAP_CLIENT_KEY and DURIANPAY_SNAP_PRIVATE_KEY set"))
				}
				ts := snap.Timestamp(time.Now())
				sig, err := snap.SignTokenRequest(cfg.PrivateKey, cfg.ClientKey, ts)
				if err != nil {
					return authErr(err)
				}
				shownKey := cfg.ClientKey
				if !showToken {
					shownKey = maskToken(cfg.ClientKey)
				}
				return flags.printJSON(cmd, map[string]any{
					"mode":           "token (asymmetric RSA-SHA256)",
					"string_to_sign": shownKey + "|" + ts,
					"timestamp":      ts,
					"x_signature":    sig,
					"headers": map[string]string{
						"X-TIMESTAMP":  ts,
						"X-SIGNATURE":  sig,
						"X-CLIENT-KEY": shownKey,
						"Content-Type": "application/json",
					},
				})
			}

			if method == "" || path == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--method and --path are required (or use --token-mode)"))
			}
			secretPlaceholder := false
			if cfg.ClientSecret == "" {
				// Debugger stays useful without credentials: sign with a
				// placeholder secret and say so, instead of refusing.
				cfg.ClientSecret = "<client-secret>"
				secretPlaceholder = true
			}
			body, err := readBodyArg(bodyArg)
			if err != nil {
				return usageErr(err)
			}
			tok := token
			usedCached := false
			if tok == "" {
				if cached, valid := c.CachedToken(); valid {
					tok = cached.AccessToken
					usedCached = true
				} else {
					tok = "<token>"
				}
			}
			sr := c.PrepareOffline(method, path, body, externalID, tok)
			if !debug && !flags.asJSON {
				fmt.Fprintln(cmd.OutOrStdout(), "X-SIGNATURE:", sr.Signature)
				fmt.Fprintln(cmd.OutOrStdout(), "X-TIMESTAMP:", sr.Timestamp)
				return nil
			}
			// Mask the real token in the displayed string-to-sign and
			// Authorization header unless --show-token is passed. The
			// placeholder "<token>" needs no masking, and X-SIGNATURE stays real
			// (it was computed over the real token).
			realToken := tok != "<token>"
			stringToSign := sr.StringToSign
			headers := sr.Headers
			if !showToken {
				// Copy headers so we never mutate the SignedRequest map.
				hc := make(map[string]string, len(sr.Headers))
				for k, v := range sr.Headers {
					hc[k] = v
				}
				if realToken {
					masked := maskToken(tok)
					stringToSign = strings.Replace(stringToSign, tok, masked, 1)
					hc["Authorization"] = "Bearer " + masked
				}
				// X-PARTNER-ID / X-CLIENT-KEY carry the client key, which the
				// merchant treats as a credential; mask both unless --show-token.
				if v := hc["X-PARTNER-ID"]; v != "" {
					hc["X-PARTNER-ID"] = maskToken(v)
				}
				if v := hc["X-CLIENT-KEY"]; v != "" {
					hc["X-CLIENT-KEY"] = maskToken(v)
				}
				headers = hc
			}
			out := map[string]any{
				"mode":           "transaction (symmetric HMAC-SHA512)",
				"method":         sr.Method,
				"path":           sr.Path,
				"timestamp":      sr.Timestamp,
				"minified_body":  sr.MinifiedBody,
				"body_sha256":    sr.BodySHA256,
				"string_to_sign": stringToSign,
				"x_signature":    sr.Signature,
				"headers":        headers,
				"token_source":   map[bool]string{true: "cached", false: "flag-or-placeholder"}[usedCached],
			}
			notes := []string{}
			if tok == "<token>" {
				notes = append(notes, "no cached token and no --token given; string-to-sign uses placeholder <token> — mint one with 'snap token --mint' for a real signature")
			}
			if realToken && !showToken {
				notes = append(notes, "the access token is masked in string_to_sign and headers.Authorization; signature was computed over the REAL token; pass --show-token to reveal it")
			}
			if secretPlaceholder {
				notes = append(notes, "DURIANPAY_SNAP_CLIENT_SECRET is not set; X-SIGNATURE was computed with placeholder <client-secret> and will not match the real one")
			}
			if len(notes) > 0 {
				out["note"] = strings.Join(notes, "; ")
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&method, "method", "", "HTTP method of the request being signed (e.g. POST)")
	cmd.Flags().StringVar(&path, "path", "", "Relative path including /v1.0 prefix (e.g. /v1.0/balance-inquiry)")
	cmd.Flags().StringVar(&bodyArg, "body", "", "Request body: inline JSON, @file.json, or '-' for stdin")
	cmd.Flags().StringVar(&token, "token", "", "B2B access token to sign with (default: cached token, else placeholder)")
	cmd.Flags().StringVar(&externalID, "external-id", "", "X-EXTERNAL-ID to include (default: auto-generated)")
	cmd.Flags().BoolVar(&debug, "debug", false, "Print every signing intermediate (minified body, hash, string-to-sign)")
	cmd.Flags().BoolVar(&tokenMode, "token-mode", false, "Build the asymmetric access-token signature instead of a transaction signature")
	cmd.Flags().BoolVar(&showToken, "show-token", false, "Reveal the real access token in string_to_sign and headers.Authorization (default: masked)")
	return cmd
}

// maskToken renders a real access token as first-6 + "…<masked>…" + last-4 so
// the signing inputs can be inspected without leaking the bearer token. Short
// tokens are fully masked.
func maskToken(tok string) string {
	if len(tok) <= 10 {
		return "…<masked>…"
	}
	return tok[:6] + "…<masked>…" + tok[len(tok)-4:]
}

// readBodyArg resolves a --body value: inline JSON, @path, or '-' (stdin).
func readBodyArg(arg string) ([]byte, error) {
	switch {
	case arg == "":
		return nil, nil
	case arg == "-":
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading body from stdin: %w", err)
		}
		return data, nil
	case strings.HasPrefix(arg, "@"):
		data, err := os.ReadFile(strings.TrimPrefix(arg, "@"))
		if err != nil {
			return nil, fmt.Errorf("reading body file: %w", err)
		}
		return data, nil
	default:
		return []byte(arg), nil
	}
}
