package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	oauthTokenURL = "https://sanita.puglia.it/sanita-auth/oauth/token"
	// Basic YW9sLWNpZDphb2xAUFdEMjAxOUA= encodes "aol-cid:aol@PWD2019@"
	oauthBasicAuth = "Basic YW9sLWNpZDphb2xAUFdEMjAxOUA="
	// oauthHTTPTimeout caps the token request. Without it the request rides
	// http.DefaultClient (no deadline): a slow or unresponsive auth endpoint
	// would hang every CLI invocation while getAutoToken holds cachedToken.mu.
	oauthHTTPTimeout = 30 * time.Second
)

// oauthHTTPClient is a dedicated, timeout-constrained client for the token
// endpoint, mirroring the bounded http.Client built in client.go.
var oauthHTTPClient = &http.Client{Timeout: oauthHTTPTimeout}

type oauthToken struct {
	value     string
	expiresAt time.Time
	mu        sync.Mutex
}

var cachedToken oauthToken

// fetchClientCredentialsToken obtains a new OAuth2 client_credentials token,
// returning the access token and the server-reported expires_in (seconds).
func fetchClientCredentialsToken() (string, int, error) {
	data := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequest("POST", oauthTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("building oauth request: %w", err)
	}
	req.Header.Set("Authorization", oauthBasicAuth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("oauth token request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", 0, fmt.Errorf("oauth token: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", 0, fmt.Errorf("parsing oauth response: %w", err)
	}
	if result.AccessToken == "" {
		return "", 0, fmt.Errorf("oauth response missing access_token")
	}
	return result.AccessToken, result.ExpiresIn, nil
}

// getAutoToken returns a cached valid token, refreshing it when expired or missing.
func getAutoToken() (string, error) {
	cachedToken.mu.Lock()
	defer cachedToken.mu.Unlock()
	if cachedToken.value != "" && time.Now().Before(cachedToken.expiresAt) {
		return cachedToken.value, nil
	}
	token, expiresIn, err := fetchClientCredentialsToken()
	if err != nil {
		return "", err
	}
	cachedToken.value = token
	// Honor the server-reported expires_in and refresh early so a token is
	// never served stale. Fall back to 1h when the server omits or zeroes
	// expires_in. Cap the early-refresh buffer at half the TTL: without this,
	// a short-lived token (expires_in < 600s) would get ttl-5min as an expiry
	// in the past, forcing a re-fetch on every single request.
	ttl := time.Duration(expiresIn) * time.Second
	if ttl <= 0 {
		ttl = 60 * time.Minute
	}
	buffer := 5 * time.Minute
	if buffer > ttl/2 {
		buffer = ttl / 2
	}
	cachedToken.expiresAt = time.Now().Add(ttl - buffer)
	return token, nil
}
