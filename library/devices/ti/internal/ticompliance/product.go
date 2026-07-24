// Copyright 2026 Arun Gopalakrishnan and contributors. Licensed under Apache-2.0. See LICENSE.

// Package ticompliance implements TI-specific compliance acquisition:
// anonymous product-page JSON-LD parsing, the blanket statement-PDF catalog,
// and cookie-injected Browserbase access to the login-gated Material Content
// surfaces (Environmental Ratings, IPC-1752A Class-D FMD XML).
package ticompliance

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const productUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

// OrderableCompliance is the anonymous compliance view for one orderable part
// number, read from the product page's JSON-LD block. Raw TI vocabulary is
// preserved verbatim ("Yes"/"No"); normalization is the caller's job.
type OrderableCompliance struct {
	PartNumber      string `json:"part_number"`
	Generic         string `json:"generic"`
	Title           string `json:"title,omitempty"`
	RoHS            string `json:"rohs_compliance,omitempty"`
	REACH           string `json:"reach_compliance,omitempty"`
	MSLRating       string `json:"msl_rating,omitempty"`
	LeadFinish      string `json:"lead_finish,omitempty"`
	Packaging       string `json:"packaging,omitempty"`
	Pins            string `json:"pins,omitempty"`
	MarketingStatus string `json:"marketing_status,omitempty"`
	SourceURL       string `json:"source_url"`
	FetchedAt       time.Time `json:"fetched_at"`
}

var ldJSONRe = regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)

// ldProduct mirrors the slice of schema.org Product JSON-LD the TI product
// page emits that we need: generic identity plus per-orderable offers.
type ldProduct struct {
	Type        string    `json:"@type"`
	Name        string    `json:"name"`
	SKU         string    `json:"sku"`
	MPN         string    `json:"mpn"`
	Description string    `json:"description"`
	Offers      []ldOffer `json:"offers"`
}

type ldOffer struct {
	ItemOffered struct {
		SKU                string   `json:"sku"`
		MPN                string   `json:"mpn"`
		AdditionalProperty []ldProp `json:"additionalProperty"`
	} `json:"itemOffered"`
}

type ldProp struct {
	Name  string          `json:"name"`
	Value json.RawMessage `json:"value"`
}

func (p ldProp) valueString() string {
	var s string
	if err := json.Unmarshal(p.Value, &s); err == nil {
		return s
	}
	return strings.Trim(string(p.Value), `"`)
}

// FetchProductPage GETs a TI product page and returns the body, following
// redirects. A 404 returns errNotFound so ResolveCompliance can keep stripping
// suffix characters.
var errNotFound = fmt.Errorf("product page not found")

func fetchProductPage(ctx context.Context, client *http.Client, generic string) (string, string, error) {
	url := "https://www.ti.com/product/" + generic
	// TI rate-limits the plain-HTTP product page with transient 429/503; retry
	// a few times with backoff before surfacing the error.
	const maxAttempts = 4
	var lastStatus int
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", url, err
		}
		req.Header.Set("User-Agent", productUserAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		resp, err := client.Do(req)
		if err != nil {
			return "", url, err
		}
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNotAcceptable {
			resp.Body.Close()
			return "", url, errNotFound
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			resp.Body.Close()
			lastStatus = resp.StatusCode
			if attempt < maxAttempts {
				if err := sleepCtx(ctx, time.Duration(attempt)*2*time.Second); err != nil {
					return "", url, err
				}
				continue
			}
			return "", url, fmt.Errorf("GET %s: HTTP %d after %d attempts (TI rate-limited)", url, lastStatus, maxAttempts)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return "", url, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if err != nil {
			return "", url, err
		}
		return string(body), url, nil
	}
	return "", url, fmt.Errorf("GET %s: HTTP %d after %d attempts", url, lastStatus, maxAttempts)
}

// parseLDProduct extracts and parses the first JSON-LD Product block.
func parseLDProduct(page string) (*ldProduct, error) {
	m := ldJSONRe.FindStringSubmatch(page)
	if m == nil {
		return nil, fmt.Errorf("no application/ld+json block on page")
	}
	var p ldProduct
	if err := json.Unmarshal([]byte(html.UnescapeString(m[1])), &p); err != nil {
		return nil, fmt.Errorf("parsing JSON-LD: %w", err)
	}
	return &p, nil
}

// genericCandidates yields resolution candidates for an OPN by progressively
// stripping trailing characters (TUSB320RWBR -> TUSB320RWB -> ... -> TUSB320).
// The full OPN is tried first; candidates shorter than 4 chars are skipped.
func genericCandidates(opn string) []string {
	opn = strings.ToUpper(strings.TrimSpace(opn))
	var out []string
	for c := opn; len(c) >= 4 && len(out) < 10; c = c[:len(c)-1] {
		out = append(out, c)
	}
	return out
}

// ResolveCompliance resolves an orderable part number to its generic's product
// page and returns the orderable's anonymous compliance view. Fail-closed: an
// error is returned when no page resolves, when the OPN is not among the
// page's orderables, or when both RoHS and REACH come back empty.
func ResolveCompliance(ctx context.Context, opn string) (*OrderableCompliance, error) {
	if err := ValidateOPN(opn); err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 25 * time.Second}
	opnUC := strings.ToUpper(strings.TrimSpace(opn))
	var lastErr error
	for _, cand := range genericCandidates(opnUC) {
		page, url, err := fetchProductPage(ctx, client, cand)
		if err == errNotFound {
			lastErr = err
			continue
		}
		if err != nil {
			return nil, err
		}
		prod, err := parseLDProduct(page)
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", url, err)
			continue
		}
		res, found := matchOrderable(prod, opnUC)
		if !found {
			var avail []string
			for _, o := range prod.Offers {
				avail = append(avail, o.ItemOffered.SKU)
			}
			return nil, fmt.Errorf("part %s not among orderables on %s (available: %s)", opnUC, url, strings.Join(avail, ", "))
		}
		res.SourceURL = url
		res.FetchedAt = time.Now().UTC()
		if res.RoHS == "" && res.REACH == "" {
			return nil, fmt.Errorf("page %s carried neither RoHS nor REACH for %s — refusing partial result", url, opnUC)
		}
		return res, nil
	}
	if lastErr == nil {
		lastErr = errNotFound
	}
	return nil, fmt.Errorf("could not resolve a TI product page for %s: %w", opnUC, lastErr)
}

func matchOrderable(prod *ldProduct, opnUC string) (*OrderableCompliance, bool) {
	for _, o := range prod.Offers {
		sku := strings.ToUpper(strings.TrimSpace(o.ItemOffered.SKU))
		if sku != opnUC && !strings.HasPrefix(sku, opnUC+".") {
			continue
		}
		res := &OrderableCompliance{
			PartNumber: o.ItemOffered.SKU,
			Generic:    prod.Name,
			Title:      strings.TrimSpace(prod.Description),
		}
		for _, p := range o.ItemOffered.AdditionalProperty {
			v := strings.TrimSpace(p.valueString())
			if v == "null" || v == "None" {
				v = ""
			}
			switch p.Name {
			case "rohsCompliant":
				res.RoHS = v
			case "reachStatus":
				res.REACH = v
			case "mslRatingPeakReflow":
				res.MSLRating = v
			case "leadball":
				if v != "null" && v != "None" {
					res.LeadFinish = v
				}
			case "packaging":
				res.Packaging = v
			case "pins":
				res.Pins = v
			case "marketingStatusText":
				res.MarketingStatus = v
			}
		}
		return res, true
	}
	return nil, false
}
