// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PasswordGrant exchanges Zaptec portal username/password for a bearer access
// token using the OAuth2 Resource Owner Password Credentials grant against
// {baseURL}/oauth/token. It returns the access token and its absolute expiry.
//
// Zaptec's API uses ROPC (grant_type=password). The response is form-posted and
// returns JSON with access_token and expires_in. No refresh token is issued, so
// the access token must be re-fetched (via auth login or transparent re-login)
// once it expires.
func PasswordGrant(baseURL, username, password string, httpClient *http.Client) (string, time.Time, error) {
	if username == "" || password == "" {
		return "", time.Time{}, fmt.Errorf("Zaptec username and password are required")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("username", username)
	form.Set("password", password)

	endpoint := strings.TrimRight(baseURL, "/") + "/oauth/token"
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("requesting token from %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized {
			return "", time.Time{}, fmt.Errorf("Zaptec rejected the credentials (HTTP %d). Check ZAPTEC_USERNAME / ZAPTEC_PASSWORD", resp.StatusCode)
		}
		return "", time.Time{}, fmt.Errorf("token endpoint returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tr struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", time.Time{}, fmt.Errorf("parsing token response: %w", err)
	}
	if tr.AccessToken == "" {
		return "", time.Time{}, fmt.Errorf("token response did not contain an access_token")
	}

	expiry := time.Now().Add(24 * time.Hour)
	if tr.ExpiresIn > 0 {
		// Subtract a small skew so we refresh slightly before the hard expiry.
		expiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second).Add(-60 * time.Second)
	}
	return tr.AccessToken, expiry, nil
}
