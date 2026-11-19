package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

func etsyForbiddenServer(t *testing.T, n int, mode *atomic.Int64) *s2Server {
	t.Helper()
	return s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
		syncCLIQuota(w, "4000")
		if r.URL.Path == "/shops/9000001/receipts" {
			if mode.Load() == 8 {
				w.Header().Set("Content-Type", "text/plain")
				_, _ = w.Write([]byte("synthetic non-json response"))
				return true
			}
			etsyOffset, etsyErr := strconv.Atoi(r.URL.Query().Get("offset"))
			if r.URL.Query().Get("offset") != "" {
				s2Must(t, etsyErr)
			}
			if r.URL.Query().Get("limit") != "100" {
				t.Errorf("limit=%q", r.URL.Query().Get("limit"))
			}
			count := n
			if mode.Load() == 6 {
				count++
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"results": syncCLIReceipts(min(100, max(0, count-etsyOffset)), etsyOffset), "count": count})
		} else if strings.HasSuffix(r.URL.Path, "/payments") {
			var etsyReceipt int64
			_, etsyErr := fmt.Sscanf(r.URL.Path, "/shops/9000001/receipts/%d/payments", &etsyReceipt)
			s2Must(t, etsyErr)
			etsyStatus := 200
			switch mode.Load() {
			case 7:
				etsyStatus = 403
				if etsyReceipt == 3000000001 {
					etsyStatus = 404
				}
			case 0, 6, 8:
				etsyStatus = 403
			case 2:
				if etsyReceipt <= 3000000002 {
					etsyStatus = 403
				}
			case 3:
				if etsyReceipt == 3000000001 {
					etsyStatus = 403
				}
			case 4:
				if etsyReceipt == 3000000001 {
					etsyStatus = 404
				}
			}
			if etsyStatus != 200 {
				w.WriteHeader(etsyStatus)
				_, _ = w.Write([]byte(`{"error":"synthetic access denial"}`))
			} else {
				_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{"payment_id": etsyReceipt + 1000000000, "receipt_id": etsyReceipt}}, "count": 1})
			}
		} else {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		return true
	})
}

func etsyForbiddenPaths(srv *s2Server) []string {
	var etsyPaths []string
	for _, etsyReq := range srv.all() {
		if strings.HasSuffix(etsyReq.Path, "/payments") {
			etsyPaths = append(etsyPaths, etsyReq.Path)
		}
	}
	return etsyPaths
}
func etsyForbiddenWantPaths(t *testing.T, got []string, ids ...int) {
	t.Helper()
	etsyWant := make([]string, 0, len(ids))
	for _, etsyID := range ids {
		etsyWant = append(etsyWant, fmt.Sprintf("/shops/9000001/receipts/%d/payments", 3000000000+etsyID))
	}
	etsyGot := append([]string{}, got...)
	sort.Strings(etsyGot)
	sort.Strings(etsyWant)
	if !reflect.DeepEqual(etsyGot, etsyWant) {
		t.Fatalf("payments=%v want=%v", etsyGot, etsyWant)
	}
}
func etsyForbiddenLedger(t *testing.T, path string, forbidden, stamped, complete int) map[string]store.PaymentCheck {
	t.Helper()
	etsyDB, etsyErr := store.Open(path)
	s2Must(t, etsyErr)
	defer etsyDB.Close()
	etsyRows, etsyErr := etsyDB.ReadPaymentChecks()
	s2Must(t, etsyErr)
	etsyCount, etsyErr := etsyDB.CountPaymentForbidden()
	s2Must(t, etsyErr)
	etsyStamps := 0
	etsyFlags := 0
	for _, etsyRow := range etsyRows {
		if etsyRow.ReceiptUpdated.Valid {
			etsyStamps++
		}
		if etsyRow.Forbidden {
			etsyFlags++
		}
	}
	var etsyComplete int
	s2Must(t, etsyDB.DB().QueryRow("SELECT last_attempt_complete FROM sync_state WHERE resource_type='shops_receipts_payments'").Scan(&etsyComplete))
	if etsyCount != forbidden || etsyFlags != forbidden || etsyStamps != stamped || etsyComplete != complete {
		t.Fatalf("forbidden=%d flags=%d stamped=%d complete=%d want %d/%d/%d", etsyCount, etsyFlags, etsyStamps, etsyComplete, forbidden, stamped, complete)
	}
	return etsyRows
}
func etsyForbiddenWarning(t *testing.T, output string, n int) {
	t.Helper()
	etsyFound := false
	for _, etsyLine := range strings.Split(output, "\n") {
		var etsyEvent struct {
			Event     string
			Reason    string
			Forbidden int
			Message   string
		}
		if json.Unmarshal([]byte(etsyLine), &etsyEvent) == nil && etsyEvent.Event == "sync_warning" && etsyEvent.Reason == "payments_forbidden" {
			if etsyEvent.Forbidden != n || (etsyEvent.Message != etsyPaymentsForbiddenMessage(n) && etsyEvent.Message != etsyPaymentsForbiddenScopeMessage(n) && etsyEvent.Message != etsyPaymentsForbiddenMessage(n)+` If this token recently lost the transactions_r scope, re-authorize (etsy-pp-cli auth login --scopes "… transactions_r"); run 'sync --full' to re-check previously synced receipts.`) {
				t.Fatalf("warning=%+v", etsyEvent)
			}
			if strings.Count(etsyEvent.Message, "re-authorize") > 1 {
				t.Fatalf("duplicated scope advice: %s", etsyEvent.Message)
			}
			etsyFound = true
		}
	}
	if etsyFound != (n > 0) {
		t.Fatalf("warning presence=%v want count=%d: %s", etsyFound, n, output)
	}
}
func etsyForbiddenExit(t *testing.T, err error) {
	t.Helper()
	if err == nil || ExitCode(err) != 1 || !strings.Contains(err.Error(), "transactions_r") {
		t.Fatalf("want forbidden exit 1, got %v (%d)", err, ExitCode(err))
	}
}

func TestEtsyPaymentsForbiddenNeverSuccessful(t *testing.T) {
	syncCLIIsolate(t)
	etsyPath := syncCLIDB(t, 0)
	var etsyMode atomic.Int64
	etsyServer := etsyForbiddenServer(t, 300, &etsyMode)
	etsyOut, etsyErr := syncCLIRun(t, etsyPath)
	etsyForbiddenExit(t, etsyErr)
	if !strings.Contains(etsyOut, "re-authorize with the transactions_r scope") {
		t.Fatal(etsyOut)
	}
	etsyIDs := make([]int, 300)
	for etsyI := range etsyIDs {
		etsyIDs[etsyI] = etsyI + 1
	}
	etsyForbiddenWantPaths(t, etsyForbiddenPaths(etsyServer), etsyIDs...)
	if len(etsyForbiddenLedger(t, etsyPath, 300, 300, 0)) != 300 {
		t.Fatal("ledger size")
	}
	etsyForbiddenWarning(t, etsyOut, 300)
	etsyBefore := len(etsyForbiddenPaths(etsyServer))
	etsyOut, etsyErr = syncCLIRun(t, etsyPath)
	etsyForbiddenExit(t, etsyErr)
	etsyProbe := etsyForbiddenPaths(etsyServer)[etsyBefore:]
	if len(etsyProbe) != 1 {
		t.Fatal(etsyProbe)
	}
	etsyForbiddenLedger(t, etsyPath, 300, 300, 0)
	etsyForbiddenWarning(t, etsyOut, 300)
	etsyMode.Store(1)
	etsyBefore = len(etsyForbiddenPaths(etsyServer))
	etsyOut, etsyErr = syncCLIRun(t, etsyPath)
	s2Must(t, etsyErr)
	etsyProbe = etsyForbiddenPaths(etsyServer)[etsyBefore:]
	if len(etsyProbe) != 1 {
		t.Fatal(etsyProbe)
	}
	etsyRows := etsyForbiddenLedger(t, etsyPath, 0, 1, 0)
	if len(etsyRows) != 300 {
		t.Fatal(len(etsyRows))
	}
	for _, etsyRow := range etsyRows {
		if etsyRow.Denied {
			t.Fatal(etsyRow)
		}
	}
	etsyForbiddenWarning(t, etsyOut, 0)
	etsyBefore = len(etsyForbiddenPaths(etsyServer))
	etsyOut, etsyErr = syncCLIRun(t, etsyPath)
	s2Must(t, etsyErr)
	etsyNext := etsyForbiddenPaths(etsyServer)[etsyBefore:]
	if len(etsyNext) != 299 {
		t.Fatal(len(etsyNext))
	}
	etsySeen := map[string]bool{}
	for _, etsyP := range etsyNext {
		if etsyP == etsyProbe[0] || etsySeen[etsyP] {
			t.Fatal("repeated path", etsyP)
		}
		etsySeen[etsyP] = true
	}
	etsyForbiddenLedger(t, etsyPath, 0, 300, 1)
	etsyForbiddenWarning(t, etsyOut, 0)
}

func TestEtsyPaymentsForbiddenDoesNotStarve(t *testing.T) {
	for _, etsyFull := range []bool{false, true} {
		etsyName := "incremental"
		if etsyFull {
			etsyName = "full"
		}
		t.Run(etsyName, func(t *testing.T) {
			syncCLIIsolate(t)
			etsyPath := syncCLIDB(t, 0)
			var etsyMode atomic.Int64
			etsyMode.Store(2)
			etsyServer := etsyForbiddenServer(t, 4, &etsyMode)
			// One flat receipts page plus exactly two payment requests.
			etsyArgs := []string{"--max-calls", "3"}
			if etsyFull {
				etsyArgs = append(etsyArgs, "--full")
			}
			_, etsyErr := syncCLIRun(t, etsyPath, etsyArgs...)
			etsyForbiddenExit(t, etsyErr)
			etsyForbiddenWantPaths(t, etsyForbiddenPaths(etsyServer), 1, 2)
			etsyBefore := len(etsyForbiddenPaths(etsyServer))
			_, etsyErr = syncCLIRun(t, etsyPath, etsyArgs...)
			syncCLIRequireQuotaExit(t, etsyErr)
			etsyForbiddenWantPaths(t, etsyForbiddenPaths(etsyServer)[etsyBefore:], 3, 4)
			etsyRows := etsyForbiddenLedger(t, etsyPath, 2, 4, 0)
			for _, etsyID := range []string{"3000000003", "3000000004"} {
				if etsyRows[etsyID].Forbidden || !etsyRows[etsyID].ReceiptUpdated.Valid {
					t.Fatal(etsyRows[etsyID])
				}
			}
		})
	}
}

func TestEtsyPaymentsForbiddenMixedProgress(t *testing.T) {
	syncCLIIsolate(t)
	etsyPath := syncCLIDB(t, 0)
	var etsyMode atomic.Int64
	etsyMode.Store(3)
	etsyServer := etsyForbiddenServer(t, 4, &etsyMode)
	etsyOut, etsyErr := syncCLIRun(t, etsyPath)
	s2Must(t, etsyErr)
	etsyForbiddenWantPaths(t, etsyForbiddenPaths(etsyServer), 1, 2, 3, 4)
	etsyForbiddenWarning(t, etsyOut, 1)
	etsyRows := etsyForbiddenLedger(t, etsyPath, 1, 4, 0)
	for etsyID, etsyRow := range etsyRows {
		if etsyRow.Forbidden != (etsyID == "3000000001") || !etsyRow.ReceiptUpdated.Valid {
			t.Fatal(etsyID, etsyRow)
		}
	}
	etsyBefore := len(etsyForbiddenPaths(etsyServer))
	etsyOut, etsyErr = syncCLIRun(t, etsyPath)
	s2Must(t, etsyErr)
	etsyForbiddenWarning(t, etsyOut, 1)
	if !strings.Contains(etsyOut, "usually receipts this token can't see") {
		t.Fatal(etsyOut)
	}
	etsyForbiddenWantPaths(t, etsyForbiddenPaths(etsyServer)[etsyBefore:], 1)
}

func TestEtsyPaymentsNotFoundStaysDurable(t *testing.T) {
	syncCLIIsolate(t)
	etsyPath := syncCLIDB(t, 0)
	var etsyMode atomic.Int64
	etsyMode.Store(4)
	etsyServer := etsyForbiddenServer(t, 3, &etsyMode)
	etsyOut, etsyErr := syncCLIRun(t, etsyPath)
	s2Must(t, etsyErr)
	etsyForbiddenWantPaths(t, etsyForbiddenPaths(etsyServer), 1, 2, 3)
	etsyRows := etsyForbiddenLedger(t, etsyPath, 0, 3, 1)
	if !etsyRows["3000000001"].Denied || etsyRows["3000000001"].Forbidden {
		t.Fatal(etsyRows)
	}
	etsyForbiddenWarning(t, etsyOut, 0)
	etsyBefore := len(etsyForbiddenPaths(etsyServer))
	etsyOut, etsyErr = syncCLIRun(t, etsyPath)
	s2Must(t, etsyErr)
	etsyForbiddenWantPaths(t, etsyForbiddenPaths(etsyServer)[etsyBefore:])
	etsyForbiddenWarning(t, etsyOut, 0)
}

// The same 403 must fail only when that receipt, rather than an unrelated one, succeeded.
func TestEtsyPaymentsForbiddenPerReceiptScopeLoss(t *testing.T) {
	for _, prior := range []string{"absent", "unrelated", "ok", "legacy", "rearmed", "forbidden", "denied"} {
		for _, strict := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s_strict%v", prior, strict), func(t *testing.T) {
				syncCLIIsolate(t)
				path := syncCLIDB(t, 0)
				db, err := store.Open(path)
				s2Must(t, err)
				s2Must(t, store.EnsurePaymentChecksSchema(db.DB()))
				switch prior {
				case "ok":
					s2Must(t, db.RecordPaymentCheck("3000000001", sql.NullInt64{}))
				case "unrelated":
					s2Must(t, db.RecordPaymentCheck("3999999999", sql.NullInt64{}))
				case "legacy":
					_, err = db.DB().Exec(`INSERT INTO etsy_payment_checks(receipt_id,receipt_updated,checked_at,status) VALUES ('3000000001',100,1,NULL)`)
				case "rearmed":
					_, err = db.DB().Exec(`INSERT INTO etsy_payment_checks(receipt_id,receipt_updated,checked_at,status) VALUES ('3000000001',NULL,1,NULL)`)
				case "forbidden":
					s2Must(t, db.RecordPaymentForbidden("3000000001", sql.NullInt64{}))
				case "denied":
					s2Must(t, db.RecordPaymentDenied("3000000001", sql.NullInt64{Int64: 100, Valid: true}))
				}
				s2Must(t, err)
				s2Must(t, db.Close())
				var mode atomic.Int64
				srv := etsyForbiddenServer(t, 1, &mode)
				args := []string{"--full"}
				if strict {
					args = append(args, "--strict")
				}
				out, err := syncCLIRun(t, path, args...)
				etsyForbiddenWantPaths(t, etsyForbiddenPaths(srv), 1)
				if strict {
					syncCLIRequireEvent(t, out, "payments_forbidden_strict")
					if err == nil || !strings.Contains(err.Error(), "--strict") || strings.Contains(err.Error(), "re-authorize") {
						t.Fatal(err)
					}
				} else if prior != "unrelated" {
					etsyForbiddenExit(t, err)
				} else {
					s2Must(t, err)
					if !strings.Contains(out, "currently marked forbidden") {
						t.Fatal(out)
					}
					if !strings.Contains(out, "run 'sync --full'") {
						t.Fatal(out)
					}
				}
			})
		}
	}
}

func TestBatchAScopeLossAfterMixedProgress(t *testing.T) {
	syncCLIIsolate(t)
	path := syncCLIDB(t, 0)
	var mode atomic.Int64
	mode.Store(1)
	srv := etsyForbiddenServer(t, 2, &mode)
	_, err := syncCLIRun(t, path)
	s2Must(t, err)
	// One previously successful receipt is forbidden while its peer still succeeds.
	mode.Store(3)
	before := len(etsyForbiddenPaths(srv))
	out, err := syncCLIRun(t, path, "--full")
	s2Must(t, err)
	etsyForbiddenWantPaths(t, etsyForbiddenPaths(srv)[before:], 1, 2)
	if !strings.Contains(out, "sync_summary") {
		t.Fatal(out)
	}
	db, e := store.Open(path)
	s2Must(t, e)
	defer db.Close()
	checks, e := db.ReadPaymentChecks()
	s2Must(t, e)
	if checks["3000000001"].EverOK || !checks["3000000001"].Forbidden {
		t.Fatal(checks)
	}
	s2Must(t, db.Close())
	for i := 0; i < 3; i++ {
		_, err := syncCLIRun(t, path)
		s2Must(t, err)
	}
}

func TestBatchAIncrementalForeignReceiptAdvice(t *testing.T) {
	syncCLIIsolate(t)
	path := syncCLIDB(t, 0)
	var mode atomic.Int64
	mode.Store(1)
	srv := etsyForbiddenServer(t, 4, &mode)
	_, err := syncCLIRun(t, path)
	s2Must(t, err)
	// The server adds one new receipt; four unchanged successes stay cached.
	before := len(etsyForbiddenPaths(srv))
	mode.Store(6)
	out, err := syncCLIRun(t, path)
	s2Must(t, err)
	etsyForbiddenWantPaths(t, etsyForbiddenPaths(srv)[before:], 5)
	etsyForbiddenWarning(t, out, 1)
	if !strings.Contains(out, "If this token recently lost the transactions_r scope") || !strings.Contains(out, "run 'sync --full'") {
		t.Fatal(out)
	}
}

// Exercise quiet runs using a pre-existing forbidden probe and a cached peer.
func TestBatchAAmbiguousVerification(t *testing.T) {
	for _, tc := range []struct {
		name   string
		n      int
		mode   int64
		strict bool
		code   int
		ever   bool
	}{
		{"peer ok", 2, 3, false, 0, false},
		{"peer forbidden", 2, 0, false, 1, true},
		{"single", 1, 0, false, 1, true},
		{"strict scope", 2, 0, true, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			syncCLIIsolate(t)
			path := syncCLIDB(t, 0)
			var mode atomic.Int64
			mode.Store(1)
			srv := etsyForbiddenServer(t, tc.n, &mode)
			_, err := syncCLIRun(t, path)
			s2Must(t, err)
			db, err := store.Open(path)
			s2Must(t, err)
			s2Must(t, db.RecordPaymentForbidden("3000000001", sql.NullInt64{}))
			s2Must(t, db.Close())
			mode.Store(tc.mode)
			before := len(etsyForbiddenPaths(srv))
			args := []string{}
			if tc.strict {
				args = append(args, "--strict")
			}
			out, err := syncCLIRun(t, path, args...)
			code := 0
			if err != nil {
				code = ExitCode(err)
			}
			if code != tc.code {
				t.Fatalf("%v: %s", err, out)
			}
			if tc.code == 1 && !strings.Contains(out, "re-authorize with the transactions_r scope") {
				t.Fatal(out)
			}
			if tc.strict && (err == nil || !strings.Contains(err.Error(), "--strict")) {
				t.Fatal(err)
			}
			if tc.n == 2 {
				etsyForbiddenWantPaths(t, etsyForbiddenPaths(srv)[before:], 1, 2)
			} else {
				etsyForbiddenWantPaths(t, etsyForbiddenPaths(srv)[before:], 1)
			}
			db, err = store.Open(path)
			s2Must(t, err)
			defer db.Close()
			checks, err := db.ReadPaymentChecks()
			s2Must(t, err)
			if checks["3000000001"].EverOK != tc.ever {
				t.Fatal(checks)
			}
			syncCLIRequireEvent(t, out, "payments_forbidden")
		})
	}
}

func TestBatchASummaryBeforeScopeError(t *testing.T) {
	syncCLIIsolate(t)
	path := syncCLIDB(t, 0)
	var mode atomic.Int64
	mode.Store(7)
	etsyForbiddenServer(t, 2, &mode)
	out, err := syncCLIRun(t, path, "--full")
	etsyForbiddenExit(t, err)
	// The event summary and the machine summary both contain total_records.
	if strings.Count(out, `"total_records"`) != 2 {
		t.Fatal(out)
	}
}

func TestBatchAOnlyForbiddenWarningExemption(t *testing.T) {
	for _, second := range []bool{false, true} {
		t.Run(fmt.Sprint(second), func(t *testing.T) {
			syncCLIIsolate(t)
			n := 2

			path := syncCLIDB(t, n)
			db, err := store.Open(path)
			s2Must(t, err)
			s2Must(t, db.RecordPaymentCheck("3000000002", sql.NullInt64{Int64: 1700000001, Valid: true}))
			s2Must(t, db.Close())
			var mode atomic.Int64
			mode.Store(0)
			if second {
				mode.Store(8)
			}
			etsyForbiddenServer(t, n, &mode)
			resource := "shops_receipts_payments"
			if second {
				resource = "receipts"
			}
			out, _, err := s2Run(t, "sync", "--resources", resource, "--db", path, "--rate-limit", "1000", "--json", "--max-pages", "1")
			if second {
				if err == nil || err.Error() != "1 resource(s) completed with warnings but no successful syncs" {
					t.Fatalf("%v: %s", err, out)
				}
			} else {
				s2Must(t, err)
			}
		})
	}
}

func TestBatchAGroupNumericTiesNullLast(t *testing.T) {
	syncCLIIsolate(t)
	path := syncCLIDB(t, 0)
	db, err := store.Open(path)
	s2Must(t, err)
	for i, raw := range []string{`{"n":10}`, `{"n":2}`, `{"n":null}`, `{"n":null}`} {
		s2Must(t, db.Upsert("listings", fmt.Sprint(i), json.RawMessage(raw)))
	}
	s2Must(t, db.Close())
	out, _, err := s2Run(t, "analytics", "--db", path, "--type", "listings", "--group-by", "n", "--json")
	s2Must(t, err)
	if strings.Join(strings.Fields(out), "") != `[{"value":"2","count":1},{"value":"10","count":1},{"value":null,"count":2}]` {
		t.Fatal(out)
	}
}
