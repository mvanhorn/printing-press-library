// Copyright 2026 jnalv414. Licensed under Apache-2.0. See LICENSE.

package client

import (
	cryptorand "crypto/rand"
	"net/http"

	"github.com/mvanhorn/printing-press-library/library/productivity/plaud/internal/config"
)

// Plaud's API gateway requires a specific set of browser-fingerprint headers
// on every request to api.plaud.ai. Without them the gateway returns 5xx —
// confirmed by source-code reads of sergivalverde/plaud-toolkit, applaud, and
// Plaud_API (the 14-method C# wrapper). The user-app frontend at app.plaud.ai
// sets these via the same Chrome-style values across every XHR.
//
// The static headers don't change. x-request-id is fresh per request;
// x-device-id and x-pld-tag persist across the CLI install (stored in
// config.Config so they survive restarts).
const plaudUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// applyPlaudHeaders sets the Plaud-required browser-fingerprint headers on the
// outbound request. Safe to call after any other header setting — Set() replaces.
func applyPlaudHeaders(req *http.Request, cfg *config.Config) {
	req.Header.Set("Origin", "https://app.plaud.ai")
	req.Header.Set("Referer", "https://app.plaud.ai/")
	req.Header.Set("app-platform", "web")
	req.Header.Set("app-language", "en")
	req.Header.Set("edit-from", "web")
	req.Header.Set("User-Agent", plaudUserAgent)

	if cfg != nil {
		if cfg.DeviceID != "" {
			req.Header.Set("x-device-id", cfg.DeviceID)
		}
		if cfg.PldTag != "" {
			req.Header.Set("x-pld-tag", cfg.PldTag)
		}
	}
	req.Header.Set("x-request-id", newRequestID())
}

// newRequestID returns a 10-char hex string for x-request-id.
func newRequestID() string {
	buf := make([]byte, 5)
	if _, err := cryptorand.Read(buf); err != nil {
		return "0000000000"
	}
	const hexdigits = "0123456789abcdef"
	out := make([]byte, 10)
	for i, b := range buf {
		out[i*2] = hexdigits[b>>4]
		out[i*2+1] = hexdigits[b&0x0F]
	}
	return string(out)
}
