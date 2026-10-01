// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

// Package actual talks directly to an Actual Budget sync server
// (actualbudget/actual packages/sync-server). It covers only the read path the
// CLI needs: password login, listing budget files, fetching file info and
// encryption key metadata, and downloading the budget zip. Writes go through
// the actual-http-api sidecar via the generated client instead.
package actual

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/cliutil"
)

// maxDownloadBytes caps a budget download. Real budgets are a few MB to a few
// hundred MB; the cap guards against a misconfigured URL streaming forever.
const maxDownloadBytes = 2 << 30

// ServerError is a structured failure returned by the sync server, carrying
// its stable reason string (unauthorized, token-expired, file-not-found, ...).
type ServerError struct {
	Status int
	Reason string
	Detail string
	Path   string
}

func (e *ServerError) Error() string {
	msg := fmt.Sprintf("actual server %s: HTTP %d", e.Path, e.Status)
	if e.Reason != "" {
		msg += ": " + e.Reason
	}
	if e.Detail != "" {
		msg += " (" + e.Detail + ")"
	}
	return msg
}

// IsAuth reports whether err is a sync-server authentication failure.
func IsAuth(err error) bool {
	var se *ServerError
	if !errors.As(err, &se) {
		return false
	}
	switch se.Reason {
	case "unauthorized", "token-expired", "invalid-password", "token-not-found":
		return true
	}
	return se.Status == http.StatusUnauthorized || se.Status == http.StatusForbidden
}

// IsNotFound reports whether err means the requested budget file does not exist.
func IsNotFound(err error) bool {
	var se *ServerError
	return errors.As(err, &se) && (se.Reason == "file-not-found" || se.Status == http.StatusNotFound)
}

// Client is a minimal sync-server client.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

// New returns a client for the sync server at baseURL (e.g. http://localhost:5006).
func New(baseURL string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Client{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		HTTP:    &http.Client{Timeout: timeout, CheckRedirect: sameOriginRedirect},
		limiter: cliutil.NewAdaptiveLimiter(5.0),
	}
}

// RemoteFile is one budget file as reported by /sync/list-user-files.
type RemoteFile struct {
	FileID       string `json:"fileId"`
	GroupID      string `json:"groupId"`
	Name         string `json:"name"`
	EncryptKeyID string `json:"encryptKeyId,omitempty"`
	Deleted      int    `json:"deleted"`
	Owner        string `json:"owner,omitempty"`
}

// EncryptMeta describes how a downloaded budget zip was encrypted.
type EncryptMeta struct {
	KeyID     string `json:"keyId"`
	Algorithm string `json:"algorithm"`
	IV        string `json:"iv"`
	AuthTag   string `json:"authTag"`
}

// FileInfo is the /sync/get-user-file-info payload.
type FileInfo struct {
	FileID      string       `json:"fileId"`
	GroupID     string       `json:"groupId"`
	Name        string       `json:"name"`
	Deleted     int          `json:"deleted"`
	EncryptMeta *EncryptMeta `json:"encryptMeta"`
}

// KeyInfo is the /sync/user-get-key payload used to derive the encryption key.
type KeyInfo struct {
	ID   string `json:"id"`
	Salt string `json:"salt"`
	Test string `json:"test"`
}

type envelope struct {
	Status  string          `json:"status"`
	Reason  string          `json:"reason"`
	Details string          `json:"details"`
	Data    json.RawMessage `json:"data"`
}

// do sends one request and returns the raw body for 200 responses. Non-200
// responses are mapped to *ServerError (or *cliutil.RateLimitError on 429).
func (c *Client) do(ctx context.Context, method, path string, headers map[string]string, body any, limit int64) ([]byte, error) {
	if c.BaseURL == "" {
		return nil, errors.New("actual server URL is not set (ACTUAL_SERVER_URL)")
	}
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(buf)
	}
	url := c.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("X-ACTUAL-TOKEN", c.Token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reaching actual server at %s: %w", RedactURL(c.BaseURL), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: RedactURL(url), RetryAfter: cliutil.RetryAfter(resp)}
	}
	if limit <= 0 {
		limit = 16 << 20
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s: response exceeds %d bytes", path, limit)
	}
	if resp.StatusCode != http.StatusOK {
		se := &ServerError{Status: resp.StatusCode, Path: path}
		var env envelope
		if json.Unmarshal(data, &env) == nil {
			se.Reason, se.Detail = env.Reason, env.Details
		}
		return nil, se
	}
	c.limiter.OnSuccess()
	return data, nil
}

func (c *Client) getJSON(ctx context.Context, method, path string, headers map[string]string, body, out any) error {
	data, err := c.do(ctx, method, path, headers, body, 0)
	if err != nil {
		return err
	}
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("%s: unexpected non-JSON response: %w", path, err)
	}
	if env.Status != "ok" {
		return &ServerError{Status: http.StatusOK, Reason: env.Reason, Detail: env.Details, Path: path}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(env.Data, out)
}

// ServerVersion returns the sync server's version from GET /info (no auth).
func (c *Client) ServerVersion(ctx context.Context) (string, error) {
	data, err := c.do(ctx, http.MethodGet, "/info", nil, nil, 0)
	if err != nil {
		return "", err
	}
	var info struct {
		Build struct {
			Version string `json:"version"`
		} `json:"build"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return "", fmt.Errorf("/info: %w", err)
	}
	return info.Build.Version, nil
}

// Login exchanges the server password for a session token and stores it on c.
func (c *Client) Login(ctx context.Context, password string) error {
	if password == "" {
		return errors.New("actual server password is not set (ACTUAL_PASSWORD)")
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := c.getJSON(ctx, http.MethodPost, "/account/login", nil, map[string]string{"loginMethod": "password", "password": password}, &out); err != nil {
		var se *ServerError
		if errors.As(err, &se) && se.Reason == "" && se.Status == http.StatusBadRequest {
			se.Reason = "invalid-password"
		}
		return err
	}
	if out.Token == "" {
		return errors.New("/account/login: server returned no token")
	}
	c.Token = out.Token
	return nil
}

// ListFiles returns the budget files visible to the logged-in user.
func (c *Client) ListFiles(ctx context.Context) ([]RemoteFile, error) {
	files := make([]RemoteFile, 0)
	if err := c.getJSON(ctx, http.MethodGet, "/sync/list-user-files", nil, nil, &files); err != nil {
		return nil, err
	}
	return files, nil
}

// FileInfo returns metadata (including encryption meta) for one budget file.
func (c *Client) FileInfo(ctx context.Context, fileID string) (*FileInfo, error) {
	var info FileInfo
	if err := c.getJSON(ctx, http.MethodGet, "/sync/get-user-file-info", map[string]string{"X-ACTUAL-FILE-ID": fileID}, nil, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// KeyInfo returns the salt and key id needed to derive the encryption key.
func (c *Client) KeyInfo(ctx context.Context, fileID string) (*KeyInfo, error) {
	var k KeyInfo
	if err := c.getJSON(ctx, http.MethodPost, "/sync/user-get-key", nil, map[string]string{"fileId": fileID}, &k); err != nil {
		return nil, err
	}
	return &k, nil
}

// Download returns the raw (possibly encrypted) budget zip bytes.
//
// http.Client.Timeout would also cap reading the body, so a large budget on a
// slow link would fail mid-transfer; the download is bounded by ctx instead.
func (c *Client) Download(ctx context.Context, fileID string) ([]byte, error) {
	hc := *c.HTTP
	hc.Timeout = 0
	dc := *c
	dc.HTTP = &hc
	return dc.do(ctx, http.MethodGet, "/sync/download-user-file", map[string]string{"X-ACTUAL-FILE-ID": fileID}, nil, maxDownloadBytes)
}

// sameOriginRedirect refuses redirects to another scheme or host: Go resends
// custom headers (X-ACTUAL-TOKEN) on redirect, and a 307/308 on login would
// resend the password body.
func sameOriginRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	if prev := via[0].URL; req.URL.Scheme != prev.Scheme || req.URL.Host != prev.Host {
		return fmt.Errorf("refusing redirect from %s to %s://%s", RedactURL(prev.String()), req.URL.Scheme, req.URL.Host)
	}
	return nil
}
