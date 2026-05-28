// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package auth

import (
	"regexp"
	"testing"
)

// The three regexes below mirror what the Superhuman bundle validates against
// for each ID-shape. If the bundle changes the format, these tests start
// failing locally — which is the right early-warning signal.
var (
	draftIDRegex      = regexp.MustCompile(`^draft00[0-9a-f]{14}$`)
	rfc822IDRegex     = regexp.MustCompile(`^<[0-9a-f]{16}\.[0-9a-f-]{36}@we\.are\.superhuman\.com>$`)
	superhumanIDRegex = regexp.MustCompile(`^[0-9a-z]{8}\.[0-9a-f-]{36}$`)
)

// TestNewDraftID_ShapeMatchesBundleRegex asserts the strict shape Superhuman's
// /v3/userdata.writeMessage validator enforces. Length must be exactly 21
// ("draft00" prefix + 14 hex). Wrong length or non-hex characters in the
// suffix fail the request with HTTP 400 before any body field is inspected.
func TestNewDraftID_ShapeMatchesBundleRegex(t *testing.T) {
	for i := 0; i < 50; i++ {
		id := NewDraftID()
		if !draftIDRegex.MatchString(id) {
			t.Fatalf("draft id %q does not match %s", id, draftIDRegex)
		}
		if len(id) != 21 {
			t.Fatalf("draft id %q has length %d, want 21", id, len(id))
		}
	}
}

// TestNewDraftID_Unique sanity-checks that two consecutive calls do not collide.
// crypto/rand provides 14 hex of entropy (56 bits) so the birthday probability
// of a collision in 50 calls is on the order of 1e-14 — effectively impossible.
func TestNewDraftID_Unique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		id := NewDraftID()
		if seen[id] {
			t.Fatalf("duplicate draft id generated: %q", id)
		}
		seen[id] = true
	}
}

// TestNewRFC822ID_ShapeMatchesBundleRegex asserts the literal-angle-bracket
// shape Superhuman parses. The literal angle brackets ARE part of the value
// (not log-line decoration) and the @we.are.superhuman.com host is required.
func TestNewRFC822ID_ShapeMatchesBundleRegex(t *testing.T) {
	for i := 0; i < 50; i++ {
		id := NewRFC822ID()
		if !rfc822IDRegex.MatchString(id) {
			t.Fatalf("rfc822 id %q does not match %s", id, rfc822IDRegex)
		}
		if id[0] != '<' || id[len(id)-1] != '>' {
			t.Fatalf("rfc822 id %q missing literal angle brackets", id)
		}
	}
}

// TestNewSuperhumanID_ShapeMatchesBundleRegex asserts the
// <8-char-base36>.<uuid> shape per edwinhu's reference. The base36 prefix
// gives analytics a coarse send-time ordering signal; the uuid suffix is the
// per-send unique identifier.
func TestNewSuperhumanID_ShapeMatchesBundleRegex(t *testing.T) {
	for i := 0; i < 50; i++ {
		id := NewSuperhumanID()
		if !superhumanIDRegex.MatchString(id) {
			t.Fatalf("superhuman id %q does not match %s", id, superhumanIDRegex)
		}
	}
}

// TestNewSuperhumanID_Base36PrefixIsExactlyEight covers the clamp behaviour
// in NewSuperhumanID: a base36 timestamp of fewer than 8 chars is left-padded,
// more than 8 (very-far-future epoch) is truncated to the trailing 8.
func TestNewSuperhumanID_Base36PrefixIsExactlyEight(t *testing.T) {
	id := NewSuperhumanID()
	// Prefix is everything before the first dot.
	for i := 0; i < len(id); i++ {
		if id[i] == '.' {
			if i != 8 {
				t.Fatalf("superhuman id %q base36 prefix length %d, want 8", id, i)
			}
			return
		}
	}
	t.Fatalf("superhuman id %q has no '.' separator", id)
}
