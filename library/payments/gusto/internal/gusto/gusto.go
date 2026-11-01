// Copyright 2026 david and contributors. Licensed under Apache-2.0. See LICENSE.

// Package gusto talks to the GraphQL API behind app.gusto.com.
//
// Every request rides the generated client (Chrome-compatible transport,
// cookie jar seeded from the press-auth session, rate limiting, dry-run and
// the verify-mode mutation gate). This package adds the two headers the web
// API requires on top of the session cookie: x-csrf-token and x-role-id.
// Both are read once per process from the HTML of an authenticated
// app.gusto.com page.
//
// Nothing in this package submits a payroll. The captured mutations only
// save draft inputs and calculate; submit is deliberately absent.
package gusto

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/gusto/internal/client"
	"github.com/mvanhorn/printing-press-library/library/payments/gusto/internal/cliutil"
)

//go:embed queries/*.graphql
var queryFS embed.FS

// BootstrapURL is the authenticated app page whose HTML carries the CSRF
// meta tag and the bootstrapped role id.
const BootstrapURL = "https://app.gusto.com/pay"

// ErrNotSignedIn means the session cookie did not authenticate: Gusto served
// its login page instead of the app.
var ErrNotSignedIn = errors.New("gusto session is not signed in; run: gusto-pp-cli auth login --chrome")

var (
	csrfRe = regexp.MustCompile(`<meta name="csrf-token" content="([^"]+)"`)
	roleRe = regexp.MustCompile(`"role_id":(\d+)`)
)

// SessionHeaders are the per-request headers the web GraphQL API requires.
type SessionHeaders struct {
	CSRFToken string
	RoleID    string
}

// ParseSessionHeaders extracts the CSRF token and role id from app HTML.
func ParseSessionHeaders(html []byte) (SessionHeaders, error) {
	csrf := csrfRe.FindSubmatch(html)
	role := roleRe.FindSubmatch(html)
	if csrf == nil || role == nil {
		return SessionHeaders{}, ErrNotSignedIn
	}
	return SessionHeaders{CSRFToken: string(csrf[1]), RoleID: string(role[1])}, nil
}

// API is a GraphQL caller bound to one generated client.
type API struct {
	c         *client.Client
	limiter   *cliutil.AdaptiveLimiter
	mu        sync.Mutex
	headers   *SessionHeaders
	sessionID string
}

// GraphQLURL is the host that serves Gusto's web GraphQL API. The session
// cookie is scoped to .app.gusto.com, so it rides requests to this subdomain.
const GraphQLURL = "https://graphql.app.gusto.com"

// New wraps a generated client and points it at the GraphQL host. The
// generated base URL (app.gusto.com) stays the session-validation origin.
func New(c *client.Client) *API {
	if c != nil {
		c.BaseURL = GraphQLURL
	}
	return &API{c: c, limiter: cliutil.NewAdaptiveLimiter(2), sessionID: NewUUID()}
}

// Headers returns cached session headers, fetching them on first use.
func (a *API) Headers(ctx context.Context) (SessionHeaders, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.headers != nil {
		return *a.headers, nil
	}
	if a.c == nil || a.c.HTTPClient == nil {
		return SessionHeaders{}, errors.New("gusto: HTTP client is not configured")
	}
	// The bootstrap GET uses the raw HTTP client, which skips the generated
	// client's dry-run gate, so refuse here instead of fetching.
	if a.c.DryRun {
		return SessionHeaders{}, errors.New("gusto: session headers are not fetched under --dry-run")
	}
	if err := a.limiter.Wait(ctx); err != nil {
		return SessionHeaders{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, BootstrapURL, nil)
	if err != nil {
		return SessionHeaders{}, err
	}
	req.Header.Set("Accept", "text/html")
	resp, err := a.c.HTTPClient.Do(req)
	if err != nil {
		return SessionHeaders{}, fmt.Errorf("loading %s: %w", BootstrapURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		a.limiter.OnRateLimit()
		return SessionHeaders{}, &cliutil.RateLimitError{URL: BootstrapURL, RetryAfter: 30 * time.Second}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return SessionHeaders{}, fmt.Errorf("reading %s: %w", BootstrapURL, err)
	}
	if resp.StatusCode >= 400 {
		return SessionHeaders{}, fmt.Errorf("loading %s: HTTP %d", BootstrapURL, resp.StatusCode)
	}
	a.limiter.OnSuccess()
	h, err := ParseSessionHeaders(body)
	if err != nil {
		return SessionHeaders{}, err
	}
	a.headers = &h
	return h, nil
}

// Query runs a read-only GraphQL operation.
func (a *API) Query(ctx context.Context, op string, vars map[string]any) (map[string]any, error) {
	return a.call(ctx, op, vars, false)
}

// Mutate runs a state-changing GraphQL operation. Under the verify harness
// without LIVE_HTTP the generated client returns a synthetic envelope and
// never dials.
func (a *API) Mutate(ctx context.Context, op string, vars map[string]any) (map[string]any, error) {
	return a.call(ctx, op, vars, true)
}

func (a *API) call(ctx context.Context, op string, vars map[string]any, mutation bool) (map[string]any, error) {
	doc, err := queryFS.ReadFile("queries/" + op + ".graphql")
	if err != nil {
		return nil, fmt.Errorf("unknown gusto operation %q", op)
	}
	if vars == nil {
		vars = map[string]any{}
	}
	h, err := a.Headers(ctx)
	if err != nil {
		return nil, err
	}
	body := map[string]any{"operationName": op, "query": string(doc), "variables": vars}
	params := map[string]string{"operationName": op}
	headers := map[string]string{"x-csrf-token": h.CSRFToken, "x-role-id": h.RoleID, "accept": "*/*"}
	var raw json.RawMessage
	if mutation {
		raw, _, err = a.c.PostWithParamsAndHeaders(ctx, "/", params, body, headers)
	} else {
		raw, _, err = a.c.PostQueryWithParamsAndHeaders(ctx, "/", params, body, headers)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("%s: decoding response: %w", op, err)
	}
	if errs, ok := out["errors"].([]any); ok && len(errs) > 0 {
		msgs := make([]string, 0, len(errs))
		for _, e := range errs {
			if m, ok := e.(map[string]any); ok {
				msgs = append(msgs, Str(m, "message"))
			}
		}
		return out, fmt.Errorf("%s: %s", op, strings.Join(msgs, "; "))
	}
	return out, nil
}

// NewUUID returns a random RFC 4122 v4 UUID.
func NewUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("gusto: crypto/rand failed: %v", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// --- generic JSON path helpers -------------------------------------------

// Get walks a key path through nested maps.
func Get(v any, keys ...string) any {
	cur := v
	for _, part := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[part]
	}
	return cur
}

// List returns the value at the key path as a slice of maps.
func List(v any, keys ...string) []map[string]any {
	raw, _ := Get(v, keys...).([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// Str returns the value at the key path as a string ("" when absent).
func Str(v any, keys ...string) string {
	switch x := Get(v, keys...).(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		return ""
	}
}

// Num returns the value at the key path as a float. Gusto returns money as strings.
func Num(v any, keys ...string) float64 {
	switch x := Get(v, keys...).(type) {
	case float64:
		return x
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	default:
		return 0
	}
}
