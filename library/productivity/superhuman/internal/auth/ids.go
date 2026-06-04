// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// ids.go generates the three identifier shapes the Superhuman send pipeline
// requires:
//
//   - draftId: "draft00" + 14 hex chars. Validated by the bundle's regex
//     `^draft00[0-9a-f]{14}$`. Wrong shape returns 400 from
//     /v3/userdata.writeMessage before any body validation runs.
//   - rfc822Id: "<random.<uuid>@we.are.superhuman.com>" with literal angle
//     brackets. The "random" prefix is 16 hex chars by convention. Used as
//     the on-the-wire RFC822 Message-Id header value.
//   - superhuman_id: "<base36(ts)>.<uuid>" where ts = time.Now().Unix()
//     clamped to 8 base36 characters. Per edwinhu/superhuman-cli reference
//     (src/draft-api.ts:648), the JS implementation is
//     Date.now()/1000 .toString(36).
//
// All three are generated locally before any HTTP call so a bad shape fails
// fast as a typed Go error rather than a cryptic 400 from the backend.

package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// NewDraftID returns a fresh draft identifier in the form
//
//	"draft00" + <14 lowercase hex chars>
//
// The bundle's writeMessage validator pins this exact shape with regex
// `^draft00[0-9a-f]{14}$` (length 21). Any other shape fails the request with
// HTTP 400 before any body content is inspected.
//
// crypto/rand.Read returns nil error on every supported platform — the OS
// random source not being available is an unrecoverable runtime fault, so a
// panic here is the right surfacing (matches the convention in
// google/uuid.NewString()).
func NewDraftID() string {
	b := make([]byte, 7) // 7 bytes = 14 hex chars
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Errorf("draftID rand: %w", err))
	}
	return "draft00" + hex.EncodeToString(b)
}

// NewRFC822ID returns a fresh RFC822 Message-Id in the form
//
//	"<<16 hex chars>.<uuid>@we.are.superhuman.com>"
//
// The literal angle brackets are part of the value Superhuman expects — they
// are NOT decorative quoting from a log line. The "@we.are.superhuman.com"
// host is hard-coded because the bundle uses that domain for every outbound
// CLI-style send; Superhuman's downstream MTAs do not parse the host as a
// routing hint.
func NewRFC822ID() string {
	b := make([]byte, 8) // 8 bytes = 16 hex chars
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Errorf("rfc822ID rand: %w", err))
	}
	return "<" + hex.EncodeToString(b) + "." + uuid.NewString() + "@we.are.superhuman.com>"
}

// NewSuperhumanID returns a fresh "superhuman_id" in the form
//
//	"<base36(ts)>.<uuid>"
//
// where ts is time.Now().Unix() formatted in base36 and zero-padded /
// truncated to exactly 8 characters. The reference TypeScript implementation
// in edwinhu/superhuman-cli does Date.now() / 1000 .toString(36) which yields
// 7–8 chars at current epoch values; we clamp to 8 so the regex
// `^[0-9a-z]{8}\.[0-9a-f-]{36}$` matches deterministically.
//
// The base36 prefix gives the analytics pipeline a coarse send-time ordering
// without needing to parse the full uuid suffix.
func NewSuperhumanID() string {
	ts := strconv.FormatInt(time.Now().Unix(), 36)
	if len(ts) > 8 {
		ts = ts[len(ts)-8:]
	}
	for len(ts) < 8 {
		ts = "0" + ts
	}
	return ts + "." + uuid.NewString()
}
