// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Survives regeneration; the one-line call site in the
// generated client.go does not — see
// .printing-press-patches/nepra-redirect-guard.json.
//
// WHY THIS FILE EXISTS. The generated CheckRedirect callback limited hop
// count and otherwise returned nil, so this client followed a redirect
// anywhere: to http, to loopback, to a private or link-local address. A
// comment above it said "Block protocol downgrade", but that line governs
// only whether the Authorization header is re-stamped — nothing refused the
// hop itself. The claim was true of the header and false of the redirect.
//
// That matters here because every NEPRA command is a read whose body is
// written to stdout: `reliability` and `sir` emit the fetched document
// directly. A compromised or hijacked redirect would therefore turn this CLI
// into a fetch-and-print primitive aimed at whatever the redirect names,
// including a host inside the operator's own network.

package client

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
)

// nepraRedirectAllowed reports whether this client may follow a redirect,
// returning a named error when it may not.
//
// WHAT IT DOES NOT DO, stated plainly rather than implied. It inspects the
// redirect URL only. A hostname that RESOLVES to an internal address is not
// caught here, and deliberately so: a pre-resolve check gives a false sense
// of safety because DNS can return a different answer to the dialer than it
// returned to the check (rebinding). Closing that properly requires a
// Control hook on the dialer, which is a transport-level change to generated
// code; this guard closes the direct cases and does not pretend to close
// that one.
func nepraRedirectAllowed(next *url.URL, via []*http.Request) error {
	switch next.Scheme {
	case "https":
		// Always allowed.
	case "http":
		// A downgrade is refused only when some earlier hop was https:
		// a run that was plaintext from the start has nothing to lose,
		// and NEPRA_BASE_URL is set to an http httptest server by this
		// package's own tests.
		for _, hop := range via {
			if hop.URL.Scheme == "https" {
				return fmt.Errorf("refusing redirect to %s://%s: protocol downgrade from https to http",
					next.Scheme, next.Host)
			}
		}
	default:
		return fmt.Errorf("refusing redirect to %q: only http and https are followed", next.Scheme)
	}
	// A hop that stays on the host the caller already chose is not the
	// threat this guard exists for: an operator who deliberately points
	// NEPRA_BASE_URL at 127.0.0.1 — a local mirror, or this package's own
	// httptest servers — has chosen that address, and refusing its
	// self-redirect would break a legitimate setup while preventing
	// nothing. SSRF is being sent somewhere you did NOT choose.
	if len(via) > 0 && next.Host == via[0].URL.Host {
		return nil
	}
	// Hostname() strips brackets from an IPv6 literal and the port from
	// either family. A non-literal host returns nil here and is allowed.
	if ip := net.ParseIP(next.Hostname()); ip != nil && nepraIPIsInternal(ip) {
		return fmt.Errorf("refusing redirect to %s: %s is a loopback, private, link-local or "+
			"unspecified address, and this client only reads NEPRA's public documents",
			next.Host, ip)
	}
	return nil
}

// nepraIPIsInternal covers the address classes a public-document reader has
// no business being redirected to. IsPrivate covers RFC1918 and IPv6 ULA;
// the link-local cases cover 169.254.0.0/16 and fe80::/10, which is how a
// cloud instance-metadata endpoint is reached.
func nepraIPIsInternal(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsUnspecified()
}
