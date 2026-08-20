package ucp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultHTTPClient is the HTTP client used by manifest and catalog calls.
// Override via SetHTTPClient in tests.
var DefaultHTTPClient = &http.Client{Timeout: 15 * time.Second}

// FetchManifest fetches and parses the /.well-known/ucp document for a domain.
// Accepts forms like "example.com", "https://example.com", "https://example.com/".
func FetchManifest(ctx context.Context, domain string) (*Manifest, error) {
	u, err := normalizeDomain(domain)
	if err != nil {
		return nil, fmt.Errorf("invalid domain %q: %w", domain, err)
	}
	u.Path = "/.well-known/ucp"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ucp-pp-cli/0.1 (+https://github.com/mvanhorn/printing-press-library)")
	resp, err := DefaultHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u.String(), err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no /.well-known/ucp manifest at %s (HTTP 404) — the domain is not UCP-enabled", u.Host)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s: HTTP %d", u.String(), resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") && !strings.HasPrefix(ct, "application/ld+json") {
		// Some merchants serve text/plain with JSON body; tolerate but flag in validate.
	}
	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("parse manifest JSON: %w (first 200B: %s)", err, truncate(string(body), 200))
	}
	return &m, nil
}

// ValidationReport is the structured output of Validate. Grade is A–F based on
// completeness: A = all core capabilities + payment_handlers + multiple transports;
// F = unparseable or missing version.
type ValidationReport struct {
	Grade           string            `json:"grade"`
	Score           int               `json:"score"` // 0–100
	Errors          []string          `json:"errors,omitempty"`
	Warnings        []string          `json:"warnings,omitempty"`
	Capabilities    []string          `json:"capabilities"`     // sorted reverse-domain names
	Transports      []string          `json:"transports"`       // rest/mcp/a2a/embedded
	PaymentHandlers []string          `json:"payment_handlers"` // sorted reverse-domain names
	Version         string            `json:"version"`
	ProtocolStatus  map[string]string `json:"protocol_status,omitempty"` // per-capability minor notes
}

// Validate inspects a manifest and returns a graded report. It does not fetch
// any network resources beyond what FetchManifest already retrieved.
func Validate(m *Manifest) *ValidationReport {
	r := &ValidationReport{
		Version:        m.UCP.Version,
		ProtocolStatus: map[string]string{},
		// Initialize as empty slices so JSON serializes as [] not null when empty —
		// agent consumers expect arrays, not nulls.
		Capabilities: []string{},
		Transports:   []string{},
	}
	if m.UCP.Version == "" {
		r.Errors = append(r.Errors, "ucp.version is empty (required)")
	}
	// Transports
	tseen := map[string]bool{}
	for _, svcs := range m.UCP.Services {
		for _, s := range svcs {
			if s.Transport != "" {
				tseen[s.Transport] = true
			}
		}
	}
	for t := range tseen {
		r.Transports = append(r.Transports, t)
	}
	// Capabilities
	for name := range m.UCP.Capabilities {
		r.Capabilities = append(r.Capabilities, name)
	}
	// Payment handlers
	// Initialize as empty slice so JSON serializes as [] not null when there
	// are no handlers — agent consumers expect arrays, not nulls.
	r.PaymentHandlers = []string{}
	for name := range m.UCP.PaymentHandlers {
		r.PaymentHandlers = append(r.PaymentHandlers, name)
	}
	sortStrings(r.Transports)
	sortStrings(r.Capabilities)
	sortStrings(r.PaymentHandlers)

	// Score
	score := 0
	if m.UCP.Version != "" {
		score += 20
	}
	if hasAny(r.Capabilities, "dev.ucp.shopping.checkout") {
		score += 20
	}
	if hasAnyPrefix(r.Capabilities, "dev.ucp.shopping.catalog") {
		score += 15
	}
	if hasAny(r.Capabilities, "dev.ucp.shopping.cart") {
		score += 10
	}
	if hasAny(r.Capabilities, "dev.ucp.shopping.fulfillment") {
		score += 5
	}
	if hasAny(r.Capabilities, "dev.ucp.shopping.discount") {
		score += 5
	}
	if hasAny(r.Capabilities, "dev.ucp.shopping.order") {
		score += 10
	}
	if len(r.PaymentHandlers) > 0 {
		score += 10
	}
	if len(r.Transports) >= 2 {
		score += 5
	}
	r.Score = score
	switch {
	case len(r.Errors) > 0:
		r.Grade = "F"
	case score >= 90:
		r.Grade = "A"
	case score >= 75:
		r.Grade = "B"
	case score >= 60:
		r.Grade = "C"
	case score >= 40:
		r.Grade = "D"
	default:
		r.Grade = "E"
	}
	if !hasAny(r.Capabilities, "dev.ucp.shopping.checkout") {
		r.Warnings = append(r.Warnings, "missing dev.ucp.shopping.checkout capability — agent cannot complete purchases")
	}
	if !hasAnyPrefix(r.Capabilities, "dev.ucp.shopping.catalog") {
		r.Warnings = append(r.Warnings, "no catalog.* capability — agent cannot search/lookup products")
	}
	if len(r.PaymentHandlers) == 0 {
		r.Warnings = append(r.Warnings, "no payment_handlers declared — checkout cannot complete via AP2 or Google Pay")
	}
	return r
}

// EndpointFor returns the (transport, endpoint) tuple for a service, preferring
// the caller's transport preference order. Returns ("", "") if no match.
func (m *Manifest) EndpointFor(service string, preferTransport ...string) (transport, endpoint string) {
	svcs := m.UCP.Services[service]
	if len(svcs) == 0 {
		return "", ""
	}
	// Try caller preferences first.
	for _, pref := range preferTransport {
		for _, s := range svcs {
			if s.Transport == pref && s.Endpoint != "" {
				return s.Transport, s.Endpoint
			}
		}
	}
	// Fallback: first service with a non-empty endpoint.
	for _, s := range svcs {
		if s.Endpoint != "" {
			return s.Transport, s.Endpoint
		}
	}
	// Last resort: first service even without endpoint (e.g. embedded).
	return svcs[0].Transport, svcs[0].Endpoint
}

// normalizeDomain accepts "example.com", "https://example.com", "https://example.com/path"
// and returns a *url.URL with the scheme + host set.
func normalizeDomain(domain string) (*url.URL, error) {
	d := strings.TrimSpace(domain)
	if d == "" {
		return nil, fmt.Errorf("empty")
	}
	if !strings.Contains(d, "://") {
		// Allow "localhost:8080" without forcing https.
		if strings.HasPrefix(d, "localhost") || strings.HasPrefix(d, "127.0.0.1") {
			d = "http://" + d
		} else {
			d = "https://" + d
		}
	}
	u, err := url.Parse(d)
	if err != nil {
		return nil, err
	}
	if u.Host == "" {
		return nil, fmt.Errorf("missing host")
	}
	return &url.URL{Scheme: u.Scheme, Host: u.Host}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func sortStrings(s []string) {
	// tiny dependency-free sort to avoid importing "sort" twice across files
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

func hasAny(xs []string, name string) bool {
	for _, x := range xs {
		if x == name {
			return true
		}
	}
	return false
}

func hasAnyPrefix(xs []string, prefix string) bool {
	for _, x := range xs {
		if strings.HasPrefix(x, prefix) {
			return true
		}
	}
	return false
}
