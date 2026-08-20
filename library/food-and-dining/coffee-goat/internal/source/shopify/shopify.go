// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

// Package shopify is the source adapter for Shopify-storefront-shaped
// roasters. Every Shopify storefront exposes /products.json with a
// stable schema (probed across all 21 Tier-1 roasters during scoping).
// This adapter fetches a single roaster's catalog, normalises each
// product into the unified RoasterProduct shape, and applies any
// roaster-specific filter declared in the registry (e.g. Black &
// White's product_type=Coffee narrowing).
package shopify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/extract"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/roasters"
)

// RoasterProduct is the unified product shape every source adapter
// emits. Stored in the roaster_products table.
type RoasterProduct struct {
	RoasterSlug string
	Handle      string
	Title       string
	Vendor      string
	BodyText    string
	Origin      string
	Producer    string
	Process     string
	Varietal    string
	Altitude    string
	RoastLevel  string
	TagsJSON    string
	PriceCents  int
	Currency    string
	WeightG     int
	URL         string
	ImageURL    string
	InStock     bool
	PublishedAt string
	UpdatedAt   string
}

// shopifyProductsResponse mirrors the slice of /products.json this
// adapter consumes. Only fields it uses are declared; extra fields
// in the upstream response are ignored.
type shopifyProductsResponse struct {
	Products []shopifyProduct `json:"products"`
}

type shopifyProduct struct {
	ID          int64            `json:"id"`
	Title       string           `json:"title"`
	Handle      string           `json:"handle"`
	Vendor      string           `json:"vendor"`
	ProductType string           `json:"product_type"`
	BodyHTML    string           `json:"body_html"`
	Tags        []string         `json:"tags"`
	Variants    []shopifyVariant `json:"variants"`
	Images      []shopifyImage   `json:"images"`
	PublishedAt string           `json:"published_at"`
	UpdatedAt   string           `json:"updated_at"`
}

type shopifyVariant struct {
	Price     string `json:"price"`
	Grams     int    `json:"grams"`
	Available bool   `json:"available"`
}

type shopifyImage struct {
	Src string `json:"src"`
}

// Fetcher is the entrypoint for callers. limiter may be nil; a nil
// limiter means no pacing.
type Fetcher struct {
	HTTP    *http.Client
	Limiter *cliutil.AdaptiveLimiter
}

// New returns a Fetcher with sensible defaults.
func New() *Fetcher {
	return &Fetcher{
		HTTP:    &http.Client{Timeout: 20 * time.Second},
		Limiter: cliutil.NewAdaptiveLimiter(2.0), // 2 req/s starting floor
	}
}

// Fetch pulls one roaster's full catalog. Pagination uses `since_id`
// (the modern Shopify-recommended approach); we follow until a
// response returns fewer than the page limit.
func (f *Fetcher) Fetch(ctx context.Context, r roasters.Roaster) ([]RoasterProduct, error) {
	if r.Transport != roasters.TransportShopify {
		return nil, fmt.Errorf("shopify.Fetch: roaster %s has transport %q, want %q",
			r.Slug, r.Transport, roasters.TransportShopify)
	}

	var all []RoasterProduct
	sinceID := int64(0)
	const pageLimit = 250
	const maxPages = 20 // 5,000-item ceiling per roaster

	for page := 0; page < maxPages; page++ {
		f.Limiter.Wait()
		url := r.SyncURL
		sep := "?"
		if strings.Contains(url, "?") {
			sep = "&"
		}
		url = fmt.Sprintf("%s%slimit=%d", url, sep, pageLimit)
		if sinceID > 0 {
			url = fmt.Sprintf("%s&since_id=%d", url, sinceID)
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return all, fmt.Errorf("shopify.Fetch %s: %w", r.Slug, err)
		}
		req.Header.Set("User-Agent", "coffee-goat-pp-cli (+specialty-coffee aggregator)")
		req.Header.Set("Accept", "application/json")

		resp, err := f.HTTP.Do(req)
		if err != nil {
			return all, fmt.Errorf("shopify.Fetch %s: %w", r.Slug, err)
		}
		if resp.StatusCode == 429 {
			f.Limiter.OnRateLimit()
			retry := cliutil.RetryAfter(resp)
			resp.Body.Close()
			return all, &cliutil.RateLimitError{
				URL:        url,
				RetryAfter: retry,
				Body:       fmt.Sprintf("shopify roaster=%s", r.Slug),
			}
		}
		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			return all, fmt.Errorf("shopify.Fetch %s: HTTP %d: %s", r.Slug, resp.StatusCode, strings.TrimSpace(string(body)))
		}
		f.Limiter.OnSuccess()

		var parsed shopifyProductsResponse
		if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
			resp.Body.Close()
			return all, fmt.Errorf("shopify.Fetch %s decode: %w", r.Slug, err)
		}
		resp.Body.Close()

		if len(parsed.Products) == 0 {
			break
		}

		for _, p := range parsed.Products {
			// Advance the since_id cursor for every product on the page,
			// not just those that pass the filter. A merch-only page with
			// every product filtered out would otherwise leave sinceID
			// unchanged, the next request would re-fetch the same page,
			// and pagination would stall until the maxPages cap silently
			// drops every coffee product that follows.
			if p.ID > sinceID {
				sinceID = p.ID
			}
			if !passesFilters(p, r.Filters) {
				continue
			}
			rp := normalise(r, p)
			all = append(all, rp)
		}

		if len(parsed.Products) < pageLimit {
			break
		}
		// Dogfood / verify environments curtail to the first page so
		// the per-command timeout doesn't trip.
		if cliutil.IsDogfoodEnv() || cliutil.IsVerifyEnv() {
			break
		}
	}

	return all, nil
}

// passesFilters applies the roaster-specific filter map. Currently
// only product_type is honoured (Black & White: Coffee).
func passesFilters(p shopifyProduct, filters map[string]string) bool {
	if want, ok := filters["product_type"]; ok && want != "" {
		if !strings.EqualFold(p.ProductType, want) {
			return false
		}
	}
	return true
}

// normalise converts a raw Shopify product into the unified shape.
func normalise(r roasters.Roaster, p shopifyProduct) RoasterProduct {
	bodyText := extract.Cleanup(p.BodyHTML)
	attrs := extract.FromBody(bodyText)

	priceCents := 0
	weightG := 0
	inStock := false
	if len(p.Variants) > 0 {
		v := p.Variants[0]
		priceCents = parsePriceToCents(v.Price)
		weightG = v.Grams
		for _, vv := range p.Variants {
			if vv.Available {
				inStock = true
				break
			}
		}
	}

	imageURL := ""
	if len(p.Images) > 0 {
		imageURL = p.Images[0].Src
	}

	tagsJSON := ""
	if len(p.Tags) > 0 {
		if b, err := json.Marshal(p.Tags); err == nil {
			tagsJSON = string(b)
		}
	}

	// Product URL: best-effort, derived from the storefront's host
	// + /products/<handle>. Not 100% accurate (some roasters use
	// custom domains under www. variants), but stable enough for
	// users to click through.
	productURL := storefrontProductURL(r.SyncURL, p.Handle)

	return RoasterProduct{
		RoasterSlug: r.Slug,
		Handle:      p.Handle,
		Title:       p.Title,
		Vendor:      p.Vendor,
		BodyText:    bodyText,
		Origin:      attrs.Origin,
		Producer:    attrs.Producer,
		Process:     attrs.Process,
		Varietal:    attrs.Varietal,
		Altitude:    attrs.Altitude,
		RoastLevel:  "",
		TagsJSON:    tagsJSON,
		PriceCents:  priceCents,
		Currency:    "USD", // Shopify storefront default; some roasters override
		WeightG:     weightG,
		URL:         productURL,
		ImageURL:    imageURL,
		InStock:     inStock,
		PublishedAt: p.PublishedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

// parsePriceToCents parses "32.50" -> 3250. Returns 0 on parse
// failure; the upstream API uses string-typed prices so a permissive
// parser is correct.
func parsePriceToCents(s string) int {
	if s == "" {
		return 0
	}
	// Strip non-digit / non-dot characters (currency symbols, commas).
	var b strings.Builder
	for _, c := range s {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		} else if c == '.' {
			b.WriteRune(c)
		}
	}
	parts := strings.SplitN(b.String(), ".", 2)
	if len(parts) == 0 || parts[0] == "" {
		return 0
	}
	dollars := atoiSafe(parts[0])
	cents := 0
	if len(parts) == 2 {
		frac := parts[1]
		if len(frac) > 2 {
			frac = frac[:2]
		}
		for len(frac) < 2 {
			frac += "0"
		}
		cents = atoiSafe(frac)
	}
	return dollars*100 + cents
}

func atoiSafe(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			continue
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func storefrontProductURL(syncURL, handle string) string {
	// Replace "/products.json" suffix with "/products/<handle>".
	if i := strings.LastIndex(syncURL, "/products.json"); i >= 0 {
		return syncURL[:i] + "/products/" + handle
	}
	return syncURL
}
