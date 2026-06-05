// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: smart payment routing across the SNAP and legacy surfaces.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelPayCmd(flags *rootFlags) *cobra.Command {
	var method, amount, currency, orderID, bank, wallet, customerNumber, surface, bodyArg, externalID string

	cmd := &cobra.Command{
		Use:   "pay",
		Short: "Charge a payment with the right API surface chosen automatically (SNAP where supported, legacy otherwise)",
		Long: strings.Trim(`
Use this command to charge/accept a payment and let the CLI pick the correct
surface per company policy: SNAP for QRIS, e-wallet, and virtual accounts;
legacy for cards, BNPL, online banking, and retail stores. Override with
--surface snap|legacy.
Do NOT use this command to send money to a recipient; use 'payout' for
disbursements. For an explicit single-surface call use the generated
'payments charge' or 'snap ...' commands.
`, "\n"),
		Example: strings.Trim(`
  durianpay-pp-cli pay --method qris --amount 50000.00 --dry-run
  durianpay-pp-cli pay --method va --amount 150000.00 --bank bca --customer-number 081234567890
  durianpay-pp-cli pay --method card --amount 200000.00 --order-id ord_abc123 --surface legacy
`, "\n"),
		Annotations: map[string]string{
			"pp:happy-args": "--method=qris;--amount=50000.00",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if method == "" && !flags.dryRun {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--method is required (qris, ewallet, va, card, bnpl, online-banking, retail-store)"))
			}
			if method == "" {
				method = "qris"
			}
			route, err := resolveRoute(paymentRoutes, method, surface)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			// SNAP routes are live-only: reject --data-source local before any
			// dry-run print or client call (guard wins).
			if route.Surface == SurfaceSNAP {
				if err := snapDataSourceGuard(flags); err != nil {
					return err
				}
			}
			if dryRunOK(flags) {
				target := "/payments/charge (type=" + route.LegacyType + ")"
				if route.Surface == SurfaceSNAP {
					target = route.SNAPPath
				}
				if flags.asJSON || flags.agent {
					return flags.printJSON(cmd, map[string]any{
						"dry_run": true, "method": route.Method,
						"surface": route.Surface, "reason": route.Reason,
						"would_post": target,
					})
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would route method %q to the %s surface (%s)\n", route.Method, route.Surface, route.Reason)
				fmt.Fprintf(cmd.OutOrStdout(), "would POST %s\n", target)
				return nil
			}
			if amount == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--amount is required"))
			}
			if currency == "" {
				currency = "IDR"
			}
			extra, err := readBodyArg(bodyArg)
			if err != nil {
				return usageErr(err)
			}

			if route.Surface == SurfaceSNAP {
				body := buildSNAPPayBody(route, amount, currency, bank, wallet, customerNumber)
				if len(extra) > 0 {
					if err := mergeJSONInto(body, extra); err != nil {
						return usageErr(fmt.Errorf("--body must be a JSON object: %w", err))
					}
				}
				c, err := snapClientFromFlags(flags)
				if err != nil {
					return err
				}
				raw, status, err := c.Do(cmd.Context(), "POST", route.SNAPPath, mustJSON(body), externalID)
				if err != nil {
					return classifySNAPCallError(err, flags)
				}
				return printSNAPResult(cmd, flags, raw, status, route)
			}

			// Legacy surface: POST /payments/charge with the method-specific type.
			if orderID == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("legacy charge requires --order-id (create one with 'orders create')"))
			}
			request := map[string]any{"order_id": orderID, "amount": amount}
			if bank != "" {
				request["bank_code"] = bank
			}
			if wallet != "" {
				request["wallet"] = wallet
			}
			if customerNumber != "" {
				request["mobile"] = customerNumber
			}
			if len(extra) > 0 {
				if err := mergeJSONInto(request, extra); err != nil {
					return usageErr(fmt.Errorf("--body must be a JSON object: %w", err))
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, _, err := c.Post(cmd.Context(), "/payments/charge", map[string]any{"type": route.LegacyType, "request": request})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&method, "method", "", "Payment method: qris, ewallet, va, card, bnpl, online-banking, retail-store")
	cmd.Flags().StringVar(&amount, "amount", "", "Amount in IDR, e.g. 50000.00")
	cmd.Flags().StringVar(&currency, "currency", "IDR", "Currency code")
	cmd.Flags().StringVar(&orderID, "order-id", "", "Order ID (required for legacy-surface methods)")
	cmd.Flags().StringVar(&bank, "bank", "", "Bank code: legacy charge uses slugs (bca); SNAP VA uses numeric codes (014)")
	cmd.Flags().StringVar(&wallet, "wallet", "", "E-wallet provider (e.g. gopay, ovo, dana)")
	cmd.Flags().StringVar(&customerNumber, "customer-number", "", "Customer phone/account number for e-wallet or VA")
	cmd.Flags().StringVar(&surface, "surface", "auto", "API surface override: auto, snap, or legacy")
	cmd.Flags().StringVar(&bodyArg, "body", "", "Extra body fields merged in: inline JSON, @file.json, or '-'")
	cmd.Flags().StringVar(&externalID, "external-id", "", "X-EXTERNAL-ID for SNAP calls (default: auto-generated)")
	return cmd
}

// buildSNAPPayBody is a thin dispatcher over the canonical snap_endpoints.go
// body builders, so each endpoint has exactly one body builder. It maps the
// pay command's flags onto each builder's parameters.
func buildSNAPPayBody(route methodRoute, amount, currency, bank, wallet, customerNumber string) map[string]any {
	switch route.Method {
	case "qris":
		return buildQrisGenerateBody(amount, currency, "")
	case "ewallet":
		// wallet -> channelCode, customerNumber -> phoneNumber.
		return buildEwalletPayBody(amount, currency, "", customerNumber, wallet)
	case "va":
		// bank -> bankCode, customerNumber -> customerNo; closed-amount VA so
		// the amount is carried as totalAmount by the shared builder.
		return buildVaCreateBody("", customerNumber, "C", snapAutoRef(""), bank, amount, currency)
	default:
		return buildQrisGenerateBody(amount, currency, "")
	}
}

// printSNAPResult prints a SNAP response, surfacing non-success responses as
// errors via the single shared classifier (checkSNAPEnvelope).
func printSNAPResult(cmd *cobra.Command, flags *rootFlags, raw json.RawMessage, status int, route methodRoute) error {
	if err := checkSNAPEnvelope(raw, status); err != nil {
		_ = printJSONFiltered(cmd.OutOrStdout(), raw, flags)
		return apiErr(fmt.Errorf("SNAP %s failed: %w", route.SNAPPath, err))
	}
	return printJSONFiltered(cmd.OutOrStdout(), raw, flags)
}

// mustJSON marshals a map that cannot fail.
func mustJSON(v map[string]any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// mergeJSONInto merges a JSON object into dst (shallow).
func mergeJSONInto(dst map[string]any, raw []byte) error {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	for k, v := range m {
		dst[k] = v
	}
	return nil
}
