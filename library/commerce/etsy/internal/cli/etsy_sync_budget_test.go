package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/spf13/cobra"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/platform"
)

// etsy-ext: Synthetic requests verify both terminal errors and exact reservations.
type syncBudgetFake struct {
	calls   atomic.Int64
	quotaAt int64
}

// etsy-ext: Supply the shop context required by dependent paths.
func (f *syncBudgetFake) CurrentTemplateVars() map[string]string {
	return map[string]string{"shop_id": "9000001"}
}
func (f *syncBudgetFake) RateLimit() float64 { return 100000 }
func (f *syncBudgetFake) Get(context.Context, string, map[string]string) (json.RawMessage, error) {
	n := f.calls.Add(1)
	if n == f.quotaAt {
		return nil, &platform.RateLimitedError{Cause: &client.DailyQuotaExhaustedError{}}
	}
	return json.RawMessage(`{"results":[],"count":0}`), nil
}
func TestEtsySyncGuardStops(t *testing.T) {
	for _, quota := range []bool{false, true} {
		f := &syncBudgetFake{}
		g := &etsySyncGuard{inner: f, maxCalls: 5}
		if quota {
			f.quotaAt = 3
		}
		for i := 0; i < 100; i++ {
			_, _ = g.Get(context.Background(), "/fake", nil)
		}
		want := int64(5)
		if quota {
			want = 3
		}
		if f.calls.Load() != want || g.Calls() != want || g.SyncStopped() == nil {
			t.Fatalf("calls %d stop %v", f.calls.Load(), g.SyncStopped())
		}
		if quota && !isDailyQuotaErr(g.SyncStopped()) {
			t.Fatal(g.SyncStopped())
		}
	}
}
func TestEtsySyncEstimateListings(t *testing.T) {
	_, db := listingStatesFixture(t)
	if n := estimateSyncCalls(db, []string{"listings"}, nil, 0, false); n != 5 {
		t.Fatalf("estimate %d", n)
	}
}

// etsy-ext: Estimate includes only the default receipt payment dependent and budget overrides never disable caps.
func TestEtsySyncPreflightBudget(t *testing.T) {
	_, db := listingStatesFixture(t)
	restore, err := cliutil.SetHomeOverride("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)
	testenv.Isolate(t, cliutil.StateDir)
	for i := 0; i < 1000; i++ {
		if _, _, err := db.UpsertBatch("receipts", []json.RawMessage{json.RawMessage(fmt.Sprintf(`{"receipt_id":%d}`, 100000+i))}); err != nil {
			t.Fatal(err)
		}
	}
	h := make(http.Header)
	h.Set("x-limit-per-day", "5000")
	h.Set("x-remaining-today", "100")
	client.ObserveEtsyQuota(h, etsyTestQuotaKey(t))
	cmd := &cobra.Command{}
	cmd.SetErr(&bytes.Buffer{})
	planned, err := etsySyncPreflight(cmd, db, []string{"receipts"}, []string{"receipts"}, 0, false, 0, false, etsySyncPlan{})
	var ce *cliError
	// etsy-ext: A nonpositive allowance restores the full estimate, so the exceeds check wins.
	if planned < 1000 || !errors.As(err, &ce) || ce.code != 7 || !strings.Contains(err.Error(), "exceeds 100 remaining") {
		t.Fatalf("planned %d error %v", planned, err)
	}
	if _, err = etsySyncPreflight(cmd, db, nil, []string{"receipts"}, 0, false, 0, true, etsySyncPlan{}); err != nil {
		t.Fatal(err)
	}
	// etsy-ext: Positive payment caps pass; explicit other dependents still refuse.
	if _, err = etsySyncPreflight(cmd, db, nil, []string{"receipts"}, 0, false, 10, true, etsySyncPlan{}); err != nil {
		t.Fatal(err)
	}
	if _, err = etsySyncPreflight(cmd, db, nil, []string{"receipts", "receipts_listings"}, 0, false, 10, true, etsySyncPlan{}); err == nil || !strings.Contains(err.Error(), "--max-calls") {
		t.Fatal(err)
	}
	h.Set("x-remaining-today", "5000")
	client.ObserveEtsyQuota(h, etsyTestQuotaKey(t))
	if _, err = etsySyncPreflight(cmd, db, []string{"receipts"}, []string{"receipts"}, 0, false, 0, false, etsySyncPlan{}); err != nil {
		t.Fatal(err)
	}
}

// etsy-ext: A stopped dependent drains its parent jobs without per-parent failures in events.
func TestEtsySyncDependentQuotaStop(t *testing.T) {
	_, db := listingStatesFixture(t)
	for i := 0; i < 1000; i++ {
		if _, _, err := db.UpsertBatch("receipts", []json.RawMessage{json.RawMessage(fmt.Sprintf(`{"receipt_id":%d}`, 100000+i))}); err != nil {
			t.Fatal(err)
		}
	}
	f := &syncBudgetFake{quotaAt: 3}
	g := &etsySyncGuard{inner: f}
	var events bytes.Buffer
	results := syncDependentResources(context.Background(), g, db, "", true, 0, false, false, []string{"receipts"}, nil, &events, 4)
	if f.calls.Load() > 6 || !isDailyQuotaErr(g.SyncStopped()) || len(results) != 1 || strings.Contains(events.String(), `"event":"sync_error"`) {
		t.Fatalf("calls %d results %d stop %v events %s", f.calls.Load(), len(results), g.SyncStopped(), events.String())
	}
}

// etsy-ext: Unknown quota at preflight bounds only flat calls, not stored receipt fan-out.
func TestEtsySyncPreflightUnknownQuotaFlatOnly(t *testing.T) {
	syncCLIIsolate(t)
	_, db := listingStatesFixture(t)
	_, _, _, err := db.UpsertBatchDetailed("receipts", syncCLIReceipts(1100, 0))
	s2Must(t, err)
	cmd := &cobra.Command{}
	cmd.SetErr(&bytes.Buffer{})
	planned, err := etsySyncPreflight(cmd, db, []string{"receipts"}, []string{"receipts"}, 0, false, 0, false, etsySyncPlan{})
	if err != nil || planned < 1100 {
		t.Fatalf("planned %d: %v", planned, err)
	}
}

// etsy-ext: Trimming must limit requests and persist exactly the successful checks.
func TestEtsyPaymentsCapTrimsDependent(t *testing.T) {
	_, db := listingStatesFixture(t)
	_, _, _, err := db.UpsertBatchDetailed("receipts", syncCLIReceipts(5, 0))
	s2Must(t, err)
	cs := &etsyPaymentsCap{allow: 2}
	f := &syncBudgetFake{}
	var events bytes.Buffer
	results := syncDependentResources(etsyWithPaymentsCap(context.Background(), cs), f, db, "", false, 0, false, false, []string{"receipts"}, nil, &events, 1)
	if len(results) != 1 || results[0].Warn == nil || !strings.Contains(results[0].Warn.Error(), "payments_capped") {
		t.Fatalf("missing capped warning: %+v", results)
	}

	checks, err := db.ReadPaymentChecks()
	s2Must(t, err)
	if len(results) != 1 || results[0].Err != nil || f.calls.Load() != 2 || len(checks) != 2 || !cs.capped || cs.total != 5 || cs.checked.Load() != 2 {
		t.Fatalf("results %+v calls %d checks %d cap %+v", results, f.calls.Load(), len(checks), cs)
	}
}

// etsy-ext: Payment allowances preserve the balance accepted by evaluateBudget.
func TestEtsyPaymentsAllowanceReserveFloor(t *testing.T) {
	for _, etsyCase := range []struct {
		name      string
		limit     string
		remaining string
		ignore    bool
		want      int
		bounded   bool
	}{
		{"belowFloor", "5000", "100", false, -150, true},
		{"aboveFloor", "5000", "400", false, 150, true},
		{"unknownLimit", "", "101", false, 95, true},
		{"ignoreBudget", "5000", "100", true, 0, false},
	} {
		t.Run(etsyCase.name, func(t *testing.T) {
			syncCLIIsolate(t)
			etsyHeaders := http.Header{"X-Remaining-Today": {etsyCase.remaining}}
			if etsyCase.limit != "" {
				etsyHeaders.Set("X-Limit-Per-Day", etsyCase.limit)
			}
			client.ObserveEtsyQuota(etsyHeaders, etsyTestQuotaKey(t))
			etsyAllow, etsyBounded := etsyPaymentsAllowance(0, 0, 0, etsyCase.ignore, etsyQuotaKey(nil))
			if etsyAllow != etsyCase.want || etsyBounded != etsyCase.bounded {
				t.Fatalf("allow %d bounded %v; want %d %v", etsyAllow, etsyBounded, etsyCase.want, etsyCase.bounded)
			}
		})
	}
}

func TestEtsyPaymentsCapExit(t *testing.T) {
	for _, etsyCase := range []struct {
		name      string
		checked   int64
		forbidden int64
		failures  []string
		want      string
		wantExit7 bool
	}{
		{"progress", 2, 0, nil, "progress is saved", true},
		{"zero", 0, 0, nil, "no progress was made", true},
		{"failure", 2, 0, []string{"shops_receipts_payments"}, "shops_receipts_payments", false},
		{"zero failure", 0, 0, []string{"shops_receipts_payments"}, "shops_receipts_payments", false},
		{"zero unrelated failure", 0, 0, []string{"shop_sections"}, "no progress was made", true},
		{"zero forbidden", 0, 3, nil, "no progress was made", true},
		{"progress and forbidden", 2, 3, nil, "progress is saved", true},
		{"zero forbidden with payments failure", 0, 3, []string{"shops_receipts_payments"}, "shops_receipts_payments", false},
		{"zero forbidden with unrelated failure", 0, 3, []string{"shop_sections"}, "no progress was made", true},
	} {
		t.Run(etsyCase.name, func(t *testing.T) {
			etsyErr := etsyPaymentsCapExit(false, etsyCase.checked, 300, etsyCase.failures)
			etsyWantForbidden := false
			if errors.Is(etsyErr, errEtsyPaymentsForbidden) != etsyWantForbidden {
				t.Fatal(etsyErr)
			}
			if etsyCase.wantExit7 {
				syncCLIRequireQuotaExit(t, etsyErr)
			} else if ExitCode(etsyErr) != 1 || !etsyPaymentsCapFailed(etsyErr) {
				t.Fatalf("want plain failure, got %v", etsyErr)
			}
			if !strings.Contains(etsyErr.Error(), etsyCase.want) || (etsyCase.name != "progress" && etsyCase.name != "progress and forbidden" && strings.Contains(etsyErr.Error(), "progress is saved")) {
				t.Fatal(etsyErr)
			}
		})
	}
}
