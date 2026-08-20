package client

import (
	"crypto/hmac"
	"crypto/sha1" // #nosec G505 -- DNS Made Easy API mandates HMAC-SHA1 request signing (x-dnsme-hmac); the algorithm is fixed by the provider, not a security choice.
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/dnsmadeeasy/internal/config"
)

// DNS Made Easy authenticates every request with an HMAC-SHA1 signature rather
// than a static header. Each request must carry three headers:
//
//	x-dnsme-apiKey       the account API key
//	x-dnsme-requestDate  an RFC1123 timestamp in GMT
//	x-dnsme-hmac         hex(HMAC-SHA1(secretKey, requestDate))
//
// The Printing Press generator has no signing auth mode (only static api_key /
// bearer / oauth), so this signing http.RoundTripper is wired into client.New
// (see the one-line hook there). Because it lives at the transport layer, it
// signs EVERY request the client makes — generated endpoint commands and
// hand-written novel commands (where-used, drift, bulk-apply, acme-purge)
// alike — with no per-call bookkeeping.
//
// Correctness note: the request date is computed ONCE per RoundTrip and used
// for both the x-dnsme-requestDate header and the HMAC input, so the server's
// recomputation over the header value always matches. (The official Go SDK
// computes the two independently, which is race-prone across a second
// boundary; this implementation is deliberately stricter.)
type dnsmeSigningTransport struct {
	apiKey string
	secret string
	// apiHost is the host of the configured base URL. Requests to any other
	// host (e.g. a redirect to a login or attacker-controlled page) are NOT
	// signed, so the API key and a replayable HMAC never leak cross-host.
	apiHost string
	base    http.RoundTripper
	// now is overridable in tests; nil means time.Now.
	now func() time.Time
}

func newSigningTransport(cfg *config.Config, base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	if cfg == nil {
		return base
	}
	return &dnsmeSigningTransport{
		apiKey:  cfg.DnsmadeeasyApiKey,
		secret:  cfg.DnsmadeeasyApiSecret,
		apiHost: hostFromURL(cfg.BaseURL),
		base:    base,
	}
}

// hostFromURL extracts the lowercased host[:port] from a base URL. Returns ""
// when the URL is empty or unparseable, in which case signing is disabled
// (fail closed — never sign an unknown host).
func hostFromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Host)
}

// signDNSMadeEasy returns the request date and hex HMAC-SHA1 signature for a
// given secret and moment. Exported-to-package so tests can assert the
// contract directly.
func signDNSMadeEasy(secret string, t time.Time) (date, signature string) {
	date = t.UTC().Format(http.TimeFormat) // "Mon, 02 Jan 2006 15:04:05 GMT"
	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write([]byte(date))
	return date, hex.EncodeToString(mac.Sum(nil))
}

func (t *dnsmeSigningTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Without both credentials there is nothing to sign; pass through so the
	// server returns its normal 401 and doctor/verify can report it honestly.
	if t.apiKey == "" || t.secret == "" {
		return t.base.RoundTrip(req)
	}
	// Only sign requests to the configured API host. A cross-host redirect
	// (open redirect, partner handoff, or attacker-controlled target) must
	// never receive the API key or an HMAC — the HMAC is computed over the
	// date alone and is therefore replayable for the whole skew window.
	if t.apiHost == "" || !strings.EqualFold(req.URL.Host, t.apiHost) {
		return t.base.RoundTrip(req)
	}
	nowFn := t.now
	if nowFn == nil {
		nowFn = time.Now
	}
	date, sig := signDNSMadeEasy(t.secret, nowFn())
	// Clone so retries (and the shared request the caller may reuse) never see
	// a mutated header set with a stale date.
	signed := req.Clone(req.Context())
	signed.Header.Set("x-dnsme-apiKey", t.apiKey)
	signed.Header.Set("x-dnsme-requestDate", date)
	signed.Header.Set("x-dnsme-hmac", sig)
	return t.base.RoundTrip(signed)
}
