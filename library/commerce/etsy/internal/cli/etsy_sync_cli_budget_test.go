package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/learn/lookups"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

func syncCLIIsolate(t *testing.T) {
	t.Helper()
	restore, err := cliutil.SetHomeOverride("")
	s2Must(t, err)
	t.Cleanup(restore)
	testenv.Isolate(t, cliutil.StateDir)
	for _, key := range []string{"ETSY_SHARED_SECRET", "ETSY_OAUTH2", "ETSY_ACCESS_TOKEN", "ETSY_REFRESH_TOKEN"} {
		t.Setenv(key, "")
	}
}

// etsy-ext: Stable timestamps permit payment-check reuse across sync runs.
func syncCLIReceipts(n, offset int) []json.RawMessage {
	rows := make([]json.RawMessage, n)
	for i := range rows {
		rows[i] = json.RawMessage(fmt.Sprintf(`{"receipt_id":%d,"shop_id":9000001,"updated_timestamp":%d}`, 3000000001+offset+i, 1700000000+offset+i))
	}
	return rows
}

func syncCLIDB(t *testing.T, n int) string {
	t.Helper()
	return s2FixtureDB(t, func(db *store.Store) {
		if n > 0 {
			_, _, _, err := db.UpsertBatchDetailed("receipts", syncCLIReceipts(n, 0))
			s2Must(t, err)
		}
	})
}

func syncCLIQuota(w http.ResponseWriter, remaining string) {
	if remaining != "" {
		w.Header().Set("x-limit-per-day", "5000")
		w.Header().Set("x-remaining-today", remaining)
	}
}

func syncCLIServer(t *testing.T, total int, remaining string, failAt int) *s2Server {
	t.Helper()
	var calls atomic.Int64
	return s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
		call := calls.Add(1)
		syncCLIQuota(w, remaining)
		if call == int64(failAt) {
			syncCLIQuota(w, "0")
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"daily quota exhausted"}`))
			return true
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method %s", r.Method)
		}
		if r.URL.Path == "/shops/9000001/receipts" {
			offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
			if r.URL.Query().Get("offset") != "" && err != nil {
				t.Errorf("offset: %v", err)
			}
			if r.URL.Query().Get("limit") != "100" {
				t.Errorf("limit=%q", r.URL.Query().Get("limit"))
			}
			n := min(100, max(0, total-offset))
			_ = json.NewEncoder(w).Encode(map[string]any{"results": syncCLIReceipts(n, offset), "count": total})
		} else if strings.HasSuffix(r.URL.Path, "/payments") {
			var receipt int64
			if _, err := fmt.Sscanf(r.URL.Path, "/shops/9000001/receipts/%d/payments", &receipt); err != nil {
				t.Errorf("payment path: %s", r.URL.Path)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{"payment_id": receipt + 1000000000, "receipt_id": receipt, "amount_gross": map[string]int{"amount": 1200}}}, "count": 1})
		} else {
			t.Errorf("unexpected path %s", r.URL.Path)
			_, _ = w.Write([]byte(`{"results":[],"count":0}`))
		}
		return true
	})
}

func syncCLIRun(t *testing.T, path string, extra ...string) (string, error) {
	t.Helper()
	args := []string{"sync", "--resources", "receipts", "--db", path, "--rate-limit", "1000", "--json"}
	out, stderr, err := s2Run(t, append(args, extra...)...)
	return out + stderr, err
}

func syncCLIRequireQuotaExit(t *testing.T, err error) {
	t.Helper()
	var ce *cliError
	if !errors.As(err, &ce) || ce.code != 7 {
		t.Fatalf("want cliError code 7, got %v (exit %d)", err, ExitCode(err))
	}
}

func syncCLIRequireCount(t *testing.T, path string, want int) {
	t.Helper()
	db, err := store.Open(path)
	s2Must(t, err)
	defer db.Close()
	n, err := db.Count("receipts")
	s2Must(t, err)
	if n != want {
		t.Fatalf("stored receipts=%d want=%d", n, want)
	}
}

func syncCLIRequireEvent(t *testing.T, output, reason string) {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		var event struct {
			Event  string `json:"event"`
			Reason string `json:"reason"`
		}
		if json.Unmarshal([]byte(line), &event) == nil && event.Event == "sync_warning" && event.Reason == reason {
			return
		}
	}
	t.Fatalf("missing sync_warning %s: %s", reason, output)
}

func TestEtsySyncCLISeededLowQuotaRefuses(t *testing.T) {
	syncCLIIsolate(t)
	path := syncCLIDB(t, 10)
	srv := syncCLIServer(t, 0, "2", 0)
	client.ObserveEtsyQuota(http.Header{"X-Limit-Per-Day": {"5000"}, "X-Remaining-Today": {"2"}}, etsyTestQuotaKey(t))
	_, err := syncCLIRun(t, path)
	syncCLIRequireQuotaExit(t, err)
	if !strings.Contains(err.Error(), "2 remaining") || len(srv.all()) != 0 {
		t.Fatalf("err=%v requests=%+v", err, srv.all())
	}
}

func TestEtsySyncCLIMaxCallsCapsRequests(t *testing.T) {
	syncCLIIsolate(t)
	path := syncCLIDB(t, 0)
	srv := syncCLIServer(t, 500, "4000", 0)
	output, err := syncCLIRun(t, path, "--max-calls", "3")
	t.Logf("max-calls exit=%d error=%v", ExitCode(err), err)
	syncCLIRequireQuotaExit(t, err)
	if n := len(srv.all()); n < 2 || n > 3 {
		t.Fatalf("requests=%d", n)
	}
	syncCLIRequireEvent(t, output, "max_calls_reached")
}

func TestEtsySyncCLIDailyQuota429StopsOnce(t *testing.T) {
	syncCLIIsolate(t)
	path := syncCLIDB(t, 0)
	srv := syncCLIServer(t, 500, "4000", 2)
	output, err := syncCLIRun(t, path)
	syncCLIRequireQuotaExit(t, err)
	if n := len(srv.all()); n != 2 {
		t.Fatalf("requests=%d want=2", n)
	}
	if n := strings.Count(output, `"reason":"daily_quota_exhausted"`); n != 1 {
		t.Fatalf("quota events=%d: %s", n, output)
	}
	syncCLIRequireEvent(t, output, "daily_quota_exhausted")
	syncCLIRequireCount(t, path, 100)
}

// etsy-ext: Flat receipts survive a boundary refusal below the daily reserve floor.
func TestEtsySyncCLIBoundaryRefusesBelowFloor(t *testing.T) {
	syncCLIIsolate(t)
	path := syncCLIDB(t, 0)
	srv := syncCLIServer(t, 300, "100", 0)
	output, err := syncCLIRun(t, path)
	syncCLIRequireQuotaExit(t, err)
	if !strings.Contains(err.Error(), "daily reserve reached (100 left, floor 250)") {
		t.Fatal(err)
	}
	syncCLIRequireEvent(t, output, "budget_boundary")
	if checked := syncCLIPaymentPaths(t, srv); len(checked) != 0 {
		t.Fatalf("payments %d", len(checked))
	}
	syncCLIRequireCount(t, path, 300)
}

// etsy-ext: Quota caps save receipt progress and the next run walks only unchecked receipts.
func TestEtsySyncCLIBoundaryCapsPayments(t *testing.T) {
	syncCLIIsolate(t)
	path := syncCLIDB(t, 0)
	srv := syncCLIServer(t, 300, "400", 0)
	output, err := syncCLIRun(t, path)
	syncCLIRequireQuotaExit(t, err)
	if !strings.Contains(err.Error(), "re-run to continue (progress is saved)") {
		t.Fatal(err)
	}
	checked := syncCLIPaymentPaths(t, srv)
	// etsy-ext: The daily floor reserves 250 of the 400 remaining calls.
	if len(checked) == 0 || len(checked) > 150 {
		t.Fatalf("payments %d", len(checked))
	}
	syncCLIRequireCount(t, path, 300)
	syncCLIRequireEvent(t, output, "payments_capped")
	found := false
	for _, line := range strings.Split(output, "\n") {
		var event struct {
			Reason             string
			Checked, Remaining int
		}
		if json.Unmarshal([]byte(line), &event) == nil && event.Reason == "payments_capped" {
			found = true
			if event.Checked != len(checked) || event.Checked+event.Remaining != 300 {
				t.Fatalf("event %s", line)
			}
		}
	}
	if !found {
		t.Fatal(output)
	}
	client.ObserveEtsyQuota(http.Header{"X-Limit-Per-Day": {"5000"}, "X-Remaining-Today": {"4000"}}, etsyTestQuotaKey(t))
	srv2 := syncCLIServer(t, 300, "4000", 0)
	_, err = syncCLIRun(t, path)
	s2Must(t, err)
	paths2 := syncCLIPaymentPaths(t, srv2)
	if len(paths2) != 300-len(checked) {
		t.Fatalf("second payments %d", len(paths2))
	}
	for p := range paths2 {
		if checked[p] {
			t.Fatalf("repeated %s", p)
		}
	}
}

func TestEtsySyncCLIBoundaryAffordableFansOut(t *testing.T) {
	syncCLIIsolate(t)
	path := syncCLIDB(t, 0)
	srv := syncCLIServer(t, 300, "4000", 0)
	output, err := syncCLIRun(t, path)
	s2Must(t, err)
	if strings.Contains(output, `"reason":"budget_boundary"`) {
		t.Fatalf("unexpected refusal: %s", output)
	}
	for i := 0; i < 300; i++ {
		p := fmt.Sprintf("/shops/9000001/receipts/%d/payments", 3000000001+i)
		if n := srv.count("GET", p); n != 1 {
			t.Fatalf("%s requests=%d", p, n)
		}
	}
	syncCLIRequireCount(t, path, 300)
}

func TestEtsySyncCLIUnknownQuotaBigPlanRefuses(t *testing.T) {
	for _, capped := range []bool{false, true} {
		name := "uncapped"
		if capped {
			name = "withMaxCalls"
		}
		t.Run(name, func(t *testing.T) {
			syncCLIIsolate(t)
			path := syncCLIDB(t, 1100)
			failAt := 0
			if capped {
				failAt = 1
			}
			srv := syncCLIServer(t, 0, "", failAt)
			var extra []string
			if capped {
				extra = []string{"--max-calls", "1200"}
			}
			_, err := syncCLIRun(t, path, extra...)
			syncCLIRequireQuotaExit(t, err)
			if capped {
				if strings.Contains(err.Error(), "quota is unknown") || len(srv.all()) != 1 {
					t.Fatalf("err=%v requests=%d", err, len(srv.all()))
				}
				// etsy-ext: Unknown quota is discovered at the dependent boundary after flat reads.
			} else if !strings.Contains(err.Error(), "--max-calls") || !strings.Contains(err.Error(), "quota is unknown") || srv.count("GET", "/shops/9000001/receipts") < 1 || len(syncCLIPaymentPaths(t, srv)) != 0 {
				t.Fatalf("err=%v requests=%d", err, len(srv.all()))
			}
		})
	}
}

// etsy-ext: Assert request identities, including absence of duplicate payment walks.
func syncCLIPaymentPaths(t *testing.T, srv *s2Server) map[string]bool {
	t.Helper()
	paths := map[string]bool{}
	for _, req := range srv.all() {
		if strings.HasSuffix(req.Path, "/payments") {
			if paths[req.Path] {
				t.Fatalf("duplicate payment request %s", req.Path)
			}
			paths[req.Path] = true
		}
	}
	return paths
}

// etsy-ext: Explicit and host-imposed call limits compose with capped payments.
func TestEtsySyncCLIMaxCallsCapsPaymentsAndSavesProgress(t *testing.T) {
	syncCLIIsolate(t)
	path := syncCLIDB(t, 0)
	srv := syncCLIServer(t, 300, "4000", 0)
	output, err := syncCLIRun(t, path, "--max-calls", "100")
	syncCLIRequireQuotaExit(t, err)
	if !strings.Contains(err.Error(), "re-run to continue") || len(srv.all()) > 100 {
		t.Fatalf("%v requests %d", err, len(srv.all()))
	}
	syncCLIRequireEvent(t, output, "payments_capped")
	payments := syncCLIPaymentPaths(t, srv)
	db, err := store.Open(path)
	s2Must(t, err)
	defer db.Close()
	checks, err := db.ReadPaymentChecks()
	s2Must(t, err)
	if len(payments) == 0 || len(checks) != len(payments) {
		t.Fatalf("checks %d payments %d", len(checks), len(payments))
	}
}

// etsy-ext: MCP's implicit 500-call limit also saves capped progress.
func TestEtsySyncCLIMCPClampCapsPayments(t *testing.T) {
	syncCLIIsolate(t)
	t.Setenv(cliutil.MCPChildEnvVar, "1")
	path := syncCLIDB(t, 0)
	srv := syncCLIServer(t, 700, "4000", 0)
	_, err := syncCLIRun(t, path)
	syncCLIRequireQuotaExit(t, err)
	if !strings.Contains(err.Error(), "re-run to continue") || len(srv.all()) > 500 {
		t.Fatalf("%v requests %d", err, len(srv.all()))
	}
}

// etsy-ext: A live daily stop retains priority over planned payment progress.
func TestEtsySyncCLIGuardStopBeatsPaymentsCap(t *testing.T) {
	syncCLIIsolate(t)
	path := syncCLIDB(t, 0)
	var first atomic.Bool
	srv := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
		// etsy-ext: Stay above the daily reserve floor so the payment guard is exercised.
		syncCLIQuota(w, "400")
		if strings.HasSuffix(r.URL.Path, "/payments") {
			if !first.Swap(true) {
				syncCLIQuota(w, "0")
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(429)
				_, _ = w.Write([]byte(`{"error":"daily quota exhausted"}`))
			} else {
				_, _ = w.Write([]byte(`{"results":[],"count":0}`))
			}
		} else {
			offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
			_ = json.NewEncoder(w).Encode(map[string]any{"results": syncCLIReceipts(min(100, max(0, 300-offset)), offset), "count": 300})
		}
		return true
	})
	output, err := syncCLIRun(t, path)
	syncCLIRequireQuotaExit(t, err)
	if len(srv.all()) == 0 || !first.Load() || !strings.Contains(err.Error(), "daily request quota exhausted") || strings.Contains(output+err.Error(), "re-run to continue") {
		t.Fatalf("%v %s", err, output)
	}
}

// etsy-ext: Flat guard stops must not dispatch or report failed dependents.
func TestEtsySyncCLIFlatMaxCallsStopSkipsDependents(t *testing.T) {
	syncCLIIsolate(t)
	path := syncCLIDB(t, 0)
	srv := syncCLIServer(t, 500, "4000", 0)
	output, err := syncCLIRun(t, path, "--max-calls", "3")
	syncCLIRequireQuotaExit(t, err)
	if err == nil || len(syncCLIPaymentPaths(t, srv)) != 0 || strings.Contains(output+err.Error(), "shops_receipts_payments") {
		t.Fatalf("%v %s", err, output)
	}
	syncCLIRequireEvent(t, output, "max_calls_reached")
}

// etsy-ext: Timestamped receipts reuse checks even when the second response lowers quota.
func TestEtsySyncCLITwoRunTimestampedReceipts(t *testing.T) {
	syncCLIIsolate(t)
	path := syncCLIDB(t, 0)
	srv := syncCLIServer(t, 300, "4000", 0)
	_, err := syncCLIRun(t, path)
	s2Must(t, err)
	if len(syncCLIPaymentPaths(t, srv)) != 300 {
		t.Fatal("first run did not check every receipt")
	}
	srv2 := syncCLIServer(t, 300, "100", 0)
	output, err := syncCLIRun(t, path)
	// etsy-ext: A nil Execute error is process exit 0; ExitCode classifies non-nil errors only.
	if err != nil || len(syncCLIPaymentPaths(t, srv2)) != 0 || strings.Contains(output, "payments_capped") {
		t.Fatalf("%v %s", err, output)
	}
}

func TestEtsySyncCLIFlatOnlyMaxCallsAfterCompleteSync(t *testing.T) {
	syncCLIIsolate(t)
	etsyPath := syncCLIDB(t, 0)
	etsyFirst := syncCLIServer(t, 300, "4000", 0)
	_, etsyErr := syncCLIRun(t, etsyPath)
	s2Must(t, etsyErr)
	if len(syncCLIPaymentPaths(t, etsyFirst)) != 300 {
		t.Fatal("incomplete first payment walk")
	}
	etsyDB, etsyErr := store.Open(etsyPath)
	s2Must(t, etsyErr)
	etsyFlat, etsyOther, etsyPayments := estimateSyncCallsSplit(etsyDB, []string{"receipts"}, []string{"receipts"}, 0, false, etsySyncPlan{checked: true})
	etsyDB.Close()
	if etsyFlat != 4 || etsyOther != 0 || etsyPayments != 0 {
		t.Fatalf("preflight: flat=%d other=%d payments=%d", etsyFlat, etsyOther, etsyPayments)
	}
	etsySecond := syncCLIServer(t, 300, "4000", 0)
	_, etsyErr = syncCLIRun(t, etsyPath, "--max-calls", "4")
	s2Must(t, etsyErr)
	if len(syncCLIPaymentPaths(t, etsySecond)) != 0 {
		t.Fatal("second run fetched payments")
	}
	syncCLIRequireCount(t, etsyPath, 300)
}

func etsyRequirePaymentsComplete(t *testing.T, etsyPath string, etsyWant int) {
	t.Helper()
	etsyDB, etsyErr := store.Open(etsyPath)
	s2Must(t, etsyErr)
	defer etsyDB.Close()
	etsyRows, etsyErr := etsyDB.Query(`SELECT last_attempt_complete FROM sync_state WHERE resource_type = 'shops_receipts_payments'`)
	s2Must(t, etsyErr)
	defer etsyRows.Close()
	etsyCount := 0
	for etsyRows.Next() {
		var etsyGot int
		s2Must(t, etsyRows.Scan(&etsyGot))
		etsyCount++
		if etsyGot != etsyWant {
			t.Fatalf("last_attempt_complete=%d want %d", etsyGot, etsyWant)
		}
	}
	s2Must(t, etsyRows.Err())
	if etsyCount == 0 {
		t.Fatal("missing payment sync state")
	}
}

func TestEtsySyncCLICappedNeverMarksPaymentsComplete(t *testing.T) {
	syncCLIIsolate(t)
	etsyPath := syncCLIDB(t, 0)
	syncCLIServer(t, 300, "400", 0)
	_, etsyErr := syncCLIRun(t, etsyPath)
	syncCLIRequireQuotaExit(t, etsyErr)
	etsyRequirePaymentsComplete(t, etsyPath, 0)
	client.ObserveEtsyQuota(http.Header{"X-Limit-Per-Day": {"5000"}, "X-Remaining-Today": {"4000"}}, etsyTestQuotaKey(t))
	syncCLIServer(t, 300, "4000", 0)
	_, etsyErr = syncCLIRun(t, etsyPath)
	s2Must(t, etsyErr)
	etsyRequirePaymentsComplete(t, etsyPath, 1)
}

// etsyPaymentFailureServer supplies full pages when pagination must exhaust a guard.
func etsyPaymentFailureServer(t *testing.T, etsyPaginate bool) *s2Server {
	t.Helper()
	t.Setenv(cliutil.VerifyEnvVar, "1")
	return s2NewServer(t, func(etsyW http.ResponseWriter, etsyR *http.Request, _ string) bool {
		syncCLIQuota(etsyW, "400")
		if strings.HasSuffix(etsyR.URL.Path, "/payments") {
			if !etsyPaginate {
				etsyW.WriteHeader(500)
				_, _ = etsyW.Write([]byte(`{"error":"synthetic payment failure"}`))
			} else {
				etsyLimit := determinePaginationDefaults("shops_receipts_payments").limit
				etsyRows := make([]map[string]any, etsyLimit)
				for etsyI := range etsyRows {
					etsyOffset, _ := strconv.Atoi(etsyR.URL.Query().Get("offset"))
					etsyRows[etsyI] = map[string]any{"payment_id": 7000000001 + etsyOffset + etsyI, "receipt_id": 3000000001}
				}
				_ = json.NewEncoder(etsyW).Encode(map[string]any{"results": etsyRows, "count": 100000})
			}
		} else {
			etsyOffset, _ := strconv.Atoi(etsyR.URL.Query().Get("offset"))
			_ = json.NewEncoder(etsyW).Encode(map[string]any{"results": syncCLIReceipts(min(100, max(0, 300-etsyOffset)), etsyOffset), "count": 300})
		}
		return true
	})
}

func TestEtsySyncCLICriticalFailureBeatsCap(t *testing.T) {
	syncCLIIsolate(t)
	etsyOld, etsyExists := criticalResources["shops_receipts_payments"]
	criticalResources["shops_receipts_payments"] = true
	t.Cleanup(func() {
		if etsyExists {
			criticalResources["shops_receipts_payments"] = etsyOld
		} else {
			delete(criticalResources, "shops_receipts_payments")
		}
	})
	etsyPath := syncCLIDB(t, 0)
	etsyPaymentFailureServer(t, false)
	_, etsyErr := syncCLIRun(t, etsyPath)
	etsyWant := describeCriticalFailedResources(1, []string{"shops_receipts_payments"})
	if etsyErr == nil || etsyErr.Error() != etsyWant || strings.Contains(etsyErr.Error(), "progress is saved") {
		t.Fatalf("want %q got %v", etsyWant, etsyErr)
	}
}

func TestEtsySyncCLICappedAllPaymentsFailReportsFailure(t *testing.T) {
	syncCLIIsolate(t)
	etsyPath := syncCLIDB(t, 0)
	etsyPaymentFailureServer(t, false)
	etsyOutput, etsyErr := syncCLIRun(t, etsyPath)
	if etsyErr == nil || ExitCode(etsyErr) != 1 || !etsyPaymentsCapFailed(etsyErr) {
		t.Fatalf("want plain failure, got %v", etsyErr)
	}
	if !strings.Contains(etsyErr.Error(), "shops_receipts_payments") || strings.Contains(etsyErr.Error(), "progress is saved") {
		t.Fatal(etsyErr)
	}
	syncCLIRequireEvent(t, etsyOutput, "payments_failed")
	matched := false
	for _, line := range strings.Split(etsyOutput, "\n") {
		var event struct{ Event, Reason, Message string }
		if json.Unmarshal([]byte(line), &event) == nil && event.Event == "sync_warning" && event.Reason == "payments_failed" {
			matched = true
			if event.Message != etsyErr.Error() {
				t.Fatalf("warning message %q != error %q", event.Message, etsyErr.Error())
			}
		}
	}
	if !matched {
		t.Fatal("missing payment failure warning")
	}

	etsyDB, etsyErr := store.Open(etsyPath)
	s2Must(t, etsyErr)
	defer etsyDB.Close()
	etsyChecks, etsyErr := etsyDB.ReadPaymentChecks()
	s2Must(t, etsyErr)
	if len(etsyChecks) != 0 {
		t.Fatalf("failed payments saved %d ledger rows", len(etsyChecks))
	}
}

// etsy-ext: Payments are not paginated, so a paginated sibling dependent (receipts_listings)
// overruns its estimate and the --max-calls guard stops while the payments walk is capped.
func TestEtsySyncCLIMaxCallsStopDuringCappedWalkNoResumeHint(t *testing.T) {
	syncCLIIsolate(t)
	etsyPath := syncCLIDB(t, 0)
	etsyServer := s2NewServer(t, func(etsyW http.ResponseWriter, etsyR *http.Request, _ string) bool {
		syncCLIQuota(etsyW, "4000")
		etsyOffset, _ := strconv.Atoi(etsyR.URL.Query().Get("offset"))
		switch {
		case strings.HasSuffix(etsyR.URL.Path, "/payments"):
			_ = json.NewEncoder(etsyW).Encode(map[string]any{"results": []any{map[string]any{"payment_id": 7000000001, "receipt_id": 3000000001}}, "count": 1})
		case strings.HasSuffix(etsyR.URL.Path, "/listings"):
			// Two receipts need a second page, so listings overrun their estimated one call each
			// and the guard stops during the capped payments walk.
			var etsyReceipt int64
			_, _ = fmt.Sscanf(etsyR.URL.Path, "/shops/9000001/receipts/%d/listings", &etsyReceipt)
			etsyRowsN := 1
			if etsyReceipt <= 3000000002 && etsyOffset == 0 {
				etsyRowsN = determinePaginationDefaults("receipts_listings").limit
			}
			etsyRows := make([]map[string]any, etsyRowsN)
			for etsyI := range etsyRows {
				etsyRows[etsyI] = map[string]any{"listing_id": 1000000001 + etsyOffset + etsyI}
			}
			_ = json.NewEncoder(etsyW).Encode(map[string]any{"results": etsyRows, "count": 100000})
		default:
			_ = json.NewEncoder(etsyW).Encode(map[string]any{"results": syncCLIReceipts(6, 0), "count": 6})
		}
		return true
	})
	etsyStdout, etsyStderr, etsyErr := s2Run(t, "sync", "--resources", "receipts,receipts_listings", "--db", etsyPath, "--rate-limit", "1000", "--json", "--max-calls", "12", "--concurrency", "1")
	syncCLIRequireQuotaExit(t, etsyErr)
	if !strings.Contains(etsyErr.Error(), "--max-calls 12 reached") {
		t.Fatal(etsyErr)
	}
	etsyAll := etsyStdout + etsyStderr
	if etsyErr != nil {
		etsyAll += etsyErr.Error()
	}
	_ = etsyServer
	syncCLIRequireEvent(t, etsyAll, "max_calls_reached")
	if strings.Contains(etsyAll, "re-run to continue") || strings.Contains(etsyAll, `"reason":"payments_capped"`) {
		t.Fatalf("guard stop claimed capped progress: %v %s", etsyErr, etsyAll)
	}
}

// etsy-ext: The flat response can consume the reserve before payments begin.
func TestEtsySyncCLIBoundaryRefusesAtReserve(t *testing.T) {
	syncCLIIsolate(t)
	path := syncCLIDB(t, 0)
	srv := syncCLIServer(t, 5, "250", 0)
	output, err := syncCLIRun(t, path)
	syncCLIRequireQuotaExit(t, err)
	if !strings.Contains(err.Error(), "daily reserve reached") {
		t.Fatal(err)
	}
	syncCLIRequireEvent(t, output, "budget_boundary")
	if paths := syncCLIPaymentPaths(t, srv); len(paths) != 0 {
		t.Fatalf("unexpected payments: %v", paths)
	}
	syncCLIRequireCount(t, path, 5)
	db, err := store.Open(path)
	s2Must(t, err)
	defer db.Close()
	rows, err := db.Query(`SELECT COUNT(*) FROM sync_state WHERE resource_type = 'shops_receipts_payments' AND last_attempt_complete = 1`)
	s2Must(t, err)
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("missing count")
	}
	var complete int
	s2Must(t, rows.Scan(&complete))
	s2Must(t, rows.Err())
	if complete != 0 {
		t.Fatal("refused payment attempt marked complete")
	}
}

func TestEtsySyncCLIStrictQuotaStopExits7(t *testing.T) {
	syncCLIIsolate(t)
	path := syncCLIDB(t, 0)
	syncCLIServer(t, 500, "4000", 2)
	_, err := syncCLIRun(t, path, "--strict")
	syncCLIRequireQuotaExit(t, err)
	if !strings.Contains(err.Error(), "daily request quota exhausted") {
		t.Fatal(err)
	}
}

func TestEtsySyncCLIMaxCallsStopStrictExits7(t *testing.T) {
	syncCLIIsolate(t)
	path := syncCLIDB(t, 0)
	syncCLIServer(t, 500, "4000", 0)
	_, err := syncCLIRun(t, path, "--strict", "--max-calls", "3")
	syncCLIRequireQuotaExit(t, err)
	if !strings.Contains(err.Error(), "--max-calls 3 reached") {
		t.Fatal(err)
	}
}

func syncCLIForceCritical(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		old, exists := criticalResources[name]
		criticalResources[name] = true
		t.Cleanup(func() {
			if exists {
				criticalResources[name] = old
			} else {
				delete(criticalResources, name)
			}
		})
	}
}

func TestEtsySyncCLIQuotaStopWithCriticalFailureCriticalWins(t *testing.T) {
	syncCLIIsolate(t)
	t.Setenv(cliutil.VerifyEnvVar, "1") // Disable retry delays for synthetic 500s.
	first, second := "shop_sections", "return_policies"
	syncCLIForceCritical(t, first, second)
	path := syncCLIDB(t, 0)
	srv := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
		switch r.URL.Path {
		case "/shops/9000001/sections":
			syncCLIQuota(w, "4000")
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"error":"synthetic failure"}`))
		case "/shops/9000001/policies/return":
			syncCLIQuota(w, "0")
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"error":"daily quota exhausted"}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		return true
	})
	_, _, err := s2Run(t, "sync", "--resources", first+","+second, "--db", path, "--concurrency", "1", "--rate-limit", "1000", "--json")
	want := describeCriticalFailedResources(1, []string{first})
	if err == nil || err.Error() != want || ExitCode(err) == 7 {
		t.Fatalf("want %q, got %v", want, err)
	}
	requests := srv.all()
	if len(requests) < 2 || requests[0].Path != "/shops/9000001/sections" || requests[len(requests)-1].Path != "/shops/9000001/policies/return" {
		t.Fatalf("not FIFO: %+v", requests)
	}
}

func TestEtsySyncCLILookupRefreshRunsOnFailedRun(t *testing.T) {
	syncCLIIsolate(t)
	t.Setenv(cliutil.VerifyEnvVar, "")
	t.Setenv(cliutil.DogfoodEnvVar, "")
	t.Setenv("ETSY_NO_LEARN", "false")
	syncCLIForceCritical(t, "return_policies")
	path := s2FixtureDB(t, func(db *store.Store) {
		s2Must(t, lookups.RecordMisses(context.Background(), db.DB(), []string{"Zephyr"}))
	})
	s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
		syncCLIQuota(w, "4000")
		switch r.URL.Path {
		case "/shops/9000001/sections":
			_, _ = w.Write([]byte(`{"results":[{"shop_section_id":123,"title":"Zephyr"}],"count":1}`))
		case "/shops/9000001/policies/return":
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"error":"synthetic failure"}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		return true
	})
	out, stderr, err := s2Run(t, "sync", "--resources", "shop_sections,return_policies", "--db", path, "--concurrency", "1", "--rate-limit", "1000", "--json")
	want := describeCriticalFailedResources(1, []string{"return_policies"})
	if err == nil || err.Error() != want {
		t.Fatalf("want %q got %v: %s", want, err, out+stderr)
	}
	db, err := store.Open(path)
	s2Must(t, err)
	defer db.Close()
	var n int
	s2Must(t, db.DB().QueryRow(`SELECT COUNT(*) FROM entity_lookups WHERE source='synced' AND kind='shop_sections' AND canonical='Zephyr'`).Scan(&n))
	if n != 1 {
		t.Fatalf("derived lookups=%d; output=%s", n, out+stderr)
	}
}
