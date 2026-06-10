// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.

// Package autotempest holds hand-authored AutoTempest-specific logic shared by
// the find/watch live-search commands: the request-signing token algorithm and
// the listing field parsers. It is intentionally free of any cobra/CLI imports
// so it can be unit-tested in isolation.
package autotempest

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
)

// tokenSalt is the constant SALT AutoTempest's frontend appends before hashing
// the ordered query params to produce the request token. Verified against the
// live site; see token_test.go for the golden vector.
const tokenSalt = "d8007486d73c168684860aae427ea1f9d74e502b06d94609691f5f4f2704a07f" // #nosec G101 -- public client-side salt shipped in AutoTempest's frontend JS, not a secret credential

// encodeURIComponent matches JS encodeURIComponent (space -> %20, not +).
// url.QueryEscape encodes a space as '+'; we rewrite it so the server's
// decodeURIComponent round-trips to the same raw value it hashed.
func encodeURIComponent(v string) string {
	return strings.ReplaceAll(url.QueryEscape(v), "+", "%20")
}

// KV is an ordered query parameter. Order is load-bearing: the server validates
// the token against the params in the order they are received, and the hash
// input must match that same order.
type KV struct{ K, V string }

// SignedQuery returns the full query string (without the leading '?') for the
// ordered params, with a &token=... appended. Each param value is sent through
// encodeURIComponent in the URL, while the RAW (un-encoded) value is what feeds
// the sha256 hash — because decodeURIComponent(encodeURIComponent(v)) === v,
// the server recomputes the identical token.
//
// Build the URL with this helper rather than net/url.Values: url.Values sorts
// keys and uses '+' for spaces, both of which would break the signature.
func SignedQuery(params []KV) string {
	var hashB, urlB strings.Builder
	for i, p := range params {
		if i > 0 {
			hashB.WriteByte('&')
			urlB.WriteByte('&')
		}
		hashB.WriteString(p.K)
		hashB.WriteByte('=')
		hashB.WriteString(p.V) // RAW value
		urlB.WriteString(p.K)
		urlB.WriteByte('=')
		urlB.WriteString(encodeURIComponent(p.V)) // encoded value
	}
	sum := sha256.Sum256([]byte(hashB.String() + tokenSalt))
	urlB.WriteString("&token=")
	urlB.WriteString(hex.EncodeToString(sum[:]))
	return urlB.String()
}
