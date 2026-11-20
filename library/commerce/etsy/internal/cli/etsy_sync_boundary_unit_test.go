package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/spf13/cobra"
)

// Exercise the threshold and each independent bypass.
func TestEtsyUnknownQuotaRefusalUnit(t *testing.T) {
	t.Setenv(cliutil.MCPChildEnvVar, "")
	for _, tc := range []struct {
		planned, cap          int
		ignore, known, refuse bool
	}{
		{1000, 0, false, false, false}, {1001, 0, false, false, true},
		{1001, 10, false, false, false}, {1001, 0, true, false, false}, {1001, 0, false, true, false},
	} {
		err := etsyUnknownQuotaRefusal(tc.planned, tc.cap, tc.ignore, tc.known)
		if !tc.refuse {
			if err != nil {
				t.Fatal(err)
			}
			continue
		}
		var ce *cliError
		if !errors.As(err, &ce) || ce.code != 7 || !strings.Contains(err.Error(), "--max-calls") || !strings.Contains(err.Error(), "--ignore-budget") {
			t.Fatalf("got %v", err)
		}
	}
}

func TestEtsyBudgetHintUnit(t *testing.T) {
	for _, child := range []string{"", "1"} {
		t.Setenv(cliutil.MCPChildEnvVar, child)
		base := "increase the cap or --ignore-budget"
		hint := etsyBudgetHint(base)
		if child == "" && hint != base {
			t.Fatal(hint)
		}
		if child == "1" && (strings.Contains(hint, "increase") || strings.Contains(hint, "--ignore-budget") || hint != "narrow --resources") {
			t.Fatal(hint)
		}
	}
}

// Synthetic parents model the population discovered by the flat phase.
func TestEtsySyncBoundaryCheckUnit(t *testing.T) {
	_, db := listingStatesFixture(t)
	restore, err := cliutil.SetHomeOverride("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)
	testenv.Isolate(t, cliutil.StateDir)
	cmd := &cobra.Command{}
	cmd.SetErr(&bytes.Buffer{})
	check := func(t *testing.T, cap int, used int64, ignore bool, wantCap int, want string) {
		t.Helper()
		planned, paymentCap, err := etsySyncBoundaryCheck(cmd, db, []string{"receipts"}, cap, used, ignore, etsySyncPlan{})
		if want == "" {
			if err != nil || paymentCap != wantCap {
				t.Fatalf("cap %d want %d: %v", paymentCap, wantCap, err)
			}
			return
		}
		var ce *cliError
		if planned != 300 || !errors.As(err, &ce) || ce.code != 7 || !strings.Contains(err.Error(), want) {
			t.Fatalf("planned %d: %v", planned, err)
		}
	}
	n, _, err := etsySyncBoundaryCheck(cmd, db, []string{"receipts"}, 0, 0, false, etsySyncPlan{})
	if n != 0 || err != nil {
		t.Fatalf("%d %v", n, err)
	}
	items := make([]json.RawMessage, 300)
	for i := range items {
		items[i] = json.RawMessage(fmt.Sprintf("{\"receipt_id\":%d}", 100000+i))
	}
	if _, _, _, err := db.UpsertBatchDetailed("receipts", items); err != nil {
		t.Fatal(err)
	}
	// etsy-ext: Payment fan-out now caps rather than refusing a positive allowance.
	check(t, 0, 0, true, -1, "")
	check(t, 10, 0, true, 10, "")
	check(t, 310, 11, true, 299, "")
	h := make(http.Header)
	h.Set("x-limit-per-day", "5000")
	h.Set("x-remaining-today", "100")
	for _, etsyCase := range []struct {
		name   string
		cap    int
		used   int64
		ignore bool
		want   string
	}{
		{"floor refusal below reserve", 0, 0, false, "daily reserve reached"},
		{"explicit cap below floor", 10, 0, false, "--max-calls"},
		{"cap exhausted", 5, 5, true, "--max-calls"},
	} {
		t.Run(etsyCase.name, func(t *testing.T) {
			client.ObserveEtsyQuota(h, etsyTestQuotaKey(t))
			check(t, etsyCase.cap, etsyCase.used, etsyCase.ignore, -1, etsyCase.want)
		})
	}
	t.Run("other dependents refuse", func(t *testing.T) {
		client.ObserveEtsyQuota(h, etsyTestQuotaKey(t))
		_, _, err := etsySyncBoundaryCheck(cmd, db, []string{"receipts", "receipts_listings"}, 10, 0, true, etsySyncPlan{})
		if err == nil || !strings.Contains(err.Error(), "--max-calls") {
			t.Fatalf("other dependents: %v", err)
		}
	})
	for _, tc := range []struct {
		name, remaining string
		filter          []string
		refuse          bool
	}{
		{"at reserve", "250", []string{"receipts"}, true},
		{"reserve plus other dependents", "251", []string{"receipts", "receipts_listings"}, true},
		{"affordable", "4000", []string{"receipts"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(cliutil.MCPChildEnvVar, "")
			h.Set("x-remaining-today", tc.remaining)
			client.ObserveEtsyQuota(h, etsyTestQuotaKey(t))
			_, paymentCap, err := etsySyncBoundaryCheck(cmd, db, tc.filter, 0, 0, false, etsySyncPlan{})
			if !tc.refuse {
				if err != nil || paymentCap <= 0 {
					t.Fatalf("cap %d: %v", paymentCap, err)
				}
				return
			}
			want := fmt.Sprintf("payments: daily reserve reached (%s left, floor 250); re-run when quota is available in the rolling 24h window, pass --ignore-budget, or narrow --resources to skip shops_receipts_payments", tc.remaining)
			var ce *cliError
			if !errors.As(err, &ce) || ce.code != 7 || err.Error() != want || paymentCap != -1 {
				t.Fatalf("cap %d, want %q: %v", paymentCap, want, err)
			}
		})
	}

}

// etsy-ext: Independently verify the two ceilings and quota override.
func TestEtsyPaymentsAllowanceUnit(t *testing.T) {
	syncCLIIsolate(t)
	cases := []struct {
		name     string
		quota    bool
		maxCalls int
		used     int64
		others   int
		ignore   bool
		want     int
		bounded  bool
	}{
		{"unbounded", false, 0, 0, 2, false, 0, false},
		{"maxCalls", false, 100, 11, 3, false, 86, true},
		// etsy-ext: Quota ceilings retain the 250-call daily reserve floor.
		{"quota", true, 0, 0, 3, false, -153, true},
		// etsy-ext: The daily reserve floor is tighter than this explicit cap.
		{"bothMax", true, 50, 10, 3, false, -153, true},
		// etsy-ext: The quota ceiling subtracts the daily reserve floor.
		{"bothQuota", true, 200, 10, 3, false, -153, true},
		{"ignoreBudget", true, 0, 0, 3, true, 0, false},
		{"ignoreWithMax", true, 50, 10, 3, true, 37, true},
		// etsy-ext: The daily reserve floor also bounds an exhausted explicit cap.
		{"exhausted", true, 5, 5, 3, false, -153, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.quota {
				client.ObserveEtsyQuota(http.Header{"X-Limit-Per-Day": {"5000"}, "X-Remaining-Today": {"100"}}, etsyTestQuotaKey(t))
			}
			got, bounded := etsyPaymentsAllowance(tc.maxCalls, tc.used, tc.others, tc.ignore, etsyQuotaKey(nil))
			if got != tc.want || bounded != tc.bounded {
				t.Fatalf("got %d %v want %d %v", got, bounded, tc.want, tc.bounded)
			}
		})
	}
}

// etsy-ext: Capped payment-only walks must still emit the low-balance budget notice.
func TestEtsySyncBoundaryCappedRunsBudgetGate(t *testing.T) {
	syncCLIIsolate(t)
	_, db := listingStatesFixture(t)
	_, _, _, err := db.UpsertBatchDetailed("receipts", syncCLIReceipts(80, 0))
	s2Must(t, err)
	client.ObserveEtsyQuota(http.Header{"X-Limit-Per-Day": {"5000"}, "X-Remaining-Today": {"300"}}, etsyTestQuotaKey(t))
	var etsyStderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&etsyStderr)
	_, etsyCap, err := etsySyncBoundaryCheck(cmd, db, []string{"shops_receipts_payments"}, 0, 0, false, etsySyncPlan{})
	if err != nil || etsyCap != 50 {
		t.Fatalf("cap %d: %v", etsyCap, err)
	}
	if !strings.Contains(etsyStderr.String(), "note: 300 of 5000") {
		t.Fatalf("missing budget notice: %s", etsyStderr.String())
	}
}
