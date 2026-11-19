package mcp

import (
	"context"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// derivedGuardedTables are guarded by a kind-filtered TEMP VIEW.
var derivedGuardedTables = map[string]bool{"entity_lookups": true}

// learnTables hold taught buyer text or recall query text. The classification test exempts the
// search_/learn prefixes from the typed-table rule, so these must be proven guarded here;
// learn_events holds only hashes and ids and is deliberately unguarded.
func TestEtsyBuyerSQLGuardCoversLearnTables(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	benign := map[string]bool{"learn_events": true}
	guarded := map[string]bool{"search_learnings": true, "learn_candidates": true, "learning_playbooks": true, "search_patterns": true, "learn_recall_misses": true}
	rows, err := db.DB().Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(name, "_fts") || !(strings.HasPrefix(name, "learn") || strings.HasPrefix(name, "search_")) {
			continue
		}
		if benign[name] {
			continue
		}
		if !guarded[name] {
			t.Errorf("unclassified learn/search table %s: guard it in buyerSQLBypass or list it as benign", name)
		}
	}
	for name := range guarded {
		if !buyerSQLBypass.MatchString("SELECT * FROM " + name) {
			t.Errorf("learn table unguarded: %s", name)
		}
	}
}

func TestEtsyBuyerAllowlistClassification(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Lazy tables only exist after first use; create them so they must be classified.
	if err := store.EnsurePaymentChecksSchema(db.DB()); err != nil {
		t.Fatal(err)
	}
	// etsy-ext: exercise production lazy DDL rather than duplicating schemas.
	for _, o := range []store.EtsyPruneOptions{{Receipts: true, Apply: true, Cutoff: 200}, {Receipts: true, Apply: true, Erase: &store.EtsyEraseOptions{BuyerIDs: []string{"1000000002"}}}} {
		if _, err := db.EtsyPrune(o); err != nil {
			t.Fatal(err)
		}
	}

	union := map[string]bool{}
	for _, name := range cli.EtsyWritableResourceTypes() {
		union[name] = true
	}
	typed := map[string]bool{}
	rows, err := db.DB().Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		// etsy_meta contains migration markers only, never resource or buyer rows.
		excluded := name == "etsy_meta" || strings.Contains(name, "_fts")
		for _, prefix := range []string{"sqlite_", "sync_state", "search_", "learn", "market_", "resources"} {
			excluded = excluded || strings.HasPrefix(name, prefix)
		}
		if !excluded {
			union[name] = true
			typed[name] = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	classified := map[string]int{}
	for i, list := range [][]string{etsyNonBuyerResourceTypes, etsyBuyerResourceTypes} {
		if !sort.StringsAreSorted(list) {
			t.Error("classification must be sorted")
		}
		for _, name := range list {
			classified[name]++
			if classified[name] != 1 {
				t.Errorf("duplicate or overlapping classification: %s", name)
			}
			if !union[name] {
				t.Errorf("rotting classification: %s", name)
			}
			if i == 1 && typed[name] && !buyerSQLBypass.MatchString("SELECT * FROM "+name) {
				t.Errorf("typed buyer table unguarded: %s", name)
			}
		}
	}
	for name := range union {
		if name == "etsy_erasure_suppression" || name == "etsy_retention_receipts" || name == "etsy_retention_floor" || name == "etsy_erasure_receipts" || name == "etsy_erasure_transactions" || name == "etsy_erasure_buyer_receipts" {
			if !buyerSQLBypass.MatchString("SELECT * FROM " + name) {
				t.Errorf("unguarded privacy table %s", name)
			}
			continue
		}
		if classified[name] != 1 && !derivedGuardedTables[name] {
			t.Errorf("classify %s in etsy_buyer_pii.go", name)
		}
	}
}

func TestEtsyPrivacyTablesSQLDenied(t *testing.T) {
	resetMCPPathEnv(t)
	t.Setenv(BuyerPIIEnvVar, "")
	path, e := mcpDBPath()
	if e != nil {
		t.Fatal(e)
	}
	db, e := store.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	for _, name := range []string{"etsy_erasure_suppression", "etsy_retention_receipts", "etsy_retention_floor", "etsy_erasure_receipts", "etsy_erasure_transactions", "etsy_erasure_buyer_receipts"} {
		if _, e = db.DB().Exec(`CREATE TABLE IF NOT EXISTS ` + name + `(id TEXT)`); e != nil {
			t.Fatal(e)
		}
		r, e := handleSQL(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"query": "SELECT * FROM " + name}}})
		if e != nil || r == nil || !r.IsError {
			t.Fatalf("%s bypass: %v %+v", name, e, r)
		}
	}
}
