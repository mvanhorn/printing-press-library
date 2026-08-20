// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/config"
)

type Client struct {
	BaseURL    string
	Config     *config.Config
	HTTPClient *http.Client
	DryRun     bool
	NoCache    bool
	cacheDir   string
	limiter    *cliutil.AdaptiveLimiter
}

// APIError carries HTTP status information for structured exit codes.
type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s %s returned HTTP %d: %s", e.Method, e.Path, e.StatusCode, e.Body)
}

// newHTTPClient returns a standard-library http.Client. Gorgias's REST API
// uses plain HTTPS Basic auth and doesn't require browser-fingerprint
// impersonation; we use the stdlib transport directly for predictable
// behavior, a smaller binary, and zero extra dependencies.
func newHTTPClient(timeout time.Duration, jar http.CookieJar) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Jar:     jar,
	}
}

func New(cfg *config.Config, timeout time.Duration, rateLimit float64) *Client {
	cacheBase := os.Getenv("XDG_CACHE_HOME")
	if cacheBase == "" {
		homeDir, _ := os.UserHomeDir()
		cacheBase = filepath.Join(homeDir, ".cache")
	}
	cacheDir := filepath.Join(cacheBase, "gorgias-pp-cli", "http")
	httpClient := newHTTPClient(timeout, nil)
	return &Client{
		BaseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		Config:     cfg,
		HTTPClient: httpClient,
		cacheDir:   cacheDir,
		limiter:    cliutil.NewAdaptiveLimiter(rateLimit),
	}
}

// RateLimit returns the current effective rate limit in req/s. Returns 0 if disabled.
func (c *Client) RateLimit() float64 {
	return c.limiter.Rate()
}

func (c *Client) Get(path string, params map[string]string) (json.RawMessage, error) {
	return c.GetWithHeaders(path, params, nil)
}

func (c *Client) GetWithHeaders(path string, params map[string]string, headers map[string]string) (json.RawMessage, error) {
	// Check cache for GET requests
	if !c.NoCache && !c.DryRun && c.cacheDir != "" {
		if cached, ok := c.readCache(path, params); ok {
			return cached, nil
		}
	}
	result, _, err := c.do("GET", path, params, nil, headers)
	if err == nil && !c.NoCache && !c.DryRun && c.cacheDir != "" {
		c.writeCache(path, params, result)
	}
	return result, err
}

func (c *Client) ProbeGet(path string) (int, error) {
	_, status, err := c.do("GET", path, nil, nil, nil)
	return status, err
}

func (c *Client) cacheKey(path string, params map[string]string) string {
	key := path
	key += "|base_url=" + c.BaseURL
	if c.Config != nil {
		key += "|auth_source=" + c.Config.AuthSource
		if authHeader := c.Config.AuthHeader(); authHeader != "" {
			authHash := sha256.Sum256([]byte(c.Config.AuthHeader()))
			key += "|auth=" + hex.EncodeToString(authHash[:8])
		}
		if c.Config.Path != "" {
			key += "|config_path=" + c.Config.Path
		}
	}
	paramKeys := make([]string, 0, len(params))
	for k := range params {
		paramKeys = append(paramKeys, k)
	}
	sort.Strings(paramKeys)
	for _, k := range paramKeys {
		key += k + "=" + params[k]
	}
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:8])
}

func (c *Client) readCache(path string, params map[string]string) (json.RawMessage, bool) {
	cacheFile := filepath.Join(c.cacheDir, c.cacheKey(path, params)+".json")
	info, err := os.Stat(cacheFile)
	if err != nil || time.Since(info.ModTime()) > 5*time.Minute {
		return nil, false
	}
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, false
	}
	return json.RawMessage(data), true
}

func (c *Client) writeCache(path string, params map[string]string, data json.RawMessage) {
	os.MkdirAll(c.cacheDir, 0o755)
	cacheFile := filepath.Join(c.cacheDir, c.cacheKey(path, params)+".json")
	os.WriteFile(cacheFile, []byte(data), 0o644)
}

// invalidateCache wholesale-removes the cache directory so the next read
// after a mutation cannot return a stale snapshot. Cache keys are opaque
// sha256 hashes — we have no index from resource → key to invalidate a
// subset, and selective invalidation against opaque hashes is no faster
// than starting fresh. Cheap: the cache TTL is 5 minutes anyway.
func (c *Client) invalidateCache() {
	if c.cacheDir == "" {
		return
	}
	_ = os.RemoveAll(c.cacheDir)
}

func (c *Client) Post(path string, body any) (json.RawMessage, int, error) {
	return c.do("POST", path, nil, body, nil)
}

func (c *Client) PostWithHeaders(path string, body any, headers map[string]string) (json.RawMessage, int, error) {
	return c.do("POST", path, nil, body, headers)
}

func (c *Client) Delete(path string) (json.RawMessage, int, error) {
	return c.do("DELETE", path, nil, nil, nil)
}

func (c *Client) DeleteWithQuery(path string, params map[string]string) (json.RawMessage, int, error) {
	return c.do("DELETE", path, params, nil, nil)
}

func (c *Client) DeleteWithHeaders(path string, headers map[string]string) (json.RawMessage, int, error) {
	return c.do("DELETE", path, nil, nil, headers)
}

func (c *Client) Put(path string, body any) (json.RawMessage, int, error) {
	return c.do("PUT", path, nil, body, nil)
}

func (c *Client) PutWithHeaders(path string, body any, headers map[string]string) (json.RawMessage, int, error) {
	return c.do("PUT", path, nil, body, headers)
}

func (c *Client) Patch(path string, body any) (json.RawMessage, int, error) {
	return c.do("PATCH", path, nil, body, nil)
}

func (c *Client) PatchWithHeaders(path string, body any, headers map[string]string) (json.RawMessage, int, error) {
	return c.do("PATCH", path, nil, body, headers)
}

// do executes an HTTP request. headerOverrides, when non-nil, override global
// RequiredHeaders for this specific request (used for per-endpoint API versioning).
func (c *Client) do(method, path string, params map[string]string, body any, headerOverrides map[string]string) (json.RawMessage, int, error) {
	targetURL := c.BaseURL + path

	var bodyBytes []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshaling body: %w", err)
		}
		bodyBytes = b
	}

	// Resolve auth material before the dry-run branch so --dry-run can preview
	// exactly what would be sent. Uses only cached credentials; a token that
	// requires a network refresh will be re-fetched on the live request path,
	// not during dry-run.
	authHeader, err := c.authHeader()
	if err != nil {
		return nil, 0, err
	}

	// Build the request for dry-run display or actual execution
	if c.DryRun {
		return c.dryRun(method, targetURL, path, params, bodyBytes, headerOverrides, authHeader)
	}

	// Per the project's no-retry-wrapper rule: one request, one response. 429
	// and 5xx propagate as a typed *APIError; callers decide what to do.
	if c.limiter != nil && c.limiter.Rate() > 0 {
		c.limiter.Wait()
	}

	var bodyReader io.Reader
	if bodyBytes != nil {
		bodyReader = strings.NewReader(string(bodyBytes))
	}

	req, err := http.NewRequest(method, targetURL, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}
	if bodyBytes != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", userAgent())

	if params != nil {
		q := req.URL.Query()
		for k, v := range params {
			if v != "" {
				q.Set(k, v)
			}
		}
		req.URL.RawQuery = q.Encode()
	}

	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	if c.Config != nil {
		for k, v := range c.Config.Headers {
			req.Header.Set(k, v)
		}
	}
	for k, v := range headerOverrides {
		req.Header.Set(k, v)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%s %s: %w", method, path, err)
	}
	respBody, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return nil, 0, fmt.Errorf("reading response: %w", readErr)
	}
	respBody = sanitizeJSONResponse(respBody)

	if resp.StatusCode < 400 {
		if c.limiter != nil {
			c.limiter.OnSuccess()
		}
		if method != http.MethodGet {
			c.invalidateCache()
		}
		return json.RawMessage(respBody), resp.StatusCode, nil
	}
	if resp.StatusCode == 429 && c.limiter != nil {
		c.limiter.OnRateLimit()
	}
	return nil, resp.StatusCode, &APIError{
		Method:     method,
		Path:       path,
		StatusCode: resp.StatusCode,
		Body:       truncateBody(respBody),
	}
}

// clientVersion holds the version string the User-Agent advertises.
// main wires this via SetVersion() at startup so the header tracks the
// same resolution chain (ldflag → BuildInfo → "0.0.0-dev" fallback) the
// user sees via `gorgias-pp-cli version`. Both binaries' main packages
// call SetVersion; tests and non-main callers can also drive it directly.
var clientVersion = "0.0.0-dev"

// SetVersion is called by main with the resolved version string. Both
// cmd/gorgias-pp-cli/main.go (via cli.Version()) and
// cmd/gorgias-pp-mcp/main.go (via resolveVersion()) pass through the same
// BuildInfo-aware path, so the User-Agent never carries "0.0.0-dev" on a
// `go install <module>@vX.Y.Z` build.
func SetVersion(v string) {
	if v != "" {
		clientVersion = v
	}
}

func userAgent() string {
	return "gorgias-pp-cli/" + clientVersion + " (+https://gorgias-pp-cli)"
}

// dryRun prints the outgoing request exactly as the live path would send it,
// using the auth material already resolved in `do()`. Never triggers a network
// call — the caller is responsible for passing cached auth material only.
//
// The structured return envelope carries the full preview (method, url,
// params, body, has_auth) so JSON consumers can inspect what would have been
// sent without parsing stderr.
func (c *Client) dryRun(method, targetURL, path string, params map[string]string, body []byte, headerOverrides map[string]string, authHeader string) (json.RawMessage, int, error) {
	fmt.Fprintf(os.Stderr, "%s %s\n", method, targetURL)
	queryPrinted := false
	if params != nil {
		keys := make([]string, 0, len(params))
		for k := range params {
			if params[k] != "" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			sep := "?"
			if queryPrinted {
				sep = "&"
			}
			fmt.Fprintf(os.Stderr, "  %s%s=%s\n", sep, k, params[k])
			queryPrinted = true
		}
	}
	_ = queryPrinted
	var bodyPreview any
	if body != nil {
		var pretty json.RawMessage
		if json.Unmarshal(body, &pretty) == nil {
			bodyPreview = pretty
			enc := json.NewEncoder(os.Stderr)
			enc.SetIndent("  ", "  ")
			fmt.Fprintf(os.Stderr, "  Body:\n")
			enc.Encode(pretty)
		}
	}
	if authHeader != "" {
		fmt.Fprintf(os.Stderr, "  %s: %s\n", "Authorization", maskToken(authHeader))
	}
	fmt.Fprintf(os.Stderr, "\n(dry run - no request sent)\n")

	envelope := map[string]any{
		"dry_run":  true,
		"method":   method,
		"url":      targetURL,
		"path":     path,
		"has_auth": authHeader != "",
	}
	if len(params) > 0 {
		envelope["params"] = params
	}
	if bodyPreview != nil {
		envelope["body"] = bodyPreview
	}
	if len(headerOverrides) > 0 {
		envelope["header_overrides"] = headerOverrides
	}
	data, marshalErr := json.Marshal(envelope)
	if marshalErr != nil {
		return json.RawMessage(`{"dry_run": true}`), 0, nil
	}
	return json.RawMessage(data), 0, nil
}

func (c *Client) ConfiguredTimeout() time.Duration {
	if c.HTTPClient != nil && c.HTTPClient.Timeout > 0 {
		return c.HTTPClient.Timeout
	}
	return 30 * time.Second
}

func (c *Client) authHeader() (string, error) {
	if c.Config == nil {
		return "", nil
	}
	return c.Config.AuthHeader(), nil
}

// sanitizeJSONResponse strips known JSONP/XSSI prefixes and UTF-8 BOM from
// response bodies so that downstream JSON parsing succeeds. For clean JSON
// responses these checks are no-ops.
func sanitizeJSONResponse(body []byte) []byte {
	// UTF-8 BOM
	body = bytes.TrimPrefix(body, []byte("\xEF\xBB\xBF"))

	// JSONP/XSSI prefixes, ordered longest-first where prefixes overlap
	prefixes := [][]byte{
		[]byte(")]}'\n"),
		[]byte(")]}'"),
		[]byte("{}&&"),
		[]byte("for(;;);"),
		[]byte("while(1);"),
	}
	for _, p := range prefixes {
		if bytes.HasPrefix(body, p) {
			body = bytes.TrimPrefix(body, p)
			body = bytes.TrimLeft(body, " \t\r\n")
			break
		}
	}
	return body
}

// maskToken redacts all but the last 4 characters of a token for safe display.
func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 4 {
		return "****"
	}
	return "****" + token[len(token)-4:]
}

func truncateBody(b []byte) string {
	s := string(b)
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
