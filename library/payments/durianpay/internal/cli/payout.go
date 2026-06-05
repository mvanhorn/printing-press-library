// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: smart payout routing (SNAP inquiry-then-transfer, legacy batch pointer).
package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/durianpay/internal/store"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelPayoutCmd(flags *rootFlags) *cobra.Command {
	var dest, bank, account, name, amount, currency, customerNumber, wallet, email, remark, surface, externalID string
	var skipInquiry bool

	cmd := &cobra.Command{
		Use:   "payout",
		Short: "Send a pay-out via SNAP (account inquiry first, then transfer), with legacy batch as an explicit option",
		Long: strings.Trim(`
Use this command to send money to a bank account or e-wallet. It routes to the
SNAP surface per company policy: verifies the destination with an account
inquiry, then submits the transfer with a signed request. Use
--surface legacy to be pointed at the legacy batch flow instead
('disbursements submit' supports multi-item batches and approval workflows).
Do NOT use this command to charge a customer; use 'pay' for accepting payments.
`, "\n"),
		Example: strings.Trim(`
  durianpay-pp-cli payout --bank 014 --account 1234567890 --amount 250000.00 --name "Budi Santoso" --dry-run
  durianpay-pp-cli payout --dest ewallet --wallet gopay --customer-number 081234567890 --amount 100000.00
`, "\n"),
		Annotations: map[string]string{
			"pp:happy-args": "--bank=014;--account=1234567890;--amount=250000.00;--name=Budi",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			// Infer destination type from flags when --dest is not given.
			if dest == "" {
				if wallet != "" || (customerNumber != "" && bank == "") {
					dest = "ewallet"
				} else {
					dest = "bank"
				}
			}
			// Legacy intercept runs before resolveRoute: payout routes reject
			// --surface legacy at the resolver level, but we want the friendly
			// 'disbursements submit' guidance to win.
			if strings.EqualFold(surface, "legacy") {
				fmt.Fprintln(cmd.OutOrStdout(), "Legacy pay-outs are batch-based: use 'durianpay-pp-cli disbursements submit' (supports X-Idempotency-Key, force_disburse, approval flow).")
				return nil
			}
			route, err := resolveRoute(payoutRoutes, dest, surface)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if err := snapDataSourceGuard(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				inquiry := "/v1.0/account-inquiry-external"
				if route.Method != "bank" {
					inquiry = "/v1.0/emoney/account-inquiry"
				}
				if flags.asJSON || flags.agent {
					return flags.printJSON(cmd, map[string]any{
						"dry_run": true, "dest": route.Method, "surface": route.Surface,
						"reason": route.Reason, "would_inquire": inquiry, "would_post": route.SNAPPath,
					})
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would route %s payout to the %s surface (%s)\n", route.Method, route.Surface, route.Reason)
				fmt.Fprintf(cmd.OutOrStdout(), "would POST %s (verify destination)\n", inquiry)
				fmt.Fprintf(cmd.OutOrStdout(), "would POST %s\n", route.SNAPPath)
				return nil
			}
			if amount == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--amount is required"))
			}
			if currency == "" {
				currency = "IDR"
			}
			c, err := snapClientFromFlags(flags)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			refNo := snapAutoRef("")

			if route.Method == "bank" {
				if bank == "" || account == "" {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--bank and --account are required for bank payouts"))
				}
				if !skipInquiry {
					inqBody := mustJSON(map[string]any{
						"beneficiaryBankCode":  bank,
						"beneficiaryAccountNo": account,
						"partnerReferenceNo":   refNo + "-inq",
					})
					raw, status, err := c.Do(ctx, "POST", "/v1.0/account-inquiry-external", inqBody, "")
					if err != nil {
						return classifySNAPCallError(err, flags)
					}
					if err := checkSNAPEnvelope(raw, status); err != nil {
						_ = printJSONFiltered(cmd.OutOrStdout(), raw, flags)
						return apiErr(fmt.Errorf("destination account inquiry failed: %w", err))
					}
				}
				body := buildTransferBody(amount, currency, bank, account, name, refNo, c.Config().MerchantID, email, remark)
				raw, status, err := c.Do(ctx, "POST", route.SNAPPath, mustJSON(body), externalID)
				if err != nil {
					return classifySNAPCallError(err, flags)
				}
				recordPayoutLocally(flags, refNo, raw)
				if err := checkSNAPEnvelope(raw, status); err != nil {
					_ = printJSONFiltered(cmd.OutOrStdout(), raw, flags)
					return apiErr(err)
				}
				return printJSONFiltered(cmd.OutOrStdout(), raw, flags)
			}

			// e-wallet payout
			if customerNumber == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--customer-number is required for e-wallet payouts"))
			}
			if !skipInquiry {
				inq := buildInquiryEwalletBody(wallet, customerNumber, refNo+"-inq")
				inqBody := mustJSON(inq)
				raw, status, err := c.Do(ctx, "POST", "/v1.0/emoney/account-inquiry", inqBody, "")
				if err != nil {
					return classifySNAPCallError(err, flags)
				}
				if err := checkSNAPEnvelope(raw, status); err != nil {
					_ = printJSONFiltered(cmd.OutOrStdout(), raw, flags)
					return apiErr(fmt.Errorf("destination e-wallet inquiry failed: %w", err))
				}
			}
			body := buildEwalletTransferBody(amount, currency, customerNumber, name, wallet, refNo)
			raw, status, err := c.Do(ctx, "POST", route.SNAPPath, mustJSON(body), externalID)
			if err != nil {
				return classifySNAPCallError(err, flags)
			}
			recordPayoutLocally(flags, refNo, raw)
			if err := checkSNAPEnvelope(raw, status); err != nil {
				_ = printJSONFiltered(cmd.OutOrStdout(), raw, flags)
				return apiErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&dest, "dest", "", "Destination type: bank or ewallet (inferred from flags when omitted)")
	cmd.Flags().StringVar(&bank, "bank", "", "SNAP numeric bank code (e.g. 014 = BCA, 002 = BRI, 008 = Mandiri); legacy batch uses slug codes (bca)")
	cmd.Flags().StringVar(&account, "account", "", "Beneficiary account number")
	cmd.Flags().StringVar(&name, "name", "", "Beneficiary account holder name")
	cmd.Flags().StringVar(&amount, "amount", "", "Amount in IDR, e.g. 250000.00")
	cmd.Flags().StringVar(&currency, "currency", "IDR", "Currency code")
	cmd.Flags().StringVar(&customerNumber, "customer-number", "", "E-wallet customer number (phone)")
	cmd.Flags().StringVar(&wallet, "wallet", "", "E-wallet platform (gopay, dana, ovo, linkaja, shopeepay)")
	cmd.Flags().StringVar(&email, "email", "", "Beneficiary email for notification")
	cmd.Flags().StringVar(&remark, "remark", "", "Transfer remark/notes")
	cmd.Flags().StringVar(&surface, "surface", "auto", "API surface override: auto, snap, or legacy")
	cmd.Flags().StringVar(&externalID, "external-id", "", "X-EXTERNAL-ID (default: auto-generated)")
	cmd.Flags().BoolVar(&skipInquiry, "skip-inquiry", false, "Skip the destination account inquiry step")
	return cmd
}

// checkSNAPEnvelope returns an error for non-success SNAP responses, delegating
// to the single shared classifier (snapResponseFields + snapResponseOK) so 202
// "Request In Progress" codes are treated as accepted submissions, not errors.
func checkSNAPEnvelope(raw json.RawMessage, status int) error {
	code, message := snapResponseFields(raw)
	if snapResponseOK(status, code) {
		return nil
	}
	return fmt.Errorf("HTTP %d, responseCode %s: %s — decode it with 'durianpay-pp-cli explain %s'",
		status, code, message, code)
}

// recordPayoutLocally upserts the payout response into the local store so
// 'stuck' and reconciliation queries can see CLI-initiated disbursements.
// Best-effort: failures are reported to stderr but never fail the payout.
func recordPayoutLocally(flags *rootFlags, refNo string, raw json.RawMessage) {
	db, err := store.Open(defaultDBPath("durianpay-pp-cli"))
	if err != nil {
		return
	}
	defer db.Close()
	var m map[string]any
	if json.Unmarshal(raw, &m) == nil {
		m["partner_reference_no"] = refNo
		m["recorded_at"] = time.Now().UTC().Format(time.RFC3339)
		if _, ok := m["status"]; !ok {
			m["status"] = "pending"
		}
		data, _ := json.Marshal(m)
		_ = db.Upsert("disbursements", refNo, data)
	}
}
