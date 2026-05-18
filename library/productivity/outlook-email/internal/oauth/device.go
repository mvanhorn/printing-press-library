// Package oauth implements the Microsoft Identity OAuth 2.0 device authorization
// grant flow for personal Microsoft 365 accounts. The flow targets the /common
// authority so personal Microsoft accounts (Outlook.com, Hotmail, Live, MSA)
// authenticate alongside work/school accounts.
package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/outlook-email/internal/cliutil"
)

// DefaultAuthority is the multi-tenant authority that accepts both work/school
// and personal Microsoft accounts. The /common path is required for personal
// MSAs; tenant-specific authorities reject personal accounts.
const DefaultAuthority = "https://login.microsoftonline.com/common"

// DefaultClientID is the Microsoft-published Graph PowerShell client. It is a
// public client (no secret) and is configured for AzureADandPersonalMicrosoftAccount
// supported account types, which means personal MSAs work out of the box.
// Users can override via OUTLOOK_EMAIL_CLIENT_ID after registering their
// own Azure AD app.
const DefaultClientID = "14d82eec-204b-4c2f-b7e8-296a70dab67e"

// DefaultScopes are the minimum scopes for read/write calendar access plus
// refresh-token rotation. offline_access is what causes Azure to return a
// refresh_token alongside the access_token.
var DefaultScopes = []string{
	"Mail.ReadWrite",
	"Mail.Send",
	"MailboxSettings.ReadWrite",
	"User.Read",
	"offline_access",
}

// DeviceCodeResponse mirrors the device authorization endpoint response.
type DeviceCodeResponse struct {
	UserCode        string `json:"user_code"`
	DeviceCode      string `json:"device_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	Message         string `json:"message"`
}

// TokenResponse mirrors the OAuth 2.0 token endpoint success body. Microsoft
// returns expires_in as a number of seconds; ExpiryTime is computed at parse.
type TokenResponse struct {
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token,omitempty"`
	ExpiryTime   time.Time
}

// errorResponse mirrors Microsoft's OAuth error envelope.
type errorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// Client encapsulates a configured OAuth client. Authority and ClientID can be
// overridden for testing or BYO-app-registration use cases.
type Client struct {
	Authority  string
	ClientID   string
	Scopes     []string
	HTTPClient *http.Client
	limiter    *cliutil.AdaptiveLimiter
}

// NewClient returns a Client with sensible defaults. Pass empty strings to
// fall back to the Microsoft-published PowerShell client and the /common
// authority.
func NewClient(authority, clientID string, scopes []string) *Client {
	if authority == "" {
		authority = DefaultAuthority
	}
	if clientID == "" {
		clientID = DefaultClientID
	}
	if len(scopes) == 0 {
		scopes = DefaultScopes
	}
	return &Client{
		Authority:  strings.TrimRight(authority, "/"),
		ClientID:   clientID,
		Scopes:     scopes,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		// 2 req/s is well within Microsoft Identity's published quotas;
		// the limiter also surfaces typed *cliutil.RateLimitError on 429
		// so callers can distinguish "we got throttled" from "auth failed".
		limiter: cliutil.NewAdaptiveLimiter(2.0),
	}
}

// RequestDeviceCode initiates the device authorization flow and returns the
// user-facing code + verification URL. The caller is expected to display
// these to the user and then call PollToken with the returned device_code.
func (c *Client) RequestDeviceCode(ctx context.Context) (*DeviceCodeResponse, error) {
	form := url.Values{}
	form.Set("client_id", c.ClientID)
	form.Set("scope", strings.Join(c.Scopes, " "))

	endpoint := c.Authority + "/oauth2/v2.0/devicecode"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("building devicecode request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	if c.limiter != nil {
		c.limiter.Wait()
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting device code: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusTooManyRequests {
		if c.limiter != nil {
			c.limiter.OnRateLimit()
		}
		retry := time.Duration(0)
		if v := resp.Header.Get("Retry-After"); v != "" {
			if n, perr := time.ParseDuration(v + "s"); perr == nil {
				retry = n
			}
		}
		return nil, &cliutil.RateLimitError{URL: endpoint, RetryAfter: retry, Body: string(body)}
	}
	if c.limiter != nil {
		c.limiter.OnSuccess()
	}

	if resp.StatusCode/100 != 2 {
		var er errorResponse
		_ = json.Unmarshal(body, &er)
		if er.Error != "" {
			return nil, fmt.Errorf("device code endpoint returned %s: %s", er.Error, er.ErrorDescription)
		}
		return nil, fmt.Errorf("device code endpoint returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var out DeviceCodeResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parsing device code response: %w", err)
	}
	if out.Interval <= 0 {
		out.Interval = 5
	}
	return &out, nil
}

// PollToken polls the token endpoint until the user completes authorization,
// the device code expires, or the context is cancelled. Polling cadence is
// dictated by the server's `interval` plus any `slow_down` responses.
func (c *Client) PollToken(ctx context.Context, device *DeviceCodeResponse) (*TokenResponse, error) {
	deadline := time.Now().Add(time.Duration(device.ExpiresIn) * time.Second)
	interval := time.Duration(device.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}

	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	form.Set("client_id", c.ClientID)
	form.Set("device_code", device.DeviceCode)

	endpoint := c.Authority + "/oauth2/v2.0/token"

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}

		if time.Now().After(deadline) {
			return nil, errors.New("device code expired before user completed authorization")
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, fmt.Errorf("building token request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		if c.limiter != nil {
			c.limiter.Wait()
		}
		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("polling token: %w", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			if c.limiter != nil {
				c.limiter.OnRateLimit()
			}
			retry := time.Duration(0)
			if v := resp.Header.Get("Retry-After"); v != "" {
				if n, perr := time.ParseDuration(v + "s"); perr == nil {
					retry = n
				}
			}
			return nil, &cliutil.RateLimitError{URL: endpoint, RetryAfter: retry, Body: string(body)}
		}

		if resp.StatusCode/100 == 2 {
			if c.limiter != nil {
				c.limiter.OnSuccess()
			}
			var tr TokenResponse
			if err := json.Unmarshal(body, &tr); err != nil {
				return nil, fmt.Errorf("parsing token response: %w", err)
			}
			tr.ExpiryTime = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
			return &tr, nil
		}

		var er errorResponse
		_ = json.Unmarshal(body, &er)
		switch er.Error {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5 * time.Second
			continue
		case "expired_token":
			return nil, errors.New("device code expired before user completed authorization")
		case "authorization_declined":
			return nil, errors.New("user declined authorization")
		case "bad_verification_code":
			return nil, errors.New("device code not recognised by the authority")
		case "":
			return nil, fmt.Errorf("token endpoint returned HTTP %d: %s", resp.StatusCode, string(body))
		default:
			return nil, fmt.Errorf("token endpoint error %s: %s", er.Error, er.ErrorDescription)
		}
	}
}

// RefreshToken exchanges a refresh token for a new access+refresh token pair.
// Microsoft rotates refresh tokens, so the caller must persist the new pair.
func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	if refreshToken == "" {
		return nil, errors.New("no refresh token available; run 'auth login --device-code' first")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", c.ClientID)
	form.Set("refresh_token", refreshToken)
	form.Set("scope", strings.Join(c.Scopes, " "))

	endpoint := c.Authority + "/oauth2/v2.0/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("building refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	if c.limiter != nil {
		c.limiter.Wait()
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refreshing token: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusTooManyRequests {
		if c.limiter != nil {
			c.limiter.OnRateLimit()
		}
		retry := time.Duration(0)
		if v := resp.Header.Get("Retry-After"); v != "" {
			if n, perr := time.ParseDuration(v + "s"); perr == nil {
				retry = n
			}
		}
		return nil, &cliutil.RateLimitError{URL: endpoint, RetryAfter: retry, Body: string(body)}
	}
	if c.limiter != nil {
		c.limiter.OnSuccess()
	}

	if resp.StatusCode/100 != 2 {
		var er errorResponse
		_ = json.Unmarshal(body, &er)
		if er.Error != "" {
			return nil, fmt.Errorf("refresh failed (%s): %s", er.Error, er.ErrorDescription)
		}
		return nil, fmt.Errorf("refresh returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var tr TokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("parsing refresh response: %w", err)
	}
	tr.ExpiryTime = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	// Microsoft rotates refresh tokens; if the response omits one, reuse the
	// caller-supplied token so callers can save unconditionally.
	if tr.RefreshToken == "" {
		tr.RefreshToken = refreshToken
	}
	return &tr, nil
}
