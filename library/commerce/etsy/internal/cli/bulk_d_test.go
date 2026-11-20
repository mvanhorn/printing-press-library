package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

func TestBulkDRunTimeoutIsPerCall(t *testing.T) {
	s2FastThrottle(t, 400*time.Millisecond)
	srv := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
		switch {
		case strings.HasSuffix(r.URL.Path, "/personalization"):
			w.WriteHeader(404)
		case strings.HasSuffix(r.URL.Path, "/inventory"):
			fmt.Fprint(w, s2Inventory)
		case r.Method == "GET":
			fmt.Fprint(w, s2Listing1)
		default:
			fmt.Fprint(w, `{}`)
		}
		return true
	})
	var items []string
	db := s2FixtureDB(t, func(db *store.Store) {
		for id := int64(1000000101); id <= 1000000110; id++ {
			s2Upsert(t, db.UpsertListings, fmt.Sprintf(`{"listing_id":%d,"title":"old"}`, id))
			items = append(items, fmt.Sprintf(`{"listing_id":%d,"field":"title","value":"new"}`, id))
		}
	})
	out, _, err := s2Run(t, "listings", "bulk-edit", "--plan", s2WritePlan(t, `{"version":1,"items":[`+strings.Join(items, ",")+`]}`), "--db", db, "--apply", "--yes", "--timeout", "2s", "--json", "--no-cache", "--rate-limit", "1000")
	s2Must(t, err)
	var res bulkApplyResult
	s2Decode(t, out, &res)
	n := 0
	for _, r := range srv.all() {
		if r.Method == "PATCH" {
			n++
		}
	}
	if res.Failed != 0 || n != 10 {
		t.Fatalf("patches=%d result=%s", n, out)
	}
}

func TestBulkDFreshAndResumeValidation(t *testing.T) {
	s2FastThrottle(t, time.Millisecond)
	srv := s2NewServer(t, s2ShopHandler(nil))
	args := []string{"listings", "bulk-edit", "--plan", s2WritePlan(t, s2TagsPlan), "--db", s2SeedShop(t), "--apply", "--yes", "--json", "--no-cache", "--rate-limit", "1000"}
	var res bulkApplyResult
	for i := 0; i < 2; i++ {
		out, _, err := s2Run(t, args...)
		s2Must(t, err)
		s2Decode(t, out, &res)
		if res.Done != 4 || res.SkippedDone != 0 {
			t.Fatal(out)
		}
	}
	if srv.count("PATCH", "/shops/9000001/listings/1000000001") != 2 || srv.count("PATCH", "/shops/9000001/listings/1000000002") != 2 {
		t.Fatal(srv.all())
	}
	for _, job := range []string{"wrong", res.Job} {
		_, _, err := s2Run(t, append(args, "--resume", job)...)
		if ExitCode(err) != 2 {
			t.Fatalf("resume %s: %v", job, err)
		}
	}
}

func TestBulkDVariationPrices(t *testing.T) {
	s2FastThrottle(t, time.Millisecond)
	srv := s2NewServer(t, s2ShopHandler(nil))
	db := s2SeedShop(t)
	st, err := store.Open(db)
	s2Must(t, err)
	inv := strings.Replace(s2Inventory, `{"products":`, `{"listing_id":1000000001,"products":`, 1)
	s2Must(t, st.Upsert("listings_inventory", "1000000001", json.RawMessage(inv)))
	s2Must(t, st.Close())
	plan := s2WritePlan(t, `{"version":1,"items":[{"listing_id":1000000001,"field":"price","value":27.5}]}`)
	args := []string{"listings", "bulk-edit", "--plan", plan, "--db", db, "--json", "--no-cache", "--rate-limit", "1000"}
	for _, extra := range [][]string{nil, {"--apply", "--yes"}} {
		out, _, err := s2Run(t, append(args, extra...)...)
		if ExitCode(err) != 2 || !strings.Contains(out, "--flatten-variation-prices") {
			t.Fatalf("%s %v", out, err)
		}
	}
	if len(srv.all()) != 0 {
		t.Fatal(srv.all())
	}
	out, _, err := s2Run(t, append(args, "--flatten-variation-prices")...)
	s2Must(t, err)
	var preview bulkPreview
	s2Decode(t, out, &preview)
	offers := preview.Changes[0].Offerings
	if len(offers) != 2 || offers[0].Old != 24 || offers[1].Old != 30 || offers[0].New != 27.5 || offers[1].New != 27.5 {
		t.Fatal(out)
	}
	_, _, err = s2Run(t, append(args, "--flatten-variation-prices", "--apply", "--yes")...)
	s2Must(t, err)
	for _, req := range srv.all() {
		if req.Method == "PUT" {
			var body map[string]any
			s2Decode(t, req.Body, &body)
			props, ok := body["price_on_property"].([]any)
			if !ok || len(props) != 0 {
				t.Fatal(req.Body)
			}
			for _, p := range body["products"].([]any) {
				for _, o := range p.(map[string]any)["offerings"].([]any) {
					if o.(map[string]any)["price"] != 27.5 {
						t.Fatal(req.Body)
					}
				}
			}
		}
	}
	// An unsynced inventory must still be protected by the authoritative live check.
	args[5] = s2SeedShop(t)
	out, _, err = s2Run(t, append(args, "--apply", "--yes")...)
	if err == nil {
		t.Fatal(out)
	}
	var res bulkApplyResult
	s2Decode(t, out, &res)
	if res.Failed != 1 {
		t.Fatal(out)
	}
	st, err = store.Open(args[5])
	s2Must(t, err)
	defer st.Close()
	ops, err := st.BulkJobOps(res.Job)
	s2Must(t, err)
	for _, op := range ops {
		if op.Kind == "price" && op.Status != store.BulkStatusFailed {
			t.Fatal(op)
		}
	}
	if srv.count("PUT", "/listings/1000000001/inventory") != 1 {
		t.Fatal(srv.all())
	}
}

func TestBulkDJournalFailureStops(t *testing.T) {
	for _, success := range []bool{false, true} {
		t.Run(fmt.Sprint(success), func(t *testing.T) {
			st, err := store.Open(s2SeedShop(t))
			s2Must(t, err)
			s2Must(t, st.Close())
			r := &bulkRunner{db: st, ctx: context.Background()}
			if success {
				r.succeed("op", 0)
			} else {
				r.fail("op", "failed", fmt.Errorf("failed"))
			}
			if r.journalErr == nil || !r.applyListing(1, bulkApplyInput{ContinueOnError: true}) {
				t.Fatal("journal failure did not stop run")
			}
		})
	}
}

func TestBulkDDryRunReadsAndQuantityTypes(t *testing.T) {
	for _, absent := range []bool{false, true} {
		t.Run(fmt.Sprint(absent), func(t *testing.T) {
			inv := s2Inventory
			if absent {
				inv = strings.Replace(inv, `"quantity":5,`, "", 1)
			}
			srv := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
				if strings.HasSuffix(r.URL.Path, "/inventory") {
					fmt.Fprint(w, inv)
				} else {
					fmt.Fprint(w, s2Listing1)
				}
				return true
			})
			out, _, err := s2Run(t, "inventory", "set-qty", "1000000001", "--product", "4000000001", "--qty", "12", "--dry-run", "--yes", "--json", "--no-cache")
			s2Must(t, err)
			var result map[string]any
			s2Decode(t, out, &result)
			diff := result["diff"].(map[string]any)
			if result["mode"] != "preview" || diff["new_quantity"] != float64(12) {
				t.Fatal(out)
			}
			if absent {
				if diff["old_quantity"] != nil {
					t.Fatal(out)
				}
			} else {
				if q, ok := diff["old_quantity"].(float64); !ok || q != 5 {
					t.Fatal(out)
				}
			}
			if len(srv.all()) != 1 || srv.all()[0].Method != "GET" {
				t.Fatal(srv.all())
			}
			out, _, err = s2Run(t, "listings", "copy", "1000000001", "--dry-run", "--yes", "--json", "--no-cache")
			s2Must(t, err)
			s2Decode(t, out, &result)
			if result["mode"] != "preview" || result["draft"] == nil {
				t.Fatal(out)
			}
			if len(srv.all()) != 2 || srv.all()[1].Method != "GET" {
				t.Fatal(srv.all())
			}
		})
	}
}

func TestBulkDYesNeedsApply(t *testing.T) {
	srv := s2NewServer(t, nil)
	_, _, err := s2Run(t, "listings", "bulk-edit", "--yes")
	if ExitCode(err) != 2 || !strings.Contains(err.Error(), "--yes only confirms --apply") || len(srv.all()) != 0 {
		t.Fatal(err)
	}
}

func TestBulkDDailyQuotaStopsAndResumes(t *testing.T) {
	dEnv(t)
	quota := true
	base := s2ShopHandler(nil)
	srv := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, body string) bool {
		if quota && r.Method == "PATCH" {
			w.Header().Set("x-remaining-today", "0")
			w.WriteHeader(429)
			fmt.Fprint(w, `{"error":"quota"}`)
			return true
		}
		return base(w, r, body)
	})
	path := s2SeedShop(t)
	args := []string{"listings", "bulk-edit", "--plan", s2WritePlan(t, s2TagsPlan), "--db", path, "--apply", "--yes", "--json", "--no-cache", "--rate-limit", "1000"}
	out, _, err := s2Run(t, args...)
	var res bulkApplyResult
	s2Decode(t, out, &res)
	if ExitCode(err) != 7 || res.Failed != 1 || res.Resume == "" || srv.count("PATCH", "/shops/9000001/listings/1000000001") != 1 || srv.count("PATCH", "/shops/9000001/listings/1000000002") != 0 {
		t.Fatalf("%v %s %v", err, out, srv.all())
	}
	db, err := store.OpenReadOnly(path)
	s2Must(t, err)
	status, err := db.BulkJobStatus(res.Job)
	s2Must(t, err)
	s2Must(t, db.Close())
	if status == "done" {
		t.Fatal("quota job finished")
	}
	quota = false
	_, _, err = s2Run(t, append(args, "--resume", res.Job, "--ignore-budget")...)
	s2Must(t, err)
}
