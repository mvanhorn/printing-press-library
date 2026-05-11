package auth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/goose/internal/cliutil"
)

// cognitoLimiter paces Cognito refresh calls. Cognito enforces 50 req/sec per
// account on InitiateAuth; we start well under that since a CLI run typically
// triggers at most one refresh per command.
var cognitoLimiter = cliutil.NewAdaptiveLimiter(2.0)

const (
	CognitoClientID = "4qv4b8pvtsqigsontd3vfmf6kf"
	CognitoEndpoint = "https://cognito-idp.us-east-2.amazonaws.com/"
)

type RefreshResult struct {
	AccessToken string
	IdToken     string
	ExpiresAt   time.Time
}

type initiateAuthResponse struct {
	AuthenticationResult struct {
		AccessToken string `json:"AccessToken"`
		IdToken     string `json:"IdToken"`
		ExpiresIn   int    `json:"ExpiresIn"`
		TokenType   string `json:"TokenType"`
	} `json:"AuthenticationResult"`
}

type cognitoError struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// Refresh mints a fresh access token from a Cognito refresh token.
// Uses the public client (4qv4b8pvtsqigsontd3vfmf6kf — visible in app.goose.pet's JS bundle).
func Refresh(refreshToken string) (*RefreshResult, error) {
	if refreshToken == "" {
		return nil, fmt.Errorf("auth: refresh token is empty — run `goose-pp-cli auth login` first")
	}

	body := map[string]any{
		"AuthFlow": "REFRESH_TOKEN_AUTH",
		"ClientId": CognitoClientID,
		"AuthParameters": map[string]string{
			"REFRESH_TOKEN": refreshToken,
		},
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("auth: marshalling refresh request: %w", err)
	}

	req, err := http.NewRequest("POST", CognitoEndpoint, bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("auth: building refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "AWSCognitoIdentityProviderService.InitiateAuth")

	cognitoLimiter.Wait()
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth: calling Cognito: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 429 || resp.StatusCode == 503 {
		cognitoLimiter.OnRateLimit()
		retryAfter, _ := time.ParseDuration(resp.Header.Get("Retry-After") + "s")
		return nil, &cliutil.RateLimitError{
			URL:        CognitoEndpoint,
			RetryAfter: retryAfter,
			Body:       string(respBody),
		}
	}
	if resp.StatusCode != 200 {
		var cogErr cognitoError
		if jerr := json.Unmarshal(respBody, &cogErr); jerr == nil && cogErr.Message != "" {
			return nil, fmt.Errorf("auth: Cognito refresh failed: %s (%s)", cogErr.Message, cogErr.Type)
		}
		return nil, fmt.Errorf("auth: Cognito refresh returned %d: %s", resp.StatusCode, string(respBody))
	}
	cognitoLimiter.OnSuccess()

	var ia initiateAuthResponse
	if err := json.Unmarshal(respBody, &ia); err != nil {
		return nil, fmt.Errorf("auth: parsing Cognito response: %w", err)
	}
	if ia.AuthenticationResult.AccessToken == "" {
		return nil, fmt.Errorf("auth: Cognito returned no access token")
	}

	expiry := time.Now().Add(time.Duration(ia.AuthenticationResult.ExpiresIn) * time.Second)
	if ia.AuthenticationResult.ExpiresIn == 0 {
		expiry = time.Now().Add(55 * time.Minute) // safe default
	}

	return &RefreshResult{
		AccessToken: ia.AuthenticationResult.AccessToken,
		IdToken:     ia.AuthenticationResult.IdToken,
		ExpiresAt:   expiry,
	}, nil
}

// ParseJWTClaims decodes the payload section of a JWT without verifying the
// signature. Used to extract the user's cognito:groups (facility memberships)
// and email from an existing access token.
func ParseJWTClaims(jwt string) (map[string]any, error) {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("auth: invalid JWT (need at least 2 segments)")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Try standard-padded base64 too — some Cognito tokens are padded.
		payload, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, fmt.Errorf("auth: decoding JWT payload: %w", err)
		}
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("auth: parsing JWT claims: %w", err)
	}
	return claims, nil
}

// ExtractFacilities pulls facility slugs out of a Cognito access token's
// `cognito:groups` claim. Groups look like "L:<facility-slug>:AA" and
// "A:<facility-slug>:AA"; we collect distinct facility slugs from either prefix.
func ExtractFacilities(jwt string) ([]string, error) {
	claims, err := ParseJWTClaims(jwt)
	if err != nil {
		return nil, err
	}
	raw, ok := claims["cognito:groups"].([]any)
	if !ok {
		return nil, nil
	}
	seen := map[string]bool{}
	out := []string{}
	for _, g := range raw {
		s, ok := g.(string)
		if !ok {
			continue
		}
		parts := strings.SplitN(s, ":", 3)
		if len(parts) < 2 {
			continue
		}
		if parts[0] == "L" || parts[0] == "A" {
			if !seen[parts[1]] {
				seen[parts[1]] = true
				out = append(out, parts[1])
			}
		}
	}
	return out, nil
}

// ExpiryNearOrPast returns true when a stored expiry is within `slack` of
// the current time (or already past). Callers use this to decide whether to
// auto-refresh before a request.
func ExpiryNearOrPast(expiry time.Time, slack time.Duration) bool {
	if expiry.IsZero() {
		return true
	}
	return time.Now().Add(slack).After(expiry)
}
