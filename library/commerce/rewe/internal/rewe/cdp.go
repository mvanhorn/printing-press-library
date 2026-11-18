package rewe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// CookiesFromCDP reads all cookies of a running Chromium over the DevTools
// protocol (for example a browser started with --remote-debugging-port=9222)
// and keeps the rewe.de ones. Only loopback endpoints are accepted, because the
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
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String()+"/json/version", nil)
	resp, err := http.DefaultClient.Do(req)
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
	cookies, berr := cdpCookies(ctx, ver.WS, "Storage.getCookies")
	if berr == nil {
		return cookies, nil
	}
	// Some DevTools proxies refuse browser-level calls; ask a page target instead.
	lreq, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String()+"/json/list", nil)
	lresp, err := http.DefaultClient.Do(lreq)
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
		if t.Type == "page" && t.WS != "" {
			return cdpCookies(ctx, t.WS, "Network.getAllCookies")
		}
	}
	return nil, fmt.Errorf("%v (and no page target to fall back to)", berr)
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
		return FilterREWE(out), nil
	}
}
