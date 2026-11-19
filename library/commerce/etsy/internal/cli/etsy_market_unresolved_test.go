package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// Exercise the CLI and its default store, including the partial-to-complete retry.
func TestEtsyMarketUnresolvedShops(t *testing.T) {
	for _, mode := range []string{"omitted", "both", "403", "404"} {
		t.Run(mode, func(t *testing.T) {
			mktTestEnv(t)
			// Disable client retry backoff for the synthetic 500 response.
			t.Setenv("PRINTING_PRESS_VERIFY", "1")
			f := newMktFakeEtsy(t)
			etsyResolved := mode != "omitted"
			etsyBatchRequests := []string{}
			f.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/listings/active":
					_ = json.NewEncoder(w).Encode(map[string]any{"count": 2, "results": []map[string]any{
						{"listing_id": 1000000001, "shop_id": 555001, "title": "linen apron"},
						{"listing_id": 1000000002, "shop_id": 555002, "title": "Juniper Ceramics"},
					}})
				case "/listings/batch":
					etsyBatchRequests = append(etsyBatchRequests, r.URL.Query().Get("listing_ids"))
					if mode == "403" || mode == "404" {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					etsyResults := []map[string]any{}
					for i := 0; i < 2; i++ {
						etsyID := fmt.Sprint(1000000001 + i)
						if !strings.Contains(r.URL.Query().Get("listing_ids"), etsyID) || (i == 1 && !etsyResolved) {
							continue
						}
						etsyResults = append(etsyResults, map[string]any{"listing_id": 1000000001 + i, "shop_id": 555001 + i,
							"Shop": map[string]any{"shop_id": 555001 + i, "transaction_sold_count": 123, "listing_active_count": 7}})
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"results": etsyResults})
				case "/shops/555001", "/shops/555002":
					if mode == "omitted" && !etsyResolved && r.URL.Path == "/shops/555002" {
						w.WriteHeader(http.StatusInternalServerError)
					} else if mode == "403" {
						w.WriteHeader(http.StatusForbidden)
					} else {
						w.WriteHeader(http.StatusNotFound)
					}
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			})
			mktPointAtFake(t, f)
			out, err := mktRunCLI(t, mktSnapArgs("--term", "linen apron", "--pages", "1", "--max-calls", "10")...)
			if err != nil {
				t.Fatalf("snapshot: %v\n%s", err, out)
			}
			sum := mktDecode(t, out)
			etsyID := int64(sum["snapshot_id"].(float64))
			db := mktOpenFixtureStore(t)
			defer db.Close()
			etsyCheck := func(shop int, exists, populated bool) {
				t.Helper()
				var sold, active sql.NullInt64
				err := db.DB().QueryRow("SELECT transaction_sold_count, listing_active_count FROM market_shops WHERE snapshot_id=? AND shop_id=?", etsyID, shop).Scan(&sold, &active)
				if !exists {
					if err != sql.ErrNoRows {
						t.Fatalf("shop %d: want no row, got %v", shop, err)
					}
					return
				}
				if err != nil || sold.Valid != populated || active.Valid != populated {
					t.Fatalf("shop %d: sold=%v active=%v err=%v", shop, sold, active, err)
				}
			}
			etsyCheck(555001, true, mode != "403" && mode != "404")
			etsyCheck(555002, mode != "omitted", mode == "both")
			if mode != "omitted" {
				if _, ok := sum["unresolved_shops"]; ok {
					t.Fatalf("unexpected unresolved: %v", sum)
				}
				return
			}
			if sum["unresolved_shops"] != float64(1) || !strings.Contains(fmt.Sprint(sum["note"]), "could not be resolved") || sum["status"] != "partial" {
				t.Fatalf("summary: %v", sum)
			}
			etsyResolved = true
			out, err = mktRunCLI(t, mktSnapArgs("--resume", fmt.Sprint(etsyID), "--max-calls", "10")...)
			if err != nil {
				t.Fatalf("resume: %v\n%s", err, out)
			}
			sum = mktDecode(t, out)
			etsyCheck(555002, true, true)
			if _, ok := sum["unresolved_shops"]; ok {
				t.Fatalf("stale unresolved count: %v", sum)
			}
			if sum["status"] != "complete" || len(etsyBatchRequests) != 2 || etsyBatchRequests[1] != "1000000002" {
				t.Fatalf("resume summary=%v batches=%v", sum, etsyBatchRequests)
			}
		})
	}
}

func TestEtsyMarketUnresolvedSummaryPreservesCapNote(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	defer db.Close()
	for _, status := range []string{"partial", "complete"} {
		sum := mktSummarize(db.DB(), &mktRun{snap: &mktSnapshot{Progress: mktProg{ReviewsCapped: []int64{1000000001}}}, unresolvedShops: 1}, status)
		note := fmt.Sprint(sum["note"])
		if sum["unresolved_shops"] != 1 || !strings.Contains(note, "Review collection capped") || !strings.Contains(note, "could not be resolved") {
			t.Fatalf("summary: %v", sum)
		}
	}
}
