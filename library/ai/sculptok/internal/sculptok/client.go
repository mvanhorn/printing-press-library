// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored sibling client for the SculptOK api-open surface. It exists
// because the headline workflow commands need three things the generated
// internal/client cannot do:
//   1. multipart/form-data image upload (POST /image/upload),
//   2. correct {code,msg,data} envelope handling — the API returns HTTP 200
//      for everything and signals failure via code != 0, and
//   3. a submit -> poll loop for the async draw endpoints.
//
// It uses cliutil.AdaptiveLimiter + cliutil.RateLimitError so throttling is a
// hard, visible error rather than empty output.

package sculptok

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/sculptok/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/sculptok/internal/config"
)

// Client talks to https://api.sculptok.com/api-open with apikey-header auth.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

// New builds a client from the resolved config. rps <= 0 disables limiting.
func New(cfg *config.Config, timeout time.Duration, rps float64) *Client {
	if timeout <= 0 {
		timeout = time.Minute
	}
	return &Client{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:  cfg.AuthHeader(),
		http:    &http.Client{Timeout: timeout},
		limiter: cliutil.NewAdaptiveLimiter(rps),
	}
}

// HasKey reports whether an API key is configured.
func (c *Client) HasKey() bool { return strings.TrimSpace(c.apiKey) != "" }

// APIError is a non-zero envelope code returned by SculptOK.
type APIError struct {
	Code int
	Msg  string
}

func (e *APIError) Error() string {
	switch e.Code {
	case 10020:
		return "SculptOK API key is missing (code 10020): set SCULPTOK_API_KEY or run 'sculptok-pp-cli auth set-token'"
	case 10021, 401:
		return fmt.Sprintf("SculptOK API key is invalid or expired (code %d): %s", e.Code, e.Msg)
	default:
		return fmt.Sprintf("SculptOK API error (code %d): %s", e.Code, e.Msg)
	}
}

type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

const maxRateRetries = 3

// do performs an HTTP request, retries on 429 with the limiter, and unwraps the
// envelope. It returns the raw data field on success (code == 0).
func (c *Client) do(ctx context.Context, method, path string, query map[string]string, contentType string, body io.Reader, bodyBytes []byte) (json.RawMessage, error) {
	reqURL := c.baseURL + path
	if len(query) > 0 {
		q := url.Values{}
		for k, v := range query {
			if v == "" {
				continue
			}
			q.Set(k, v)
		}
		if encoded := q.Encode(); encoded != "" {
			reqURL += "?" + encoded
		}
	}

	var lastErr error
	for attempt := 0; attempt <= maxRateRetries; attempt++ {
		c.limiter.Wait()

		var reqBody io.Reader
		if bodyBytes != nil {
			reqBody = bytes.NewReader(bodyBytes)
		} else {
			reqBody = body
		}
		req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
		if err != nil {
			return nil, err
		}
		if c.apiKey != "" {
			req.Header.Set("apikey", c.apiKey)
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		raw, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			c.limiter.OnRateLimit()
			if attempt < maxRateRetries {
				wait := cliutil.RetryAfter(resp)
				lastErr = &cliutil.RateLimitError{URL: reqURL, RetryAfter: wait, Body: string(raw)}
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(wait):
				}
				continue
			}
			return nil, &cliutil.RateLimitError{URL: reqURL, RetryAfter: cliutil.RetryAfter(resp), Body: string(raw)}
		}
		c.limiter.OnSuccess()

		if resp.StatusCode >= 500 {
			return nil, fmt.Errorf("SculptOK server error: HTTP %d for %s", resp.StatusCode, path)
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			// Non-JSON body (e.g. a gateway HTML error) — surface a bounded snippet.
			snippet := strings.TrimSpace(string(raw))
			if len(snippet) > 200 {
				snippet = snippet[:200]
			}
			return nil, fmt.Errorf("unexpected non-JSON response from %s (HTTP %d): %s", path, resp.StatusCode, snippet)
		}
		if env.Code != 0 {
			return nil, &APIError{Code: env.Code, Msg: env.Msg}
		}
		return env.Data, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("request to %s failed after retries", path)
}

// Balance returns the current credit balance.
func (c *Client) Balance(ctx context.Context) (int, error) {
	data, err := c.do(ctx, http.MethodGet, "/point/info", nil, "", nil, nil)
	if err != nil {
		return 0, err
	}
	var out struct {
		Point int `json:"point"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return 0, fmt.Errorf("parsing balance: %w", err)
	}
	return out.Point, nil
}

// Upload posts a local image file as multipart/form-data and returns the
// SculptOK-hosted src URL to feed into a draw.
func (c *Client) Upload(ctx context.Context, imagePath string) (string, error) {
	// #nosec G304 -- imagePath is the local image the user explicitly asked to
	// upload; reading a user-supplied file is the command's entire purpose.
	f, err := os.Open(imagePath)
	if err != nil {
		return "", fmt.Errorf("opening image: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", filepath.Base(imagePath))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, f); err != nil {
		return "", err
	}
	if err := mw.Close(); err != nil {
		return "", err
	}

	data, err := c.do(ctx, http.MethodPost, "/image/upload", nil, mw.FormDataContentType(), nil, buf.Bytes())
	if err != nil {
		return "", err
	}
	var out struct {
		Src string `json:"src"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("parsing upload response: %w", err)
	}
	if out.Src == "" {
		return "", fmt.Errorf("upload returned no src URL")
	}
	return out.Src, nil
}

// Submit posts a draw body to the given sub-path (e.g. /draw/prompt) and
// returns the promptId.
func (c *Client) Submit(ctx context.Context, path string, body map[string]any) (string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	data, err := c.do(ctx, http.MethodPost, path, nil, "application/json", nil, payload)
	if err != nil {
		return "", err
	}
	var out struct {
		PromptID string `json:"promptId"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("parsing submit response: %w", err)
	}
	if out.PromptID == "" {
		return "", fmt.Errorf("submit returned no promptId")
	}
	return out.PromptID, nil
}

// Status describes a draw's progress. status: typically 2 == completed.
type Status struct {
	ID          string          `json:"id"`
	PromptID    string          `json:"promptId"`
	Status      int             `json:"status"`
	CurrentStep int             `json:"currentStep"`
	Position    int             `json:"position"`
	UpImageURL  string          `json:"upImageUrl"`
	ImgRecords  []string        `json:"imgRecords"`
	Raw         json.RawMessage `json:"-"`
}

// Done reports whether the draw has produced results.
func (s *Status) Done() bool {
	return len(s.ImgRecords) > 0
}

// GetStatus fetches the current status of a submitted draw.
func (c *Client) GetStatus(ctx context.Context, promptID string) (*Status, error) {
	data, err := c.do(ctx, http.MethodGet, "/draw/prompt", map[string]string{"uuid": promptID}, "", nil, nil)
	if err != nil {
		return nil, err
	}
	var s Status
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing status: %w", err)
	}
	s.Raw = data
	if s.PromptID == "" {
		s.PromptID = promptID
	}
	return &s, nil
}

// Poll repeatedly fetches status until results are present or ctx/deadline ends.
// onUpdate (optional) is called with each non-terminal status for progress.
func (c *Client) Poll(ctx context.Context, promptID string, interval time.Duration, onUpdate func(*Status)) (*Status, error) {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	for {
		s, err := c.GetStatus(ctx, promptID)
		if err != nil {
			return nil, err
		}
		if s.Done() {
			return s, nil
		}
		if onUpdate != nil {
			onUpdate(s)
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out waiting for draw %s (last status %d, queue position %d): %w", promptID, s.Status, s.Position, ctx.Err())
		case <-time.After(interval):
		}
	}
}

// ListPage fetches one page of a paged list endpoint (/point/page, /image/page)
// and returns the raw list items.
func (c *Client) ListPage(ctx context.Context, path string, page, limit int) ([]json.RawMessage, int, error) {
	q := map[string]string{"page": strconv.Itoa(page), "limit": strconv.Itoa(limit)}
	data, err := c.do(ctx, http.MethodGet, path, q, "", nil, nil)
	if err != nil {
		return nil, 0, err
	}
	var out struct {
		Total int               `json:"total"`
		List  []json.RawMessage `json:"list"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, 0, fmt.Errorf("parsing list page: %w", err)
	}
	return out.List, out.Total, nil
}
