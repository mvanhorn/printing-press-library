package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/ucp/internal/ucp"
)

const etsyListingsURL = "https://openapi.etsy.com/v3/application/listings/active"

// EtsySearch queries the Etsy Open API v3 active-listings endpoint.
// Requires ETSY_API_KEY env var. Returns a clear error if unset.
//
// Etsy's listing search is keyword-based (no semantic match). Limit caps results.
func EtsySearch(ctx context.Context, query string, limit int) ([]ucp.SearchHit, error) {
	apiKey := os.Getenv("ETSY_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("Etsy adapter requires ETSY_API_KEY env var — get one at https://www.etsy.com/developers/your-apps")
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	u, _ := url.Parse(etsyListingsURL)
	q := u.Query()
	q.Set("keywords", query)
	q.Set("limit", strconv.Itoa(limit))
	q.Set("includes", "Images")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ucp-pp-cli/1.3")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u.String(), err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("Etsy returned HTTP %d — ETSY_API_KEY may be invalid or revoked. Check at https://www.etsy.com/developers/your-apps", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Etsy returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var parsed struct {
		Results []struct {
			ListingID int64  `json:"listing_id"`
			Title     string `json:"title"`
			URL       string `json:"url"`
			Price     struct {
				Amount       int64  `json:"amount"`
				Divisor      int64  `json:"divisor"`
				CurrencyCode string `json:"currency_code"`
			} `json:"price"`
			SKU    []string `json:"sku"`
			Images []struct {
				URLFullxFull string `json:"url_fullxfull"`
			} `json:"images"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse Etsy response: %w", err)
	}
	hits := make([]ucp.SearchHit, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		hit := ucp.SearchHit{
			Merchant: "etsy.com",
			Title:    r.Title,
			URL:      r.URL,
			Currency: r.Price.CurrencyCode,
		}
		if r.Price.Divisor > 0 {
			// Etsy prices are amount/divisor — multiply to cents.
			hit.Price = int((r.Price.Amount * 100) / r.Price.Divisor)
		} else {
			hit.Price = int(r.Price.Amount)
		}
		if hit.Currency == "" {
			hit.Currency = "USD"
		}
		if len(r.SKU) > 0 {
			hit.SKU = r.SKU[0]
		}
		hits = append(hits, hit)
	}
	return hits, nil
}

// truncate returns s truncated to at most n bytes with "…" suffix when trimmed.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
