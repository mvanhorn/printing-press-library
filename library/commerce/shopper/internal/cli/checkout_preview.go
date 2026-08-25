// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command: pre-checkout summary aggregating siteapi reads.
// Checkout confirmation and order placement require a browser session — this
// command is strictly read-only, aggregating siteapi REST endpoints only.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type checkoutPreviewView struct {
	Store           string               `json:"store"`
	Cart            *checkoutCartSummary `json:"cart"`
	Delivery        *checkoutDelivery    `json:"delivery,omitempty"`
	ChargeCalendar  *chargeCalendarEntry `json:"charge_calendar,omitempty"`
	PaymentParams   *checkoutPayment     `json:"payment_params,omitempty"`
	CheckoutURL     string               `json:"checkout_url"`
	BrowserRequired string               `json:"browser_required"`
	Note            string               `json:"note,omitempty"`
}

type checkoutCartSummary struct {
	TotalAmount    string  `json:"total_amount"`
	TotalValue     float64 `json:"total_value"`
	ItemCount      int     `json:"item_count"`
	MinValue       float64 `json:"min_value"`
	MinValueMet    bool    `json:"min_value_met"`
	CashbackAmount string  `json:"cashback_amount,omitempty"`
}

type checkoutDelivery struct {
	DeliveryDate string `json:"delivery_date,omitempty"`
	Message      string `json:"message,omitempty"`
	IsUltraFast  bool   `json:"is_ultra_fast"`
}

type checkoutPayment struct {
	MinDaysCardCredit int  `json:"min_days_credit_card,omitempty"`
	MinDaysBoleto     int  `json:"min_days_bankslip,omitempty"`
	AllowPix          bool `json:"allow_pix"`
}

func newNovelCheckoutPreviewCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Pre-checkout summary: basket totals, delivery date, charge date, and accepted payment types",
		Long: `Aggregates /cart/summary + /delivery/summary + /features/stores into one
pre-checkout view so you can confirm basket readiness before opening the browser.

Read-only — does not place an order. Order confirmation requires a browser
session (Shopper checkout is a Django server-rendered form with CSRF/session).
Run 'shopper-pp-cli checkout open --store <store>' to open the checkout page.`,
		Example: "  shopper-pp-cli checkout preview --store programada\n  shopper-pp-cli checkout preview --store now --json",
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"would":"fetch /cart/summary + /delivery/summary + /features/stores and build checkout preview"}`)
				return nil
			}

			storeName := flags.store
			if storeName == "" {
				storeName = "programada"
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			view := checkoutPreviewView{
				Store:           storeName,
				CheckoutURL:     "https://" + resolveSubdomain(storeName) + ".shopper.com.br/shop/checkout",
				BrowserRequired: "Order confirmation requires a browser session. Run 'shopper-pp-cli checkout open --store " + storeName + "' to open the checkout page.",
			}

			cartData, cartErr := c.Get(cmd.Context(), "/cart/summary", nil)
			if cartErr == nil {
				view.Cart = extractCheckoutCart(cartData)
			}

			delivData, delivErr := c.Get(cmd.Context(), "/delivery/summary", nil)
			if delivErr == nil {
				view.Delivery = extractCheckoutDelivery(delivData)
			}

			// If cart and delivery both failed (most likely cause: no auth token),
			// add a note so the caller understands which fields are absent and why.
			if cartErr != nil && delivErr != nil {
				view.Note = "cart, delivery, and payment data unavailable — authenticate first: shopper-pp-cli auth set-token <token>"
			}

			if isSubscriptionStore(storeName) && delivErr == nil {
				calData, _ := c.Get(cmd.Context(), "/delivery/v2/calendar", nil)
				cal := buildChargeCalendar(delivData, calData, 0, false)
				view.ChargeCalendar = cal.NextDelivery
			} else if !isSubscriptionStore(storeName) {
				if view.Delivery == nil {
					view.Delivery = &checkoutDelivery{}
				}
				view.Delivery.IsUltraFast = true
				view.Note = "Ultra-fast delivery (now/now-bebidas): delivery slot is selected at checkout in the browser."
			}

			storesData, storesErr := c.Get(cmd.Context(), "/features/stores", nil)
			if storesErr == nil {
				view.PaymentParams = extractPaymentParams(storesData, storeName)
			}

			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	return cmd
}

func extractCheckoutCart(data json.RawMessage) *checkoutCartSummary {
	res := resultsEnvelope(data)
	if res == nil {
		return nil
	}
	var s checkoutCartSummary
	if raw, ok := res["totalAmount"]; ok {
		_ = json.Unmarshal(raw, &s.TotalAmount)
	}
	if raw, ok := res["totalValue"]; ok {
		_ = json.Unmarshal(raw, &s.TotalValue)
	}
	if raw, ok := res["totalCart"]; ok {
		_ = json.Unmarshal(raw, &s.ItemCount)
	}
	if raw, ok := res["minValue"]; ok {
		_ = json.Unmarshal(raw, &s.MinValue)
	}
	if s.TotalValue > 0 && s.MinValue > 0 {
		s.MinValueMet = s.TotalValue >= s.MinValue
	}
	if raw, ok := res["totalCashbackAmount"]; ok {
		_ = json.Unmarshal(raw, &s.CashbackAmount)
	}
	return &s
}

func extractCheckoutDelivery(data json.RawMessage) *checkoutDelivery {
	res := resultsEnvelope(data)
	if res == nil {
		return nil
	}
	var d checkoutDelivery
	if raw, ok := res["deliveryDate"]; ok {
		var ds string
		if json.Unmarshal(raw, &ds) == nil && ds != "" {
			if t, err := parseShopperDate(ds); err == nil {
				d.DeliveryDate = t.Format("2006-01-02")
			}
		}
	}
	if raw, ok := res["message"]; ok {
		var msg map[string]any
		if json.Unmarshal(raw, &msg) == nil {
			if t, ok := msg["text"].(string); ok {
				d.Message = stripHTMLTags(t)
			}
		}
	}
	return &d
}

func extractPaymentParams(data json.RawMessage, storeName string) *checkoutPayment {
	res := resultsEnvelope(data)
	if res == nil {
		return nil
	}
	var storesList []json.RawMessage
	if raw, ok := res["stores"]; ok {
		_ = json.Unmarshal(raw, &storesList)
	}
	for _, rawStore := range storesList {
		var s map[string]json.RawMessage
		if json.Unmarshal(rawStore, &s) != nil {
			continue
		}
		var name string
		if raw, ok := s["name"]; ok {
			_ = json.Unmarshal(raw, &name)
		}
		if !storeMatchesSubdomain(name, storeName) {
			continue
		}
		var p checkoutPayment
		if raw, ok := s["parameters"]; ok {
			var params map[string]any
			if json.Unmarshal(raw, &params) == nil {
				if v, ok := params["minimal_days_credit_card"]; ok {
					if n, ok := v.(float64); ok {
						p.MinDaysCardCredit = int(n)
					}
				}
				if v, ok := params["minimal_days_bankslip"]; ok {
					if n, ok := v.(float64); ok {
						p.MinDaysBoleto = int(n)
					}
				}
				if v, ok := params["allow_pix"]; ok && v != nil {
					if b, ok := v.(bool); ok {
						p.AllowPix = b
					}
				}
			}
		}
		return &p
	}
	return nil
}

// isSubscriptionStore returns true for stores with recurring basket cycles.
func isSubscriptionStore(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "programada", "mensal", "1", "fresh", "2", "pet", "5":
		return true
	}
	return false
}

// resolveSubdomain maps a store name to its web subdomain.
func resolveSubdomain(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "programada", "mensal", "1":
		return "programada"
	case "fresh", "2":
		return "fresh"
	case "unica", "pontual", "3":
		return "unica"
	case "pet", "5":
		return "pet"
	case "now", "6":
		return "now"
	case "now-bebidas", "nowbebidas", "8":
		return "now-bebidas"
	}
	return "programada"
}

// storeMatchesSubdomain checks if an API internal store name matches a user input.
func storeMatchesSubdomain(apiName, userInput string) bool {
	apiName = strings.ToLower(apiName)
	userInput = strings.ToLower(userInput)
	aliases := map[string][]string{
		"mensal":      {"programada", "1"},
		"fresh":       {"fresh", "2"},
		"pontual":     {"unica", "3"},
		"pet":         {"pet", "5"},
		"now":         {"now", "6"},
		"now-bebidas": {"now-bebidas", "nowbebidas", "8"},
	}
	for api, variants := range aliases {
		if apiName != api {
			continue
		}
		for _, v := range variants {
			if userInput == v {
				return true
			}
		}
	}
	return false
}
