// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: disbursement completion-signature verification (HMAC dis_id|amount).
package cli

import (
	"crypto/hmac"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/durianpay/internal/config"
	"github.com/mvanhorn/printing-press-library/library/payments/durianpay/internal/snap"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelDisbursementsVerifyCompletionCmd(flags *rootFlags) *cobra.Command {
	var id, amount, signature, keyOverride string

	cmd := &cobra.Command{
		Use:   "verify-completion",
		Short: "Verify a disbursement-completion callback signature (HMAC-SHA256 of dis_id|amount with your API key)",
		Long: strings.Trim(`
Use this command to verify a disbursement-completion callback signature.
Durianpay computes it as lowerhex(HMAC-SHA256(disbursement_id + "|" + amount))
keyed with your API key (use the dp_test key for sandbox events, dp_live for
live). Amount must be in the "15000.00" format from the webhook payload.
Do NOT use this command for general legacy/SNAP webhook payload verification;
use 'webhook verify' instead.
`, "\n"),
		Example: strings.Trim(`
  durianpay-pp-cli disbursements verify-completion --id dis_abc123 --amount 50000.00 --key dp_test_demo --signature 936658ef04244256212e98d13c1059dc606f777fcbfac7fdbdb7bb65f86bd196
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--id=dis_abc123;--amount=50000.00;--key=dp_test_demo;--signature=936658ef04244256212e98d13c1059dc606f777fcbfac7fdbdb7bb65f86bd196",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would recompute the completion signature locally (no API call)")
				return nil
			}
			if id == "" || amount == "" || signature == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--id, --amount, and --signature are all required"))
			}
			apiKey := keyOverride
			if apiKey == "" {
				cfg, err := config.Load(flags.configPath)
				if err != nil {
					return configErr(err)
				}
				apiKey = cfg.DurianpayApiKey
			}
			if apiKey == "" {
				return authErr(fmt.Errorf("DURIANPAY_API_KEY is not set; the completion signature is keyed with your API key (or pass --key)"))
			}
			expected := snap.LegacyCompletionSignature(id, amount, apiKey)
			valid := hmac.Equal([]byte(strings.ToLower(signature)), []byte(expected))
			out := map[string]any{
				"valid":           valid,
				"disbursement_id": id,
				"amount":          amount,
			}
			if !valid {
				out["hint"] = "mismatch causes: wrong environment key (dp_test vs dp_live), amount not in '15000.00' format, or tampered payload"
				if err := flags.printJSON(cmd, out); err != nil {
					return err
				}
				return fmt.Errorf("signature mismatch for %s", id)
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Disbursement ID from the webhook (dis_xxx)")
	cmd.Flags().StringVar(&amount, "amount", "", "Amount exactly as sent in the webhook, e.g. 50000.00")
	cmd.Flags().StringVar(&signature, "signature", "", "Signature value from the webhook payload (lowercase hex)")
	cmd.Flags().StringVar(&keyOverride, "key", "", "API key to verify with (default: DURIANPAY_API_KEY / config; note --key is visible in process listings — prefer the env var)")
	return cmd
}
