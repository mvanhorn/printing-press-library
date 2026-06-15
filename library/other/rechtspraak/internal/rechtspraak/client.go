// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

package rechtspraak

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/rechtspraak/internal/cliutil"
)

// RateLimitError is returned when the server signals an HTTP 429 (or 503).
// Callers can type-assert via errors.As to honour the Retry-After hint in
// batch workflows or surface a typed rate-limit signal to the user.
type RateLimitError struct {
	Path       string
	StatusCode int
	RetryAfter time.Duration
	Body       string
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rechtspraak %s: HTTP %d rate-limited (retry after %s): %s", e.Path, e.StatusCode, e.RetryAfter, truncate(e.Body, 200))
}

// IsRateLimitError reports whether err is (or wraps) a *RateLimitError.
func IsRateLimitError(err error) bool {
	var r *RateLimitError
	return errors.As(err, &r)
}

// HTTP wraps the data.rechtspraak.nl API with typed fetchers. The IVO 1.15
// guidance is explicit: "preferably no concurrent requests" — so this client
// is serial by construction and shares a single process-wide
// cliutil.AdaptiveLimiter for outbound pacing. Sharing the limiter is
// load-bearing: every HTTP instance (CLI command, MCP endpoint tool, sync
// loop) talks to the same host, so giving each its own limiter would let N
// concurrent agents punch through the floor N times over. The limiter halves
// on 429 and ramps up on success, discovering the true server ceiling
// without manual tuning.
type HTTP struct {
	BaseURL string
	UA      string
	Client  *http.Client
	Limiter *cliutil.AdaptiveLimiter
}

// sharedLimiter is the process-wide limiter every NewHTTP() reuses by default.
// data.rechtspraak.nl is a single host with a single shared budget; one
// limiter per process matches that reality.
var sharedLimiter = cliutil.NewAdaptiveLimiter(10)

// NewHTTP returns a default polite client paced via the shared
// cliutil.AdaptiveLimiter at 10 req/s floor. The limiter discovers the true
// ceiling adaptively (halving on 429, ramping on consecutive successes).
func NewHTTP() *HTTP {
	return &HTTP{
		BaseURL: "https://data.rechtspraak.nl",
		UA:      "rechtspraak-pp-cli/0.1.0 (+printing-press)",
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
		Limiter: sharedLimiter,
	}
}

// pace is a thin shim over the AdaptiveLimiter so existing call sites
// keep working. The limiter is nil-safe.
func (h *HTTP) pace() {
	h.Limiter.Wait()
}

// fetch performs a GET and returns the raw body bytes. On HTTP 429 (and 503
// with Retry-After), fetch parses Retry-After and retries once after waiting
// the indicated interval (capped at 60s). A second 429 surfaces as a typed
// *RateLimitError so callers (sync, search, watch) can stop a batch loop or
// emit a typed sync_warning event rather than fail opaquely.
func (h *HTTP) fetch(ctx context.Context, path string, q url.Values) ([]byte, error) {
	const maxRetryAfter = 60 * time.Second
	const defaultRetryAfter = 2 * time.Second

	attempt := func() (*http.Response, []byte, error) {
		h.pace()
		full := h.BaseURL + path
		if len(q) > 0 {
			// url.Values.Encode escapes the PSI URIs with %23 for '#' which
			// the data.rechtspraak.nl API accepts. Do not pre-escape values;
			// let net/url handle it consistently.
			full += "?" + q.Encode()
		}
		req, err := http.NewRequestWithContext(ctx, "GET", full, nil)
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("User-Agent", h.UA)
		req.Header.Set("Accept", "application/atom+xml, application/xml;q=0.9, */*;q=0.5")
		resp, err := h.Client.Do(req)
		if err != nil {
			return nil, nil, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return resp, nil, err
		}
		return resp, body, nil
	}

	resp, body, err := attempt()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode == 503 {
		h.Limiter.OnRateLimit()
		wait := parseRetryAfter(resp.Header.Get("Retry-After"), defaultRetryAfter)
		if wait > maxRetryAfter {
			wait = maxRetryAfter
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
		resp2, body2, err2 := attempt()
		if err2 != nil {
			return nil, err2
		}
		if resp2.StatusCode == 429 || resp2.StatusCode == 503 {
			h.Limiter.OnRateLimit()
			retry2 := parseRetryAfter(resp2.Header.Get("Retry-After"), defaultRetryAfter)
			if retry2 > maxRetryAfter {
				retry2 = maxRetryAfter
			}
			return nil, &RateLimitError{Path: path, StatusCode: resp2.StatusCode, RetryAfter: retry2, Body: string(body2)}
		}
		resp, body = resp2, body2
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("rechtspraak GET %s: HTTP %d: %s", path, resp.StatusCode, truncate(string(body), 200))
	}
	h.Limiter.OnSuccess()
	return body, nil
}

// parseRetryAfter parses an HTTP Retry-After header (delta-seconds or HTTP
// date) and returns the wait duration. Falls back to def on parse error.
func parseRetryAfter(h string, def time.Duration) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return def
	}
	if n, err := strconv.Atoi(h); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return def
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// SearchParams collects the filters supported server-side by /uitspraken/zoeken.
// Repeated fields (Dates, Modified, Subjects, Creators, Replaces) become
// repeated query params (per IVO 1.15: same-type = OR, cross-type = AND).
type SearchParams struct {
	Dates    []string // YYYY-MM-DD - 1 value = exact, 2 values = range
	Modified []string // ISO8601 - 1 value = since, 2 values = range
	Subjects []string // PSI URIs of rechtsgebieden
	Creators []string // PSI URIs of instanties
	Type     string   // Uitspraak | Conclusie
	Replaces []string // LJN codes
	Return   string   // DOC = only ECLIs with bodies
	Sort     string   // ASC | DESC
	Max      int
	From     int
}

// ToValues serialises SearchParams to a url.Values (repeated keys).
func (p SearchParams) ToValues() url.Values {
	v := url.Values{}
	for _, d := range p.Dates {
		if d = strings.TrimSpace(d); d != "" {
			v.Add("date", d)
		}
	}
	for _, m := range p.Modified {
		if m = strings.TrimSpace(m); m != "" {
			v.Add("modified", m)
		}
	}
	for _, s := range p.Subjects {
		if s = strings.TrimSpace(s); s != "" {
			v.Add("subject", s)
		}
	}
	for _, c := range p.Creators {
		if c = strings.TrimSpace(c); c != "" {
			v.Add("creator", c)
		}
	}
	for _, r := range p.Replaces {
		if r = strings.TrimSpace(r); r != "" {
			v.Add("replaces", r)
		}
	}
	if p.Type != "" {
		v.Set("type", p.Type)
	}
	if p.Return != "" {
		v.Set("return", p.Return)
	}
	if p.Sort != "" {
		v.Set("sort", p.Sort)
	}
	if p.Max > 0 {
		v.Set("max", fmt.Sprintf("%d", p.Max))
	}
	if p.From > 0 {
		v.Set("from", fmt.Sprintf("%d", p.From))
	}
	return v
}

// Search calls /uitspraken/zoeken and returns entries plus total count.
func (h *HTTP) Search(ctx context.Context, p SearchParams) ([]SearchEntry, int, error) {
	body, err := h.fetch(ctx, "/uitspraken/zoeken", p.ToValues())
	if err != nil {
		return nil, 0, err
	}
	return ParseSearchResponse(bytes.NewReader(body))
}

// Get fetches a single decision's content.
func (h *HTTP) Get(ctx context.Context, ecli string, metaOnly bool) (*Decision, error) {
	q := url.Values{}
	q.Set("id", ecli)
	if metaOnly {
		q.Set("return", "META")
	}
	body, err := h.fetch(ctx, "/uitspraken/content", q)
	if err != nil {
		return nil, err
	}
	d, err := ParseDecision(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if d.ECLI == "" {
		d.ECLI = ecli
	}
	return d, nil
}

// Courts fetches the Instanties vocab.
func (h *HTTP) Courts(ctx context.Context) ([]Court, error) {
	body, err := h.fetch(ctx, "/Waardelijst/Instanties", nil)
	if err != nil {
		return nil, err
	}
	return ParseCourts(bytes.NewReader(body))
}

// ForeignCourts fetches the InstantiesBuitenlands vocab.
func (h *HTTP) ForeignCourts(ctx context.Context) ([]Court, error) {
	body, err := h.fetch(ctx, "/Waardelijst/InstantiesBuitenlands", nil)
	if err != nil {
		return nil, err
	}
	return ParseCourts(bytes.NewReader(body))
}

// Subjects fetches the Rechtsgebieden vocab.
func (h *HTTP) Subjects(ctx context.Context) ([]Subject, error) {
	body, err := h.fetch(ctx, "/Waardelijst/Rechtsgebieden", nil)
	if err != nil {
		return nil, err
	}
	return ParseSubjects(bytes.NewReader(body))
}

// Procedures fetches the Proceduresoorten vocab.
func (h *HTTP) Procedures(ctx context.Context) ([]Procedure, error) {
	body, err := h.fetch(ctx, "/Waardelijst/Proceduresoorten", nil)
	if err != nil {
		return nil, err
	}
	return ParseProcedures(bytes.NewReader(body))
}

// Relations fetches the FormeleRelaties vocab.
func (h *HTTP) Relations(ctx context.Context) ([]RelationDef, error) {
	body, err := h.fetch(ctx, "/Waardelijst/FormeleRelaties", nil)
	if err != nil {
		return nil, err
	}
	return ParseRelations(bytes.NewReader(body))
}

// ForeignDecisions fetches the NietNederlandseUitspraken vocab (~3.5MB).
func (h *HTTP) ForeignDecisions(ctx context.Context) ([]ForeignDecision, error) {
	body, err := h.fetch(ctx, "/Waardelijst/NietNederlandseUitspraken", nil)
	if err != nil {
		return nil, err
	}
	return ParseForeignDecisions(bytes.NewReader(body))
}
