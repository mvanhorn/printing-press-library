// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// Hand-built Chrome cookie extraction for the Suno Clerk auth flow. kooky reads
// Chrome's encrypted cookie store and handles macOS keychain decryption, so the
// CLI can pull the __client cookie (and ajs_anonymous_id for the device id)
// straight from the browser the user already logged in with.

package auth

import (
	"context"
	"regexp"
	"strings"

	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/chrome" // register only the Chrome cookie store finder
)

const zeroUUID = "00000000-0000-0000-0000-000000000000"

var uuidish = regexp.MustCompile(`[^0-9a-fA-F-]`)

// ChromeSession holds the cookie material extracted from Chrome.
type ChromeSession struct {
	ClientCookie string // raw __client value
	DeviceID     string // sanitized ajs_anonymous_id, or zero UUID
}

// ReadChromeSession pulls the __client cookie and ajs_anonymous_id from the
// user's Chrome cookie store. It prefers a __client cookie scoped to
// auth.suno.com over one on the apex/.suno.com domain. Returns ChromeSession
// with an empty ClientCookie when no __client cookie could be found.
func ReadChromeSession(ctx context.Context) (ChromeSession, error) {
	out := ChromeSession{DeviceID: zeroUUID}

	// Traverse every discovered cookie store, skipping the ones that fail to open
	// (e.g. a Chrome Canary dir with no Local State, or a profile using the older
	// Default/Cookies layout instead of Default/Network/Cookies). Collect ignores
	// per-store errors, so a single unreadable store can't abort the lookup the
	// way the simpler kooky.ReadCookies aggregate did.
	cookies := kooky.TraverseCookies(ctx, kooky.DomainHasSuffix("suno.com")).Collect(ctx)

	var authScoped, apexScoped string
	for _, c := range cookies {
		if c == nil {
			continue
		}
		domain := strings.ToLower(strings.TrimPrefix(c.Domain, "."))
		switch c.Name {
		case "__client":
			if domain == "auth.suno.com" && authScoped == "" {
				authScoped = c.Value
			} else if apexScoped == "" {
				apexScoped = c.Value
			}
		case "ajs_anonymous_id":
			if v := sanitizeDeviceID(c.Value); v != "" {
				out.DeviceID = v
			}
		}
	}

	if authScoped != "" {
		out.ClientCookie = authScoped
	} else {
		out.ClientCookie = apexScoped
	}
	return out, nil
}

// sanitizeDeviceID strips quotes/whitespace and rejects values that don't look
// UUID-ish, returning "" so the caller falls back to the zero UUID.
func sanitizeDeviceID(v string) string {
	v = strings.TrimSpace(v)
	v = strings.Trim(v, `"`)
	// Segment.io sometimes URL-encodes the quotes.
	v = strings.TrimPrefix(v, "%22")
	v = strings.TrimSuffix(v, "%22")
	if v == "" {
		return ""
	}
	if uuidish.MatchString(v) {
		return ""
	}
	return v
}
