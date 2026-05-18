// Copyright 2026 greatclips. Licensed under Apache-2.0. See LICENSE.

package auth0silent

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Auth0 tenant configuration discovered from the SPA's JS bundle
// chunks (01000ffd9a85230f.js) on 2026-05-11.
const (
	// CIDDomain is the Auth0 tenant host backing GreatClips customer
	// identity. /authorize and /oauth/token live here.
	CIDDomain = "cid.greatclips.com"
	// ClientID is the Auth0 client_id from the SPA's auth0.config.
	// Rotates when GreatClips reissues the SPA; if mint starts
	// returning invalid_client, re-discover this from the JS bundle.
	ClientID = "eq2A3lIn48Afym7azte124bPd7iSoaIZ"
	// SPAOrigin is the redirect_uri the SPA registers with Auth0.
	// Auth0 validates this matches a configured callback URL.
	SPAOrigin = "https://app.greatclips.com"
	// Scope is the OpenID scope set the SPA requests at sign-in.
	Scope = "openid profile email"
)

// Token is one minted access token for a specific audience.
type Token struct {
	AccessToken string
	Audience    string
	ExpiresAt   time.Time
}

// LoginRequiredError indicates Auth0 rejected the silent-auth call
// because the user's session cookies are no longer valid. The user
// needs to log in at https://app.greatclips.com via a browser, then
// re-run `auth login --chrome` to refresh stored cookies.
type LoginRequiredError struct {
	Detail string
}

func (e *LoginRequiredError) Error() string {
	return fmt.Sprintf("auth0 login_required: %s — log in at %s and re-extract cookies", e.Detail, SPAOrigin)
}

// Mint executes the Auth0 silent-auth flow for one audience. Returns
// a Token on success. Returns a *LoginRequiredError if the user's
// cookies are stale (cookies still present client-side but Auth0's
// server-side session has expired).
//
// Implementation: GET /authorize with prompt=none. Auth0 responds
// with a 302 to redirect_uri with the access token in the URL
// fragment (or an error= parameter on failure).
func Mint(audience string, cookies map[string]string) (*Token, error) {
	state, err := randomURLSafe(16)
	if err != nil {
		return nil, fmt.Errorf("generating state: %w", err)
	}
	nonce, err := randomURLSafe(16)
	if err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	q := url.Values{}
	q.Set("client_id", ClientID)
	q.Set("audience", audience)
	q.Set("prompt", "none")
	q.Set("response_type", "token")
	q.Set("redirect_uri", SPAOrigin)
	q.Set("scope", Scope)
	q.Set("state", state)
	q.Set("nonce", nonce)

	authURL := "https://" + CIDDomain + "/authorize?" + q.Encode()
	req, err := http.NewRequest("GET", authURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", CookieHeader(cookies))
	// Auth0 validates Origin/Referer on silent calls.
	req.Header.Set("Origin", SPAOrigin)
	req.Header.Set("Referer", SPAOrigin+"/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36")

	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth0 /authorize call: %w", err)
	}
	defer resp.Body.Close()

	loc := resp.Header.Get("Location")
	if loc == "" {
		return nil, fmt.Errorf("auth0 returned HTTP %d with no Location header (silent auth likely needs a real login)", resp.StatusCode)
	}

	// Parse the fragment for either access_token=... or error=...
	hashIdx := strings.Index(loc, "#")
	if hashIdx < 0 {
		// No fragment — could be a query-string error response.
		u, perr := url.Parse(loc)
		if perr == nil {
			if errCode := u.Query().Get("error"); errCode != "" {
				return nil, classifyAuth0Error(errCode, u.Query().Get("error_description"))
			}
		}
		return nil, fmt.Errorf("auth0 Location has no fragment: %s", loc)
	}
	fragment := loc[hashIdx+1:]
	frag, err := url.ParseQuery(fragment)
	if err != nil {
		return nil, fmt.Errorf("parsing auth0 fragment: %w", err)
	}
	if errCode := frag.Get("error"); errCode != "" {
		return nil, classifyAuth0Error(errCode, frag.Get("error_description"))
	}
	// PATCH(oauth-state-roundtrip): verify Auth0 echoed back the state we
	// sent. Even on a CLI without a browser-redirect attack surface,
	// validating state catches a tampered or replayed Location header
	// before we accept the token (greptile P2).
	if got := frag.Get("state"); got != state {
		return nil, fmt.Errorf("auth0 state mismatch: got %q, want %q", got, state)
	}
	accessToken := frag.Get("access_token")
	if accessToken == "" {
		return nil, fmt.Errorf("no access_token in auth0 fragment: %s", fragment)
	}

	exp, err := parseJWTExp(accessToken)
	if err != nil {
		return nil, fmt.Errorf("parsing token exp: %w", err)
	}
	return &Token{
		AccessToken: accessToken,
		Audience:    audience,
		ExpiresAt:   exp,
	}, nil
}

func classifyAuth0Error(code, desc string) error {
	switch code {
	case "login_required", "interaction_required", "consent_required":
		return &LoginRequiredError{Detail: code + ": " + desc}
	}
	return fmt.Errorf("auth0 error %s: %s", code, desc)
}

func randomURLSafe(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// parseJWTExp pulls the `exp` claim out of a JWT's payload segment
// without validating the signature. We never validate — the token is
// a Bearer we forward to the API; the API server validates it.
func parseJWTExp(jwt string) (time.Time, error) {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return time.Time{}, errors.New("not a JWT (expected 3 dot-delimited parts)")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Some tokens use standard padding-stripped base64
		payload, err = base64.RawStdEncoding.DecodeString(parts[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("decoding payload: %w", err)
		}
	}
	var claims struct {
		Exp int64 `json:"exp"`
		Aud any   `json:"aud"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}, fmt.Errorf("unmarshaling payload: %w", err)
	}
	if claims.Exp == 0 {
		return time.Time{}, errors.New("no exp claim in jwt payload")
	}
	return time.Unix(claims.Exp, 0), nil
}
