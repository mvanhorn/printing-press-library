// Package customertier rewrites Open-Meteo URLs to route through the
// customer-tier subdomains when OPEN_METEO_API_KEY is set in the
// environment. Free-tier hosts (api.open-meteo.com,
// archive-api.open-meteo.com, marine-api.open-meteo.com, etc.) become
// customer-* hosts and the API key is appended as the apikey query
// parameter. When the env var is unset, URLs pass through unchanged so
// the free tier remains the default.
package customertier

import (
	"net/url"
	"os"
	"strings"
)

// EnvVar is the environment variable that activates customer-tier routing.
const EnvVar = "OPEN_METEO_API_KEY"

// freeToCustomer maps free-tier hosts to their customer-tier equivalents.
// Hosts not in this map (already customer-*, third-party, etc.) pass through.
var freeToCustomer = map[string]string{
	"api.open-meteo.com":             "customer-api.open-meteo.com",
	"archive-api.open-meteo.com":     "customer-archive-api.open-meteo.com",
	"marine-api.open-meteo.com":      "customer-marine-api.open-meteo.com",
	"air-quality-api.open-meteo.com": "customer-air-quality-api.open-meteo.com",
	"flood-api.open-meteo.com":       "customer-flood-api.open-meteo.com",
	"climate-api.open-meteo.com":     "customer-climate-api.open-meteo.com",
	"ensemble-api.open-meteo.com":    "customer-ensemble-api.open-meteo.com",
	"seasonal-api.open-meteo.com":    "customer-seasonal-api.open-meteo.com",
	"geocoding-api.open-meteo.com":   "customer-geocoding-api.open-meteo.com",
	"satellite-api.open-meteo.com":   "customer-satellite-api.open-meteo.com",
}

// APIKey returns the customer-tier API key from the environment, or "" when unset.
func APIKey() string {
	return strings.TrimSpace(os.Getenv(EnvVar))
}

// Active reports whether customer-tier routing should apply to outbound requests.
func Active() bool {
	return APIKey() != ""
}

// RewriteURL returns the customer-tier-routed URL when OPEN_METEO_API_KEY is set
// and the input URL is on a free-tier Open-Meteo host. The bool reports whether
// any rewrite happened. The free-tier path returns the input unchanged.
func RewriteURL(raw string) (string, bool) {
	key := APIKey()
	if key == "" {
		return raw, false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw, false
	}
	customerHost, ok := freeToCustomer[u.Host]
	if !ok {
		// Already a customer-* host or a third-party host. Still inject the apikey
		// query param when the host is a customer-* Open-Meteo host so users can
		// override the host in the spec without losing the auth append.
		if !strings.HasPrefix(u.Host, "customer-") || !strings.HasSuffix(u.Host, ".open-meteo.com") {
			return raw, false
		}
	} else {
		u.Host = customerHost
	}
	q := u.Query()
	q.Set("apikey", key)
	u.RawQuery = q.Encode()
	return u.String(), true
}
