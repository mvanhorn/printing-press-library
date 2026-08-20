// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// Hand-built Suno dynamic-header injection. The Suno studio API rejects
// requests that don't carry a per-request Device-Id and a freshly-stamped
// Browser-Token (a base64-wrapped {"timestamp":<ms>} that the WAF checks for
// recency). The generated client.do() already sets static Origin/Referer/UA
// headers, but a per-request fresh Browser-Token cannot be a static config
// header, so it must be injected at transport time.
//
// HEADER-INJECTION PATH CHOSEN: a wrapping http.RoundTripper installed onto the
// generated Client's HTTPClient.Transport via InstallSunoTransport. This is the
// most durable hook: every outbound request the Client makes (typed endpoint
// commands, sync, future novel commands) passes through Do() -> Transport, so
// the Device-Id + fresh Browser-Token land on all of them automatically,
// scoped to the studio-api-prod.suno.com host so Clerk and other hosts are
// untouched. InstallSunoTransport is called from rootFlags.newClient (the one
// place every command's client is constructed). Hand-written code may also pass
// SunoDynamicHeaders(deviceID) to *WithHeaders calls; the values are identical.

package client

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

const sunoAPIHost = "studio-api-prod.suno.com"

// browserToken builds the Browser-Token header value: a JSON object
// {"token":"<b64>"} where <b64> is the standard-base64 encoding of
// {"timestamp":<ms_since_epoch>}. The timestamp is computed fresh on each call.
func browserToken() string {
	inner, _ := json.Marshal(map[string]any{
		"timestamp": time.Now().UnixMilli(),
	})
	encoded := base64.StdEncoding.EncodeToString(inner)
	outer, _ := json.Marshal(map[string]string{"token": encoded})
	return string(outer)
}

// SunoDynamicHeaders returns the per-request Suno headers. Browser-Token is
// computed fresh each call. deviceID falls back to the zero UUID when empty.
func SunoDynamicHeaders(deviceID string) map[string]string {
	if strings.TrimSpace(deviceID) == "" {
		deviceID = "00000000-0000-0000-0000-000000000000"
	}
	return map[string]string{
		"Device-Id":     deviceID,
		"Browser-Token": browserToken(),
		"Origin":        "https://suno.com",
		"Referer":       "https://suno.com/",
	}
}

// sunoRoundTripper injects Device-Id and a fresh Browser-Token on every request
// to the Suno studio API host. Requests to any other host pass through
// untouched so the Clerk auth flow (auth.suno.com) is not affected.
type sunoRoundTripper struct {
	base         http.RoundTripper
	deviceID     string
	mu           sync.RWMutex
	cookieHeader string
}

func (t *sunoRoundTripper) setCookieHeader(h string) {
	t.mu.Lock()
	t.cookieHeader = h
	t.mu.Unlock()
}

func (t *sunoRoundTripper) getCookieHeader() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.cookieHeader
}

func (t *sunoRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL != nil && strings.EqualFold(req.URL.Host, sunoAPIHost) {
		for k, v := range SunoDynamicHeaders(t.deviceID) {
			// Don't clobber Origin/Referer if do() already set them — but
			// Device-Id/Browser-Token are ours to set fresh every time.
			if req.Header.Get(k) == "" || k == "Device-Id" || k == "Browser-Token" {
				req.Header.Set(k, v)
			}
		}
		// The browser sends its suno.com session/analytics cookies to the
		// studio API; the WAF cross-checks them against the Bearer JWT and
		// returns 422 token_validation_failed without them.
		if h := t.getCookieHeader(); h != "" && req.Header.Get("Cookie") == "" {
			req.Header.Set("Cookie", h)
		}
	}
	return t.base.RoundTrip(req)
}

// InstallSunoTransport wraps the Client's HTTPClient transport so every
// outbound request to the Suno studio API carries Device-Id + a fresh
// Browser-Token. Idempotent: calling it twice does not double-wrap.
func InstallSunoTransport(c *Client, deviceID, cookieHeader string) {
	if c == nil || c.HTTPClient == nil {
		return
	}
	if _, already := c.HTTPClient.Transport.(*sunoRoundTripper); already {
		return
	}
	base := c.HTTPClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	rt := &sunoRoundTripper{base: base, deviceID: deviceID, cookieHeader: cookieHeader}
	c.HTTPClient.Transport = rt
	c.sunoRT = rt
}
