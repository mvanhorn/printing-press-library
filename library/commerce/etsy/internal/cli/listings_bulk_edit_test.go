package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

const s2Listing1 = `{"listing_id":1000000001,"shop_id":9000001,"state":"active","title":"Hand-glazed ceramic mug",
 "tags":["mug","ceramic","handmade"],"materials":["stoneware"],"quantity":4,
 "price":{"amount":2400,"divisor":100,"currency_code":"USD"},"who_made":"i_did","when_made":"made_to_order","taxonomy_id":1234,
 "description":"A speckled mug.","views":12,"num_favorers":3,"url":"https://example.test/l/1","created_timestamp":1700000000}`
const s2Listing2 = `{"listing_id":1000000002,"shop_id":9000001,"state":"active","title":"Linen apron",
 "tags":["apron","linen"],"materials":["linen"],"quantity":9,"price":{"amount":3800,"divisor":100,"currency_code":"USD"}}`

// s2Inventory is a getListingInventory reply the way Etsy returns it (Money prices).
const s2Inventory = `{"products":[
 {"product_id":4000000001,"sku":"MUG-SPECKLE-S","is_deleted":false,"property_values":[{"property_id":200,"property_name":"Size","scale_id":null,"scale_name":null,"value_ids":[1],"values":["Small"]}],
  "offerings":[{"offering_id":5000000001,"quantity":5,"is_enabled":true,"is_deleted":false,"price":{"amount":2400,"divisor":100,"currency_code":"USD"}}]},
 {"product_id":4000000002,"sku":"MUG-SPECKLE-L","is_deleted":false,"property_values":[{"property_id":200,"property_name":"Size","scale_id":null,"scale_name":null,"value_ids":[2],"values":["Large"]}],
  "offerings":[{"offering_id":5000000002,"quantity":2,"is_enabled":true,"is_deleted":false,"price":{"amount":3000,"divisor":100,"currency_code":"USD"}}]},
 {"product_id":4000000003,"sku":"MUG-OLD","is_deleted":true,"offerings":[{"offering_id":5000000003,"quantity":9,"is_enabled":true,"is_deleted":false,"price":{"amount":100,"divisor":100,"currency_code":"USD"}}]}],
 "price_on_property":[200],"quantity_on_property":[200],"sku_on_property":[200]}`

func s2ShopHandler(failPatchFor *atomic.Value) func(w http.ResponseWriter, r *http.Request, body string) bool {
	return func(w http.ResponseWriter, r *http.Request, body string) bool {
		switch {
		case r.Method == "GET" && r.URL.Path == "/listings/1000000001":
			fmt.Fprint(w, s2Listing1)
		case r.Method == "GET" && r.URL.Path == "/listings/1000000002":
			fmt.Fprint(w, s2Listing2)
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/inventory"):
			fmt.Fprint(w, s2Inventory)
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/personalization"):
			w.WriteHeader(404)
			fmt.Fprint(w, `{"error":"not personalizable"}`)
		case r.Method == "PATCH":
			if failPatchFor != nil {
				if v, _ := failPatchFor.Load().(string); v != "" && strings.HasSuffix(r.URL.Path, "/"+v) {
					w.WriteHeader(400)
					fmt.Fprint(w, `{"error":"title is invalid"}`)
					return true
				}
			}
			id := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			fmt.Fprintf(w, `{"listing_id":%s,"title":"patched","state":"active"}`, id)
		default:
			return false
		}
		return true
	}
}

func s2SeedShop(t *testing.T) string {
	return s2FixtureDB(t, func(db *store.Store) {
		s2Upsert(t, db.UpsertListings, s2Listing1)
		s2Upsert(t, db.UpsertListings, s2Listing2)
		s2Upsert(t, db.UpsertListingImages, `{"listing_image_id":777,"listing_id":1000000001,"alt_text":"old alt","rank":1}`)
	})
}

func s2WritePlan(t *testing.T, js string) string {
	p := filepath.Join(t.TempDir(), "plan.json")
	s2Must(t, os.WriteFile(p, []byte(js), 0o600))
	return p
}

func s2FastThrottle(t *testing.T, d time.Duration) {
	old := bulkMinInterval
	bulkMinInterval = d
	t.Cleanup(func() { bulkMinInterval = old })
}

const s2TagsPlan = `{"version":1,"generated_by":"test","items":[
 {"listing_id":1000000001,"field":"tags","op":"add","value":["gift"]},
 {"listing_id":1000000001,"field":"title","value":"Hand-glazed speckled mug"},
 {"listing_id":1000000002,"field":"tags","op":"set","value":["apron","linen","kitchen"]}]}`

func TestBulkEditPreviewIsZeroHTTPAndShowsDiff(t *testing.T) {
	srv := s2NewServer(t, s2ShopHandler(nil))
	db, plan := s2SeedShop(t), s2WritePlan(t, s2TagsPlan)
	out, _, err := s2Run(t, "listings", "bulk-edit", "--plan", plan, "--db", db, "--json", "--no-cache", "--rate-limit", "1000")
	s2Must(t, err)
	if n := len(srv.all()); n != 0 {
		t.Fatalf("preview made %d HTTP calls: %+v", n, srv.all())
	}
	var res struct {
		Mode     string `json:"mode"`
		Listings int    `json:"listings"`
		Changes  []struct {
			ListingID int64  `json:"listing_id"`
			Field     string `json:"field"`
			Old, New  any
		} `json:"changes"`
		Problems     []any `json:"problems"`
		PlannedCalls int   `json:"planned_calls"`
	}
	s2Decode(t, out, &res)
	if res.Mode != "preview" || res.Listings != 2 || len(res.Changes) != 3 || len(res.Problems) != 0 || res.PlannedCalls == 0 {
		t.Fatalf("preview = %s", out)
	}
	for _, c := range res.Changes {
		if c.ListingID == 1000000001 && c.Field == "tags" {
			if fmt.Sprint(c.Old) != "[mug ceramic handmade]" || fmt.Sprint(c.New) != "[mug ceramic handmade gift]" {
				t.Fatalf("tag diff old=%v new=%v", c.Old, c.New)
			}
		}
		if c.ListingID == 1000000001 && c.Field == "title" && (c.Old != "Hand-glazed ceramic mug" || c.New != "Hand-glazed speckled mug") {
			t.Fatalf("title diff = %+v", c)
		}
	}
}

func TestBulkEditDryRunEnvelopeAndAnnotations(t *testing.T) {
	srv := s2NewServer(t, nil)
	out, _, err := s2Run(t, "listings", "bulk-edit", "--dry-run", "--json", "--plan", s2WritePlan(t, s2TagsPlan), "--db", s2SeedShop(t))
	s2Must(t, err)
	var env map[string]any
	s2Decode(t, out, &env)
	if env["mode"] != "preview" || env["changes"] == nil || len(srv.all()) != 0 {
		t.Fatalf("dry-run = %s", out)
	}
	cmd := newS2ListingsBulkEditCmd(&rootFlags{})
	if cmd.Annotations["pp:preview-happy-path"] != "true" {
		t.Fatalf("annotations = %v", cmd.Annotations)
	}
	if cmd.Flags().Lookup("ignore-budget") == nil {
		t.Fatal("--ignore-budget not registered")
	}
}

func TestBulkEditApplyWithoutYesRefuses(t *testing.T) {
	srv := s2NewServer(t, s2ShopHandler(nil))
	_, _, err := s2Run(t, "listings", "bulk-edit", "--plan", s2WritePlan(t, s2TagsPlan), "--db", s2SeedShop(t), "--apply")
	if err == nil || !strings.Contains(err.Error(), "--yes") || ExitCode(err) != 2 {
		t.Fatalf("err = %v (exit %d)", err, ExitCode(err))
	}
	if len(srv.all()) != 0 {
		t.Fatal("refused apply still called the API")
	}
}

func TestBulkEditValidationNamesListingAndField(t *testing.T) {
	srv := s2NewServer(t, s2ShopHandler(nil))
	tags := make([]string, 14)
	for i := range tags {
		tags[i] = fmt.Sprintf("t%d", i)
	}
	b, _ := json.Marshal(tags)
	plan := s2WritePlan(t, fmt.Sprintf(`{"version":1,"items":[
	 {"listing_id":1000000001,"field":"tags","op":"set","value":%s},
	 {"listing_id":1000000002,"field":"tags","op":"add","value":"this tag is much too long"},
	 {"listing_id":1000000002,"field":"title","value":"%s"}]}`, b, strings.Repeat("T", 141)))
	out, _, err := s2Run(t, "listings", "bulk-edit", "--plan", plan, "--db", s2SeedShop(t), "--apply", "--yes", "--json")
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("err = %v", err)
	}
	var res struct {
		Mode     string `json:"mode"`
		Problems []struct {
			ListingID int64  `json:"listing_id"`
			Field     string `json:"field"`
			Message   string `json:"message"`
		} `json:"problems"`
	}
	s2Decode(t, out, &res)
	if res.Mode != "invalid" || len(res.Problems) != 3 {
		t.Fatalf("problems = %s", out)
	}
	want := map[string]string{"1000000001/tags": "14 tags", "1000000002/tags": "characters", "1000000002/title": "141 characters"}
	for _, p := range res.Problems {
		k := fmt.Sprintf("%d/%s", p.ListingID, p.Field)
		if !strings.Contains(p.Message, want[k]) {
			t.Fatalf("problem %s = %q", k, p.Message)
		}
		delete(want, k)
	}
	if len(want) != 0 || len(srv.all()) != 0 {
		t.Fatalf("missing problems %v, http=%d", want, len(srv.all()))
	}
}

func TestBulkEditApplySnapshotsThenPatchesAndJournals(t *testing.T) {
	s2FastThrottle(t, 5*time.Millisecond)
	srv := s2NewServer(t, s2ShopHandler(nil))
	db := s2SeedShop(t)
	out, _, err := s2Run(t, "listings", "bulk-edit", "--plan", s2WritePlan(t, s2TagsPlan), "--db", db, "--apply", "--yes", "--json", "--no-cache", "--rate-limit", "1000")
	s2Must(t, err)
	var res bulkApplyResult
	s2Decode(t, out, &res)
	// 2 listings: snapshot done x2 + patch x2
	if res.Done != 4 || res.Failed != 0 || res.Calls == 0 {
		t.Fatalf("result = %s", out)
	}
	var patches []s2Req
	firstPatch, lastSnapGet := -1, -1
	for i, r := range srv.all() {
		if r.Method == "PATCH" {
			patches = append(patches, r)
			if firstPatch < 0 {
				firstPatch = i
			}
		}
		if r.Method == "GET" && strings.HasSuffix(r.Path, "/personalization") && firstPatch < 0 {
			lastSnapGet = i
		}
	}
	if len(patches) != 2 {
		t.Fatalf("patches = %+v", patches)
	}
	if lastSnapGet < 0 || lastSnapGet > firstPatch {
		t.Fatalf("snapshot reads (%d) must precede the first write (%d)", lastSnapGet, firstPatch)
	}
	if patches[0].Path != "/shops/9000001/listings/1000000001" {
		t.Fatalf("patch path = %s", patches[0].Path)
	}
	form, err := url.ParseQuery(patches[0].Body)
	s2Must(t, err)
	if form.Get("tags") != "mug,ceramic,handmade,gift" || form.Get("title") != "Hand-glazed speckled mug" {
		t.Fatalf("patch 1 form = %v", form)
	}
	form2, _ := url.ParseQuery(patches[1].Body)
	if form2.Get("tags") != "apron,linen,kitchen" || form2.Get("title") != "" {
		t.Fatalf("patch 2 form = %v", form2)
	}
	// journal + snapshots in the DB
	st, err := store.Open(db)
	s2Must(t, err)
	defer st.Close()
	ops, err := st.BulkJobOps(res.Job)
	s2Must(t, err)
	if len(ops) != 4 {
		t.Fatalf("journal = %+v", ops)
	}
	for _, o := range ops {
		if o.Status != store.BulkStatusDone || o.Attempts != 1 {
			t.Fatalf("op %+v", o)
		}
	}
	snaps, err := st.ListListingSnapshots(1000000001, 10, true)
	s2Must(t, err)
	if len(snaps) != 1 || !strings.Contains(snaps[0].ListingJSON, "Hand-glazed ceramic mug") || !strings.Contains(snaps[0].InventoryJSON, "MUG-SPECKLE-S") {
		t.Fatalf("snapshot = %+v", snaps)
	}
	if strings.Contains(snaps[0].ListingJSON, "speckled mug\"") && strings.Contains(snaps[0].ListingJSON, `"title":"patched"`) {
		t.Fatal("snapshot holds post-write data")
	}
}

func TestBulkEditResumesAfterMidRunFailure(t *testing.T) {
	s2FastThrottle(t, time.Millisecond)
	var failFor atomic.Value
	failFor.Store("1000000002")
	srv := s2NewServer(t, s2ShopHandler(&failFor))
	db, plan := s2SeedShop(t), s2WritePlan(t, s2TagsPlan)
	args := []string{"listings", "bulk-edit", "--plan", plan, "--db", db, "--apply", "--yes", "--json", "--no-cache", "--rate-limit", "1000"}
	out, _, err := s2Run(t, args...)
	if err == nil {
		t.Fatalf("expected failure, got %s", out)
	}
	var first bulkApplyResult
	s2Decode(t, out, &first)
	if first.Failed != 1 || first.Done != 3 || first.Resume == "" {
		t.Fatalf("first run = %s", out)
	}
	if srv.count("PATCH", "/shops/9000001/listings/1000000001") != 1 {
		t.Fatal("listing 1 not patched once")
	}
	failFor.Store("")
	args = append(args, "--resume", first.Job)
	out, _, err = s2Run(t, args...)
	s2Must(t, err)
	var second bulkApplyResult
	s2Decode(t, out, &second)
	if second.Done != 1 || second.SkippedDone != 1 || second.Failed != 0 {
		t.Fatalf("second run = %s", out)
	}
	if n := srv.count("PATCH", "/shops/9000001/listings/1000000001"); n != 1 {
		t.Fatalf("completed listing was redone: %d PATCHes", n)
	}
	if n := srv.count("PATCH", "/shops/9000001/listings/1000000002"); n != 2 {
		t.Fatalf("failed listing PATCHes = %d, want 2 (fail + retry)", n)
	}
	if n := srv.count("GET", "/listings/1000000001"); n != 1 {
		t.Fatalf("completed listing was re-read: %d GETs", n)
	}
	st, err := store.Open(db)
	s2Must(t, err)
	defer st.Close()
	for _, id := range []int64{1000000001, 1000000002} {
		if snaps, _ := st.ListListingSnapshots(id, 10, false); len(snaps) != 1 {
			t.Fatalf("listing %d has %d snapshots, want exactly 1", id, len(snaps))
		}
	}
}

func TestBulkEditThrottleStaysUnderTwoPerSecond(t *testing.T) {
	if bulkDefaultInterval < 500*time.Millisecond {
		t.Fatalf("default interval %v allows more than 2 rps", bulkDefaultInterval)
	}
	s2FastThrottle(t, 150*time.Millisecond)
	srv := s2NewServer(t, s2ShopHandler(nil))
	plan := s2WritePlan(t, `{"version":1,"items":[{"listing_id":1000000001,"field":"title","value":"Speckled mug"}]}`)
	_, _, err := s2Run(t, "listings", "bulk-edit", "--plan", plan, "--db", s2SeedShop(t), "--apply", "--yes", "--json", "--no-cache", "--rate-limit", "1000")
	s2Must(t, err)
	reqs := srv.all()
	if len(reqs) < 4 {
		t.Fatalf("expected GET listing, inventory, personalization and PATCH; got %d", len(reqs))
	}
	for i := 1; i < len(reqs); i++ {
		if gap := reqs[i].At.Sub(reqs[i-1].At); gap < 120*time.Millisecond {
			t.Fatalf("requests %d and %d only %v apart", i-1, i, gap)
		}
	}
}

func TestBulkEditBudgetGuardRefusesAndOverride(t *testing.T) {
	restore, err := cliutil.SetHomeOverride("")
	s2Must(t, err)
	t.Cleanup(restore)
	testenv.Isolate(t, cliutil.StateDir)
	s2FastThrottle(t, time.Millisecond)
	srv := s2NewServer(t, s2ShopHandler(nil))
	h := make(http.Header)
	h.Set("x-limit-per-day", "5000")
	h.Set("x-remaining-today", "2")
	client.ObserveEtsyQuota(h, etsyTestQuotaKey(t))
	db, plan := s2SeedShop(t), s2WritePlan(t, s2TagsPlan)
	_, _, err = s2Run(t, "listings", "bulk-edit", "--plan", plan, "--db", db, "--apply", "--yes", "--json", "--no-cache", "--rate-limit", "1000")
	if err == nil || ExitCode(err) != 7 || !strings.Contains(err.Error(), "planned") {
		t.Fatalf("err = %v (exit %d)", err, ExitCode(err))
	}
	if len(srv.all()) != 0 {
		t.Fatal("budget refusal still called the API")
	}
	// preview reports the budget warning without failing
	out, _, err := s2Run(t, "listings", "bulk-edit", "--plan", plan, "--db", db, "--json")
	s2Must(t, err)
	if !strings.Contains(out, "budget_warning") {
		t.Fatalf("preview missing budget warning: %s", out)
	}
	_, _, err = s2Run(t, "listings", "bulk-edit", "--plan", plan, "--db", db, "--apply", "--yes", "--ignore-budget", "--json", "--no-cache", "--rate-limit", "1000")
	s2Must(t, err)
	if srv.count("PATCH", "/shops/9000001/listings/1000000001") != 1 {
		t.Fatal("--ignore-budget did not run the plan")
	}
}

func TestBulkEditPriceAndAltTextWrites(t *testing.T) {
	s2FastThrottle(t, time.Millisecond)
	srv := s2NewServer(t, s2ShopHandler(nil))
	plan := s2WritePlan(t, `{"version":1,"items":[
	 {"listing_id":1000000001,"field":"price","value":27.5},
	 {"listing_id":1000000001,"field":"alt_text","value":"Speckled mug on a shelf","image_id":777}]}`)
	_, _, err := s2Run(t, "listings", "bulk-edit", "--plan", plan, "--db", s2SeedShop(t), "--apply", "--yes", "--flatten-variation-prices", "--json", "--no-cache", "--rate-limit", "1000")
	s2Must(t, err)
	var put, post *s2Req
	for _, r := range srv.all() {
		r := r
		if r.Method == "PUT" {
			put = &r
		}
		if r.Method == "POST" {
			post = &r
		}
	}
	if put == nil || put.Path != "/listings/1000000001/inventory" {
		t.Fatalf("no inventory PUT: %+v", srv.all())
	}
	var body struct {
		Products []struct {
			SKU       string `json:"sku"`
			Offerings []struct {
				Price    float64 `json:"price"`
				Quantity int     `json:"quantity"`
			} `json:"offerings"`
		} `json:"products"`
	}
	s2Decode(t, put.Body, &body)
	if len(body.Products) != 2 { // the deleted product is not sent back
		t.Fatalf("products = %s", put.Body)
	}
	for _, p := range body.Products {
		if p.Offerings[0].Price != 27.5 {
			t.Fatalf("price not set: %s", put.Body)
		}
	}
	if body.Products[0].Offerings[0].Quantity != 5 || body.Products[1].Offerings[0].Quantity != 2 {
		t.Fatalf("quantities changed: %s", put.Body)
	}
	if post == nil || post.Path != "/shops/9000001/listings/1000000001/images" ||
		!strings.Contains(post.Body, "Speckled mug on a shelf") || !strings.Contains(post.Body, "777") {
		t.Fatalf("alt-text upload = %+v", post)
	}
}

func TestBulkEditFlagModeAndUnsyncedStore(t *testing.T) {
	srv := s2NewServer(t, nil)
	missing := filepath.Join(t.TempDir(), "absent.db")
	out, errOut, err := s2Run(t, "listings", "bulk-edit", "--listings", "1000000005", "--field", "tags", "--add", "linen apron", "--add", "gift", "--db", missing, "--json")
	s2Must(t, err)
	if !strings.Contains(errOut, "sync") {
		t.Fatalf("missing sync hint: %q", errOut)
	}
	var res struct {
		Changes []bulkChange `json:"changes"`
	}
	s2Decode(t, out, &res)
	if len(res.Changes) != 1 || res.Changes[0].Status != bulkStatusUnresolved || res.Changes[0].New != nil {
		t.Fatalf("changes = %s", out)
	}
	if len(srv.all()) != 0 {
		t.Fatal("preview called the API")
	}
	_, _, err = s2Run(t, "listings", "bulk-edit", "--listings", "1", "--field", "tags", "--db", missing)
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("flag mode without --set/--add/--remove: %v", err)
	}
}
