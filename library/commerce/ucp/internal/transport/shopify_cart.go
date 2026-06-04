// Package transport provides anonymous catalog adapters for UCP merchants.
package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ShopifyCartAddResult is the normalized response from POST /cart/add.js.
type ShopifyCartAddResult struct {
	CartToken   string          `json:"cart_token"`
	CheckoutURL string          `json:"checkout_url"`
	ItemCount   int             `json:"item_count"`
	TotalCents  int             `json:"total_cents"`
	Currency    string          `json:"currency"`
	RawResponse json.RawMessage `json:"raw_response,omitempty"`
}

// ShopifyCartAdd POSTs a single line item to https://<merchant>/cart/add.js
// (anonymous Shopify cart endpoint — no auth). Returns the merchant's cart token
// and a checkout URL the buyer can be redirected to.
//
// variantID is the Shopify variant ID (numeric string like "42563115909207").
// merchant is the bare domain like "bark.co" (no scheme).
func ShopifyCartAdd(ctx context.Context, merchant, variantID string, qty int) (*ShopifyCartAddResult, error) {
	if merchant == "" || variantID == "" {
		return nil, fmt.Errorf("merchant and variantID are required")
	}
	if qty <= 0 {
		qty = 1
	}
	host := normalizeHost(merchant)
	url := fmt.Sprintf("https://%s/cart/add.js", host)
	payload := map[string]any{
		"items": []map[string]any{
			{"id": variantID, "quantity": qty},
		},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ucp-pp-cli/1.2")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode >= 400 {
		msg := string(raw)
		if len(msg) > 200 {
			msg = msg[:197] + "..."
		}
		return nil, fmt.Errorf("shopify /cart/add.js at %s returned HTTP %d: %s", url, resp.StatusCode, msg)
	}

	// Parse Shopify's response. Shape varies slightly between stores.
	// Common shapes:
	//   - {items: [...], item_count: N, total_price: 999, currency: "USD"}
	//   - or the added item directly (older themes)
	var parsed struct {
		ItemCount  int    `json:"item_count"`
		TotalPrice int    `json:"total_price"` // cents
		Currency   string `json:"currency"`
		Token      string `json:"token"`
	}
	_ = json.Unmarshal(raw, &parsed)

	// Construct a deterministic checkout URL from variant ID — this is the universal Shopify pattern.
	checkoutURL := fmt.Sprintf("https://%s/cart/%s:%d", host, variantID, qty)

	return &ShopifyCartAddResult{
		CartToken:   parsed.Token,
		CheckoutURL: checkoutURL,
		ItemCount:   parsed.ItemCount,
		TotalCents:  parsed.TotalPrice,
		Currency:    parsed.Currency,
		RawResponse: json.RawMessage(raw),
	}, nil
}
