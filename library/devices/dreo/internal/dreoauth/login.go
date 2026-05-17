// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

// Package dreoauth implements the Dreo mobile-app OAuth login flow:
// MD5-hashed password POST against the regional auth host, with a
// transparent retry against the alternate region if the server replies
// with a region disagreement.
package dreoauth

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/dreo/internal/cliutil"
)

// loginLimiter paces auth requests. The Dreo cloud has no documented rate
// limit, but auth endpoints are the cheapest thing to abuse — a slow
// limiter keeps backoff sane during retry storms and surfaces a typed
// RateLimitError when 429s are encountered.
var loginLimiter = cliutil.NewAdaptiveLimiter(2.0)

const (
	defaultClientID     = "7de37c362ee54dcf9c4561812309347a"
	defaultClientSecret = "32dfa0764f25451d99f94e1693498791"
	defaultHimei        = "faede31549d649f58864093158787ec9"
)

// LoginResponse is the subset of the OAuth response we care about.
type LoginResponse struct {
	AccessToken string
	Region      string
	ExpiresIn   int
}

type loginEnvelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		AccessToken string `json:"access_token"`
		Region      string `json:"region"`
		ExpiresIn   int    `json:"expiresIn"`
	} `json:"data"`
}

// Login exchanges a Dreo email/password for an access token. If the
// server reports the account belongs to a different region than the
// supplied baseURL targets, Login retries against the correct region's
// host once. Returns ErrInvalidCredentials on auth failure.
func Login(ctx context.Context, baseURL, username, password string) (*LoginResponse, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("login: username and password required")
	}
	resp, err := loginOnce(ctx, baseURL, username, password)
	if err != nil {
		// Cross-region path: server returns code != 0 with a Region hint
		// when the account belongs to a different region. loginOnce wraps
		// that into (&LoginResponse{Region: region}, error). Retry against
		// the correct host before surfacing the error — without this branch
		// EU users whose client defaults to the US endpoint hit a hard
		// auth failure instead of the intended transparent retry.
		if resp != nil && resp.Region != "" {
			wantRegion := strings.ToUpper(resp.Region)
			currentHost := hostFromBase(baseURL)
			if wantRegion == "EU" && !strings.Contains(currentHost, "-eu.") {
				return loginOnce(ctx, "https://app-api-eu.dreo-tech.com", username, password)
			}
			if wantRegion == "NA" && strings.Contains(currentHost, "-eu.") {
				return loginOnce(ctx, "https://app-api-us.dreo-tech.com", username, password)
			}
		}
		return nil, err
	}
	// Region mismatch on successful response: retry there.
	wantRegion := strings.ToUpper(resp.Region)
	currentHost := hostFromBase(baseURL)
	if wantRegion == "EU" && !strings.Contains(currentHost, "-eu.") {
		return loginOnce(ctx, "https://app-api-eu.dreo-tech.com", username, password)
	}
	if wantRegion == "NA" && strings.Contains(currentHost, "-eu.") {
		return loginOnce(ctx, "https://app-api-us.dreo-tech.com", username, password)
	}
	return resp, nil
}

func loginOnce(ctx context.Context, baseURL, username, password string) (*LoginResponse, error) {
	sum := md5.Sum([]byte(password))
	pwHash := hex.EncodeToString(sum[:])
	body := map[string]any{
		"acceptLanguage": "en",
		"client_id":      defaultClientID,
		"client_secret":  defaultClientSecret,
		"email":          username,
		"encrypt":        "ciphertext",
		"grant_type":     "email-password",
		"himei":          defaultHimei,
		"password":       pwHash,
		"scope":          "all",
	}
	raw, _ := json.Marshal(body)

	ts := time.Now().UnixMilli()
	url := strings.TrimRight(baseURL, "/") + fmt.Sprintf("/api/oauth/login?timestamp=%d", ts)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("login: build request: %w", err)
	}
	req.Header.Set("ua", "dreo/2.8.2")
	req.Header.Set("lang", "en")
	req.Header.Set("content-type", "application/json; charset=UTF-8")
	req.Header.Set("User-Agent", "okhttp/4.9.1")
	req.Header.Set("Accept-Encoding", "identity") // avoid gzip framing in this transport

	// Pace via the shared limiter. nil-receiver Wait() is a no-op, so
	// tests that hit httptest servers stay fast.
	loginLimiter.Wait()

	client := &http.Client{Timeout: 30 * time.Second}
	httpResp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("login: POST: %w", err)
	}
	defer httpResp.Body.Close()

	// Typed RateLimitError on 429 so callers can distinguish throttling
	// from auth failure. The limiter discovers the new ceiling on its own
	// on the next call.
	if httpResp.StatusCode == http.StatusTooManyRequests {
		retryAfter := parseRetryAfter(httpResp.Header.Get("Retry-After"))
		return nil, &cliutil.RateLimitError{
			URL:        url,
			RetryAfter: retryAfter,
			Body:       "Dreo auth endpoint throttled",
		}
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("login: read response: %w", err)
	}

	var env loginEnvelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return nil, fmt.Errorf("login: parse response (status %d): %w; body=%s", httpResp.StatusCode, err, truncate(respBody, 256))
	}
	// Dreo signals success with code==0; non-zero is an auth or input failure.
	if env.Code != 0 && env.Data.AccessToken == "" {
		// If server returned a region hint, surface it as the Region field.
		region := strings.ToUpper(env.Data.Region)
		if region != "" {
			return &LoginResponse{Region: region}, fmt.Errorf("login: code=%d msg=%q", env.Code, env.Msg)
		}
		return nil, fmt.Errorf("login: code=%d msg=%q", env.Code, env.Msg)
	}
	return &LoginResponse{
		AccessToken: env.Data.AccessToken,
		Region:      strings.ToUpper(env.Data.Region),
		ExpiresIn:   env.Data.ExpiresIn,
	}, nil
}

func hostFromBase(base string) string {
	s := strings.TrimPrefix(base, "https://")
	s = strings.TrimPrefix(s, "http://")
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx]
	}
	return s
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	// Retry-After can be seconds (integer). HTTP-date is rare here.
	var secs int64
	if _, err := fmt.Sscanf(v, "%d", &secs); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
}
