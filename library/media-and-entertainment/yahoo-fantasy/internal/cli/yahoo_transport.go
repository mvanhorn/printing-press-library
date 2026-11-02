package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/yahoofantasy"
)

func init() {
	config.StoreScopeIdentity = yahooStoreScopeIdentity
	registerClientHook(installYahooTransport)
}

func yahooStoreScopeIdentity(c *config.Config) string {
	if c == nil || c.CredentialRouteBlocked() || c.HasCredentialRefusals() {
		return ""
	}
	if strings.HasPrefix(c.AuthSource, "env:") {
		activeTok := strings.TrimSpace(c.AccessToken)
		if activeTok == "" {
			activeTok = strings.TrimSpace(c.YahooFantasyToken)
		}
		if activeTok != "" {
			if scope := lookupYahooAccountScope("access:" + activeTok); scope != "" {
				return scope
			}
		}
		return ""
	}
	envRefresh := strings.TrimSpace(cliutil.EnvOverride("YAHOO_REFRESH_TOKEN"))
	envCode := strings.TrimSpace(cliutil.EnvOverride("YAHOO_AUTH_CODE"))
	if envRefresh != "" {
		if scope := lookupYahooAccountScope("refresh:" + envRefresh); scope != "" {
			return scope
		}
		return "yahoo-account:" + hashDefaultDBScope("refresh:"+envRefresh)
	}
	if envCode != "" && !isYahooAuthCodeAlreadyExchanged(envCode, strings.TrimSpace(c.RefreshToken)) {
		if scope := lookupYahooAccountScope("code:" + envCode); scope != "" {
			return scope
		}
		return "yahoo-account:" + hashDefaultDBScope("code:"+envCode)
	}
	refreshToken := strings.TrimSpace(c.RefreshToken)
	if refreshToken != "" {
		if scope := lookupYahooAccountScope("refresh:" + refreshToken); scope != "" {
			return scope
		}
		return "yahoo-account:" + hashDefaultDBScope("refresh:"+refreshToken)
	}
	if accessTok := strings.TrimSpace(c.AccessToken); accessTok != "" {
		if scope := lookupYahooAccountScope("access:" + accessTok); scope != "" {
			return scope
		}
	}
	return ""
}

func isYahooAuthCodeAlreadyExchanged(authCode, savedRefreshToken string) bool {
	authCode = strings.TrimSpace(authCode)
	savedRefreshToken = strings.TrimSpace(savedRefreshToken)
	if authCode == "" || savedRefreshToken == "" {
		return false
	}
	codeScope := lookupYahooAccountScope("code:" + authCode)
	return codeScope != "" && codeScope == lookupYahooAccountScope("refresh:"+savedRefreshToken)
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
	rt := &yahooRoundTripper{
		base:   base,
		client: c,
	}
	if c.Config != nil {
		rt.cfgClientID = strings.TrimSpace(c.Config.ClientID)
		rt.cfgClientSecret = strings.TrimSpace(c.Config.ClientSecret)
		rt.cfgRefreshToken = strings.TrimSpace(c.Config.RefreshToken)
		rt.cfgTokenExpiry = c.Config.TokenExpiry
		rt.cfgPath = strings.TrimSpace(c.Config.Path)
		envRefresh := strings.TrimSpace(cliutil.EnvOverride("YAHOO_REFRESH_TOKEN"))
		envCode := strings.TrimSpace(cliutil.EnvOverride("YAHOO_AUTH_CODE"))
		unexchangedCode := envCode != "" && !isYahooAuthCodeAlreadyExchanged(envCode, rt.cfgRefreshToken)
		if !strings.HasPrefix(c.Config.AuthSource, "env:") &&
			((envRefresh != "" && envRefresh != rt.cfgRefreshToken) || unexchangedCode) {
			rt.cfgTokenExpiry = time.Unix(1, 0)
		}
	}
	c.HTTPClient.Transport = rt
	return nil
}

type yahooRoundTripper struct {
	base            http.RoundTripper
	client          *client.Client
	mu              sync.Mutex
	cfgClientID     string
	cfgClientSecret string
	cfgRefreshToken string
	cfgTokenExpiry  time.Time
	cfgPath         string
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
	if cloned.Header.Get("Authorization") == "" || rt.shouldUseRefreshedBearer() {
		if bearer, err := rt.resolveRefreshedBearer(cloned); err == nil && bearer != "" {
			cloned.Header.Set("Authorization", "Bearer "+bearer)
		}
	}

	sentAuth := cloned.Header.Get("Authorization")
	resp, err := rt.base.RoundTrip(cloned)
	if err != nil {
		return nil, err
	}

	// 4. Retry once on 401 if refresh credentials are configured.
	if resp.StatusCode == http.StatusUnauthorized && rt.hasRefreshCredentials() {
		if bearer, refErr := rt.forceRefreshBearer(cloned, sentAuth); refErr == nil && bearer != "" {
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

func (rt *yahooRoundTripper) shouldUseRefreshedBearer() bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.refreshedBearer != "" {
		return true
	}
	return !rt.cfgTokenExpiry.IsZero() && time.Now().After(rt.cfgTokenExpiry)
}

func (rt *yahooRoundTripper) hasRefreshCredentials() bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	cid, csec, rtok := rt.refreshCredentialsLocked()
	acode := strings.TrimSpace(cliutil.EnvOverride("YAHOO_AUTH_CODE"))
	return cid != "" && csec != "" && (rtok != "" || acode != "")
}

func (rt *yahooRoundTripper) refreshCredentialsLocked() (clientID, clientSecret, refreshToken string) {
	clientID = strings.TrimSpace(cliutil.EnvOverride("YAHOO_CLIENT_ID"))
	clientSecret = strings.TrimSpace(cliutil.EnvOverride("YAHOO_CLIENT_SECRET"))
	refreshToken = strings.TrimSpace(cliutil.EnvOverride("YAHOO_REFRESH_TOKEN"))
	authCode := strings.TrimSpace(cliutil.EnvOverride("YAHOO_AUTH_CODE"))
	if clientID == "" {
		clientID = rt.cfgClientID
	}
	if clientSecret == "" {
		clientSecret = rt.cfgClientSecret
	}
	unexchangedCode := authCode != "" && !isYahooAuthCodeAlreadyExchanged(authCode, rt.cfgRefreshToken)
	if refreshToken == "" && (!unexchangedCode || rt.refreshedBearer != "") {
		refreshToken = rt.cfgRefreshToken
	}
	return clientID, clientSecret, refreshToken
}

func (rt *yahooRoundTripper) resolveRefreshedBearer(req *http.Request) (string, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.refreshedBearer != "" && time.Now().Before(rt.refreshedExpiry) {
		return rt.refreshedBearer, nil
	}
	return rt.refreshBearerLocked(req)
}

func (rt *yahooRoundTripper) forceRefreshBearer(req *http.Request, failedAuthHeader string) (string, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.refreshedBearer != "" && time.Now().Before(rt.refreshedExpiry) &&
		failedAuthHeader != "" && failedAuthHeader != "Bearer "+rt.refreshedBearer {
		return rt.refreshedBearer, nil
	}
	return rt.refreshBearerLocked(req)
}

func (rt *yahooRoundTripper) refreshBearerLocked(req *http.Request) (string, error) {
	clientID, clientSecret, refreshToken := rt.refreshCredentialsLocked()
	prevRefreshToken := refreshToken
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
		AccessToken     string `json:"access_token"`
		RefreshToken    string `json:"refresh_token"`
		ExpiresIn       int    `json:"expires_in"`
		XOAuthYahooGUID string `json:"xoauth_yahoo_guid"`
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
	rt.refreshedBearer = accessTok
	rt.refreshedExpiry = expiry
	rt.cfgClientID = clientID
	rt.cfgClientSecret = clientSecret
	rt.cfgRefreshToken = refreshToken
	rt.cfgTokenExpiry = expiry
	if !cliutil.IsAnyHarness() && !cliutil.IsVerifyEnv() && !cliutil.IsDogfoodEnv() {
		_ = persistYahooOAuthTokens(rt.cfgPath, clientID, clientSecret, accessTok, refreshToken, prevRefreshToken, authCode, strings.TrimSpace(payload.XOAuthYahooGUID), expiry)
	}
	return accessTok, nil
}

func persistYahooOAuthTokens(cfgPath, clientID, clientSecret, accessTok, refreshToken, prevRefreshToken, authCode, yahooGUID string, expiry time.Time) error {
	loadPath := strings.TrimSpace(cfgPath)
	if os.Getenv("YAHOO_FANTASY_CONFIG") == "" {
		if dir, err := cliutil.ConfigDir(); err == nil && loadPath == filepath.Join(dir, "config.toml") {
			loadPath = ""
		}
	}
	cfg, err := config.Load(loadPath)
	if err != nil || cfg == nil {
		return err
	}
	legacyCred := cfg.StoreScopeLegacyCredential()
	diskRefresh := strings.TrimSpace(cfg.RefreshToken)
	diskAccess := strings.TrimSpace(cfg.AccessToken)
	prevAccess := ""
	if prevRefreshToken == "" && authCode == "" {
		prevRefreshToken = diskRefresh
		prevAccess = diskAccess
	} else if prevRefreshToken != "" && prevRefreshToken == diskRefresh {
		prevAccess = diskAccess
	}
	pinYahooAccountScope(yahooGUID, prevRefreshToken, prevAccess, authCode, refreshToken, accessTok)
	cfg.ClientID = clientID
	cfg.RefreshToken = refreshToken
	cfg.AccessToken = accessTok
	setDefaultDBScopeIdentity(cfg.StoreScopeCredential(), legacyCred, cfg.BaseURL)
	if !currentDefaultDBClaimSuppressed() {
		_ = defaultDBPath("yahoo-fantasy-pp-cli")
	}
	return cfg.SaveTokens(clientID, clientSecret, accessTok, refreshToken, expiry)
}

func yahooAccountScopesPath() (string, error) {
	dir, err := cliutil.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".oauth-account-scopes.json"), nil
}

func lookupYahooAccountScope(tokenKey string) string {
	keyHash := hashDefaultDBScope(tokenKey)
	if keyHash == "" {
		return ""
	}
	path, err := yahooAccountScopesPath()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Clean(path)) // #nosec G304 -- app-owned data directory path from cliutil.DataDir.
	if err != nil {
		return ""
	}
	var ledger map[string]string
	if err := json.Unmarshal(data, &ledger); err != nil {
		return ""
	}
	return strings.TrimSpace(ledger[keyHash])
}

func pinYahooAccountScope(yahooGUID, prevRefresh, prevAccess, authCode, newRefresh, newAccess string) {
	path, err := yahooAccountScopesPath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	_ = cliutil.WithFileLock(path, func() error {
		ledger := map[string]string{}
		if data, readErr := os.ReadFile(filepath.Clean(path)); readErr == nil { // #nosec G304 -- app-owned data directory path from cliutil.DataDir.
			_ = json.Unmarshal(data, &ledger)
		}
		var candidates []string
		if yahooGUID != "" {
			candidates = append(candidates, "guid:"+yahooGUID)
		}
		if prevRefresh != "" {
			candidates = append(candidates, "refresh:"+prevRefresh)
		}
		if prevAccess != "" {
			candidates = append(candidates, "access:"+prevAccess)
		}
		if authCode != "" {
			candidates = append(candidates, "code:"+authCode)
		}
		if newRefresh != "" {
			candidates = append(candidates, "refresh:"+newRefresh)
		}
		if newAccess != "" {
			candidates = append(candidates, "access:"+newAccess)
		}
		scopeID := ""
		for _, c := range candidates {
			if h := hashDefaultDBScope(c); h != "" {
				if existing := strings.TrimSpace(ledger[h]); existing != "" {
					scopeID = existing
					break
				}
			}
		}
		if scopeID == "" {
			switch {
			case prevRefresh != "":
				scopeID = "yahoo-account:" + hashDefaultDBScope("refresh:"+prevRefresh)
			case authCode != "":
				scopeID = "yahoo-account:" + hashDefaultDBScope("code:"+authCode)
			case newRefresh != "":
				scopeID = "yahoo-account:" + hashDefaultDBScope("refresh:"+newRefresh)
			case yahooGUID != "":
				scopeID = "yahoo-account:" + hashDefaultDBScope("guid:"+yahooGUID)
			case newAccess != "":
				scopeID = "yahoo-account:" + hashDefaultDBScope("access:"+newAccess)
			}
		}
		if scopeID == "" {
			return nil
		}
		for _, c := range candidates {
			if h := hashDefaultDBScope(c); h != "" {
				ledger[h] = scopeID
			}
		}
		encoded, mErr := json.Marshal(ledger)
		if mErr != nil {
			return mErr
		}
		return cliutil.AtomicWritePrivateFile(path, encoded, 0o600, 0o700)
	})
}
