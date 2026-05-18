// Copyright 2026 greatclips. Licensed under Apache-2.0. See LICENSE.

// Package icssign implements the request-signing scheme that the
// GreatClips Online Check-In SPA uses for every call to
// www.stylewaretouch.net (the ICS Net Check-In service).
//
// The scheme is a custom HMAC-SHA-256 construction with a 32-byte seed
// hardcoded in the SPA's JavaScript bundle. The signing input is the
// concatenation of a millisecond-epoch timestamp string and the request
// body. The 43-character URL-safe base64 (no padding) signature is
// appended to every stylewaretouch URL as ?t=<timestamp>&s=<signature>.
//
// Without these query parameters, the stylewaretouch service returns
// HTTP 500 with a leaked Java NullPointerException on the missing
// parameter.
package icssign

// seed is the 32-byte secret hardcoded in the SPA's JS bundle
// (file 01000ffd9a85230f.js as of 2026-05-11). The bundle stores it
// as a comma-separated string of signed-byte integers; we encode it
// as unsigned bytes here.
//
// The seed is rotated only when GreatClips redeploys with a new
// bundle. If wait-time or check-in calls start returning 500 with a
// signature-rejection envelope, this is the first place to look.
var seed = [32]byte{
	0xb8, 0x93, 0x44, 0xc1, 0x4a, 0x1b, 0x90, 0xbb,
	0x83, 0x30, 0xae, 0x52, 0x33, 0x98, 0x72, 0xec,
	0xbd, 0x6b, 0xa9, 0x2a, 0x9e, 0xf3, 0xb8, 0xa8,
	0xe5, 0xe9, 0xe0, 0xb1, 0x9c, 0xe1, 0xd1, 0x4c,
}

// label is the static string used to derive the per-instance signing
// key from seed. The SPA passes this verbatim to its key-derivation
// function.
const label = "Online Check-In"
