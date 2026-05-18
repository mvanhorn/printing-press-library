// Package porkbun fetches Porkbun's public pricing endpoint (no auth required).
package porkbun

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/cliutil"
)

// limiter paces outbound Porkbun pricing requests. The pricing table is
// typically synced once per day per CLI process, so the conservative
// 1 req/sec floor is plenty; the adaptive logic ramps up on success and
// halves on 429.
var limiter = cliutil.NewAdaptiveLimiter(1.0)

// PricingEndpoint is Porkbun's public no-auth TLD pricing URL.
const PricingEndpoint = "https://api.porkbun.com/api/json/v3/pricing/get"

// PriceEntry holds the price triplet for one TLD.
type PriceEntry struct {
	TLD          string  `json:"tld"`
	Registration float64 `json:"registration"`
	Renewal      float64 `json:"renewal"`
	Transfer     float64 `json:"transfer"`
}

// pricingResponse mirrors the raw Porkbun JSON. The per-TLD map mixes string
// fields (registration/renewal/transfer) with an array (coupons), so we
// decode into json.RawMessage and parse the string fields explicitly.
type pricingResponse struct {
	Status  string                                `json:"status"`
	Pricing map[string]map[string]json.RawMessage `json:"pricing"`
}

// ErrRateLimited is returned when Porkbun responds with HTTP 429 (Too Many Requests).
type ErrRateLimited struct {
	RetryAfter time.Duration
}

func (e *ErrRateLimited) Error() string {
	return fmt.Sprintf("porkbun: rate limited (HTTP 429), retry after %s", e.RetryAfter)
}

// parseRetryAfter parses a Retry-After header value.
func parseRetryAfter(h string) time.Duration {
	if h == "" {
		return 60 * time.Second
	}
	if secs, err := strconv.Atoi(h); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 60 * time.Second
}

// FetchPricing pulls the full TLD pricing table. On HTTP 429 it returns a
// typed *ErrRateLimited so callers can back off appropriately. Outbound
// requests are paced via a shared AdaptiveLimiter.
func FetchPricing(ctx context.Context) ([]PriceEntry, error) {
	limiter.Wait()
	c := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, PricingEndpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("porkbun: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		limiter.OnRateLimit()
		return nil, &ErrRateLimited{RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
	}
	limiter.OnSuccess()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("porkbun: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var raw pricingResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode porkbun: %w", err)
	}
	if !strings.EqualFold(raw.Status, "success") {
		return nil, fmt.Errorf("porkbun: status=%s", raw.Status)
	}
	out := make([]PriceEntry, 0, len(raw.Pricing))
	for tld, prices := range raw.Pricing {
		e := PriceEntry{TLD: strings.ToLower(tld)}
		e.Registration = parseRawFloat(prices["registration"])
		e.Renewal = parseRawFloat(prices["renewal"])
		e.Transfer = parseRawFloat(prices["transfer"])
		out = append(out, e)
	}
	return out, nil
}

func parseRawFloat(raw json.RawMessage) float64 {
	if len(raw) == 0 {
		return 0
	}
	s := strings.Trim(string(raw), `"`)
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}
