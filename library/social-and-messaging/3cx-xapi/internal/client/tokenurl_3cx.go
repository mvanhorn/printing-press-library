// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: derive the OAuth2 token endpoint from the configured base URL.

package client

import (
	"net/url"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/3cx-xapi/internal/config"
)

// defaultTokenURL is empty: there is no baked instance. The token endpoint is
// always derived from the configured base URL host (3CX serves /connect/token
// at the host root). When the base URL is unset/unparseable, this returns ""
// and the caller surfaces a base-URL / TCX_FQDN configuration error.
const defaultTokenURL = ""

// resolveOAuthTokenURL returns the OAuth2 client-credentials token endpoint.
// An explicit Config.TokenURL (set directly or via the API_3CX_XAPI_TOKEN_URL
// env override) wins. Otherwise it derives "<scheme>://<host>/connect/token"
// from the configured BaseURL so the token mint always targets the same 3CX
// instance as the API — and automatically follows any base-URL override
// (e.g. TCX_FQDN / API_3CX_XAPI_BASE_URL / printing-press verify mock servers).
// The 3CX token endpoint always lives at the host root, sibling to /xapi/v1.
func resolveOAuthTokenURL(cfg *config.Config) string {
	if cfg == nil {
		return defaultTokenURL
	}
	if cfg.TokenURL != "" {
		return cfg.TokenURL
	}
	if u, err := url.Parse(cfg.BaseURL); err == nil && u.Scheme != "" && u.Host != "" {
		return u.Scheme + "://" + u.Host + "/connect/token"
	}
	return defaultTokenURL
}
