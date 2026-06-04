// Package transport provides anonymous catalog adapters for UCP merchants.
package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/ucp/internal/ucp"
)

// ShopifyProductsSearch fetches /products.json from a Shopify-hosted UCP merchant
// and returns normalized SearchHits filtered by the query terms.
//
// This is the v1.1 anonymous catalog path used as the primary transport for Shopify
// UCP merchants whose products.json is publicly readable. The full UCP MCP path
// (with hosted agent profile + JSON-RPC envelope) is v1.2.
//
// domain may be a bare host ("bark.co") or a scheme+host ("https://bark.co").
// query is whitespace-tokenized and matched case-insensitively against product
// title and product_type. Empty query returns the first `limit` products.
// limit caps the number of returned hits (and is sent as ?limit= to Shopify;
// Shopify itself caps at 250).
func ShopifyProductsSearch(ctx context.Context, domain string, query string, limit int) ([]ucp.SearchHit, error) {
	host := normalizeHost(domain)

	// Fetch 4x the limit client-side so filtering still returns enough results.
	fetchLimit := limit * 4
	if fetchLimit > 250 {
		fetchLimit = 250
	}
	if fetchLimit < 1 {
		fetchLimit = 250
	}

	rawURL := fmt.Sprintf("https://%s/products.json?limit=%d", host, fetchLimit)

	httpClient := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "ucp-pp-cli/1.1")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("shopify products.json at %s returned HTTP %d — merchant may not expose anonymous catalog; try `ucp check %s` to see supported transports", rawURL, resp.StatusCode, domain)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var catalog struct {
		Products []struct {
			ID          int64  `json:"id"`
			Title       string `json:"title"`
			Handle      string `json:"handle"`
			ProductType string `json:"product_type"`
			Variants    []struct {
				ID        int64  `json:"id"`
				SKU       string `json:"sku"`
				Price     string `json:"price"`
				Available bool   `json:"available"`
			} `json:"variants"`
		} `json:"products"`
	}
	if err := json.Unmarshal(body, &catalog); err != nil {
		return nil, fmt.Errorf("parse products.json: %w", err)
	}

	// Tokenize query for case-insensitive matching.
	var tokens []string
	for _, t := range strings.Fields(strings.ToLower(query)) {
		tokens = append(tokens, t)
	}

	hits := make([]ucp.SearchHit, 0, limit)
	for _, p := range catalog.Products {
		if len(hits) >= limit {
			break
		}
		if len(p.Variants) == 0 {
			continue
		}
		if len(tokens) > 0 && !matchesTokens(p.Title, p.ProductType, tokens) {
			continue
		}
		v := p.Variants[0]
		priceCents := parsePriceCents(v.Price)
		hits = append(hits, ucp.SearchHit{
			Merchant:  host,
			Title:     p.Title,
			Price:     priceCents,
			Currency:  "USD",
			SKU:       v.SKU,
			URL:       fmt.Sprintf("https://%s/products/%s", host, p.Handle),
			VariantID: v.ID,
		})
	}
	return hits, nil
}

// matchesTokens returns true if any token appears in title or productType (case-insensitive).
func matchesTokens(title, productType string, tokens []string) bool {
	haystack := strings.ToLower(title + " " + productType)
	for _, t := range tokens {
		if strings.Contains(haystack, t) {
			return true
		}
	}
	return false
}

// parsePriceCents converts a Shopify price string like "9.99" to integer cents (999).
func parsePriceCents(price string) int {
	if price == "" {
		return 0
	}
	// Parse as float, multiply by 100, round.
	var f float64
	_, err := fmt.Sscanf(price, "%f", &f)
	if err != nil {
		return 0
	}
	return int(math.Round(f * 100))
}

// normalizeHost strips scheme and trailing slash, returns "bark.co" from any input form.
func normalizeHost(domain string) string {
	d := strings.TrimSpace(domain)
	// Strip scheme.
	if idx := strings.Index(d, "://"); idx >= 0 {
		d = d[idx+3:]
	}
	// Strip trailing slash and path.
	if idx := strings.Index(d, "/"); idx >= 0 {
		d = d[:idx]
	}
	return d
}
