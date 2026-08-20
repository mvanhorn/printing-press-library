// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package granola

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
)

// PATCH(cli-owned-workos-session): OAuth 2.0 device authorization grant against
// Granola's WorkOS deployment. This is how the CLI obtains a session of its
// own, which is the only durable option once the desktop's DEK moved out of
// reach -- see safestorage/testdata/scheme.md for the derived contract.

const (
	// DeviceAuthorizeEndpoint issues a device code.
	DeviceAuthorizeEndpoint = "https://auth.granola.ai/user_management/authorize/device"
	// DeviceTokenEndpoint is polled until the user approves.
	DeviceTokenEndpoint = "https://auth.granola.ai/user_management/authenticate"
	// DeviceGrantType is the RFC 8628 grant type.
	DeviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"
	// GranolaDeviceClientID is Granola's public OAuth client. A public client
	// id is not a secret; it identifies the application, not the user.
	GranolaDeviceClientID = "client_01JZJ0XBDAT8PHJWQY09Y0VD61"
)

// ErrDeviceCodeExpired reports that the user did not approve in time.
var ErrDeviceCodeExpired = errors.New("granola: device code expired before approval")

// ErrDeviceAuthDenied reports that the user declined the request.
var ErrDeviceAuthDenied = errors.New("granola: device authorization denied")

// DeviceCode is the issuing response. UserCode and VerificationURIComplete are
// what the operator acts on.
type DeviceCode struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// deviceTokenResponse is the success envelope from the token endpoint.
type deviceTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
	AuthenticationMethod string `json:"authentication_method"`
}

// deviceHTTPClient is overridable so tests can drive the polling loop against
// an httptest server rather than the live service.
var deviceHTTPClient = &http.Client{Timeout: 30 * time.Second}

// SetDeviceHTTPClient swaps the client used for the device grant.
func SetDeviceHTTPClient(c *http.Client) { deviceHTTPClient = c }

// deviceEndpoints are overridable for the same reason.
var (
	deviceAuthorizeURL = DeviceAuthorizeEndpoint
	deviceTokenURL     = DeviceTokenEndpoint
)

// SetDeviceEndpoints overrides both endpoints. Tests only.
func SetDeviceEndpoints(authorize, token string) {
	deviceAuthorizeURL, deviceTokenURL = authorize, token
}

// RequestDeviceCode starts the grant.
func RequestDeviceCode(ctx context.Context) (DeviceCode, error) {
	form := url.Values{"client_id": {GranolaDeviceClientID}}
	req, err := http.NewRequestWithContext(ctx, "POST", deviceAuthorizeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return DeviceCode{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := deviceHTTPClient.Do(req)
	if err != nil {
		return DeviceCode{}, fmt.Errorf("device authorize: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return DeviceCode{}, fmt.Errorf("device authorize: status %d: %s", resp.StatusCode, truncateBody(string(body), 200))
	}
	var dc DeviceCode
	if err := json.Unmarshal(body, &dc); err != nil {
		return DeviceCode{}, fmt.Errorf("device authorize: parse response: %w", err)
	}
	if dc.DeviceCode == "" || dc.UserCode == "" {
		return DeviceCode{}, fmt.Errorf("device authorize: response missing device_code or user_code")
	}
	if dc.Interval <= 0 {
		dc.Interval = 5
	}
	if dc.ExpiresIn <= 0 {
		dc.ExpiresIn = 300
	}
	return dc, nil
}

// PollDeviceToken polls until the user approves, the code expires, or ctx is
// cancelled.
//
// Per RFC 8628 the two waiting states are not failures: authorization_pending
// means keep going at the current cadence, and slow_down means the same but
// lengthen the interval. Treating either as an error would abandon a grant the
// user is about to approve.
func PollDeviceToken(ctx context.Context, dc DeviceCode) (CLISession, error) {
	interval := time.Duration(dc.Interval) * time.Second
	deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)

	for {
		select {
		case <-ctx.Done():
			return CLISession{}, ctx.Err()
		case <-time.After(interval):
		}
		if time.Now().After(deadline) {
			return CLISession{}, ErrDeviceCodeExpired
		}

		form := url.Values{
			"client_id":   {GranolaDeviceClientID},
			"grant_type":  {DeviceGrantType},
			"device_code": {dc.DeviceCode},
		}
		req, err := http.NewRequestWithContext(ctx, "POST", deviceTokenURL, strings.NewReader(form.Encode()))
		if err != nil {
			return CLISession{}, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := deviceHTTPClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return CLISession{}, ctx.Err()
			}
			return CLISession{}, fmt.Errorf("device token: %w", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var tr deviceTokenResponse
			if err := json.Unmarshal(body, &tr); err != nil {
				return CLISession{}, fmt.Errorf("device token: parse response: %w", err)
			}
			if tr.AccessToken == "" {
				return CLISession{}, fmt.Errorf("device token: response carried no access_token")
			}
			now := time.Now().UTC()
			return CLISession{
				AccessToken:  tr.AccessToken,
				RefreshToken: tr.RefreshToken,
				ObtainedAt:   now,
				AccountID:    tr.User.ID,
				AccountEmail: tr.User.Email,
			}, nil
		}

		switch deviceErrorCode(body) {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5 * time.Second
		case "expired_token":
			return CLISession{}, ErrDeviceCodeExpired
		case "access_denied":
			return CLISession{}, ErrDeviceAuthDenied
		default:
			return CLISession{}, fmt.Errorf("device token: status %d: %s", resp.StatusCode, truncateBody(string(body), 200))
		}
	}
}

func deviceErrorCode(body []byte) string {
	var e struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		return ""
	}
	return e.Error
}

func truncateBody(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
