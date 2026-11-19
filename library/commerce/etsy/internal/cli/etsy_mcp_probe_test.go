package cli

import (
	"database/sql"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

// The probes below are plain (no subtests) single-purpose tests, one per
// load-bearing guard, each failing with a unique PROBE_ token. They exist so
// mutation-verify.sh can deselect exactly one test per hazard.

func TestEtsyProbeOrdersFullRefused(t *testing.T) {
	t.Setenv(cliutil.MCPChildEnvVar, "1")
	t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", "")
	s2SetNow(t, s2Epoch)
	out, _, err := s2Run(t, "orders", "unshipped", "--full", "--db", s2ReceiptFixture(t), "--json")
	if err == nil || ExitCode(err) != 2 || strings.Contains(out, "Jordan Example") {
		t.Fatalf("PROBE_ORDERS_FULL_LEAK: err=%v leaked=%v", err, strings.Contains(out, "Jordan Example"))
	}
}

func TestEtsyProbeLowStockEnvelopeWithoutNotes(t *testing.T) {
	dEnv(t)
	t.Setenv("ETSY_SHOP_ID", "9000001")
	out, _, err := s2Run(t, "inventory", "low-stock", "--db", etsyRefreshFixture(t, 1), "--json")
	if err != nil || !strings.HasPrefix(strings.TrimSpace(out), "{") || !strings.Contains(out, `"notes": []`) {
		t.Fatalf("PROBE_ENVELOPE_BARE_ARRAY: err=%v out=%s", err, out)
	}
}

func TestEtsyProbeLowStockAtomicReplace(t *testing.T) {
	dEnv(t)
	path := etsyRefreshFixture(t, 1)
	raw, err := sql.Open("sqlite", path)
	s2Must(t, err)
	_, err = raw.Exec(`CREATE TRIGGER etsy_fail_inv BEFORE INSERT ON resources WHEN NEW.resource_type='listings_inventory' BEGIN SELECT RAISE(ABORT,'boom'); END;`)
	s2Must(t, err)
	s2Must(t, raw.Close())
	s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool { dBatch(w, r, 2); return true })
	_, _, _ = s2Run(t, "inventory", "low-stock", "--refresh", "--db", path, "--json")
	db, err := store.OpenReadOnly(path)
	s2Must(t, err)
	defer db.Close()
	rows, err := s2LowStockRows(db, 3)
	s2Must(t, err)
	if len(rows) != 1 || rows[0].Quantity != 1 {
		t.Fatalf("PROBE_ATOMIC_OLD_ROWS_LOST: rows=%+v", rows)
	}
}

func TestEtsyProbeLowStockMCPIgnoreBudgetRefused(t *testing.T) {
	dEnv(t)
	t.Setenv(cliutil.MCPChildEnvVar, "1")
	_, _, err := s2Run(t, "inventory", "low-stock", "--refresh", "--ignore-budget", "--dry-run")
	if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "--ignore-budget is not available over MCP") {
		t.Fatalf("PROBE_MCP_IGNORE_BUDGET_ALLOWED: %v", err)
	}
}
