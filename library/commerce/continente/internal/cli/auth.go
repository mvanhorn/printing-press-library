package cli

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/commerce/continente/internal/client"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	authLoginCheckPath             = "/checkout/carrinho/"
	authLoginClientID              = "NLR6WHyO8Iba4eRS"
	authLoginUsernameURL           = "https://login.continente.pt/api/username"
	authLoginValidatePasswordURL   = "https://login.continente.pt/api/email/login/validate-password"
	authLoginAuthorizeURL          = "https://login.continente.pt/api/credentials/authorize"
	authStorefrontAccountLoginPath = "/on/demandware.store/Sites-continente-Site/default/Account-Login"
)

type authStatusPayload struct {
	Authenticated bool           `json:"authenticated"`
	Summary       string         `json:"summary"`
	Location      string         `json:"location,omitempty"`
	StatusCode    int            `json:"status_code,omitempty"`
	CookieJarPath string         `json:"cookie_jar_path,omitempty"`
	CookieCount   int            `json:"cookie_count,omitempty"`
	CookiesByHost map[string]int `json:"cookies_by_host,omitempty"`
}

type authProbeResult struct {
	Authenticated bool
	Location      string
	StatusCode    int
}

type harFile struct {
	Log struct {
		Entries []harEntry `json:"entries"`
	} `json:"log"`
}

type harEntry struct {
	Request struct {
		URL     string      `json:"url"`
		Cookies []harCookie `json:"cookies"`
		Headers []harHeader `json:"headers"`
	} `json:"request"`
	Response struct {
		Cookies []harCookie `json:"cookies"`
		Headers []harHeader `json:"headers"`
	} `json:"response"`
}

type harCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Expires  string `json:"expires"`
	HTTPOnly bool   `json:"httpOnly"`
	Secure   bool   `json:"secure"`
}

type harHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type exportedCookie struct {
	Name           string   `json:"name"`
	Value          string   `json:"value"`
	Domain         string   `json:"domain"`
	Path           string   `json:"path"`
	Secure         bool     `json:"secure"`
	HTTPOnly       bool     `json:"httpOnly"`
	HostOnly       bool     `json:"hostOnly"`
	Session        bool     `json:"session"`
	ExpirationDate *float64 `json:"expirationDate"`
	Expires        string   `json:"expires"`
}

var stdinReader io.Reader = os.Stdin

type authLoginAuthorizeResponse struct {
	AuthorizationCode string `json:"authorizationCode"`
	Code              string `json:"code"`
}

type authLoginResult struct {
	LoggedIn      bool              `json:"logged_in"`
	Email         string            `json:"email"`
	Summary       string            `json:"summary"`
	SessionMode   string            `json:"session_mode,omitempty"`
	CookieJarPath string            `json:"cookie_jar_path,omitempty"`
	CookieCount   int               `json:"cookie_count,omitempty"`
	CookiesByHost map[string]int    `json:"cookies_by_host,omitempty"`
	Status        authStatusPayload `json:"status"`
}

func newAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Session-backed authentication workflows",
		Long:  "Import local authenticated browser state and inspect whether the current cookie jar appears logged in on continente.pt.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newAuthImportHARCmd(flags))
	cmd.AddCommand(newAuthImportCookiesCmd(flags))
	cmd.AddCommand(newAuthLoginCmd(flags))
	cmd.AddCommand(newAuthStatusCmd(flags))
	return cmd
}

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var email string
	var passwordStdin bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in directly and persist a fresh storefront session",
		Long:  "Runs the observed Continente login flow against login.continente.pt, exchanges the authorization result on the storefront, and persists the resulting cookies into the CLI cookie jar.",
		RunE: func(cmd *cobra.Command, args []string) error {
			email = strings.TrimSpace(email)
			if email == "" {
				return usageErr(fmt.Errorf("--email is required"))
			}
			if !passwordStdin {
				return usageErr(fmt.Errorf("--password-stdin is required"))
			}
			if dryRunOK(flags) {
				return nil
			}
			password, err := readSecretFromStdin()
			if err != nil {
				return err
			}
			if password == "" {
				return usageErr(fmt.Errorf("password from stdin is empty"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if err := c.ClearCookieDomainSuffix("continente.pt"); err != nil {
				return err
			}
			if err := runContinenteDirectLogin(cmd.Context(), c, email, password); err != nil {
				return err
			}
			c.Config.SessionMode = "direct"
			if err := c.Config.Save(); err != nil {
				return err
			}
			status, err := buildAuthStatusPayload(cmd.Context(), c)
			if err != nil {
				return err
			}
			if !status.Authenticated {
				return authErr(fmt.Errorf("login completed but storefront session was not established"))
			}
			payload := authLoginResult{
				LoggedIn:      true,
				Email:         email,
				Summary:       "authenticated storefront session established",
				SessionMode:   c.Config.SessionMode,
				CookieJarPath: c.Config.CookieJarPath,
				CookieCount:   status.CookieCount,
				CookiesByHost: status.CookiesByHost,
				Status:        status,
			}
			return emitStructuredOutput(cmd, flags, payload, DataProvenance{Source: "live"}, 1, nil)
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "Continente account email")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "Read the account password from stdin")
	return cmd
}

func newAuthImportHARCmd(flags *rootFlags) *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:   "import-har",
		Short: "Import continente.pt session cookies from a local HAR file",
		Long:  "Imports cookie state from a local browser HAR capture into the CLI cookie jar. Use only local secret-bearing files; never commit them into the repo.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(filePath) == "" {
				return usageErr(fmt.Errorf("--file is required"))
			}
			raw, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}
			cookieSets, err := parseContinuenteHARCookieSets(raw)
			if err != nil {
				return usageErr(err)
			}
			if len(cookieSets) == 0 {
				return usageErr(fmt.Errorf("no continente.pt cookies found in HAR"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			imported := 0
			hosts := make([]string, 0, len(cookieSets))
			for rawURL, cookies := range cookieSets {
				u, err := url.Parse(rawURL)
				if err != nil {
					continue
				}
				c.HTTPClient.Jar.SetCookies(u, cookies)
				imported += len(cookies)
				hosts = append(hosts, u.Hostname())
			}
			sort.Strings(hosts)
			c.Config.SessionMode = "imported"
			if err := c.Config.Save(); err != nil {
				return err
			}

			payload := map[string]any{
				"imported":         true,
				"cookies_imported": imported,
				"cookie_jar_path":  c.Config.CookieJarPath,
				"hosts":            hosts,
				"source_file":      filePath,
			}
			return emitStructuredOutput(cmd, flags, payload, DataProvenance{Source: "local"}, imported, nil)
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Path to a local HAR file exported from the browser")
	return cmd
}

func newAuthImportCookiesCmd(flags *rootFlags) *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:   "import-cookies",
		Short: "Import continente.pt session cookies from a browser cookie export",
		Long:  "Imports cookies from a browser cookie export JSON file into the CLI cookie jar. The file should contain exported cookies with domain, name, and value fields.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(filePath) == "" {
				return usageErr(fmt.Errorf("--file is required"))
			}
			raw, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}
			cookieSets, err := parseContinuenteCookieExportSets(raw)
			if err != nil {
				return usageErr(err)
			}
			if len(cookieSets) == 0 {
				return usageErr(fmt.Errorf("no continente.pt cookies found in cookie export"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			imported := 0
			hosts := make([]string, 0, len(cookieSets))
			for rawURL, cookies := range cookieSets {
				u, err := url.Parse(rawURL)
				if err != nil {
					continue
				}
				c.HTTPClient.Jar.SetCookies(u, cookies)
				imported += len(cookies)
				hosts = append(hosts, u.Hostname())
			}
			sort.Strings(hosts)
			c.Config.SessionMode = "imported"
			if err := c.Config.Save(); err != nil {
				return err
			}
			payload := map[string]any{
				"imported":         true,
				"cookies_imported": imported,
				"cookie_jar_path":  c.Config.CookieJarPath,
				"hosts":            hosts,
				"source_file":      filePath,
			}
			return emitStructuredOutput(cmd, flags, payload, DataProvenance{Source: "local"}, imported, nil)
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Path to a browser cookie export JSON file")
	return cmd
}

func newAuthStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Check whether the current cookie jar appears authenticated",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			payload, err := buildAuthStatusPayload(cmd.Context(), c)
			if err != nil {
				return err
			}
			return emitStructuredOutput(cmd, flags, payload, DataProvenance{Source: "live"}, 1, nil)
		},
	}
	return cmd
}

func buildAuthStatusPayload(ctx context.Context, c *client.Client) (authStatusPayload, error) {
	payload := authStatusPayload{
		Summary:       "guest session only",
		CookieJarPath: c.Config.CookieJarPath,
		CookiesByHost: map[string]int{},
	}
	for _, rawURL := range []string{"https://www.continente.pt/", "https://login.continente.pt/"} {
		u, err := url.Parse(rawURL)
		if err != nil {
			continue
		}
		count := len(c.HTTPClient.Jar.Cookies(u))
		if count == 0 {
			continue
		}
		payload.CookiesByHost[u.Hostname()] = count
		payload.CookieCount += count
	}
	if len(payload.CookiesByHost) == 0 {
		payload.Summary = "no persisted continente.pt session cookies"
		return payload, nil
	}

	probe, err := probeStorefrontSessionStatus(ctx, c)
	if err != nil {
		payload.Summary = fmt.Sprintf("session cookies present but auth check failed: %v", err)
		return payload, nil
	}
	payload.Authenticated = probe.Authenticated
	payload.Location = probe.Location
	payload.StatusCode = probe.StatusCode
	switch {
	case probe.Authenticated:
		payload.Summary = "authenticated storefront session"
	case probe.Location != "":
		payload.Summary = "login required"
		if c.Config != nil && c.Config.SessionMode == "imported" {
			payload.Summary = "imported session incomplete or expired"
		}
	default:
		payload.Summary = "session cookies present but not authenticated"
	}
	return payload, nil
}

func probeStorefrontSessionStatus(ctx context.Context, c *client.Client) (authProbeResult, error) {
	checkURL := c.RequestBaseURL() + authLoginCheckPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	if err != nil {
		return authProbeResult{}, err
	}
	applyCommonClientHeaders(req, c)

	httpClient := *c.HTTPClient
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return authProbeResult{}, err
	}
	defer resp.Body.Close()

	location := resp.Header.Get("Location")
	if location != "" {
		if strings.Contains(strings.ToLower(location), "/login") {
			return authProbeResult{
				Authenticated: false,
				Location:      location,
				StatusCode:    resp.StatusCode,
			}, nil
		}
	}

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return authProbeResult{
			Authenticated: false,
			Location:      location,
			StatusCode:    resp.StatusCode,
		}, nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	lower := strings.ToLower(string(body))
	if strings.Contains(lower, "account-login") || strings.Contains(lower, "login.continente.pt") {
		return authProbeResult{
			Authenticated: false,
			Location:      resp.Request.URL.String(),
			StatusCode:    resp.StatusCode,
		}, nil
	}

	return authProbeResult{
		Authenticated: true,
		Location:      resp.Request.URL.String(),
		StatusCode:    resp.StatusCode,
	}, nil
}

func applyCommonClientHeaders(req *http.Request, c *client.Client) {
	if req == nil || c == nil {
		return
	}
	if c.Config != nil {
		for k, v := range c.Config.Headers {
			req.Header.Set(k, v)
		}
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "continente-pp-cli/0.1.0")
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "text/html,application/json;q=0.9,*/*;q=0.8")
	}
}

func runContinenteDirectLogin(ctx context.Context, c *client.Client, email, password string) error {
	if err := authUsernameStep(ctx, c, email); err != nil {
		return err
	}
	if err := authValidatePasswordStep(ctx, c, password); err != nil {
		return err
	}
	verifier, challenge, err := generatePKCEPair()
	if err != nil {
		return err
	}
	authorizationCode, err := authAuthorizeStep(ctx, c, challenge)
	if err != nil {
		return err
	}
	if err := authStorefrontLoginStep(ctx, c, authorizationCode, verifier); err != nil {
		return err
	}
	return nil
}

func authUsernameStep(ctx context.Context, c *client.Client, email string) error {
	payload := map[string]any{
		"username":  email,
		"clientId":  authLoginClientID,
		"returnUrl": nil,
	}
	if _, err := doJSONRequest(ctx, c, http.MethodPost, authLoginUsernameURL, payload, jsonRequestHeaders("https://login.continente.pt/", false)); err != nil {
		return authErr(fmt.Errorf("username step failed: %w", err))
	}
	return nil
}

func authValidatePasswordStep(ctx context.Context, c *client.Client, password string) error {
	payload := map[string]any{
		"passwordRecover": false,
		"password":        password,
	}
	if _, err := doJSONRequest(ctx, c, http.MethodPost, authLoginValidatePasswordURL, payload, jsonRequestHeaders("https://login.continente.pt/", false)); err != nil {
		return authErr(fmt.Errorf("password validation failed: %w", err))
	}
	return nil
}

func authAuthorizeStep(ctx context.Context, c *client.Client, challenge string) (string, error) {
	reqURL, err := url.Parse(authLoginAuthorizeURL)
	if err != nil {
		return "", err
	}
	q := reqURL.Query()
	q.Set("clientId", authLoginClientID)
	q.Set("codeChallenge", challenge)
	q.Set("codeChallengeMethod", "S256")
	reqURL.RawQuery = q.Encode()

	data, err := doRawRequest(ctx, c, http.MethodGet, reqURL.String(), nil, ajaxLikeHeaders("https://login.continente.pt/", "application/json, text/javascript, */*; q=0.01", false))
	if err != nil {
		return "", authErr(fmt.Errorf("authorize step failed: %w", err))
	}
	code, err := parseAuthorizationCode(data)
	if err != nil {
		return "", authErr(fmt.Errorf("authorize step returned no authorization code: %w", err))
	}
	return code, nil
}

func authStorefrontLoginStep(ctx context.Context, c *client.Client, authorizationCode, verifier string) error {
	form := url.Values{
		"authorizationCode": {authorizationCode},
		"codeVerifier":      {verifier},
		"ssoLogin":          {"false"},
		"rurl":              {c.RequestBaseURL() + "/"},
	}
	headers := map[string]string{
		"Accept":           "application/json, text/javascript, */*; q=0.01",
		"Origin":           c.RequestBaseURL(),
		"Referer":          c.RequestBaseURL() + "/login/",
		"X-Requested-With": "XMLHttpRequest",
		"User-Agent":       browserLikeUserAgent(),
	}
	if _, _, err := c.PostFormWithHeaders(ctx, authStorefrontAccountLoginPath, form, headers); err != nil {
		return authErr(fmt.Errorf("storefront login exchange failed: %w", err))
	}
	return nil
}

func doJSONRequest(ctx context.Context, c *client.Client, method, rawURL string, body any, headers map[string]string) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	merged := map[string]string{
		"Content-Type": "application/json; charset=utf-8",
	}
	for k, v := range headers {
		merged[k] = v
	}
	return doRawRequest(ctx, c, method, rawURL, bytes.NewReader(payload), merged)
}

func doRawRequest(ctx context.Context, c *client.Client, method, rawURL string, body io.Reader, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	applyCommonClientHeaders(req, c)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		bodyPreview := strings.TrimSpace(string(respBody))
		if len(bodyPreview) > 512 {
			bodyPreview = bodyPreview[:512] + "..."
		}
		return nil, &client.APIError{
			Method:     method,
			Path:       rawURL,
			StatusCode: resp.StatusCode,
			Body:       bodyPreview,
		}
	}
	return respBody, nil
}

func jsonRequestHeaders(referer string, xhr bool) map[string]string {
	headers := map[string]string{
		"Accept":     "application/json, text/javascript, */*; q=0.01",
		"Origin":     "https://login.continente.pt",
		"Referer":    referer,
		"User-Agent": browserLikeUserAgent(),
	}
	if xhr {
		headers["X-Requested-With"] = "XMLHttpRequest"
	}
	return headers
}

func ajaxLikeHeaders(referer, accept string, xhr bool) map[string]string {
	headers := map[string]string{
		"Accept":     accept,
		"Referer":    referer,
		"User-Agent": browserLikeUserAgent(),
	}
	if strings.HasPrefix(referer, "https://login.continente.pt") {
		headers["Origin"] = "https://login.continente.pt"
	}
	if xhr {
		headers["X-Requested-With"] = "XMLHttpRequest"
	}
	return headers
}

func browserLikeUserAgent() string {
	return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
}

func generatePKCEPair() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	verifier := base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

func parseAuthorizationCode(data []byte) (string, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return "", fmt.Errorf("empty response")
	}

	var object authLoginAuthorizeResponse
	if err := json.Unmarshal(data, &object); err == nil {
		if code := strings.TrimSpace(object.AuthorizationCode); code != "" {
			return code, nil
		}
		if code := strings.TrimSpace(object.Code); code != "" {
			return code, nil
		}
	}

	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		asString = strings.TrimSpace(asString)
		if asString != "" {
			return asString, nil
		}
	}

	if !strings.Contains(trimmed, "{") && !strings.Contains(trimmed, "[") && !strings.Contains(trimmed, "\"") {
		return trimmed, nil
	}
	return "", fmt.Errorf("unsupported authorize response")
}

func readSecretFromStdin() (string, error) {
	data, err := io.ReadAll(io.LimitReader(stdinReader, 8192))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func parseContinuenteHARCookieSets(data []byte) (map[string][]*http.Cookie, error) {
	var har harFile
	if err := json.Unmarshal(data, &har); err != nil {
		return nil, fmt.Errorf("parsing HAR: %w", err)
	}
	grouped := map[string][]*http.Cookie{}
	for _, entry := range har.Log.Entries {
		entryURL, err := url.Parse(entry.Request.URL)
		if err != nil || !continenteHost(entryURL.Hostname()) {
			continue
		}
		key := entryURL.Scheme + "://" + entryURL.Hostname() + "/"
		for _, candidate := range append(entry.Request.Cookies, entry.Response.Cookies...) {
			cookie := harCookieToHTTP(entryURL, candidate)
			if cookie == nil {
				continue
			}
			grouped[key] = append(grouped[key], cookie)
		}
		for _, cookie := range cookiesFromHARHeaders(entryURL, entry.Request.Headers, true) {
			grouped[key] = append(grouped[key], cookie)
		}
		for _, cookie := range cookiesFromHARHeaders(entryURL, entry.Response.Headers, false) {
			grouped[key] = append(grouped[key], cookie)
		}
	}
	return dedupeCookieSets(grouped), nil
}

func cookiesFromHARHeaders(entryURL *url.URL, headers []harHeader, requestSide bool) []*http.Cookie {
	out := make([]*http.Cookie, 0)
	for _, header := range headers {
		name := strings.ToLower(strings.TrimSpace(header.Name))
		switch {
		case requestSide && name == "cookie":
			out = append(out, parseRequestCookieHeader(entryURL, header.Value)...)
		case !requestSide && (name == "set-cookie" || name == "x-set-cookie"):
			if cookie := parseSetCookieHeader(entryURL, header.Value); cookie != nil {
				out = append(out, cookie)
			}
		}
	}
	return out
}

func parseRequestCookieHeader(entryURL *url.URL, raw string) []*http.Cookie {
	parts := strings.Split(raw, ";")
	out := make([]*http.Cookie, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		out = append(out, &http.Cookie{
			Name:   strings.TrimSpace(name),
			Value:  strings.TrimSpace(value),
			Domain: entryURL.Hostname(),
			Path:   "/",
			Secure: entryURL.Scheme == "https",
		})
	}
	return out
}

func parseSetCookieHeader(entryURL *url.URL, raw string) *http.Cookie {
	cookie, err := http.ParseSetCookie(raw)
	if err != nil || cookie == nil {
		return nil
	}
	if cookie.Domain == "" {
		cookie.Domain = entryURL.Hostname()
	}
	if cookie.Path == "" {
		cookie.Path = "/"
	}
	if !continenteHost(strings.TrimPrefix(cookie.Domain, ".")) {
		return nil
	}
	if !cookie.Expires.IsZero() && !cookie.Expires.After(time.Now()) {
		return nil
	}
	return cookie
}

func harCookieToHTTP(entryURL *url.URL, source harCookie) *http.Cookie {
	if entryURL == nil || strings.TrimSpace(source.Name) == "" {
		return nil
	}
	cookie := &http.Cookie{
		Name:     source.Name,
		Value:    source.Value,
		Domain:   strings.TrimSpace(source.Domain),
		Path:     strings.TrimSpace(source.Path),
		HttpOnly: source.HTTPOnly,
		Secure:   source.Secure,
	}
	if cookie.Domain == "" {
		cookie.Domain = entryURL.Hostname()
	}
	if cookie.Path == "" {
		cookie.Path = "/"
	}
	if cookie.Expires = parseHARCookieExpiry(source.Expires); !cookie.Expires.IsZero() && !cookie.Expires.After(time.Now()) {
		return nil
	}
	if !continenteHost(strings.TrimPrefix(cookie.Domain, ".")) {
		return nil
	}
	return cookie
}

func parseHARCookieExpiry(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, time.RFC1123, time.RFC1123Z} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func continenteHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(host, ".")))
	return host == "continente.pt" || strings.HasSuffix(host, ".continente.pt")
}

func dedupeCookieSets(grouped map[string][]*http.Cookie) map[string][]*http.Cookie {
	out := map[string][]*http.Cookie{}
	for rawURL, cookies := range grouped {
		seen := map[string]int{}
		deduped := make([]*http.Cookie, 0, len(cookies))
		for _, cookie := range cookies {
			if cookie == nil {
				continue
			}
			key := strings.ToLower(cookie.Name) + "|" + strings.ToLower(strings.TrimPrefix(cookie.Domain, ".")) + "|" + cookie.Path
			if idx, ok := seen[key]; ok {
				deduped[idx] = cookie
				continue
			}
			seen[key] = len(deduped)
			deduped = append(deduped, cookie)
		}
		if len(deduped) > 0 {
			out[rawURL] = deduped
		}
	}
	return out
}

func parseContinuenteCookieExportSets(data []byte) (map[string][]*http.Cookie, error) {
	var exported []exportedCookie
	if err := json.Unmarshal(data, &exported); err != nil {
		return nil, fmt.Errorf("parsing cookie export: %w", err)
	}
	grouped := map[string][]*http.Cookie{}
	for _, source := range exported {
		domain := strings.TrimPrefix(strings.TrimSpace(source.Domain), ".")
		if !continenteHost(domain) || strings.TrimSpace(source.Name) == "" {
			continue
		}
		cookie := &http.Cookie{
			Name:     source.Name,
			Value:    source.Value,
			Domain:   source.Domain,
			Path:     source.Path,
			Secure:   source.Secure,
			HttpOnly: source.HTTPOnly,
		}
		if cookie.Path == "" {
			cookie.Path = "/"
		}
		if source.ExpirationDate != nil {
			cookie.Expires = time.Unix(int64(*source.ExpirationDate), 0)
		} else if source.Expires != "" {
			cookie.Expires = parseHARCookieExpiry(source.Expires)
		}
		if !cookie.Expires.IsZero() && !cookie.Expires.After(time.Now()) {
			continue
		}
		scheme := "https"
		rawURL := scheme + "://" + domain + "/"
		grouped[rawURL] = append(grouped[rawURL], cookie)
	}
	return dedupeCookieSets(grouped), nil
}

func storedCookieCount(path string) (int, error) {
	if strings.TrimSpace(path) == "" {
		return 0, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return 0, err
	}
	return len(raw), nil
}
