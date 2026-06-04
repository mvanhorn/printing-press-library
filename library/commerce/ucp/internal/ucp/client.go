package ucp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
)

// MerchantClient is an HTTP client keyed to a specific UCP merchant.
type MerchantClient struct {
	Manifest *Manifest
	HTTP     *http.Client
	domain   string
}

// NewMerchantClient fetches the manifest for domain and returns a client.
func NewMerchantClient(ctx context.Context, domain string) (*MerchantClient, error) {
	m, err := FetchManifest(ctx, domain)
	if err != nil {
		return nil, err
	}
	return &MerchantClient{
		Manifest: m,
		HTTP:     &http.Client{Timeout: 20 * time.Second},
		domain:   domain,
	}, nil
}

// Search queries the merchant's catalog search endpoint.
func (c *MerchantClient) Search(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	transport, endpoint := c.Manifest.EndpointFor("dev.ucp.shopping.catalog.search", "rest")
	if transport == "" {
		transport, endpoint = c.Manifest.EndpointFor("dev.ucp.shopping", "rest")
	}
	if transport != "" && transport != "rest" {
		return nil, fmt.Errorf("merchant only advertises %s transport; v0.1 of this CLI requires a merchant with REST transport. See Known Gaps in README.", transport)
	}
	if endpoint == "" {
		return nil, fmt.Errorf("merchant does not advertise a REST catalog search endpoint")
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint: %w", err)
	}
	q := u.Query()
	q.Set("q", query)
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("UCP-Agent", `profile="ucp-pp-cli/0.1"`)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u.String(), err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s: HTTP %d: %s", u.String(), resp.StatusCode, truncate(string(body), 200))
	}

	return normalizeSearchResponse(body, c.domain)
}

// normalizeSearchResponse maps common result field names into []SearchHit.
func normalizeSearchResponse(body []byte, merchant string) ([]SearchHit, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return nil, fmt.Errorf("parse search response: %w", err)
	}

	var rawItems []json.RawMessage
	for _, key := range []string{"results", "products", "items", "data"} {
		if raw, ok := top[key]; ok {
			if err := json.Unmarshal(raw, &rawItems); err == nil {
				break
			}
		}
	}
	if rawItems == nil {
		return nil, fmt.Errorf("unknown search response shape — expected one of: results, products, items, data. Got keys: %v. Please file a bug.", jsonKeys(top))
	}

	hits := make([]SearchHit, 0, len(rawItems))
	for _, raw := range rawItems {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		hit := SearchHit{Merchant: merchant}
		hit.Title = strField(obj, "title", "name")
		hit.SKU = strField(obj, "sku", "id")
		hit.GTIN = strField(obj, "gtin")
		hit.URL = strField(obj, "url")
		hit.Price = intField(obj, "price", "amount")
		hit.Currency = strField(obj, "currency")
		// Populate VariantID from variant_id or fall back to id (mirrors shopify_products.go).
		if vid, ok := obj["variant_id"]; ok {
			var f float64
			if json.Unmarshal(vid, &f) == nil {
				hit.VariantID = int64(f)
			}
		}
		if hit.VariantID == 0 {
			if id, ok := obj["id"]; ok {
				var f float64
				if json.Unmarshal(id, &f) == nil {
					hit.VariantID = int64(f)
				}
			}
		}
		hits = append(hits, hit)
	}
	return hits, nil
}

// CreateCheckoutSession POSTs a cart payload to the merchant's checkout endpoint
// and returns the raw JSON response body.
func (c *MerchantClient) CreateCheckoutSession(ctx context.Context, cart *Cart) ([]byte, error) {
	transport, endpoint := c.Manifest.EndpointFor("dev.ucp.shopping.checkout", "rest")
	if transport != "" && transport != "rest" {
		return nil, fmt.Errorf("merchant only advertises %s transport; v0.1 of this CLI requires a merchant with REST transport. See Known Gaps in README.", transport)
	}
	if endpoint == "" {
		return nil, fmt.Errorf("merchant does not advertise a REST checkout endpoint")
	}

	type checkoutPayload struct {
		LineItems []LineItem `json:"line_items"`
		Currency  string     `json:"currency,omitempty"`
		Buyer     *Buyer     `json:"buyer,omitempty"`
	}
	p := checkoutPayload{
		LineItems: cart.LineItems,
		Currency:  cart.Currency,
		Buyer:     cart.Buyer,
	}
	body, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal checkout payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("UCP-Agent", `profile="ucp-pp-cli/0.1"`)
	// Derive idempotency key from cart ID so repeated prep calls against the
	// same cart send the same key — which is what Idempotency-Key is for per RFC.
	if cart != nil && cart.ID != "" {
		req.Header.Set("Idempotency-Key", cart.ID)
	} else {
		req.Header.Set("Idempotency-Key", uuid.New().String())
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("POST %s: HTTP %d: %s", endpoint, resp.StatusCode, truncate(string(respBody), 200))
	}
	return respBody, nil
}

// helpers

func strField(obj map[string]json.RawMessage, keys ...string) string {
	for _, k := range keys {
		if raw, ok := obj[k]; ok {
			var s string
			if json.Unmarshal(raw, &s) == nil {
				return s
			}
		}
	}
	return ""
}

func intField(obj map[string]json.RawMessage, keys ...string) int {
	for _, k := range keys {
		if raw, ok := obj[k]; ok {
			var n float64
			if json.Unmarshal(raw, &n) == nil {
				return int(n)
			}
		}
	}
	return 0
}

func jsonKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
