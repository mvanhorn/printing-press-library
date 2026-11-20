package cli

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func etsyRound6Mixed(t *testing.T, etsyN int, etsyBad map[string]bool, etsyMaxCalls int) {
	t.Helper()
	dEnv(t)
	etsyServer := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
		for _, etsyID := range strings.Split(r.URL.Query().Get("listing_ids"), ",") {
			if etsyBad[etsyID] {
				w.WriteHeader(http.StatusNotFound)
				return true
			}
		}
		dBatch(w, r, 2)
		return true
	})
	etsyPath := etsyRefreshFixture(t, etsyN)
	etsyOut, etsyStderr, etsyErr := s2Run(t, "inventory", "low-stock", "--refresh", "--db", etsyPath, "--json")
	s2Must(t, etsyErr)
	etsyFresh := 0
	for _, etsyRow := range etsyLowStockResults(t, etsyOut) {
		if etsyRow.Quantity == 2 {
			etsyFresh++
		}
		if etsyN == 4 && etsyRow.ListingID == 1000000003 && etsyRow.Quantity != 2 {
			t.Fatal("good listing was not refreshed")
		}
	}
	if etsyFresh != etsyN-len(etsyBad) {
		t.Fatalf("fresh=%d want=%d", etsyFresh, etsyN-len(etsyBad))
	}
	// Read again without refresh to verify the fresh rows were persisted.
	etsyOut, _, etsyErr = s2Run(t, "inventory", "low-stock", "--db", etsyPath, "--json")
	s2Must(t, etsyErr)
	etsyFresh = 0
	for _, etsyRow := range etsyLowStockResults(t, etsyOut) {
		if etsyRow.Quantity == 2 {
			etsyFresh++
		}
	}
	if etsyFresh != etsyN-len(etsyBad) {
		t.Fatalf("stored fresh=%d", etsyFresh)
	}
	if etsyN == 4 && !strings.Contains(etsyStderr, "3 of 4 inventory reads failed") {
		t.Fatal(etsyStderr)
	}
	if len(etsyServer.all()) > etsyMaxCalls {
		t.Fatalf("calls=%d max=%d", len(etsyServer.all()), etsyMaxCalls)
	}
}

func TestEtsyLowStockMixedHalves(t *testing.T) {
	etsyRound6Mixed(t, 4, map[string]bool{"1000000001": true, "1000000002": true, "1000000004": true}, 12)
}

func TestEtsyLowStockMixedHalvesLarge(t *testing.T) {
	etsyRound6Mixed(t, 100, map[string]bool{"1000000010": true, "1000000011": true, "1000000012": true, "1000000090": true}, 60)
}

func TestEtsyLowStockTimeoutFlagDoesNotBoundRun(t *testing.T) {
	dEnv(t)
	t.Setenv("ETSY_MCP_CHILD", "")
	etsyServer := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
		time.Sleep(40 * time.Millisecond)
		dBatch(w, r, 2)
		return true
	})
	_, etsyStderr, etsyErr := s2Run(t, "inventory", "low-stock", "--refresh", "--db", etsyRefreshFixture(t, 350), "--json", "--timeout", "100ms")
	s2Must(t, etsyErr)
	if len(etsyServer.all()) < 4 || strings.Contains(etsyStderr, "interrupted") {
		t.Fatalf("calls=%d stderr=%s", len(etsyServer.all()), etsyStderr)
	}
}

func TestEtsyLowStockMCPCeiling(t *testing.T) {
	for _, etsyMCP := range []bool{true, false} {
		etsyName := "parent deadline"
		if etsyMCP {
			etsyName = "MCP ceiling"
		}
		t.Run(etsyName, func(t *testing.T) {
			dEnv(t)
			t.Setenv("ETSY_MCP_CHILD", "")
			if etsyMCP {
				t.Setenv("ETSY_MCP_CHILD", "1")
			}
			etsyOld := etsyLowStockMCPCeiling
			etsyLowStockMCPCeiling = 100 * time.Millisecond
			t.Cleanup(func() { etsyLowStockMCPCeiling = etsyOld })
			s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
				time.Sleep(60 * time.Millisecond)
				dBatch(w, r, 2)
				return true
			})
			etsyPath := etsyRefreshFixture(t, 350)
			var etsyFlags rootFlags
			etsyRoot := newRootCmd(&etsyFlags)
			var etsyOut bytes.Buffer
			etsyRoot.SetOut(&etsyOut)
			etsyRoot.SetErr(&etsyOut)
			etsyRoot.SetArgs([]string{"inventory", "low-stock", "--refresh", "--db", etsyPath, "--json"})
			etsyCtx := context.Background()
			if !etsyMCP {
				var etsyCancel context.CancelFunc
				etsyCtx, etsyCancel = context.WithTimeout(etsyCtx, 80*time.Millisecond)
				defer etsyCancel()
			}
			etsyErr := etsyRoot.ExecuteContext(etsyCtx)
			if etsyErr == nil {
				t.Fatal("expected interruption")
			}
			etsyMessage := etsyErr.Error()
			if !strings.Contains(etsyMessage, "interrupted") || strings.Contains(etsyMessage, "--timeout") || strings.Contains(etsyMessage, "MCP run ceiling") != etsyMCP {
				t.Fatal(etsyMessage)
			}
			if !etsyMCP && (!strings.Contains(etsyMessage, "calling context") || !strings.Contains(etsyMessage, "each API call")) {
				t.Fatal(etsyMessage)
			}
		})
	}
}
