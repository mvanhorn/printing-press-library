// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

// Package firebase implements the Firebase Authentication exchange Function
// Health uses for member auth. Two operations matter:
//
//   - SignInWithPassword: POST identitytoolkit.googleapis.com/v1/accounts:signInWithPassword
//     trades an email + password for an idToken (1h TTL) and refreshToken.
//   - Refresh: POST securetoken.googleapis.com/v1/token trades a refreshToken
//     for a new idToken before expiry, fixing the API_KEY_INVALID failure
//     daveremy/function-health-mcp issue #22 ships with.
//
// Function Health's Firebase Web API key is public-by-design (it ships in the
// page-bundled SPA on the web; Google's docs are explicit that it is NOT a
// secret), but it is intentionally NOT bundled in this repository. Set
// FUNCTION_HEALTH_FIREBASE_API_KEY to use these flows; the documented
// `auth set-token` path does not need it.
package firebase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/other/function-health/internal/cliutil"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

// APIKey returns the Firebase Web API key from the environment, or "" if unset.
// Function Health's project key is public-by-design
// (https://firebase.google.com/docs/projects/api-keys), but it is intentionally
// NOT bundled in this repository. Set FUNCTION_HEALTH_FIREBASE_API_KEY to use
// the `auth login` / `auth refresh` flows; the documented `auth set-token` path
// (see README) does not need it.
func APIKey() string {
	return os.Getenv("FUNCTION_HEALTH_FIREBASE_API_KEY")
}

// TokenSet is the canonical credential bundle the auth login flow stores.
type TokenSet struct {
	IDToken      string
	RefreshToken string
	Expiry       time.Time
	LocalID      string
	Email        string
}

// Client wraps the Firebase Auth REST endpoints we care about.
type Client struct {
	HTTPClient *http.Client
	APIKey     string
	Limiter    *cliutil.AdaptiveLimiter
}

// NewClient returns a Client wired with a sensible default timeout and an
// adaptive limiter capped at 2 req/s — generous for the at-most-2-call-per-
// session auth flow, but still surfaces a typed *cliutil.RateLimitError when
// Firebase returns 429 after retries are exhausted.
func NewClient() *Client {
	return &Client{
		HTTPClient: &http.Client{Timeout: 20 * time.Second},
		APIKey:     APIKey(),
		Limiter:    cliutil.NewAdaptiveLimiter(2.0),
	}
}

// SignInWithPassword exchanges an email + password for a fresh TokenSet.
// Returns an APIError describing Firebase's structured error body when the
// HTTP status is non-2xx; the caller surfaces the message to the user.
func (c *Client) SignInWithPassword(ctx context.Context, email, password string) (*TokenSet, error) {
	body := map[string]any{
		"email":             email,
		"password":          password,
		"returnSecureToken": true,
	}
	url := "https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=" + c.APIKey
	var resp struct {
		IDToken      string `json:"idToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    string `json:"expiresIn"`
		LocalID      string `json:"localId"`
		Email        string `json:"email"`
	}
	if err := c.post(ctx, url, body, &resp); err != nil {
		return nil, err
	}
	secs, _ := strconv.Atoi(resp.ExpiresIn)
	if secs == 0 {
		secs = 3600
	}
	return &TokenSet{
		IDToken:      resp.IDToken,
		RefreshToken: resp.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(secs) * time.Second),
		LocalID:      resp.LocalID,
		Email:        resp.Email,
	}, nil
}

// Refresh exchanges a refreshToken for a new idToken. Returns a new TokenSet
// inheriting the same Email/LocalID (Firebase doesn't echo those on refresh).
func (c *Client) Refresh(ctx context.Context, refreshToken string) (*TokenSet, error) {
	body := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
	}
	url := "https://securetoken.googleapis.com/v1/token?key=" + c.APIKey
	var resp struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    string `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		UserID       string `json:"user_id"`
	}
	if err := c.post(ctx, url, body, &resp); err != nil {
		return nil, err
	}
	secs, _ := strconv.Atoi(resp.ExpiresIn)
	if secs == 0 {
		secs = 3600
	}
	return &TokenSet{
		IDToken:      resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(secs) * time.Second),
		LocalID:      resp.UserID,
	}, nil
}

func (c *Client) post(ctx context.Context, url string, body any, out any) error {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}
	// Retry once on a 429; surface a typed *cliutil.RateLimitError when the
	// retry also returns 429. This satisfies the source-client-rate-limiting
	// rule even though firebase is at most 2 calls per session in practice.
	const maxAttempts = 2
	var resp *http.Response
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if c.Limiter != nil {
			c.Limiter.Wait()
		}
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
		if err != nil {
			return fmt.Errorf("new request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		// Firebase Web API keys are commonly restricted to the SPA's HTTP
		// referer. Function Health's key blocks Identity Toolkit calls when
		// the Referer header is absent — they require the SPA origin.
		req.Header.Set("Referer", "https://my.functionhealth.com/")
		req.Header.Set("Origin", "https://my.functionhealth.com")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
		req.Header.Set("X-Client-Version", "Chrome/JsCore/12.0.0/FirebaseCore-web")
		r, err := c.HTTPClient.Do(req)
		if err != nil {
			return fmt.Errorf("firebase request: %w", err)
		}
		if r.StatusCode == 429 {
			if c.Limiter != nil {
				c.Limiter.OnRateLimit()
			}
			r.Body.Close()
			if attempt == maxAttempts {
				return &cliutil.RateLimitError{
					URL:        url,
					RetryAfter: time.Second,
					Body:       fmt.Sprintf("firebase rate-limited after %d attempts", attempt),
				}
			}
			continue
		}
		if c.Limiter != nil {
			c.Limiter.OnSuccess()
		}
		resp = r
		break
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		var errResp struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(respBody, &errResp)
		msg := errResp.Error.Message
		if msg == "" {
			msg = string(respBody)
		}
		return &APIError{StatusCode: resp.StatusCode, Message: msg}
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode firebase response: %w", err)
	}
	return nil
}

// APIError carries a non-2xx Firebase response in a typed shape so callers can
// special-case INVALID_PASSWORD, EMAIL_NOT_FOUND, TOKEN_EXPIRED, etc. without
// parsing free-form error strings.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("firebase auth %d: %s", e.StatusCode, e.Message)
}

// IsRefreshable reports whether a SignInWithPassword failure is the kind a
// fresh interactive login could resolve (token revoked, expired, invalidated).
// API_KEY_INVALID is specifically the daveremy/#22 failure mode: the refresh
// token is no longer accepted and the only recovery is a new password login.
func (e *APIError) IsRefreshable() bool {
	switch e.Message {
	case "TOKEN_EXPIRED", "USER_DISABLED", "USER_NOT_FOUND",
		"INVALID_REFRESH_TOKEN", "API_KEY_INVALID":
		return true
	}
	return false
}
