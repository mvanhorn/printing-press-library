// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored session-context / live-search CLI tests (synthetic fixtures).

package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type opRecorder struct {
	mu      sync.Mutex
	methods map[string]string
	vars    map[string]map[string]any
	bodies  map[string]map[string]any
	query   map[string]string
}

// routedSameDay serves synthetic GraphQL replies keyed by operationName.
func routedSameDay(t *testing.T, replies map[string]string) *opRecorder {
	t.Helper()
	rec := &opRecorder{methods: map[string]string{}, vars: map[string]map[string]any{}, bodies: map[string]map[string]any{}, query: map[string]string{}}
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
		rec.mu.Lock()
		rec.methods[op] = r.Method
		rec.vars[op] = vars
		rec.bodies[op] = body
		rec.query[op] = r.URL.RawQuery
		rec.mu.Unlock()
		reply, ok := replies[op]
		if !ok {
			reply = `{"errors":[{"message":"unexpected operation"}],"data":null}`
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("COSTCO_SAMEDAY_CONFIG", filepath.Join(dir, "config.toml"))
	t.Setenv("COSTCO_SAMEDAY_BASE_URL", srv.URL)
	t.Setenv("COSTCO_SAMEDAY_ZIP", "")
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "")
	return rec
}

func runRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := RootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(append(args, "--no-cache"))
	err := root.Execute()
	return buf.String(), err
}

var sessionReplies = map[string]string{
	"Geolocation":          `{"data":{"geolocationWithUserLocation":{"geolocation":{"postalCode":"00000","zoneId":"zone-1","coordinates":{"latitude":1,"longitude":2}}}}}`,
	"RetailersZone":        `{"data":{"zoneV2":{"id":"zone-zip"}}}`,
	"ShopCollectionScoped": `{"data":{"shopCollection":{"shops":[{"id":"shop-1","serviceType":"delivery","retailerLocationId":"loc-1"}]}}}`,
	"ActiveCartId":         `{"data":{"shopBasket":{"cartId":"cart-1"}}}`,
}

func withReplies(extra map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range sessionReplies {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func TestZipFlagDrivesShopLookupForSlots(t *testing.T) {
	rec := routedSameDay(t, withReplies(map[string]string{
		"AvailableServices": `{"data":{"checkoutAvailableServices":{"services":[{"cartId":"cart-1"}]}}}`,
	}))
	if out, err := runRoot(t, "slots", "availableservices", "--zip", "98027", "--json"); err != nil {
		t.Fatalf("slots: %v\n%s", err, out)
	}
	if got := rec.vars["ShopCollectionScoped"]["postalCode"]; got != "98027" {
		t.Fatalf("shop lookup ZIP = %v", got)
	}
	v := rec.vars["AvailableServices"]
	loc, _ := v["userLocation"].(map[string]any)
	if v["shopId"] != "shop-1" || v["cartId"] != "cart-1" || loc["postalCode"] != "98027" {
		t.Fatalf("AvailableServices variables = %v", v)
	}
	if _, err := runRoot(t, "slots", "availableservices", "--zip", "6571"); err == nil || ExitCode(err) != 2 {
		t.Fatalf("invalid --zip should be a usage error, got %v", err)
	}
	t.Setenv("COSTCO_SAMEDAY_ZIP", "abc")
	if _, err := runRoot(t, "session", "context"); err == nil || ExitCode(err) != 2 {
		t.Fatalf("invalid COSTCO_SAMEDAY_ZIP should be a usage error, got %v", err)
	}
	if _, err := runRoot(t, "slots", "availableservices", "--zip", "98027", "--json"); err != nil {
		t.Fatalf("valid --zip must override a bad COSTCO_SAMEDAY_ZIP: %v", err)
	}
}

func TestGeneratedReadCommandFailsOnGraphQLErrors(t *testing.T) {
	routedSameDay(t, withReplies(map[string]string{
		"CartTotals": `{"errors":[{"message":"Variable $cartId of type ID! was provided invalid value","extensions":{"value":null}}],"data":null}`,
	}))
	out, err := runRoot(t, "cart", "carttotals", "--json")
	if err == nil {
		t.Fatalf("GraphQL errors exited 0:\n%s", out)
	}
	if ExitCode(err) == 0 || !strings.Contains(err.Error(), "missing cartId") {
		t.Fatalf("err = %v (exit %d)", err, ExitCode(err))
	}
}

func TestLiveSearchUsesPersistedPOSTAndHydratesItems(t *testing.T) {
	search := `{"data":{"searchResultsPlacements":{"placements":[
	  {"content":{"__typename":"AdsSearchDisplayCreativePlacement","placement":{"items":[{"id":"ad-1","name":"Ad"}]}}},
	  {"content":{"__typename":"SearchContentManagementSearchItemGrid","items":[{"id":"item-a","name":"Synthetic A","size":"1 ct","price":{"viewSection":{"priceString":"$1.00"}},"availability":{"available":true}}],"itemIds":["item-a"]}},
	  {"content":{"__typename":"SearchContentManagementSearchItemGrid","itemIds":["item-b","item-c"]}},
	  {"content":{"__typename":"SearchContentManagementSearchItemCarousel","itemIds":["carousel-x"]}}
	]}}}`
	rec := routedSameDay(t, withReplies(map[string]string{
		"SearchResultsPlacements": search,
		"Items":                   `{"data":{"items":[{"id":"item-c","name":"Synthetic C"},{"id":"item-b","name":"Synthetic B"}]}}`,
	}))
	out, err := runRoot(t, "search", "kirkland", "--data-source", "live", "--zip", "98027", "--json")
	if err != nil {
		t.Fatalf("search: %v\n%s", err, out)
	}
	if rec.methods["SearchResultsPlacements"] != http.MethodPost {
		t.Fatalf("search method = %s, want POST", rec.methods["SearchResultsPlacements"])
	}
	body := rec.bodies["SearchResultsPlacements"]
	if _, hasText := body["query"]; hasText {
		t.Fatalf("search sent ad-hoc query text: %v", body)
	}
	ext, _ := body["extensions"].(map[string]any)
	pq, _ := ext["persistedQuery"].(map[string]any)
	if h, _ := pq["sha256Hash"].(string); len(h) != 64 {
		t.Fatalf("search body lacks persisted hash: %v", body)
	}
	if strings.Contains(rec.query["SearchResultsPlacements"], "query=") {
		t.Fatalf("search query leaked into URL: %s", rec.query["SearchResultsPlacements"])
	}
	v := rec.vars["SearchResultsPlacements"]
	if v["query"] != "kirkland" || v["shopId"] != "shop-1" || v["zoneId"] != "zone-zip" || v["postalCode"] != "98027" {
		t.Fatalf("search variables = %v", v)
	}
	ids, _ := rec.vars["Items"]["ids"].([]any)
	if len(ids) != 2 || ids[0] != "item-b" || ids[1] != "item-c" {
		t.Fatalf("Items hydration ids = %v", ids)
	}
	a := strings.Index(out, "Synthetic A")
	b := strings.Index(out, "Synthetic B")
	c := strings.Index(out, "Synthetic C")
	if a < 0 || b < a || c < b || strings.Contains(out, "ad-1") || strings.Contains(out, "carousel-x") {
		t.Fatalf("unexpected search output order/content:\n%s", out)
	}
	if !strings.Contains(out, "$1.00") {
		t.Fatalf("price missing from compact item:\n%s", out)
	}
}

func TestLiveSearchGraphQLErrorsExitNonZero(t *testing.T) {
	routedSameDay(t, withReplies(map[string]string{
		"SearchResultsPlacements": `{"errors":[{"message":"Variable $zoneId of type ID! was provided invalid value","extensions":{"value":null}}]}`,
	}))
	_, err := runRoot(t, "search", "kirkland", "--data-source", "live", "--zip", "98027")
	if err == nil || ExitCode(err) == 0 || !strings.Contains(err.Error(), "missing zoneId") {
		t.Fatalf("err = %v", err)
	}
}

func TestUserLocationPostalSelectsShopNotSavedZip(t *testing.T) {
	rec := routedSameDay(t, withReplies(map[string]string{
		"AvailableServices": `{"data":{"checkoutAvailableServices":{"services":[{"cartId":"cart-1"}]}}}`,
	}))
	if out, err := runRoot(t, "slots", "availableservices", "--zip", "98027", "--json"); err != nil {
		t.Fatalf("seed zip: %v\n%s", err, out)
	}
	if out, err := runRoot(t, "slots", "availableservices", "--user-location", `{"postalCode":"10001"}`, "--json"); err != nil {
		t.Fatalf("user-location: %v\n%s", err, out)
	}
	if got := rec.vars["ShopCollectionScoped"]["postalCode"]; got != "10001" {
		t.Fatalf("shop lookup ZIP = %v, want 10001 from --user-location", got)
	}
	if out, err := runRoot(t, "slots", "availableservices", "--json"); err != nil {
		t.Fatalf("saved zip: %v\n%s", err, out)
	}
	if got := rec.vars["ShopCollectionScoped"]["postalCode"]; got != "98027" {
		t.Fatalf("remembered ZIP = %v, want 98027 (request postal must not replace it)", got)
	}
}

func TestExplicitShopResolvesCartWithoutZip(t *testing.T) {
	rec := routedSameDay(t, withReplies(map[string]string{
		"Geolocation": `{"data":null}`,
		"CartTotals":  `{"data":{"cartTotals":{"total":"1.00"}}}`,
	}))
	out, err := runRoot(t, "cart", "carttotals", "--shop-id", "shop-explicit", "--json")
	if err != nil {
		t.Fatalf("carttotals: %v\n%s", err, out)
	}
	if got := rec.vars["ActiveCartId"]["shopId"]; got != "shop-explicit" {
		t.Fatalf("ActiveCartId shopId = %v", got)
	}
	if _, ok := rec.vars["Geolocation"]; ok {
		t.Fatal("Geolocation ran for an explicit shop")
	}
	if _, ok := rec.vars["ShopCollectionScoped"]; ok {
		t.Fatal("shop lookup ran for an explicit shop")
	}
	if v := rec.vars["CartTotals"]; v["shopId"] != "shop-explicit" || v["cartId"] != "cart-1" {
		t.Fatalf("CartTotals variables = %v", v)
	}
}
