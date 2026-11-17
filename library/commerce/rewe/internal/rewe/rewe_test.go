package rewe

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/rewe/internal/cliutil"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseOffersPage(t *testing.T) {
	rendered, pending, err := ParseOffersPage(fixture(t, "offers.html"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rendered) == 0 {
		t.Fatal("no rendered offers")
	}
	for _, o := range rendered {
		if o.Title == "" || o.NAN == "" || o.Week == "" || o.PriceCents <= 0 || o.Category == "" {
			t.Errorf("incomplete offer: %+v", o)
		}
	}
	for _, p := range pending {
		if p.NAN == "" || p.Week == "" {
			t.Errorf("incomplete placeholder: %+v", p)
		}
	}
	seen := map[string]bool{}
	for _, o := range rendered {
		key := o.NAN + o.Week
		if seen[key] {
			t.Errorf("duplicate offer %s", key)
		}
		seen[key] = true
	}
	for _, p := range pending {
		if seen[p.NAN+p.Week] {
			t.Errorf("offer %s both rendered and pending", p.NAN)
		}
	}
}

func TestParseTileHTML(t *testing.T) {
	var answers []struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(fixture(t, "frontend-include.json")), &answers); err != nil {
		t.Fatal(err)
	}
	o, err := ParseTileHTML(answers[0].Content)
	if err != nil {
		t.Fatal(err)
	}
	if o.Title == "" || o.NAN == "" || o.PriceCents <= 0 {
		t.Fatalf("tile not parsed: %+v", o)
	}
	if _, err := ParseTileHTML("<div>nothing</div>"); err == nil {
		t.Fatal("expected error for fragment without tile")
	}
}

func TestPriceToCents(t *testing.T) {
	for in, want := range map[string]int{"0,99 €": 99, "12,49 €": 1249, "abc": 0, "": 0, "1 kg = 3,96 €": 396} {
		if got := PriceToCents(in); got != want {
			t.Errorf("PriceToCents(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParseRecipePage(t *testing.T) {
	r, err := ParseRecipePage(fixture(t, "recipe.html"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Name != "Gemüse-Lasagne" {
		t.Errorf("name = %q", r.Name)
	}
	if len(r.Ingredients) <= 5 {
		t.Errorf("only %d ingredients", len(r.Ingredients))
	}
	var node map[string]any
	if err := json.Unmarshal(r.SchemaJSON, &node); err != nil {
		t.Fatal(err)
	}
	if node["@type"] != "Recipe" {
		t.Errorf("schema @type = %v, want Recipe", node["@type"])
	}
	if _, ok := node["image"].([]any); !ok {
		t.Errorf("image must be a JSON array for the mapper, got %T", node["image"])
	}
	if _, err := ParseRecipePage("<html></html>"); err == nil {
		t.Error("expected error for page without schema")
	}
}

func TestRecipeSlug(t *testing.T) {
	for in, want := range map[string]string{
		"gemuese-lasagne":                              "gemuese-lasagne",
		"/rezepte/gemuese-lasagne/":                    "gemuese-lasagne",
		"https://www.rewe.de/rezepte/gemuese-lasagne/": "gemuese-lasagne",
	} {
		if got := RecipeSlug(in); got != want {
			t.Errorf("RecipeSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseProducts(t *testing.T) {
	ps, err := ParseProducts([]byte(fixture(t, "products_market.json")))
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) == 0 {
		t.Fatal("no products")
	}
	for _, p := range ps {
		if p.ListingID == "" || p.PriceCents <= 0 || p.Name == "" {
			t.Errorf("incomplete product %+v", p)
		}
	}
	// Without a selected market REWE returns products without listings:
	// nothing is orderable, so BestMatch must refuse instead of guessing.
	nm, err := ParseProducts([]byte(fixture(t, "products_nomarket.json")))
	if err != nil || len(nm) == 0 {
		t.Fatalf("no-market fixture: %v %d", err, len(nm))
	}
	for _, p := range nm {
		if p.ListingID != "" {
			t.Errorf("unexpected listing without market: %+v", p)
		}
	}
	if _, ok := BestMatch("hafermilch", nm); ok {
		t.Error("BestMatch must not pick an unorderable product")
	}
}

func TestBestMatch(t *testing.T) {
	ps := []Product{
		{Name: "Milka Alpenmilch Schokolade", ListingID: "a", PriceCents: 129},
		{Name: "Hafer Haferflocken ohne Listing", ListingID: ""},
		{Name: "Alnatura Bio Hafer Drink ohne Zucker 1l", ListingID: "b", PriceCents: 189},
	}
	p, ok := BestMatch("hafermilch", ps)
	if !ok || p.ListingID != "b" {
		t.Fatalf("hafermilch -> %+v %v, want the Hafer Drink", p, ok)
	}
	if _, ok := BestMatch("xyzqwv", ps); ok {
		t.Fatal("nonsense query must not match")
	}
	if p, ok := BestMatch("haferflocken", ps); ok && p.ListingID == "" {
		t.Fatal("product without listing must be skipped")
	}
}

func TestParseList(t *testing.T) {
	got := ParseList("2x Hafermilch\n3 Äpfel\nKaffee 2\n- Butter\n\n# Kommentar\n200 Stück Eier\n")
	want := []ListItem{{"Hafermilch", 2}, {"Äpfel", 3}, {"Kaffee", 2}, {"Butter", 1}, {"Eier", 99}}
	if len(got) != len(want) {
		t.Fatalf("got %d items %+v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestBasketAndOrderPages(t *testing.T) {
	if id := ExtractBasketID(fixture(t, "basket.html")); id != "00000000-0000-4000-8000-000000000001" {
		t.Errorf("basket id = %q", id)
	}
	empty := fixture(t, "basket_empty.html")
	if ExtractBasketID(empty) != "" || !emptyBasketRe.MatchString(empty) {
		t.Error("empty basket not recognised")
	}
	ids := ParseOrderIDs(fixture(t, "orders_overview.html"))
	if len(ids) != 2 || ids[0] != "B-AAA-BBB-CCC" {
		t.Errorf("order ids = %v", ids)
	}
	open := ParseOrderDetail("B-AAA-BBB-CCC", fixture(t, "order_open.html"))
	if open.Status != "Bestellung eingegangen" || !strings.Contains(open.Slot, "18.11.26") {
		t.Errorf("open order = %+v", open)
	}
	cancelled := ParseOrderDetail("B-AAA-BBB-CCC", fixture(t, "order_cancelled.html"))
	if cancelled.Status != "Einkauf storniert" {
		t.Errorf("cancelled order status = %q", cancelled.Status)
	}
}

func TestSessionCookies(t *testing.T) {
	future := float64(time.Now().Add(24 * time.Hour).Unix())
	past := float64(time.Now().Add(-time.Hour).Unix())
	arr := `[{"name":"rstp","value":"abc","domain":".www.rewe.de","path":"/","expires":-1},
	         {"name":"KEYCLOAK_IDENTITY","value":"kid","domain":"account.rewe.de","path":"/realms/sso/","expires":` + jsonNum(future) + `},
	         {"name":"old","value":"x","domain":".rewe.de","path":"/","expires":` + jsonNum(past) + `},
	         {"name":"consentSettings","value":"{\"a\":1}","domain":".rewe.de","path":"/"},
	         {"name":"foreign","value":"y","domain":".example.com","path":"/"}]`
	cs, err := ParseCookieExport([]byte(arr))
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 4 {
		t.Fatalf("filter kept %d cookies, want 4 (example.com dropped)", len(cs))
	}
	obj, err := ParseCookieExport([]byte(`{"cookies":` + arr + `}`))
	if err != nil || len(obj) != 4 {
		t.Fatalf("object form: %v %d", err, len(obj))
	}
	s := &Session{Cookies: cs}
	if !s.Has("rstp") || !s.Has("KEYCLOAK_IDENTITY") || s.Has("old") {
		t.Error("Has() wrong about expiry")
	}
	if s.SSOExpiry().IsZero() {
		t.Error("SSO expiry missing")
	}
	jar, err := s.Jar()
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse("https://www.rewe.de/shop/")
	names := map[string]bool{}
	for _, c := range jar.Cookies(u) {
		names[c.Name] = true
	}
	if !names["rstp"] || names["consentSettings"] {
		t.Errorf("jar cookies = %v (rstp needed, quoted consent value skipped)", names)
	}
	// Server rotates rstp and sets a new cookie.
	jar.SetCookies(u, []*http.Cookie{{Name: "rstp", Value: "rotated", Path: "/"}, {Name: "mtc", Value: "new", Path: "/"}})
	s.MergeFromJar(jar)
	got := map[string]string{}
	for _, c := range s.Cookies {
		got[c.Name] = c.Value
	}
	if got["rstp"] != "rotated" || got["mtc"] != "new" {
		t.Errorf("merge result %v", got)
	}
	if !strings.Contains(s.ShopCookieHeader(), "rstp=rotated") || strings.Contains(s.ShopCookieHeader(), "KEYCLOAK") {
		t.Errorf("shop header = %q", s.ShopCookieHeader())
	}
}

func jsonNum(f float64) string { b, _ := json.Marshal(f); return string(b) }

func TestSessionSaveLoadAndPath(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.json")
	t.Setenv("REWE_SESSION_FILE", p)
	got, err := SessionPath()
	if err != nil || got != p {
		t.Fatalf("SessionPath = %q %v", got, err)
	}
	if _, err := LoadSession(p); !errors.Is(err, ErrNoSession) {
		t.Fatalf("missing file: %v, want ErrNoSession", err)
	}
	s := &Session{Cookies: []Cookie{{Name: "rstp", Value: "v", Domain: "www.rewe.de", Path: "/"}}}
	if err := s.Save(p); err != nil {
		t.Fatal(err)
	}
	back, err := LoadSession(p)
	if err != nil || len(back.Cookies) != 1 || back.Cookies[0].Value != "v" {
		t.Fatalf("round trip: %v %+v", err, back)
	}
}

// rewriteTransport sends every request to the test server, keeping path and query.
type rewriteTransport struct{ target *url.URL }

func (r rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = r.target.Scheme
	req.URL.Host = r.target.Host
	return http.DefaultTransport.RoundTrip(req)
}

func testShop(t *testing.T, h http.HandlerFunc, s *Session) *Shop {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	jar, _ := cookiejar.New(nil)
	return NewShopWithClient(&http.Client{Transport: rewriteTransport{u}, Jar: jar}, s)
}

func TestShopErrors(t *testing.T) {
	ctx := context.Background()
	sh := testShop(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/marketselection/configuration":
			w.Write([]byte(`{"selectedService":"PICKUP","selectedMarket":"3200008","isLoggedIn":true,"customerZipCode":"50667"}`))
		case "/limited":
			w.WriteHeader(429)
		case "/blocked":
			w.WriteHeader(403)
			w.Write([]byte(`<html>Just a moment... challenge-platform</html>`))
		default:
			w.WriteHeader(404)
		}
	}, &Session{})
	c, err := sh.Config(ctx)
	if err != nil || !c.IsLoggedIn || c.SelectedMarket != "3200008" {
		t.Fatalf("config = %+v %v", c, err)
	}
	var rl *cliutil.RateLimitError
	if err := sh.JSON(ctx, http.MethodGet, "/limited", nil, nil, nil); !errors.As(err, &rl) {
		t.Fatalf("429 gave %v, want RateLimitError", err)
	}
	if err := sh.JSON(ctx, http.MethodGet, "/blocked", nil, nil, nil); !IsCloudflareBlock(err) {
		t.Fatalf("403 challenge gave %v, want Cloudflare block", err)
	}
	if err := sh.Renew(ctx); !errors.Is(err, ErrReloginNeeded) {
		t.Fatalf("renew without SSO cookies = %v, want ErrReloginNeeded", err)
	}
}

func TestEnsureRenewsLoggedOutSession(t *testing.T) {
	loggedIn := false
	sh := testShop(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/marketselection/configuration":
			json.NewEncoder(w).Encode(map[string]any{"isLoggedIn": loggedIn})
		case r.URL.Path == "/mydata/login":
			w.Write([]byte(`<form class="login-confirmation tracking" action="/confirm?x=1&amp;y=2" method="post"><button name="login">Weiter</button></form>`))
		case r.URL.Path == "/confirm" && r.Method == http.MethodPost:
			if r.URL.Query().Get("y") != "2" {
				t.Errorf("&amp; not unescaped: %s", r.URL.RawQuery)
			}
			loggedIn = true
			w.Write([]byte("ok"))
		default:
			w.WriteHeader(404)
		}
	}, &Session{Cookies: []Cookie{{Name: "KEYCLOAK_IDENTITY", Value: "k", Domain: "account.rewe.de", Path: "/realms/sso/"}}})
	c, err := sh.Ensure(context.Background())
	if err != nil || !c.IsLoggedIn || !sh.Renewed() {
		t.Fatalf("Ensure = %+v %v renewed=%v", c, err, sh.Renewed())
	}
}

func TestMarketPaymentEnabled(t *testing.T) {
	on := json.RawMessage(`{"paymentOptions":{"enabledOfflinePaymentMethods":[{"key":"MARKET_PAYMENT"}]}}`)
	off := json.RawMessage(`{"paymentOptions":{"enabledOfflinePaymentMethods":[]}}`)
	if !MarketPaymentEnabled(on) || MarketPaymentEnabled(off) || MarketPaymentEnabled(json.RawMessage(`x`)) {
		t.Fatal("MarketPaymentEnabled wrong")
	}
}
