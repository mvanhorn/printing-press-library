// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.

package agoda

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// newRequestID returns a random RFC-4122-shaped v4 identifier.
//
// Agoda's schema types searchId and the AG-REQUEST-ID header as non-nullable
// strings, so a value is always required. Generating a fresh one per call also
// avoids replaying a single captured session identifier across every request.
func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// A non-random fallback is acceptable here: this value is a correlation
		// token, not a security boundary.
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}
