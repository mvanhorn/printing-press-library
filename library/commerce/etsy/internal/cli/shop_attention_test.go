// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
)

// TestNovelShopAttentionHelpWires smoke-tests that the shop attention command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelShopAttentionHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"shop", "attention", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("shop attention --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "attention"} {
		if !strings.Contains(help, want) {
			t.Fatalf("shop attention --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestShopAttentionSummary(t *testing.T) {
	f := shopxNewFixture(t)
	f.put("receipts",
		map[string]any{"receipt_id": 55, "is_paid": true, "is_shipped": false},
		map[string]any{"receipt_id": 56, "is_paid": true, "is_shipped": true},
		map[string]any{"receipt_id": 57, "is_paid": false},
		map[string]any{"receipt_id": 58, "is_paid": true, "status": "canceled"})
	f.put("listings",
		map[string]any{"listing_id": 1000000001, "state": "active", "title": "Linen apron", "tags": []string{"linen", "apron", "gift"}, "ending_timestamp": shopxIn(10 * shopxDay)},
		map[string]any{"listing_id": 1000000002, "state": "active", "ending_timestamp": shopxIn(45 * shopxDay)},
		map[string]any{"listing_id": 1000000003, "state": "inactive", "ending_timestamp": shopxIn(5 * shopxDay)})
	f.put("ledger_entries",
		map[string]any{"entry_id": 1, "ledger_id": 1, "sequence_number": 1, "amount": 100, "balance": 100, "currency": "USD", "created_timestamp": shopxAgo(2 * shopxDay)},
		map[string]any{"entry_id": 2, "ledger_id": 1, "sequence_number": 2, "amount": 50, "balance": 160, "currency": "USD", "created_timestamp": shopxAgo(shopxDay)})
	f.synced("listings", 2*time.Hour)
	out, stderr, err := shopxRun(t, f, "shop", "attention")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "hint: ") {
		t.Fatalf("unexpected stderr: %s", stderr)
	}
	obj := shopxMustObject(t, out)
	_, envelopeNotes := shopxMustEnvelope(t, out)
	for key, want := range map[string]float64{"unshipped_count": 1, "expiring_30d": 1, "reconcile_mismatches": 1} {
		if obj[key] != want {
			t.Fatalf("%s = %v, want %v", key, obj[key], want)
		}
	}
	ages := obj["last_sync_age_seconds"].(map[string]any)
	shopxAttentionCheckAges(t, ages, true)
	db, err := store.OpenReadOnly(f.path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, auditNotes := shopxAuditListings(db, shopxTestShopID, "all")
	got := obj["audit_findings"].(map[string]any)
	want := shopxAuditCounts(rows)
	if len(got) != len(want) {
		t.Fatalf("audit = %v, want %v", got, want)
	}
	for rule, count := range want {
		if got[rule] != float64(count) {
			t.Fatalf("audit %s = %v, want %d", rule, got[rule], count)
		}
	}
	if got["tags_lt_13"] != float64(1) || got["title_lt_40"] != float64(1) {
		t.Fatalf("audit: %v", got)
	}
	reconcile, reconcileNotes := shopxReconcile(db, shopxTestShopID, shopxFixedNow, 30*shopxDay)
	if obj["reconcile_mismatches"] != float64(reconcile.MismatchCount) {
		t.Fatal("reconcile inconsistent")
	}
	notes := make([]string, 0)
	for _, note := range auditNotes {
		notes = append(notes, "audit: "+note)
	}
	for _, note := range reconcileNotes {
		notes = append(notes, "reconcile: "+note)
	}
	if !reflect.DeepEqual(envelopeNotes, notes) {
		t.Fatalf("notes = %v, want %v", envelopeNotes, notes)
	}
	out, _, err = shopxRun(t, f, "listings", "expiring")
	if err != nil {
		t.Fatal(err)
	}
	expiring := shopxMustList(t, out)
	if len(expiring) != 1 || expiring[0]["listing_id"] != float64(1000000001) || obj["expiring_30d"] != float64(len(expiring)) {
		t.Fatalf("expiring inconsistent: %v", expiring)
	}
	out, _, err = shopxRun(t, f, "money", "reconcile")
	if err != nil {
		t.Fatal(err)
	}
	if shopxMustObject(t, out)["mismatch_count"] != obj["reconcile_mismatches"] {
		t.Fatal("reconcile command inconsistent")
	}
}

func shopxAttentionCheckAges(t *testing.T, ages map[string]any, synced bool) {
	t.Helper()
	keys := []string{"listings", "listing_images", "listings_inventory", "receipts", "transactions", "shops_receipts_payments", "ledger_entries", "reviews", "economic_operators"}
	if len(ages) != len(keys) {
		t.Fatalf("ages: %v", ages)
	}
	for _, key := range keys {
		value, present := ages[key]
		if !present {
			t.Fatalf("missing age %s", key)
		}
		if synced && key == "listings" {
			if value != float64(7200) {
				t.Fatalf("listing age: %v", value)
			}
		} else if value != nil {
			t.Fatalf("%s age = %v, want null", key, value)
		}
	}
}

func TestShopAttentionEmpty(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "empty"}[existing], func(t *testing.T) {
			var f *shopxFixture
			if existing {
				f = shopxNewFixture(t)
			}
			out, _, err := shopxRun(t, f, "shop", "attention")
			if err != nil {
				t.Fatal(err)
			}
			obj := shopxMustObject(t, out)
			_, envelopeNotes := shopxMustEnvelope(t, out)
			for _, key := range []string{"unshipped_count", "expiring_30d", "reconcile_mismatches"} {
				if obj[key] != nil {
					t.Fatalf("%s: %v", key, obj[key])
				}
			}
			if obj["audit_findings"] != nil {
				t.Fatal(obj)
			}
			shopxAttentionCheckAges(t, obj["last_sync_age_seconds"].(map[string]any), false)
			notes := envelopeNotes
			found := false
			for _, note := range notes {
				if note == "store empty: run 'etsy-pp-cli sync'" {
					found = true
				}
			}
			if !found && existing {
				t.Fatal("missing empty-store note")
			}
		})
	}
}

func TestShopAttentionReceiptAliases(t *testing.T) {
	for _, tc := range []struct {
		name string
		row  map[string]any
		want float64
	}{
		{"legacy paid", map[string]any{"was_paid": true, "created_timestamp": shopxAgo(365 * shopxDay)}, 1},
		{"legacy shipped", map[string]any{"is_paid": true, "was_shipped": true}, 0},
		{"cancelled uppercase", map[string]any{"is_paid": true, "status": "CANCELLED"}, 0},
		{"first paid flag wins", map[string]any{"is_paid": false, "was_paid": true}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := shopxNewFixture(t)
			tc.row["receipt_id"] = 55
			f.put("receipts", tc.row)
			out, _, err := shopxRun(t, f, "shop", "attention")
			if err != nil {
				t.Fatal(err)
			}
			if got := shopxMustObject(t, out)["unshipped_count"]; got != tc.want {
				t.Fatalf("count=%v, want %v", got, tc.want)
			}
		})
	}
}
