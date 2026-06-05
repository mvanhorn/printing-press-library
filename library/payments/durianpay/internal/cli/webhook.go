// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/durianpay/internal/snap"
	"github.com/spf13/cobra"
)

// pp:data-source local

type webhookVerifyResult struct {
	Valid  bool   `json:"valid"`
	Mode   string `json:"mode"`
	Detail string `json:"detail"`
}

// readPayload reads the webhook payload from a file path, or from stdin when
// path is "-".
func readPayload(cmd *cobra.Command, path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(cmd.InOrStdin())
	}
	// #nosec G304 -- path is the webhook payload file the user explicitly passed.
	return os.ReadFile(path)
}

func newWebhookCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "webhook",
		Short:       "Verify Durianpay webhook signatures locally.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWebhookVerifyCmd(flags))
	return cmd
}

func newWebhookVerifyCmd(flags *rootFlags) *cobra.Command {
	var flagPayload string
	var flagSignature string
	var flagPublicKey string
	var flagLegacy bool
	var flagID string
	var flagAmount string

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify a webhook signature: SNAP RSA mode (default) or legacy HMAC mode (--legacy).",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--legacy;--id=dis_test;--amount=10000.00",
		},
		Long: `Verify a Durianpay webhook signature locally — no network call.

SNAP RSA mode (default): supply --payload (file or '-' for stdin),
--signature (base64), and --public-key (Durianpay's public-key PEM file).

Legacy HMAC mode (--legacy): recompute HMAC-SHA256(id|amount) with your
DURIANPAY_API_KEY and compare to --signature. Requires --id and --amount;
omit --signature to print the expected value instead.

Exits 0 when the signature is valid, non-zero when invalid.`,
		Example: strings.TrimLeft(`
  durianpay-pp-cli webhook verify --legacy --id dis_123 --amount 10000.00
  durianpay-pp-cli webhook verify --legacy --id dis_123 --amount 10000.00 --signature "$SIG"
  durianpay-pp-cli webhook verify --payload body.json --signature "$SIG" --public-key dp.pem
  cat body.json | durianpay-pp-cli webhook verify --payload - --signature "$SIG" --public-key dp.pem`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			if flagLegacy {
				if flagID == "" || flagAmount == "" {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--legacy mode requires --id and --amount"))
				}
				apiKey := os.Getenv("DURIANPAY_API_KEY")
				if apiKey == "" {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--legacy mode needs DURIANPAY_API_KEY in the environment (export DURIANPAY_API_KEY=<your-key>)"))
				}
				expected := snap.LegacyCompletionSignature(flagID, flagAmount, apiKey)
				if flagSignature == "" {
					// No signature to compare: print the expected value so
					// callers can generate it server-side.
					return flags.printJSON(cmd, map[string]any{
						"mode": "legacy", "disbursement_id": flagID,
						"amount": flagAmount, "expected_signature": expected,
					})
				}
				valid := hmacEqual(expected, flagSignature)
				return emitWebhookResult(cmd, flags, webhookVerifyResult{
					Valid:  valid,
					Mode:   "legacy-hmac",
					Detail: legacyDetail(valid),
				})
			}

			// SNAP RSA mode.
			if flagPayload == "" || flagSignature == "" || flagPublicKey == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("SNAP RSA mode requires --payload, --signature, and --public-key (or use --legacy)"))
			}
			payload, err := readPayload(cmd, flagPayload)
			if err != nil {
				return fmt.Errorf("reading payload: %w", err)
			}
			// #nosec G304 -- flagPublicKey is the public-key path the user passed via --public-key.
			pubKey, err := os.ReadFile(flagPublicKey)
			if err != nil {
				return fmt.Errorf("reading public key: %w", err)
			}
			verr := snap.VerifyRSASignature(string(pubKey), payload, flagSignature)
			valid := verr == nil
			detail := "RSA-SHA256 signature verified against the supplied public key"
			if !valid {
				detail = "RSA signature verification failed: " + verr.Error()
			}
			return emitWebhookResult(cmd, flags, webhookVerifyResult{
				Valid:  valid,
				Mode:   "snap-rsa",
				Detail: detail,
			})
		},
	}
	cmd.Flags().StringVar(&flagPayload, "payload", "", "Path to the JSON payload file, or '-' for stdin (SNAP RSA mode)")
	cmd.Flags().StringVar(&flagSignature, "signature", "", "Signature to verify (base64 for RSA, lowerhex for legacy HMAC)")
	cmd.Flags().StringVar(&flagPublicKey, "public-key", "", "Path to Durianpay's public-key PEM file (SNAP RSA mode)")
	cmd.Flags().BoolVar(&flagLegacy, "legacy", false, "Use legacy HMAC mode: recompute HMAC-SHA256(id|amount) with DURIANPAY_API_KEY")
	cmd.Flags().StringVar(&flagID, "id", "", "Disbursement id (legacy HMAC mode)")
	cmd.Flags().StringVar(&flagAmount, "amount", "", "Amount string (legacy HMAC mode)")
	return cmd
}

func legacyDetail(valid bool) string {
	if valid {
		return "recomputed HMAC-SHA256(id|amount) matches the supplied signature"
	}
	return "recomputed HMAC-SHA256(id|amount) does NOT match the supplied signature"
}

// hmacEqual compares two hex signatures in constant time after normalizing case.
func hmacEqual(a, b string) bool {
	a = strings.TrimSpace(strings.ToLower(a))
	b = strings.TrimSpace(strings.ToLower(b))
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

// emitWebhookResult prints the result and returns a non-zero (plain) error
// when the signature is invalid so the process exits 1.
func emitWebhookResult(cmd *cobra.Command, flags *rootFlags, res webhookVerifyResult) error {
	if flags.asJSON {
		if err := printJSONFiltered(cmd.OutOrStdout(), res, flags); err != nil {
			return err
		}
	} else {
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "valid:  %t\n", res.Valid)
		fmt.Fprintf(out, "mode:   %s\n", res.Mode)
		fmt.Fprintf(out, "detail: %s\n", res.Detail)
	}
	if !res.Valid {
		return fmt.Errorf("signature invalid")
	}
	return nil
}
