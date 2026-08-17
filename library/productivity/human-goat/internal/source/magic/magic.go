// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package magic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/cliutil"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

const (
	defaultBaseURL       = "https://console.api.getmagic.com/api/v1"
	defaultRateLimit     = 2.0
	defaultTimeout       = 30 * time.Second
	maxRateLimitRetries  = 3
	maxErrorSnippetBytes = 500
)

// Request is a Magic remote-errand request.
type Request struct {
	ID           string                `json:"id"`
	Status       string                `json:"status"`
	Title        string                `json:"title"`
	CompletedAt  string                `json:"completed_at"`
	Result       string                `json:"result"`
	Conversation []ConversationMessage `json:"conversation"`
}

// MarshalJSON keeps empty conversation collections encoded as [] instead of null.
func (r Request) MarshalJSON() ([]byte, error) {
	type requestAlias Request
	out := requestAlias(r)
	if out.Conversation == nil {
		out.Conversation = make([]ConversationMessage, 0)
	}
	return json.Marshal(out)
}

// UnmarshalJSON keeps absent conversation collections as empty slices.
func (r *Request) UnmarshalJSON(data []byte) error {
	type requestAlias Request
	var out requestAlias
	if err := json.Unmarshal(data, &out); err != nil {
		return err
	}
	*r = Request(out)
	normalizeRequest(r)
	return nil
}

// ConversationMessage is a single Magic conversation message.
type ConversationMessage struct {
	Content string `json:"content"`
}

// SendParams describes a Magic request to create.
type SendParams struct {
	Title        string
	Instructions string
	Objective    string
	MaxMinutes   *int
	Relaxed      bool
	Tag          string
}

// Client is a small Magic API client.
type Client struct {
	baseURL     string
	httpClient  *http.Client
	limiter     *cliutil.AdaptiveLimiter
	keyProvider func() (string, error)
}

// NewClient returns a Magic client configured from environment variables.
func NewClient() (*Client, error) {
	baseURL := strings.TrimSpace(os.Getenv("MAGIC_BASE"))
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if err := validateBaseURL(baseURL); err != nil {
		return nil, err
	}
	return &Client{
		baseURL:     baseURL,
		httpClient:  &http.Client{Timeout: defaultTimeout},
		limiter:     cliutil.NewAdaptiveLimiter(defaultRateLimit),
		keyProvider: ResolveKey,
	}, nil
}

// ResolveKey resolves the Magic API key from MAGIC_API_KEY, then api_key in
// MAGIC_CONFIG_DIR or $HOME/.magic.
func ResolveKey() (string, error) {
	if key := strings.TrimSpace(os.Getenv("MAGIC_API_KEY")); key != "" {
		return key, nil
	}

	keyPath, err := magicAPIKeyPath()
	if err != nil {
		return "", err
	}
	// #nosec G304 -- keyPath derives only from MAGIC_CONFIG_DIR or $HOME/.magic, a trusted config location, never user-supplied request input.
	keyBytes, err := os.ReadFile(keyPath)
	if err == nil {
		if key := strings.TrimRightFunc(string(keyBytes), unicode.IsSpace); key != "" {
			return key, nil
		}
		return "", fmt.Errorf("magic: API key not found; set MAGIC_API_KEY or write a non-empty key to MAGIC_CONFIG_DIR/api_key (%s)", keyPath)
	}
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("magic: API key not found; set MAGIC_API_KEY or write key to MAGIC_CONFIG_DIR/api_key (%s)", keyPath)
	}
	return "", fmt.Errorf("magic: API key not found in MAGIC_API_KEY and could not read MAGIC_CONFIG_DIR/api_key (%s): %w", keyPath, err)
}

// Send creates a Magic request.
func (c *Client) Send(ctx context.Context, params SendParams) (*Request, error) {
	if strings.TrimSpace(params.Title) == "" {
		return nil, errors.New("magic: title is required")
	}
	if strings.TrimSpace(params.Instructions) == "" {
		return nil, errors.New("magic: instructions are required")
	}
	if strings.TrimSpace(params.Objective) == "" {
		return nil, errors.New("magic: objective is required")
	}

	payload := sendPayload{
		Title:        params.Title,
		Instructions: params.Instructions,
		Objective:    params.Objective,
		MaxMinutes:   params.MaxMinutes,
		Relaxed:      params.Relaxed,
	}
	if strings.TrimSpace(params.Tag) != "" {
		payload.CallbackMeta = &callbackMeta{Tag: params.Tag}
	}

	body, err := c.do(ctx, http.MethodPost, "/request", payload)
	if err != nil {
		return nil, err
	}
	var request Request
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, fmt.Errorf("magic: decode request: %w", err)
	}
	normalizeRequest(&request)
	return &request, nil
}

// Call creates a Magic request to make a phone call and report the answer.
func (c *Client) Call(ctx context.Context, number, ask string) (*Request, error) {
	if strings.TrimSpace(number) == "" {
		return nil, errors.New("magic: number is required")
	}
	if strings.TrimSpace(ask) == "" {
		return nil, errors.New("magic: ask is required")
	}
	return c.Send(ctx, SendParams{
		Title:        "Phone call: " + number,
		Instructions: "Call " + number + ". Ask: " + ask + ". Report the answer verbatim. If there is no answer, the line is busy, or the business is closed, say so and include any hours or voicemail message you hear.",
		Objective:    "Get the answer to: " + ask,
		Tag:          number,
	})
}

// GetRequest fetches a Magic request by ID.
func (c *Client) GetRequest(ctx context.Context, id string) (*Request, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("magic: request id is required")
	}
	body, err := c.do(ctx, http.MethodGet, "/request/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	var request Request
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, fmt.Errorf("magic: decode request: %w", err)
	}
	normalizeRequest(&request)
	return &request, nil
}

// Reply posts a conversation reply for a Magic request.
func (c *Client) Reply(ctx context.Context, requestID, content string) (map[string]any, error) {
	if strings.TrimSpace(requestID) == "" {
		return nil, errors.New("magic: request id is required")
	}
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("magic: content is required")
	}
	body, err := c.do(ctx, http.MethodPost, "/conversation", map[string]string{
		"content":    content,
		"request_id": requestID,
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]any)
	if len(bytes.TrimSpace(body)) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("magic: decode conversation response: %w", err)
	}
	if out == nil {
		out = make(map[string]any)
	}
	return out, nil
}

// IsInProgress reports whether a Magic status should be treated as non-terminal.
func IsInProgress(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "":
		return false
	case "COMPLETED", "DONE", "CANCELLED", "CANCELED", "FAILED", "CLOSED":
		return false
	case "PENDING", "ONGOING", "IN_PROGRESS", "ASSIGNED", "ACTIVE", "CREATED", "QUEUED":
		return true
	default:
		return true
	}
}

// Answer returns the best available human answer text for a Magic request.
func (r *Request) Answer() string {
	if r == nil {
		return ""
	}
	for i := len(r.Conversation) - 1; i >= 0; i-- {
		if content := strings.TrimSpace(r.Conversation[i].Content); content != "" {
			return r.Conversation[i].Content
		}
	}
	return r.Result
}

func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	if c == nil {
		return nil, errors.New("magic: nil client")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	keyProvider := c.keyProvider
	if keyProvider == nil {
		keyProvider = ResolveKey
	}
	key, err := keyProvider()
	if err != nil {
		return nil, fmt.Errorf("magic: resolving API key: %w", err)
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errors.New("magic: resolved API key is empty")
	}

	var encoded []byte
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("magic: encode request body: %w", err)
		}
	}

	targetURL, err := c.urlFor(path)
	if err != nil {
		return nil, err
	}
	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	for attempt := 0; ; attempt++ {
		c.limiter.Wait()
		var reqBody io.Reader
		if encoded != nil {
			reqBody = bytes.NewReader(encoded)
		}
		req, err := http.NewRequestWithContext(ctx, method, targetURL, reqBody)
		if err != nil {
			return nil, fmt.Errorf("magic: build request %s %s: %w", method, path, err)
		}
		req.Header.Set("x-api-key", key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			return nil, fmt.Errorf("magic: %s %s: %w", method, path, err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("magic: read response %s %s: %w", method, path, readErr)
		}

		if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
			c.limiter.OnSuccess()
			return respBody, nil
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			c.limiter.OnRateLimit()
			wait := cliutil.RetryAfter(resp)
			if attempt >= maxRateLimitRetries {
				return nil, &cliutil.RateLimitError{
					URL:        targetURL,
					RetryAfter: wait,
					Body:       errorSnippet(respBody),
				}
			}
			if err := sleepContext(ctx, wait); err != nil {
				return nil, err
			}
			continue
		}

		return nil, fmt.Errorf("magic: HTTP %d %s %s: %s", resp.StatusCode, method, path, errorSnippet(respBody))
	}
}

type sendPayload struct {
	Title        string        `json:"title"`
	Instructions string        `json:"instructions"`
	Objective    string        `json:"objective"`
	MaxMinutes   *int          `json:"max_minutes,omitempty"`
	Relaxed      bool          `json:"relaxed,omitempty"`
	CallbackMeta *callbackMeta `json:"callback_meta,omitempty"`
}

type callbackMeta struct {
	Tag string `json:"tag"`
}

func validateBaseURL(baseURL string) error {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("magic: invalid MAGIC_BASE %q: %w", baseURL, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("magic: invalid MAGIC_BASE %q: must include scheme and host", baseURL)
	}
	return nil
}

func (c *Client) urlFor(path string) (string, error) {
	baseURL := strings.TrimRight(c.baseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if err := validateBaseURL(baseURL); err != nil {
		return "", err
	}
	if path == "" {
		return baseURL, nil
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return baseURL + path, nil
}

func magicAPIKeyPath() (string, error) {
	configDir := strings.TrimSpace(os.Getenv("MAGIC_CONFIG_DIR"))
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("magic: API key not found; set MAGIC_API_KEY or MAGIC_CONFIG_DIR/api_key (default $HOME/.magic/api_key): %w", err)
		}
		configDir = filepath.Join(home, ".magic")
	}
	return filepath.Join(configDir, "api_key"), nil
}

func normalizeRequest(request *Request) {
	if request != nil && request.Conversation == nil {
		request.Conversation = make([]ConversationMessage, 0)
	}
}

func errorSnippet(body []byte) string {
	snippet := strings.TrimSpace(string(body))
	if len(snippet) <= maxErrorSnippetBytes {
		return snippet
	}
	return snippet[:maxErrorSnippetBytes] + "..."
}

func sleepContext(ctx context.Context, wait time.Duration) error {
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
