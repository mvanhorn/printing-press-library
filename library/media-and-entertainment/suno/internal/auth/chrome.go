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

// SunoCookie is a browser cookie for a Suno/Clerk domain, in a shape the
// captcha solver can map directly to CDP Network.setCookies params.
type SunoCookie struct {
	Name     string
	Value    string
	Domain   string
	Path     string
	Secure   bool
	HTTPOnly bool
}

// ReadSunoCookies returns every cookie on suno.com / auth.suno.com / .suno.com
// from the user's Chrome store — the full jar, including the Clerk __client and
// __session cookies. The captcha solver re-seeds from this on every solve so the
// dedicated profile's session can never go stale (this is what keeps us clear of
// paperfoot/suno-cli#3's popup); SunoStudioCookieHeader also reads it. Returns
// an empty slice (not an error) when none are found.
func ReadSunoCookies(ctx context.Context) ([]SunoCookie, error) {
	raw := kooky.TraverseCookies(ctx, kooky.DomainHasSuffix("suno.com")).Collect(ctx)
	out := make([]SunoCookie, 0, len(raw))
	for _, c := range raw {
		if c == nil || c.Name == "" {
			continue
		}
		path := c.Path
		if path == "" {
			path = "/"
		}
		out = append(out, SunoCookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     path,
			Secure:   c.Secure,
			HTTPOnly: c.HttpOnly,
		})
	}
	return out, nil
}

// SunoStudioCookieHeader builds a "name=value; ..." Cookie header from the
// browser's stored cookies that a real browser sends to
// studio-api-prod.suno.com — the apex (.suno.com / suno.com) session and
// analytics cookies. Suno's WAF cross-checks these against the Bearer JWT and
// returns 422 token_validation_failed without them. Returns "" when none found.
func SunoStudioCookieHeader(ctx context.Context) string {
	cs, err := ReadSunoCookies(ctx)
	if err != nil {
		return ""
	}
	return studioCookieHeader(cs)
}

// studioCookieHeader is the pure filter+join behind SunoStudioCookieHeader:
// keep only cookies a browser attaches to studio-api-prod.suno.com (apex
// suno.com host + domain-scoped, and the studio-api host itself); drop other
// subdomains (auth.suno.com, hcaptcha-*.suno.com).
func studioCookieHeader(cs []SunoCookie) string {
	var b strings.Builder
	for _, c := range cs {
		if c.Name == "" {
			continue
		}
		dom := strings.ToLower(strings.TrimPrefix(c.Domain, "."))
		if dom != "suno.com" && dom != "studio-api-prod.suno.com" {
			continue
		}
		// The rotating Clerk __session* cookies are not required by the studio
		// WAF (verified: the feed returns 200 without them) and would go stale
		// in a long-lived cache; drop them so we only cache durable cookies.
		if strings.HasPrefix(c.Name, "__session") {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("; ")
		}
		b.WriteString(c.Name)
		b.WriteString("=")
		b.WriteString(c.Value)
	}
	return b.String()
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
