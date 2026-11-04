// PATCH (instacart-regional-host): the Instacart site is selectable so
// regional storefronts such as www.instacart.ca work. The host drives the
// GraphQL endpoint, Origin/Referer headers and browser cookie filtering.

package auth

import (
	"fmt"
	"os"
	"strings"
)

// DefaultHost is the US site and stays the default.
const DefaultHost = "www.instacart.com"

// HostEnv overrides the configured host for one invocation.
const HostEnv = "INSTACART_HOST"

var activeHost = DefaultHost

// knownDomains are the Instacart sites this CLI accepts. Only domains
// Instacart operates are listed, so session cookies never go to a host
// someone else might own.
var knownDomains = []string{"instacart.com", "instacart.ca"}

// NormalizeHost strips an optional scheme and trailing slash and accepts only
// a known Instacart domain, bare or with "www.".
func NormalizeHost(h string) (string, error) {
	h = strings.ToLower(strings.TrimSpace(h))
	h = strings.TrimPrefix(strings.TrimPrefix(h, "https://"), "http://")
	h = strings.TrimSuffix(h, "/")
	if h == "" {
		return DefaultHost, nil
	}
	for _, d := range knownDomains {
		if h == d || h == "www."+d {
			return h, nil
		}
	}
	return "", fmt.Errorf("unsupported Instacart host %q: want one of www.instacart.com, www.instacart.ca", h)
}

// ResolveHost picks the host: $INSTACART_HOST, then the configured value,
// then DefaultHost.
func ResolveHost(configured string) (string, error) {
	if env := strings.TrimSpace(os.Getenv(HostEnv)); env != "" {
		return NormalizeHost(env)
	}
	return NormalizeHost(configured)
}

// SetHost sets the host used by this process. h must already be normalized.
func SetHost(h string) {
	if h == "" {
		h = DefaultHost
	}
	activeHost = h
}

// Host returns the active hostname, e.g. "www.instacart.ca".
func Host() string { return activeHost }

// BaseURL returns the active site origin, e.g. "https://www.instacart.ca".
func BaseURL() string { return "https://" + activeHost }

// CookieDomain returns the registrable domain used to match cookies,
// e.g. "instacart.ca".
func CookieDomain() string { return strings.TrimPrefix(activeHost, "www.") }

// cookieDomainMatches reports whether a cookie domain belongs to the active
// site, e.g. ".instacart.ca" or "www.instacart.ca" for www.instacart.ca.
func cookieDomainMatches(domain string) bool {
	d := strings.TrimPrefix(strings.ToLower(domain), ".")
	base := CookieDomain()
	return d == base || strings.HasSuffix(d, "."+base)
}
