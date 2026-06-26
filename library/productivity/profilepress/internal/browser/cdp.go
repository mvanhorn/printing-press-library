package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// PageText is the read-only text payload captured from a browser tab.
type PageText struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

type Client struct {
	BaseURL string
	HTTP    *http.Client
	nextID  atomic.Int64
}

type Target struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	Title                string `json:"title"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:9222"
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{BaseURL: baseURL, HTTP: &http.Client{Timeout: timeout}}
}

func (c *Client) ListTargets(ctx context.Context) ([]Target, error) {
	var targets []Target
	if err := c.getJSON(ctx, "/json/list", &targets); err != nil {
		return nil, fmt.Errorf("list Chrome CDP targets at %s: %w", c.BaseURL, err)
	}
	return targets, nil
}

func (c *Client) NewTarget(ctx context.Context, rawURL string) (Target, error) {
	endpoint := "/json/new?" + url.QueryEscape(rawURL)
	var target Target
	if err := c.putJSON(ctx, endpoint, nil, &target); err != nil {
		// Older Chrome builds accept GET for /json/new. Retry once for compatibility.
		if err2 := c.getJSON(ctx, endpoint, &target); err2 != nil {
			return Target{}, fmt.Errorf("create Chrome CDP target for %q: %w", rawURL, err)
		}
	}
	if target.WebSocketDebuggerURL == "" {
		return Target{}, fmt.Errorf("created target for %q but Chrome did not return a websocket debugger URL", rawURL)
	}
	return target, nil
}

func (c *Client) SelectTarget(ctx context.Context, profileURL, targetURLMatch string) (Target, error) {
	targets, err := c.ListTargets(ctx)
	if err != nil {
		return Target{}, err
	}
	match := targetURLMatch
	if match == "" {
		match = profileURL
	}
	if match != "" {
		for _, t := range targets {
			if t.Type == "page" && t.WebSocketDebuggerURL != "" && strings.Contains(t.URL, match) {
				return t, nil
			}
		}
	}
	for _, t := range targets {
		if t.Type == "page" && t.WebSocketDebuggerURL != "" && strings.Contains(strings.ToLower(t.URL), "linkedin.com") {
			return t, nil
		}
	}
	for _, t := range targets {
		if t.Type == "page" && t.WebSocketDebuggerURL != "" {
			return t, nil
		}
	}
	if profileURL != "" {
		return c.NewTarget(ctx, profileURL)
	}
	return Target{}, errors.New("no usable Chrome page target found; pass --profile-url or open LinkedIn in a Chrome tab")
}

func (c *Client) CapturePageText(ctx context.Context, profileURL, targetURLMatch string, wait time.Duration) (PageText, error) {
	target, err := c.SelectTarget(ctx, profileURL, targetURLMatch)
	if err != nil {
		return PageText{}, err
	}
	if profileURL != "" && !sameURL(target.URL, profileURL) {
		if err := c.navigate(ctx, target.WebSocketDebuggerURL, profileURL); err != nil {
			return PageText{}, err
		}
		if wait <= 0 {
			wait = 4 * time.Second
		}
		select {
		case <-ctx.Done():
			return PageText{}, ctx.Err()
		case <-time.After(wait):
		}
	}
	return c.evaluatePageText(ctx, target.WebSocketDebuggerURL)
}

func (c *Client) navigate(ctx context.Context, wsURL, rawURL string) error {
	_, err := c.call(ctx, wsURL, "Page.navigate", map[string]any{"url": rawURL})
	if err != nil {
		return fmt.Errorf("navigate Chrome target to %q: %w", rawURL, err)
	}
	return nil
}

func (c *Client) evaluatePageText(ctx context.Context, wsURL string) (PageText, error) {
	expr := `JSON.stringify({url: location.href, title: document.title, text: document.body ? document.body.innerText : ""})`
	result, err := c.call(ctx, wsURL, "Runtime.evaluate", map[string]any{"expression": expr, "returnByValue": true})
	if err != nil {
		return PageText{}, fmt.Errorf("evaluate page text: %w", err)
	}
	var envelope struct {
		Result struct {
			Result struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"result"`
			ExceptionDetails any `json:"exceptionDetails"`
		} `json:"result"`
	}
	if err := json.Unmarshal(result, &envelope); err != nil {
		return PageText{}, err
	}
	if envelope.Result.ExceptionDetails != nil {
		return PageText{}, fmt.Errorf("page evaluation raised exception: %v", envelope.Result.ExceptionDetails)
	}
	var page PageText
	if err := json.Unmarshal([]byte(envelope.Result.Result.Value), &page); err != nil {
		return PageText{}, fmt.Errorf("decode page text payload: %w", err)
	}
	return page, nil
}

func (c *Client) call(ctx context.Context, wsURL, method string, params map[string]any) ([]byte, error) {
	dialer := websocket.Dialer{HandshakeTimeout: c.HTTP.Timeout}
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	id := c.nextID.Add(1)
	msg := map[string]any{"id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	if err := conn.WriteJSON(msg); err != nil {
		return nil, err
	}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		_, b, err := conn.ReadMessage()
		if err != nil {
			return nil, err
		}
		var resp struct {
			ID    int64           `json:"id"`
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(b, &resp); err != nil {
			return nil, err
		}
		if resp.ID != id {
			continue
		}
		if len(resp.Error) > 0 && string(resp.Error) != "null" {
			return nil, fmt.Errorf("CDP %s error: %s", method, string(resp.Error))
		}
		return b, nil
	}
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	return c.doJSON(req, out)
}

func (c *Client) putJSON(ctx context.Context, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.BaseURL+path, body)
	if err != nil {
		return err
	}
	return c.doJSON(req, out)
}

func (c *Client) doJSON(req *http.Request, out any) error {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(b, out)
}

func sameURL(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ua, ea := url.Parse(a)
	ub, eb := url.Parse(b)
	if ea != nil || eb != nil {
		return strings.TrimRight(a, "/") == strings.TrimRight(b, "/")
	}
	return strings.EqualFold(ua.Host, ub.Host) && strings.TrimRight(ua.Path, "/") == strings.TrimRight(ub.Path, "/")
}
