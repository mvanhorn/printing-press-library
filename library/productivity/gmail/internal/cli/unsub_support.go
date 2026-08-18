// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written pure machinery for the unsubscribe engine (grill R1-C3,
// R2-C3, R3-C5): List-Unsubscribe header parsing, the one-click
// classification ladder, the registrable-domain (eTLD+1) approximation,
// Authentication-Results and DKIM-Signature checks, and the hardened
// HTTPS POST path (public-unicast SSRF guard, resolved-IP pinned dialing,
// redirects never followed). No cobra wiring and no store access here so
// every rule is table-testable.

package cli

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

// Unsubscribe classification vocabulary. classUnsubOneClick carries the
// "(pending-header-check)" suffix because the DKIM-Signature header is not
// stored offline — `unsub run` performs the final live check.
const (
	classUnsubOneClick   = "one-click-verified(pending-header-check)"
	classUnsubPlainURL   = "plain-url"
	classUnsubMailtoOnly = "mailto-only"
	classUnsubUnusable   = "unusable"
)

// oneClickPostValue is the exact RFC 8058 List-Unsubscribe-Post value (and
// the exact POST body `unsub run` sends).
const oneClickPostValue = "List-Unsubscribe=One-Click"

// unsubPostTimeout bounds one unsubscribe POST end to end.
const unsubPostTimeout = 10 * time.Second

// unsubPostBodyCap caps how much response body is read (then discarded).
const unsubPostBodyCap = 64 << 10

// unsubHeaderInfo is a parsed List-Unsubscribe header value.
type unsubHeaderInfo struct {
	HTTPSURLs []string
	HTTPURLs  []string
	Mailtos   []string
}

// parseListUnsubscribe parses an RFC 2369 List-Unsubscribe value:
// comma-separated angle-bracketed entries (`<https://...>, <mailto:...>`).
// Text outside angle brackets (RFC 2369 comments) is ignored. Entries are
// bucketed by scheme; anything unparseable is dropped.
func parseListUnsubscribe(v string) unsubHeaderInfo {
	info := unsubHeaderInfo{}
	rest := v
	for {
		open := strings.Index(rest, "<")
		if open < 0 {
			break
		}
		close := strings.Index(rest[open:], ">")
		if close < 0 {
			break
		}
		entry := strings.TrimSpace(rest[open+1 : open+close])
		rest = rest[open+close+1:]
		if entry == "" {
			continue
		}
		lower := strings.ToLower(entry)
		switch {
		case strings.HasPrefix(lower, "https://"):
			info.HTTPSURLs = append(info.HTTPSURLs, entry)
		case strings.HasPrefix(lower, "http://"):
			info.HTTPURLs = append(info.HTTPURLs, entry)
		case strings.HasPrefix(lower, "mailto:"):
			info.Mailtos = append(info.Mailtos, entry)
		}
	}
	return info
}

// registrableDomain is the eTLD+1 of a host per the full Public Suffix
// List (golang.org/x/net/publicsuffix, private section included — so
// foo.github.io and bar.github.io are DIFFERENT parties), after IDNA
// normalization (punycode and unicode spellings of one host compare
// equal). IP literals return as-is. A host that IS a public suffix (or
// otherwise has no eTLD+1) returns the normalized host itself, so two
// different registrations under an exotic suffix can never collapse into
// one "aligned" value.
func registrableDomain(host string) string {
	h := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if h == "" {
		return ""
	}
	if ip := net.ParseIP(strings.Trim(h, "[]")); ip != nil {
		return h
	}
	if a, err := idna.Lookup.ToASCII(h); err == nil && a != "" {
		h = a
	}
	if d, err := publicsuffix.EffectiveTLDPlusOne(h); err == nil {
		return d
	}
	return h
}

// emailDomain returns the domain part of an address (” when malformed).
func emailDomain(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return ""
	}
	return strings.ToLower(email[at+1:])
}

// unsubURLHost is the lowercase hostname of an unsubscribe URL ("" when
// unparsable). Ports and userinfo never participate in alignment.
func unsubURLHost(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// unsubHostAligned reports whether the unsubscribe URL's host shares the
// sender's registrable domain. DKIM alignment ties the message to the
// sender; this ties the POST destination to the same party. Unparsable
// URLs and malformed senders are never aligned.
func unsubHostAligned(rawURL, sender string) bool {
	h := unsubURLHost(rawURL)
	sd := emailDomain(sender)
	if h == "" || sd == "" {
		return false
	}
	return registrableDomain(h) == registrableDomain(sd)
}

// dmarcPassRe matches a boundary-checked dmarc=pass token.
var dmarcPassRe = regexp.MustCompile(`(?i)(^|[\s;])dmarc=pass([\s;(]|$)`)

// authResultsIsGmailDMARCPass reports whether a stored
// Authentication-Results header is Gmail's own verdict (authserv-id
// mx.google.com) AND carries dmarc=pass. A forwarded or foreign
// Authentication-Results header (different authserv-id) fails: this binary
// only trusts the verdict Gmail itself stamped on delivery.
func authResultsIsGmailDMARCPass(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	first := v
	if i := strings.Index(first, ";"); i >= 0 {
		first = first[:i]
	}
	fields := strings.Fields(strings.TrimSpace(first))
	if len(fields) == 0 || !strings.EqualFold(fields[0], "mx.google.com") {
		return false
	}
	return dmarcPassRe.MatchString(v)
}

// unsubClassification is one sender's audit verdict.
type unsubClassification struct {
	Class  string // classUnsub* constant
	Reason string // first failed one-click condition ('' when verified)
	URL    string // the https unsubscribe URL ('' when none)
	Mailto string // first mailto entry ('' when none)
}

// classifyUnsubSender applies the one-click ladder to a sender's newest
// unsubscribe-bearing stored message:
//
//	(a) the single stored List-Unsubscribe value must contain exactly one
//	    https URL — internal duplicates are ambiguity and downgrade,
//	(b) List-Unsubscribe-Post must equal "List-Unsubscribe=One-Click"
//	    (case-insensitive, trimmed),
//	(c) an https URL must be present,
//	(d) the stored Authentication-Results must be Gmail's own
//	    (authserv-id mx.google.com) and carry dmarc=pass.
//
// DKIM coverage — condition (e) — cannot be checked offline (the
// DKIM-Signature header is not stored), so the passing class is
// "one-click-verified(pending-header-check)" and `unsub run` performs the
// final live check.
//
// Non-one-click senders: https present -> plain-url (manual click);
// mailto only -> mailto-only (surfaced as a desk list, NEVER acted on);
// neither (e.g. plain-http only) -> unusable.
func classifyUnsubSender(listUnsub, listUnsubPost, authResults string) unsubClassification {
	info := parseListUnsubscribe(listUnsub)
	out := unsubClassification{}
	if len(info.Mailtos) > 0 {
		out.Mailto = info.Mailtos[0]
	}
	if len(info.HTTPSURLs) == 0 {
		if len(info.Mailtos) > 0 {
			out.Class = classUnsubMailtoOnly
			out.Reason = "no https unsubscribe URL; mailto only (never acted on)"
			return out
		}
		out.Class = classUnsubUnusable
		if len(info.HTTPURLs) > 0 {
			out.Reason = "unsubscribe URL is http, not https"
		} else {
			out.Reason = "no https or mailto unsubscribe entry parsed"
		}
		return out
	}
	out.URL = info.HTTPSURLs[0]
	switch {
	case len(info.HTTPSURLs) > 1:
		out.Class = classUnsubPlainURL
		out.Reason = fmt.Sprintf("ambiguous: %d https unsubscribe URLs in one header", len(info.HTTPSURLs))
	case !strings.EqualFold(strings.TrimSpace(listUnsubPost), oneClickPostValue):
		out.Class = classUnsubPlainURL
		if strings.TrimSpace(listUnsubPost) == "" {
			out.Reason = "List-Unsubscribe-Post header missing"
		} else {
			out.Reason = fmt.Sprintf("List-Unsubscribe-Post is %q, not %q", strings.TrimSpace(listUnsubPost), oneClickPostValue)
		}
	case !authResultsIsGmailDMARCPass(authResults):
		out.Class = classUnsubPlainURL
		if strings.TrimSpace(authResults) == "" {
			out.Reason = "no stored Authentication-Results header"
		} else if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(authResults)), "mx.google.com") {
			out.Reason = "Authentication-Results is not Gmail's own (authserv-id != mx.google.com)"
		} else {
			out.Reason = "Authentication-Results lacks dmarc=pass"
		}
	default:
		out.Class = classUnsubOneClick
	}
	return out
}

// parseDKIMTags splits a DKIM-Signature value into its tag map (keys
// lowercased, values trimmed with folding whitespace collapsed).
func parseDKIMTags(sig string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(sig, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		eq := strings.Index(part, "=")
		if eq <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(part[:eq]))
		val := strings.TrimSpace(part[eq+1:])
		val = strings.Join(strings.Fields(val), "") // collapse folding whitespace inside values
		if _, dup := out[key]; !dup {
			out[key] = val
		}
	}
	return out
}

// dkimCoversUnsubHeaders reports whether the signature's h= tag list
// (case-insensitive) covers BOTH list-unsubscribe and
// list-unsubscribe-post — i.e. the one-click headers are inside the signed
// surface, so a forwarder or intermediary could not have injected them.
func dkimCoversUnsubHeaders(sig string) bool {
	h := parseDKIMTags(sig)["h"]
	if h == "" {
		return false
	}
	var hasUnsub, hasPost bool
	for _, name := range strings.Split(h, ":") {
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "list-unsubscribe":
			hasUnsub = true
		case "list-unsubscribe-post":
			hasPost = true
		}
	}
	return hasUnsub && hasPost
}

// dkimDomain returns the signature's d= domain (lowercased, ” if absent).
func dkimDomain(sig string) string {
	return strings.ToLower(parseDKIMTags(sig)["d"])
}

// ---------------------------------------------------------------------------
// Network guards + the hardened POST
// ---------------------------------------------------------------------------

// Test seams. Overridden ONLY by in-process tests (no env var, flag, or
// config reaches them): unsubLookupIP replaces DNS resolution;
// unsubTransportOverride replaces the pinned-dial transport (the http.Client
// wrapper — redirect policy, timeout, cookie-less-ness — and the request
// construction still run exactly as production).
var (
	unsubLookupIP          func(ctx context.Context, host string) ([]net.IP, error)
	unsubTransportOverride http.RoundTripper
)

// ipClass names the non-routable class of an IP, or "" for public unicast.
// The reject set (grill R2-C3, DNS-rebinding/SSRF defense): RFC1918,
// loopback, link-local (v4+v6), unique-local v6, CGNAT 100.64/10,
// unspecified, multicast, broadcast, and the 240/4 reserved block.
func ipClass(ip net.IP) string {
	if ip == nil {
		return "unparseable"
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	switch {
	case ip.IsUnspecified():
		return "unspecified"
	case ip.IsLoopback():
		return "loopback"
	case ip.IsLinkLocalUnicast():
		return "link-local"
	case ip.IsLinkLocalMulticast(), ip.IsMulticast():
		return "multicast"
	case ip.IsPrivate():
		return "private (RFC1918/ULA)"
	}
	if v4 := ip.To4(); v4 != nil {
		switch {
		case v4[0] == 100 && v4[1]&0xc0 == 64: // 100.64.0.0/10
			return "carrier-grade NAT (100.64/10)"
		case v4[0] >= 240 && !v4.Equal(net.IPv4bcast): // 240/4 reserved
			return "reserved (240/4)"
		case v4.Equal(net.IPv4bcast):
			return "broadcast"
		}
	}
	if !ip.IsGlobalUnicast() {
		return "non-global"
	}
	return ""
}

// vetPublicUnicast rejects unless EVERY resolved IP is public unicast — a
// single private A/AAAA record poisons the whole answer (a rebinding server
// can interleave public and private answers).
func vetPublicUnicast(host string, ips []net.IP) error {
	if len(ips) == 0 {
		return fmt.Errorf("host %s resolved to no addresses", host)
	}
	for _, ip := range ips {
		if class := ipClass(ip); class != "" {
			return fmt.Errorf("host %s resolves to %s (%s) — refusing to POST", host, ip, class)
		}
	}
	return nil
}

// unsubResolve resolves a host through the seam (production: the default
// resolver, both address families).
func unsubResolve(ctx context.Context, host string) ([]net.IP, error) {
	if unsubLookupIP != nil {
		return unsubLookupIP(ctx, host)
	}
	return net.DefaultResolver.LookupIP(ctx, "ip", host)
}

// pinnedDialTransport returns a transport whose DialContext ignores the
// address derived from the URL host and dials exactly the already-resolved,
// already-vetted IP (DNS-rebinding defense: the guard's answer IS the
// answer used). TLS SNI and the Host header still come from req.URL, so
// certificate verification runs against the original hostname. Proxies are
// explicitly disabled — a proxy would both defeat the IP pinning and hand
// the request to a third party.
func pinnedDialTransport(resolved net.IP, dial func(ctx context.Context, network, addr string) (net.Conn, error)) *http.Transport {
	if dial == nil {
		d := &net.Dialer{Timeout: unsubPostTimeout}
		dial = d.DialContext
	}
	return &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			_, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("pinned dial: %w", err)
			}
			return dial(ctx, network, net.JoinHostPort(resolved.String(), port))
		},
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        1,
		TLSHandshakeTimeout: unsubPostTimeout,
	}
}

// unsubPostResult reports one hardened POST's outcome.
type unsubPostResult struct {
	SkipReason  string // non-empty: refused before anything left this machine
	Status      int    // >0 when an HTTP response arrived (3xx recorded, never followed)
	RedirectLoc string // Location header on a 3xx (informational)
	Unknown     bool   // network error after the connection was established — the POST may have been received
	Err         error  // detail for Unknown / connect failures
}

// performOneClickPost executes one RFC 8058 one-click POST with the full
// guard stack: https-only URL (no userinfo), resolve + public-unicast vet,
// pinned-IP dial, no cookie jar, no Authorization, body exactly
// "List-Unsubscribe=One-Click" as application/x-www-form-urlencoded, 10s
// deadline, redirects never followed (a 3xx is terminal and recorded),
// response body read capped at 64KB then discarded.
func performOneClickPost(ctx context.Context, rawURL string) unsubPostResult {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return unsubPostResult{SkipReason: "url-unparseable"}
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return unsubPostResult{SkipReason: "url-not-https"}
	}
	if u.User != nil {
		return unsubPostResult{SkipReason: "url-carries-userinfo"}
	}
	host := u.Hostname()
	if host == "" {
		return unsubPostResult{SkipReason: "url-has-no-host"}
	}

	ctx, cancel := context.WithTimeout(ctx, unsubPostTimeout)
	defer cancel()

	ips, err := unsubResolve(ctx, host)
	if err != nil {
		return unsubPostResult{SkipReason: "dns-resolution-failed", Err: err}
	}
	if err := vetPublicUnicast(host, ips); err != nil {
		return unsubPostResult{SkipReason: "ssrf-guard", Err: err}
	}

	connected := false
	var transport http.RoundTripper
	if unsubTransportOverride != nil {
		transport = unsubTransportOverride
		connected = true // seam bypasses the dialer; treat post-hand-off errors as ambiguous
	} else {
		base := &net.Dialer{Timeout: unsubPostTimeout}
		transport = pinnedDialTransport(ips[0], func(dctx context.Context, network, addr string) (net.Conn, error) {
			conn, derr := base.DialContext(dctx, network, addr)
			if derr == nil {
				connected = true
			}
			return conn, derr
		})
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   unsubPostTimeout,
		Jar:       nil, // NO cookie jar
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // a 3xx is terminal, recorded, never followed
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), strings.NewReader(oneClickPostValue))
	if err != nil {
		return unsubPostResult{SkipReason: "request-build-failed", Err: err}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// No Authorization, no cookies — the request carries nothing but the
	// RFC 8058 body.

	resp, err := client.Do(req)
	if err != nil {
		if connected {
			return unsubPostResult{Unknown: true, Err: err}
		}
		return unsubPostResult{SkipReason: "connect-failed", Err: err}
	}
	defer resp.Body.Close()
	_, _ = io.CopyN(io.Discard, resp.Body, unsubPostBodyCap)
	out := unsubPostResult{Status: resp.StatusCode}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		out.RedirectLoc = resp.Header.Get("Location")
	}
	return out
}
