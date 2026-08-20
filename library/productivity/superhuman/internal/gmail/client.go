// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// Package gmail is the CLI's Gmail API passthrough. Every command that hits
// gmail.googleapis.com — inbox listing, per-message reads, attachment
// download, label list/modify — goes through one Client whose DoWithRefresh
// method handles the 401-refresh-and-retry-once policy.
//
// Why a dedicated package rather than inline calls in internal/cli/:
//
//   - The OAuth access token captured at `auth login --disk` expires ~1h after
//     capture. Without a shared wrapper, every consumer would re-implement the
//     refresh path (or forget to). plan 2026-05-14-003 U1 already paid for
//     this lesson on the send path; U2 generalizes the seam so the next nine
//     parity commands inherit it for free.
//
//   - Tests inject BaseURL to point at an httptest server. The existing
//     internal/client/ package uses the same shape for Superhuman backend
//     mocks; mirroring that pattern keeps the test ergonomics consistent.
package gmail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/auth"
)

// BaseURL is the Gmail API host. Production points at the public endpoint;
// tests override this to point at an httptest server. The package keeps a
// single package-level var rather than threading the URL through every
// helper because callers in internal/cli/ build a Client once per command
// invocation and the URL never changes mid-call.
var BaseURL = "https://gmail.googleapis.com/gmail/v1"

// RefreshFn is the seam used by DoWithRefresh on a 401. Production callers
// install auth.RefreshFromChromeCookies; tests inject a stub. The function
// signature matches that of auth.RefreshFromChromeCookies so the production
// install is a one-liner.
type RefreshFn func(ctx context.Context, email, googleID string) (*auth.CookieAuthResult, error)

// Client is the Gmail API passthrough handle. AccessToken is mutable —
// DoWithRefresh updates it after a successful refresh so subsequent calls
// on the same Client instance reuse the fresh token.
type Client struct {
	Store       *auth.Store
	Email       string
	GoogleID    string
	AccessToken string
	HTTP        *http.Client
	Refresh     RefreshFn

	// Stderr receives the persist-failure warning when store.Upsert fails
	// after a successful refresh. Defaults to io.Discard if unset.
	Stderr io.Writer
}

// New constructs a Client wired to the public Gmail API endpoint and the
// production refresh path. Callers in internal/cli/ resolve the active
// account before constructing the Client.
func New(store *auth.Store, email, googleID, accessToken string) *Client {
	return &Client{
		Store:       store,
		Email:       email,
		GoogleID:    googleID,
		AccessToken: accessToken,
		HTTP:        &http.Client{Timeout: 30 * time.Second},
		Refresh: func(ctx context.Context, email, googleID string) (*auth.CookieAuthResult, error) {
			return auth.RefreshFromChromeCookies(ctx, email, googleID)
		},
	}
}

// APIError is the typed error every typed helper returns on a non-2xx Gmail
// response that isn't 401. (401 is consumed by DoWithRefresh's retry path
// and never surfaces to callers unless the retry also returns 401, at which
// point AuthError is the type returned.)
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("gmail api: HTTP %d: %s", e.Status, e.Body)
}

// AuthError is returned by DoWithRefresh when the refresh path could not
// recover authentication (refresh seam failed, returned an empty access
// token, or the retry also returned 401). Callers in internal/cli/ surface
// the wrapped message so the user sees "run auth login --disk" as the
// remediation step.
type AuthError struct {
	Reason string
	Inner  error
}

func (e *AuthError) Error() string {
	if e.Inner != nil {
		return fmt.Sprintf("gmail unauthorized: %s: %v (run 'auth login --disk')", e.Reason, e.Inner)
	}
	return fmt.Sprintf("gmail unauthorized: %s (run 'auth login --disk')", e.Reason)
}

// DoWithRefresh fires req with the Client's current bearer token. On HTTP
// 401 it refreshes via the seam, persists the new tokens into the store,
// rebuilds the request with the fresh bearer, and retries exactly once.
//
// Semantics:
//   - 2xx -> returns the body bytes; AccessToken updated only if a refresh fired.
//   - 401 once, then 2xx -> returns the second body; AccessToken updated.
//   - 401 + refresh fails -> *AuthError; AccessToken unchanged.
//   - 401 + retry 401 -> *AuthError; AccessToken updated to the (rejected) fresh value
//     so the on-disk store reflects what we tried (the user can still see the
//     state with `auth status`).
//   - Non-401 4xx/5xx -> *APIError.
//   - Network error -> bare error from net/http.
//
// The retry is exactly one — no exponential backoff, no second refresh. A
// runaway loop on a revoked refresh token is the failure mode this gate
// prevents.
func (c *Client) DoWithRefresh(ctx context.Context, req *http.Request, body []byte) ([]byte, error) {
	resp, err := c.do(ctx, req, body, c.AccessToken)
	if err != nil {
		return nil, err
	}
	if resp.status != http.StatusUnauthorized {
		return c.handleStatus(resp)
	}

	// 401: refresh and retry once.
	if c.Refresh == nil {
		return nil, &AuthError{Reason: "no refresh function configured"}
	}
	fresh, refreshErr := c.Refresh(ctx, c.Email, c.GoogleID)
	if refreshErr != nil {
		return nil, &AuthError{Reason: "refresh failed", Inner: refreshErr}
	}
	if fresh == nil || fresh.AccessToken == "" {
		return nil, &AuthError{Reason: "refresh returned no access token"}
	}

	// Persist + update in-memory state.
	c.AccessToken = fresh.AccessToken
	if c.Store != nil {
		newAcct := auth.AccountTokens{
			Type:           "google",
			AccessToken:    fresh.AccessToken,
			UserID:         fresh.GoogleID,
			UserExternalID: fresh.ExternalID,
			DeviceID:       fresh.DeviceID,
			SuperhumanToken: auth.SuperhumanToken{
				Token:   fresh.IDToken,
				Expires: fresh.IDTokenExpires,
			},
			LastUsedAt: time.Now().UnixMilli(),
		}
		if _, perr := c.Store.Upsert(c.Email, newAcct); perr != nil {
			stderr := c.Stderr
			if stderr == nil {
				stderr = io.Discard
			}
			fmt.Fprintf(stderr, "warning: gmail refresh succeeded but token persist failed: %v\n", perr)
		}
	}

	// Retry once with the fresh bearer. Build a new request because req.Body
	// may have been consumed (this is why we cached body []byte upfront).
	retry, err := c.do(ctx, req, body, c.AccessToken)
	if err != nil {
		return nil, err
	}
	if retry.status == http.StatusUnauthorized {
		return nil, &AuthError{Reason: "still unauthorized after refresh"}
	}
	return c.handleStatus(retry)
}

// rawResp captures the parts of an http.Response the DoWithRefresh decision
// table cares about. Kept private — callers only ever see []byte + error.
type rawResp struct {
	status int
	body   []byte
}

// do fires a single HTTP request with the supplied bearer. The body []byte
// is cached so retry can rebuild the same request without re-reading from
// req.Body (which is single-use).
func (c *Client) do(ctx context.Context, req *http.Request, body []byte, bearer string) (*rawResp, error) {
	clone := req.Clone(ctx)
	if body != nil {
		clone.Body = io.NopCloser(bytes.NewReader(body))
		clone.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body)), nil
		}
		clone.ContentLength = int64(len(body))
	}
	clone.Header.Set("Authorization", "Bearer "+bearer)
	if clone.Header.Get("Content-Type") == "" && len(body) > 0 {
		clone.Header.Set("Content-Type", "application/json")
	}

	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(clone)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return &rawResp{status: resp.StatusCode, body: respBody}, nil
}

// handleStatus turns a non-401 response into either the body bytes or an
// *APIError. (401 short-circuits the retry path before reaching here.)
func (c *Client) handleStatus(r *rawResp) ([]byte, error) {
	if r.status >= 200 && r.status < 300 {
		return r.body, nil
	}
	return nil, &APIError{Status: r.status, Body: string(r.body)}
}

// GetJSON is a thin convenience wrapper that builds a GET request, calls
// DoWithRefresh, and unmarshals the response into out. Typed helpers in
// messages.go and labels.go use it to keep their bodies short.
func (c *Client) GetJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, BaseURL+path, nil)
	if err != nil {
		return fmt.Errorf("gmail: build request: %w", err)
	}
	respBody, err := c.DoWithRefresh(ctx, req, nil)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("gmail: decode response: %w (body: %s)", err, truncateForLog(respBody))
	}
	return nil
}

// PostJSON is the symmetric convenience wrapper for state-mutating calls
// (messages.modify, etc.). reqBody is marshaled to JSON; out receives the
// parsed response. Pass out=nil to ignore the response body.
func (c *Client) PostJSON(ctx context.Context, path string, reqBody, out any) error {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("gmail: marshal request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("gmail: build request: %w", err)
	}
	respBody, derr := c.DoWithRefresh(ctx, req, body)
	if derr != nil {
		return derr
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("gmail: decode response: %w (body: %s)", err, truncateForLog(respBody))
	}
	return nil
}

// truncateForLog caps a body excerpt at 256 bytes so error messages stay
// readable in stderr but still surface enough context to debug.
func truncateForLog(b []byte) string {
	const cap = 256
	if len(b) <= cap {
		return string(b)
	}
	return string(b[:cap]) + "...(truncated)"
}

// IsAuth returns true when err is an *AuthError (or wraps one). Callers in
// internal/cli/ use this to route to the exit-code-4 error helper.
func IsAuth(err error) bool {
	var a *AuthError
	return errors.As(err, &a)
}

// IsAPI returns true when err is an *APIError (or wraps one). status will
// be the HTTP status when true, 0 otherwise. Callers use this to surface
// 404 as not-found and other 4xx/5xx as the generic API error.
func IsAPI(err error) (bool, int) {
	var a *APIError
	if errors.As(err, &a) {
		return true, a.Status
	}
	return false, 0
}
