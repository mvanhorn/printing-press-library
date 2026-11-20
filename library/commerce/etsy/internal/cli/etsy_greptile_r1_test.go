package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/config"
)

func TestR1GPSRTypes(t *testing.T) {
	for _, tc := range []struct {
		name, typ         string
		id                int64
		missing, unsynced int
	}{
		{"seller", "seller", 0, 0, 0}, {"missing operator id", "economic_operator", 0, 1, 0},
		{"unsynced operator", "economic_operator", 1000000002, 0, 2}, {"empty type", "", 0, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := shopxNewFixture(t)
			f.put("listings", map[string]any{"listing_id": int64(1000000001), "state": "active", "title": "linen apron", "gpsr_manufacturer_type": tc.typ, "gpsr_responsible_person_type": tc.typ, "gpsr_manufacturer_id": tc.id, "gpsr_responsible_person_id": tc.id})
			f.put("economic_operators", map[string]any{"economic_operator_id": int64(1000000003), "operator_name": "Juniper Ceramics"})
			rows, _ := shopxAuditListings(f.db, shopxTestShopID, "gpsr")
			counts := shopxAuditCounts(rows)
			if counts["gpsr_manufacturer_missing"] != tc.missing || counts["gpsr_responsible_person_missing"] != tc.missing || counts["gpsr_operator_not_synced"] != tc.unsynced {
				t.Fatalf("findings: %v", counts)
			}
		})
	}
}

func TestR1SyncRetryBudget(t *testing.T) {
	for _, cap := range []int{1, 3} {
		t.Run(fmt.Sprint(cap), func(t *testing.T) {
			mktTestEnv(t)
			var requests atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if requests.Add(1) <= int64(cap) {
					w.Header().Set("Retry-After", "0")
					w.WriteHeader(429)
					return
				}
				fmt.Fprint(w, `{"results":[]}`)
			}))
			defer server.Close()
			c := client.New(&config.Config{BaseURL: server.URL}, 30*time.Second, 0)
			c.NoCache = true
			guard := &etsySyncGuard{inner: c, maxCalls: cap}
			_, err := guard.Get(context.Background(), "/listings/active", nil)
			if !errors.Is(err, errEtsySyncMaxCalls) || requests.Load() != int64(cap) || guard.Calls() != int64(cap) {
				t.Fatalf("err=%v requests=%d charged=%d", err, requests.Load(), guard.Calls())
			}
		})
	}
}

func TestR1SnapshotRetryBudget(t *testing.T) {
	for _, cap := range []int{1, 3} {
		t.Run(fmt.Sprint(cap), func(t *testing.T) {
			mktTestEnv(t)
			var requests atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if requests.Add(1) <= int64(cap) {
					w.Header().Set("Retry-After", "0")
					if cap == 1 {
						w.WriteHeader(500)
					} else {
						w.WriteHeader(429)
					}
					return
				}
				fmt.Fprint(w, `{"results":[],"count":0}`)
			}))
			defer server.Close()
			t.Setenv("ETSY_BASE_URL", server.URL)
			t.Setenv("ETSY_API_KEY", "synthetic:synthetic")
			out, err := mktRunCLI(t, mktSnapArgs("--term", "linen apron", "--pages", "1", "--max-calls", fmt.Sprint(cap))...)
			if err != nil || requests.Load() != int64(cap) {
				t.Fatalf("err=%v requests=%d out=%s", err, requests.Load(), out)
			}
			report := mktDecode(t, out)
			if report["status"] != "partial" || report["error"] != nil {
				t.Fatal(out)
			}
			wantNotes := []string{fmt.Sprintf("etsy-pp-cli market snapshot --resume %.0f", report["snapshot_id"])}
			if !reflect.DeepEqual(anyNotesStrings(report), wantNotes) {
				t.Fatal(out)
			}
			db := mktOpenFixtureStore(t)
			var status string
			if err := db.DB().QueryRow("SELECT status FROM market_snapshots WHERE id=?", report["snapshot_id"]).Scan(&status); err != nil || status != "partial" {
				t.Fatalf("saved status=%s err=%v", status, err)
			}
			db.Close()
			if mktDecode(t, out)["calls_this_run"] != float64(cap) {
				t.Fatal(out)
			}
		})
	}
}

func TestR1ReconcileAmounts(t *testing.T) {
	for _, tc := range []struct {
		name             string
		amounts          []int64
		currency         string
		want             int
		currencyMismatch bool
	}{
		{"different", []int64{100}, "USD", 1, false}, {"equal", []int64{4200}, "USD", 0, false},
		{"split", []int64{2000, 2200}, "USD", 0, false}, {"currency", []int64{4200}, "EUR", 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := shopxNewFixture(t)
			f.put("receipts", map[string]any{"receipt_id": int64(1000000001), "created_timestamp": shopxAgo(shopxDay), "grandtotal": shopxMoneyObj(4200, "USD")})
			for i, a := range tc.amounts {
				f.put("payments", map[string]any{"payment_id": int64(1000000002 + i), "receipt_id": int64(1000000001), "amount_gross": shopxMoneyObj(a, tc.currency)})
			}
			result, _ := shopxReconcile(f.db, shopxTestShopID, shopxFixedNow, 30*shopxDay)
			if len(result.AmountMismatches) != tc.want || result.MismatchCount != tc.want {
				t.Fatalf("%+v", result)
			}
			out, _, err := shopxRun(t, f, "money", "reconcile")
			if err != nil {
				t.Fatal(err)
			}
			if len(shopxReconcileArray(t, shopxMustObject(t, out), "amount_mismatches")) != tc.want {
				t.Fatal(out)
			}
			if tc.want > 0 {
				m := result.AmountMismatches[0]
				if m.ReceiptID != 1000000001 || m.ReceiptAmount != 4200 || m.PaymentsAmount != tc.amounts[0] || m.Currency != "USD" || m.CurrencyMismatch != tc.currencyMismatch || len(m.PaymentIDs) != len(tc.amounts) || m.PaymentIDs[0] != 1000000002 {
					t.Fatalf("%+v", m)
				}
			}
		})
	}
}

func TestR1ReviewWholeDaysAndResume(t *testing.T) {
	for _, window := range []string{"36h", "12h", "0d", "90d", "12mo", "2w", "1y"} {
		t.Run(window, func(t *testing.T) {
			mktTestEnv(t)
			fake := newMktFakeEtsy(t)
			mktPointAtFake(t, fake)
			out, err := mktRunCLI(t, mktSnapArgs("--term", "linen apron", "--pages", "1", "--review-window", window, "--with-reviews", "--max-calls", "1")...)
			valid := window == "90d" || window == "12mo" || window == "2w" || window == "1y"
			if !valid {
				if ExitCode(err) != 2 || !strings.Contains(err.Error(), "whole days") || fake.pageGets != 0 {
					t.Fatalf("err=%v calls=%d", err, fake.pageGets)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			id := mktDecode(t, out)["snapshot_id"]
			db := mktOpenFixtureStore(t)
			defer db.Close()
			var days int
			if err := db.DB().QueryRow("SELECT review_window_days FROM market_snapshots WHERE id=?", id).Scan(&days); err != nil {
				t.Fatal(err)
			}
			want, _ := mktParseWindow(window)
			if days != int(want/(24*time.Hour)) {
				t.Fatalf("days=%d", days)
			}
			_, err = mktRunCLI(t, mktSnapArgs("--resume", fmt.Sprint(id), "--review-window", "invalid-ignored", "--max-calls", "200")...)
			if err != nil {
				t.Fatal(err)
			}
			var taken string
			if err := db.DB().QueryRow("SELECT taken_at FROM market_snapshots WHERE id=?", id).Scan(&taken); err != nil {
				t.Fatal(err)
			}
			ts, err := time.Parse(time.RFC3339, taken)
			if err != nil {
				t.Fatal(err)
			}
			if fake.revGets == 0 || fake.revSince != fmt.Sprint(ts.Add(-want).Unix()) {
				t.Fatalf("resume review cutoff=%s want=%d", fake.revSince, ts.Add(-want).Unix())
			}
			var resumed int
			if err := db.DB().QueryRow("SELECT review_window_days FROM market_snapshots WHERE id=?", id).Scan(&resumed); err != nil || resumed != days {
				t.Fatalf("days=%d err=%v", resumed, err)
			}
		})
	}
}

func TestR1ReviewCoverageReports(t *testing.T) {
	for _, command := range []string{"demand", "price", "tags"} {
		for _, days := range []int{0, 90, 365} {
			for _, hasReviews := range []bool{false, true} {
				for _, window := range []string{"30d", "90d", "12mo"} {
					t.Run(fmt.Sprintf("%s/%d/%s", command, days, window)+fmt.Sprint(hasReviews), func(t *testing.T) {
						mktTestEnv(t)
						db := mktOpenFixtureStore(t)
						now := time.Now()
						id := mktFxSnapshot(t, db.DB(), now, []string{"linen apron"}, 1, "complete", mktFxProg{})
						mktFxResult(t, db.DB(), id, mktFxListing{term: "linen apron", pos: 1, id: 1000000001, shop: 1000000002, title: "linen apron", price: 4200})
						if hasReviews {
							mktFxReviews(t, db.DB(), id, 1000000001, 10, now.Add(-24*time.Hour))
						}
						if _, err := db.DB().Exec("UPDATE market_snapshots SET review_window_days=? WHERE id=?", days, id); err != nil {
							t.Fatal(err)
						}
						db.Close()
						out, err := mktRunCLI(t, "market", command, "--window", window)
						if err != nil {
							t.Fatal(err)
						}
						report := mktDecode(t, out)
						mktCheckNotesInvariant(t, report)
						coverage, present := report["review_coverage_days"]
						wantCoverage := hasReviews && days == 90 && window == "12mo"
						if present != wantCoverage || (present && coverage != float64(days)) {
							t.Fatal(out)
						}
						advisory := strings.Contains(out, "reviews were collected for the last")
						if advisory != wantCoverage {
							t.Fatal(out)
						}
					})
				}
			}
		}
	}
}

func TestR1SyncCLIRetryBudget(t *testing.T) {
	for _, cap := range []int{1, 3} {
		t.Run(fmt.Sprint(cap), func(t *testing.T) {
			syncCLIIsolate(t)
			var requests atomic.Int64
			s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
				if requests.Add(1) <= int64(cap) {
					w.Header().Set("Retry-After", "0")
					w.WriteHeader(429)
				} else {
					fmt.Fprint(w, `{"results":[],"count":0}`)
				}
				return true
			})
			path := syncCLIDB(t, 0)
			out, err := syncCLIRun(t, path, "--max-calls", fmt.Sprint(cap), "--no-cache", "--concurrency", "1")
			if ExitCode(err) != 7 || requests.Load() != int64(cap) {
				t.Fatalf("err=%v requests=%d out=%s", err, requests.Load(), out)
			}
		})
	}
}

// etsy-ext: Exercise GPSR fields independently, including stale seller IDs.
func TestR1GPSREdges(t *testing.T) {
	for _, tc := range []struct {
		name, mt, rt string
		ops          bool
		want         map[string]int
	}{
		{"seller no operators", "seller", "seller", false, map[string]int{}},
		{"seller leftover ids", "seller", "seller", true, map[string]int{}},
		{"mixed", "seller", "economic_operator", true, map[string]int{"gpsr_operator_not_synced": 1}},
		{"unknown", "manufacturer", "eu_responsible_person", true, map[string]int{"gpsr_manufacturer_type_invalid": 1, "gpsr_responsible_person_type_invalid": 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := shopxNewFixture(t)
			f.put("listings", map[string]any{"listing_id": int64(1000000001), "state": "active", "gpsr_manufacturer_type": tc.mt, "gpsr_responsible_person_type": tc.rt, "gpsr_manufacturer_id": int64(1000000002), "gpsr_responsible_person_id": int64(1000000002)})
			if tc.ops {
				f.put("economic_operators", map[string]any{"economic_operator_id": int64(1000000003)})
			}
			rows, notes := shopxAuditListings(f.db, shopxTestShopID, "gpsr")
			if !reflect.DeepEqual(shopxAuditCounts(rows), tc.want) || len(notes) != 0 {
				t.Fatalf("findings=%v notes=%v", shopxAuditCounts(rows), notes)
			}
		})
	}
}

func TestR1ReconcileEdges(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		age                    time.Duration
		receipt, first, second any
		want, skipped          int
	}{
		{"outside window", 60 * shopxDay, shopxMoneyObj(4200, "USD"), shopxMoneyObj(1, "USD"), nil, 0, 0},
		{"early currency differs", shopxDay, shopxMoneyObj(4200, "USD"), shopxMoneyObj(2000, "EUR"), shopxMoneyObj(2200, "USD"), 1, 0},
		{"invalid receipt", shopxDay, "bad", shopxMoneyObj(4200, "USD"), nil, 0, 1},
		{"invalid payment", shopxDay, shopxMoneyObj(4200, "USD"), "bad", nil, 0, 1},
		{"missing receipt", shopxDay, nil, shopxMoneyObj(4200, "USD"), nil, 0, 1},
		{"missing payment", shopxDay, shopxMoneyObj(4200, "USD"), nil, nil, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := shopxNewFixture(t)
			f.put("receipts", map[string]any{"receipt_id": int64(1000000001), "created_timestamp": shopxAgo(tc.age), "grandtotal": tc.receipt})
			f.put("payments", map[string]any{"payment_id": int64(1000000002), "receipt_id": int64(1000000001), "amount_gross": tc.first})
			if tc.second != nil {
				f.put("payments", map[string]any{"payment_id": int64(1000000003), "receipt_id": int64(1000000001), "amount_gross": tc.second})
			}
			result, notes := shopxReconcile(f.db, shopxTestShopID, shopxFixedNow, 30*shopxDay)
			if len(result.AmountMismatches) != tc.want || result.MismatchCount != tc.want {
				t.Fatalf("%+v", result)
			}
			if tc.want > 0 && !result.AmountMismatches[0].CurrencyMismatch {
				t.Fatal(result)
			}
			wantNote := "1 receipt(s) skipped from amount check: missing or invalid money fields"
			if strings.Contains(strings.Join(notes, "; "), wantNote) != (tc.skipped > 0) {
				t.Fatal(notes)
			}
		})
	}
}

func TestR1CoverageCompositeDemand(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	defer db.Close()
	now := time.Now()
	id := mktFxSnapshot(t, db.DB(), now, []string{"linen apron", "empty term", "unfetched term"}, 1, "partial", mktFxProg{})
	mktFxResult(t, db.DB(), id, mktFxListing{term: "linen apron", pos: 1, id: 1000000001, shop: 1000000002, title: "linen apron", price: 4200})
	mktFxReviews(t, db.DB(), id, 1000000001, 10, now.Add(-24*time.Hour))
	if _, err := db.DB().Exec("UPDATE market_snapshots SET review_window_days=90 WHERE id=?", id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec("INSERT INTO market_term_pages (snapshot_id, term, page, listings) VALUES (?, ?, 1, 0)", id, "empty term"); err != nil {
		t.Fatal(err)
	}
	out, err := mktRunCLI(t, "market", "demand", "--snapshot", fmt.Sprint(id), "--window", "12mo")
	if err != nil {
		t.Fatal(err)
	}
	report := mktDecode(t, out)
	mktCheckNotesInvariant(t, report)
	// The exact notes below also exclude the old composite string from notes[].
	want := []string{
		"1 term(s) returned no results from Etsy's search API",
		fmt.Sprintf("1 term(s) not yet fetched; resume with etsy-pp-cli market snapshot --resume %d", id),
		"reviews were collected for the last 90 days; counts for the requested 360-day window cover only 90 days",
		"\"linen apron\": can't tell: 10 pooled recent reviews across 1 listing(s) with any; need at least 10 reviews from at least 3 listings",
		"\"empty term\": Etsy's search returned no results for this term",
	}
	if !reflect.DeepEqual(anyNotesStrings(report), want) || report["note"] != strings.Join(want, "; ") {
		t.Fatal(out)
	}
}

func TestR1SyncCacheBudget(t *testing.T) {
	mktTestEnv(t)
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); fmt.Fprint(w, `{"results":[]}`) }))
	defer server.Close()
	c := client.New(&config.Config{BaseURL: server.URL}, time.Second, 0)
	guard := &etsySyncGuard{inner: c, maxCalls: 1}
	for i := 0; i < 2; i++ {
		if _, err := guard.Get(context.Background(), "/cached", nil); err != nil {
			t.Fatal(err)
		}
	}
	if requests.Load() != 1 || guard.Calls() != 1 {
		t.Fatalf("requests=%d charged=%d", requests.Load(), guard.Calls())
	}
	if _, err := guard.Get(context.Background(), "/uncached", nil); !errors.Is(err, errEtsySyncMaxCalls) {
		t.Fatal(err)
	}
	if _, err := guard.Get(context.Background(), "/cached", nil); !errors.Is(err, errEtsySyncMaxCalls) {
		t.Fatalf("stopped cache read: %v", err)
	}
	if requests.Load() != 1 || guard.Calls() != 1 {
		t.Fatalf("requests=%d charged=%d", requests.Load(), guard.Calls())
	}
}
