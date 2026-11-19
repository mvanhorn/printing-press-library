package edeka

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/cliutil"
)

// cdpLimiter paces the DevTools discovery calls (local browser, but the same
// rate-limit discipline as every outbound client).
var cdpLimiter = cliutil.NewAdaptiveLimiter(5)

// cdpGet fetches a DevTools discovery document with pacing and typed 429s.
func cdpGet(ctx context.Context, rawURL string) (*http.Response, error) {
	if err := cdpLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		_ = resp.Body.Close()
		cdpLimiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: rawURL}
	}
	cdpLimiter.OnSuccess()
	return resp, nil
}

// CookiesFromCDP reads all cookies of a running Chromium over the DevTools
// protocol (for example a browser started with --remote-debugging-port=9222)
// and keeps the edeka.shop ones. Only loopback endpoints are accepted, because the
// answer contains live session cookies.
func CookiesFromCDP(ctx context.Context, endpoint string) ([]Cookie, error) {
	u, err := url.Parse(strings.TrimRight(endpoint, "/"))
	if err != nil {
		return nil, err
	}
	host := u.Hostname()
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return nil, fmt.Errorf("refusing non-loopback DevTools endpoint %q", endpoint)
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	resp, err := cdpGet(ctx, u.String()+"/json/version")
	if err != nil {
		return nil, fmt.Errorf("reaching DevTools at %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	var ver struct {
		WS string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ver); err != nil || ver.WS == "" {
		return nil, fmt.Errorf("DevTools /json/version gave no webSocketDebuggerUrl")
	}
	if !loopbackWS(ver.WS) {
		return nil, fmt.Errorf("refusing non-loopback DevTools websocket %q", ver.WS)
	}
	cookies, berr := cdpCookies(ctx, ver.WS, "Storage.getCookies")
	if berr == nil {
		return cookies, nil
	}
	// Some DevTools proxies refuse browser-level calls; ask a page target instead.
	lresp, err := cdpGet(ctx, u.String()+"/json/list")
	if err != nil {
		return nil, berr
	}
	defer lresp.Body.Close()
	var targets []struct {
		Type string `json:"type"`
		WS   string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(lresp.Body).Decode(&targets); err != nil {
		return nil, berr
	}
	for _, t := range targets {
		if t.Type == "page" && t.WS != "" && loopbackWS(t.WS) {
			return cdpCookies(ctx, t.WS, "Network.getAllCookies")
		}
	}
	return nil, fmt.Errorf("%v (and no page target to fall back to)", berr)
}

// loopbackWS reports whether a DevTools websocket URL points at this machine.
func loopbackWS(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "ws" && u.Scheme != "wss") {
		return false
	}
	h := u.Hostname()
	return h == "127.0.0.1" || h == "localhost" || h == "::1"
}

func cdpCookies(ctx context.Context, ws, method string) ([]Cookie, error) {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, ws, nil)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetReadDeadline(dl)
	}
	if err := conn.WriteJSON(map[string]any{"id": 1, "method": method}); err != nil {
		return nil, err
	}
	for {
		var msg struct {
			ID     int `json:"id"`
			Result struct {
				Cookies []struct {
					Name     string  `json:"name"`
					Value    string  `json:"value"`
					Domain   string  `json:"domain"`
					Path     string  `json:"path"`
					Expires  float64 `json:"expires"`
					HTTPOnly bool    `json:"httpOnly"`
					Secure   bool    `json:"secure"`
				} `json:"cookies"`
			} `json:"result"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := conn.ReadJSON(&msg); err != nil {
			return nil, err
		}
		if msg.ID != 1 {
			continue
		}
		if msg.Error != nil {
			return nil, fmt.Errorf("DevTools %s: %s", method, msg.Error.Message)
		}
		out := make([]Cookie, 0, len(msg.Result.Cookies))
		for _, c := range msg.Result.Cookies {
			out = append(out, Cookie{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path, Expires: c.Expires, HTTPOnly: c.HTTPOnly, Secure: c.Secure})
		}
		return FilterEDEKA(out), nil
	}
}
