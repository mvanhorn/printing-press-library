// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newCheckoutPreviewCmd is deliberately non-transactional. Its entire network
// surface is GET history/cards/profile plus POST validate/price; there is no
// place-order request in this command or any helper it calls.
func newCheckoutPreviewCmd(flags *rootFlags) *cobra.Command {
	var useLast bool
	cmd := &cobra.Command{
		Use:   "preview [customerID]",
		Short: "Validate and price the latest account order without placing it",
		Long: `Fetch the latest signed-in account order, remove prior payment and
order-identifying fields, validate and price it, and report saved-card and
store payment capabilities. This command cannot place an order.`,
		Example:     "  dominos-pp-cli checkout preview --market ca --last --json\n  dominos-pp-cli checkout preview <customerID> --market ca --last --json",
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !useLast {
				return usageErr(fmt.Errorf("checkout preview currently requires --last"))
			}
			customerID, err := resolveCustomerID(flags, args)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			historyPath := replacePathParam("/power/customer/{customerID}/order", "customerID", customerID)
			history, err := c.GetUncached(historyPath, map[string]string{"limit": "1", "lang": "en"})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action":                       "checkout_preview",
					"dry_run":                      true,
					"lookup_performed":             false,
					"previewed_stage":              "order_history",
					"dependent_requests_previewed": false,
					"place_order_available":        false,
					"placed":                       false,
				}, flags)
			}
			order, err := latestHistoricalOrder(history)
			if err != nil {
				return err
			}
			order = sanitizedReorder(order)
			storeID := stringValue(order["StoreID"])
			if storeID == "" {
				if details, ok := order["Details"].(map[string]any); ok {
					storeID = stringValue(details["StoreID"])
				}
			}

			body := map[string]any{"Order": order}
			validation, validateStatus, err := c.Post("/power/validate-order", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			validationOK, validationKnown := validationSucceeded(validation)
			if !validationKnown {
				return fmt.Errorf("Domino's validation response did not include a recognized success status")
			}
			if !validationOK {
				return fmt.Errorf("latest account order did not pass Domino's validation")
			}
			priceBody := body
			validatedOrder, hasValidatedOrder, err := orderFromValidation(validation)
			if err != nil {
				return err
			}
			if hasValidatedOrder {
				priceBody = map[string]any{"Order": sanitizedReorder(validatedOrder)}
			}
			priced, priceStatus, err := c.Post("/power/price-order", priceBody)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			pricingOK, pricingKnown := validationSucceeded(priced)
			if !pricingKnown {
				return fmt.Errorf("Domino's pricing response did not include a recognized success status")
			}
			if !pricingOK {
				return fmt.Errorf("latest account order was rejected by Domino's pricing")
			}
			pricedTotal, err := extractCustomerTotal(priced)
			if err != nil {
				return err
			}

			cardPath := replacePathParam("/power/customer/{customerID}/card", "customerID", customerID)
			cards, err := c.GetUncached(cardPath, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			cardCount, err := savedCardCount(cards)
			if err != nil {
				return err
			}

			capabilities := map[string]any{}
			if storeID != "" {
				profilePath := replacePathParam("/power/store/{storeID}/profile", "storeID", storeID)
				profile, profileErr := c.Get(profilePath, nil)
				if profileErr != nil {
					return classifyAPIError(profileErr, flags)
				}
				capabilities, err = paymentCapabilities(profile)
				if err != nil {
					return err
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"action":                "checkout_preview",
				"source":                "latest_account_order",
				"store_id":              storeID,
				"service_method":        stringValue(order["ServiceMethod"]),
				"validate_status":       validateStatus,
				"validation_ok":         validationOK,
				"price_status":          priceStatus,
				"pricing_ok":            pricingOK,
				"priced_total":          pricedTotal,
				"saved_card_count":      cardCount,
				"has_saved_cards":       cardCount > 0,
				"payment_capabilities":  capabilities,
				"payment_details":       "redacted",
				"place_order_available": false,
				"placed":                false,
			}, flags)
		},
	}
	cmd.Flags().BoolVar(&useLast, "last", false, "Use the most recent order from the signed-in account")
	return cmd
}

func latestHistoricalOrder(data json.RawMessage) (map[string]any, error) {
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("parsing order history: %w", err)
	}
	var candidates []any
	switch value := decoded.(type) {
	case []any:
		candidates = value
	case map[string]any:
		for _, key := range []string{"customerOrders", "CustomerOrders", "orders", "Orders"} {
			if list, ok := value[key].([]any); ok {
				candidates = list
				break
			}
		}
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no account orders were returned")
	}
	entry, ok := candidates[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("latest account order has an unexpected shape")
	}
	for _, key := range []string{"order", "Order"} {
		if order, ok := entry[key].(map[string]any); ok {
			return order, nil
		}
	}
	return entry, nil
}

var historicalOrderBlockedKeys = map[string]struct{}{
	"payments": {}, "payment": {}, "creditcards": {}, "creditcard": {}, "cards": {}, "card": {},
	"giftcards": {}, "giftcard": {}, "paymentmethods": {}, "paymentmethod": {}, "wallets": {}, "wallet": {},
	"cardid": {}, "maskednumber": {}, "lastfour": {}, "cvv": {}, "securitycode": {},
	"orderid": {}, "orderkey": {}, "status": {}, "orderstatus": {}, "trackingid": {},
	"estimatedwaitminutes": {}, "estimateddeliverytime": {}, "placedtime": {}, "completedtime": {},
}

func sanitizedReorder(order map[string]any) map[string]any {
	return sanitizeOrderMap(order)
}

func sanitizeOrderMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		if _, blocked := historicalOrderBlockedKeys[strings.ToLower(key)]; blocked {
			continue
		}
		switch typed := value.(type) {
		case map[string]any:
			out[key] = sanitizeOrderMap(typed)
		case []any:
			items := make([]any, 0, len(typed))
			for _, item := range typed {
				if nested, ok := item.(map[string]any); ok {
					items = append(items, sanitizeOrderMap(nested))
				} else {
					items = append(items, item)
				}
			}
			out[key] = items
		default:
			out[key] = value
		}
	}
	return out
}

func validationSucceeded(data json.RawMessage) (bool, bool) {
	var decoded map[string]any
	if json.Unmarshal(data, &decoded) != nil {
		return false, false
	}
	for _, key := range []string{"Status", "status", "Success", "success"} {
		if value, ok := decoded[key]; ok {
			switch typed := value.(type) {
			case bool:
				return typed, true
			case float64:
				switch typed {
				case -1:
					return false, true
				case 0, 1:
					return true, true
				default:
					return false, false
				}
			case string:
				switch {
				case strings.EqualFold(typed, "ok"), strings.EqualFold(typed, "success"), strings.EqualFold(typed, "true"):
					return true, true
				case typed == "-1", strings.EqualFold(typed, "false"), strings.EqualFold(typed, "failed"), strings.EqualFold(typed, "error"):
					return false, true
				case typed == "0", typed == "1":
					return true, true
				}
			}
		}
	}
	return false, false
}

func orderFromValidation(data json.RawMessage) (map[string]any, bool, error) {
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, false, fmt.Errorf("parsing validation response: %w", err)
	}
	rawOrder, exists := mapValueFold(decoded, "Order")
	if !exists {
		return nil, false, nil
	}
	order, ok := rawOrder.(map[string]any)
	if !ok {
		return nil, false, fmt.Errorf("validation response Order has an unexpected shape")
	}
	return order, true, nil
}

func extractCustomerTotal(data json.RawMessage) (any, error) {
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("parsing price response: %w", err)
	}
	paths := [][]string{
		{"Order", "Amounts", "Customer"},
		{"Amounts", "Customer"},
		{"CustomerTotal"},
		{"GrandTotal"},
		{"Total"},
	}
	for _, path := range paths {
		if value, ok := numericValueAtPath(decoded, path); ok {
			return value, nil
		}
	}
	return nil, fmt.Errorf("price response did not include a recognized customer total")
}

func numericValueAtPath(root map[string]any, path []string) (any, bool) {
	var current any = root
	for _, segment := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = mapValueFold(object, segment)
		if !ok {
			return nil, false
		}
	}
	switch current.(type) {
	case float64, int, json.Number:
		return current, true
	default:
		return nil, false
	}
}

func mapValueFold(values map[string]any, wanted string) (any, bool) {
	for key, value := range values {
		if strings.EqualFold(key, wanted) {
			return value, true
		}
	}
	return nil, false
}

func paymentCapabilities(data json.RawMessage) (map[string]any, error) {
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("parsing store payment capabilities: %w", err)
	}
	allowed := []string{"AcceptCash", "AcceptCreditCard", "AcceptDebitCard", "AcceptSavedCreditCard", "AllowCardSaving", "AcceptGiftCard", "CashLimit"}
	out := map[string]any{}
	for _, key := range allowed {
		if value, ok := findMapKey(decoded, key); ok {
			out[key] = value
		}
	}
	return out, nil
}

func findMapKey(value map[string]any, wanted string) (any, bool) {
	for key, child := range value {
		if strings.EqualFold(key, wanted) {
			return child, true
		}
		if nested, ok := child.(map[string]any); ok {
			if found, exists := findMapKey(nested, wanted); exists {
				return found, true
			}
		}
	}
	return nil, false
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return fmt.Sprintf("%.0f", typed)
	default:
		return ""
	}
}
