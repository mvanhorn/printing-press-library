package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMarketSelectEverySubcommand(t *testing.T) {
	for _, tc := range []struct{ command, field string }{{"demand", "term"}, {"gaps", "name"}, {"price", "group"}, {"shops", "shop_id,transaction_sold_count"}, {"snapshot", "listings"}, {"tags", "term"}, {"trend", "term"}} {
		for _, mode := range []string{"row", "anchored", "typo", "notes", "NOTES", "empty_store_notes", "empty_store_NOTES", "empty_store_row", "empty_store_anchored", "empty_snapshot_row", "empty_snapshot_anchored"} {
			t.Run(tc.command+"_"+mode, func(t *testing.T) {
				// snapshot has no empty-store or empty-snapshot state.
				if tc.command == "snapshot" && strings.HasPrefix(mode, "empty_") {
					t.Skip("snapshot has no empty state")
				}
				mktTestEnv(t)
				var latest int64
				if !strings.HasPrefix(mode, "empty_store") {
					db := mktOpenFixtureStore(t)
					for j := 0; j < 2 && !strings.HasPrefix(mode, "empty_store"); j++ {
						if j == 1 && strings.HasPrefix(mode, "empty_snapshot") {
							break
						}
						latest = mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron"}, 1, "complete", mktFxProg{})
						for i := 0; i < 12 && !strings.HasPrefix(mode, "empty_snapshot"); i++ {
							mktFxResult(t, db.DB(), latest, mktFxListing{term: "linen apron", pos: i + 1 + j, id: int64(1000000001 + i), shop: 9000001, title: "linen apron", price: 2400, tags: []string{"linen apron"}})
						}
						mktFxShop(t, db.DB(), latest, 9000001, 100, 20, time.Now().Add(-400*24*time.Hour).Unix())
					}
					db.Close()
				}
				args := []string{"market", tc.command}
				if tc.command == "gaps" {
					p := filepath.Join(t.TempDir(), "rules.json")
					if err := os.WriteFile(p, []byte(`{"targets":[{"name":"linen apron","include":["linen apron"]}]}`), 0600); err != nil {
						t.Fatal(err)
					}
					args = append(args, "--rules", p)
				}
				// Snapshot summaries contain scalar counts, no row collection. A complete
				// resume exercises the closest hermetic path: selecting its listings count.
				if tc.command == "snapshot" {
					args = append(args, "--resume", fmt.Sprint(latest))
				}

				if mode == "notes" || mode == "NOTES" || strings.HasSuffix(mode, "_notes") || strings.HasSuffix(mode, "_NOTES") {
					field := strings.TrimPrefix(mode, "empty_store_")
					baseline, err := mktRunCLI(t, args...)
					if err != nil {
						t.Fatalf("MARKET_SELECT_BASELINE: %v", err)
					}
					out, err := mktRunCLI(t, append(args, "--select", field)...)
					if err != nil {
						t.Fatalf("MARKET_SELECT_NOTES_ONLY_REJECTED: %v %s", err, out)
					}
					got := mktDecode(t, out)
					if _, ok := got["notes"]; !ok || len(got) != 1 {
						t.Fatalf("MARKET_SELECT_NOTES_ONLY_SHAPE: %s", out)
					}
					if !reflect.DeepEqual(got["notes"], mktDecode(t, baseline)["notes"]) {
						t.Fatalf("MARKET_SELECT_NOTES_ONLY_VALUE: %s baseline %s", out, baseline)
					}
					return
				}
				field := tc.field
				if mode == "anchored" || strings.HasSuffix(mode, "_anchored") {
					field = "results." + strings.ReplaceAll(field, ",", ",results.")
				}
				if strings.HasPrefix(mode, "empty_") {
					if strings.HasSuffix(mode, "_anchored") && tc.command == "tags" {
						field = "results.tags"
					}
					if strings.HasSuffix(mode, "_anchored") && tc.command == "trend" {
						field = "results.new_listings"
					}
					baseline, err := mktRunCLI(t, args...)
					if err != nil {
						t.Fatalf("MARKET_SELECT_BASELINE: %v", err)
					}
					out, err := mktRunCLI(t, append(args, "--select", field)...)
					if err != nil {
						t.Fatalf("MARKET_SELECT_EMPTY_REJECTED: %v %s", err, out)
					}
					got := mktDecode(t, out)
					switch r := got["results"].(type) {
					case []any:
						if len(r) != 0 && !(strings.HasPrefix(mode, "empty_snapshot") && (tc.command == "demand" || tc.command == "gaps")) {
							t.Fatalf("MARKET_SELECT_EMPTY_ROWS: %s", out)
						}
					case map[string]any:
						if len(r) != 0 {
							t.Fatalf("MARKET_SELECT_EMPTY_ROWS: %s", out)
						}
					default:
						t.Fatalf("MARKET_SELECT_EMPTY_NULL: %s", out)
					}
					notes, ok := got["notes"].([]any)
					if !ok || !reflect.DeepEqual(notes, mktDecode(t, baseline)["notes"]) {
						t.Fatalf("MARKET_SELECT_NOTES_DROPPED: %s baseline %s", out, baseline)
					}
					if strings.HasPrefix(mode, "empty_store") && !reflect.DeepEqual(notes, []any{mktNoSnapshotHint}) {
						t.Fatalf("MARKET_SELECT_EMPTY_HINT: %s", out)
					}
					if strings.HasPrefix(mode, "empty_snapshot") && tc.command != "gaps" && tc.command != "shops" && len(notes) == 0 {
						t.Fatalf("MARKET_SELECT_EMPTY_NOTES: %s", out)
					}
					// demand and gaps emit aggregate rows even without listing rows.
					// gaps and shops emit no advisory for a complete empty snapshot.
					return
				}
				if mode == "typo" {
					field = "bogusfield"
					if tc.command == "shops" {
						field = "results.bogus"
					}
				}
				out, err := mktRunCLI(t, append(args, "--select", field)...)
				if mode == "typo" {
					if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "--select matched no fields") {
						t.Fatalf("MARKET_SELECT_TYPO_ACCEPTED: %v %s", err, out)
					}
					return
				}
				if err != nil {
					t.Fatalf("MARKET_SELECT_ROW_LOST: %v %s", err, out)
				}
				baseline, baselineErr := mktRunCLI(t, args...)
				if baselineErr != nil {
					t.Fatalf("MARKET_SELECT_ROW_NOTES_BASELINE_RUN: %v", baselineErr)
				}
				m := mktDecode(t, out)
				if !reflect.DeepEqual(m["notes"], mktDecode(t, baseline)["notes"]) {
					t.Fatalf("MARKET_SELECT_ROW_NOTES_BASELINE: %s baseline %s", out, baseline)
				}
				if _, ok := m["notes"].([]any); !ok {
					t.Fatalf("MARKET_SELECT_NOTES_LOST: %s", out)
				}
				if tc.command == "shops" {
					want := []any{map[string]any{"shop_id": float64(9000001), "transaction_sold_count": float64(100)}}
					expected := map[string]any{"results": want, "notes": []any{}}
					if mode == "row" {
						full, err := mktRunCLI(t, args...)
						if err != nil {
							t.Fatal(err)
						}
						baseline := mktDecode(t, full)
						// Generic fallback preserves non-row siblings verbatim.
						expected["crowd_out"] = baseline["crowd_out"]
						expected["snapshot_id"] = baseline["snapshot_id"]
					}
					if !reflect.DeepEqual(m, expected) {
						t.Fatalf("MARKET_SELECT_SHOP_SHAPE: %s", out)
					}
				} else if tc.command != "snapshot" {
					raw := fmt.Sprint(m["results"])
					if !strings.Contains(raw, strings.Split(tc.field, ",")[0]+":") {
						t.Fatalf("MARKET_SELECT_FIELD_LOST: %s", out)
					}
				}
				out, err = mktRunCLI(t, append(args, "--select", "results")...)
				if err != nil {
					t.Fatal(err)
				}
				if _, ok := mktDecode(t, out)["notes"].([]any); !ok {
					t.Fatalf("MARKET_SELECT_RESULTS_NOTES_LOST: %s", out)
				}
			})
		}
	}
}

func TestMarketDryRunTopLevel(t *testing.T) {
	for _, command := range []string{"tags", "snapshot"} {
		for _, mode := range []string{"json", "agent", "select"} {
			t.Run(command+"_"+mode, func(t *testing.T) {
				mktTestEnv(t)
				args := []string{"market", command, "--dry-run"}
				if command == "snapshot" {
					args = append(args, "--term", "linen apron")
				}
				if mode == "agent" {
					args = append(args, "--agent")
				}
				if mode == "select" {
					args = append(args, "--select", "bogusfield")
				}
				out, err := mktRunCLI(t, args...)
				if err != nil {
					t.Fatalf("MARKET_DRY_RUN_FILTERED: %v %s", err, out)
				}
				m := mktDecode(t, out)
				if m["dry_run"] != true || m["success"] != true || m["action"] != "market "+command || m["would"] == nil {
					t.Fatalf("MARKET_DRY_RUN_NOT_FLAT: %s", out)
				}
				for _, key := range []string{"results", "notes"} {
					if a, ok := m[key].([]any); !ok || len(a) != 0 {
						t.Fatalf("MARKET_DRY_RUN_ARRAY: %s", out)
					}
				}
				if command == "snapshot" && m["plan"] == nil {
					t.Fatalf("MARKET_DRY_RUN_PLAN_LOST: %s", out)
				}
			})
		}
	}
}

func TestMarketTagsTrendCSVRefused(t *testing.T) {
	for _, command := range []string{"tags", "trend"} {
		t.Run(command, func(t *testing.T) {
			mktTestEnv(t)
			out, err := mktRunCLI(t, "market", command, "--csv")
			if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "market "+command+" output is nested; use --json") {
				t.Fatalf("MARKET_NESTED_CSV_ACCEPTED: %v %s", err, out)
			}
		})
	}
}
