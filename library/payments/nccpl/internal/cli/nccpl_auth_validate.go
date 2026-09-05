package cli

import "strings"

// nccplStdlibValidationIsMeaningless reports whether a stdlib HTTP probe can say
// anything useful about the freshness of these credentials.
//
// This CLI ships Surf browser-compatible transport because the origin sits behind
// Cloudflare. A `cf_clearance` cookie is bound to the TLS/JA3 fingerprint that earned
// it, so presenting it from Go's stdlib client is treated as token abuse and answered
// with a 403 "you have been blocked" page -- regardless of whether the cookie is
// valid. Measured during this build: the identical cookie jar returns 403 from stdlib
// while an unauthenticated probe of the same origin returns only `cf-mitigated:
// challenge`, i.e. the hard block is caused by the fingerprint mismatch, not by an
// expired session.
//
// The generated validateComposedAuth uses a stdlib client and maps 403 to
// "session expired", which is a false negative that makes `auth login --chrome`
// unreachable for every browser-clearance CLI. Skip that probe when a clearance
// cookie is present; real staleness still surfaces on the first actual API call,
// which goes through Surf.
func nccplStdlibValidationIsMeaningless(cookieHeader string) bool {
	return strings.Contains(cookieHeader, "cf_clearance=")
}
