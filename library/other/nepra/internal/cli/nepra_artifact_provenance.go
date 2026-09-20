// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// nepraArtifact is machine-checkable provenance for one fetched document.
//
// The absorb manifest's parity target here was ebillpakistan.pk's NEPRA_SOURCE
// object, which records the source of each tariff figure as prose in a JS
// file. This is the beat-it version: every field is measured at fetch time and
// checkable afterwards, so a stored row can be re-derived from the exact bytes
// it came from.
//
// The generated DataProvenance carries only {source, synced_at, reason,
// resource_type, freshness}, and widening a generated struct would be reverted
// by the next regen — so this rides alongside it.
type nepraArtifact struct {
	URL string `json:"url"`
	// Bytes is the DECODED length: what the parser actually saw, after any
	// Content-Encoding was removed. The compressed transfer size is a
	// property of the connection, not of the document.
	Bytes int `json:"bytes"`
	// SHA256Raw is over those same decoded bytes.
	//
	// IT IS NOT A DOCUMENT IDENTITY AND MUST NOT BE USED FOR CHANGE
	// DETECTION. MEASURED: two identical fetches of one determination page,
	// seconds apart, returned the same 74,431 bytes with DIFFERENT hashes,
	// because Cloudflare's email-obfuscation filter rewrites every
	// data-cfemail token with a fresh key on each response. The page also
	// varies with the request's Accept header — 74,431 bytes against 74,792
	// for this client's own Accept. So a raw hash records what THIS fetch
	// received, nothing more.
	SHA256Raw string `json:"sha256_raw"`
	// SHA256Content is over the EXTRACTED DATA rather than the markup, and it
	// is the hash to compare across fetches: it is blind to the rotating
	// obfuscation tokens, to Accept-driven boilerplate and to whitespace, so
	// it changes only when the published figures change.
	SHA256Content string `json:"sha256_content"`
	FetchedAt     string `json:"fetched_at"`
	ContentType   string `json:"content_type,omitempty"`
	// Rows is how many rows this artifact yielded, so a completeness claim
	// travels with the bytes it was computed from.
	Rows int `json:"rows"`
	// SkippedRows is how many table rows were read but not emitted. A
	// non-zero value here with no explanation is a reason to distrust Rows.
	SkippedRows int `json:"skipped_rows,omitempty"`
	// NOTE: http_status is deliberately ABSENT. The generated client returns
	// the body without the status code, and every path that reaches this
	// point has already succeeded — so recording "200" would be asserting a
	// value that was never observed. See the upstream deferral for a client
	// accessor.
}

// newNepraArtifact records provenance for one fetched document. content is the
// extracted data the rows were built from; hashing it rather than the markup
// is what makes the hash comparable across fetches.
func newNepraArtifact(url string, body []byte, content []byte, contentType string, rows, skipped int) nepraArtifact {
	raw := sha256.Sum256(body)
	stable := sha256.Sum256(content)
	return nepraArtifact{
		URL:           url,
		Bytes:         len(body),
		SHA256Raw:     hex.EncodeToString(raw[:]),
		SHA256Content: hex.EncodeToString(stable[:]),
		FetchedAt:     time.Now().UTC().Format(time.RFC3339),
		ContentType:   contentType,
		Rows:          rows,
		SkippedRows:   skipped,
	}
}
