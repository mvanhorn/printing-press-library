// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

// Package grubhub holds the hand-written Grubhub web-API integration that the
// generator does not produce: the anonymous-bearer auth handshake (dynamic
// client_id scrape + token mint/cache) and typed parsing of search, restaurant,
// and menu responses. Generated endpoint commands authenticate via the token
// this package mints into the shared config; the friendly top-level and
// transcendence commands call EnsureToken before issuing requests so the CLI
// needs zero credential setup.
package grubhub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/grubhub/internal/config"
)

const (
	// DesktopUserAgent mirrors a real Chrome desktop client so PerimeterX does
	// not treat the request as an obvious bot.
	DesktopUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
	staticContentURL = "https://www.grubhub.com/eat/static-content-unauth?contentOnly=1"
	authURL          = "https://api-gtm.grubhub.com/auth"
	defaultTokenTTL  = 50 * time.Minute
)

var clientIDRe = regexp.MustCompile(`beta_[A-Za-z0-9]+`)

// RequestHeaders returns the browser-fingerprint headers every Grubhub API call
// should carry, on top of the Authorization header the generated client adds.
func RequestHeaders() map[string]string {
	return map[string]string{
		"Origin":  "https://www.grubhub.com",
		"Referer": "https://www.grubhub.com/",
		"Accept":  "application/json",
	}
}

// EnsureToken returns a valid anonymous bearer token, minting and caching one in
// the config file when none is present or the cached token has expired. When the
// user has set GRUBHUB_TOKEN, that value is honored and no mint occurs.
func EnsureToken(ctx context.Context, configPath string) (string, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return "", err
	}
	if cfg.GrubhubToken != "" {
		return cfg.GrubhubToken, nil
	}
	if cfg.AccessToken != "" && !cfg.TokenExpiry.IsZero() && time.Now().Before(cfg.TokenExpiry.Add(-2*time.Minute)) {
		return cfg.AccessToken, nil
	}
	access, refresh, expiry, err := Mint(ctx)
	if err != nil {
		return "", err
	}
	if err := cfg.SaveTokens("", "", access, refresh, expiry); err != nil {
		// A read-only or unwritable config must not block API use; the token is
		// still valid for this process even if it could not be persisted.
		return access, nil
	}
	return access, nil
}

// Mint performs the full anonymous handshake: scrape a fresh client_id, then
// exchange it at /auth for an access token. Returns the access token, refresh
// token, and the absolute expiry time.
func Mint(ctx context.Context) (access, refresh string, expiry time.Time, err error) {
	clientID, err := scrapeClientID(ctx)
	if err != nil {
		return "", "", time.Time{}, err
	}
	return mintWithClientID(ctx, clientID)
}

func scrapeClientID(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, staticContentURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", DesktopUserAgent)
	req.Header.Set("Accept", "text/html")
	resp, err := httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("scraping Grubhub client id: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	match := clientIDRe.Find(body)
	if match == nil {
		return "", fmt.Errorf("could not find a Grubhub client id on the unauth content page (status %d)", resp.StatusCode)
	}
	return string(match), nil
}

type sessionEnvelope struct {
	SessionHandle struct {
		AccessToken       string  `json:"access_token"`
		RefreshToken      string  `json:"refresh_token"`
		TokenRemainingSec float64 `json:"token_remaining_secs"`
	} `json:"session_handle"`
}

func mintWithClientID(ctx context.Context, clientID string) (string, string, time.Time, error) {
	// device_id is a non-secret synthetic identifier the web app sends with the
	// anonymous auth handshake; it only needs to look like a plausible 10-digit
	// id, not be unpredictable, so a weak PRNG is correct and intentional here.
	deviceID := rand.Int63n(9_000_000_000) + 1_000_000_000 // #nosec G404 -- non-secret synthetic device id, not a security primitive
	payload := map[string]any{
		"brand":     "GRUBHUB",
		"client_id": clientID,
		"device_id": deviceID,
		"scope":     "anonymous",
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return "", "", time.Time{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL, bytes.NewReader(buf))
	if err != nil {
		return "", "", time.Time{}, err
	}
	// The first /auth call carries an empty Bearer seed, exactly like the web app.
	req.Header.Set("Authorization", "Bearer")
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", DesktopUserAgent)
	req.Header.Set("Origin", "https://www.grubhub.com")
	req.Header.Set("Referer", "https://www.grubhub.com/")
	resp, err := httpClient().Do(req)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("minting Grubhub token: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", "", time.Time{}, fmt.Errorf("Grubhub auth returned status %d (the anonymous client id may have rotated): %s", resp.StatusCode, truncate(string(body), 160))
	}
	var env sessionEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return "", "", time.Time{}, fmt.Errorf("parsing Grubhub auth response: %w", err)
	}
	if env.SessionHandle.AccessToken == "" {
		return "", "", time.Time{}, fmt.Errorf("Grubhub auth response did not include an access token")
	}
	ttl := defaultTokenTTL
	if env.SessionHandle.TokenRemainingSec > 60 {
		ttl = time.Duration(env.SessionHandle.TokenRemainingSec) * time.Second
	}
	return env.SessionHandle.AccessToken, env.SessionHandle.RefreshToken, time.Now().Add(ttl), nil
}

func httpClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// FormatPoint builds Grubhub's WKT location string, which is POINT(longitude
// latitude) with the longitude first.
func FormatPoint(lng, lat float64) string {
	return "POINT(" + strconv.FormatFloat(lng, 'f', -1, 64) + " " + strconv.FormatFloat(lat, 'f', -1, 64) + ")"
}
