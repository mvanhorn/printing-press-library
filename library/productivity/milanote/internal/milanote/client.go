// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.

package milanote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/cliutil"
)

// DefaultBaseURL is the Milanote web app origin.
const DefaultBaseURL = "https://app.milanote.com"

// SessionCookie is the HttpOnly cookie that authenticates every request.
const SessionCookie = "__Secure-milanote.session_token"

// ErrUnauthorized means the session cookie is missing, expired or revoked.
var ErrUnauthorized = errors.New("milanote session is not valid (HTTP 401): log in to Milanote in your browser, then run 'milanote-pp-cli chrome-login' or 'auth login --cookies-file'")

// APIError is a non-2xx answer from the REST API.
type APIError struct {
	Method  string
	Path    string
	Status  int
	Message string
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("milanote %s %s: HTTP %d", e.Method, e.Path, e.Status)
	if e.Message != "" {
		msg += ": " + e.Message
	}
	return msg
}

// IsNotFound reports whether err is a 404 from Milanote.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound
}

// Client is a cookie-authenticated Milanote client. One jar is shared by
// REST and socket.io requests so the load balancer's stickiness cookies
// (AWSALB/AWSALBCORS) ride every call; without them socket.io POSTs fail.
type Client struct {
	BaseURL   string
	HTTP      *http.Client
	Poll      *http.Client
	limiter   *cliutil.AdaptiveLimiter
	userAgent string
	user      *User
	ids       *IDGen
}

// NormalizeCookie turns a stored credential into a Cookie header value.
// It accepts a bare token, "name=value", or a "a=b; c=d" jar string.
func NormalizeCookie(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "Cookie:")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "=") {
		return SessionCookie + "=" + raw
	}
	return raw
}

// DefaultRequestRate is the pace, in requests per second, used unless the
// caller asks for another.
const DefaultRequestRate = 4.0

// Options tunes a Client. The zero value is not the default for RateLimit:
// use RateLimit < 0 for DefaultRequestRate, 0 to turn pacing off, and a positive
// number for that many requests per second.
type Options struct {
	Timeout   time.Duration     // bounds REST calls; socket.io long-polls use a longer budget
	RateLimit float64           // see above
	Transport http.RoundTripper // nil uses the default transport
}

// New builds a client from a session credential.
func New(baseURL, credential string, opts Options) (*Client, error) {
	cookie := NormalizeCookie(credential)
	if cookie == "" {
		return nil, ErrUnauthorized
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing base url: %w", err)
	}
	jar, _ := cookiejar.New(nil)
	var cookies []*http.Cookie
	for _, part := range strings.Split(cookie, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || name == "" {
			continue
		}
		cookies = append(cookies, &http.Cookie{Name: name, Value: value, Path: "/", Secure: u.Scheme == "https"})
	}
	jar.SetCookies(u, cookies)
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	rate := opts.RateLimit
	if rate < 0 {
		rate = DefaultRequestRate
	}
	return &Client{
		BaseURL:   baseURL,
		HTTP:      &http.Client{Jar: jar, Timeout: timeout, Transport: opts.Transport},
		Poll:      &http.Client{Jar: jar, Timeout: 40 * time.Second, Transport: opts.Transport},
		limiter:   cliutil.NewAdaptiveLimiter(rate),
		userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
	}, nil
}

func (c *Client) newRequest(ctx context.Context, method, rawURL string, body io.Reader, contentType string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Origin", c.BaseURL)
	req.Header.Set("Referer", c.BaseURL+"/")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req, nil
}

// Do sends one REST request and returns the raw body of a 2xx answer.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body any) ([]byte, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	full := path
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		full = c.BaseURL + path
	}
	if len(query) > 0 {
		full += "?" + query.Encode()
	}
	var reader io.Reader
	contentType := ""
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding request body: %w", err)
		}
		reader = bytes.NewReader(buf)
		contentType = "application/json"
	}
	req, err := c.newRequest(ctx, method, full, reader, contentType)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("milanote %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, fmt.Errorf("reading milanote response: %w", err)
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		c.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: path, RetryAfter: cliutil.RetryAfter(resp), Body: string(data)}
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case resp.StatusCode >= 400:
		return nil, &APIError{Method: method, Path: path, Status: resp.StatusCode, Message: errorMessage(data)}
	}
	c.limiter.OnSuccess()
	return data, nil
}

func errorMessage(data []byte) string {
	var body struct {
		Error any `json:"error"`
	}
	if json.Unmarshal(data, &body) == nil {
		switch e := body.Error.(type) {
		case string:
			return e
		case map[string]any:
			if m, ok := e["message"].(string); ok {
				return m
			}
		}
	}
	text := strings.TrimSpace(string(data))
	if len(text) > 200 || strings.HasPrefix(text, "<") {
		return ""
	}
	return text
}

// GetJSON decodes a GET answer into out.
func (c *Client) GetJSON(ctx context.Context, path string, query url.Values, out any) error {
	data, err := c.Do(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decoding %s: %w", path, err)
	}
	return nil
}

// Me returns the signed-in user, cached per client.
func (c *Client) Me(ctx context.Context) (*User, error) {
	if c.user != nil {
		return c.user, nil
	}
	var answer struct {
		User *User `json:"user"`
	}
	if err := c.GetJSON(ctx, "/api/users/me", nil, &answer); err != nil {
		return nil, err
	}
	if answer.User == nil || answer.User.ID == "" {
		return nil, ErrUnauthorized
	}
	c.user = answer.User
	c.ids = NewIDGen(answer.User.ClientID, answer.User.ClientTick)
	return c.user, nil
}

// IDs returns the element id generator (Me must have run).
func (c *Client) IDs(ctx context.Context) (*IDGen, error) {
	if c.ids == nil {
		if _, err := c.Me(ctx); err != nil {
			return nil, err
		}
	}
	return c.ids, nil
}

// Boards loads one or more boards with every element, comment and label in them.
func (c *Client) Boards(ctx context.Context, ids ...string) (*BoardPayload, error) {
	q := url.Values{}
	q.Set("excludeSelf", "false")
	q.Set("loadAncestors", "false")
	q.Set("ids", strings.Join(ids, ","))
	var payload BoardPayload
	if err := c.GetJSON(ctx, "/api/boards", q, &payload); err != nil {
		return nil, err
	}
	if payload.Elements == nil {
		payload.Elements = map[string]*Element{}
	}
	return &payload, nil
}

// Board loads a single board and checks that it exists.
func (c *Client) Board(ctx context.Context, id string) (*BoardPayload, error) {
	payload, err := c.Boards(ctx, id)
	if err != nil {
		return nil, err
	}
	el := payload.Elements[id]
	if el == nil || el.ElementType != TypeBoard {
		return nil, &APIError{Method: "GET", Path: "/api/boards", Status: http.StatusNotFound, Message: "no board " + id + " visible to this account"}
	}
	return payload, nil
}

// SearchBoards runs Milanote's own title search.
func (c *Client) SearchBoards(ctx context.Context, query string, offset int) ([]SearchHit, error) {
	q := url.Values{}
	q.Set("query", query)
	q.Set("offset", fmt.Sprint(offset))
	q.Set("sort", "")
	q.Set("locale", "en-au")
	var answer struct {
		Results []SearchHit `json:"results"`
	}
	if err := c.GetJSON(ctx, "/api/boards/search", q, &answer); err != nil {
		return nil, err
	}
	return answer.Results, nil
}

// History returns recently viewed boards, newest first, with the board
// elements Milanote sends alongside.
func (c *Client) History(ctx context.Context) ([]HistoryEntry, map[string]*Element, error) {
	var answer struct {
		History    []HistoryEntry      `json:"history"`
		HistoryIDs []string            `json:"historyIds"`
		Elements   map[string]*Element `json:"elements"`
	}
	if err := c.GetJSON(ctx, "/api/users/me/history", nil, &answer); err != nil {
		return nil, nil, err
	}
	if len(answer.History) == 0 {
		for _, id := range answer.HistoryIDs {
			answer.History = append(answer.History, HistoryEntry{ElementID: id})
		}
	}
	return answer.History, answer.Elements, nil
}

// Trash lists the account's trashed elements.
func (c *Client) Trash(ctx context.Context) (*BoardPayload, error) {
	q := url.Values{}
	q.Set("other-users", "false")
	var payload BoardPayload
	if err := c.GetJSON(ctx, "/api/users/me/trash", q, &payload); err != nil {
		return nil, err
	}
	if payload.Elements == nil {
		payload.Elements = map[string]*Element{}
	}
	return &payload, nil
}

// Labels lists the account's labels.
func (c *Client) Labels(ctx context.Context) ([]Label, error) {
	var answer struct {
		Labels []Label `json:"labels"`
	}
	if err := c.GetJSON(ctx, "/api/labels", nil, &answer); err != nil {
		return nil, err
	}
	return answer.Labels, nil
}

// Counts returns plan usage counts.
func (c *Client) Counts(ctx context.Context) (map[string]any, error) {
	var answer map[string]any
	err := c.GetJSON(ctx, "/api/users/me/counts", nil, &answer)
	return answer, err
}

// Permissions returns a board's sharing rows.
func (c *Client) Permissions(ctx context.Context, boardID string) ([]map[string]any, error) {
	var answer struct {
		Permissions []map[string]any `json:"permissions"`
	}
	err := c.GetJSON(ctx, "/api/permissions/board/"+url.PathEscape(boardID), nil, &answer)
	return answer.Permissions, err
}

// Users looks up collaborators by id.
func (c *Client) Users(ctx context.Context, ids []string) (map[string]map[string]any, error) {
	if len(ids) == 0 {
		return map[string]map[string]any{}, nil
	}
	q := url.Values{}
	q.Set("ids", strings.Join(ids, ","))
	var answer struct {
		Users map[string]map[string]any `json:"users"`
	}
	err := c.GetJSON(ctx, "/api/users", q, &answer)
	if answer.Users == nil {
		answer.Users = map[string]map[string]any{}
	}
	return answer.Users, err
}

// PostJSON sends a JSON body with method and decodes the answer into out.
func (c *Client) PostJSON(ctx context.Context, method, path string, body, out any) error {
	data, err := c.Do(ctx, method, path, nil, body)
	if err != nil {
		return err
	}
	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

// Fetch downloads an absolute URL (export files) with the session jar.
func (c *Client) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	req, err := c.newRequest(ctx, http.MethodGet, rawURL, nil, "")
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, &APIError{Method: "GET", Path: rawURL, Status: resp.StatusCode}
	}
	c.limiter.OnSuccess()
	return io.ReadAll(io.LimitReader(resp.Body, 256<<20))
}

// storageTransport keeps the trusted CAs for uploads but drops any certificate
// name override, which describes the Milanote host and not the storage host.
func storageTransport(rt http.RoundTripper) http.RoundTripper {
	tr, ok := rt.(*http.Transport)
	if !ok || tr.TLSClientConfig == nil || tr.TLSClientConfig.ServerName == "" {
		return rt
	}
	clone := tr.Clone()
	clone.TLSClientConfig.ServerName = ""
	return clone
}

// PutBytes uploads bytes to a presigned storage URL. No session cookie is
// sent: the URL itself carries the authorization.
func (c *Client) PutBytes(ctx context.Context, rawURL, contentType string, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, rawURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := (&http.Client{Timeout: 5 * time.Minute, Transport: storageTransport(c.HTTP.Transport)}).Do(req)
	if err != nil {
		return fmt.Errorf("uploading file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("uploading file: storage answered HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
