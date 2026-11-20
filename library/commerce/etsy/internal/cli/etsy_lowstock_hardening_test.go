package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

func TestEtsyLowStockRefreshPriceOnProperty(t *testing.T) {
	dEnv(t)
	s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
		fmt.Fprint(w, `{"results":[{"listing_id":1000000001,"inventory":{"products":[{"product_id":7,"offerings":[{"offering_id":71,"price":{"amount":2500,"divisor":100,"currency_code":"USD"},"quantity":2,"is_enabled":true}]},{"product_id":8,"offerings":[{"offering_id":81,"price":{"amount":2500,"divisor":100,"currency_code":"USD"},"quantity":3,"is_enabled":true}]}],"price_on_property":[513],"quantity_on_property":[513],"sku_on_property":[513],"readiness_state_on_property":[513]}}]}`)
		return true
	})
	etsyPath := etsyRefreshFixture(t, 1)
	_, _, etsyErr := s2Run(t, "inventory", "low-stock", "--refresh", "--db", etsyPath, "--json")
	s2Must(t, etsyErr)
	etsyDB, etsyErr := store.OpenReadOnly(etsyPath)
	s2Must(t, etsyErr)
	defer etsyDB.Close()
	etsyRaw := s2StoredInventory(etsyDB, 1000000001)
	var etsyCurrent bulkCurrent
	bulkInventoryCurrent(&etsyCurrent, []byte(etsyRaw))
	if !etsyCurrent.VariationPrices || len(etsyCurrent.Offerings) != 2 {
		t.Fatalf("PROBE_LOWSTOCK_PRICE_ON_PROPERTY_DROPPED: bulk preview inventory=%+v", etsyCurrent)
	}
	for _, etsyKey := range []string{"price_on_property", "quantity_on_property", "sku_on_property", "readiness_state_on_property"} {
		if !strings.Contains(etsyRaw, etsyKey) {
			t.Fatalf("PROBE_LOWSTOCK_PRICE_ON_PROPERTY_DROPPED: missing %s", etsyKey)
		}
	}
}

func TestEtsyLowStockSplitIsHalving(t *testing.T) {
	dEnv(t)
	etsyServer := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
		if strings.Contains(r.URL.Query().Get("listing_ids"), "1000000037") {
			w.WriteHeader(404)
		} else {
			dBatch(w, r, 2)
		}
		return true
	})
	_, _, etsyErr := s2Run(t, "inventory", "low-stock", "--refresh", "--db", etsyRefreshFixture(t, 100), "--json")
	if etsyErr != nil || len(etsyServer.all()) > 15 {
		t.Fatalf("PROBE_LOWSTOCK_LINEAR_SPLIT: calls=%d err=%v", len(etsyServer.all()), etsyErr)
	}
}

func TestEtsyLowStockMidRunCancel(t *testing.T) {
	dEnv(t)
	etsyCtx, etsyCancel := context.WithCancel(context.Background())
	defer etsyCancel()
	etsyServer := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
		etsyCancel()
		dBatch(w, r, 2)
		return true
	})
	var etsyFlags rootFlags
	etsyRoot := newRootCmd(&etsyFlags)
	var etsyOut bytes.Buffer
	etsyRoot.SetOut(&etsyOut)
	etsyRoot.SetErr(&etsyOut)
	etsyRoot.SetArgs([]string{"inventory", "low-stock", "--refresh", "--db", etsyRefreshFixture(t, 250), "--json"})
	etsyErr := etsyRoot.ExecuteContext(etsyCtx)
	if etsyErr == nil || !strings.Contains(etsyErr.Error(), "interrupted") || strings.Contains(etsyOut.String(), "inventory reads failed") || len(etsyServer.all()) != 1 {
		t.Fatalf("PROBE_LOWSTOCK_CANCEL_IGNORED: err=%v output=%s", etsyErr, etsyOut.String())
	}
}

func TestEtsyLowStockMCPCallCap(t *testing.T) {
	for _, etsyMCP := range []bool{true, false} {
		t.Run(fmt.Sprint(etsyMCP), func(t *testing.T) {
			dEnv(t)
			if etsyMCP {
				t.Setenv("ETSY_MCP_CHILD", "1")
			} else {
				t.Setenv("ETSY_MCP_CHILD", "")
			}
			etsyOld := etsyLowStockMCPMaxCalls
			etsyLowStockMCPMaxCalls = 5
			t.Cleanup(func() { etsyLowStockMCPMaxCalls = etsyOld })
			etsyServer := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool { w.WriteHeader(404); return true })
			_, _, etsyErr := s2Run(t, "inventory", "low-stock", "--refresh", "--db", etsyRefreshFixture(t, 100), "--json")
			if etsyMCP {
				if etsyErr == nil || !strings.Contains(etsyErr.Error(), "MCP call cap") || len(etsyServer.all()) > 5 {
					t.Fatalf("PROBE_LOWSTOCK_MCP_CAP_MISSING: err=%v calls=%d", etsyErr, len(etsyServer.all()))
				}
			} else if etsyErr != nil || len(etsyServer.all()) <= 5 {
				t.Fatalf("PROBE_LOWSTOCK_MCP_CAP_MISSING: outside MCP err=%v calls=%d", etsyErr, len(etsyServer.all()))
			}
		})
	}
}

func TestEtsyLowStockBothHalvesFail(t *testing.T) {
	dEnv(t)
	etsyServer := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
		w.WriteHeader(404)
		return true
	})
	_, etsyStderr, etsyErr := s2Run(t, "inventory", "low-stock", "--refresh", "--db", etsyRefreshFixture(t, 100), "--json")
	if etsyErr != nil || !strings.Contains(etsyStderr, "100 of 100 inventory reads failed") || len(etsyServer.all()) > 110 {
		t.Fatalf("PROBE_LOWSTOCK_SPLIT_UNBOUNDED: calls=%d err=%v stderr=%s", len(etsyServer.all()), etsyErr, etsyStderr)
	}
}
