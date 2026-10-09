// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored session-context tests (synthetic fixtures, httptest only).

package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/costco-sameday/internal/config"
)

// fakeSameDay routes GraphQL requests by operationName and records the
// variables each operation received.
type fakeSameDay struct {
	mu      sync.Mutex
	calls   map[string]int
	vars    map[string]map[string]any
	methods map[string]string
	bodies  map[string]map[string]any
	replies map[string]string
	status  map[string]int
}

func newFakeSameDay(t *testing.T) (*fakeSameDay, *Client) {
	t.Helper()
	f := &fakeSameDay{
		calls: map[string]int{}, vars: map[string]map[string]any{}, methods: map[string]string{},
		bodies: map[string]map[string]any{}, status: map[string]int{},
		replies: map[string]string{
			"Geolocation":          `{"data":{"geolocationWithUserLocation":{"geolocation":{"postalCode":"99999","zoneId":"zone-geo","coordinates":{"latitude":1.5,"longitude":-2.5}}}}}`,
			"RetailersZone":        `{"data":{"zoneV2":{"id":"zone-zip"}}}`,
			"ShopCollectionScoped": `{"data":{"shopCollection":{"shops":[{"id":"shop-pickup","serviceType":"pickup","retailerLocationId":"loc-p"},{"id":"shop-delivery","serviceType":"delivery","retailerLocationId":"loc-d"}]}}}`,
			"ActiveCartId":         `{"data":{"shopBasket":{"cartId":"cart-1"}}}`,
			"PersonalActiveCarts":  `{"data":{"userCarts":{"carts":[{"id":"cart-other","retailer":{"slug":"other"}},{"id":"cart-costco","retailer":{"slug":"costco"}}]}}}`,
			"CartTotals":           `{"data":{"cartTotals":{"total":"1.00"}}}`,
			"AvailableServices":    `{"data":{"checkoutAvailableServices":{"services":[]}}}`,
			"CurrentRetailer":      `{"data":{"userCart":{"retailer":{"slug":"costco"}}}}`,
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		op := r.URL.Query().Get("operationName")
		vars := map[string]any{}
		_ = json.Unmarshal([]byte(r.URL.Query().Get("variables")), &vars)
		var body map[string]any
		if r.Method == http.MethodPost {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &body)
			if name, _ := body["operationName"].(string); name != "" {
				op = name
			}
			if v, ok := body["variables"].(map[string]any); ok {
				vars = v
			}
		}
		f.mu.Lock()
		f.calls[op]++
		f.vars[op] = vars
		f.methods[op] = r.Method
		f.bodies[op] = body
		reply, ok := f.replies[op]
		status := f.status[op]
		f.mu.Unlock()
		if !ok {
			reply = `{"errors":[{"message":"unknown op"}],"data":null}`
		}
		if status == 0 {
			status = http.StatusOK
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv(SessionZipEnv, "")
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "")
	c := New(&config.Config{BaseURL: srv.URL}, 5*time.Second, 0)
	c.NoCache = true
	return f, c
}

func (f *fakeSameDay) count(op string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[op]
}

func (f *fakeSameDay) varsOf(op string) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.vars[op]
}

func TestSessionContextFillsMissingShopAndCart(t *testing.T) {
	f, c := newFakeSameDay(t)
	c.SetSessionPostalCode("98027")
	if _, err := c.GetMutating(context.Background(), "/graphql", map[string]string{"operationName": "CartTotals"}); err != nil {
		t.Fatalf("CartTotals: %v", err)
	}
	got := f.varsOf("CartTotals")
	if got["shopId"] != "shop-delivery" || got["cartId"] != "cart-1" {
		t.Fatalf("CartTotals variables = %v, want shop-delivery/cart-1", got)
	}
	if zip := f.varsOf("ShopCollectionScoped")["postalCode"]; zip != "98027" {
		t.Fatalf("shop lookup postalCode = %v, want explicit --zip 98027", zip)
	}
	if slug := f.varsOf("ShopCollectionScoped")["retailerSlug"]; slug != "costco" {
		t.Fatalf("retailerSlug = %v", slug)
	}
	// Geolocation coordinates belong to ZIP 99999, so they must not be sent for 98027.
	coords, _ := f.varsOf("ShopCollectionScoped")["coordinates"].(map[string]any)
	if coords["latitude"] != float64(0) || coords["longitude"] != float64(0) {
		t.Fatalf("coordinates for a different ZIP leaked: %v", coords)
	}
}

func TestSessionContextExplicitFlagsWin(t *testing.T) {
	f, c := newFakeSameDay(t)
	if _, err := c.GetMutating(context.Background(), "/graphql", map[string]string{
		"operationName": "CartTotals", "shopId": "shop-explicit", "cartId": "cart-explicit",
	}); err != nil {
		t.Fatalf("CartTotals: %v", err)
	}
	got := f.varsOf("CartTotals")
	if got["shopId"] != "shop-explicit" || got["cartId"] != "cart-explicit" {
		t.Fatalf("explicit flags overridden: %v", got)
	}
	for _, op := range []string{"Geolocation", "ShopCollectionScoped", "ActiveCartId"} {
		if n := f.count(op); n != 0 {
			t.Fatalf("%s called %d times despite explicit flags", op, n)
		}
	}
}

func TestSessionContextCachesShopAndRemembersZip(t *testing.T) {
	f, c := newFakeSameDay(t)
	c.SetSessionPostalCode("98027")
	c.NoCache = false
	ctx := context.Background()
	if _, err := c.ResolveSessionContext(ctx, SessionNeeds{Shop: true, Zone: true, Location: true}); err != nil {
		t.Fatal(err)
	}
	// A new client with no --zip uses the remembered ZIP and the cached shop.
	c2 := New(&config.Config{BaseURL: c.BaseURL}, 5*time.Second, 0)
	sc, err := c2.ResolveSessionContext(ctx, SessionNeeds{Shop: true, Zone: true, Location: true})
	if err != nil {
		t.Fatal(err)
	}
	if sc.PostalCode != "98027" || sc.PostalCodeSource != "saved" || sc.ShopID != "shop-delivery" || sc.ZoneID != "zone-zip" {
		t.Fatalf("resolved = %+v", sc)
	}
	if n := f.count("ShopCollectionScoped"); n != 1 {
		t.Fatalf("ShopCollectionScoped called %d times, want 1 (cached)", n)
	}
	if n := f.count("RetailersZone"); n != 1 {
		t.Fatalf("RetailersZone called %d times, want 1 (cached)", n)
	}
	// --refresh bypasses the cache.
	if _, err := c2.ResolveSessionContext(ctx, SessionNeeds{Shop: true, Refresh: true}); err != nil {
		t.Fatal(err)
	}
	if n := f.count("ShopCollectionScoped"); n != 2 {
		t.Fatalf("refresh did not re-resolve: %d calls", n)
	}
	info, err := os.Stat(c2.sessionCachePath())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("session cache mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestSessionContextZipFallbacks(t *testing.T) {
	f, c := newFakeSameDay(t)
	sc, err := c.ResolveSessionContext(context.Background(), SessionNeeds{Shop: true, Zone: true})
	if err != nil {
		t.Fatal(err)
	}
	if sc.PostalCode != "99999" || sc.PostalCodeSource != "geolocation" || sc.ZoneID != "zone-geo" {
		t.Fatalf("geolocation fallback = %+v", sc)
	}
	if n := f.count("RetailersZone"); n != 0 {
		t.Fatalf("RetailersZone called %d times for a matching geolocation ZIP", n)
	}
	coords, _ := f.varsOf("ShopCollectionScoped")["coordinates"].(map[string]any)
	if coords["latitude"] != 1.5 {
		t.Fatalf("matching-ZIP coordinates not sent: %v", coords)
	}
	t.Setenv(SessionZipEnv, "12345")
	c2 := New(&config.Config{BaseURL: c.BaseURL}, 5*time.Second, 0)
	c2.NoCache = true
	sc, err = c2.ResolveSessionContext(context.Background(), SessionNeeds{Shop: true})
	if err != nil || sc.PostalCode != "12345" || sc.PostalCodeSource != "env" {
		t.Fatalf("env ZIP = %+v err=%v", sc, err)
	}
	c2.SetSessionPostalCode("abc")
	if _, err := c2.ResolveSessionContext(context.Background(), SessionNeeds{Shop: true}); err == nil || !strings.Contains(err.Error(), "5-digit US ZIP") {
		t.Fatalf("invalid ZIP not rejected: %v", err)
	}
}

func TestSessionContextCartFallsBackToPersonalActiveCarts(t *testing.T) {
	f, c := newFakeSameDay(t)
	f.replies["ActiveCartId"] = `{"data":{"shopBasket":null}}`
	sc, err := c.ResolveSessionContext(context.Background(), SessionNeeds{Cart: true})
	if err != nil {
		t.Fatal(err)
	}
	if sc.CartID != "cart-costco" {
		t.Fatalf("cartId = %q, want Costco cart from PersonalActiveCarts", sc.CartID)
	}
}

func TestSessionContextUserLocationForAvailableServices(t *testing.T) {
	f, c := newFakeSameDay(t)
	c.SetSessionPostalCode("98027")
	if _, err := c.GetMutating(context.Background(), "/graphql", map[string]string{"operationName": "AvailableServices"}); err != nil {
		t.Fatal(err)
	}
	got := f.varsOf("AvailableServices")
	loc, _ := got["userLocation"].(map[string]any)
	if loc["postalCode"] != "98027" || got["shopId"] != "shop-delivery" || got["cartId"] != "cart-1" {
		t.Fatalf("AvailableServices variables = %v", got)
	}
}

func TestSessionContextNotUsedInVerifyOrDryRun(t *testing.T) {
	f, c := newFakeSameDay(t)
	c.DryRun = true
	_, _ = c.GetMutating(context.Background(), "/graphql", map[string]string{"operationName": "CartTotals"})
	c.DryRun = false
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")
	_, _ = c.Get(context.Background(), "/graphql", map[string]string{"operationName": "CurrentRetailer"})
	if n := f.count("Geolocation") + f.count("ShopCollectionScoped") + f.count("ActiveCartId"); n != 0 {
		t.Fatalf("resolver ran %d lookups under dry-run/verify", n)
	}
}

func TestGraphQLErrorsWithoutDataFail(t *testing.T) {
	f, c := newFakeSameDay(t)
	f.replies["CurrentRetailer"] = `{"errors":[{"message":"Variable $cartId of type ID! was provided invalid value","extensions":{"value":null,"problems":[{"path":[],"explanation":"Expected value to not be null"}]}}]}`
	f.replies["ActiveCartId"] = `{"errors":[{"message":"Not Authenticated"}],"data":null}`
	f.replies["PersonalActiveCarts"] = `{"errors":[{"message":"Not Authenticated"}],"data":null}`
	_, err := c.Get(context.Background(), "/graphql", map[string]string{"operationName": "CurrentRetailer"})
	var ge *GraphQLError
	if !errors.As(err, &ge) {
		t.Fatalf("want *GraphQLError, got %v", err)
	}
	if len(ge.Missing) != 1 || ge.Missing[0] != "cartId" || ge.ResolveErr == nil {
		t.Fatalf("GraphQLError = %+v", ge)
	}
	msg := err.Error()
	for _, want := range []string{"missing cartId", "session context", "auto-resolve"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q lacks %q", msg, want)
		}
	}
}

func TestGraphQLResponseErrorClassification(t *testing.T) {
	cases := []struct {
		name string
		body string
		fail bool
	}{
		{"data ok", `{"data":{"x":1}}`, false},
		{"partial data", `{"data":{"x":1,"y":null},"errors":[{"message":"y failed"}]}`, false},
		{"errors null data", `{"data":null,"errors":[{"message":"boom"}]}`, true},
		{"errors all-null data", `{"data":{"x":null},"errors":[{"message":"boom"}]}`, true},
		{"errors no data", `{"errors":[{"message":"boom"}]}`, true},
		{"verify synthetic", `{"__pp_verify_synthetic__":true,"errors":[{"message":"x"}]}`, false},
	}
	for _, tc := range cases {
		err := graphQLResponseError("Op", []byte(tc.body), nil)
		if (err != nil) != tc.fail {
			t.Fatalf("%s: err=%v, want fail=%v", tc.name, err, tc.fail)
		}
	}
	err := graphQLResponseError("Items", []byte(`{"errors":[{"message":"Variable $ids of type [ID!]! was provided invalid value","extensions":{"value":null}}]}`), nil)
	if err == nil || !strings.Contains(err.Error(), "missing ids") || !strings.Contains(err.Error(), "--ids") {
		t.Fatalf("generic hint missing: %v", err)
	}
}

func TestPersistedQueryHashesMergeSeedWithRegistry(t *testing.T) {
	_, c := newFakeSameDay(t)
	path := c.persistedQueryRegistryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`[{"operation_name":"CartTotals","hash":"override"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	h := c.persistedQueryHashes()
	if h["CartTotals"] != "override" {
		t.Fatalf("registry override lost: %q", h["CartTotals"])
	}
	for _, op := range []string{geolocationOperation, retailersZoneOperation, shopCollectionScopedOperation, activeCartIDOperation, "SearchResultsPlacements", "Items", finalizeCheckoutOperation} {
		if h[op] == "" {
			t.Fatalf("seed op %s hidden by user registry", op)
		}
	}
}

func TestNewPageViewIDIsUUIDv4(t *testing.T) {
	id := NewPageViewID()
	if len(id) != 36 || id[14] != '4' || strings.Count(id, "-") != 4 {
		t.Fatalf("pageViewId %q is not a UUIDv4", id)
	}
}

func TestSessionCartCacheDroppedAfterMutation(t *testing.T) {
	f, c := newFakeSameDay(t)
	c.NoCache = false
	ctx := context.Background()
	if _, err := c.ResolveSessionContext(ctx, SessionNeeds{Cart: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ResolveSessionContext(ctx, SessionNeeds{Cart: true}); err != nil {
		t.Fatal(err)
	}
	if n := f.count("ActiveCartId"); n != 1 {
		t.Fatalf("cart not cached: %d lookups", n)
	}
	f.replies["UpdateCartItemsMutation"] = `{"data":{"updateCartItems":{"cart":{"id":"cart-1"}}}}`
	if _, _, err := c.PostWithParams(ctx, "/graphql", nil, map[string]any{"operationName": "UpdateCartItemsMutation", "variables": map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ResolveSessionContext(ctx, SessionNeeds{Cart: true}); err != nil {
		t.Fatal(err)
	}
	if n := f.count("ActiveCartId"); n != 2 {
		t.Fatalf("cart cache survived a mutation: %d lookups", n)
	}
}

func TestSessionContextCartResolvedForExplicitShop(t *testing.T) {
	f, c := newFakeSameDay(t)
	c.SetSessionPostalCode("98027")
	if _, err := c.GetMutating(context.Background(), "/graphql", map[string]string{"operationName": "CartTotals", "shopId": "shop-explicit"}); err != nil {
		t.Fatal(err)
	}
	if got := f.varsOf("ActiveCartId")["shopId"]; got != "shop-explicit" {
		t.Fatalf("cart resolved for shop %v, want the explicit shop", got)
	}
	if n := f.count("ShopCollectionScoped"); n != 0 {
		t.Fatalf("ZIP shop lookup ran despite explicit --shop-id (%d)", n)
	}
	if got := f.varsOf("CartTotals"); got["shopId"] != "shop-explicit" || got["cartId"] != "cart-1" {
		t.Fatalf("CartTotals variables = %v", got)
	}
}

func TestSessionCacheUpdatesMergeAcrossClients(t *testing.T) {
	_, c := newFakeSameDay(t)
	c2 := New(&config.Config{BaseURL: c.BaseURL}, 5*time.Second, 0)
	c.updateSessionCache(func(cc *sessionContextCache) {
		cc.Shops = map[string]cachedShop{"11111": {ShopID: "a", ResolvedAt: time.Now()}}
	})
	c2.updateSessionCache(func(cc *sessionContextCache) {
		if cc.Shops == nil {
			cc.Shops = map[string]cachedShop{}
		}
		cc.Shops["22222"] = cachedShop{ShopID: "b", ResolvedAt: time.Now()}
	})
	got := c.loadSessionCache().Shops
	if got["11111"].ShopID != "a" || got["22222"].ShopID != "b" {
		t.Fatalf("concurrent updates not merged: %v", got)
	}
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(c.sessionCachePath()), "*.tmp"))
	if len(matches) != 0 {
		t.Fatalf("temp files left behind: %v", matches)
	}
}

func TestZoneFollowsSelectedZipNotAccountZone(t *testing.T) {
	f, c := newFakeSameDay(t)
	c.SetSessionPostalCode("98027")
	f.replies["Items"] = `{"data":{"items":[]}}`
	if _, err := c.Get(context.Background(), "/graphql", map[string]string{"operationName": "Items"}); err != nil {
		t.Fatal(err)
	}
	got := f.varsOf("Items")
	if got["postalCode"] != "98027" || got["zoneId"] != "zone-zip" || got["shopId"] != "shop-delivery" {
		t.Fatalf("Items variables = %v", got)
	}
	if zip := f.varsOf("RetailersZone")["postalCode"]; zip != "98027" {
		t.Fatalf("RetailersZone postalCode = %v", zip)
	}
	if n := f.count("RetailersZone"); n != 1 {
		t.Fatalf("RetailersZone called %d times", n)
	}
	coords, _ := f.varsOf("ShopCollectionScoped")["coordinates"].(map[string]any)
	if coords["latitude"] != float64(0) || coords["longitude"] != float64(0) {
		t.Fatalf("coordinates for a different ZIP leaked: %v", coords)
	}
}

func TestSessionContextRequestPostalOverridesSavedZip(t *testing.T) {
	f, c := newFakeSameDay(t)
	c.NoCache = false
	ctx := context.Background()
	c.SetSessionPostalCode("98027")
	if _, err := c.ResolveSessionContext(ctx, SessionNeeds{Shop: true}); err != nil {
		t.Fatal(err)
	}
	c.SetSessionPostalCode("")
	c.NoCache = true

	cases := []map[string]string{
		{"operationName": "AvailableServices", "userLocation": `{"postalCode":"10001"}`},
		{"operationName": "Items", "variables": `{"postalCode":"10001"}`},
		{"operationName": "Items", "postalCode": "10001"},
	}
	f.replies["Items"] = `{"data":{"items":[]}}`
	for _, params := range cases {
		before := f.count("ShopCollectionScoped")
		if _, err := c.Get(ctx, "/graphql", params); err != nil {
			t.Fatalf("%v: %v", params, err)
		}
		if zip := f.varsOf("ShopCollectionScoped")["postalCode"]; zip != "10001" {
			t.Fatalf("%v shop lookup postalCode = %v", params, zip)
		}
		if f.count("ShopCollectionScoped") != before+1 {
			t.Fatalf("%v did not look up the request ZIP", params)
		}
	}

	c2 := New(&config.Config{BaseURL: c.BaseURL}, 5*time.Second, 0)
	sc, err := c2.ResolveSessionContext(ctx, SessionNeeds{Shop: true})
	if err != nil {
		t.Fatal(err)
	}
	if sc.PostalCode != "98027" || sc.PostalCodeSource != "saved" {
		t.Fatalf("request ZIP was remembered: %+v", sc)
	}
}

func TestSessionContextInvalidRequestPostalDoesNotUseSavedZip(t *testing.T) {
	f, c := newFakeSameDay(t)
	c.SetSessionPostalCode("98027")
	_, _ = c.Get(context.Background(), "/graphql", map[string]string{
		"operationName": "AvailableServices",
		"userLocation":  `{"postalCode":"abc"}`,
	})
	if n := f.count("ShopCollectionScoped"); n != 0 {
		t.Fatalf("invalid request ZIP fell through to a shop lookup (%d)", n)
	}
}

func TestExplicitShopCartDoesNotNeedLocation(t *testing.T) {
	f, c := newFakeSameDay(t)
	f.replies["Geolocation"] = `{"data":null,"errors":[{"message":"no location"}]}`
	sc, err := c.ResolveSessionContext(context.Background(), SessionNeeds{Shop: true, Cart: true, ShopID: "shop-explicit"})
	if err != nil {
		t.Fatal(err)
	}
	if sc.ShopID != "shop-explicit" || sc.CartID != "cart-1" {
		t.Fatalf("resolved = %+v", sc)
	}
	if n := f.count("Geolocation"); n != 0 {
		t.Fatalf("Geolocation called %d times", n)
	}
	if n := f.count("ShopCollectionScoped"); n != 0 {
		t.Fatalf("ShopCollectionScoped called %d times", n)
	}
	if got := f.varsOf("ActiveCartId")["shopId"]; got != "shop-explicit" {
		t.Fatalf("ActiveCartId shopId = %v", got)
	}

	if _, err := c.Get(context.Background(), "/graphql", map[string]string{
		"operationName": "CartTotals", "shopId": "shop-explicit",
	}); err != nil {
		t.Fatal(err)
	}
	if got := f.varsOf("CartTotals"); got["shopId"] != "shop-explicit" || got["cartId"] != "cart-1" {
		t.Fatalf("CartTotals variables = %v", got)
	}
	if n := f.count("Geolocation"); n != 0 {
		t.Fatalf("Geolocation ran while filling cart for an explicit shop (%d)", n)
	}
}

func TestSessionCacheParallelWritesKeepEveryKey(t *testing.T) {
	_, c := newFakeSameDay(t)
	const n = 16
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			c.updateSessionCache(func(cc *sessionContextCache) {
				if cc.Shops == nil {
					cc.Shops = map[string]cachedShop{}
				}
				time.Sleep(5 * time.Millisecond)
				cc.Shops[fmt.Sprintf("%05d", i)] = cachedShop{ShopID: fmt.Sprintf("shop-%d", i), ResolvedAt: time.Now()}
			})
		}()
	}
	wg.Wait()
	got := c.loadSessionCache().Shops
	if len(got) != n {
		t.Fatalf("cached shops = %d, want %d (%v)", len(got), n, got)
	}
	if runtime.GOOS != "windows" {
		if _, err := os.Stat(c.sessionCachePath() + ".lock"); err != nil {
			t.Fatal(err)
		}
	}
}
