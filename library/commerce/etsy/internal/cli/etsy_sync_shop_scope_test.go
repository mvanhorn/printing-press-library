package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

type shopScopeSyncClient struct {
	url, shopID string
	client      *http.Client
	paths       []string
}

func (c *shopScopeSyncClient) RateLimit() float64 { return 1000 }
func (c *shopScopeSyncClient) CurrentTemplateVars() map[string]string {
	return map[string]string{"shop_id": c.shopID}
}
func (c *shopScopeSyncClient) Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error) {
	path = strings.ReplaceAll(path, "{shop_id}", c.shopID)
	c.paths = append(c.paths, path)
	req, err := http.NewRequestWithContext(ctx, "GET", c.url+path, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return io.ReadAll(res.Body)
}
func shopScopeFixture(t *testing.T) (*shopScopeSyncClient, *store.Store) {
	t.Helper()
	testenv.Isolate(t)
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	s2Must(t, err)
	t.Cleanup(func() { db.Close() })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/listings/active") {
			t.Errorf("marketplace request: %s", r.URL.Path)
		}
		switch r.URL.Path {
		case "/shops/9000001/listings":
			io.WriteString(w, `{"results":[{"listing_id":1000000001,"state":"active","title":"Synthetic mug"},{"listing_id":1000000002,"state":"active","title":"Synthetic bowl"}],"count":2}`)
		case "/listings/1000000001/inventory", "/listings/1000000002/inventory":
			io.WriteString(w, `{"products":[{"product_id":7000000001,"sku":"SYNTHETIC","offerings":[{"offering_id":8000000001,"quantity":2,"is_enabled":true,"price":{"amount":1000,"divisor":100,"currency_code":"USD"}}]}],"price_on_property":[],"quantity_on_property":[],"sku_on_property":[]}`)
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			io.WriteString(w, `{"results":[],"count":0}`)
		}
	}))
	t.Cleanup(srv.Close)
	return &shopScopeSyncClient{url: srv.URL, shopID: "9000001", client: srv.Client()}, db
}
func shopScopeDep(t *testing.T, name string) dependentResourceDef {
	t.Helper()
	for _, d := range dependentResourceDefs() {
		if d.Name == name {
			return d
		}
	}
	t.Fatalf("missing dependent %s", name)
	return dependentResourceDef{}
}
func TestSyncListingsUsesShopPath(t *testing.T) {
	c, db := shopScopeFixture(t)
	path, err := syncResourcePath("listings")
	s2Must(t, err)
	if path != "/shops/{shop_id}/listings" {
		t.Fatalf("path: %s", path)
	}
	r := syncResource(context.Background(), c, db, "listings", "", true, 0, false, false, nil, nil)
	if r.Err != nil || r.Warn != nil {
		t.Fatalf("result: %+v", r)
	}
	n, err := db.Count("listings")
	s2Must(t, err)
	if n != 2 {
		t.Fatalf("rows: %d", n)
	}
	if len(c.paths) != 5 {
		t.Fatalf("paths: %v", c.paths)
	}
	for _, p := range c.paths {
		if p != "/shops/9000001/listings" {
			t.Fatalf("path: %s", p)
		}
	}
}
func TestSyncDependentsIterateShopListingsOnly(t *testing.T) {
	c, db := shopScopeFixture(t)
	r := syncResource(context.Background(), c, db, "listings", "", true, 0, false, false, nil, nil)
	if r.Err != nil || r.Warn != nil {
		t.Fatalf("result: %+v", r)
	}
	r = syncDependentResource(context.Background(), c, db, shopScopeDep(t, "listings_inventory"), "", true, 0, false, false, nil, nil, 1)
	if r.Err != nil {
		t.Fatalf("result: %+v", r)
	}
	counts := map[string]int{}
	for _, p := range c.paths {
		counts[p]++
		if strings.HasPrefix(p, "/listings/active") {
			t.Fatalf("marketplace: %s", p)
		}
	}
	if len(c.paths) != 7 || counts["/listings/1000000001/inventory"] != 1 || counts["/listings/1000000002/inventory"] != 1 {
		t.Fatalf("paths: %v", c.paths)
	}
}
func TestDefaultSyncResourcesNeverUnscopedMarketplace(t *testing.T) {
	allowed := map[string]bool{"/buyer-taxonomy/nodes": true, "/seller-taxonomy/nodes": true, "/user/addresses": true}
	for _, name := range defaultSyncResources() {
		p, err := syncResourcePath(name)
		s2Must(t, err)
		if p == "/listings/active" || (!strings.Contains(p, "{shop_id}") && !allowed[p]) {
			t.Errorf("unscoped %s: %s", name, p)
		}
	}
}
func TestSyncSkipsShopResourcesWithoutShopID(t *testing.T) {
	c, db := shopScopeFixture(t)
	c.shopID = ""
	old := humanFriendly
	humanFriendly = false
	defer func() { humanFriendly = old }()
	var events bytes.Buffer
	for _, name := range []string{"listings", "receipts"} {
		r := syncResource(context.Background(), c, db, name, "", true, 0, false, false, nil, &events)
		if r.Warn == nil || r.Err != nil {
			t.Errorf("%s: %+v", name, r)
		}
	}
	r := syncDependentResource(context.Background(), c, db, shopScopeDep(t, "listings_transactions"), "", true, 0, false, false, nil, &events, 1)
	if r.Warn == nil || r.Err != nil {
		t.Errorf("dependent: %+v", r)
	}
	if len(c.paths) != 0 {
		t.Fatalf("requests: %v", c.paths)
	}
	if strings.Count(events.String(), `"reason":"missing_shop_id"`) != 3 || !strings.Contains(events.String(), "ETSY_SHOP_ID") {
		t.Fatalf("events: %s", events.String())
	}
}
