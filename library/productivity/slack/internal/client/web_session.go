// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// PreferUserToken returns a shallow clone of c whose Config prefers
// SlackUserToken (xoxp-) over SlackBotToken (xoxb-) whenever a user token is
// configured. Slack's search.* endpoints reject bot tokens with
// not_allowed_token_type.
func (c *Client) PreferUserToken() *Client {
	if c == nil || c.Config == nil || strings.TrimSpace(c.Config.SlackUserToken) == "" {
		return c
	}
	cfgCopy := *c.Config
	cfgCopy.SlackBotToken = ""
	if cfgCopy.AuthHeaderVal == "" || strings.HasPrefix(strings.TrimSpace(cfgCopy.AuthHeaderVal), "Bearer xoxb-") {
		cfgCopy.AuthHeaderVal = ""
	}
	cfgCopy.AuthSource = "env:SLACK_USER_TOKEN"
	clientCopy := *c
	clientCopy.Config = &cfgCopy
	return &clientCopy
}

// WebSessionErrorKind classifies failures from Slack desktop/web internal RPC
// calls so CLI callers can map them to typed exit codes (4=auth, 7=rate limit,
// 5=API error).
type WebSessionErrorKind int

const (
	WebSessionErrAPI WebSessionErrorKind = iota
	WebSessionErrAuth
	WebSessionErrRateLimit
)

// WebSessionError is a structured error returned by PostWebSession.
type WebSessionError struct {
	Kind    WebSessionErrorKind
	Code    string
	Message string
	Err     error
}

func (e *WebSessionError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "slack web-client request failed"
}

func (e *WebSessionError) Unwrap() error {
	return e.Err
}

// IsSlackWebSessionAuthError reports whether a Slack HTTP-200 {"ok":false}
// error code indicates missing, expired, revoked, or rejected session
// credentials.
func IsSlackWebSessionAuthError(code string) bool {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "invalid_auth",
		"not_authed",
		"token_revoked",
		"token_expired",
		"account_inactive",
		"not_allowed_token_type",
		"invalid_auth_cookie",
		"missing_auth_cookie",
		"invalid_cookie",
		"invalid_refresh_token",
		"session_expired",
		"user_disabled",
		"ekm_access_denied":
		return true
	default:
		return false
	}
}

// SlackWebCookieHeader resolves the Slack `d` session cookie without ever
// printing or persisting cookie values. Order:
//  1. SLACK_COOKIE_D env var (either bare xoxd-... value or "d=..." pair)
//  2. `agentcookie cookies --domain .slack.com` CLI if installed on PATH
func SlackWebCookieHeader(ctx context.Context) (string, error) {
	if raw := strings.TrimSpace(os.Getenv("SLACK_COOKIE_D")); raw != "" {
		if strings.HasPrefix(raw, "d=") {
			return raw, nil
		}
		return "d=" + raw, nil
	}
	bin, err := exec.LookPath("agentcookie")
	if err != nil {
		return "", &WebSessionError{
			Kind:    WebSessionErrAuth,
			Message: "missing Slack session cookie: set SLACK_COOKIE_D or install agentcookie with a logged-in .slack.com session",
		}
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, bin, "cookies", "--domain", ".slack.com").Output()
	if err != nil {
		return "", &WebSessionError{
			Kind:    WebSessionErrAuth,
			Message: fmt.Sprintf("failed to resolve .slack.com cookies via agentcookie: %v", err),
			Err:     err,
		}
	}
	header := strings.TrimSpace(string(out))
	if header == "" {
		return "", &WebSessionError{
			Kind:    WebSessionErrAuth,
			Message: "agentcookie returned no active .slack.com cookies; sign into Slack in Chrome or set SLACK_COOKIE_D",
		}
	}
	return header, nil
}

// SlackWebSessionToken reads SLACK_SESSION_TOKEN (xoxc-...).
func SlackWebSessionToken() (string, error) {
	tok := strings.TrimSpace(os.Getenv("SLACK_SESSION_TOKEN"))
	if tok == "" {
		return "", &WebSessionError{
			Kind:    WebSessionErrAuth,
			Message: "SLACK_SESSION_TOKEN (xoxc-...) is required for web-client internal endpoints",
		}
	}
	return tok, nil
}

// SlackWebBaseURL returns SLACK_BASE_URL or https://slack.com/api.
func SlackWebBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("SLACK_BASE_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://slack.com/api"
}

// PostWebSession invokes a read-only Slack desktop/web RPC method using the
// xoxc session token paired with Slack's d session cookie. It classifies both
// HTTP-level auth failures and HTTP-200 {"ok":false} session-auth errors as
// WebSessionErrAuth so callers can exit with code 4 or prompt re-login.
func PostWebSession(ctx context.Context, path string, body map[string]any, timeout time.Duration) (json.RawMessage, error) {
	token, err := SlackWebSessionToken()
	if err != nil {
		return nil, err
	}
	cookieHeader, err := SlackWebCookieHeader(ctx)
	if err != nil {
		return nil, err
	}

	if body == nil {
		body = map[string]any{}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, &WebSessionError{
			Kind:    WebSessionErrAPI,
			Message: fmt.Sprintf("marshal request body: %v", err),
			Err:     err,
		}
	}

	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, SlackWebBaseURL()+path, bytes.NewReader(payload))
	if err != nil {
		return nil, &WebSessionError{
			Kind:    WebSessionErrAPI,
			Message: fmt.Sprintf("build slack web-client %s request: %v", path, err),
			Err:     err,
		}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return nil, &WebSessionError{
			Kind:    WebSessionErrAPI,
			Message: fmt.Sprintf("slack web-client %s: %v", path, err),
			Err:     err,
		}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		return nil, &WebSessionError{
			Kind:    WebSessionErrAPI,
			Message: fmt.Sprintf("reading slack web-client %s response: %v", path, err),
			Err:     err,
		}
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, &WebSessionError{
			Kind:    WebSessionErrAuth,
			Message: fmt.Sprintf("slack web-client %s returned HTTP %d", path, resp.StatusCode),
		}
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, &WebSessionError{
			Kind:    WebSessionErrRateLimit,
			Message: fmt.Sprintf("slack web-client %s rate limited (HTTP 429)", path),
		}
	}
	if resp.StatusCode >= 400 {
		return nil, &WebSessionError{
			Kind:    WebSessionErrAPI,
			Message: fmt.Sprintf("slack web-client %s returned HTTP %d", path, resp.StatusCode),
		}
	}

	var envelope struct {
		OK     bool   `json:"ok"`
		Error  string `json:"error"`
		Needed string `json:"needed"`
	}
	if json.Unmarshal(raw, &envelope) == nil && !envelope.OK && envelope.Error != "" {
		if IsSlackWebSessionAuthError(envelope.Error) {
			return nil, &WebSessionError{
				Kind:    WebSessionErrAuth,
				Code:    envelope.Error,
				Message: fmt.Sprintf("slack web-client %s session auth error: %s (refresh SLACK_SESSION_TOKEN and Slack browser session)", path, envelope.Error),
			}
		}
		if strings.EqualFold(envelope.Error, "ratelimited") {
			return nil, &WebSessionError{
				Kind:    WebSessionErrRateLimit,
				Code:    envelope.Error,
				Message: fmt.Sprintf("slack web-client %s rate limited: %s", path, envelope.Error),
			}
		}
		if envelope.Error == "missing_scope" && envelope.Needed != "" {
			return nil, &WebSessionError{
				Kind:    WebSessionErrAPI,
				Code:    envelope.Error,
				Message: fmt.Sprintf("missing Slack scope: %s", envelope.Needed),
			}
		}
		return nil, &WebSessionError{
			Kind:    WebSessionErrAPI,
			Code:    envelope.Error,
			Message: fmt.Sprintf("Slack API error: %s", envelope.Error),
		}
	}

	return json.RawMessage(raw), nil
}
