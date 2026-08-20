package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/hostextract"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/source/airbnb"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/store"
	_ "modernc.org/sqlite"
)

func newPersistTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// TestPersistAirbnbListing_WritesRowAndSkipsEmptyID covers the F1 listing-
// persist path: a listing with an id lands in the airbnb_listing table, and a
// card with no id is skipped (Airbnb SSR search cards sometimes lack a stable
// id) rather than erroring.
func TestPersistAirbnbListing_WritesRowAndSkipsEmptyID(t *testing.T) {
	db := newPersistTestStore(t)

	persistAirbnbListing(db, &airbnb.Listing{ID: "37124493", Title: "Lakefront cabin", City: "South Lake Tahoe"})

	var count int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM airbnb_listing`).Scan(&count); err != nil {
		t.Fatalf("count airbnb_listing: %v", err)
	}
	if count != 1 {
		t.Fatalf("airbnb_listing count = %d, want 1", count)
	}

	// A card with no id must not error and must not add a row.
	persistAirbnbListing(db, &airbnb.Listing{Title: "no id card"})
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM airbnb_listing`).Scan(&count); err != nil {
		t.Fatalf("recount airbnb_listing: %v", err)
	}
	if count != 1 {
		t.Fatalf("airbnb_listing count after empty-id persist = %d, want 1", count)
	}

	// nil store is a no-op (must not panic).
	persistAirbnbListing(nil, &airbnb.Listing{ID: "1"})
}

// TestPersistPriceSnapshot_GuardsOnPositivePrice is the F1/F2 shared
// invariant: a positive total writes a snapshot, while a zero or negative
// total writes NOTHING — an unavailable price is "no price data", never a $0
// snapshot that would pollute price history and wishlist diff.
func TestPersistPriceSnapshot_GuardsOnPositivePrice(t *testing.T) {
	db := newPersistTestStore(t)

	persistPriceSnapshot(db, "37124493", "airbnb", "2026-07-10", "2026-07-14", 1500, map[string]float64{"cleaning": 90, "service": 60})

	var count int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM price_snapshots`).Scan(&count); err != nil {
		t.Fatalf("count snapshots: %v", err)
	}
	if count != 1 {
		t.Fatalf("snapshot count = %d, want 1", count)
	}

	// Zero and negative prices are no-ops: no phantom snapshots.
	persistPriceSnapshot(db, "37124493", "airbnb", "2026-08-01", "2026-08-05", 0, nil)
	persistPriceSnapshot(db, "37124493", "airbnb", "2026-09-01", "2026-09-05", -10, nil)
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM price_snapshots`).Scan(&count); err != nil {
		t.Fatalf("recount snapshots: %v", err)
	}
	if count != 1 {
		t.Fatalf("snapshot count after zero/neg persist = %d, want 1 (no phantom rows)", count)
	}

	// Empty listing id is a no-op even with a positive price.
	persistPriceSnapshot(db, "", "airbnb", "2026-10-01", "2026-10-05", 999, nil)
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM price_snapshots`).Scan(&count); err != nil {
		t.Fatalf("recount snapshots after empty-id: %v", err)
	}
	if count != 1 {
		t.Fatalf("snapshot count after empty-id persist = %d, want 1", count)
	}

	// nil store is a no-op (must not panic).
	persistPriceSnapshot(nil, "1", "airbnb", "", "", 100, nil)

	// Fee aliases project into the dedicated columns.
	snaps, err := db.ListPriceSnapshotsSince(0)
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("listed snapshots = %d, want 1", len(snaps))
	}
	if snaps[0].CleaningFee != 90 || snaps[0].ServiceFee != 60 {
		t.Fatalf("fees = (cleaning %v, service %v), want (90, 60)", snaps[0].CleaningFee, snaps[0].ServiceFee)
	}
}

// TestFeeLookup_TriesAliasesInOrder confirms the fee-alias resolution used
// when mapping Airbnb's unstable SSR fee-map keys into snapshot columns.
func TestFeeLookup_TriesAliasesInOrder(t *testing.T) {
	fees := map[string]float64{"serviceFee": 42}
	if got := feeLookup(fees, "service", "service_fee", "serviceFee"); got != 42 {
		t.Fatalf("feeLookup = %v, want 42", got)
	}
	if got := feeLookup(fees, "nope", "missing"); got != 0 {
		t.Fatalf("feeLookup miss = %v, want 0", got)
	}
}

// captureStderr swaps os.Stderr for a pipe, runs fn, and returns everything
// written to stderr. Keep the captured output small so the OS pipe buffer
// never fills before the read.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	// Drain concurrently so a write larger than the OS pipe buffer can never
	// deadlock against the deferred read.
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	w.Close()
	out := <-done
	r.Close()
	return out
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, ln := range strings.Split(s, "\n") {
		if strings.TrimSpace(ln) != "" {
			out = append(out, ln)
		}
	}
	return out
}

// TestPersistHelpers_MachineModeStreamPurity is the regression lock for the
// machine-mode stream-purity contract (the Greptile P1 finding): when
// humanFriendly is false, a store-write failure inside any persist helper must
// emit ONLY structured persist_warning JSON events on stderr — never a
// free-form "warning: ..." line that would corrupt the JSON sync event stream
// a machine consumer parses (runScrapeSync -> computeCheapest calls these on
// every sync iteration). Closing the store makes every write fail
// deterministically. The host name carries embedded quotes to prove jsonEscape
// keeps the event valid JSON.
func TestPersistHelpers_MachineModeStreamPurity(t *testing.T) {
	origHF := humanFriendly
	t.Cleanup(func() { humanFriendly = origHF })

	s, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	s.Close() // subsequent writes now fail deterministically.

	humanFriendly = false
	out := captureStderr(t, func() {
		persistAirbnbListing(s, &airbnb.Listing{ID: "37124493", Title: "Lakefront cabin"})
		persistHost(s, &hostextract.HostInfo{Name: `Tahoe "Lux" Getaways`})
		persistPriceSnapshot(s, "37124493", "airbnb", "2026-07-10", "2026-07-14", 1500, nil)
	})

	lines := nonEmptyLines(out)
	if len(lines) != 3 {
		t.Fatalf("machine-mode stderr lines = %d, want 3\n%s", len(lines), out)
	}
	// Each helper supplies a non-empty id, so every event must carry the
	// matching "id" field. Asserting the value (not just parseability) catches
	// a dropped or misnamed id key that the event-shape struct would otherwise
	// silently ignore. The host name carries embedded quotes to prove the id
	// field round-trips through jsonEscape.
	wantID := map[string]string{
		"persist_listing_failed":  "37124493",
		"persist_host_failed":     `Tahoe "Lux" Getaways`,
		"persist_snapshot_failed": "37124493",
	}
	seen := map[string]bool{}
	for _, line := range lines {
		var ev struct {
			Event   string `json:"event"`
			Reason  string `json:"reason"`
			ID      string `json:"id"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("machine-mode stderr is not pure JSON: %q (%v)", line, err)
		}
		if ev.Event != "persist_warning" {
			t.Fatalf("event = %q, want persist_warning (line %q)", ev.Event, line)
		}
		wantIDVal, known := wantID[ev.Reason]
		if !known {
			t.Fatalf("unexpected reason %q (line %q)", ev.Reason, line)
		}
		if seen[ev.Reason] {
			t.Fatalf("duplicate persist_warning for reason %q", ev.Reason)
		}
		seen[ev.Reason] = true
		if ev.ID != wantIDVal {
			t.Fatalf("reason %q id = %q, want %q", ev.Reason, ev.ID, wantIDVal)
		}
	}
	for reason := range wantID {
		if !seen[reason] {
			t.Fatalf("missing persist_warning for reason %q", reason)
		}
	}

	// Human mode preserves the legacy "warning: ..." line (not JSON) and keeps
	// the listing id in the message text.
	humanFriendly = true
	human := captureStderr(t, func() {
		persistAirbnbListing(s, &airbnb.Listing{ID: "37124493"})
	})
	if !strings.HasPrefix(strings.TrimSpace(human), "warning:") {
		t.Fatalf("human-mode output = %q, want a 'warning:' line", human)
	}
	if !strings.Contains(human, "37124493") {
		t.Fatalf("human-mode output = %q, want it to name the listing id", human)
	}
}

// TestPersistWarn_MachineModeEscapesAllFieldsAndOmitsEmptyID covers persistWarn
// directly: openScrapeStore's store_unavailable path is the only caller of the
// empty-id branch (which must omit the "id" key), and every interpolated field
// — reason included — must pass through jsonEscape so a value carrying a quote
// or backslash still yields exactly one valid JSON object on stderr.
func TestPersistWarn_MachineModeEscapesAllFieldsAndOmitsEmptyID(t *testing.T) {
	origHF := humanFriendly
	t.Cleanup(func() { humanFriendly = origHF })
	humanFriendly = false

	out := captureStderr(t, func() {
		persistWarn(`store_"un"available`, "", `open \ failed: "boom"`)
	})
	lines := nonEmptyLines(out)
	if len(lines) != 1 {
		t.Fatalf("lines = %d, want 1\n%s", len(lines), out)
	}

	// Empty id must omit the "id" key entirely (not emit "id":"").
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(lines[0]), &raw); err != nil {
		t.Fatalf("emitted line is not valid JSON: %q (%v)", lines[0], err)
	}
	if _, ok := raw["id"]; ok {
		t.Fatalf("empty id must omit the \"id\" key, got %q", lines[0])
	}

	// reason and message must round-trip their embedded quotes/backslash —
	// proving reason is jsonEscape'd like id and message, not interpolated raw.
	var ev struct {
		Event   string `json:"event"`
		Reason  string `json:"reason"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &ev); err != nil {
		t.Fatalf("decode event: %v", err)
	}
	if ev.Event != "persist_warning" {
		t.Fatalf("event = %q, want persist_warning", ev.Event)
	}
	if ev.Reason != `store_"un"available` {
		t.Fatalf("reason = %q, want the embedded quotes to round-trip", ev.Reason)
	}
	if ev.Message != `open \ failed: "boom"` {
		t.Fatalf("message = %q, want quotes and backslash to round-trip", ev.Message)
	}
}
