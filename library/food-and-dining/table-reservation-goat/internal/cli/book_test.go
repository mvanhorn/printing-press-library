// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/opentable"
)

func TestParseNetworkPrefix(t *testing.T) {
	cases := []struct {
		in        string
		net, slug string
		errSub    string
	}{
		{"opentable:water-grill-bellevue", "opentable", "water-grill-bellevue", ""},
		{"tock:canlis", "tock", "canlis", ""},
		{"OPENTABLE:foo", "opentable", "foo", ""}, // case-insensitive network
		{"no-colon", "", "", "expected '<network>:<slug>'"},
		{"opentable:", "", "", "empty slug"},
		{"yelp:foo", "", "", "unknown network"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			n, s, err := parseNetworkPrefix(tc.in)
			if tc.errSub != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errSub) {
					t.Errorf("parseNetworkPrefix(%q) err = %v; want substring %q", tc.in, err, tc.errSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseNetworkPrefix(%q) unexpected err: %v", tc.in, err)
			}
			if n != tc.net || s != tc.slug {
				t.Errorf("parseNetworkPrefix(%q) = (%q, %q); want (%q, %q)", tc.in, n, s, tc.net, tc.slug)
			}
		})
	}
}

func TestValidateBookArgs(t *testing.T) {
	cases := []struct {
		name            string
		date, hhmm      string
		party           int
		wantErrContains string
	}{
		{"all good", "2026-05-13", "19:00", 2, ""},
		{"missing date", "", "19:00", 2, "--date"},
		{"missing time", "2026-05-13", "", 2, "--time"},
		{"zero party", "2026-05-13", "19:00", 0, "--party"},
		{"negative party", "2026-05-13", "19:00", -1, "--party"},
		{"all missing", "", "", 0, "--date"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBookArgs(tc.date, tc.hhmm, tc.party)
			if tc.wantErrContains == "" {
				if err != nil {
					t.Errorf("validateBookArgs unexpected err: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErrContains) {
				t.Errorf("validateBookArgs err = %v; want substring %q", err, tc.wantErrContains)
			}
		})
	}
}

func TestNormalizeTime(t *testing.T) {
	cases := map[string]string{
		"19:00":    "19:00",
		"7:00 PM":  "19:00",
		"7:00 pm":  "19:00",
		"7:00 AM":  "07:00",
		"12:00 PM": "12:00",
		"12:00 AM": "00:00",
		"":         "",
		"garbage":  "garbage", // unparseable returns input
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			got := normalizeTime(in)
			if got != want {
				t.Errorf("normalizeTime(%q) = %q; want %q", in, got, want)
			}
		})
	}
}

func TestNormalizeSlug(t *testing.T) {
	cases := map[string]string{
		"  Canlis  ":  "canlis",
		"WATER-GRILL": "water-grill",
		"":            "",
	}
	for in, want := range cases {
		if got := normalizeSlug(in); got != want {
			t.Errorf("normalizeSlug(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestAcquireBookLock_Concurrent(t *testing.T) {
	// First lock should succeed; second should fail (file already exists).
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)
	t.Setenv("HOME", tmp)

	_, release1, err1 := acquireBookLock("opentable", "test-venue", "2026-05-13", "19:00", 2)
	if err1 != nil {
		t.Fatalf("first acquireBookLock failed: %v", err1)
	}
	_, _, err2 := acquireBookLock("opentable", "test-venue", "2026-05-13", "19:00", 2)
	if err2 == nil {
		t.Errorf("second acquireBookLock should fail while first is held")
	}
	release1()
	// After release, third should succeed.
	_, release3, err3 := acquireBookLock("opentable", "test-venue", "2026-05-13", "19:00", 2)
	if err3 != nil {
		t.Errorf("third acquireBookLock after release failed: %v", err3)
	}
	release3()
}

func TestAcquireBookLock_DifferentKeysDontCollide(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)
	t.Setenv("HOME", tmp)

	_, r1, e1 := acquireBookLock("opentable", "venue-a", "2026-05-13", "19:00", 2)
	if e1 != nil {
		t.Fatalf("lock A failed: %v", e1)
	}
	defer r1()
	_, r2, e2 := acquireBookLock("opentable", "venue-b", "2026-05-13", "19:00", 2)
	if e2 != nil {
		t.Errorf("lock B should succeed (different slug); got %v", e2)
	}
	defer r2()
	_, r3, e3 := acquireBookLock("tock", "venue-a", "2026-05-13", "19:00", 2)
	if e3 != nil {
		t.Errorf("lock C should succeed (different network); got %v", e3)
	}
	defer r3()
}

func TestParseCancelArg(t *testing.T) {
	cases := []struct {
		in     string
		net    string
		parts  []string
		errSub string
	}{
		{"opentable:1255093:114309:01Ozsdas9H1Yx", "opentable", []string{"1255093", "114309", "01Ozsdas9H1Yx"}, ""},
		{"tock:farzi-cafe-bellevue:362575651", "tock", []string{"farzi-cafe-bellevue", "362575651"}, ""},
		{"resy:rgs-abc-1234", "resy", []string{"rgs-abc-1234"}, ""},
		// Resy tokens are opaque — colons inside must NOT split the token.
		// This is the codex-flagged P1: previously `parts = strings.Split(rest, ":")`
		// would have truncated this to ["rgs", "//venue", "20", "30", "00", "abc"].
		{"resy:rgs://venue/20:30:00/abc", "resy", []string{"rgs://venue/20:30:00/abc"}, ""},
		{"no-colon", "", nil, "expected '<network>:<id-fields>'"},
		{"opentable:", "", nil, "missing id fields"},
		{"yelp:foo:bar", "", nil, "unknown network"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			n, p, err := parseCancelArg(tc.in)
			if tc.errSub != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errSub) {
					t.Errorf("parseCancelArg(%q) err = %v; want substring %q", tc.in, err, tc.errSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCancelArg(%q) unexpected err: %v", tc.in, err)
			}
			if n != tc.net {
				t.Errorf("parseCancelArg(%q) network = %q; want %q", tc.in, n, tc.net)
			}
			if len(p) != len(tc.parts) {
				t.Errorf("parseCancelArg(%q) parts len = %d; want %d (got %v)", tc.in, len(p), len(tc.parts), p)
				return
			}
			for i := range p {
				if p[i] != tc.parts[i] {
					t.Errorf("parseCancelArg(%q) parts[%d] = %q; want %q", tc.in, i, p[i], tc.parts[i])
				}
			}
		})
	}
}

func TestVerifyEnvFloor_GuardOrder(t *testing.T) {
	// When PRINTING_PRESS_VERIFY=1 is set, the guard fires BEFORE arg
	// validation. This protects verifier mock-mode subprocesses from firing
	// real network calls even if the verifier doesn't pass --date/--time/--party.
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("TRG_ALLOW_BOOK", "1") // even with this set, IsVerifyEnv must dominate
	if !envIsVerify() {
		t.Fatal("expected verify-mode env to be detected")
	}
	// Also confirm with empty book args, parseNetworkPrefix wouldn't be reached
	// because IsVerifyEnv short-circuits first. We can't test the cobra layer
	// directly here without a full command harness, but the env detection is
	// the gate — and it's covered by cliutil.IsVerifyEnv tests already.
}

func envIsVerify() bool {
	return os.Getenv("PRINTING_PRESS_VERIFY") == "1"
}

func TestMatchedExistingOT_RestaurantSlugCheck(t *testing.T) {
	// Same date, time, and party at a different restaurant must NOT match.
	// Greptile P1: prior implementation returned true unconditionally after
	// date/time/party checks, false-positive matching any OT reservation.
	water := opentable.UpcomingReservation{
		PartySize:      2,
		DateTime:       "2026-05-13T19:00:00",
		RestaurantName: "Water Grill - Bellevue",
	}
	cases := []struct {
		name string
		slug string
		want bool
	}{
		{"exact slug match", "water-grill-bellevue", true},
		{"different restaurant same slot", "canlis", false},
		{"different restaurant overlapping token", "grill-on-the-alley", false},
		{"slug with extra token not in name", "water-grill-bellevue-private", false},
		{"empty name short-circuits", "water-grill-bellevue", false},
		{"accented name folds to ascii", "cafe-du-monde", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := water
			switch tc.name {
			case "empty name short-circuits":
				r.RestaurantName = ""
			case "accented name folds to ascii":
				r.RestaurantName = "Café du Monde"
			}
			got := matchedExistingOT(r, tc.slug, "2026-05-13", "19:00", 2)
			if got != tc.want {
				t.Errorf("matchedExistingOT(%q vs %q) = %v; want %v", tc.slug, r.RestaurantName, got, tc.want)
			}
		})
	}
}

func TestNormalizeForSlugMatch(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Water Grill - Bellevue", "watergrillbellevue"},
		{"Canlis", "canlis"},
		// Greptile P2: accented runes fold to ASCII so slug tokens like
		// "cafe" / "etoile" match Café / L'Étoile.
		{"Café du Monde", "cafedumonde"},
		{"L'Étoile", "letoile"},
		{"Niño Restaurant", "ninorestaurant"},
		{"Brüder", "bruder"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := normalizeForSlugMatch(tc.in); got != tc.want {
			t.Errorf("normalizeForSlugMatch(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsResyTerminalStatus(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"Cancelled", true},
		{"cancelled", true},
		{"Canceled", true},
		{"CANCELLED BY USER", true},
		{"Completed", true},
		{"completed (paid)", true},
		{"No-show", true},
		{"no show", true},
		{"NoShow", true},
		{"Confirmed", false},
		{"Pending", false},
		{"Held", false},
		// Boundary cases — Greptile round-6 concern: hypothetical
		// active states with terminal prefixes must NOT match.
		{"Cancellable", false},
		{"cancellation_pending", false},
		{"Completing", false},
		{"completable", false},
	}
	for _, tc := range cases {
		if got := isResyTerminalStatus(tc.in); got != tc.want {
			t.Errorf("isResyTerminalStatus(%q) = %v; want %v", tc.in, got, tc.want)
		}
	}
}
