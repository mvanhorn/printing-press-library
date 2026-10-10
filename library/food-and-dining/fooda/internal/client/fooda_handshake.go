package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
)

var (
	clientTokenRegexp  = regexp.MustCompile(`<meta\s+name="api-client-token"\s+content="([^"]+)"`)
	sessionTokenRegexp = regexp.MustCompile(`<meta\s+name="api-session-token"\s+content="([^"]+)"`)
)

type handshakeCache struct {
	mu           sync.Mutex
	clientToken  string
	sessionToken string
}

var (
	foodaHandshakeMu    sync.Mutex
	foodaHandshakeByCli = map[*Client]*handshakeCache{}
)

func (c *Client) handshakeCache() *handshakeCache {
	foodaHandshakeMu.Lock()
	defer foodaHandshakeMu.Unlock()
	hc := foodaHandshakeByCli[c]
	if hc == nil {
		hc = &handshakeCache{}
		foodaHandshakeByCli[c] = hc
	}
	return hc
}

func (c *Client) clearHandshakeCache() {
	hc := c.handshakeCache()
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.clientToken = ""
	hc.sessionToken = ""
}

type ErrAuthRequired struct {
	Message string
}

func (e *ErrAuthRequired) Error() string {
	return e.Message
}

func (c *Client) ensureHandshakeTokens(ctx context.Context) (string, string, error) {
	hc := c.handshakeCache()
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if hc.clientToken != "" && hc.sessionToken != "" {
		return hc.clientToken, hc.sessionToken, nil
	}

	// Fetch /my with the cookie to extract client-token and session-token
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/my", nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create handshake request: %w", err)
	}

	// Set realistic Chrome User-Agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to execute handshake: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == 403 {
		return "", "", &ErrAuthRequired{Message: "Please log in to app.fooda.com in Chrome and re-run `auth login --chrome`"}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read handshake response: %w", err)
	}

	bodyStr := string(body)

	// Check if we were redirected to login page or body has login-like content
	if resp.Request != nil && (strings.Contains(resp.Request.URL.Path, "/login") || strings.Contains(resp.Request.URL.Path, "/users/sign_in")) {
		return "", "", &ErrAuthRequired{Message: "Please log in to app.fooda.com in Chrome and re-run `auth login --chrome`"}
	}

	clientTokenMatch := clientTokenRegexp.FindStringSubmatch(bodyStr)
	sessionTokenMatch := sessionTokenRegexp.FindStringSubmatch(bodyStr)

	if len(clientTokenMatch) < 2 || len(sessionTokenMatch) < 2 {
		return "", "", &ErrAuthRequired{Message: "Please log in to app.fooda.com in Chrome and re-run `auth login --chrome`"}
	}

	hc.clientToken = clientTokenMatch[1]
	hc.sessionToken = sessionTokenMatch[1]

	return hc.clientToken, hc.sessionToken, nil
}

func (c *Client) EnsureHandshakeTokens(ctx context.Context) (string, string, error) {
	return c.ensureHandshakeTokens(ctx)
}

func (c *Client) DiscoverAccountAndBuilding(ctx context.Context) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/my", nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create discovery request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to execute discovery: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == 403 {
		return "", "", &ErrAuthRequired{Message: "Please log in to app.fooda.com in Chrome and re-run `auth login --chrome`"}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read discovery response: %w", err)
	}

	bodyStr := string(body)

	if resp.Request != nil && (strings.Contains(resp.Request.URL.Path, "/login") || strings.Contains(resp.Request.URL.Path, "/users/sign_in")) {
		return "", "", &ErrAuthRequired{Message: "Please log in to app.fooda.com in Chrome and re-run `auth login --chrome`"}
	}

	return ParseMyPage(bodyStr)
}

// foodaBrowserUA must stay consistent across the handshake and HTML fetches;
// Cloudflare binds cf_clearance to the user agent that earned it.
const foodaBrowserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Safari/537.36"

// GetHTML fetches a logged-in HTML page as a browser would: no JSON Accept,
// Origin, or Referer headers, which the generated JSON path adds and which
// make Cloudflare challenge the request.
func (c *Client) GetHTML(ctx context.Context, path string) ([]byte, error) {
	if c.DryRun {
		fmt.Fprintf(os.Stderr, "GET %s\n\n(dry run - no request sent)\n", c.BaseURL+path)
		return []byte{}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("building request for %s: %w", path, err)
	}
	req.Header.Set("User-Agent", foodaBrowserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	bodyStr := string(body)
	if strings.Contains(bodyStr, "Just a moment") || resp.Header.Get("cf-mitigated") != "" || resp.Header.Get("Cf-Mitigated") != "" {
		return nil, fmt.Errorf("Cloudflare challenged this page for non-browser clients")
	}
	if resp.StatusCode == http.StatusForbidden || (resp.Request != nil && strings.Contains(resp.Request.URL.Path, "/login")) {
		return nil, &ErrAuthRequired{Message: "Please log in to app.fooda.com in Chrome and re-run `auth login --chrome`"}
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s returned HTTP %d", path, resp.StatusCode)
	}
	return body, nil
}
