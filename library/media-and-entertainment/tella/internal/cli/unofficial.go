// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH(library): unofficial-API support for the Cut-panel features Tella
// ships in the web UI but doesn't expose on api.tella.com. Live probes
// against www.tella.tv and prod-stream.tella.tv on 2026-05-16 returned
// 401 `not_authenticated` to the public-API Bearer token; both hosts
// require a session cookie issued to a logged-in browser. This file is a
// small helper that the find-mistakes command path uses to call those
// hosts with the user-supplied TELLA_SESSION_COOKIE. The helper is
// deliberately separate from internal/client/client.go — that client is
// generated and tied to the public-API Bearer scheme; this surface is
// off-spec and may break on Tella deploys at any time. Cataloged in
// .printing-press-patches.json#add-cut-panel-parity.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// unofficialHosts records the two non-public hosts the find-mistakes flow
// touches. Documented here so a future reader (or a regen) can find every
// unofficial dependency in one place rather than chasing literal strings.
const (
	unofficialAIHost       = "https://prod-stream.tella.tv"
	unofficialFrontendHost = "https://www.tella.tv"
)

// unofficialClient is a thin HTTP wrapper for the two undocumented hosts.
// It deliberately doesn't share state with internal/client.Client — that
// client is generated, Bearer-auth, and rate-limited around api.tella.com.
// The unofficial hosts are off-spec, may rate-limit differently, and use
// cookie auth.
type unofficialClient struct {
	http      *http.Client
	cookie    string
	aiBaseURL string
}

// newUnofficialClient builds a session-cookie-authenticated client. It
// returns an error if the cookie isn't set; callers are expected to gate
// on `--unofficial` flag presence before reaching here so the error
// message can carry that context.
func newUnofficialClient(sessionCookie string, timeout time.Duration) (*unofficialClient, error) {
	if sessionCookie == "" {
		return nil, fmt.Errorf("unofficial API needs TELLA_SESSION_COOKIE (or session_cookie in config). " +
			"Acquire from a logged-in browser: DevTools → Application → Cookies → tella.tv → copy the full " +
			"`Cookie:` header value. This auth is fragile (session expires, may break on Tella deploys); " +
			"the public-API Bearer token does NOT work against the unofficial AI service")
	}
	const minSSETimeout = 60 * time.Second // analyze-scene SSE streams can run for tens of seconds
	if timeout < minSSETimeout {
		timeout = minSSETimeout
	}
	return &unofficialClient{
		http:      &http.Client{Timeout: timeout},
		cookie:    sessionCookie,
		aiBaseURL: unofficialAIHost,
	}, nil
}

// setStandardHeaders applies every header the HAR shows the web UI sending
// to either unofficial host: cookie auth, JSON content type, Origin /
// Referer (some endpoints CSRF-check), and a user-agent that identifies
// the CLI so Tella's logs can see who's calling.
func (u *unofficialClient) setStandardHeaders(req *http.Request, contentType string) {
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Cookie", u.cookie)
	req.Header.Set("Origin", unofficialFrontendHost)
	req.Header.Set("Referer", unofficialFrontendHost+"/")
	req.Header.Set("User-Agent", "tella-pp-cli/unofficial")
}

// postSSE makes a POST and returns the raw response body for streaming
// parsing. Caller must read line-by-line and Close the body. Returns
// status code separately so callers can short-circuit on non-2xx without
// trying to parse SSE from an HTML error page.
func (u *unofficialClient) postSSE(url string, body any) (io.ReadCloser, int, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, 0, fmt.Errorf("marshaling unofficial POST body: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return nil, 0, err
	}
	u.setStandardHeaders(req, "application/json")
	// SSE responses use text/event-stream; advertising it gets the right
	// content type back.
	req.Header.Set("Accept", "text/event-stream, application/json")
	resp, err := u.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		drained, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, resp.StatusCode, fmt.Errorf("unofficial API returned 401 — session cookie expired or invalid. Refresh by opening tella.tv in a browser and copying a new Cookie value into TELLA_SESSION_COOKIE. Body: %s", truncate(string(drained), 200))
		}
		return nil, resp.StatusCode, fmt.Errorf("unofficial API returned HTTP %d. Body: %s", resp.StatusCode, truncate(string(drained), 200))
	}
	return resp.Body, resp.StatusCode, nil
}
