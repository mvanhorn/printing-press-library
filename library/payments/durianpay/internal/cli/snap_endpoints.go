// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored SNAP endpoint commands over the internal/snap signing transport.

package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// snapDataSourceGuard rejects --data-source local for live-only SNAP commands.
func snapDataSourceGuard(flags *rootFlags) error {
	if flags != nil && flags.dataSource == "local" {
		return usageErr(fmt.Errorf("SNAP commands require live API access; --data-source local is not supported"))
	}
	return nil
}

// snapAutoRef returns a time-based unique partnerReferenceNo when v is empty.
func snapAutoRef(v string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return fmt.Sprintf("pp-%d", time.Now().UnixNano())
}

// snapTxnDate returns the current time in the SNAP transactionDate layout
// (RFC3339 with a colon-separated numeric zone, no fractional seconds).
func snapTxnDate() string {
	return time.Now().Format("2006-01-02T15:04:05-07:00")
}

// snapAmount builds the SNAP amount object {"value":..,"currency":..}.
func snapAmount(value, currency string) map[string]any {
	if currency == "" {
		currency = "IDR"
	}
	return map[string]any{"value": value, "currency": currency}
}

// snapResponseOK reports whether an HTTP status + SNAP responseCode pair
// indicates success. SNAP success is an HTTP status < 400 AND a responseCode
// that is empty, begins with "200" (OK), or begins with "202" (per Durianpay
// docs, HTTP 202 / responseCode 202xx00 = "Request In Progress" — an accepted
// submission, NOT an error). The SNAP code convention is HHHSSCC where HHH
// echoes the HTTP family. An empty responseCode with a sub-400 status is
// treated as success (some endpoints return a bare body).
func snapResponseOK(status int, responseCode string) bool {
	if status >= 400 {
		return false
	}
	if responseCode == "" {
		return true
	}
	return strings.HasPrefix(responseCode, "200") || strings.HasPrefix(responseCode, "202")
}

// snapResponseFields extracts responseCode/responseMessage from a SNAP body.
func snapResponseFields(raw json.RawMessage) (code, message string) {
	var env struct {
		ResponseCode    string `json:"responseCode"`
		ResponseMessage string `json:"responseMessage"`
	}
	_ = json.Unmarshal(raw, &env)
	return env.ResponseCode, env.ResponseMessage
}

// runSnapPost marshals body, signs+sends it via the SNAP transport, classifies
// the result, and prints the response through the shared output pipeline.
func runSnapPost(cmd *cobra.Command, flags *rootFlags, method, path string, body map[string]any, externalID string) error {
	c, err := snapClientFromFlags(flags)
	if err != nil {
		return err
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling request body: %w", err)
	}
	ctx := cmd.Context()
	raw, status, err := c.Do(ctx, method, path, bodyBytes, externalID)
	if err != nil {
		return classifySNAPCallError(err, flags)
	}
	// Dry-run returns status 0 with the signing metadata; print and stop.
	if status == 0 && c.DryRun {
		return printJSONFiltered(cmd.OutOrStdout(), raw, flags)
	}
	code, message := snapResponseFields(raw)
	if !snapResponseOK(status, code) {
		return apiErr(fmt.Errorf("SNAP request failed (HTTP %d, responseCode %s): %s\nhint: run `durianpay-pp-cli explain %s` to decode this code",
			status, code, message, code))
	}
	return printJSONFiltered(cmd.OutOrStdout(), raw, flags)
}

// snapPostMethod is POST; PUT endpoints pass their own method.
const snapPOST = http.MethodPost

// --- body builders (factored out for testing) ---

func buildBalanceBody(accountNo, partnerRef string) map[string]any {
	b := map[string]any{"accountNo": accountNo}
	if partnerRef != "" {
		b["partnerReferenceNo"] = partnerRef
	}
	return b
}

// printSnapDryRun emits the dry-run "would <METHOD> <path>" notice, as a JSON
// envelope when --json/--agent is set so json_fidelity probes stay parseable.
func printSnapDryRun(cmd *cobra.Command, flags *rootFlags, method, path string) {
	if flags != nil && (flags.asJSON || flags.agent) {
		_ = flags.printJSON(cmd, map[string]any{"dry_run": true, "would_send": method + " " + path})
		return
	}
	fmt.Fprintf(cmd.OutOrStdout(), "would %s %s\n", method, path)
}

func buildInquiryBankBody(bankCode, accountNo string) map[string]any {
	return map[string]any{
		"beneficiaryBankCode":  bankCode,
		"beneficiaryAccountNo": accountNo,
	}
}

func buildInquiryEwalletBody(platform, customerNumber, partnerRef string) map[string]any {
	return map[string]any{
		"customerNumber":     customerNumber,
		"amount":             snapAmount("1", "IDR"),
		"partnerReferenceNo": snapAutoRef(partnerRef),
		"additionalInfo":     map[string]any{"platformCode": platform},
	}
}

func buildTransferBody(amount, currency, bankCode, accountNo, accountName, partnerRef, sourceAccountNo, email, remark string) map[string]any {
	if currency == "" {
		currency = "IDR"
	}
	b := map[string]any{
		"partnerReferenceNo":     snapAutoRef(partnerRef),
		"amount":                 snapAmount(amount, currency),
		"beneficiaryAccountName": accountName,
		"beneficiaryAccountNo":   accountNo,
		"beneficiaryBankCode":    bankCode,
		"sourceAccountNo":        sourceAccountNo,
		"transactionDate":        snapTxnDate(),
		"currency":               currency,
	}
	if email != "" {
		b["beneficiaryEmail"] = email
	}
	if remark != "" {
		b["customerReference"] = remark
	}
	return b
}

func buildTransferStatusBody(originalPartnerRef, originalRef, serviceCode string) map[string]any {
	if serviceCode == "" {
		serviceCode = "18"
	}
	b := map[string]any{"serviceCode": serviceCode}
	if originalPartnerRef != "" {
		b["originalPartnerReferenceNo"] = originalPartnerRef
	}
	if originalRef != "" {
		b["originalReferenceNo"] = originalRef
	}
	return b
}

func buildEwalletTransferBody(amount, currency, customerNumber, customerName, platform, partnerRef string) map[string]any {
	b := map[string]any{
		"partnerReferenceNo": snapAutoRef(partnerRef),
		"customerNumber":     customerNumber,
		"amount":             snapAmount(amount, currency),
		"transactionDate":    snapTxnDate(),
		"additionalInfo":     map[string]any{"platformCode": platform},
	}
	if customerName != "" {
		b["customerName"] = customerName
	}
	return b
}

func buildEwalletTransferStatusBody(originalPartnerRef, originalRef, serviceCode string) map[string]any {
	if serviceCode == "" {
		serviceCode = "38"
	}
	b := map[string]any{"serviceCode": serviceCode}
	if originalPartnerRef != "" {
		b["originalPartnerReferenceNo"] = originalPartnerRef
	}
	if originalRef != "" {
		b["originalReferenceNo"] = originalRef
	}
	return b
}

func buildEwalletPayBody(amount, currency, partnerRef, phoneNumber, channelCode string) map[string]any {
	return map[string]any{
		"partnerReferenceNo": snapAutoRef(partnerRef),
		"amount":             snapAmount(amount, currency),
		"additionalInfo": map[string]any{
			"phoneNumber": phoneNumber,
			"channelCode": channelCode,
		},
	}
}

func buildEwalletStatusBody(originalRef, serviceCode string) map[string]any {
	if serviceCode == "" {
		serviceCode = "54"
	}
	return map[string]any{
		"originalReferenceNo": originalRef,
		"serviceCode":         serviceCode,
	}
}

func buildEwalletCancelBody(partnerRef, originalRef, reason string) map[string]any {
	return map[string]any{
		"partnerReferenceNo":  partnerRef,
		"originalReferenceNo": originalRef,
		"reason":              reason,
	}
}

func buildRefundBody(originalPartnerRef, originalRef, partnerRefundNo, amount, currency string) map[string]any {
	return map[string]any{
		"originalPartnerReferenceNo": originalPartnerRef,
		"originalReferenceNo":        originalRef,
		"partnerRefundNo":            snapAutoRef(partnerRefundNo),
		"refundAmount":               snapAmount(amount, currency),
	}
}

func buildQrisGenerateBody(amount, currency, partnerRef string) map[string]any {
	return map[string]any{
		"partnerReferenceNo": snapAutoRef(partnerRef),
		"amount":             snapAmount(amount, currency),
		"additionalInfo":     map[string]any{},
	}
}

func buildQrisQueryBody(originalRef, serviceCode string) map[string]any {
	if serviceCode == "" {
		serviceCode = "47"
	}
	return map[string]any{
		"originalReferenceNo": originalRef,
		"serviceCode":         serviceCode,
	}
}

func buildQrisCancelBody(merchantID, originalRef, reason string) map[string]any {
	return map[string]any{
		"merchantId":          merchantID,
		"originalReferenceNo": originalRef,
		"reason":              reason,
	}
}

func buildVaCreateBody(name, customerNo, trxType, trxID, bankCode, amount, currency string) map[string]any {
	additional := map[string]any{"bankCode": bankCode}
	b := map[string]any{
		"virtualAccountName":    name,
		"virtualAccountTrxType": trxType,
		"trxId":                 trxID,
		"additionalInfo":        additional,
	}
	if customerNo != "" {
		b["customerNo"] = customerNo
	}
	if trxType == "C" && amount != "" {
		b["totalAmount"] = snapAmount(amount, currency)
	}
	return b
}

func buildVaUpdateBody(partnerServiceID, customerNo, virtualAccountNo, name, trxID, amount, currency string) map[string]any {
	b := map[string]any{
		"partnerServiceId":   partnerServiceID,
		"customerNo":         customerNo,
		"virtualAccountNo":   virtualAccountNo,
		"virtualAccountName": name,
		"trxId":              trxID,
	}
	if amount != "" {
		b["totalAmount"] = snapAmount(amount, currency)
	}
	return b
}

func buildVaInquiryBody(partnerServiceID, customerNo, virtualAccountNo, trxID string) map[string]any {
	return map[string]any{
		"partnerServiceId": partnerServiceID,
		"customerNo":       customerNo,
		"virtualAccountNo": virtualAccountNo,
		"trxId":            trxID,
	}
}

// snapExample trims a leading/trailing newline from a raw example block.
func snapExample(s string) string { return strings.Trim(s, "\n") }

// --- command constructors ---

// pp:data-source live
func newSnapBalanceCmd(flags *rootFlags) *cobra.Command {
	var accountNo, partnerRef, externalID string
	cmd := &cobra.Command{
		Use:   "balance",
		Short: "Inquire merchant SNAP balance",
		Example: snapExample(`
  durianpay-pp-cli snap balance
  durianpay-pp-cli snap balance --account-no mer_xxxx`),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := snapDataSourceGuard(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				printSnapDryRun(cmd, flags, "POST", "/v1.0/balance-inquiry")
				return nil
			}
			if accountNo == "" {
				if c, err := snapClientFromFlags(flags); err == nil {
					accountNo = c.Config().MerchantID
				}
			}
			if accountNo == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--account-no is required (or set DURIANPAY_MERCHANT_ID)"))
			}
			return runSnapPost(cmd, flags, snapPOST, "/v1.0/balance-inquiry", buildBalanceBody(accountNo, partnerRef), externalID)
		},
	}
	cmd.Flags().StringVar(&accountNo, "account-no", "", "Merchant balance account number (default: DURIANPAY_MERCHANT_ID)")
	cmd.Flags().StringVar(&partnerRef, "partner-reference-no", "", "Unique partner reference (auto-generated if empty)")
	cmd.Flags().StringVar(&externalID, "external-id", "", "X-EXTERNAL-ID (auto-generated if empty)")
	return cmd
}

// pp:data-source live
func newSnapInquiryBankCmd(flags *rootFlags) *cobra.Command {
	var bank, account, externalID string
	cmd := &cobra.Command{
		Use:   "inquiry-bank",
		Short: "Inquire a beneficiary bank account name",
		Example: snapExample(`
  durianpay-pp-cli snap inquiry-bank --bank 014 --account 1234567890`),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--bank=014;--account=1234567890",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := snapDataSourceGuard(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				printSnapDryRun(cmd, flags, "POST", "/v1.0/account-inquiry-external")
				return nil
			}
			if bank == "" || account == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--bank and --account are required"))
			}
			return runSnapPost(cmd, flags, snapPOST, "/v1.0/account-inquiry-external", buildInquiryBankBody(bank, account), externalID)
		},
	}
	cmd.Flags().StringVar(&bank, "bank", "", "SNAP numeric bank code (e.g. 014 = BCA, 002 = BRI, 008 = Mandiri)")
	cmd.Flags().StringVar(&account, "account", "", "Beneficiary account number")
	cmd.Flags().StringVar(&externalID, "external-id", "", "X-EXTERNAL-ID (auto-generated if empty)")
	return cmd
}

// pp:data-source live
func newSnapInquiryEwalletCmd(flags *rootFlags) *cobra.Command {
	var platform, customerNumber, partnerRef, externalID string
	cmd := &cobra.Command{
		Use:   "inquiry-ewallet",
		Short: "Inquire an e-wallet account holder name",
		Example: snapExample(`
  durianpay-pp-cli snap inquiry-ewallet --platform gopay --customer-number 081234567890`),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--platform=gopay;--customer-number=081234567890",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := snapDataSourceGuard(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				printSnapDryRun(cmd, flags, "POST", "/v1.0/emoney/account-inquiry")
				return nil
			}
			if platform == "" || customerNumber == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--platform and --customer-number are required"))
			}
			return runSnapPost(cmd, flags, snapPOST, "/v1.0/emoney/account-inquiry", buildInquiryEwalletBody(platform, customerNumber, partnerRef), externalID)
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "", "E-wallet platform code (gopay, dana, ovo, shopeepay, linkaja)")
	cmd.Flags().StringVar(&customerNumber, "customer-number", "", "Recipient phone number (628... or 08...)")
	cmd.Flags().StringVar(&partnerRef, "partner-reference-no", "", "Unique partner reference (auto-generated if empty)")
	cmd.Flags().StringVar(&externalID, "external-id", "", "X-EXTERNAL-ID (auto-generated if empty)")
	return cmd
}

// pp:data-source live
func newSnapTransferCmd(flags *rootFlags) *cobra.Command {
	var amount, currency, bank, account, name, partnerRef, source, email, remark, externalID string
	cmd := &cobra.Command{
		Use:   "transfer",
		Short: "Disburse an interbank transfer over SNAP",
		Example: snapExample(`
  durianpay-pp-cli snap transfer --amount 10000.00 --bank 014 --account 1234567890 --name "Jane Doe"`),
		Annotations: map[string]string{
			"pp:happy-args": "--amount=10000.00;--bank=014;--account=1234567890;--name=Jane Doe",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := snapDataSourceGuard(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				printSnapDryRun(cmd, flags, "POST", "/v1.0/transfer-interbank")
				return nil
			}
			if amount == "" || bank == "" || account == "" || name == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--amount, --bank, --account and --name are required"))
			}
			if source == "" {
				if c, err := snapClientFromFlags(flags); err == nil {
					source = c.Config().MerchantID
				}
			}
			body := buildTransferBody(amount, currency, bank, account, name, partnerRef, source, email, remark)
			return runSnapPost(cmd, flags, snapPOST, "/v1.0/transfer-interbank", body, externalID)
		},
	}
	cmd.Flags().StringVar(&amount, "amount", "", "Transfer amount, 2 decimals (e.g. 10000.00)")
	cmd.Flags().StringVar(&currency, "currency", "IDR", "Currency code")
	cmd.Flags().StringVar(&bank, "bank", "", "SNAP numeric bank code (e.g. 014 = BCA, 002 = BRI, 008 = Mandiri)")
	cmd.Flags().StringVar(&account, "account", "", "Beneficiary account number")
	cmd.Flags().StringVar(&name, "name", "", "Beneficiary account name")
	cmd.Flags().StringVar(&partnerRef, "partner-reference-no", "", "Unique partner reference (auto-generated if empty)")
	cmd.Flags().StringVar(&source, "source-account-no", "", "Source account number (merchant id)")
	cmd.Flags().StringVar(&email, "email", "", "Beneficiary email")
	cmd.Flags().StringVar(&remark, "remark", "", "Customer reference / remark")
	cmd.Flags().StringVar(&externalID, "external-id", "", "X-EXTERNAL-ID (auto-generated if empty)")
	return cmd
}

// pp:data-source live
func newSnapTransferStatusCmd(flags *rootFlags) *cobra.Command {
	var originalPartnerRef, originalRef, serviceCode, externalID string
	cmd := &cobra.Command{
		Use:   "transfer-status",
		Short: "Check an interbank transfer status",
		Example: snapExample(`
  durianpay-pp-cli snap transfer-status --original-reference-no dis_item_xxxx`),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--original-reference-no=dis_item_xxxx",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := snapDataSourceGuard(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				printSnapDryRun(cmd, flags, "POST", "/v1.0/transfer/status")
				return nil
			}
			if originalPartnerRef == "" && originalRef == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("one of --original-partner-reference-no or --original-reference-no is required"))
			}
			body := buildTransferStatusBody(originalPartnerRef, originalRef, serviceCode)
			return runSnapPost(cmd, flags, snapPOST, "/v1.0/transfer/status", body, externalID)
		},
	}
	cmd.Flags().StringVar(&originalPartnerRef, "original-partner-reference-no", "", "Original partner reference number")
	cmd.Flags().StringVar(&originalRef, "original-reference-no", "", "Original Durianpay reference (dis_item_xxx)")
	cmd.Flags().StringVar(&serviceCode, "service-code", "18", "Service code (18 = bank transfer)")
	cmd.Flags().StringVar(&externalID, "external-id", "", "X-EXTERNAL-ID (auto-generated if empty)")
	return cmd
}

// pp:data-source live
func newSnapEwalletTransferCmd(flags *rootFlags) *cobra.Command {
	var amount, currency, customerNumber, customerName, platform, partnerRef, externalID string
	cmd := &cobra.Command{
		Use:   "ewallet-transfer",
		Short: "Top up an e-wallet account over SNAP",
		Example: snapExample(`
  durianpay-pp-cli snap ewallet-transfer --amount 10000.00 --platform gopay --customer-number 081234567890`),
		Annotations: map[string]string{
			"pp:happy-args": "--amount=10000.00;--platform=gopay;--customer-number=081234567890",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := snapDataSourceGuard(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				printSnapDryRun(cmd, flags, "POST", "/v1.0/emoney/topup")
				return nil
			}
			if amount == "" || platform == "" || customerNumber == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--amount, --platform and --customer-number are required"))
			}
			body := buildEwalletTransferBody(amount, currency, customerNumber, customerName, platform, partnerRef)
			return runSnapPost(cmd, flags, snapPOST, "/v1.0/emoney/topup", body, externalID)
		},
	}
	cmd.Flags().StringVar(&amount, "amount", "", "Top-up amount, 2 decimals (e.g. 10000.00)")
	cmd.Flags().StringVar(&currency, "currency", "IDR", "Currency code")
	cmd.Flags().StringVar(&customerNumber, "customer-number", "", "Recipient phone number (628... or 08...)")
	cmd.Flags().StringVar(&customerName, "name", "", "Recipient name")
	cmd.Flags().StringVar(&platform, "platform", "", "E-wallet platform code (gopay, dana, ovo, shopeepay, linkaja)")
	cmd.Flags().StringVar(&partnerRef, "partner-reference-no", "", "Unique partner reference (auto-generated if empty)")
	cmd.Flags().StringVar(&externalID, "external-id", "", "X-EXTERNAL-ID (auto-generated if empty)")
	return cmd
}

// pp:data-source live
func newSnapEwalletTransferStatusCmd(flags *rootFlags) *cobra.Command {
	var originalPartnerRef, originalRef, serviceCode, externalID string
	cmd := &cobra.Command{
		Use:   "ewallet-transfer-status",
		Short: "Check an e-wallet top-up status",
		Example: snapExample(`
  durianpay-pp-cli snap ewallet-transfer-status --original-reference-no dis_item_xxxx`),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--original-reference-no=dis_item_xxxx",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := snapDataSourceGuard(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				printSnapDryRun(cmd, flags, "POST", "/v1.0/emoney/topup-status")
				return nil
			}
			if originalPartnerRef == "" && originalRef == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("one of --original-partner-reference-no or --original-reference-no is required"))
			}
			body := buildEwalletTransferStatusBody(originalPartnerRef, originalRef, serviceCode)
			return runSnapPost(cmd, flags, snapPOST, "/v1.0/emoney/topup-status", body, externalID)
		},
	}
	cmd.Flags().StringVar(&originalPartnerRef, "original-partner-reference-no", "", "Original partner reference number")
	cmd.Flags().StringVar(&originalRef, "original-reference-no", "", "Original Durianpay reference (dis_item_xxx)")
	cmd.Flags().StringVar(&serviceCode, "service-code", "38", "Service code (38 = e-wallet transfer)")
	cmd.Flags().StringVar(&externalID, "external-id", "", "X-EXTERNAL-ID (auto-generated if empty)")
	return cmd
}

// pp:data-source live
func newSnapEwalletPayCmd(flags *rootFlags) *cobra.Command {
	var amount, currency, partnerRef, phone, channel, externalID string
	cmd := &cobra.Command{
		Use:   "ewallet-pay",
		Short: "Create an e-wallet debit payment over SNAP",
		Example: snapExample(`
  durianpay-pp-cli snap ewallet-pay --amount 10000.00 --channel SHOPEEPAY --phone 085103885747`),
		Annotations: map[string]string{
			"pp:happy-args": "--amount=10000.00;--channel=SHOPEEPAY;--phone=085103885747",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := snapDataSourceGuard(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				printSnapDryRun(cmd, flags, "POST", "/v1.0/debit/payment-host-to-host")
				return nil
			}
			if amount == "" || channel == "" || phone == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--amount, --channel and --phone are required"))
			}
			body := buildEwalletPayBody(amount, currency, partnerRef, phone, channel)
			return runSnapPost(cmd, flags, snapPOST, "/v1.0/debit/payment-host-to-host", body, externalID)
		},
	}
	cmd.Flags().StringVar(&amount, "amount", "", "Payment amount, 2 decimals (e.g. 10000.00)")
	cmd.Flags().StringVar(&currency, "currency", "IDR", "Currency code")
	cmd.Flags().StringVar(&partnerRef, "partner-reference-no", "", "Unique partner reference (auto-generated if empty)")
	cmd.Flags().StringVar(&phone, "phone", "", "Phone number attached to the e-wallet")
	cmd.Flags().StringVar(&channel, "channel", "", "E-wallet channel code (GOPAY, SHOPEEPAY, DANA, OVO, LINKAJA)")
	cmd.Flags().StringVar(&externalID, "external-id", "", "X-EXTERNAL-ID (auto-generated if empty)")
	return cmd
}

// pp:data-source live
func newSnapEwalletStatusCmd(flags *rootFlags) *cobra.Command {
	var originalRef, serviceCode, externalID string
	cmd := &cobra.Command{
		Use:   "ewallet-status",
		Short: "Inquire an e-wallet payment status",
		Example: snapExample(`
  durianpay-pp-cli snap ewallet-status --original-reference-no pay_WCRPdO1Vhb4690`),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--original-reference-no=pay_WCRPdO1Vhb4690",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := snapDataSourceGuard(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				printSnapDryRun(cmd, flags, "POST", "/v1.0/debit/status")
				return nil
			}
			if originalRef == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--original-reference-no is required"))
			}
			return runSnapPost(cmd, flags, snapPOST, "/v1.0/debit/status", buildEwalletStatusBody(originalRef, serviceCode), externalID)
		},
	}
	cmd.Flags().StringVar(&originalRef, "original-reference-no", "", "Durianpay payment reference ID")
	cmd.Flags().StringVar(&serviceCode, "service-code", "54", "Service code (54 = e-wallet)")
	cmd.Flags().StringVar(&externalID, "external-id", "", "X-EXTERNAL-ID (auto-generated if empty)")
	return cmd
}

// pp:data-source live
func newSnapEwalletCancelCmd(flags *rootFlags) *cobra.Command {
	var partnerRef, originalRef, reason, externalID string
	cmd := &cobra.Command{
		Use:   "ewallet-cancel",
		Short: "Cancel an e-wallet payment",
		Example: snapExample(`
  durianpay-pp-cli snap ewallet-cancel --partner-reference-no payment_ref_id --original-reference-no pay_WCRPdO1Vhb4690`),
		Annotations: map[string]string{
			"pp:happy-args": "--partner-reference-no=payment_ref_id;--original-reference-no=pay_WCRPdO1Vhb4690",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := snapDataSourceGuard(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				printSnapDryRun(cmd, flags, "POST", "/v1.0/debit/cancel")
				return nil
			}
			if partnerRef == "" || originalRef == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--partner-reference-no and --original-reference-no are required"))
			}
			return runSnapPost(cmd, flags, snapPOST, "/v1.0/debit/cancel", buildEwalletCancelBody(partnerRef, originalRef, reason), externalID)
		},
	}
	cmd.Flags().StringVar(&partnerRef, "partner-reference-no", "", "Merchant internal payment reference ID")
	cmd.Flags().StringVar(&originalRef, "original-reference-no", "", "Durianpay internal payment reference ID")
	cmd.Flags().StringVar(&reason, "reason", "", "Reason for cancellation")
	cmd.Flags().StringVar(&externalID, "external-id", "", "X-EXTERNAL-ID (auto-generated if empty)")
	return cmd
}

// pp:data-source live
func newSnapEwalletRefundCmd(flags *rootFlags) *cobra.Command {
	var originalPartnerRef, originalRef, refundNo, amount, currency, externalID string
	cmd := &cobra.Command{
		Use:   "ewallet-refund",
		Short: "Refund an e-wallet payment",
		Example: snapExample(`
  durianpay-pp-cli snap ewallet-refund --original-reference-no pay_WCRPdO1Vhb4690 --original-partner-reference-no pay_ref_id --amount 10000.00`),
		Annotations: map[string]string{
			"pp:happy-args": "--original-reference-no=pay_WCRPdO1Vhb4690;--original-partner-reference-no=pay_ref_id;--amount=10000.00",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := snapDataSourceGuard(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				printSnapDryRun(cmd, flags, "POST", "/v1.0/debit/refund")
				return nil
			}
			if originalRef == "" || originalPartnerRef == "" || amount == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--original-reference-no, --original-partner-reference-no and --amount are required"))
			}
			body := buildRefundBody(originalPartnerRef, originalRef, refundNo, amount, currency)
			return runSnapPost(cmd, flags, snapPOST, "/v1.0/debit/refund", body, externalID)
		},
	}
	cmd.Flags().StringVar(&originalPartnerRef, "original-partner-reference-no", "", "Merchant's unique transaction reference")
	cmd.Flags().StringVar(&originalRef, "original-reference-no", "", "Durianpay's unique transaction reference")
	cmd.Flags().StringVar(&refundNo, "partner-refund-no", "", "Merchant's unique refund reference (auto-generated if empty)")
	cmd.Flags().StringVar(&amount, "amount", "", "Refund amount, 2 decimals (e.g. 10000.00)")
	cmd.Flags().StringVar(&currency, "currency", "IDR", "Currency code")
	cmd.Flags().StringVar(&externalID, "external-id", "", "X-EXTERNAL-ID (auto-generated if empty)")
	return cmd
}

// pp:data-source live
func newSnapQrisGenerateCmd(flags *rootFlags) *cobra.Command {
	var amount, currency, partnerRef, externalID string
	cmd := &cobra.Command{
		Use:   "qris-generate",
		Short: "Generate a QRIS MPM payment code",
		Example: snapExample(`
  durianpay-pp-cli snap qris-generate --amount 10000.00`),
		Annotations: map[string]string{
			"pp:happy-args": "--amount=10000.00",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := snapDataSourceGuard(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				printSnapDryRun(cmd, flags, "POST", "/v1.0/qr/qr-mpm-generate")
				return nil
			}
			if amount == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--amount is required"))
			}
			return runSnapPost(cmd, flags, snapPOST, "/v1.0/qr/qr-mpm-generate", buildQrisGenerateBody(amount, currency, partnerRef), externalID)
		},
	}
	cmd.Flags().StringVar(&amount, "amount", "", "Transaction amount, 2 decimals (e.g. 10000.00)")
	cmd.Flags().StringVar(&currency, "currency", "IDR", "Currency code")
	cmd.Flags().StringVar(&partnerRef, "partner-reference-no", "", "Unique partner reference (auto-generated if empty)")
	cmd.Flags().StringVar(&externalID, "external-id", "", "X-EXTERNAL-ID (auto-generated if empty)")
	return cmd
}

// pp:data-source live
func newSnapQrisQueryCmd(flags *rootFlags) *cobra.Command {
	var originalRef, serviceCode, externalID string
	cmd := &cobra.Command{
		Use:   "qris-query",
		Short: "Query a QRIS payment status",
		Example: snapExample(`
  durianpay-pp-cli snap qris-query --original-reference-no pay_WCRPdO1Vhb4690`),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--original-reference-no=pay_WCRPdO1Vhb4690",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := snapDataSourceGuard(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				printSnapDryRun(cmd, flags, "POST", "/v1.0/qr/qr-mpm-query")
				return nil
			}
			if originalRef == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--original-reference-no is required"))
			}
			return runSnapPost(cmd, flags, snapPOST, "/v1.0/qr/qr-mpm-query", buildQrisQueryBody(originalRef, serviceCode), externalID)
		},
	}
	cmd.Flags().StringVar(&originalRef, "original-reference-no", "", "Durianpay payment reference ID")
	cmd.Flags().StringVar(&serviceCode, "service-code", "47", "Service code (47 = QRIS)")
	cmd.Flags().StringVar(&externalID, "external-id", "", "X-EXTERNAL-ID (auto-generated if empty)")
	return cmd
}

// pp:data-source live
func newSnapQrisCancelCmd(flags *rootFlags) *cobra.Command {
	var merchantID, originalRef, reason, externalID string
	cmd := &cobra.Command{
		Use:   "qris-cancel",
		Short: "Cancel a QRIS payment",
		Example: snapExample(`
  durianpay-pp-cli snap qris-cancel --merchant-id mer_xxxx --original-reference-no pay_WCRPdO1Vhb4690 --reason "customer cancelled"`),
		Annotations: map[string]string{
			"pp:happy-args": "--merchant-id=mer_xxxx;--original-reference-no=pay_WCRPdO1Vhb4690;--reason=customer cancelled",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := snapDataSourceGuard(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				printSnapDryRun(cmd, flags, "POST", "/v1.0/qr/qr-mpm-cancel")
				return nil
			}
			if merchantID == "" || originalRef == "" || reason == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--merchant-id, --original-reference-no and --reason are required"))
			}
			return runSnapPost(cmd, flags, snapPOST, "/v1.0/qr/qr-mpm-cancel", buildQrisCancelBody(merchantID, originalRef, reason), externalID)
		},
	}
	cmd.Flags().StringVar(&merchantID, "merchant-id", "", "Merchant unique reference ID")
	cmd.Flags().StringVar(&originalRef, "original-reference-no", "", "Durianpay internal payment reference ID")
	cmd.Flags().StringVar(&reason, "reason", "", "Reason for cancellation")
	cmd.Flags().StringVar(&externalID, "external-id", "", "X-EXTERNAL-ID (auto-generated if empty)")
	return cmd
}

// pp:data-source live
func newSnapQrisRefundCmd(flags *rootFlags) *cobra.Command {
	var originalPartnerRef, originalRef, refundNo, amount, currency, externalID string
	cmd := &cobra.Command{
		Use:   "qris-refund",
		Short: "Refund a QRIS payment",
		Example: snapExample(`
  durianpay-pp-cli snap qris-refund --original-reference-no pay_WCRPdO1Vhb4690 --original-partner-reference-no pay_ref_id --amount 10000.00`),
		Annotations: map[string]string{
			"pp:happy-args": "--original-reference-no=pay_WCRPdO1Vhb4690;--original-partner-reference-no=pay_ref_id;--amount=10000.00",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := snapDataSourceGuard(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				printSnapDryRun(cmd, flags, "POST", "/v1.0/qr/qr-mpm-refund")
				return nil
			}
			if originalRef == "" || originalPartnerRef == "" || amount == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--original-reference-no, --original-partner-reference-no and --amount are required"))
			}
			body := buildRefundBody(originalPartnerRef, originalRef, refundNo, amount, currency)
			return runSnapPost(cmd, flags, snapPOST, "/v1.0/qr/qr-mpm-refund", body, externalID)
		},
	}
	cmd.Flags().StringVar(&originalPartnerRef, "original-partner-reference-no", "", "Merchant's unique transaction reference")
	cmd.Flags().StringVar(&originalRef, "original-reference-no", "", "Durianpay's unique transaction reference")
	cmd.Flags().StringVar(&refundNo, "partner-refund-no", "", "Merchant's unique refund reference (auto-generated if empty)")
	cmd.Flags().StringVar(&amount, "amount", "", "Refund amount, 2 decimals (e.g. 10000.00)")
	cmd.Flags().StringVar(&currency, "currency", "IDR", "Currency code")
	cmd.Flags().StringVar(&externalID, "external-id", "", "X-EXTERNAL-ID (auto-generated if empty)")
	return cmd
}

// pp:data-source live
func newSnapVaCreateCmd(flags *rootFlags) *cobra.Command {
	var name, customerNo, trxType, trxID, bank, amount, currency, externalID string
	cmd := &cobra.Command{
		Use:   "va-create",
		Short: "Create a SNAP virtual account",
		Example: snapExample(`
  durianpay-pp-cli snap va-create --name "Jane Doe" --bank BCA --trx-id merchant_trx_0001`),
		Annotations: map[string]string{
			"pp:happy-args": "--name=Jane Doe;--bank=BCA;--trx-id=merchant_trx_0001",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := snapDataSourceGuard(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				printSnapDryRun(cmd, flags, "POST", "/v1.0/transfer-va/create-va")
				return nil
			}
			if name == "" || bank == "" || trxID == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--name, --bank and --trx-id are required"))
			}
			body := buildVaCreateBody(name, customerNo, trxType, trxID, bank, amount, currency)
			return runSnapPost(cmd, flags, snapPOST, "/v1.0/transfer-va/create-va", body, externalID)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Virtual account display name")
	cmd.Flags().StringVar(&customerNo, "customer-no", "", "Account suffix (random if empty)")
	cmd.Flags().StringVar(&trxType, "trx-type", "O", "Transaction type: O (open) or C (closed amount)")
	cmd.Flags().StringVar(&trxID, "trx-id", "", "Merchant transaction reference (unique)")
	cmd.Flags().StringVar(&bank, "bank", "", "Bank code (BRI, BCA, MANDIRI, PERMATA, CIMB, BNI, NOBU)")
	cmd.Flags().StringVar(&amount, "amount", "", "Total amount for closed-amount VAs, 2 decimals")
	cmd.Flags().StringVar(&currency, "currency", "IDR", "Currency code")
	cmd.Flags().StringVar(&externalID, "external-id", "", "X-EXTERNAL-ID (auto-generated if empty)")
	return cmd
}

// pp:data-source live
func newSnapVaUpdateCmd(flags *rootFlags) *cobra.Command {
	var partnerServiceID, customerNo, virtualAccountNo, name, trxID, amount, currency, externalID string
	cmd := &cobra.Command{
		Use:   "va-update",
		Short: "Update a SNAP virtual account",
		Example: snapExample(`
  durianpay-pp-cli snap va-update --partner-service-id 88008800 --customer-no 9876543210 --virtual-account-no 880088009876543210 --name "Jane Doe" --trx-id merchant_trx_0001`),
		Annotations: map[string]string{
			"pp:happy-args": "--partner-service-id=88008800;--customer-no=9876543210;--virtual-account-no=880088009876543210;--name=Jane Doe;--trx-id=merchant_trx_0001",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := snapDataSourceGuard(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				printSnapDryRun(cmd, flags, "PUT", "/v1.0/transfer-va/update-va")
				return nil
			}
			if partnerServiceID == "" || customerNo == "" || virtualAccountNo == "" || name == "" || trxID == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--partner-service-id, --customer-no, --virtual-account-no, --name and --trx-id are required"))
			}
			body := buildVaUpdateBody(partnerServiceID, customerNo, virtualAccountNo, name, trxID, amount, currency)
			return runSnapPost(cmd, flags, http.MethodPut, "/v1.0/transfer-va/update-va", body, externalID)
		},
	}
	cmd.Flags().StringVar(&partnerServiceID, "partner-service-id", "", "Virtual account prefix")
	cmd.Flags().StringVar(&customerNo, "customer-no", "", "Virtual account suffix")
	cmd.Flags().StringVar(&virtualAccountNo, "virtual-account-no", "", "Full virtual account number")
	cmd.Flags().StringVar(&name, "name", "", "Virtual account display name")
	cmd.Flags().StringVar(&trxID, "trx-id", "", "Merchant transaction reference")
	cmd.Flags().StringVar(&amount, "amount", "", "Total amount (closed-amount VAs), 2 decimals")
	cmd.Flags().StringVar(&currency, "currency", "IDR", "Currency code")
	cmd.Flags().StringVar(&externalID, "external-id", "", "X-EXTERNAL-ID (auto-generated if empty)")
	return cmd
}

// pp:data-source live
func newSnapVaInquiryCmd(flags *rootFlags) *cobra.Command {
	var partnerServiceID, customerNo, virtualAccountNo, trxID, externalID string
	cmd := &cobra.Command{
		Use:   "va-inquiry",
		Short: "Inquire a SNAP virtual account",
		Example: snapExample(`
  durianpay-pp-cli snap va-inquiry --partner-service-id 88008800 --customer-no 9876543210 --virtual-account-no 880088009876543210 --trx-id merchant_trx_0001`),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--partner-service-id=88008800;--customer-no=9876543210;--virtual-account-no=880088009876543210;--trx-id=merchant_trx_0001",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := snapDataSourceGuard(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				printSnapDryRun(cmd, flags, "POST", "/v1.0/transfer-va/inquiry-va")
				return nil
			}
			if partnerServiceID == "" || customerNo == "" || virtualAccountNo == "" || trxID == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--partner-service-id, --customer-no, --virtual-account-no and --trx-id are required"))
			}
			body := buildVaInquiryBody(partnerServiceID, customerNo, virtualAccountNo, trxID)
			return runSnapPost(cmd, flags, snapPOST, "/v1.0/transfer-va/inquiry-va", body, externalID)
		},
	}
	cmd.Flags().StringVar(&partnerServiceID, "partner-service-id", "", "Virtual account prefix")
	cmd.Flags().StringVar(&customerNo, "customer-no", "", "Virtual account suffix")
	cmd.Flags().StringVar(&virtualAccountNo, "virtual-account-no", "", "Full virtual account number")
	cmd.Flags().StringVar(&trxID, "trx-id", "", "Merchant transaction reference")
	cmd.Flags().StringVar(&externalID, "external-id", "", "X-EXTERNAL-ID (auto-generated if empty)")
	return cmd
}
