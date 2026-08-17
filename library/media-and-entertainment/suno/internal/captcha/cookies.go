// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// Per-solve seeding: map the user's full Suno cookie jar (read live from their
// main browser on every solve) into CDP Network.setCookies params. Re-seeding
// each call — rather than trusting the dedicated profile to persist the session
// — keeps the Clerk/Google-SSO session fresh and avoids paperfoot/suno-cli#3's
// per-generation "Continue as <user>" OAuth popup (a stale dedicated-profile
// session that can no longer re-auth silently).

package captcha

import (
	"context"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/auth"
)

// CDPCookie is the subset of CDP Network.CookieParam we set. Kept local so the
// CLI/auth layers don't depend on cdproto.
type CDPCookie struct {
	Name     string
	Value    string
	Domain   string
	Path     string
	Secure   bool
	HTTPOnly bool
}

// toCDPCookies maps auth.SunoCookie values to CDPCookie, dropping nameless
// entries and defaulting an empty path to "/".
func toCDPCookies(in []auth.SunoCookie) []CDPCookie {
	out := make([]CDPCookie, 0, len(in))
	for _, c := range in {
		if c.Name == "" {
			continue
		}
		path := c.Path
		if path == "" {
			path = "/"
		}
		out = append(out, CDPCookie{
			Name: c.Name, Value: c.Value, Domain: c.Domain,
			Path: path, Secure: c.Secure, HTTPOnly: c.HTTPOnly,
		})
	}
	return out
}

// seedFromBrowser is the default SeedFunc: read Suno cookies from the user's
// main browser and map them to CDP params.
func seedFromBrowser(ctx context.Context) ([]CDPCookie, error) {
	raw, err := auth.ReadSunoCookies(ctx)
	if err != nil {
		return nil, err
	}
	return toCDPCookies(raw), nil
}
