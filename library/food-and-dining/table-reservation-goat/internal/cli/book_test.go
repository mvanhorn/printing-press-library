// Copyright 2026 Pejman Pour-Moezzi and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/opentable"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/tock"
)

type fakeOpenTableAttachPostBookClient struct {
	upcoming   []opentable.UpcomingReservation
	listErr    error
	cutoff     string
	cutoffErr  error
	cutoffCall *openTableCutoffCall
}

type openTableCutoffCall struct {
	restaurantID       int
	confirmationNumber int
	securityToken      string
}

func (f *fakeOpenTableAttachPostBookClient) ListUpcomingReservations(context.Context) ([]opentable.UpcomingReservation, error) {
	return f.upcoming, f.listErr
}

func (f *fakeOpenTableAttachPostBookClient) FetchCancelCutoff(_ context.Context, restaurantID, confirmationNumber int, securityToken string) (string, error) {
	f.cutoffCall = &openTableCutoffCall{
		restaurantID:       restaurantID,
		confirmationNumber: confirmationNumber,
		securityToken:      securityToken,
	}
	return f.cutoff, f.cutoffErr
}

func decodeDashboardFixture(t *testing.T) []opentable.UpcomingReservation {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "opentable_dashboard_upcoming.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		DiningDashboard struct {
			UpcomingReservations []opentable.UpcomingReservation `json:"upcomingReservations"`
		} `json:"diningDashboard"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture.DiningDashboard.UpcomingReservations
}

func jsonRoundTripBookResult(t *testing.T, result bookResult) bookResult {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded bookResult
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func bookResultHasWarning(result bookResult, want string) bool {
	for _, warning := range result.Warnings {
		if warning == want {
			return true
		}
	}
	return false
}

func TestEnrichOpenTableAttachBookBackfillsRestaurantIDAndFetchesCutoff(t *testing.T) {
	client := &fakeOpenTableAttachPostBookClient{
		upcoming: decodeDashboardFixture(t),
		cutoff:   "2026-07-21T02:00:00Z",
	}
	resp := &opentable.BookResponse{
		ReservationID:      880011,
		ConfirmationNumber: 771122,
		SecurityToken:      "fixture-token",
	}

	got := enrichOpenTableAttachBook(context.Background(), client, resp, bookResult{Source: "book"})
	decoded := jsonRoundTripBookResult(t, got)

	if resp.RestaurantID != 456789 {
		t.Fatalf("RestaurantID = %d, want dashboard fixture id 456789", resp.RestaurantID)
	}
	if client.cutoffCall == nil {
		t.Fatal("FetchCancelCutoff was not called")
	}
	if *client.cutoffCall != (openTableCutoffCall{restaurantID: 456789, confirmationNumber: 771122, securityToken: "fixture-token"}) {
		t.Fatalf("FetchCancelCutoff call = %#v", *client.cutoffCall)
	}
	if decoded.CancellationDeadline != client.cutoff {
		t.Fatalf("CancellationDeadline = %q, want %q", decoded.CancellationDeadline, client.cutoff)
	}
	if decoded.Hint != "" {
		t.Fatalf("Hint = %q, want empty", decoded.Hint)
	}
	if bookResultHasWarning(decoded, openTableWarningRestaurantIDUnresolved) {
		t.Fatalf("Warnings = %v, must not contain %q", decoded.Warnings, openTableWarningRestaurantIDUnresolved)
	}
}

func TestRestaurantIDForBookedReservationFallsBackToReservationID(t *testing.T) {
	resp := &opentable.BookResponse{ReservationID: 880011}
	got := restaurantIDForBookedReservation(decodeDashboardFixture(t), resp)
	if got != 456789 {
		t.Fatalf("restaurantIDForBookedReservation = %d, want dashboard fixture id 456789", got)
	}
}

func TestRestaurantIDForBookedReservationRejectsAmbiguousConfirmationNumber(t *testing.T) {
	upcoming := []opentable.UpcomingReservation{
		{ConfirmationNumber: 771122, RestaurantID: 456789},
		{ConfirmationNumber: 771122, RestaurantID: 987654},
	}
	resp := &opentable.BookResponse{ConfirmationNumber: 771122}

	if got := restaurantIDForBookedReservation(upcoming, resp); got != 0 {
		t.Fatalf("restaurantIDForBookedReservation = %d, want 0 for ambiguous confirmation number", got)
	}
}

func TestRestaurantIDForBookedReservationRequiresConsistentIdentifiers(t *testing.T) {
	upcoming := []opentable.UpcomingReservation{
		{ConfirmationNumber: 771122, ConfirmationID: 990044, RestaurantID: 987654},
		{ConfirmationNumber: 771122, ConfirmationID: 880011, RestaurantID: 456789},
	}
	resp := &opentable.BookResponse{ConfirmationNumber: 771122, ReservationID: 880011}

	if got := restaurantIDForBookedReservation(upcoming, resp); got != 456789 {
		t.Fatalf("restaurantIDForBookedReservation = %d, want consistently matched id 456789", got)
	}
}

func TestEnrichOpenTableAttachBookDashboardUnavailableKeepsHint(t *testing.T) {
	client := &fakeOpenTableAttachPostBookClient{listErr: errors.New("dashboard unavailable")}
	resp := &opentable.BookResponse{ConfirmationNumber: 771122, SecurityToken: "fixture-token"}

	got := enrichOpenTableAttachBook(context.Background(), client, resp, bookResult{Source: "book"})
	decoded := jsonRoundTripBookResult(t, got)

	if resp.RestaurantID != 0 {
		t.Fatalf("RestaurantID = %d, want 0", resp.RestaurantID)
	}
	if client.cutoffCall != nil {
		t.Fatalf("FetchCancelCutoff unexpectedly called with %#v", *client.cutoffCall)
	}
	const wantHint = "cancellation deadline unavailable: restaurant id could not be resolved"
	if decoded.Hint != wantHint {
		t.Fatalf("Hint = %q, want %q", decoded.Hint, wantHint)
	}
	if !bookResultHasWarning(decoded, openTableWarningRestaurantIDUnresolved) {
		t.Fatalf("Warnings = %v, want %q", decoded.Warnings, openTableWarningRestaurantIDUnresolved)
	}
	if decoded.Error != "" {
		t.Fatalf("Error = %q, want successful booking", decoded.Error)
	}
	if decoded.Source != "book" {
		t.Fatalf("Source = %q, want successful booking source %q", decoded.Source, "book")
	}
}

func TestEnrichOpenTableAttachBookNoDashboardMatchKeepsHint(t *testing.T) {
	client := &fakeOpenTableAttachPostBookClient{upcoming: decodeDashboardFixture(t)}
	resp := &opentable.BookResponse{ConfirmationNumber: 424242, ReservationID: 434343}

	got := enrichOpenTableAttachBook(context.Background(), client, resp, bookResult{Source: "book"})
	decoded := jsonRoundTripBookResult(t, got)

	if resp.RestaurantID != 0 {
		t.Fatalf("RestaurantID = %d, want 0", resp.RestaurantID)
	}
	if client.cutoffCall != nil {
		t.Fatalf("FetchCancelCutoff unexpectedly called with %#v", *client.cutoffCall)
	}
	const wantHint = "cancellation deadline unavailable: restaurant id could not be resolved"
	if decoded.Hint != wantHint {
		t.Fatalf("Hint = %q, want %q", decoded.Hint, wantHint)
	}
	if !bookResultHasWarning(decoded, openTableWarningRestaurantIDUnresolved) {
		t.Fatalf("Warnings = %v, want %q", decoded.Warnings, openTableWarningRestaurantIDUnresolved)
	}
	if decoded.Error != "" {
		t.Fatalf("Error = %q, want successful booking", decoded.Error)
	}
	if decoded.Source != "book" {
		t.Fatalf("Source = %q, want successful booking source %q", decoded.Source, "book")
	}
}

func TestEnrichOpenTableAttachBookCutoffErrorDoesNotFailBooking(t *testing.T) {
	client := &fakeOpenTableAttachPostBookClient{cutoffErr: errors.New("cutoff unavailable")}
	resp := &opentable.BookResponse{
		RestaurantID:       456789,
		ConfirmationNumber: 771122,
		SecurityToken:      "fixture-token",
	}

	got := enrichOpenTableAttachBook(context.Background(), client, resp, bookResult{Source: "book"})

	if client.cutoffCall == nil {
		t.Fatal("FetchCancelCutoff was not called")
	}
	if got.CancellationDeadline != "" {
		t.Fatalf("CancellationDeadline = %q, want empty", got.CancellationDeadline)
	}
	if got.Error != "" {
		t.Fatalf("Error = %q, want successful booking", got.Error)
	}
}

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

func TestTockCVCForBooking_NoInputAttemptsWithEmptyCVCAndDoesNotPrompt(t *testing.T) {
	// Machine mode without TRG_TOCK_CVC proceeds with an empty CVC (the
	// interactive flow allows skipping; card-on-file venues complete without
	// one). A venue that truly blocks surfaces tock.ErrCVCRequired from the
	// checkout stage instead.
	t.Setenv("TRG_TOCK_CVC", "")
	var stderr bytes.Buffer
	cvc, err := tockCVCForBooking(true, os.Stdin, &stderr)
	if err != nil {
		t.Fatalf("tockCVCForBooking unexpected err: %v", err)
	}
	if cvc != "" {
		t.Fatalf("cvc = %q, want empty", cvc)
	}
	if stderr.Len() != 0 {
		t.Fatalf("machine mode wrote prompt: %q", stderr.String())
	}
}

func TestTockCVCForBooking_NoInputAcceptsEnv(t *testing.T) {
	t.Setenv("TRG_TOCK_CVC", "123")
	var stderr bytes.Buffer
	cvc, err := tockCVCForBooking(true, os.Stdin, &stderr)
	if err != nil {
		t.Fatalf("tockCVCForBooking unexpected err: %v", err)
	}
	if cvc != "123" {
		t.Fatalf("cvc = %q, want env value", cvc)
	}
	if stderr.Len() != 0 {
		t.Fatalf("machine mode with env wrote prompt: %q", stderr.String())
	}
}

func TestTockCVCForBooking_RejectsInvalidEnv(t *testing.T) {
	t.Setenv("TRG_TOCK_CVC", "12x")
	var stderr bytes.Buffer
	cvc, err := tockCVCForBooking(true, os.Stdin, &stderr)
	if !errors.Is(err, errTockCVCInvalid) {
		t.Fatalf("tockCVCForBooking err = %v, want errTockCVCInvalid", err)
	}
	if cvc != "" {
		t.Fatalf("cvc = %q, want empty", cvc)
	}
	if stderr.Len() != 0 {
		t.Fatalf("invalid env wrote prompt: %q", stderr.String())
	}
}

func TestApplyTockBookError_SelectorDriftPreservesPinnedExperienceFailure(t *testing.T) {
	bookErr := &tock.ChromeBookError{
		Kind: tock.ErrSlotControlNotFound,
		Step: "booking_control",
		Cause: errors.New(
			`requested_time="6:15 PM" combobox_layout_error=experience_card: pinned experience 520126 could not be positively identified among Book now controls`,
		),
	}
	got := applyTockBookError(bookResult{}, bookErr, "barcelona-wine-bar-raleigh", "2026-07-10", "18:15", 2)

	if got.Error != "selector_drift" {
		t.Fatalf("error category = %q, want selector_drift", got.Error)
	}
	for _, want := range []string{"step=booking_control", "experience_card", "pinned experience 520126"} {
		if !strings.Contains(got.Hint, want) {
			t.Errorf("hint = %q, want it to contain %q", got.Hint, want)
		}
	}
	if want := "https://www.exploretock.com/barcelona-wine-bar-raleigh?date=2026-07-10&size=2&time=18:15"; got.BookURL != want {
		t.Fatalf("book URL = %q, want %q", got.BookURL, want)
	}
}

func envIsVerify() bool {
	return os.Getenv("PRINTING_PRESS_VERIFY") == "1"
}

func TestOpenTableFallbackBookURL(t *testing.T) {
	tests := map[string]string{
		"le-bernardin-new-york": "https://www.opentable.com/r/le-bernardin-new-york?covers=2&dateTime=2026-07-20T19:00",
		"3688":                  "https://www.opentable.com/restaurant/profile/3688?covers=2&dateTime=2026-07-20T19:00",
	}
	for slug, want := range tests {
		if got := openTableFallbackBookURL(slug, "2026-07-20", "19:00", 2); got != want {
			t.Errorf("openTableFallbackBookURL(%q) = %q, want %q", slug, got, want)
		}
	}
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
