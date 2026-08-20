// Package rdap is a thin client over github.com/openrdap/rdap that returns
// our domain-shaped responses with availability + events + status.
package rdap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	openrdap "github.com/openrdap/rdap"
	openbootstrap "github.com/openrdap/rdap/bootstrap"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/cliutil"
)

// limiter paces outbound RDAP requests. RDAP servers are operated by
// individual registries and typically accept low single-digit QPS without
// throttling; the adaptive logic ramps up on success and halves on 429.
var limiter = cliutil.NewAdaptiveLimiter(2.0)

// ErrRateLimited is returned when an RDAP server responds with HTTP 429.
// The openrdap library surfaces HTTP errors via its error string; we detect
// the 429 case here and convert it to a typed error so callers can back off.
type ErrRateLimited struct {
	RetryAfter time.Duration
}

func (e *ErrRateLimited) Error() string {
	return fmt.Sprintf("rdap: rate limited (HTTP 429), retry after %s", e.RetryAfter)
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

// rateLimitTransport wraps http.RoundTripper and intercepts HTTP 429 responses,
// stashing the Retry-After hint on a shared pointer so Lookup can return a
// typed *ErrRateLimited to the caller. The openrdap library does not expose
// the underlying HTTP response, so this is the cleanest interception point.
type rateLimitTransport struct {
	base    http.RoundTripper
	limited *time.Duration
}

func (t *rateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err == nil && resp != nil && resp.StatusCode == http.StatusTooManyRequests {
		d := parseRetryAfter(resp.Header.Get("Retry-After"))
		*t.limited = d
	}
	return resp, err
}

// Result is the structured outcome of an RDAP lookup.
type Result struct {
	FQDN       string          `json:"fqdn"`
	Available  bool            `json:"available"`
	Status     []string        `json:"status,omitempty"`
	StatusText string          `json:"status_text,omitempty"`
	Events     []EventEntry    `json:"events,omitempty"`
	Source     string          `json:"source"`
	RDAPServer string          `json:"rdap_server,omitempty"`
	Raw        json.RawMessage `json:"raw,omitempty"`
}

// EventEntry captures one RDAP event (e.g., registration, expiration).
type EventEntry struct {
	Action string `json:"action"`
	Date   string `json:"date"`
	Actor  string `json:"actor,omitempty"`
}

// Lookup runs an RDAP query for fqdn with a deadline. If the RDAP server
// returns HTTP 429, Lookup returns a typed *ErrRateLimited error so callers
// can honor the Retry-After hint.
func Lookup(ctx context.Context, fqdn string) (*Result, error) {
	if fqdn == "" {
		return nil, errors.New("empty fqdn")
	}
	limiter.Wait()
	var rateLimited time.Duration
	rlt := &rateLimitTransport{base: http.DefaultTransport, limited: &rateLimited}
	c := &openrdap.Client{
		HTTP:      &http.Client{Timeout: 12 * time.Second, Transport: rlt},
		Bootstrap: &openbootstrap.Client{HTTP: &http.Client{Timeout: 10 * time.Second, Transport: rlt}},
	}
	req := &openrdap.Request{
		Type:  openrdap.DomainRequest,
		Query: fqdn,
	}
	req = req.WithContext(ctx)
	resp, err := c.Do(req)
	if rateLimited > 0 {
		limiter.OnRateLimit()
		return nil, &ErrRateLimited{RetryAfter: rateLimited}
	}
	if err != nil {
		return rdapAvailableFromError(fqdn, err), err
	}
	limiter.OnSuccess()
	if resp == nil || resp.Object == nil {
		return &Result{FQDN: fqdn, Available: true, Source: "rdap", StatusText: "404"}, nil
	}
	dom, ok := resp.Object.(*openrdap.Domain)
	if !ok {
		raw, _ := json.Marshal(resp.Object)
		return &Result{FQDN: fqdn, Available: false, Source: "rdap", Raw: raw}, nil
	}
	out := &Result{
		FQDN:       fqdn,
		Source:     "rdap",
		Status:     dom.Status,
		StatusText: strings.Join(dom.Status, ","),
	}
	for _, e := range dom.Events {
		out.Events = append(out.Events, EventEntry{Action: e.Action, Date: e.Date, Actor: e.Actor})
	}
	if raw, err := json.Marshal(dom); err == nil {
		out.Raw = raw
	}
	return out, nil
}

// rdapAvailableFromError inspects an openrdap error: a 404 is the canonical
// "available" signal. Anything else returns nil result and the caller falls
// back to WHOIS.
func rdapAvailableFromError(fqdn string, err error) *Result {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "404") || strings.Contains(msg, "not found") || strings.Contains(msg, "no matching records") {
		return &Result{FQDN: fqdn, Available: true, Source: "rdap", StatusText: "404"}
	}
	return nil
}

// FindEvent returns the first event matching the action (case-insensitive) or zero.
func (r *Result) FindEvent(action string) (EventEntry, bool) {
	for _, e := range r.Events {
		if strings.EqualFold(e.Action, action) {
			return e, true
		}
	}
	return EventEntry{}, false
}

// ExpiresAt returns the RDAP `expiration` event date, or empty.
func (r *Result) ExpiresAt() string {
	if e, ok := r.FindEvent("expiration"); ok {
		return e.Date
	}
	return ""
}

// CreatedAt returns the RDAP `registration` event date, or empty.
func (r *Result) CreatedAt() string {
	if e, ok := r.FindEvent("registration"); ok {
		return e.Date
	}
	return ""
}

// EventsJSON serializes events to a JSON string for storage.
func (r *Result) EventsJSON() string {
	if len(r.Events) == 0 {
		return "[]"
	}
	b, err := json.Marshal(r.Events)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// String formats for human display.
func (r *Result) String() string {
	if r.Available {
		return fmt.Sprintf("%s: available (rdap 404)", r.FQDN)
	}
	return fmt.Sprintf("%s: %s", r.FQDN, r.StatusText)
}
