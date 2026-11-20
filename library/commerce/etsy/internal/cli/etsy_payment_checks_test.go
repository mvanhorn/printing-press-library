package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

// etsy-ext: Preserve HTTP failure semantics while using the shop-scope harness.
type paymentCheckClient struct{ *shopScopeSyncClient }

func (c *paymentCheckClient) Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error) {
	path = strings.ReplaceAll(path, "{shop_id}", c.shopID)
	req, err := http.NewRequestWithContext(ctx, "GET", c.url+path, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		etsyBody, _ := io.ReadAll(res.Body)
		return nil, &client.APIError{Method: "GET", Path: path, StatusCode: res.StatusCode, Body: string(etsyBody)}
	}
	return io.ReadAll(res.Body)
}

type paymentCheckHarness struct {
	t      *testing.T
	db     *store.Store
	c      *paymentCheckClient
	hits   []string
	status func(string) int
	empty  bool
}

func newPaymentCheckHarness(t *testing.T) *paymentCheckHarness {
	t.Helper()
	testenv.Isolate(t)
	h := &paymentCheckHarness{t: t}
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	s2Must(t, err)
	h.db = db
	t.Cleanup(func() { db.Close() })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.hits = append(h.hits, r.URL.Path)
		if h.status != nil {
			if status := h.status(r.URL.Path); status != 200 {
				w.WriteHeader(status)
				fmt.Fprint(w, `{"error":"synthetic"}`)
				return
			}
		}
		if h.empty {
			fmt.Fprint(w, `{"results":[],"count":0}`)
			return
		}
		var id int64
		if _, err := fmt.Sscanf(r.URL.Path, "/shops/9000001/receipts/%d/payments", &id); err != nil {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		fmt.Fprintf(w, `{"results":[{"payment_id":%d,"receipt_id":%d}],"count":1}`, id+1000000000, id)
	}))
	t.Cleanup(srv.Close)
	h.c = &paymentCheckClient{&shopScopeSyncClient{url: srv.URL, shopID: "9000001", client: srv.Client()}}
	return h
}
func (h *paymentCheckHarness) seed(id int64, stamp int64) {
	h.t.Helper()
	_, _, _, err := h.db.UpsertBatchDetailed("receipts", []json.RawMessage{json.RawMessage(fmt.Sprintf(`{"receipt_id":%d,"updated_timestamp":%d,"shop_name":"Juniper Ceramics"}`, id, stamp))})
	s2Must(h.t, err)
}
func (h *paymentCheckHarness) run() {
	h.t.Helper()
	h.hits = nil
	syncDependentResources(context.Background(), h.c, h.db, "", false, 0, false, false, []string{"shops_receipts_payments"}, nil, io.Discard, 1)
}
func (h *paymentCheckHarness) want(ids ...int64) {
	h.t.Helper()
	want := []string{}
	for _, id := range ids {
		want = append(want, fmt.Sprintf("/shops/9000001/receipts/%d/payments", id))
	}
	got := append([]string{}, h.hits...)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		h.t.Fatalf("requests %v want %v", got, want)
	}
}
func (h *paymentCheckHarness) checks(n int) map[string]store.PaymentCheck {
	h.t.Helper()
	rows, err := h.db.ReadPaymentChecks()
	s2Must(h.t, err)
	if len(rows) != n {
		h.t.Fatalf("checks %v want count %d", rows, n)
	}
	return rows
}
func TestEtsyPaymentCheckColdStartResume(t *testing.T) {
	h := newPaymentCheckHarness(t)
	for i := int64(1); i <= 6; i++ {
		h.seed(3000000000+i, 100)
	}
	distinct := map[string]bool{}
	h.status = func(path string) int {
		distinct[path] = true
		if len(distinct) >= 4 {
			return 500
		}
		return 200
	}
	h.run()
	h.want(3000000001, 3000000002, 3000000003, 3000000004, 3000000005, 3000000006)
	checked := h.checks(3)
	var remaining []int64
	for i := int64(1); i <= 6; i++ {
		if _, ok := checked[fmt.Sprint(3000000000+i)]; !ok {
			remaining = append(remaining, 3000000000+i)
		}
	}
	h.status = nil
	h.run()
	h.want(remaining...)
	h.checks(6)
}
func TestEtsyPaymentCheckEmptyPayments(t *testing.T) {
	h := newPaymentCheckHarness(t)
	h.empty = true
	h.seed(3000000001, 100)
	h.run()
	h.want(3000000001)
	h.checks(1)
	h.run()
	h.want()
	h.seed(3000000001, 200)
	h.run()
	h.want(3000000001)
}
func TestEtsyPaymentCheckRefundRace(t *testing.T) {
	h := newPaymentCheckHarness(t)
	h.seed(3000000001, 100)
	h.run()
	h.want(3000000001)
	if h.checks(1)["3000000001"].ReceiptUpdated.Int64 != 100 {
		t.Fatal("wrong first stamp")
	}
	h.seed(3000000001, 200)
	h.run()
	h.want(3000000001)
	if h.checks(1)["3000000001"].ReceiptUpdated.Int64 != 200 {
		t.Fatal("wrong refund stamp")
	}
	h.run()
	h.want()
}
func TestEtsyPaymentCheckNon2xxNoRow(t *testing.T) {
	for _, code := range []int{500} {
		t.Run(fmt.Sprintf("status%d", code), func(t *testing.T) {
			h := newPaymentCheckHarness(t)
			h.seed(3000000001, 100)
			h.status = func(string) int { return code }
			h.run()
			h.want(3000000001)
			h.checks(0)
			h.status = nil
			h.run()
			h.want(3000000001)
			h.checks(1)
		})
	}
}

// etsy-ext: Repeated capped full walks must cover the tail before revisiting checks.
func TestEtsyCappedFullRunsRotate(t *testing.T) {
	h := newPaymentCheckHarness(t)
	for i := int64(1); i <= 6; i++ {
		h.seed(3000000000+i, 100)
	}
	etsySeen := map[string]bool{}
	etsyRun := func() *etsyPaymentsCap {
		t.Helper()
		h.hits = nil
		etsyCap := &etsyPaymentsCap{allow: 2}
		for _, result := range syncDependentResources(etsyWithPaymentsCap(context.Background(), etsyCap), h.c, h.db, "", true, 0, false, false, []string{"shops_receipts_payments"}, nil, io.Discard, 1) {
			s2Must(t, result.Err)
			if result.Warn != nil && !strings.Contains(result.Warn.Error(), "(payments_capped)") {
				t.Fatal(result.Warn)
			}
		}
		if len(h.hits) != 2 {
			t.Fatalf("requests %v", h.hits)
		}
		return etsyCap
	}
	for run := 0; run < 3; run++ {
		etsyCap := etsyRun()
		if run < 2 && !etsyCap.capped {
			t.Fatalf("run %d not capped", run+1)
		}
		for _, path := range h.hits {
			if etsySeen[path] {
				t.Fatalf("receipt repeated before full coverage: %s", path)
			}
			etsySeen[path] = true
		}
	}
	if len(etsySeen) != 6 {
		t.Fatalf("coverage %v", etsySeen)
	}
	h.checks(6)
	for i := int64(1); i <= 6; i++ {
		_, err := h.db.DB().Exec("UPDATE etsy_payment_checks SET checked_at=? WHERE receipt_id=?", 700-i*100, fmt.Sprint(3000000000+i))
		s2Must(t, err)
	}
	etsyRun()
	h.want(3000000006, 3000000005)
}

func TestEtsyPaymentCheckDeniedRecorded(t *testing.T) {
	for _, etsyCode := range []int{403, 404} {
		t.Run(fmt.Sprint(etsyCode), func(t *testing.T) {
			etsyHarness := newPaymentCheckHarness(t)
			etsyHarness.seed(3000000001, 100)
			etsyHarness.status = func(string) int { return etsyCode }
			etsyHarness.run()
			etsyHarness.want(3000000001)
			if etsyCode == 403 {
				for run := 0; run < 2; run++ {
					etsyCheck := etsyHarness.checks(1)["3000000001"]
					if !etsyCheck.Forbidden || etsyCheck.Denied || etsyCheck.ReceiptUpdated.Int64 != 100 {
						t.Fatal(etsyCheck)
					}
					var complete int
					s2Must(t, etsyHarness.db.DB().QueryRow(`SELECT last_attempt_complete FROM sync_state WHERE resource_type='shops_receipts_payments'`).Scan(&complete))
					if complete != 0 {
						t.Fatal("403 completed attempt")
					}
					etsyHarness.run()
					etsyHarness.want(3000000001)
				}
				etsyHarness.status = nil
				etsyHarness.run()
				etsyHarness.want(3000000001)
				if etsyCheck := etsyHarness.checks(1)["3000000001"]; etsyCheck.Denied || etsyCheck.Forbidden {
					t.Fatal("success denied")
				}
				var complete int
				s2Must(t, etsyHarness.db.DB().QueryRow(`SELECT last_attempt_complete FROM sync_state WHERE resource_type='shops_receipts_payments'`).Scan(&complete))
				if complete != 1 {
					t.Fatal("success incomplete")
				}
				return
			}
			var status string
			s2Must(t, etsyHarness.db.DB().QueryRow(`SELECT status FROM etsy_payment_checks WHERE receipt_id='3000000001'`).Scan(&status))
			if status != "not_found" {
				t.Fatal(status)
			}
			etsyCheck := etsyHarness.checks(1)["3000000001"]
			if !etsyCheck.Denied || etsyCheck.ReceiptUpdated.Int64 != 100 {
				t.Fatal(etsyCheck)
			}
			etsyHarness.run()
			etsyHarness.want()
			etsyHarness.seed(3000000001, 200)
			etsyHarness.run()
			etsyHarness.want(3000000001)
			etsyCheck = etsyHarness.checks(1)["3000000001"]
			if !etsyCheck.Denied || etsyCheck.ReceiptUpdated.Int64 != 200 {
				t.Fatal(etsyCheck)
			}
			etsyHarness.status = nil
			// A newer receipt re-arms the denial so success can clear it.
			etsyHarness.seed(3000000001, 300)
			etsyHarness.run()
			etsyHarness.want(3000000001)
			if etsyCheck := etsyHarness.checks(1)["3000000001"]; etsyCheck.Denied || etsyCheck.Forbidden {
				t.Fatal("denial survived success")
			}
			etsyHarness.run()
			etsyHarness.want()
		})
	}
}

func TestEtsyDeniedReceiptsDoNotConsumeCap(t *testing.T) {
	etsyHarness := newPaymentCheckHarness(t)
	for etsyID := int64(3000000001); etsyID <= 3000000004; etsyID++ {
		etsyHarness.seed(etsyID, 100)
	}
	etsyHarness.status = func(etsyPath string) int {
		if strings.Contains(etsyPath, "3000000001") || strings.Contains(etsyPath, "3000000002") {
			return 404
		}
		return 200
	}
	for etsyRun := 0; etsyRun < 2; etsyRun++ {
		etsyCap := &etsyPaymentsCap{allow: 2}
		etsyHarness.hits = nil
		syncDependentResources(etsyWithPaymentsCap(context.Background(), etsyCap), etsyHarness.c, etsyHarness.db, "", false, 0, false, false, []string{"shops_receipts_payments"}, nil, io.Discard, 1)
		if etsyCap.checked.Load() != 2 {
			t.Fatal("denials must count as checks", etsyCap.checked.Load())
		}
		if etsyRun == 0 {
			etsyHarness.want(3000000001, 3000000002)
			if !etsyCap.capped || etsyCap.denied.Load() != 2 {
				t.Fatal("missing denial counters")
			}
		} else {
			etsyHarness.want(3000000003, 3000000004)
			if etsyCap.capped || etsyCap.denied.Load() != 0 {
				t.Fatal("unexpected cap or denials")
			}
		}
	}
}

func TestEtsyPaymentDeniedUnknownStampAndStatus(t *testing.T) {
	etsyHarness := newPaymentCheckHarness(t)
	for _, etsyStamp := range []sql.NullInt64{{}, {Int64: 100, Valid: true}} {
		s2Must(t, etsyHarness.db.RecordPaymentDenied("3000000001", etsyStamp))
		etsyWalk, _, etsyErr := etsyPaymentParentsToWalk(etsyHarness.db, []string{"3000000001"}, false)
		s2Must(t, etsyErr)
		if len(etsyWalk) != 0 {
			t.Fatal("unknown receipt stamp retried denial", etsyWalk)
		}
	}
	for _, etsyCode := range []int{400, 401, 403, 404, 429, 500} {
		etsyWarning, etsyDenied := etsyPaymentsDeniedWarning(fmt.Errorf("wrapped: %w", &client.APIError{StatusCode: etsyCode, Body: "synthetic"}))
		if etsyDenied != (etsyCode == 403 || etsyCode == 404) {
			t.Fatal(etsyCode, etsyDenied)
		}
		if etsyDenied && (etsyWarning.Status != etsyCode || etsyWarning.Message != "synthetic") {
			t.Fatal(etsyWarning)
		}
	}
	if _, etsyDenied := etsyPaymentsDeniedWarning(io.EOF); etsyDenied {
		t.Fatal("transport error treated as denial")
	}
}

// etsy-ext: A permanent 404 denial must not hold attempt-complete at 0; a 403 must.
func TestEtsyPaymentsDenialAttemptComplete(t *testing.T) {
	for _, etsyCase := range []struct {
		code int
		want int
	}{{404, 1}, {403, 0}} {
		t.Run(fmt.Sprint(etsyCase.code), func(t *testing.T) {
			etsyHarness := newPaymentCheckHarness(t)
			etsyHarness.seed(3000000001, 100)
			etsyHarness.seed(3000000002, 100)
			etsyHarness.status = func(etsyPath string) int {
				if strings.Contains(etsyPath, "3000000001") {
					return etsyCase.code
				}
				return 200
			}
			etsyCap := &etsyPaymentsCap{allow: 10}
			syncDependentResources(etsyWithPaymentsCap(context.Background(), etsyCap), etsyHarness.c, etsyHarness.db, "", false, 0, false, false, []string{"shops_receipts_payments"}, nil, io.Discard, 1)
			if etsyCap.capped || etsyCap.denied.Load() != int64(etsyCase.want) {
				t.Fatalf("capped=%v denied=%d, want uncapped with payments_denied=%d", etsyCap.capped, etsyCap.denied.Load(), etsyCase.want)
			}
			if etsyCap.forbidden.Load() != int64(1-etsyCase.want) {
				t.Fatal("forbidden", etsyCap.forbidden.Load())
			}
			var etsyGot int
			s2Must(t, etsyHarness.db.DB().QueryRow(`SELECT last_attempt_complete FROM sync_state WHERE resource_type = 'shops_receipts_payments'`).Scan(&etsyGot))
			if etsyGot != etsyCase.want {
				t.Fatalf("last_attempt_complete=%d want %d for %d denial", etsyGot, etsyCase.want, etsyCase.code)
			}
		})
	}
}

func TestEtsyPaymentForbiddenRetriesDoNotCountAsChecked(t *testing.T) {
	h := newPaymentCheckHarness(t)
	h.seed(3000000001, 100)
	h.status = func(string) int { return 403 }
	for run := 0; run < 2; run++ {
		h.hits = nil
		cap := &etsyPaymentsCap{allow: 2}
		syncDependentResources(etsyWithPaymentsCap(context.Background(), cap), h.c, h.db, "", false, 0, false, false, []string{"shops_receipts_payments"}, nil, io.Discard, 1)
		h.want(3000000001)
		etsyCheck := h.checks(1)["3000000001"]
		if !etsyCheck.Forbidden || etsyCheck.Denied || etsyCheck.ReceiptUpdated.Int64 != 100 {
			t.Fatal(etsyCheck)
		}
		if cap.checked.Load() != 0 || cap.denied.Load() != 0 || cap.forbidden.Load() != 1 {
			t.Fatal("403 counted as checked", cap)
		}
	}
}

func TestEtsyPaymentLegacyDeniedWalk(t *testing.T) {
	h := newPaymentCheckHarness(t)
	h.seed(3000000001, 100)
	_, err := h.db.DB().Exec(`CREATE TABLE etsy_payment_checks(receipt_id TEXT PRIMARY KEY,receipt_updated INTEGER,checked_at INTEGER NOT NULL,status TEXT);
 INSERT INTO etsy_payment_checks VALUES('3000000001',100,10,'denied')`)
	s2Must(t, err)
	walk, _, err := etsyPaymentParentsToWalk(h.db, []string{"3000000001"}, false)
	s2Must(t, err)
	if !reflect.DeepEqual(walk, []string{"3000000001"}) {
		t.Fatal(walk)
	}
}
