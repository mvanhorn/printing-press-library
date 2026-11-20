package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

// etsy-ext: Synthetic fixtures cover missing joins and conservative timestamp handling.
func TestEtsyPaymentParentsToWalk(t *testing.T) {
	testenv.Isolate(t)
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	s2Must(t, err)
	defer db.Close()
	seed := func(typ, raw string) {
		_, _, _, err := db.UpsertBatchDetailed(typ, []json.RawMessage{json.RawMessage(raw)})
		s2Must(t, err)
	}
	for _, row := range []string{`{"receipt_id":3000000001,"updated_timestamp":100}`, `{"receipt_id":3000000002,"updated_timestamp":200}`, `{"receipt_id":3000000003}`, `{"receipt_id":3000000004,"updated_timestamp":100}`, `{"receipt_id":3000000005,"updated_timestamp":100}`} {
		seed("receipts", row)
	}
	for _, id := range []string{"3000000001", "3000000002", "3000000003"} {
		s2Must(t, db.RecordPaymentCheck(id, sql.NullInt64{Int64: 100, Valid: true}))
	}
	s2Must(t, db.RecordPaymentCheck("3000000005", sql.NullInt64{}))
	ids := []string{"3000000005", "3000000004", "3000000003", "3000000002", "3000000001"}
	for _, tc := range []struct {
		name  string
		full  bool
		input []string
		want  []string
	}{
		{"noCheck", false, []string{"3000000004"}, []string{"3000000004"}},
		{"equalStamp", false, []string{"3000000001"}, []string{}},
		{"increasedStamp", false, []string{"3000000002"}, []string{"3000000002"}},
		{"unknownStamp", false, []string{"3000000003"}, []string{"3000000003"}},
		{"nullCheckStamp", false, []string{"3000000005"}, []string{"3000000005"}},
		{"orderPreserved", false, ids, ids[:4]}, {"full", true, ids, []string{"3000000004", "3000000005", "3000000003", "3000000002", "3000000001"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, stamps, err := etsyPaymentParentsToWalk(db, tc.input, tc.full)
			s2Must(t, err)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
			if stamps["3000000002"] != 200 {
				t.Fatalf("stamps %v", stamps)
			}
			if _, ok := stamps["3000000003"]; ok {
				t.Fatal("unknown stamp became known")
			}
		})
	}
}

// etsy-ext: Preview must enumerate all parents without touching the checkpoint.
type incrementalPaymentPreview struct{ *shopScopeSyncClient }

func (*incrementalPaymentPreview) IsDryRun() bool { return true }

// etsy-ext: Exercise actual fan-out, scope estimates, skip events, and start watermarks.
func TestEtsyPaymentIncrementalWalk(t *testing.T) {
	testenv.Isolate(t)
	oldHuman := humanFriendly
	humanFriendly = false
	t.Cleanup(func() { humanFriendly = oldHuman })
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	s2Must(t, err)
	defer db.Close()
	seed := func(id int, stamp int64) {
		_, _, _, err := db.UpsertBatchDetailed("receipts", []json.RawMessage{json.RawMessage(fmt.Sprintf(`{"receipt_id":%d,"updated_timestamp":%d}`, id, stamp))})
		s2Must(t, err)
	}
	for i := 1; i <= 3; i++ {
		seed(3000000000+i, 1700000000)
	}
	hits := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits[r.URL.Path]++
		var id int64
		if _, err := fmt.Sscanf(r.URL.Path, "/shops/9000001/receipts/%d/payments", &id); err != nil {
			t.Errorf("unexpected path %s", r.URL.Path)
			return
		}
		fmt.Fprintf(w, `{"results":[{"payment_id":%d,"receipt_id":%d,"amount_gross":{"amount":1200}}],"count":1}`, id+2000000000, id)
	}))
	defer srv.Close()
	c := &shopScopeSyncClient{url: srv.URL, shopID: "9000001", client: srv.Client()}
	scope := func(name string) string {
		return syncCheckpointScope(name, true, nil, syncClientTemplateVars(c), syncActivePathContext())
	}
	var events bytes.Buffer
	run := func(full bool) {
		t.Helper()
		for _, r := range syncDependentResources(context.Background(), c, db, "", full, 0, false, false, []string{"shops_receipts_payments"}, nil, &events, 1) {
			s2Must(t, r.Err)
			if r.Warn != nil {
				t.Fatal(r.Warn)
			}
		}
	}
	before := time.Now().Add(-syncWatermarkOverlap)
	run(false)
	if len(hits) != 3 {
		t.Fatalf("first hits %v", hits)
	}
	n, err := db.Count("shops_receipts_payments")
	s2Must(t, err)
	if n != 3 {
		t.Fatalf("stored %d", n)
	}
	_, first, _, err := db.ReadSyncScope("shops_receipts_payments", scope("shops_receipts_payments"))
	s2Must(t, err)
	if first.Before(before.Add(-time.Second)) || first.After(time.Now().Add(-syncWatermarkOverlap)) {
		t.Fatalf("unexpected start watermark %v", first)
	}
	for _, tc := range []struct {
		plan etsySyncPlan
		want int
	}{{etsySyncPlan{checked: true}, 0}, {etsySyncPlan{}, 3}, {etsySyncPlan{}, 3}} {
		if n := estimateSyncCallsPlan(db, nil, []string{"receipts"}, 0, false, tc.plan); n != tc.want {
			t.Fatalf("estimate %d want %d", n, tc.want)
		}
	}
	if n := estimateSyncCalls(db, nil, []string{"receipts"}, 0, false); n != 3 {
		t.Fatalf("legacy %d", n)
	}
	clear(hits)
	events.Reset()
	run(false)
	_, second, _, err := db.ReadSyncScope("shops_receipts_payments", scope("shops_receipts_payments"))
	s2Must(t, err)
	if len(hits) != 0 || second.Before(first) || !strings.Contains(events.String(), `"reason":"no_receipt_changes"`) || !strings.Contains(events.String(), `"event":"sync_skipped"`) {
		t.Fatalf("skip hits %v events %s", hits, events.String())
	}
	seed(3000000002, time.Now().Unix()+10)
	run(false)
	if len(hits) != 1 || hits["/shops/9000001/receipts/3000000002/payments"] != 1 {
		t.Fatalf("changed hits %v", hits)
	}
	clear(hits)
	run(true)
	if len(hits) != 3 {
		t.Fatalf("full hits %v", hits)
	}
	cursor, stamp, count, err := db.ReadSyncScope("shops_receipts_payments", scope("shops_receipts_payments"))
	s2Must(t, err)
	clear(hits)
	for _, r := range syncDependentResources(context.Background(), &incrementalPaymentPreview{c}, db, "", false, 0, false, false, []string{"shops_receipts_payments"}, nil, nil, 1) {
		s2Must(t, r.Err)
	}
	afterCursor, afterStamp, afterCount, err := db.ReadSyncScope("shops_receipts_payments", scope("shops_receipts_payments"))
	s2Must(t, err)
	if len(hits) != 3 || cursor != afterCursor || !stamp.Equal(afterStamp) || count != afterCount {
		t.Fatalf("preview changed state or filtered parents: %v", hits)
	}
}

// etsy-ext: Full walks prioritize missing checks, then age, with stable ties.
func TestEtsyFullWalkOrdersUncheckedThenOldest(t *testing.T) {
	h := newPaymentCheckHarness(t)
	ids := []string{}
	for i := int64(1); i <= 6; i++ {
		h.seed(3000000000+i, 100)
		ids = append(ids, fmt.Sprint(3000000000+i))
	}
	got, _, err := etsyPaymentParentsToWalk(h.db, ids, true)
	s2Must(t, err)
	if !reflect.DeepEqual(got, ids) {
		t.Fatalf("cold walk %v", got)
	}
	for _, i := range []int{2, 4, 6} {
		s2Must(t, h.db.RecordPaymentCheck(ids[i-1], sql.NullInt64{Int64: 100, Valid: true}))
	}
	for i, stamp := range map[int]int64{4: 100, 6: 200, 2: 300} {
		_, err := h.db.DB().Exec("UPDATE etsy_payment_checks SET checked_at=? WHERE receipt_id=?", stamp, ids[i-1])
		s2Must(t, err)
	}
	got, _, err = etsyPaymentParentsToWalk(h.db, ids, true)
	s2Must(t, err)
	want := []string{ids[0], ids[2], ids[4], ids[3], ids[5], ids[1]}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("full walk %v want %v", got, want)
	}
	got, _, err = etsyPaymentParentsToWalk(h.db, ids, false)
	s2Must(t, err)
	want = []string{ids[0], ids[2], ids[4]}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("incremental walk %v want %v", got, want)
	}
	// Equal ages retain the original order, including checked receipts.
	_, err = h.db.DB().Exec("UPDATE etsy_payment_checks SET checked_at=100")
	s2Must(t, err)
	got, _, err = etsyPaymentParentsToWalk(h.db, ids, true)
	s2Must(t, err)
	want = []string{ids[0], ids[2], ids[4], ids[1], ids[3], ids[5]}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tied walk %v want %v", got, want)
	}
}

func TestEtsyPaymentsCappedMessage(t *testing.T) {
	for _, full := range []bool{false, true} {
		got := etsyPaymentsCappedMessage(full, 2, 6)
		want := "payments: 2 of 6 receipts checked; re-run to continue (progress is saved)"
		if full {
			want = "payments: 2 of 6 receipts checked this run (--full checks never-checked receipts first, then the oldest-checked); re-run to continue (progress is saved)"
		}
		if got != want {
			t.Fatalf("message %q want %q", got, want)
		}
		if !strings.Contains(got, "re-run to continue (progress is saved)") || strings.Contains(got, "--full") != full {
			t.Fatalf("message %q", got)
		}
	}
}

// etsy-ext: F1 selects one oldest forbidden probe after all normal work.
func TestEtsyPaymentWalkForbiddenProbe(t *testing.T) {
	for _, etsyCase := range []struct {
		etsyName      string
		etsyForbidden []int
		etsyAges      []int64
		etsySuccess   []int
		etsyDenied    bool
		etsyChanged   bool
		etsyInput     []int
		etsyWant      []int
		etsyFullWant  []int
		etsyBoth      bool
		etsyLimit     bool
	}{
		{etsyName: "incrementalProbeLast", etsyForbidden: []int{1, 2}, etsyAges: []int64{30, 10}, etsySuccess: []int{5}, etsyInput: []int{1, 2, 3, 4, 5}, etsyWant: []int{3, 4, 2}},
		{etsyName: "fullProbeLast", etsyForbidden: []int{1, 2}, etsyAges: []int64{30, 10}, etsySuccess: []int{5}, etsyInput: []int{1, 2, 3, 4, 5}, etsyWant: []int{3, 4, 5, 2}, etsyFullWant: []int{3, 4, 5, 2}},
		{etsyName: "allForbiddenOneProbe", etsyForbidden: []int{1, 2}, etsyAges: []int64{30, 10}, etsyInput: []int{1, 2}, etsyWant: []int{2}, etsyBoth: true, etsyLimit: true},
		{etsyName: "allForbiddenFiveRows", etsyForbidden: []int{1, 2, 3, 4, 5}, etsyAges: []int64{30, 10, 20, 40, 50}, etsyInput: []int{1, 2, 3, 4, 5}, etsyWant: []int{2}, etsyBoth: true, etsyLimit: true},
		{etsyName: "forbiddenNeverFirst", etsyForbidden: []int{1, 2}, etsyAges: []int64{1, 2}, etsySuccess: []int{5}, etsyInput: []int{1, 2, 3, 4, 5}, etsyWant: []int{3, 4, 1}, etsyFullWant: []int{3, 4, 5, 1}, etsyBoth: true},
		{etsyName: "forbiddenNeverFirstTwoUnchecked", etsyForbidden: []int{1, 2}, etsyAges: []int64{1, 2}, etsyInput: []int{1, 2, 3, 4}, etsyWant: []int{3, 4, 1}, etsyBoth: true},
		{etsyName: "changedStampStaysProbeOnly", etsyForbidden: []int{1}, etsyAges: []int64{10}, etsyChanged: true, etsyInput: []int{1, 3}, etsyWant: []int{3, 1}},
		{etsyName: "changedStampOnlyProbe", etsyForbidden: []int{1}, etsyAges: []int64{10}, etsyChanged: true, etsyInput: []int{1}, etsyWant: []int{1}},
		{etsyName: "staleLedgerRowIgnored", etsyForbidden: []int{9}, etsyAges: []int64{1}, etsyInput: []int{3}, etsyWant: []int{3}, etsyBoth: true},
		{etsyName: "notFoundRowStillDurable", etsyDenied: true, etsyInput: []int{1, 3}, etsyWant: []int{3}},
		{etsyName: "notFoundChangedNormalPosition", etsyDenied: true, etsyChanged: true, etsyInput: []int{1, 3}, etsyWant: []int{1, 3}},
		{etsyName: "probeAgeTieInputOrder", etsyForbidden: []int{1, 2}, etsyAges: []int64{10, 10}, etsyInput: []int{2, 1, 3}, etsyWant: []int{3, 2}, etsyBoth: true},
	} {
		t.Run(etsyCase.etsyName, func(t *testing.T) {
			etsyHarness := newPaymentCheckHarness(t)
			etsyIDs := func(etsyNumbers []int) []string {
				etsyResult := []string{}
				for _, etsyNumber := range etsyNumbers {
					etsyResult = append(etsyResult, fmt.Sprint(3000000000+int64(etsyNumber)))
				}
				return etsyResult
			}
			for _, etsyNumber := range etsyCase.etsyInput {
				etsyHarness.seed(3000000000+int64(etsyNumber), 100)
			}
			etsyStamp := sql.NullInt64{Int64: 100, Valid: true}
			for etsyIndex, etsyID := range etsyIDs(etsyCase.etsyForbidden) {
				s2Must(t, etsyHarness.db.RecordPaymentForbidden(etsyID, etsyStamp))
				_, etsyErr := etsyHarness.db.DB().Exec("UPDATE etsy_payment_checks SET checked_at=? WHERE receipt_id=?", etsyCase.etsyAges[etsyIndex], etsyID)
				s2Must(t, etsyErr)
			}
			for _, etsyID := range etsyIDs(etsyCase.etsySuccess) {
				s2Must(t, etsyHarness.db.RecordPaymentCheck(etsyID, etsyStamp))
				_, etsyErr := etsyHarness.db.DB().Exec("UPDATE etsy_payment_checks SET checked_at=? WHERE receipt_id=?", 100, etsyID)
				s2Must(t, etsyErr)
			}
			if etsyCase.etsyDenied {
				s2Must(t, etsyHarness.db.RecordPaymentDenied("3000000001", etsyStamp))
			}
			if etsyCase.etsyChanged {
				etsyHarness.seed(3000000001, 999)
			}
			etsyModes := []bool{false}
			if etsyCase.etsyBoth {
				etsyModes = []bool{false, true}
			} else if etsyCase.etsyFullWant != nil {
				etsyModes = []bool{true}
			}
			for _, etsyFull := range etsyModes {
				etsyWant := etsyIDs(etsyCase.etsyWant)
				if etsyFull && etsyCase.etsyFullWant != nil {
					etsyWant = etsyIDs(etsyCase.etsyFullWant)
				}
				etsyGot, _, etsyErr := etsyPaymentParentsToWalk(etsyHarness.db, etsyIDs(etsyCase.etsyInput), etsyFull)
				s2Must(t, etsyErr)
				if !reflect.DeepEqual(etsyGot, etsyWant) {
					t.Fatalf("full %t: got %v want %v", etsyFull, etsyGot, etsyWant)
				}
				if etsyCase.etsyLimit && len(etsyGot) != 1 {
					t.Fatalf("full %t: probe count %d want 1", etsyFull, len(etsyGot))
				}
				if etsyCase.etsyName == "forbiddenNeverFirstTwoUnchecked" && len(etsyGot) != 3 {
					t.Fatalf("full %t: walk count %d want 3", etsyFull, len(etsyGot))
				}
			}
		})
	}
}
