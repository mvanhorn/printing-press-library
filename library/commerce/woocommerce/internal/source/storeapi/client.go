// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

// Package storeapi is a minimal client for the WooCommerce Store API
// (/wp-json/wc/store/v1), which is PUBLIC: it needs no credentials and works
// against any WooCommerce store on the internet. That is what lets this CLI
// read catalogs of stores the operator does not own.
package storeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/woocommerce/internal/cliutil"
)

const (
	defaultRatePerSec = 4.0
	maxRetries        = 3
	userAgent         = "woocommerce-pp-cli (+https://github.com/mvanhorn/printing-press-library)"
)

// Prices mirrors the Store API price envelope.
//
// WooCommerce delivers these as MINOR units in a string: {"price":"4900",
// "currency_minor_unit":2} means 49.00. Reading them as-is is the single
// easiest way to be 100x wrong, so callers must go through MajorPrices.
type Prices struct {
	Price             string `json:"price"`
	RegularPrice      string `json:"regular_price"`
	SalePrice         string `json:"sale_price"`
	CurrencyCode      string `json:"currency_code"`
	CurrencyMinorUnit int    `json:"currency_minor_unit"`
}

// Product is the subset of a Store API product the snapshot commands persist.
type Product struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SKU       string `json:"sku"`
	Permalink string `json:"permalink"`
	OnSale    bool   `json:"on_sale"`
	InStock   bool   `json:"is_in_stock"`
	Prices    Prices `json:"prices"`
}

// MajorPrices converts the minor-unit strings into major currency units.
func (p Product) MajorPrices() (price, regular, sale float64) {
	return toMajor(p.Prices.Price, p.Prices.CurrencyMinorUnit),
		toMajor(p.Prices.RegularPrice, p.Prices.CurrencyMinorUnit),
		toMajor(p.Prices.SalePrice, p.Prices.CurrencyMinorUnit)
}

// CleanName strips the HTML entities the Store API embeds in product names.
func (p Product) CleanName() string { return cliutil.CleanText(p.Name) }

func toMajor(raw string, minorUnit int) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if minorUnit <= 0 {
		return v
	}
	div := 1.0
	for i := 0; i < minorUnit; i++ {
		div *= 10
	}
	return v / div
}

// Client talks to one store's public Store API.
type Client struct {
	productsURL string
	origin      string
	http        *http.Client
	limiter     *cliutil.AdaptiveLimiter
}

// New accepts https://store.com, https://store.com/, or https://store.com/wp-json
// and resolves the products endpoint from it.
func New(rawURL string, timeout time.Duration) (*Client, error) {
	raw := strings.TrimSpace(rawURL)
	if raw == "" {
		return nil, fmt.Errorf("store URL is empty")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parsing store URL %q: %w", rawURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("store URL must be http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("store URL %q has no host", rawURL)
	}
	origin := u.Scheme + "://" + u.Host
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		productsURL: origin + "/wp-json/wc/store/v1/products",
		origin:      origin,
		http:        &http.Client{Timeout: timeout},
		limiter:     cliutil.NewAdaptiveLimiter(defaultRatePerSec),
	}, nil
}

// Origin is the normalized store root, used as the snapshot key.
func (c *Client) Origin() string { return c.origin }

// Products fetches one page and reports the store's total page count.
//
// On exhausted 429 retries it returns a *cliutil.RateLimitError rather than an
// empty page: empty must mean "no products", never "we got throttled".
func (c *Client) Products(ctx context.Context, page, perPage int) ([]Product, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 100
	}
	endpoint := fmt.Sprintf("%s?page=%d&per_page=%d", c.productsURL, page, perPage)

	var lastRetryAfter time.Duration
	var lastBody string
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if c.limiter != nil {
			c.limiter.Wait()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, 0, fmt.Errorf("building request: %w", err)
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, 0, fmt.Errorf("fetching %s: %w", endpoint, err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, 0, fmt.Errorf("reading response from %s: %w", endpoint, readErr)
		}

		if remaining, resetAt, ok := cliutil.ParseRateLimitHeaders(resp.Header); ok && c.limiter != nil {
			c.limiter.ObserveHeaders(remaining, resetAt)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			if c.limiter != nil {
				c.limiter.OnRateLimit()
			}
			lastRetryAfter = cliutil.RetryAfter(resp)
			lastBody = strings.TrimSpace(string(body))
			if attempt == maxRetries {
				break
			}
			select {
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			case <-time.After(retryDelay(attempt, lastRetryAfter)):
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, 0, fmt.Errorf("%s returned HTTP %d: %s", endpoint, resp.StatusCode,
				cliutil.SanitizeErrorBody(string(body)))
		}
		if c.limiter != nil {
			c.limiter.OnSuccess()
		}

		var products []Product
		if err := json.Unmarshal(body, &products); err != nil {
			return nil, 0, fmt.Errorf("decoding products from %s: %w", endpoint, err)
		}
		totalPages := 0
		if v := strings.TrimSpace(resp.Header.Get("X-WP-TotalPages")); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				totalPages = n
			}
		}
		return products, totalPages, nil
	}
	return nil, 0, &cliutil.RateLimitError{URL: endpoint, RetryAfter: lastRetryAfter, Body: lastBody}
}

// retryDelay honours a server-supplied Retry-After when present, otherwise
// falls back to the shared exponential backoff.
func retryDelay(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return retryAfter
	}
	return cliutil.Backoff(attempt)
}
