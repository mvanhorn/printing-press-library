package cli

import (
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// etsySummaryObjects reads stdout only; stderr's sync_summary is a separate stream.
func etsySummaryObjects(t *testing.T, etsyOut string) []map[string]any {
	t.Helper()
	var etsyObjects []map[string]any
	etsyDecoder := json.NewDecoder(strings.NewReader(etsyOut))
	for {
		var etsyObject map[string]any
		etsyErr := etsyDecoder.Decode(&etsyObject)
		if etsyErr == io.EOF {
			break
		}
		if etsyErr != nil {
			t.Fatalf("invalid stdout JSON: %v: %s", etsyErr, etsyOut)
		}
		if etsyObject["total_records"] != nil {
			etsyObjects = append(etsyObjects, etsyObject)
		}
	}

	return etsyObjects
}

func etsySummaryRun(t *testing.T, etsyTotal int, etsyQuota string, etsyFail int, etsyCap string, etsyWant int, etsyHint string) {
	t.Helper()
	syncCLIIsolate(t)
	etsyPath := syncCLIDB(t, 0)
	syncCLIServer(t, etsyTotal, etsyQuota, etsyFail)
	etsyArgs := []string{"sync", "--resources", "receipts", "--db", etsyPath, "--rate-limit", "1000", "--json"}
	if etsyCap != "" {
		etsyArgs = append(etsyArgs, "--max-calls", etsyCap)
	}
	etsyOut, _, etsyErr := s2Run(t, etsyArgs...)
	if etsyHint == "" {
		s2Must(t, etsyErr)
	} else {
		syncCLIRequireQuotaExit(t, etsyErr)
		if !strings.Contains(etsyErr.Error(), etsyHint) {
			t.Fatal(etsyErr)
		}
	}
	etsyObjects := etsySummaryObjects(t, etsyOut)
	if len(etsyObjects) != 1 {
		t.Fatalf("summaries=%d stdout=%s", len(etsyObjects), etsyOut)
	}
	etsyObject := etsyObjects[0]
	t.Logf("summary: %+v", etsyObject)
	if etsyObject["total_records"] != float64(etsyWant) {
		t.Fatalf("total=%v want %d", etsyObject["total_records"], etsyWant)
	}
	etsyDB, etsyErr := store.Open(etsyPath)
	s2Must(t, etsyErr)
	defer etsyDB.Close()
	etsyReceipts, etsyErr := etsyDB.Count("receipts")
	s2Must(t, etsyErr)
	etsyPayments, etsyErr := etsyDB.Count("shops_receipts_payments")
	s2Must(t, etsyErr)
	etsyStoredWant := etsyWant
	if etsyFail == 2 {
		etsyStoredWant = 100
	} else if etsyCap == "3" {
		etsyStoredWant = 300
	}
	if etsyReceipts+etsyPayments != etsyStoredWant {
		t.Fatalf("stored receipts=%d payments=%d total=%d", etsyReceipts, etsyPayments, etsyWant)
	}
	if etsyTotal == 300 && etsyReceipts != 300 {
		t.Fatalf("receipts=%d", etsyReceipts)
	}
}

func TestEtsySyncSummaryBeforeMaxCallsStop(t *testing.T) {
	etsySummaryRun(t, 500, "4000", 0, "3", 0, "re-run sync to continue (a re-run restarts capped listings walks")
}
func TestEtsySyncSummaryBeforeDependentMaxCallsStop(t *testing.T) {
	etsySummaryRun(t, 300, "4000", 0, "100", 396, "re-run to continue")
}
func TestEtsySyncSummaryBeforeDailyQuotaStop(t *testing.T) {
	etsySummaryRun(t, 500, "4000", 2, "", 0, "daily request quota exhausted")
}
func TestEtsySyncSummaryBeforeBoundaryRefusal(t *testing.T) {
	etsySummaryRun(t, 300, "100", 0, "", 300, "daily reserve reached (100 left, floor 250); re-run when quota is available in the rolling 24h window, pass --ignore-budget, or narrow --resources to skip shops_receipts_payments")
}
func TestEtsySyncSummaryBeforeCappedPayments(t *testing.T) {
	etsySummaryRun(t, 300, "400", 0, "", 450, "re-run to continue")
}
func TestEtsySyncSummaryExactlyOnceOnSuccess(t *testing.T) {
	etsySummaryRun(t, 3, "4000", 0, "", 6, "")
}

func TestEtsySyncSummaryNotPrintedForPlainFailure(t *testing.T) {
	syncCLIIsolate(t)
	etsyPath := syncCLIDB(t, 0)
	s2NewServer(t, func(etsyW http.ResponseWriter, etsyR *http.Request, _ string) bool {
		syncCLIQuota(etsyW, "4000")
		if strings.HasSuffix(etsyR.URL.Path, "/payments") {
			etsyW.WriteHeader(http.StatusForbidden)
			_, _ = etsyW.Write([]byte(`{"error":"synthetic forbidden"}`))
		} else {
			_ = json.NewEncoder(etsyW).Encode(map[string]any{"results": syncCLIReceipts(5, 0), "count": 5})
		}
		return true
	})
	etsyOut, _, etsyErr := s2Run(t, "sync", "--resources", "receipts", "--db", etsyPath, "--rate-limit", "1000", "--json")
	if etsyErr == nil || ExitCode(etsyErr) != 1 {
		t.Fatalf("want exit 1: %v", etsyErr)
	}
	// Forbidden is a plain failure and returns before the machine summary.
	if etsyN := len(etsySummaryObjects(t, etsyOut)); etsyN != 0 {
		t.Fatalf("summaries=%d: %s", etsyN, etsyOut)
	}
}
func TestEtsySyncSummaryBeforeMaxCallsDependentHint(t *testing.T) {
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
	if !strings.Contains(etsyErr.Error(), "re-run sync to continue") || !strings.Contains(etsyErr.Error(), "resumes where this stopped") || strings.Contains(etsyAll, "restarts capped listings") {
		t.Fatalf("wrong dependent hint: %v %s", etsyErr, etsyAll)
	}
	if etsyObjects := etsySummaryObjects(t, etsyStdout); len(etsyObjects) != 1 {
		t.Fatalf("summaries=%d: %s", len(etsyObjects), etsyStdout)
	}
}
