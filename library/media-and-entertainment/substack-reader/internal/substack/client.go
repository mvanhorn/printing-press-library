// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

// Package substack is the hand-built, per-publication fetch layer for the
// Substack Reader CLI. Substack is per-publication (every author is their own
// host), so this layer takes a publication host at call time rather than a
// fixed base_url — it sits alongside the generated chassis, mirroring how
// Medium Reader's keyless GraphQL fetch layer sat on its generated chassis.
//
// Tier 0 (anonymous, keyless) is the default: free posts (audience "everyone")
// return a full body_html with no setup. Tier 1 attaches the user's own
// substack.sid session cookie (NewAuthedClient) to unlock paid posts they
// already subscribe to — see ../../../discovery/auth-model.md §"CRACK RESULTS".
package substack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/cliutil"
)

const defaultUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"

// maxResponseBytes caps how much of a success (2xx) response body we read into
// memory. Substack's largest archive/post JSON is a few MB; 50 MiB leaves ample
// headroom while stopping an oversized, malformed, or redirected body from
// ballooning the heap. The error paths already bound their reads with
// io.LimitReader — this bounds the success path too.
const maxResponseBytes = 50 << 20 // 50 MiB

// Client fetches from a single Substack publication host.
type Client struct {
	http     *http.Client
	limiter  *cliutil.AdaptiveLimiter
	ua       string
	session  Session
	maxBytes int64 // cap on a success-path body read; defaults to maxResponseBytes
}

// NewClient returns a Tier-0 (anonymous) client. The redirect policy follows
// cross-host 301s, which transparently handles the <pub>.substack.com ->
// custom-domain redirect for anonymous reads.
func NewClient() *Client {
	return newClient(Session{})
}

// NewAuthedClient returns a Tier-1 client that attaches the given session's
// substack.sid cookie to every request. A zero Session yields the same
// behaviour as NewClient (anonymous). See auth-model.md for why substack.sid
// alone suffices.
func NewAuthedClient(sess Session) *Client {
	return newClient(sess)
}

func newClient(sess Session) *Client {
	c := &Client{
		limiter:  cliutil.NewAdaptiveLimiter(2.0),
		ua:       defaultUA,
		session:  sess,
		maxBytes: maxResponseBytes,
	}
	c.http = &http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: c.checkRedirect,
	}
	return c
}

// Authed reports whether this client carries Tier-1 session material.
func (c *Client) Authed() bool { return !c.session.IsZero() }

// checkRedirect re-attaches the session cookie on redirects that STAY within
// substack.com. This is the deliberately-narrow half of the Medium
// ForwardHeadersOnRedirect fix: Go's stdlib strips the Cookie header on any
// cross-host redirect (since Go 1.8), so a <sub>.substack.com -> another
// substack.com hop would silently downgrade an authed request to anonymous.
//
// We do NOT blindly re-attach across ALL hosts, because the <sub>.substack.com
// -> custom-domain 301 targets an arbitrary registrable domain and forwarding
// substack.sid to a host we did not resolve would leak the session. Instead the
// Tier-1 read path RESOLVES the canonical host first (subdomain or the post's
// own custom_domain) and fetches THAT host directly with the cookie — no blind
// redirect forwarding needed (auth-model.md: "resolve the 301 Location and
// fetch it directly with the cookie"). This backstop only covers same-provider
// substack.com hops, which are always Substack-controlled.
func (c *Client) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	if c.session.IsZero() {
		return nil
	}
	if isSubstackComHost(req.URL.Host) {
		if h := c.session.CookieHeader(); h != "" {
			// #nosec G119 -- the session cookie is re-attached ONLY on
			// first-party substack.com hosts (isSubstackComHost allowlist);
			// this is the intended ForwardHeadersOnRedirect fix, narrowed so
			// the cookie is never forwarded to an arbitrary custom domain.
			req.Header.Set("Cookie", h)
		}
	}
	return nil
}

// isSubstackComHost reports whether host is substack.com or one of its
// subdomains (the hosts where substack.sid is always first-party).
func isSubstackComHost(host string) bool {
	h := strings.ToLower(host)
	if i := strings.IndexByte(h, ':'); i >= 0 {
		h = h[:i] // strip any :port
	}
	return h == "substack.com" || strings.HasSuffix(h, ".substack.com")
}

// ResolveHost turns a user publication argument into an https base URL.
// Accepts a handle ("astralcodexten" -> astralcodexten.substack.com), a bare
// host ("blog.bytebytego.com"), or a full URL.
func ResolveHost(input string) string {
	s := strings.TrimSpace(input)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimSuffix(s, "/")
	if s == "" {
		return ""
	}
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	if !strings.Contains(s, ".") {
		s += ".substack.com"
	}
	return "https://" + s
}

func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	c.limiter.Wait()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "application/json")
	// Attach the Tier-1 session cookie on the initial request — but ONLY when the
	// destination is first-party substack.com (isSubstackComHost). substack.sid
	// is a .substack.com cookie; sending it to any other registrable domain
	// (e.g. a publication's custom domain) would leak the user's session. The
	// authed path only ever fetches substack.com's by-id endpoint, so this gate
	// never blocks a legitimate authed request; it makes the "never emit the
	// cookie off substack.com" invariant enforced by the client itself rather
	// than by call-site convention (custom domains are never fetched authed — the
	// by-id path on substack.com is the universal Tier-1 route). checkRedirect
	// applies the same gate on cross-host redirect hops.
	if h := c.session.CookieHeader(); h != "" && isSubstackComHost(req.URL.Host) {
		req.Header.Set("Cookie", h)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, &cliutil.RateLimitError{URL: rawURL, Body: strings.TrimSpace(string(body))}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GET %s: HTTP %d: %s", rawURL, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	c.limiter.OnSuccess()
	// Bound the success-path read: read at most maxBytes+1 and treat a body that
	// reaches the cap as an error rather than silently truncating it (a truncated
	// JSON would fail to parse downstream with a far less obvious message). This
	// keeps an oversized or maliciously-redirected body from ballooning the heap.
	data, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > c.maxBytes {
		return nil, fmt.Errorf("GET %s: response body exceeds %d bytes", rawURL, c.maxBytes)
	}
	return data, nil
}

// PostSummary is the minimal shape the CLI needs from an archive item; the full
// item JSON is stored verbatim for search/SQL.
type PostSummary struct {
	ID       json.Number `json:"id"`
	Slug     string      `json:"slug"`
	Title    string      `json:"title"`
	Audience string      `json:"audience"`
	PostDate string      `json:"post_date"`
}

// FetchArchivePage returns one page of archive post-summary objects (newest
// first for sort="new"). The archive list carries metadata + truncated preview
// but NOT the full body; use FetchPost for a post's body_html.
func (c *Client) FetchArchivePage(ctx context.Context, base, sort string, limit, offset int) ([]json.RawMessage, error) {
	q := url.Values{}
	q.Set("sort", sort)
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	data, err := c.get(ctx, base+"/api/v1/archive?"+q.Encode())
	if err != nil {
		return nil, err
	}
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("parsing archive JSON from %s: %w", base, err)
	}
	return items, nil
}

// FetchPost returns a single post object by slug (includes body_html for free
// posts; empty body for paid posts when anonymous).
func (c *Client) FetchPost(ctx context.Context, base, slug string) (json.RawMessage, error) {
	data, err := c.get(ctx, base+"/api/v1/posts/"+url.PathEscape(slug))
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

// FetchPostByID resolves a reader-app numeric post id via
// substack.com/api/v1/posts/by-id/<id>, which returns a {post, publication}
// envelope carrying publication.subdomain / publication.custom_domain and
// post.slug / post.audience — the fields needed to canonicalise a reader-app
// URL (shape D) to its publication host. Returns the raw envelope.
func (c *Client) FetchPostByID(ctx context.Context, id string) (json.RawMessage, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("empty post id")
	}
	data, err := c.get(ctx, "https://substack.com/api/v1/posts/by-id/"+url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

// Summarize parses the minimal fields from a raw archive/post item.
func Summarize(raw json.RawMessage) (PostSummary, error) {
	var p PostSummary
	err := json.Unmarshal(raw, &p)
	return p, err
}
