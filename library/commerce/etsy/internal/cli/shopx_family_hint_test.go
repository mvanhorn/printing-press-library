package cli

import (
	"strings"
	"testing"
	"time"
)

func TestShopxFamilyHints(t *testing.T) {
	if syncHintCommand == "" {
		t.Fatal("expected a working sync hint command")
	}
	for _, tc := range []struct {
		name                   string
		args                   []string
		listings, transactions bool
		want, absent           string
	}{
		{"sibling listings", []string{"listings", "expiring"}, true, false, "", "listings not synced yet"},
		{"missing transactions", []string{"listings", "dead-weight"}, true, false, "transactions not synced yet", "listings not synced yet"},
		{"sibling transactions", []string{"listings", "dead-weight"}, false, true, "listings not synced yet", "transactions not synced yet"},
		{"synced receipts", []string{"orders", "unshipped"}, false, false, "", "receipts not synced yet"},
		{"empty receipts", []string{"orders", "unshipped"}, false, false, "receipts not synced yet", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := shopxNewFixture(t)
			if tc.listings {
				f.put("listings-featured", map[string]any{"listing_id": 1000000001, "state": "active", "ending_timestamp": shopxIn(shopxDay)})
				f.synced("listings-featured", time.Hour)
			}
			if tc.transactions {
				f.put("listings_transactions", map[string]any{"transaction_id": 1, "listing_id": 1000000001})
				f.synced("listings_transactions", time.Hour)
			}
			if tc.name == "synced receipts" {
				f.synced("receipts", time.Hour)
			}
			_, stderr, err := shopxRun(t, f, tc.args...)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(stderr, "has not been synced yet") {
				t.Fatalf("generic hint: %s", stderr)
			}
			if tc.want != "" && !strings.Contains(stderr, tc.want) {
				t.Fatalf("missing %q: %s", tc.want, stderr)
			}
			if tc.absent != "" && strings.Contains(stderr, tc.absent) {
				t.Fatalf("unexpected %q: %s", tc.absent, stderr)
			}
		})
	}
}

func TestShopAttentionUnknownFamilyNotes(t *testing.T) {
	const receiptNote = "receipts not synced; unshipped_count is unknown, not zero"
	const listingNote = "listings not synced; expiring_30d and audit_findings are unknown, not zero"
	for _, tc := range []struct {
		name                            string
		listings, receipts, syncedEmpty bool
	}{
		{name: "empty"},
		{name: "listings only", listings: true},
		{name: "both populated", listings: true, receipts: true},
		{name: "synced empty", syncedEmpty: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := shopxNewFixture(t)
			if tc.listings {
				f.put("listings-featured", map[string]any{"listing_id": 1000000001, "state": "active"})
			}
			if tc.receipts {
				f.put("receipts", map[string]any{"receipt_id": 1})
			}
			if tc.syncedEmpty {
				f.synced("listings-featured", time.Hour)
				f.synced("receipts", time.Hour)
			}
			out, _, err := shopxRun(t, f, "shop", "attention")
			if err != nil {
				t.Fatal(err)
			}
			_, notes := shopxMustEnvelope(t, out)
			for note, want := range map[string]bool{receiptNote: !tc.receipts && !tc.syncedEmpty, listingNote: !tc.listings && !tc.syncedEmpty} {
				found := false
				for _, got := range notes {
					if got == note {
						found = true
					}
				}
				if found != want {
					t.Fatalf("note %q present=%v, want %v; notes=%v", note, found, want, notes)
				}
			}
		})
	}
}
