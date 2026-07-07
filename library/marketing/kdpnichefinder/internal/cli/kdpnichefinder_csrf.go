// Hand-authored, not generated. Laravel CSRF support for write endpoints.
//
// KDP Niche Finder is a Laravel app: GET requests authenticate with the
// session cookie alone, but state-changing POSTs additionally require an
// X-XSRF-TOKEN header whose value is the (URL-decoded) XSRF-TOKEN cookie.
// Cookie auth stores the captured cookie string in Config.AccessToken (see
// auth login --chrome -> SaveTokens), so we parse XSRF-TOKEN out of it here
// and hand it to PostWithParamsAndHeaders on the save/create commands.
package cli

import (
	"net/url"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/kdpnichefinder/internal/client"
)

// kdpCSRFHeaders returns the X-XSRF-TOKEN header for Laravel write requests,
// or an empty map when no XSRF-TOKEN cookie is available (e.g. not logged in).
// Returning an empty (non-nil) map is safe to pass to *WithHeaders helpers.
func kdpCSRFHeaders(c *client.Client) map[string]string {
	headers := map[string]string{}
	if c == nil || c.Config == nil {
		return headers
	}
	// The captured cookie string lives in AccessToken for cookie auth; fall
	// back to the legacy auth_header value if that is where it landed.
	for _, src := range []string{c.Config.AccessToken, c.Config.AuthHeaderVal} {
		if tok := xsrfFromCookieString(src); tok != "" {
			headers["X-XSRF-TOKEN"] = tok
			return headers
		}
	}
	return headers
}

// xsrfFromCookieString extracts and URL-decodes the XSRF-TOKEN cookie value
// from a "name=value; name2=value2" cookie string. Laravel stores the token
// URL-encoded in the cookie; the header must carry the decoded value.
func xsrfFromCookieString(cookieStr string) string {
	if cookieStr == "" {
		return ""
	}
	for _, part := range strings.Split(cookieStr, ";") {
		part = strings.TrimSpace(part)
		name, value, found := strings.Cut(part, "=")
		if !found {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(name), "XSRF-TOKEN") {
			if decoded, err := url.QueryUnescape(value); err == nil {
				return decoded
			}
			return value
		}
	}
	return ""
}
