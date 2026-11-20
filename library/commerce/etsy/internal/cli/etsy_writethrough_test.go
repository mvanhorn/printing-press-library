package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/config"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

func etsyWriteThroughFixture(t *testing.T) *store.Store {
	t.Helper()
	testenv.Isolate(t)
	t.Setenv("ETSY_SHOP_ID", " 9000001 ")
	t.Setenv(noLocalDataEnvVar, "")
	db, err := store.Open(defaultDBPath("etsy-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func etsyWriteThroughCount(t *testing.T, db *store.Store, resource string, want int) {
	t.Helper()
	got, err := db.Count(resource)
	if err != nil || got != want {
		t.Fatalf("Count(%s) = %d, %v; want %d", resource, got, err, want)
	}
}

func TestEtsyWriteThroughPartitions(t *testing.T) {
	for _, shape := range []string{
		`{"results":[{"listing_id":1000000001,"shop_id":9000001,"title":"Owned mug"},{"listing_id":2000000002,"shop_id":5550002,"title":"Foreign bowl"}]}`,
		`[{"listing_id":1000000001,"shop_id":"9000001"},{"listing_id":2000000002,"shop_id":5550002,"title":"Foreign bowl"}]`,
	} {
		t.Run(shape, func(t *testing.T) {
			db := etsyWriteThroughFixture(t)
			writeThroughCache(context.Background(), "listings", json.RawMessage(shape))
			etsyWriteThroughCount(t, db, "listings", 1)
			etsyWriteThroughCount(t, db, "marketplace_listings", 1)
			if _, err := db.Get("listings", "2000000002"); err == nil {
				t.Fatal("foreign listing in owned partition")
			}
			rows, err := db.Search("Foreign", 10, "listings")
			if err != nil || len(rows) != 0 {
				t.Fatalf("foreign search: %s, %v", rows, err)
			}
			if _, err := db.Get("marketplace_listings", "2000000002"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestEtsyWriteThroughForeignDetail(t *testing.T) {
	db := etsyWriteThroughFixture(t)
	writeThroughCache(context.Background(), "listings", json.RawMessage(`{"listing_id":2000000002,"shop_id":5550002}`))
	etsyWriteThroughCount(t, db, "listings", 0)
	etsyWriteThroughCount(t, db, "marketplace_listings", 1)
}

func TestEtsyWriteThroughPartial(t *testing.T) {
	db := etsyWriteThroughFixture(t)
	_, _, err := db.UpsertBatch("listings", []json.RawMessage{json.RawMessage(`{"listing_id":1000000001,"shop_id":9000001}`), json.RawMessage(`{"listing_id":3000000003,"shop_id":5550002}`)})
	if err != nil {
		t.Fatal(err)
	}
	writeThroughCache(context.Background(), "listings", json.RawMessage(`[{"listing_id":1000000001,"shipping_profile_id":77},{"listing_id":2000000002,"shipping_profile_id":88},{"listing_id":3000000003,"shipping_profile_id":99}]`))
	raw, err := db.Get("listings", "1000000001")
	if err != nil {
		t.Fatal(err)
	}
	obj, err := store.DecodeJSONObject(raw)
	if err != nil || obj["shop_id"] != json.Number("9000001") || obj["shipping_profile_id"] != json.Number("77") {
		t.Fatalf("merged row: %s, %v", raw, err)
	}
	etsyWriteThroughCount(t, db, "marketplace_listings", 2)
	if _, err := db.Get("marketplace_listings", "1000000001"); err == nil {
		t.Fatal("owned partial duplicated in marketplace")
	}
}

func TestEtsyWriteThroughUnsetShop(t *testing.T) {
	db := etsyWriteThroughFixture(t)
	t.Setenv("ETSY_SHOP_ID", "")
	writeThroughCache(context.Background(), "listings", json.RawMessage(`[{"listing_id":1000000001,"shop_id":9000001},{"listing_id":2000000002}]`))
	etsyWriteThroughCount(t, db, "listings", 0)
	etsyWriteThroughCount(t, db, "marketplace_listings", 2)
}

func TestEtsyWriteThroughOtherResource(t *testing.T) {
	db := etsyWriteThroughFixture(t)
	writeThroughCache(context.Background(), "shops", json.RawMessage(`{"shop_id":9000001,"shop_name":"Synthetic"}`))
	etsyWriteThroughCount(t, db, "shops", 1)
}

func TestEtsyWriteThroughAutoResolver(t *testing.T) {
	db := etsyWriteThroughFixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"results":[{"listing_id":2000000002,"shop_id":5550002}]}`)
	}))
	defer srv.Close()
	cfg := &config.Config{BaseURL: srv.URL}
	c := client.New(cfg, time.Second*5, 1000)
	_, _, err := resolveReadWithStrategyAndResponsePath(context.Background(), c, &rootFlags{}, "auto", "listings", true, "/listings/batch", nil, nil, "", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	etsyWriteThroughCount(t, db, "listings", 0)
	etsyWriteThroughCount(t, db, "marketplace_listings", 1)
}
