// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored narrative order commands (preserved across force regen via registerNovelCommand).
// Place-order mutation FinalizeCheckout is intentionally NOT exposed as a raw generated endpoint.
// Order cancel is REST PUT /api/v2/orders/{orderId}/cancel (not GraphQL).

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/costco-sameday/internal/client"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newOrderNarrativeCmd(flags))
	})
}

func newOrderNarrativeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "order",
		Short: "Checkout preview, tip update, and gated place-order (FinalizeCheckout)",
		Long: strings.TrimSpace(`
Narrative checkout/order commands for Costco Same-Day.

  order preview   Read-only checkout invoice / totals (no charge)
  order tip       Update tip via UpdateCheckout tipsUpdate (mutates checkout; not a charge)
  order place     Charge path: FinalizeCheckout — HARD GATED
  order cancel-options  Read cancel-reason options (CustomerCancelSelections)
  order cancel    Cancel an order via PUT /api/v2/orders/{orderId}/cancel (gated)

Place-order gate (all required for a live charge):
  --yes
  --confirm-charge
  Preview output is always printed first.
  --dry-run never sends FinalizeCheckout (even with both confirm flags).

Card tokenization (createVgsCardInstrument / VGS) is intentionally not exposed
as a CLI command — too sensitive. Payment instrument attach is available as
checkout updatebuyflowpaymentinstructionsv2 for advanced use.
`),
		Example: `  costco-sameday-pp-cli order preview --checkout-session-id <id>
  costco-sameday-pp-cli order tip --checkout-session-id <id> --tip-input-token <token> --yes
  costco-sameday-pp-cli order place --checkout-session-id <id> --dry-run
  costco-sameday-pp-cli order place --checkout-session-id <id> --yes --confirm-charge`,
	}
	cmd.AddCommand(newOrderPreviewCmd(flags))
	cmd.AddCommand(newOrderTipCmd(flags))
	cmd.AddCommand(newOrderPlaceCmd(flags))
	cmd.AddCommand(newOrderCancelOptionsCmd(flags))
	cmd.AddCommand(newOrderCancelStubCmd(flags))
	return cmd
}

func newOrderPreviewCmd(flags *rootFlags) *cobra.Command {
	var checkoutSessionID string
	var shopID string
	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Preview checkout totals / invoice (no charge)",
		Long:  "Fetches CheckoutDraftOrderInvoiceV2 (and optionally CheckoutCmd). Never calls FinalizeCheckout.",
		Example: `  costco-sameday-pp-cli order preview --checkout-session-id <id>
  costco-sameday-pp-cli order preview --checkout-session-id <id> --json`,
		Annotations: map[string]string{
			"pp:narrative":  "order.preview",
			"pp:charge":     "false",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if checkoutSessionID == "" {
				return fmt.Errorf("missing --checkout-session-id (from InitializeCheckout / checkout session)")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{
				"operationName":     "CheckoutDraftOrderInvoiceV2",
				"checkoutSessionId": checkoutSessionID,
				"variables":         mustJSON(map[string]any{"checkoutSessionId": checkoutSessionID}),
			}
			data, err := c.Get(cmd.Context(), "/graphql", params)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			out := map[string]any{
				"would_charge":        false,
				"dry_run":             flags.dryRun,
				"confirm_charge":      false,
				"preview_operation":   "CheckoutDraftOrderInvoiceV2",
				"checkout_session_id": checkoutSessionID,
				"invoice":             json.RawMessage(data),
			}
			if shopID != "" {
				cmdParams := map[string]string{
					"operationName": "CheckoutCmd",
					"shopId":        shopID,
					"variables":     mustJSON(map[string]any{"shopId": shopID}),
				}
				if cmdData, cmdErr := c.Get(cmd.Context(), "/graphql", cmdParams); cmdErr == nil {
					out["checkout_cmd"] = json.RawMessage(cmdData)
				}
			}
			return printNarrativeJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&checkoutSessionID, "checkout-session-id", "", "Checkout session id from InitializeCheckout")
	cmd.Flags().StringVar(&shopID, "shop-id", "", "Optional shop id for CheckoutCmd layout")
	return cmd
}

func newOrderTipCmd(flags *rootFlags) *cobra.Command {
	var checkoutSessionID string
	var tipInputToken string
	var checkoutTrackingUUID string
	var pageViewID string
	cmd := &cobra.Command{
		Use:   "tip",
		Short: "Update tip via UpdateCheckout tipsUpdate (not a charge)",
		Long: `Sends UpdateCheckout with checkoutUpdates.tipsUpdate.tippingFields.tipInputToken.
This mutates the checkout session tip (e.g. $2→$6 in the paid-test capture) but does NOT place the order.
Requires --yes for a live mutation. --dry-run never sends the mutation.`,
		Example: `  costco-sameday-pp-cli order tip --checkout-session-id <id> --tip-input-token <token> --dry-run
  costco-sameday-pp-cli order tip --checkout-session-id <id> --tip-input-token <token> --yes`,
		Annotations: map[string]string{
			"pp:narrative": "order.tip",
			"pp:charge":    "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if checkoutSessionID == "" || tipInputToken == "" {
				return fmt.Errorf("require --checkout-session-id and --tip-input-token")
			}
			variables := map[string]any{
				"checkoutSessionId": checkoutSessionID,
				"checkoutUpdates": map[string]any{
					"tipsUpdate": map[string]any{
						"tippingFields": map[string]any{
							"tipInputToken": tipInputToken,
						},
					},
				},
			}
			if checkoutTrackingUUID != "" || pageViewID != "" {
				variables["checkoutCreationTrackingParams"] = map[string]any{
					"checkoutTrackingUuid": checkoutTrackingUUID,
					"pageViewId":           pageViewID,
				}
			}
			body := map[string]any{
				"operationName": "UpdateCheckout",
				"variables":     variables,
				"extensions": map[string]any{
					"persistedQuery": map[string]any{"version": 1},
				},
			}
			if dryRunOK(flags) {
				return printNarrativeJSON(cmd, flags, map[string]any{
					"would_charge":   false,
					"dry_run":        true,
					"operation":      "UpdateCheckout",
					"action":         "tipsUpdate",
					"would":          "POST /graphql UpdateCheckout tipsUpdate (no FinalizeCheckout)",
					"variables_keys": []string{"checkoutSessionId", "checkoutUpdates.tipsUpdate.tippingFields.tipInputToken"},
				})
			}
			if !flags.yes {
				return fmt.Errorf("refusing UpdateCheckout tip mutation without --yes (use --dry-run to preview)")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{"operationName": "UpdateCheckout"}
			ctx := client.WithDeclaredGraphQLOperation(cmd.Context(), "UpdateCheckout")
			data, status, err := c.PostWithParams(ctx, "/graphql", params, body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			return printNarrativeJSON(cmd, flags, map[string]any{
				"would_charge": false,
				"dry_run":      false,
				"operation":    "UpdateCheckout",
				"action":       "tipsUpdate",
				"http_status":  status,
				"response":     json.RawMessage(data),
			})
		},
	}
	cmd.Flags().StringVar(&checkoutSessionID, "checkout-session-id", "", "Checkout session id")
	cmd.Flags().StringVar(&tipInputToken, "tip-input-token", "", "Tip option token from checkout tip UI / UpdateCheckout selections")
	cmd.Flags().StringVar(&checkoutTrackingUUID, "checkout-tracking-uuid", "", "Optional checkoutTrackingUuid")
	cmd.Flags().StringVar(&pageViewID, "page-view-id", "", "Optional pageViewId")
	return cmd
}

func newOrderPlaceCmd(flags *rootFlags) *cobra.Command {
	var checkoutSessionID string
	var checkoutTrackingUUID string
	var pageViewID string
	var confirmCharge bool
	cmd := &cobra.Command{
		Use:   "place",
		Short: "Place order / charge via FinalizeCheckout (HARD GATED)",
		Long: `Calls FinalizeCheckout — the charge mutation discovered in the paid-test HAR.

HARD GATE (live charge requires ALL of):
  1. Preview block is always printed first
  2. --yes
  3. --confirm-charge
  4. --dry-run must NOT be set (dry-run never charges, even with confirm flags)

Without --yes and --confirm-charge the command refuses and does not POST.
Card PANs/CVV are never accepted here; payment must already be attached on the checkout session.`,
		Example: `  costco-sameday-pp-cli order place --checkout-session-id <id> --dry-run
  costco-sameday-pp-cli order place --checkout-session-id <id> --yes --confirm-charge`,
		Annotations: map[string]string{
			"pp:narrative":      "order.place",
			"pp:charge":         "true",
			"pp:confirm-charge": "required",
			"pp:mutation":       "FinalizeCheckout",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if checkoutSessionID == "" {
				return fmt.Errorf("missing --checkout-session-id")
			}

			preview := map[string]any{
				"would_charge":        true,
				"dry_run":             flags.dryRun,
				"yes":                 flags.yes,
				"confirm_charge":      confirmCharge,
				"mutation":            "FinalizeCheckout",
				"checkout_session_id": checkoutSessionID,
				"gate":                "requires --yes AND --confirm-charge; --dry-run never charges",
			}
			// Always emit preview first.
			if err := printNarrativeJSON(cmd, flags, map[string]any{"preview": preview}); err != nil {
				return err
			}

			if dryRunOK(flags) {
				return printNarrativeJSON(cmd, flags, map[string]any{
					"would_charge": true,
					"dry_run":      true,
					"charged":      false,
					"would":        "POST /graphql FinalizeCheckout (NOT sent)",
					"operation":    "FinalizeCheckout",
				})
			}

			if !flags.yes || !confirmCharge {
				missing := []string{}
				if !flags.yes {
					missing = append(missing, "--yes")
				}
				if !confirmCharge {
					missing = append(missing, "--confirm-charge")
				}
				return fmt.Errorf("refusing to charge: missing %s (preview printed above; pass both flags to place a real order, or --dry-run to simulate)", strings.Join(missing, " and "))
			}

			variables := map[string]any{
				"checkoutSessionId": checkoutSessionID,
				"paymentsClientInfo": map[string]any{
					"applePayEligible":  false,
					"googlePayEligible": false,
					"venmoAddable":      false,
				},
			}
			if checkoutTrackingUUID != "" || pageViewID != "" {
				variables["checkoutCreationTrackingParams"] = map[string]any{
					"checkoutTrackingUuid": checkoutTrackingUUID,
					"pageViewId":           pageViewID,
				}
			}
			// riskData (forter/recaptcha) intentionally omitted — caller must
			// supply via --risk-data-json if required by live anti-abuse; we do
			// not invent tokens.
			body := map[string]any{
				"operationName": "FinalizeCheckout",
				"variables":     variables,
				"extensions": map[string]any{
					"persistedQuery": map[string]any{"version": 1},
				},
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{"operationName": "FinalizeCheckout"}
			// The client re-checks BOTH flags at the transport layer.
			ctx := client.WithChargeConsent(cmd.Context(), flags.yes, confirmCharge)
			ctx = client.WithDeclaredGraphQLOperation(ctx, "FinalizeCheckout")
			data, status, err := c.PostWithParams(ctx, "/graphql", params, body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			outcome := client.ClassifyFinalizeCheckoutResponse(status, data)
			charged := outcome == client.ChargeConfirmed
			out := map[string]any{
				"would_charge":  true,
				"dry_run":       false,
				"charged":       charged,
				"charge_status": outcome,
				"http_status":   status,
				"operation":     "FinalizeCheckout",
				"response":      json.RawMessage(data),
			}
			if !charged {
				out["success"] = false
			}
			if err := printNarrativeJSON(cmd, flags, out); err != nil {
				return err
			}
			switch outcome {
			case client.ChargeNotCharged:
				return fmt.Errorf("FinalizeCheckout did not charge (HTTP %d with errors or a verify-mode no-op reply); see response above", status)
			case client.ChargeUnconfirmed:
				return fmt.Errorf("FinalizeCheckout charge is UNCONFIRMED (HTTP %d, no finalizeCheckout result): check order history before retrying to avoid a double charge", status)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&checkoutSessionID, "checkout-session-id", "", "Checkout session id from InitializeCheckout")
	cmd.Flags().StringVar(&checkoutTrackingUUID, "checkout-tracking-uuid", "", "Optional checkoutTrackingUuid")
	cmd.Flags().StringVar(&pageViewID, "page-view-id", "", "Optional pageViewId")
	cmd.Flags().BoolVar(&confirmCharge, "confirm-charge", false, "REQUIRED with --yes to send FinalizeCheckout (live charge)")
	return cmd
}

func newOrderCancelOptionsCmd(flags *rootFlags) *cobra.Command {
	var orderDeliveryID, orderUUID, serviceType string
	cmd := &cobra.Command{
		Use:     "cancel-options",
		Short:   "List cancel-reason options (CustomerCancelSelections) — read-only",
		Long:    "Does not cancel an order. Lists CustomerCancelSelections reasons; use order cancel to PUT /api/v2/orders/{orderId}/cancel.",
		Example: `  costco-sameday-pp-cli order cancel-options --order-delivery-id <id> --json`,
		Annotations: map[string]string{
			"pp:narrative":  "order.cancel-options",
			"pp:charge":     "false",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if orderDeliveryID == "" {
				return fmt.Errorf("missing --order-delivery-id")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			vars := map[string]any{"orderDeliveryId": orderDeliveryID}
			if orderUUID != "" {
				vars["orderUuid"] = orderUUID
			}
			if serviceType != "" {
				vars["serviceType"] = serviceType
			}
			params := map[string]string{
				"operationName": "CustomerCancelSelections",
				"variables":     mustJSON(vars),
			}
			data, err := c.Get(cmd.Context(), "/graphql", params)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			return printNarrativeJSON(cmd, flags, map[string]any{
				"would_charge": false,
				"operation":    "CustomerCancelSelections",
				"note":         "read-only options; live cancel is: order cancel --order-id <id> --yes",
				"response":     json.RawMessage(data),
			})
		},
	}
	cmd.Flags().StringVar(&orderDeliveryID, "order-delivery-id", "", "orderDeliveryId from FinalizeCheckout / post-checkout")
	cmd.Flags().StringVar(&orderUUID, "order-uuid", "", "Optional orderUuid")
	cmd.Flags().StringVar(&serviceType, "service-type", "", "Optional serviceType")
	return cmd
}

func newOrderCancelStubCmd(flags *rootFlags) *cobra.Command {
	// Real cancel: REST PUT (not GraphQL). Name kept for registerNovelCommand stability.
	var orderID string
	var costRelated string
	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel order via PUT /api/v2/orders/{orderId}/cancel (gated)",
		Long: `Cancels a Same-Day order with:
  PUT /api/v2/orders/{orderId}/cancel?source=web
  body: {"cancellation_reason":{"costRelated":""}}

This is NOT a GraphQL mutation and never charges.
Gate: live cancel requires --yes. --dry-run never sends the PUT.
Use order cancel-options to list CustomerCancelSelections reasons first.`,
		Example: `  costco-sameday-pp-cli order cancel --order-id <id> --dry-run
  costco-sameday-pp-cli order cancel --order-id <id> --yes`,
		Annotations: map[string]string{
			"pp:narrative": "order.cancel",
			"pp:charge":    "false",
			"pp:method":    "PUT",
			"pp:path":      "/api/v2/orders/{order_id}/cancel",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if orderID == "" {
				return fmt.Errorf("missing --order-id (legacyOrderId / orders path id, e.g. from FinalizeCheckout)")
			}
			path := "/api/v2/orders/" + orderID + "/cancel"
			body := map[string]any{
				"cancellation_reason": map[string]any{
					"costRelated": costRelated,
				},
			}
			preview := map[string]any{
				"would_charge": false,
				"method":       "PUT",
				"path":         path,
				"query":        map[string]string{"source": "web"},
				"body":         body,
				"gate":         "requires --yes for live cancel; --dry-run never sends",
				"yes":          flags.yes,
				"dry_run":      flags.dryRun,
			}
			_ = printNarrativeJSON(cmd, flags, map[string]any{"preview": preview})

			if flags.dryRun {
				return printNarrativeJSON(cmd, flags, map[string]any{
					"canceled":     false,
					"dry_run":      true,
					"would_charge": false,
					"would":        "PUT " + path + "?source=web (NOT sent)",
					"body":         body,
				})
			}
			if !flags.yes {
				return fmt.Errorf("refusing to cancel: missing --yes (preview printed above; pass --yes to send PUT cancel, or --dry-run to simulate)")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{"source": "web"}
			data, status, err := c.PutWithParams(cmd.Context(), path, params, body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			canceled := client.RESTResponseSucceeded(status, data)
			out := map[string]any{
				"canceled":     canceled,
				"dry_run":      false,
				"would_charge": false,
				"http_status":  status,
				"method":       "PUT",
				"path":         path,
				"response":     json.RawMessage(data),
			}
			if !canceled {
				out["success"] = false
			}
			if err := printNarrativeJSON(cmd, flags, out); err != nil {
				return err
			}
			if !canceled {
				return fmt.Errorf("order cancel not confirmed (HTTP %d with an error body or a verify-mode no-op reply); see response above", status)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&orderID, "order-id", "", "Order id path segment for PUT /api/v2/orders/{orderId}/cancel (required)")
	cmd.Flags().StringVar(&costRelated, "cost-related", "", "cancellation_reason.costRelated (default empty string, as observed)")
	return cmd
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func printNarrativeJSON(cmd *cobra.Command, flags *rootFlags, payload map[string]any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	if flags != nil && (flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout())) {
		return enc.Encode(payload)
	}
	// Human-friendly: still JSON for structured preview/gate clarity.
	return enc.Encode(payload)
}
