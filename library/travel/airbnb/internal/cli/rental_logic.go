package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/hostextract"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/searchbackend"
	_ "github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/searchbackend/brave"
	_ "github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/searchbackend/duckduckgo"
	_ "github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/searchbackend/parallel"
	_ "github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/searchbackend/tavily"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/source/airbnb"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/source/vrbo"
)

type listingRef struct {
	Platform string
	ID       string
	URL      string
}

type cheapestParams struct {
	Checkin          string
	Checkout         string
	Guests           int
	SearchBackend    string
	MaxDirectResults int
}

type cheapestOutput struct {
	Listing  map[string]any `json:"listing"`
	Host     any            `json:"host"`
	Options  []any          `json:"options"`
	Cheapest any            `json:"cheapest,omitempty"`
	Method   string         `json:"method,omitempty"`
	Meta     map[string]any `json:"meta,omitempty"`
}

type directCandidate struct {
	URL        string   `json:"url,omitempty"`
	Title      string   `json:"title,omitempty"`
	Total      *float64 `json:"total,omitempty"`
	Confidence float64  `json:"confidence,omitempty"`
	Note       string   `json:"note,omitempty"`
	Domain     string   `json:"domain,omitempty"`
}

func parseListingURL(raw string) (*listingRef, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	host := strings.ToLower(u.Hostname())
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	ref := &listingRef{URL: raw}
	for i, p := range parts {
		switch {
		case strings.Contains(host, "airbnb.") && p == "rooms" && i+1 < len(parts):
			ref.Platform, ref.ID = "airbnb", parts[i+1]
		case strings.Contains(host, "vrbo.") && ref.ID == "" && strings.HasPrefix(p, "h") && numberish(strings.TrimPrefix(p, "h")):
			ref.Platform, ref.ID = "vrbo", strings.TrimPrefix(p, "h")
		case strings.Contains(host, "vrbo.") && ref.ID == "" && numberish(p):
			ref.Platform, ref.ID = "vrbo", p
		}
	}
	if ref.Platform == "" || ref.ID == "" {
		return nil, fmt.Errorf("unsupported listing URL: expected airbnb.com/rooms/{id} or vrbo.com/{id}")
	}
	return ref, nil
}

func stripURLArg(raw string) string {
	target := strings.TrimSpace(raw)
	return strings.Trim(target, `"'`)
}

func computeCheapest(ctx context.Context, rawURL string, p cheapestParams) (*cheapestOutput, error) {
	rawURL = stripURLArg(rawURL)
	ref, err := parseListingURL(rawURL)
	if err != nil {
		return nil, err
	}
	if p.Guests <= 0 {
		p.Guests = 1
	}
	if p.MaxDirectResults <= 0 {
		p.MaxDirectResults = 5
	}
	out := &cheapestOutput{Listing: map[string]any{"platform": ref.Platform, "id": ref.ID, "url": rawURL}}
	var host *hostextract.HostInfo
	var platformOption map[string]any
	switch ref.Platform {
	case "airbnb":
		l, err := airbnb.Get(ctx, ref.ID, airbnb.GetParams{Checkin: p.Checkin, Checkout: p.Checkout, Adults: p.Guests})
		if err != nil {
			return nil, err
		}
		host = hostextract.FromAirbnbListing(l)
		out.Listing["title"], out.Listing["city"] = l.Title, l.City
		total, fees := airbnbTotals(l)
		platformOption = map[string]any{"source": "airbnb", "url": l.URL, "total": nullableFloat(total), "fees": fees, "currency": "USD"}
		if total == 0 {
			platformOption["note"] = airbnbPricingUnavailableNote
		}
	case "vrbo":
		return nil, apiErr(vrbo.ErrDisabled)
	}
	out.Host = host
	out.Options = append(out.Options, platformOption)
	if ref.Platform == "airbnb" {
		out.Options = append(out.Options, map[string]any{"source": "vrbo", "url": "", "total": nil, "note": vrbo.ErrDisabled.Error()})
	} else {
		out.Options = append(out.Options, map[string]any{"source": "airbnb", "url": "", "total": nil, "note": "not searched (single-platform mode)"})
	}
	candidates, meta, _ := directCandidates(ctx, host, fmt.Sprint(out.Listing["title"]), fmt.Sprint(out.Listing["city"]), p)
	out.Options = append(out.Options, map[string]any{"source": "direct", "candidates": candidates})
	if meta != nil {
		out.Meta = meta
	}
	platformTotal := valueAsFloat(platformOption["total"])
	var best *directCandidate
	for i := range candidates {
		if candidates[i].Total != nil && (best == nil || *candidates[i].Total < *best.Total) {
			best = &candidates[i]
		}
	}
	if best != nil {
		cheapest := map[string]any{"source": "direct", "url": best.URL, "total": *best.Total}
		if platformTotal > 0 {
			cheapest["savings_vs_"+ref.Platform] = platformTotal - *best.Total
		}
		out.Cheapest = cheapest
	} else {
		out.Cheapest = map[string]any{"source": ref.Platform, "total": nullableFloat(platformTotal)}
	}
	return out, nil
}

const airbnbPricingUnavailableNote = "pricing not available in SSR; try 'auth login --chrome' or different dates"

func directCandidates(ctx context.Context, host *hostextract.HostInfo, listingTitle, city string, p cheapestParams) ([]directCandidate, map[string]any, error) {
	name := ""
	if host != nil {
		name = host.Brand
		if name == "" {
			name = host.Name
		}
	}
	if name == "" {
		return nil, nil, nil
	}
	if p.MaxDirectResults <= 0 {
		p.MaxDirectResults = 5
	}
	chain := searchbackend.Select(p.SearchBackend)
	query := directSearchQuery(listingTitle, name, city)
	searchLimit := p.MaxDirectResults * 3
	if searchLimit < 10 {
		searchLimit = 10
	}
	var (
		results          []searchbackend.Result
		activeBackend    string
		attemptedNames   []string
		backendErrors    []string
		usedFallback     bool
	)
	for i, backend := range chain {
		attemptedNames = append(attemptedNames, backend.Name())
		r, err := backend.Search(ctx, query, searchbackend.SearchOpts{Limit: searchLimit})
		if err != nil {
			backendErrors = append(backendErrors, backend.Name()+": "+err.Error())
			continue
		}
		if len(r) == 0 {
			backendErrors = append(backendErrors, backend.Name()+": no results")
			continue
		}
		results = r
		activeBackend = backend.Name()
		usedFallback = i > 0
		break
	}
	meta := map[string]any{
		"search_backend": activeBackend,
		"attempted":      attemptedNames,
	}
	if usedFallback {
		meta["fallback"] = true
		meta["fallback_reasons"] = backendErrors
	}
	if results == nil {
		if len(backendErrors) > 0 {
			meta["reason"] = backendErrors[len(backendErrors)-1]
		} else {
			meta["reason"] = "no_results"
		}
		return nil, meta, nil
	}
	var sources []searchbackend.Result
	for _, r := range results {
		domain := r.Domain
		if domain == "" {
			if u, err := url.Parse(r.URL); err == nil {
				domain = u.Hostname()
			}
		}
		if !isOTADomain(domain) && r.URL != "" {
			sources = append(sources, r)
		}
		if len(sources) >= p.MaxDirectResults {
			break
		}
	}
	fanout, errs := cliutil.FanoutRun(ctx, sources, func(r searchbackend.Result) string { return r.URL }, func(ctx context.Context, r searchbackend.Result) (directCandidate, error) {
		total, note := scanDirectPrice(ctx, r.URL, p.Checkin, p.Checkout)
		c := directCandidate{URL: r.URL, Title: r.Title, Domain: r.Domain, Confidence: r.Score, Note: note}
		if total > 0 {
			c.Total = &total
			c.Note = ""
		}
		return c, nil
	}, cliutil.WithConcurrency(3))
	_ = errs
	out := make([]directCandidate, 0, len(fanout))
	for _, r := range fanout {
		out = append(out, r.Value)
	}
	return out, meta, nil
}

func isOTADomain(domain string) bool {
	blocked := []string{"airbnb.", "vrbo.", "booking.", "expedia.", "hotels.", "tripadvisor.", "agoda.", "homeaway."}
	d := strings.ToLower(domain)
	for _, b := range blocked {
		if strings.HasPrefix(d, b) || strings.Contains(d, "."+b) || d == strings.TrimSuffix(b, ".") {
			return true
		}
	}
	return false
}

func scanDirectPrice(ctx context.Context, rawURL, checkin, checkout string) (float64, string) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return 0, "found_site_no_price"
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_6_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "found_site_no_price"
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, "found_site_blocked"
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	re := regexp.MustCompile(`(?i)\$\s*([0-9][0-9,]*(?:\.[0-9]{2})?)\s*(?:total|night|/night)?`)
	body := string(data)
	matches := re.FindAllStringSubmatchIndex(body, 40)
	var bestTotal, bestNightly float64
	for _, m := range matches {
		raw := body[m[0]:m[1]]
		amount := body[m[2]:m[3]]
		v, _ := strconv.ParseFloat(strings.ReplaceAll(amount, ",", ""), 64)
		if v < 50 {
			continue
		}
		start, end := m[0]-80, m[1]+80
		if start < 0 {
			start = 0
		}
		if end > len(body) {
			end = len(body)
		}
		context := strings.ToLower(body[start:end])
		switch {
		case strings.Contains(context, "total"):
			if bestTotal == 0 || v < bestTotal {
				bestTotal = v
			}
		case strings.Contains(context, "night") || strings.Contains(context, "/night"):
			if bestNightly == 0 || v < bestNightly {
				bestNightly = v
			}
		case strings.Contains(strings.ToLower(raw), "night"):
			if bestNightly == 0 || v < bestNightly {
				bestNightly = v
			}
		}
	}
	if bestTotal > 0 {
		return bestTotal, ""
	}
	if bestNightly > 0 {
		if nights := stayNights(checkin, checkout); nights > 0 {
			return bestNightly * float64(nights), ""
		}
		return bestNightly, ""
	}
	if len(matches) == 0 {
		return 0, "found_site_no_price"
	}
	return 0, "found_site_no_price"
}

func stayNights(checkin, checkout string) int {
	if checkin == "" || checkout == "" {
		return 0
	}
	in, err := time.Parse("2006-01-02", checkin)
	if err != nil {
		return 0
	}
	out, err := time.Parse("2006-01-02", checkout)
	if err != nil {
		return 0
	}
	nights := int(out.Sub(in).Hours() / 24)
	if nights < 0 {
		return 0
	}
	return nights
}

func directSearchQuery(listingTitle, hostName, city string) string {
	parts := []string{titleSearchPhrase(listingTitle), hostName, city, "vacation rental", "direct booking"}
	var cleaned []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return strings.Join(cleaned, " ")
}

func titleSearchPhrase(s string) string {
	for _, sep := range []string{",", "|", " - "} {
		if before, _, ok := strings.Cut(s, sep); ok {
			s = before
			break
		}
	}
	return truncateToWords(s, 5)
}

func truncateToWords(s string, max int) string {
	words := strings.Fields(s)
	if max <= 0 || len(words) <= max {
		return strings.Join(words, " ")
	}
	return strings.Join(words[:max], " ")
}

func airbnbTotals(l *airbnb.Listing) (float64, map[string]float64) {
	fees := map[string]float64{}
	if l == nil {
		return 0, fees
	}
	if l.PriceBreakdown == nil {
		return l.PriceTotal, fees
	}
	if l.PriceBreakdown.Total == 0 {
		return l.PriceTotal, l.PriceBreakdown.Fees
	}
	return l.PriceBreakdown.Total, l.PriceBreakdown.Fees
}

func vrboTotals(l *vrbo.Property) (float64, map[string]float64) {
	fees := map[string]float64{}
	if l == nil || l.PriceBreakdown == nil {
		return 0, fees
	}
	return l.PriceBreakdown.Total, l.PriceBreakdown.Fees
}

func dryRunCheapest(rawURL string) *cheapestOutput {
	rawURL = stripURLArg(rawURL)
	ref, _ := parseListingURL(rawURL)
	if ref == nil {
		ref = &listingRef{Platform: "airbnb", ID: "12345", URL: rawURL}
	}
	return &cheapestOutput{
		Listing: map[string]any{"platform": ref.Platform, "id": ref.ID, "title": "dry run listing", "city": ""},
		Host:    map[string]any{"name": "", "type": "individual", "confidence": 0},
		Options: []any{
			map[string]any{"source": ref.Platform, "url": rawURL, "total": nil, "fees": map[string]float64{}, "currency": "USD"},
			map[string]any{"source": "direct", "candidates": []any{}},
		},
		Cheapest: map[string]any{"source": ref.Platform, "total": nil},
		Method:   "dry_run",
	}
}

func nullableFloat(v float64) any {
	if v == 0 {
		return nil
	}
	return v
}

func valueAsFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case *float64:
		if x != nil {
			return *x
		}
	}
	return 0
}

func numberish(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func parseSinceDate(s string) int64 {
	if s == "" {
		return 0
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Unix()
	}
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d).Unix()
	}
	return 0
}

func sortBySavings(items []map[string]any) {
	sort.Slice(items, func(i, j int) bool {
		return valueAsFloat(items[i]["savings"]) > valueAsFloat(items[j]["savings"])
	})
}
