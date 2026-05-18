// Copyright 2026 greatclips. Licensed under Apache-2.0. See LICENSE.

package icssign

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"sync"
	"time"
)

// Sign returns the URL-safe base64 signature for input, computed as
// HMAC-SHA-256(derivedKey, input), where derivedKey is the seed XORed
// with HMAC-SHA-256(empty_key, "Online Check-In").
//
// **The SPA preserves the trailing "=" padding character** — its JS
// code is `btoa(sig).replaceAll("+","-").replaceAll("/","_")` with no
// `.replace(/=+$/,”)`. The server validates the literal string
// including padding, so we must use URLEncoding (with `=`), NOT
// RawURLEncoding. A v0.2 bug used RawURLEncoding and every
// stylewaretouch call returned HTTP 400 "Invalid request".
//
// The output is always 44 ASCII characters.
func Sign(input string) string {
	key := derivedKey()
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(input))
	return base64.URLEncoding.EncodeToString(mac.Sum(nil))
}

// Timestamp returns the current time as a millisecond-epoch decimal
// string, matching the JS Date.now().toString() the SPA uses.
//
// Exposed for test injection: callers may compute their own timestamp
// when they need byte-for-byte determinism against a captured browser
// request.
func Timestamp() string {
	return strconv.FormatInt(time.Now().UnixMilli(), 10)
}

// SignRequest is a convenience wrapper that signs the concatenation of
// timestamp and body and returns both. The timestamp goes in the URL's
// ?t= parameter; the signature goes in ?s=.
//
// Passing an empty body is correct for GET requests; the SPA signs only
// the timestamp in that case.
func SignRequest(timestamp, body string) (t, s string) {
	return timestamp, Sign(timestamp + body)
}

var (
	derivedKeyOnce sync.Once
	derivedKeyBuf  [32]byte
)

// derivedKey returns the per-instance signing key:
//
//	innerKey = HMAC-SHA-256(key="Online Check-In" bytes, msg="")
//	out[i]   = seed[i] XOR innerKey[i]   for i in 0..31
//
// The JS reference is _(seed, "Online Check-In") which calls
// y(encode("Online Check-In"), "") — y's first argument is the key,
// second is the message. The label string is the HMAC key here, not
// the message.
//
// The result is cached after first computation; it never changes during
// a process lifetime.
func derivedKey() []byte {
	derivedKeyOnce.Do(func() {
		mac := hmac.New(sha256.New, []byte(label)) // label bytes as HMAC key
		mac.Write([]byte{})                        // empty message
		inner := mac.Sum(nil)
		for i := 0; i < 32; i++ {
			derivedKeyBuf[i] = seed[i] ^ inner[i]
		}
	})
	return derivedKeyBuf[:]
}
