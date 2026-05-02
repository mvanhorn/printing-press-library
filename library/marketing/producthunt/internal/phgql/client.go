package phgql

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/config"
)

// Endpoint is the GraphQL endpoint Product Hunt exposes for v2.
const Endpoint = "https://api.producthunt.com/v2/api/graphql"

// OAuthTokenURL is the token-exchange endpoint for OAuth client_credentials.
const OAuthTokenURL = "https://api.producthunt.com/v2/oauth/token"

// RateLimit summarizes a single response's X-Rate-Limit-* headers.
type RateLimit struct {
	Limit     int `json:"limit"`
	Remaining int `json:"remaining"`
	ResetSecs int `json:"reset_seconds"`
}

// Error is returned when the GraphQL endpoint reports a rejection.
type Error struct {
	HTTPCode  int
	Code      string // e.g. "invalid_oauth_token", "RATE_LIMITED", "GRAPHQL_ERROR"
	Message   string
	Details   any
	RateLimit RateLimit
}

func (e *Error) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return e.Message
}

// Client is a thin GraphQL wrapper that handles auth, rate-limit headers, and
// GraphQL-level error mapping. It composes a generated *config.Config (for
// auth) and the cliutil adaptive limiter (per-source rate-limit cooperation).
type Client struct {
	HTTP     *http.Client
	Cfg      *config.Config
	Limiter  *cliutil.AdaptiveLimiter
	LastRate RateLimit
	DryRun   bool
}

// New builds a GraphQL client around the supplied generated config.
func New(cfg *config.Config) *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		Cfg:     cfg,
		Limiter: cliutil.NewAdaptiveLimiter(4.0), // 4 rps starting floor; PH allows ~6,250 complexity / 15min
	}
}

// Query executes a GraphQL operation, deserializes the `data` field into out,
// and returns the response rate-limit info plus any error. If out is nil the
// caller only cares about side effects (rare).
func (c *Client) Query(ctx context.Context, query string, variables map[string]any, out any) (RateLimit, error) {
	body := map[string]any{"query": query}
	if len(variables) > 0 {
		body["variables"] = variables
	}
	raw, _ := json.Marshal(body)

	if c.DryRun {
		fmt.Println(string(raw))
		return c.LastRate, nil
	}

	header, err := c.bearer()
	if err != nil {
		return RateLimit{}, &Error{Code: "NO_TOKEN", Message: err.Error()}
	}

	if c.Limiter != nil {
		c.Limiter.Wait()
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, Endpoint, bytes.NewReader(raw))
	req.Header.Set("Authorization", header)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "producthunt-pp-cli")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return RateLimit{}, &Error{Code: "TRANSPORT", Message: err.Error()}
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	rate := parseRateLimitHeaders(resp.Header)
	c.LastRate = rate

	if resp.StatusCode == http.StatusTooManyRequests {
		if c.Limiter != nil {
			c.Limiter.OnRateLimit()
		}
		return rate, &cliutil.RateLimitError{URL: Endpoint, RetryAfter: time.Duration(rate.ResetSecs) * time.Second, Body: strings.TrimSpace(string(respBody))}
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		errCode, errMsg := parseAuthError(respBody)
		// If we are in OAuth client_credentials mode, try to refresh the
		// access token once and replay.
		if errCode == "invalid_oauth_token" && c.Cfg != nil && c.Cfg.ClientID != "" && c.Cfg.ClientSecret != "" {
			if newTok, refreshErr := c.refreshAccessToken(ctx); refreshErr == nil {
				c.Cfg.AccessToken = newTok
				c.Cfg.AuthHeaderVal = ""
				return c.Query(ctx, query, variables, out)
			}
		}
		return rate, &Error{HTTPCode: resp.StatusCode, Code: errCode, Message: errMsg, RateLimit: rate}
	}

	if resp.StatusCode != http.StatusOK {
		return rate, &Error{HTTPCode: resp.StatusCode, Code: "HTTP_" + strconv.Itoa(resp.StatusCode), Message: strings.TrimSpace(string(respBody)), RateLimit: rate}
	}

	// Parse GraphQL envelope
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
			Path    []any  `json:"path"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return rate, &Error{Code: "PARSE", Message: err.Error(), RateLimit: rate}
	}
	if len(envelope.Errors) > 0 {
		return rate, &Error{Code: "GRAPHQL_ERROR", Message: envelope.Errors[0].Message, Details: envelope.Errors, RateLimit: rate}
	}
	if c.Limiter != nil {
		c.Limiter.OnSuccess()
	}
	if out != nil && len(envelope.Data) > 0 {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return rate, &Error{Code: "PARSE_DATA", Message: err.Error(), RateLimit: rate}
		}
	}
	return rate, nil
}

// QueryRaw returns the raw `data` JSON without unmarshaling — useful when the
// caller wants to pass through to printJSONFiltered without typing.
func (c *Client) QueryRaw(ctx context.Context, query string, variables map[string]any) (json.RawMessage, RateLimit, error) {
	var raw json.RawMessage
	rate, err := c.Query(ctx, query, variables, &raw)
	return raw, rate, err
}

func (c *Client) bearer() (string, error) {
	if c.Cfg == nil {
		return "", errors.New("no config loaded")
	}
	header := c.Cfg.AuthHeader()
	if header != "" {
		return header, nil
	}
	// Try OAuth client_credentials path if creds present.
	if c.Cfg.ClientID != "" && c.Cfg.ClientSecret != "" {
		tok, err := c.refreshAccessToken(context.Background())
		if err != nil {
			return "", fmt.Errorf("oauth client_credentials: %w", err)
		}
		c.Cfg.AccessToken = tok
		c.Cfg.AuthHeaderVal = ""
		return "Bearer " + tok, nil
	}
	return "", errors.New("PRODUCT_HUNT_TOKEN unset and no OAuth credentials configured — run `producthunt-pp-cli auth onboard`")
}

func (c *Client) refreshAccessToken(ctx context.Context) (string, error) {
	form := strings.NewReader(fmt.Sprintf(
		"client_id=%s&client_secret=%s&grant_type=client_credentials&scope=public",
		c.Cfg.ClientID, c.Cfg.ClientSecret,
	))
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, OAuthTokenURL, form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oauth token: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(respBody, &tok); err != nil {
		return "", err
	}
	if tok.AccessToken == "" {
		return "", errors.New("oauth token: empty access_token in response")
	}
	if tok.ExpiresIn > 0 {
		c.Cfg.TokenExpiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	}
	return tok.AccessToken, nil
}

func parseRateLimitHeaders(h http.Header) RateLimit {
	atoi := func(s string) int {
		v, _ := strconv.Atoi(strings.TrimSpace(s))
		return v
	}
	return RateLimit{
		Limit:     atoi(h.Get("X-Rate-Limit-Limit")),
		Remaining: atoi(h.Get("X-Rate-Limit-Remaining")),
		ResetSecs: atoi(h.Get("X-Rate-Limit-Reset")),
	}
}

func parseAuthError(body []byte) (code, msg string) {
	var env struct {
		Errors []struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		} `json:"errors"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(body, &env)
	if env.Error != "" {
		return env.Error, env.ErrorDescription
	}
	if len(env.Errors) > 0 {
		return env.Errors[0].Error, env.Errors[0].ErrorDescription
	}
	return "AUTH_FAILED", strings.TrimSpace(string(body))
}
