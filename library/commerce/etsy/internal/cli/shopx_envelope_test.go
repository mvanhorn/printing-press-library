package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var shopxEnvelopeCommands = [][]string{
	{"listings", "audit"}, {"listings", "expiring"}, {"listings", "dead-weight"},
	{"inventory", "audit"}, {"money", "fees"}, {"money", "reconcile"},
	{"money", "summary"}, {"reviews", "digest"}, {"shop", "attention"}, {"orders", "unshipped"},
}

func TestShopxEnvelopeCommands(t *testing.T) {
	for _, args := range shopxEnvelopeCommands {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			f := shopxNewFixture(t)
			f.put("listings", map[string]any{"listing_id": 1000000001, "state": "active", "ending_timestamp": shopxIn(shopxDay)})
			out, _, err := shopxRun(t, f, args...)
			if err != nil {
				t.Fatal(err)
			}
			var obj map[string]any
			if err := json.Unmarshal([]byte(out), &obj); err != nil {
				t.Fatal(err)
			}
			if len(obj) != 2 {
				t.Fatal(out)
			}
			shopxMustEnvelope(t, out)
		})
	}
}

func TestShopxNoStoreEnvelopeAndHint(t *testing.T) {
	for _, args := range shopxEnvelopeCommands {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			f := shopxNewFixture(t)
			f.close()
			out, stderr, err := shopxRun(t, nil, args...)
			if err != nil {
				t.Fatal(err)
			}
			results, notes := shopxMustEnvelope(t, out)
			switch args[0] + " " + args[1] {
			case "listings audit", "listings expiring", "listings dead-weight", "inventory audit", "money fees", "orders unshipped":
				rows, ok := results.([]any)
				if !ok || len(rows) != 0 {
					t.Fatal(out)
				}
			case "shop attention":
				obj := results.(map[string]any)
				for _, key := range []string{"unshipped_count", "expiring_30d", "audit_findings", "reconcile_mismatches"} {
					if value, ok := obj[key]; !ok || value != nil {
						t.Fatal(out)
					}
				}
				if len(notes) != 1 || notes[0] != shopxNoteNoStore {
					t.Fatal(out)
				}
			}
			if !strings.Contains(strings.Join(notes, "\n"), shopxNoteNoStore) || !strings.Contains(stderr, "hint: "+shopxNoteNoStore) {
				t.Fatal(out, stderr)
			}
		})
	}
}

func TestShopxNotesMirrorToStderr(t *testing.T) {
	for _, args := range [][]string{{"shop", "attention"}, {"money", "reconcile"}} {
		f := shopxNewFixture(t)
		out, stderr, err := shopxRun(t, f, args...)
		if err != nil {
			t.Fatal(err)
		}
		_, notes := shopxMustEnvelope(t, out)
		seen := map[string]bool{}
		for _, note := range notes {
			if seen[note] || strings.Count(stderr, "hint: "+note+"\n") != 1 {
				t.Fatal(notes, stderr)
			}
			seen[note] = true
		}
	}
}

func TestShopAttentionNullsForUnknownFamilies(t *testing.T) {
	f := shopxNewFixture(t)
	out, _, err := shopxRun(t, f, "shop", "attention")
	if err != nil {
		t.Fatal(err)
	}
	obj := shopxMustObject(t, out)
	for _, key := range []string{"unshipped_count", "expiring_30d", "audit_findings", "reconcile_mismatches"} {
		v, ok := obj[key]
		if !ok || v != nil {
			t.Fatalf("%s = %v", key, v)
		}
	}
	f = shopxNewFixture(t)
	f.synced("receipts", time.Hour)
	out, _, err = shopxRun(t, f, "shop", "attention")
	if err != nil {
		t.Fatal(err)
	}
	if shopxMustObject(t, out)["unshipped_count"] != float64(0) {
		t.Fatal(out)
	}
	f = shopxNewFixture(t)
	f.put("listings", map[string]any{"listing_id": 1000000001, "state": "active"})
	t.Setenv("ETSY_SHOP_ID", "")
	out, _, err = shopxRun(t, f, "shop", "attention")
	if err != nil {
		t.Fatal(err)
	}
	obj = shopxMustObject(t, out)
	if obj["expiring_30d"] != nil || obj["audit_findings"] != nil || !strings.Contains(out, shopxNoteShopUnset) {
		t.Fatal(out)
	}
}

func TestShopxEnvelopeAgentSelection(t *testing.T) {
	for _, flags := range [][]string{{"--agent"}, {"--compact"}, {"--agent", "--compact", "--select", "results.listing_id"}} {
		f := shopxNewFixture(t)
		f.put("listings", map[string]any{"listing_id": 1000000001, "state": "active", "ending_timestamp": shopxIn(shopxDay)})
		out, _, err := shopxRun(t, f, append([]string{"listings", "expiring"}, flags...)...)
		if err != nil {
			t.Fatal(err)
		}
		results, _ := shopxMustEnvelope(t, out)
		b, _ := json.Marshal(results)
		if len(flags) > 2 && strings.Contains(string(b), "title") {
			t.Fatal(out)
		}
		if !strings.Contains(string(b), "1000000001") {
			t.Fatal(out)
		}
	}
}
