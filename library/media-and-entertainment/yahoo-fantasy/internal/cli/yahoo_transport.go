package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/yahoofantasy"
)

func init() {
	registerClientHook(installYahooTransport)
}

func installYahooTransport(c *client.Client) error {
	if c == nil || c.HTTPClient == nil {
		return nil
	}
	base := c.HTTPClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	if _, already := base.(*yahooRoundTripper); already {
		return nil
	}
	c.HTTPClient.Transport = &yahooRoundTripper{
		base:   base,
		client: c,
	}
	return nil
}

type yahooRoundTripper struct {
	base            http.RoundTripper
	client          *client.Client
	mu              sync.Mutex
	refreshedBearer string
	refreshedExpiry time.Time
}

func (rt *yahooRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())

	// 1. If GET request has no format query param and targets Yahoo Fantasy v2, default format=json.
	if cloned.Method == http.MethodGet && cloned.URL != nil {
		q := cloned.URL.Query()
		if q.Get("format") == "" && strings.Contains(cloned.URL.Path, "/") {
			q.Set("format", "json")
			cloned.URL.RawQuery = cliutil.EncodeQuery(q)
		}
	}

	// 2. If PUT/POST mutation carries JSON for a Yahoo XML write endpoint, convert to <fantasy_content> XML.
	var bodyBytes []byte
	if cloned.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(cloned.Body)
		_ = cloned.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading outgoing request body: %w", err)
		}
		xmlBytes, converted, convErr := yahoofantasy.TranslateJSONMutationToXML(cloned.Method, cloned.URL.Path, bodyBytes)
		if convErr != nil {
			return nil, convErr
		}
		if converted {
			bodyBytes = xmlBytes
			cloned.Header.Set("Content-Type", "application/xml")
		} else if bytes.HasPrefix(bytes.TrimSpace(bodyBytes), []byte("<?xml")) || bytes.HasPrefix(bytes.TrimSpace(bodyBytes), []byte("<fantasy_content")) {
			cloned.Header.Set("Content-Type", "application/xml")
		}
		cloned.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		cloned.ContentLength = int64(len(bodyBytes))
	}

	// 3. Attach refreshed OAuth2 Bearer token if no static token is set (or stored token is expired) and refresh credentials exist.
	tokenExpired := rt.client != nil && rt.client.Config != nil && !rt.client.Config.TokenExpiry.IsZero() && time.Now().After(rt.client.Config.TokenExpiry)
	if cloned.Header.Get("Authorization") == "" || tokenExpired {
		if bearer, err := rt.resolveRefreshedBearer(cloned); err == nil && bearer != "" {
			cloned.Header.Set("Authorization", "Bearer "+bearer)
		}
	}

	resp, err := rt.base.RoundTrip(cloned)
	if err != nil {
		return nil, err
	}

	// 4. Retry once on 401 if refresh credentials are configured.
	if resp.StatusCode == http.StatusUnauthorized && rt.hasRefreshCredentials() {
		if bearer, refErr := rt.forceRefreshBearer(cloned); refErr == nil && bearer != "" {
			_ = resp.Body.Close()
			retryReq := req.Clone(req.Context())
			if bodyBytes != nil {
				retryReq.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				retryReq.ContentLength = int64(len(bodyBytes))
				retryReq.Header.Set("Content-Type", cloned.Header.Get("Content-Type"))
			}
			retryReq.Header.Set("Authorization", "Bearer "+bearer)
			resp, err = rt.base.RoundTrip(retryReq)
			if err != nil {
				return nil, err
			}
		}
	}

	// 5. Normalize XML response bodies into JSON so downstream JSON consumers succeed.
	if resp.Body != nil {
		rawResp, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		normalized := yahoofantasy.NormalizeYahooResponse(rawResp)
		if !bytes.Equal(normalized, rawResp) {
			resp.Header.Set("Content-Type", "application/json")
		}
		resp.Body = io.NopCloser(bytes.NewReader(normalized))
		resp.ContentLength = int64(len(normalized))
	}
	return resp, nil
}

func (rt *yahooRoundTripper) hasRefreshCredentials() bool {
	cid, csec, rtok := rt.refreshCredentials()
	acode := strings.TrimSpace(cliutil.EnvOverride("YAHOO_AUTH_CODE"))
	return cid != "" && csec != "" && (rtok != "" || acode != "")
}

func (rt *yahooRoundTripper) refreshCredentials() (clientID, clientSecret, refreshToken string) {
	clientID = strings.TrimSpace(cliutil.EnvOverride("YAHOO_CLIENT_ID"))
	clientSecret = strings.TrimSpace(cliutil.EnvOverride("YAHOO_CLIENT_SECRET"))
	refreshToken = strings.TrimSpace(cliutil.EnvOverride("YAHOO_REFRESH_TOKEN"))
	if rt.client != nil && rt.client.Config != nil {
		if clientID == "" {
			clientID = strings.TrimSpace(rt.client.Config.ClientID)
		}
		if clientSecret == "" {
			clientSecret = strings.TrimSpace(rt.client.Config.ClientSecret)
		}
		if refreshToken == "" {
			refreshToken = strings.TrimSpace(rt.client.Config.RefreshToken)
		}
	}
	return clientID, clientSecret, refreshToken
}

func (rt *yahooRoundTripper) resolveRefreshedBearer(req *http.Request) (string, error) {
	rt.mu.Lock()
	if rt.refreshedBearer != "" && time.Now().Before(rt.refreshedExpiry) {
		tok := rt.refreshedBearer
		rt.mu.Unlock()
		return tok, nil
	}
	rt.mu.Unlock()
	if !rt.hasRefreshCredentials() {
		return "", nil
	}
	return rt.forceRefreshBearer(req)
}

func (rt *yahooRoundTripper) forceRefreshBearer(req *http.Request) (string, error) {
	clientID, clientSecret, refreshToken := rt.refreshCredentials()
	authCode := strings.TrimSpace(cliutil.EnvOverride("YAHOO_AUTH_CODE"))
	if clientID == "" || clientSecret == "" || (refreshToken == "" && authCode == "") {
		return "", nil
	}
	tokenURL := strings.TrimSpace(cliutil.EnvOverride("YAHOO_OAUTH_TOKEN_URL"))
	if tokenURL == "" {
		tokenURL = "https://api.login.yahoo.com/oauth2/get_token" // #nosec G101 -- public Yahoo OAuth2 token endpoint URL, not a credential.
	}

	form := url.Values{}
	if refreshToken != "" {
		form.Set("grant_type", "refresh_token")
		form.Set("refresh_token", refreshToken)
	} else {
		redirectURI := strings.TrimSpace(cliutil.EnvOverride("YAHOO_REDIRECT_URI"))
		if redirectURI == "" {
			redirectURI = "oob"
		}
		form.Set("grant_type", "authorization_code")
		form.Set("code", authCode)
		form.Set("redirect_uri", redirectURI)
	}
	encodedForm := form.Encode()

	tokenReq, err := http.NewRequestWithContext(req.Context(), http.MethodPost, tokenURL, strings.NewReader(encodedForm))
	if err != nil {
		return "", err
	}
	basic := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
	tokenReq.Header.Set("Authorization", "Basic "+basic)
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenReq.Header.Set("Accept", "application/json")

	resp, err := rt.base.RoundTrip(tokenReq)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 65536))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("yahoo oauth2 token refresh returned HTTP %d", resp.StatusCode)
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || strings.TrimSpace(payload.AccessToken) == "" {
		return "", fmt.Errorf("invalid yahoo oauth2 token response")
	}
	if strings.TrimSpace(payload.RefreshToken) != "" {
		refreshToken = strings.TrimSpace(payload.RefreshToken)
	}
	ttl := 55 * time.Minute
	if payload.ExpiresIn > 120 {
		ttl = time.Duration(payload.ExpiresIn-60) * time.Second
	}
	expiry := time.Now().Add(ttl)
	accessTok := strings.TrimSpace(payload.AccessToken)
	rt.mu.Lock()
	rt.refreshedBearer = accessTok
	rt.refreshedExpiry = expiry
	rt.mu.Unlock()
	if rt.client != nil && rt.client.Config != nil && !cliutil.IsAnyHarness() && !cliutil.IsVerifyEnv() && !cliutil.IsDogfoodEnv() {
		_ = rt.client.Config.SaveTokens(clientID, clientSecret, accessTok, refreshToken, expiry)
	}
	return accessTok, nil
}
