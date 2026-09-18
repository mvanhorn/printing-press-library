// Copyright 2026 Prashant Kamani and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored Garmin auth flow. This file carries NO generator header on
// purpose: `cli-printing-press generate --force` rewrites only its own
// template outputs, so a markerless file in package cli survives a reprint.
// The shape follows the published precedent
// library/health/peloton/internal/cli/peloton_oauth.go: a novel auth file
// beside the generated auth.go/config/credentials code, wired through the
// generated registerNovelCommand / registerClientHook seams rather than by
// editing generated constructors.
//
// Garmin issues no personal API token. The only route to the Connect API is
// the same SSO the web and mobile apps use: sign in at Garmin's own page in a
// real browser, catch the single-use CAS service ticket on a loopback port,
// and exchange it at Garmin's DI OAuth2 service for a bearer + refresh pair.
// This CLI never sees the password and never handles MFA.
//
// Flow and header shapes are our own code, informed by two independently
// converging community clients (bpauli/gccli internal/garminauth/{sso,di}.go,
// MIT; cyberjunky/python-garminconnect garminconnect/client.py) and by the
// live probe recorded in the N131 design (2026-09-07).

package cli

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/garmin/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/health/garmin/internal/config"
	"github.com/pelletier/go-toml/v2"
)

// ---------------------------------------------------------------------------
// Endpoints and domains
// ---------------------------------------------------------------------------

const (
	// garminDomainGlobal and garminDomainChina are the only Garmin Connect
	// deployments. Both reference clients carry the same two-entry allowlist.
	garminDomainGlobal = "garmin.com"
	garminDomainChina  = "garmin.cn"
)

var garminAllowedDomains = map[string]bool{garminDomainGlobal: true, garminDomainChina: true}

// garminCheckDomain keeps a caller-supplied domain on the allowlist so a
// stored or flag-supplied value can never redirect the token exchange.
func garminCheckDomain(domain string) error {
	if garminAllowedDomains[domain] {
		return nil
	}
	keys := make([]string, 0, len(garminAllowedDomains))
	for k := range garminAllowedDomains {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return fmt.Errorf("domain %q not allowed; must be one of %v", domain, keys)
}

// garminEndpoints holds every URL one login touches.
type garminEndpoints struct {
	Domain     string
	SSOSignin  string
	SSOLogout  string
	DITokenURL string
	ConnectAPI string
}

// garminNewEndpoints builds the URL set for a domain. SSOLogout is Garmin's
// CAS logout route: probed live 2026-09-07, HTTP 200 with the body
// "<p>Logged out!</p>". Neither reference client models a logout at all
// (python-garminconnect's logout() is local state only and says so), so this
// route is verified by probe rather than borrowed.
func garminNewEndpoints(domain string) garminEndpoints {
	sso := "https://sso." + domain
	return garminEndpoints{
		Domain:     domain,
		SSOSignin:  sso + "/sso/signin",
		SSOLogout:  sso + "/sso/logout",
		DITokenURL: "https://diauth." + domain + "/di-oauth2-service/oauth/token",
		ConnectAPI: "https://connectapi." + domain,
	}
}

const (
	// garminDIGrantType is the service-ticket grant URL.
	garminDIGrantType = "https://connectapi.garmin.com/di-oauth2-service/oauth/grant/service_ticket"

	// garminPersonalInformationPath carries userInfo.email — the only Garmin
	// response observed in the N131.2 probe that names the account. It is the
	// identity assertion `auth login` refuses to store a token without.
	garminPersonalInformationPath = "/userprofile-service/userprofile/personal-information"

	// garminSocialProfilePath carries profileId, which auth status reports.
	garminSocialProfilePath = "/userprofile-service/socialProfile"

	// garminExpirySkew is how far ahead of expiry a token is refreshed. The
	// generated client's own refresh (not emitted for this CLI: the spec
	// declares no token_url) uses 60s; 900s matches python-garminconnect and
	// leaves room for a long sync started just under the wire.
	garminExpirySkew = 900 * time.Second

	// garminLoginTimeoutDefault bounds the whole browser round trip. The
	// listener stays open for all of it: a callback arriving after the
	// listener closed is the failure mode that made a real login look like a
	// timeout while Garmin had in fact signed the user in.
	garminLoginTimeoutDefault = 10 * time.Minute

	garminHTTPTimeout = 30 * time.Second
)

// garminDIClientID is the client id the exchange presents. Service tickets are
// single-use, so a failed exchange burns the ticket: this never loops over a
// candidate list.
const garminDIClientID = "GARMIN_CONNECT_MOBILE_ANDROID_DI_2025Q2"

// garminSetNativeHeaders applies the Garmin Connect Mobile header set. It
// rides on the SSO ticket exchange and on the API calls this file makes
// directly. DI-Backend is deliberately absent: both reference clients send it
// only on their cookie fallback path, never with a DI bearer.
func garminSetNativeHeaders(h http.Header) {
	h.Set("User-Agent", "GCM-Android-5.23")
	h.Set("X-Garmin-User-Agent", "com.garmin.android.apps.connectmobile/5.23; ; Google/sdk_gphone64_arm64/google; Android/33; Dalvik/2.1.0")
	h.Set("X-Garmin-Paired-App-Version", "10861")
	h.Set("X-Garmin-Client-Platform", "Android")
	h.Set("X-App-Ver", "10861")
	h.Set("X-Lang", "en")
	h.Set("X-GCExperience", "GC5")
	h.Set("Accept-Language", "en-US,en;q=0.9")
}

// ---------------------------------------------------------------------------
// Token chain
// ---------------------------------------------------------------------------

// garminTokens is one home's DI token chain. No password is ever part of it.
type garminTokens struct {
	AccessToken   string
	RefreshToken  string
	ClientID      string
	TokenExpiry   time.Time
	RefreshExpiry time.Time
}

// garminEdgeBlockedError is a 403/429 from diauth: an edge block or a rate
// limit, never retried. The body is truncated and carries no token (diauth
// error bodies are JSON error descriptors).
type garminEdgeBlockedError struct {
	Status int
	Body   string
}

func (e *garminEdgeBlockedError) Error() string {
	return fmt.Sprintf("Garmin's token service returned HTTP %d (edge block or rate limit; not retried): %s", e.Status, e.Body)
}

// garminRefreshRejectedError is a 400/401 on refresh: the refresh token is
// dead and only a browser login can recover.
type garminRefreshRejectedError struct {
	Status int
	Body   string
}

func (e *garminRefreshRejectedError) Error() string {
	return fmt.Sprintf("Garmin refused the stored refresh token (HTTP %d); run `garmin-pp-cli auth login --email <your account>`: %s", e.Status, e.Body)
}

type garminTokenResponse struct {
	TokenType             string `json:"token_type"`
	AccessToken           string `json:"access_token"`
	RefreshToken          string `json:"refresh_token"`
	ExpiresIn             int    `json:"expires_in"`
	RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"`
}

func garminHTTPClient() *http.Client {
	return &http.Client{
		Timeout: garminHTTPTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// The token endpoint must never be followed off-origin: a
			// redirect would carry the Authorization header or the form body
			// to somewhere Garmin does not control.
			return http.ErrUseLastResponse
		},
	}
}

// garminBasicAuth builds `Basic base64(client_id + ":")`, the header the
// service-ticket exchange requires. Refresh deliberately omits it (see
// garminRefreshTokens).
func garminBasicAuth(clientID string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(clientID+":"))
}

// garminExchangeServiceTicket POSTs the single-use CAS ticket once. serviceURL
// must be byte-identical to the `service` value used at SSO.
func garminExchangeServiceTicket(ctx context.Context, hc *http.Client, ep garminEndpoints, ticket, serviceURL string) (*garminTokens, error) {
	if ticket == "" {
		return nil, errors.New("token exchange: empty service ticket")
	}
	form := url.Values{
		"client_id":      {garminDIClientID},
		"service_ticket": {ticket},
		"grant_type":     {garminDIGrantType},
		"service_url":    {serviceURL},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.DITokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	garminSetNativeHeaders(req.Header)
	req.Header.Set("Authorization", garminBasicAuth(garminDIClientID))
	req.Header.Set("Accept", "application/json,text/html;q=0.9,*/*;q=0.8")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := garminDoTokenRequest(hc, req, false)
	if err != nil {
		return nil, fmt.Errorf("exchanging the Garmin sign-in ticket: %w", err)
	}
	return garminTokensFrom(resp, time.Now()), nil
}

// garminRefreshTokens performs grant_type=refresh_token.
//
// The request body carries client_id and the refresh token and NOTHING ELSE:
// no Authorization header and no native app headers. Both reference clients
// send a Basic header here; the plain body-only form was proven accepted
// (HTTP 200, rotated refresh token) against diauth on 2026-09-07, and it is
// the shape the printing-press generated refresh would use, so this CLI keeps
// the two identical rather than diverging from the framework it ships in.
//
// Garmin rotates the refresh token on every refresh. A response that omits a
// new one leaves the stored one in place.
func garminRefreshTokens(ctx context.Context, hc *http.Client, ep garminEndpoints, old *garminTokens) (*garminTokens, error) {
	if old == nil || old.RefreshToken == "" {
		return nil, errors.New("refresh: no refresh token stored")
	}
	clientID := old.ClientID
	if clientID == "" {
		clientID = garminDIClientID
	}
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {old.RefreshToken},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.DITokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := garminDoTokenRequest(hc, req, true)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	t := garminTokensFrom(resp, now)
	if t.ClientID == "" {
		t.ClientID = clientID
	}
	if resp.RefreshToken == "" {
		t.RefreshToken = old.RefreshToken
		t.RefreshExpiry = old.RefreshExpiry
	}
	return t, nil
}

func garminDoTokenRequest(hc *http.Client, req *http.Request, isRefresh bool) (*garminTokenResponse, error) {
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	switch {
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		return nil, &garminEdgeBlockedError{Status: resp.StatusCode, Body: garminTruncate(body, 200)}
	case isRefresh && (resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized):
		return nil, &garminRefreshRejectedError{Status: resp.StatusCode, Body: garminTruncate(body, 200)}
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("Garmin's token service returned HTTP %d: %s", resp.StatusCode, garminTruncate(body, 200))
	}
	var parsed garminTokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decoding the token response: %w", err)
	}
	if parsed.AccessToken == "" {
		return nil, errors.New("token response carried no access_token")
	}
	if parsed.ExpiresIn <= 0 {
		return nil, fmt.Errorf("token response carried an unusable expires_in: %d", parsed.ExpiresIn)
	}
	return &parsed, nil
}

func garminTokensFrom(resp *garminTokenResponse, now time.Time) *garminTokens {
	t := &garminTokens{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ClientID:     garminJWTString(resp.AccessToken, "client_id"),
		TokenExpiry:  now.Add(time.Duration(resp.ExpiresIn) * time.Second),
	}
	if resp.RefreshTokenExpiresIn > 0 {
		t.RefreshExpiry = now.Add(time.Duration(resp.RefreshTokenExpiresIn) * time.Second)
	}
	return t
}

// garminTruncate bounds an error body. Non-200 diauth bodies are error
// descriptors; a token is never present in one.
func garminTruncate(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// ---------------------------------------------------------------------------
// JWT claims (informational, never trusted for authorization)
// ---------------------------------------------------------------------------

// garminJWTPayload decodes the second segment without verifying the
// signature. A header claiming `alg: none` is refused so an unsigned payload
// cannot steer the client id, the guid or the expiry.
func garminJWTPayload(token string) (map[string]any, bool) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, false
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if !garminDecodeJWTSegment(parts[0], &header) || header.Alg == "" || strings.EqualFold(header.Alg, "none") {
		return nil, false
	}
	var m map[string]any
	if !garminDecodeJWTSegment(parts[1], &m) {
		return nil, false
	}
	return m, true
}

func garminDecodeJWTSegment(seg string, v any) bool {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(seg, "="))
	if err != nil {
		return false
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	return dec.Decode(v) == nil
}

// garminJWTString reads a string claim, returning "" when absent. The
// identity claim is `garmin_guid` (a 36-character UUID, confirmed by decoding
// a live DI access token on 2026-09-07).
func garminJWTString(token, claim string) string {
	m, ok := garminJWTPayload(token)
	if !ok {
		return ""
	}
	if s, ok := m[claim].(string); ok {
		return s
	}
	return ""
}

// garminJWTExpiry returns the exp claim, or the zero time when it is absent
// or not a plausible unix second.
func garminJWTExpiry(token string) time.Time {
	m, ok := garminJWTPayload(token)
	if !ok {
		return time.Time{}
	}
	n, ok := m["exp"].(json.Number)
	if !ok {
		return time.Time{}
	}
	f, err := n.Float64()
	if err != nil || f <= 0 || f > 1e12 {
		return time.Time{}
	}
	return time.Unix(int64(f), 0)
}

// garminNeedsRefresh reports whether the access token expires within
// garminExpirySkew. The JWT exp claim wins when readable, the stored expiry is
// the fallback, and with neither the answer is yes: one refresh call is a
// cheaper mistake than a run of 401s.
func garminNeedsRefresh(accessToken string, stored time.Time, now time.Time) bool {
	exp := garminJWTExpiry(accessToken)
	if exp.IsZero() {
		exp = stored
	}
	if exp.IsZero() {
		return true
	}
	return now.After(exp.Add(-garminExpirySkew))
}

// ---------------------------------------------------------------------------
// Loopback callback
// ---------------------------------------------------------------------------

// garminNewState returns a 128-bit hex nonce for the callback.
func garminNewState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating the login nonce: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// garminCallback is a one-shot loopback listener on 127.0.0.1:0 that accepts
// exactly one Garmin redirect carrying both ?ticket= and the expected ?state=.
type garminCallback struct {
	URL    string
	state  string
	srv    *http.Server
	ticket chan string

	mu       sync.Mutex
	got      bool
	closed   bool
	rejected int
}

// garminStartCallback binds an ephemeral loopback port and serves until Close.
func garminStartCallback(state string) (*garminCallback, error) {
	if state == "" {
		return nil, errors.New("loopback callback: empty state nonce")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("binding the loopback callback: %w", err)
	}
	c := &garminCallback{state: state, ticket: make(chan string, 1)}
	port := ln.Addr().(*net.TCPAddr).Port
	c.URL = fmt.Sprintf("http://127.0.0.1:%d/callback?state=%s", port, state)

	mux := http.NewServeMux()
	mux.HandleFunc("/", c.handle)
	// A local process holding header-less connections open must not be able
	// to stall the login window.
	c.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 8 << 10}
	go func() { _ = c.srv.Serve(ln) }()
	return c, nil
}

func (c *garminCallback) handle(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ticket := q.Get("ticket")
	if ticket == "" {
		// favicon and health probes reach the same handler.
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(c.state)) != 1 {
		c.mu.Lock()
		c.rejected++
		c.mu.Unlock()
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}
	c.mu.Lock()
	if c.got {
		c.mu.Unlock()
		http.Error(w, "ticket already received", http.StatusConflict)
		return
	}
	c.got = true
	c.mu.Unlock()

	// Write and flush the browser's success page before handing the ticket
	// over: the receiver closes the listener the moment it has the ticket.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<html><body><h1>Garmin sign-in received</h1>` +
		`<p>You can close this window and return to the terminal.</p></body></html>`))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	c.ticket <- ticket
}

// Ticket yields the single accepted ticket.
func (c *garminCallback) Ticket() <-chan string { return c.ticket }

// Rejected reports how many callbacks failed the state check.
func (c *garminCallback) Rejected() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rejected
}

// Close stops the listener: graceful first so an in-flight success page
// finishes, then hard. Idempotent.
func (c *garminCallback) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.srv.Shutdown(ctx); err != nil {
		return c.srv.Close()
	}
	return nil
}

// garminBuildSSOURL builds the sign-in URL that redirects back to the
// loopback callback. callbackURL already carries the state nonce, so the
// nonce rides through Garmin's redirect without needing a parameter Garmin
// would have to preserve.
func garminBuildSSOURL(ep garminEndpoints, callbackURL, email string) string {
	params := url.Values{
		"service":                         {callbackURL},
		"gauthHost":                       {"https://sso." + ep.Domain + "/sso"},
		"source":                          {callbackURL},
		"redirectAfterAccountLoginUrl":    {callbackURL},
		"redirectAfterAccountCreationUrl": {callbackURL},
	}
	if email != "" {
		params.Set("prepopUsername", email)
	}
	return ep.SSOSignin + "?" + params.Encode()
}

// ---------------------------------------------------------------------------
// Identity assertion
// ---------------------------------------------------------------------------

// garminSetAPIHeaders applies the Bearer data-call header set.
func garminSetAPIHeaders(h http.Header, token string) {
	garminSetNativeHeaders(h)
	h.Set("Authorization", "Bearer "+token)
	h.Set("Accept", "application/json")
}

type garminPersonalInformation struct {
	UserInfo struct {
		Email string `json:"email"`
	} `json:"userInfo"`
}

type garminSocialProfile struct {
	ProfileID   json.Number `json:"profileId"`
	DisplayName string      `json:"displayName"`
}

// garminGetJSON performs one authenticated GET and decodes it.
func garminGetJSON(ctx context.Context, hc *http.Client, ep garminEndpoints, token, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep.ConnectAPI+path, nil)
	if err != nil {
		return err
	}
	garminSetAPIHeaders(req.Header, token)
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("calling Garmin at %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Garmin returned HTTP %d for %s", resp.StatusCode, path)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decoding the response from %s: %w", path, err)
	}
	return nil
}

// garminAssertEmail fetches the signed-in account's identity block and
// compares its email with want, case-insensitively. It returns the email the
// API reported so the caller can name the wrong account in an error.
func garminAssertEmail(ctx context.Context, hc *http.Client, ep garminEndpoints, token, want string) (string, error) {
	var pi garminPersonalInformation
	if err := garminGetJSON(ctx, hc, ep, token, garminPersonalInformationPath, &pi); err != nil {
		return "", err
	}
	got := strings.TrimSpace(pi.UserInfo.Email)
	if got == "" {
		return "", fmt.Errorf("Garmin's identity response carried no account email, so the account could not be confirmed")
	}
	if !strings.EqualFold(got, strings.TrimSpace(want)) {
		return got, errGarminIdentityMismatch
	}
	return got, nil
}

// errGarminIdentityMismatch marks the case the caller must turn into a
// discard-and-explain, not a retry.
var errGarminIdentityMismatch = errors.New("the signed-in Garmin account is not the one you named")

// ---------------------------------------------------------------------------
// Identity sidecar
// ---------------------------------------------------------------------------

// garminIdentityFileName holds the non-secret facts about which account a home
// is bound to.
//
// It is a separate file rather than keys in config.toml because the generated
// config only ever persists base_url and headers: internal/config/config.go's
// persistedConfig has exactly those two fields, and every SaveTokens call
// rewrites config.toml from it, so any extra key there is destroyed by the
// next login. It lives beside config.toml, is written 0600 in a 0700 dir, and
// holds no secret — but it does hold an email address, so it is private-mode
// like the rest of the home.
const garminIdentityFileName = "garmin-identity.toml"

type garminIdentity struct {
	Email         string    `toml:"email"`
	GarminGUID    string    `toml:"garmin_guid"`
	ProfileID     string    `toml:"profile_id"`
	DisplayName   string    `toml:"display_name"`
	Domain        string    `toml:"domain"`
	ClientID      string    `toml:"client_id"`
	RefreshExpiry time.Time `toml:"refresh_expiry"`
	AssertedAt    time.Time `toml:"asserted_at"`
}

// garminIdentityPath returns the sidecar beside the resolved config file, so
// --home, --config and the per-kind env rungs all move it with the home.
func garminIdentityPath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), garminIdentityFileName)
}

// garminLoadIdentity reads the sidecar through the same symlink-resolution and
// permission check the generated loader applies to config.toml and
// credentials.toml (internal/config/config.go and internal/cliutil/
// credentials.go). The sidecar holds no secret, but it does hold an account
// email, so a group- or world-readable file is refused rather than read. A
// symlink is resolved, not refused: what is checked is the mode of whatever it
// points at.
func garminLoadIdentity(configPath string) (*garminIdentity, error) {
	path := filepath.Clean(garminIdentityPath(configPath))
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	if err := cliutil.VerifyCredsPerms(real); err != nil {
		return nil, fmt.Errorf("refusing to read %s: %w", path, err)
	}
	data, err := os.ReadFile(real)
	if err != nil {
		return nil, err
	}
	var id garminIdentity
	if err := toml.Unmarshal(data, &id); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", garminIdentityPath(configPath), err)
	}
	return &id, nil
}

func garminSaveIdentity(configPath string, id *garminIdentity) error {
	data, err := toml.Marshal(id)
	if err != nil {
		return fmt.Errorf("encoding the identity file: %w", err)
	}
	return cliutil.AtomicWritePrivateFile(garminIdentityPath(configPath), data, 0o600, 0o700)
}

func garminRemoveIdentity(configPath string) error {
	if err := os.Remove(garminIdentityPath(configPath)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ---------------------------------------------------------------------------
// Lock
// ---------------------------------------------------------------------------

// garminAuthLockName is the lock this CLI holds around every
// refresh-then-write. Two concurrent invocations must not both refresh: Garmin
// rotates the refresh token on use, so the loser would persist a token Garmin
// has already retired.
//
// cliutil.WithFileLock appends ".lock" to the path it is given, so the file on
// disk is <config>/.garmin-auth.lock. Reusing the generated helper keeps the
// Windows/Unix flock split in one place.
const garminAuthLockName = ".garmin-auth"

func garminWithAuthLock(configPath string, fn func() error) error {
	return cliutil.WithFileLock(filepath.Join(filepath.Dir(configPath), garminAuthLockName), fn)
}

// ---------------------------------------------------------------------------
// Credential read/write through the generated layout
// ---------------------------------------------------------------------------

// garminStoredTokens reads the chain the generated config/credentials layer
// resolved. Tokens live in <data>/credentials.toml; nothing here reads a
// bespoke token file.
func garminStoredTokens(cfg *config.Config) *garminTokens {
	if cfg == nil {
		return nil
	}
	access := cfg.AccessToken
	if access == "" {
		access = cfg.GarminToken
	}
	if access == "" && cfg.RefreshToken == "" {
		return nil
	}
	return &garminTokens{
		AccessToken:  access,
		RefreshToken: cfg.RefreshToken,
		ClientID:     cfg.ClientID,
		TokenExpiry:  cfg.TokenExpiry,
	}
}

// garminPersistTokens writes the chain through the generated SaveTokens so
// credentials.toml stays the single home for secrets, then updates the
// sidecar's non-secret refresh expiry. Callers hold the auth lock.
func garminPersistTokens(cfg *config.Config, t *garminTokens) error {
	if err := cfg.SaveTokens(t.ClientID, "", t.AccessToken, t.RefreshToken, t.TokenExpiry); err != nil {
		return err
	}
	if t.RefreshExpiry.IsZero() {
		return nil
	}
	id, err := garminLoadIdentity(cfg.Path)
	if err != nil {
		return nil //nolint:nilerr // no sidecar yet is not an error: login writes it.
	}
	id.RefreshExpiry = t.RefreshExpiry
	return garminSaveIdentity(cfg.Path, id)
}

// ---------------------------------------------------------------------------
// Pre-call refresh
// ---------------------------------------------------------------------------

// garminEnsureFreshToken is registered as a client hook, so it runs once per
// client construction for both CLI commands and MCP tool calls
// (internal/mcp/tools.go builds its client directly and calls
// ApplyClientHooks for exactly this reason).
//
// This CLI has no generated refreshAccessToken: that is emitted only when the
// spec declares a token_url, and this spec declares a plain bearer scheme
// (0 hits for refreshAccessToken across internal/). The pre-call refresh below
// is the whole refresh path.
func garminEnsureFreshToken(cfg *config.Config, stderr io.Writer, now time.Time) error {
	if cfg == nil {
		return nil
	}
	// The verifier's mock-mode subprocesses must never dial Garmin.
	if cliutil.IsVerifyEnv() {
		return nil
	}
	// An env-supplied token is not this home's chain to rotate.
	if strings.HasPrefix(cfg.AuthSource, "env:") {
		return nil
	}
	stored := garminStoredTokens(cfg)
	if stored == nil || stored.RefreshToken == "" {
		return nil
	}
	if !garminNeedsRefresh(stored.AccessToken, stored.TokenExpiry, now) {
		return nil
	}
	ep := garminEndpointsFor(cfg, garminDomainForConfig(cfg))

	var refreshed *garminTokens
	err := garminWithAuthLock(cfg.Path, func() error {
		// Re-read under the lock: a concurrent invocation may have already
		// refreshed this chain while we waited, and re-presenting the
		// rotated-out refresh token would fail.
		fresh, loadErr := config.Load(cfg.Path)
		if loadErr == nil {
			if current := garminStoredTokens(fresh); current != nil && current.RefreshToken != "" {
				if !garminNeedsRefresh(current.AccessToken, current.TokenExpiry, now) {
					refreshed = current
					return nil
				}
				stored = current
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), garminHTTPTimeout)
		defer cancel()
		next, refreshErr := garminRefreshTokens(ctx, garminHTTPClient(), ep, stored)
		if refreshErr != nil {
			return refreshErr
		}
		if err := garminPersistTokens(cfg, next); err != nil {
			return err
		}
		refreshed = next
		return nil
	})
	if err != nil {
		// A refresh that failed while the access token is still usable is a
		// warning: the command can still run. Only a dead token is fatal.
		if stored.AccessToken != "" && !garminTokenExpired(stored, now) {
			if stderr != nil {
				fmt.Fprintf(stderr, "warning: could not refresh the Garmin token (%v); continuing with the token already stored\n", err)
			}
			return nil
		}
		return err
	}
	if refreshed != nil {
		cfg.AccessToken = refreshed.AccessToken
		cfg.RefreshToken = refreshed.RefreshToken
		cfg.TokenExpiry = refreshed.TokenExpiry
		cfg.AuthHeaderVal = ""
	}
	return nil
}

// garminTokenExpired reports whether the access token is past its expiry with
// no skew: the point at which continuing would only produce 401s.
func garminTokenExpired(t *garminTokens, now time.Time) bool {
	exp := garminJWTExpiry(t.AccessToken)
	if exp.IsZero() {
		exp = t.TokenExpiry
	}
	if exp.IsZero() {
		return false
	}
	return now.After(exp)
}

// garminEndpointsFor resolves the endpoint set for a stored chain, honouring
// the two config-level overrides every printed CLI accepts: GARMIN_BASE_URL
// (applied by the generated config loader, and persisted as base_url in
// config.toml) redirects the API calls, and GARMIN_TOKEN_URL redirects the
// token service. A mock-server harness reaches the auth flow through those and
// nothing else.
//
// Both overrides are accepted only for a loopback host, and for the same
// reason. The refresh POST carries the refresh token in its body, and every
// call built on ConnectAPI carries a freshly minted Garmin bearer in an
// Authorization header, so an override that could name any host would turn one
// environment variable — or one persisted config key — into a credential
// exfiltration path. A mock server is on loopback by definition, and Garmin's
// own hosts are the defaults. A base_url that simply restates the default for
// this domain is not an override and passes silently.
func garminEndpointsFor(cfg *config.Config, domain string) garminEndpoints {
	ep := garminNewEndpoints(domain)
	if v := strings.TrimSpace(os.Getenv("GARMIN_TOKEN_URL")); v != "" {
		if garminIsLoopbackURL(v) {
			ep.DITokenURL = v
		} else {
			fmt.Fprintf(os.Stderr, "warning: ignoring GARMIN_TOKEN_URL: only a loopback host is accepted for the token service\n")
		}
	}
	if cfg != nil {
		if v := strings.TrimSuffix(strings.TrimSpace(cfg.BaseURL), "/"); v != "" && v != ep.ConnectAPI {
			if garminIsLoopbackURL(v) {
				ep.ConnectAPI = v
			} else {
				fmt.Fprintf(os.Stderr, "warning: ignoring base_url %q for the Garmin account check: only a loopback host is accepted; using %s\n", v, ep.ConnectAPI)
			}
		}
	}
	return ep
}

// garminIsLoopbackURL reports whether raw names a host on this machine.
func garminIsLoopbackURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// garminDomainForConfig resolves which Garmin deployment this home is bound
// to, defaulting to the global one.
func garminDomainForConfig(cfg *config.Config) string {
	if cfg != nil {
		if id, err := garminLoadIdentity(cfg.Path); err == nil && garminCheckDomain(id.Domain) == nil {
			return id.Domain
		}
	}
	return garminDomainGlobal
}

// ---------------------------------------------------------------------------
// One-time migration of the N131.2 spike token file
// ---------------------------------------------------------------------------

// garminSpikeConfig is the flat key set the N131.2 auth spike wrote to
// <config>/config.toml. Those keys are not part of this CLI's config schema,
// and their token_expiry is a quoted string where the generated config wants a
// TOML datetime, so leaving the file in place makes every invocation print
// "legacy config parse skipped". Migrating it once removes the warning and the
// duplicate credential home in the same step.
type garminSpikeConfig struct {
	DIToken        string `toml:"di_token"`
	DIRefreshToken string `toml:"di_refresh_token"`
	DIClientID     string `toml:"di_client_id"`
	TokenExpiry    string `toml:"token_expiry"`
	RefreshExpiry  string `toml:"refresh_expiry"`
	Email          string `toml:"email"`
	Domain         string `toml:"domain"`
}

// garminResolveConfigPath mirrors the generated config loader's own path
// resolution (--config, then GARMIN_CONFIG, then the config-kind directory on
// the home ladder). The migration below has to resolve the path WITHOUT
// calling config.Load: a spike token file at a config-kind path makes
// config.Load fail outright rather than warn, so a command that loaded first
// would die before it could migrate.
func garminResolveConfigPath(configFlag string) (string, error) {
	if strings.TrimSpace(configFlag) != "" {
		return configFlag, nil
	}
	if path := strings.TrimSpace(os.Getenv("GARMIN_CONFIG")); path != "" {
		return path, nil
	}
	dir, err := cliutil.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// garminMigrateResolved resolves this home's config path and migrates a spike
// token file if one is sitting there. Every command that owns auth calls this
// before config.Load.
func garminMigrateResolved(configFlag string, stdout io.Writer) (bool, error) {
	path, err := garminResolveConfigPath(configFlag)
	if err != nil {
		return false, err
	}
	return garminMigrateSpikeTokenFile(path, configFlag, stdout)
}

// garminMigrateSpikeTokenFile converts a spike token file at the resolved
// config path into the press layout: secrets to <data>/credentials.toml,
// identity to the sidecar, and config.toml rewritten as a valid press config.
//
// It only ever reads THIS home's resolved config path. The generated loader
// also falls back to ~/.config/garmin-pp-cli/config.toml when a home has no
// config of its own, but migrating through that fallback would copy one
// account's tokens into another account's home, so this deliberately does not.
//
// Returns true when it migrated something. Idempotent: a config.toml with no
// spike keys is left untouched.
// configFlag is the raw --config value (usually empty) so the credentials are
// saved through the same home ladder every other command uses; configPath is
// the already-resolved file to read and rewrite.
func garminMigrateSpikeTokenFile(configPath, configFlag string, stdout io.Writer) (bool, error) {
	data, err := os.ReadFile(filepath.Clean(configPath))
	if err != nil {
		return false, nil //nolint:nilerr // no config file is the normal first-run case.
	}
	var spike garminSpikeConfig
	if err := toml.Unmarshal(data, &spike); err != nil {
		return false, nil //nolint:nilerr // not a spike file; the generated loader reports its own parse problems.
	}
	if spike.DIToken == "" || spike.DIRefreshToken == "" {
		return false, nil
	}
	domain := spike.Domain
	if garminCheckDomain(domain) != nil {
		domain = garminDomainGlobal
	}
	tokens := &garminTokens{
		AccessToken:   spike.DIToken,
		RefreshToken:  spike.DIRefreshToken,
		ClientID:      spike.DIClientID,
		TokenExpiry:   garminParseFlexibleTime(spike.TokenExpiry),
		RefreshExpiry: garminParseFlexibleTime(spike.RefreshExpiry),
	}
	if tokens.ClientID == "" {
		tokens.ClientID = garminJWTString(tokens.AccessToken, "client_id")
	}
	if tokens.TokenExpiry.IsZero() {
		tokens.TokenExpiry = garminJWTExpiry(tokens.AccessToken)
	}

	err = garminWithAuthLock(configPath, func() (migrateErr error) {
		// config.toml must be rewritten before config.Load: the generated
		// loader fails outright on the spike keys, so it cannot be read in
		// its original state. That leaves a window in which the spike file is
		// gone and credentials.toml is not yet written, and the refresh chain
		// exists only in the in-memory `tokens` struct — losing it would cost
		// an owner-gated browser login to recover. So the original bytes are
		// held and restored if anything between here and the credential write
		// fails.
		original := data
		if err := cliutil.AtomicWritePrivateFile(configPath,
			[]byte("base_url = 'https://connectapi.garmin.com'\n"), 0o600, 0o700); err != nil {
			return err
		}
		credentialsWritten := false
		defer func() {
			if migrateErr == nil || credentialsWritten {
				return
			}
			if restoreErr := cliutil.AtomicWritePrivateFile(configPath, original, 0o600, 0o700); restoreErr != nil {
				migrateErr = fmt.Errorf("%w; the original token file at %s could not be restored either (%v), so its refresh chain is lost and this home must sign in again", migrateErr, configPath, restoreErr)
			}
		}()
		cfg, err := config.Load(configFlag)
		if err != nil {
			return err
		}
		if err := cfg.SaveTokens(tokens.ClientID, "", tokens.AccessToken, tokens.RefreshToken, tokens.TokenExpiry); err != nil {
			return err
		}
		credentialsWritten = true
		id := &garminIdentity{
			Email:         spike.Email,
			GarminGUID:    garminJWTString(tokens.AccessToken, "garmin_guid"),
			Domain:        domain,
			ClientID:      tokens.ClientID,
			RefreshExpiry: tokens.RefreshExpiry,
		}
		if existing, err := garminLoadIdentity(configPath); err == nil {
			if existing.ProfileID != "" {
				id.ProfileID = existing.ProfileID
			}
			if existing.DisplayName != "" {
				id.DisplayName = existing.DisplayName
			}
			id.AssertedAt = existing.AssertedAt
		}
		return garminSaveIdentity(configPath, id)
	})
	if err != nil {
		return false, err
	}
	if stdout != nil {
		fmt.Fprintf(stdout, "Migrated the Garmin auth-spike token file at %s into this CLI's credential layout.\n", configPath)
		fmt.Fprintf(stdout, "  Tokens now live in the credentials file; the account it names is in %s.\n", garminIdentityPath(configPath))
		fmt.Fprintln(stdout, "  The email in that file has not been re-checked against Garmin; run `auth status --verify` to confirm it.")
	}
	return true, nil
}

// garminParseFlexibleTime accepts the RFC3339 forms the spike wrote (with and
// without fractional seconds, local or UTC offset).
func garminParseFlexibleTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
