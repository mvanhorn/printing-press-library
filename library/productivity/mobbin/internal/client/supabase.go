// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"net/url"

	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/config"
)

// isSupabaseRESTURL reports whether a request targets Mobbin's Supabase
// PostgREST host, which needs bearer + apikey auth instead of the Mobbin
// session cookie.
func isSupabaseRESTURL(u *url.URL) bool {
	return u != nil && u.Hostname() == config.SupabaseHost
}
