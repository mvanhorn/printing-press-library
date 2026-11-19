package edeka

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Fixtures are synthetic: they copy the shop's markup shape with placeholder
// products and no customer data.
const gridHTML = `<html><body><div id="searchList"><div class="row">
<div class="product-box col-6"><div class="product-inner"><form>
<input type="hidden" name="aid" value="1000001MXST"></form>
<div class="picture"><a href="https://test-market.edeka.shop/Kuehlprodukte-EDEKA/Milch/Test-Milch.html?force_sid=x"><img src="https://img.example/p1.webp"></a></div>
<div class="listDetails"><div class="title"><a id="searchList_1000001MXST" href="https://test-market.edeka.shop/Kuehlprodukte-EDEKA/Milch/Test-Milch.html?force_sid=x"> Test Milch 1 l </a></div>
<div class="price"><div class="article-price"><div class="pricewrap"><p><span>1,19€</span></p></div></div>
<div class="pricePerUnit">1,19 €/<span>l</span><div class="o3-icon-in-basket"></div></div></div>
<span class="badge-text" aria-label="Angebot">%</span>
<div class="amountInput"><input type="text" value="2" class="form-control amountToBasket" id="amountToBasketList-1000001MXST"></div>
</div></div></div>
<div class="product-box col-6"><div class="product-inner"><form><input type="hidden" name="aid" value="2000002MXKG"></form>
<div class="listDetails"><div class="title"><a href="https://test-market.edeka.shop/Frischetheke-EDEKA/Fleisch/Braten.html">Braten</a></div>
<div class="price"><div class="article-price"><span>1.234,50€</span></div><div class="pricePerUnit">12,90 €/<span>kg</span></div></div>
<span class="mx-weightarticle-badge" aria-label="Gewichtsartikel"></span></div></div></div>
</div></div><a href="/index.php?cl=search&amp;pgNr=1&amp;searchparam=milch">next</a>
<form><input type="hidden" name="stoken" value="AB12CD34"></form></body></html>`

func TestParseProductGrid(t *testing.T) {
	g, err := ParseProductGrid(gridHTML, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Products) != 2 {
		t.Fatalf("want 2 products, got %d", len(g.Products))
	}
	p := g.Products[0]
	if p.ID != "1000001MXST" || p.Title != "Test Milch 1 l" || p.PriceCents != 119 || !p.OnOffer || p.InBasket != 2 || p.WeightArticle {
		t.Fatalf("unexpected first product: %+v", p)
	}
	if strings.Contains(p.URL, "force_sid") {
		t.Fatalf("session id left in URL: %s", p.URL)
	}
	if p.UnitPrice != "1,19 €/l" {
		t.Fatalf("unit price %q", p.UnitPrice)
	}
	if q := g.Products[1]; q.PriceCents != 123450 || !q.WeightArticle {
		t.Fatalf("unexpected second product: %+v", q)
	}
	if !g.HasNext {
		t.Fatal("expected a next page")
	}
	if ExtractStoken(gridHTML) != "AB12CD34" {
		t.Fatal("stoken not extracted")
	}
}

const basketHTML = `<html><body><ul>
<li class="basket-item" aria-labelledby="basket-item-title-1000001MXST">
<a id="basket-item-title-1000001MXST" class="basket-item-title" href="https://test-market.edeka.shop/x/Test-Milch.html">Test Milch</a>
<p id="productPriceUnit">1,19 €/ <span>l</span></p>
<input type="hidden" name="aproducts[k1][aid]" value="1000001MXST">
<input type="hidden" name="aproducts[k1][basketitemid]" value="k1">
<input type="number" name="aproducts[k1][am]" value="3">
<p id="unitPriceValue_1000001MXST">1,19 €</p><p id="totalPriceValue_1000001MXST">3,57 €</p>
</li></ul>
<span id="basketTotalProductsGross">3,57 €</span><span id="basketDeliveryGross">0,00 €</span><span id="basketGrandTotal">3,57 €</span>
</body></html>`

func TestParseBasket(t *testing.T) {
	b, err := ParseBasket(basketHTML)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Items) != 1 || b.Count != 3 || b.TotalCents != 357 {
		t.Fatalf("unexpected basket: %+v", b)
	}
	it := b.Items[0]
	if it.ID != "1000001MXST" || it.BasketItemID != "k1" || it.UnitPriceCents != 119 || it.TotalCents != 357 || it.PricePerUnit != "1,19 €/l" {
		t.Fatalf("unexpected item: %+v", it)
	}
	empty, err := ParseBasket("<html><body><p>leer</p></body></html>")
	if err != nil || len(empty.Items) != 0 || empty.Total != "0,00 €" {
		t.Fatalf("empty basket: %+v %v", empty, err)
	}
}

const calendarHTML = `<div id="calendar"><label id="label-pickup">Selbstabholung</label>
<div class="pickup__address"><address>Teststraße 1 - 12345 Teststadt</address></div>
<script type="text/javascript" id="pickupdate_pickupdata_script">
 (function() {
 window['pickupData'] = [];
 window['pickupData']['all'] = {"2030-01-02":[{"available":false,"time":"10:00","timestamp":1893571200,"timespan":{"from":"10:00","until":"11:00"},"slotPricelist":[]},{"available":true,"time":"11:00","timestamp":1893574800,"timespan":{"from":"11:00","until":"12:00"},"slotPricelist":[]}],"2030-01-01":[{"available":true,"time":"16:00","timestamp":1893513600,"timespan":{"from":"16:00","until":"17:00"},"slotPricelist":[]}]};
 window['pickupData']['premium'] = [];
 })()
</script></div>`

func TestParseCalendar(t *testing.T) {
	c, err := ParseCalendar(calendarHTML)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Pickup) != 3 {
		t.Fatalf("want 3 windows, got %d", len(c.Pickup))
	}
	if c.Pickup[0].Date != "2030-01-01" || c.Pickup[0].From != "16:00" || !c.Pickup[0].Available {
		t.Fatalf("slots not sorted or parsed: %+v", c.Pickup)
	}
	if c.Pickup[1].Available {
		t.Fatalf("10:00 should be booked out: %+v", c.Pickup[1])
	}
	if c.PickupAddress == "" || len(c.Types) != 1 {
		t.Fatalf("address/types: %+v", c)
	}
	if _, err := ParseCalendar("<div>nothing</div>"); err == nil {
		t.Fatal("expected an error without slot data")
	}
}

func TestParseOrderReviewAndForm(t *testing.T) {
	page := `<html><body><div class="order_white"><div><div class="text-nowrap">Ausgewählter Termin</div><span>02.01.2030 <span>11:00 - 12:00</span></span></div></div>
<p>Ihre Auswahl Selbstabholung</p><img class="order-payment__icon" title="Barzahlung">` + strings.TrimPrefix(basketHTML, "<html><body>") + `
<form id="orderConfirmAgbBottom"><input type="hidden" name="stoken" value="T1"><input type="hidden" name="cl" value="order"><input type="hidden" name="fnc" value="execute"><input type="hidden" name="sDeliveryAddressMD5" value="m"><input type="hidden" name="ord_agb" value="1"><input type="checkbox" name="unchecked" value="1"><button type="submit">JETZT KAUFEN</button></form>`
	r, err := ParseOrderReview(page)
	if err != nil {
		t.Fatal(err)
	}
	if r.Slot != "02.01.2030 11:00 - 12:00" || r.Payment != "Barzahlung" || r.ShippingMethod != "Selbstabholung" || !r.Ready || r.TotalCents != 357 {
		t.Fatalf("unexpected review: %+v", r)
	}
	f, ok := FormFields(page, func(id, _ string) bool { return id == "orderConfirmAgbBottom" })
	if !ok || f["fnc"][0] != "execute" || f["sDeliveryAddressMD5"][0] != "m" {
		t.Fatalf("form fields: %v", f)
	}
	if _, has := f["unchecked"]; has {
		t.Fatal("unchecked checkbox must not be submitted")
	}
}

func TestParsePaymentOptions(t *testing.T) {
	page := `<form name="order"><input type="radio" name="paymentid" value="mxcash" title="Barzahlung"><input type="radio" name="paymentid" value="mxtelecashdebit" title="SEPA-Lastschrift" checked></form>`
	opts := ParsePaymentOptions(page)
	if len(opts) != 2 || !opts[0].AtPickup || opts[1].AtPickup || !opts[1].Selected {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestPricesAndMarkets(t *testing.T) {
	for in, want := range map[string]int{"1,59€": 159, "1.234,50 €": 123450, "12,90 €/kg": 1290, "0,05": 5} {
		if got, ok := ParseCents(in); !ok || got != want {
			t.Fatalf("ParseCents(%q) = %d, %v", in, got, ok)
		}
	}
	if _, ok := ParseCents("kostenlos"); ok {
		t.Fatal("no amount expected")
	}
	if FormatCents(1290) != "12,90 €" || FormatCents(5) != "0,05 €" {
		t.Fatal("FormatCents")
	}
	for in, want := range map[string]string{"test-market": "test-market.edeka.shop", "Test-Market.edeka.shop": "test-market.edeka.shop", "https://test-market.edeka.shop/warenkorb/": "test-market.edeka.shop"} {
		if got, err := NormalizeMarket(in); err != nil || got != want {
			t.Fatalf("NormalizeMarket(%q) = %q, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "evil.example.com/x y", "www", "-x"} {
		if _, err := NormalizeMarket(bad); err == nil {
			t.Fatalf("NormalizeMarket(%q) should fail", bad)
		}
	}
	if !IsShopHost("a.edeka.shop") || IsShopHost("edeka.shop.evil.example") {
		t.Fatal("IsShopHost")
	}
}

func TestListLinesAndMatching(t *testing.T) {
	cases := map[string]struct {
		n int
		q string
	}{"2x butter": {2, "butter"}, "3 bananen": {3, "bananen"}, "kaffee x2": {2, "kaffee"}, "milch": {1, "milch"}, "Milch 1,5%": {1, "Milch 1,5%"}}
	for in, want := range cases {
		n, q := ParseListLine(in)
		if n != want.n || q != want.q {
			t.Fatalf("ParseListLine(%q) = %d %q", in, n, q)
		}
	}
	ps := []Product{
		{ID: "1MXST", Title: "Shampoo Hafermilch 250 ml", URL: "https://m.edeka.shop/Drogerie-EDEKA/Haar/Shampoo.html"},
		{ID: "2MXST", Title: "Belgian Butters Waffles", URL: "https://m.edeka.shop/Suesswaren-EDEKA/Waffeln/W.html"},
		{ID: "3MXST", Title: "Markenbutter 250 g", URL: "https://m.edeka.shop/Kuehlprodukte-EDEKA/Butter-Fette/B.html"},
	}
	if p, ok := BestMatch("butter", ps); !ok || p.ID != "3MXST" {
		t.Fatalf("butter matched %+v", p)
	}
	if p, ok := BestMatch("hafermilch", ps); !ok || p.ID != "1MXST" || !IsNonFood(p.URL) {
		t.Fatalf("hafermilch should match the only candidate, flagged non-food: %+v", p)
	}
	if AlternativeQuery("hafermilch") != "haferdrink" || AlternativeQuery("butter") != "" {
		t.Fatal("AlternativeQuery")
	}
	if !LooksLikeArticleID("5915407005MXST") || LooksLikeArticleID("milch") {
		t.Fatal("LooksLikeArticleID")
	}
}

func TestSessionCookiesAndSave(t *testing.T) {
	s := &Session{Cookies: []Cookie{
		{Name: "sid", Value: "old", Domain: "test-market.edeka.shop", Path: "/"},
		{Name: "oxid_" + "1", Value: "placeholder", Domain: "test-market.edeka.shop", Path: "/", Expires: float64(time.Now().Add(time.Hour).Unix())},
		{Name: "language", Value: "0", Domain: ".edeka.shop", Path: "/"},
		{Name: "other", Value: "x", Domain: "example.com", Path: "/"},
	}}
	s.Cookies = FilterEDEKA(s.Cookies)
	if len(s.Cookies) != 3 {
		t.Fatalf("filter kept %d cookies", len(s.Cookies))
	}
	if ms := s.Markets(); len(ms) != 1 || ms[0] != "test-market.edeka.shop" {
		t.Fatalf("markets %v", ms)
	}
	s.ApplySetCookies([]setCookieUpdate{
		{Host: "test-market.edeka.shop", Cookie: &http.Cookie{Name: "sid", Value: "new"}},
		{Host: "test-market.edeka.shop", Cookie: &http.Cookie{Name: "mxpickupdate", Value: "2030-01-01", MaxAge: 600}},
		{Host: "test-market.edeka.shop", Cookie: &http.Cookie{Name: "language", Domain: "edeka.shop", Value: "deleted", MaxAge: -1}},
	})
	got := map[string]string{}
	for _, c := range s.Cookies {
		got[c.Name] = c.Value
	}
	if got["sid"] != "new" || got["mxpickupdate"] != "2030-01-01" {
		t.Fatalf("cookies after update: %v", got)
	}
	if _, ok := got["language"]; ok {
		t.Fatal("deleted cookie kept")
	}
	if len(s.HostCookies("other-market.edeka.shop")) != 0 {
		t.Fatal("host-only cookies leaked to another market")
	}
	path := filepath.Join(t.TempDir(), "session.json")
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	back, err := LoadSession(path)
	if err != nil || len(back.Cookies) != len(s.Cookies) {
		t.Fatalf("reload: %v %v", back, err)
	}
	if _, err := LoadSession(filepath.Join(t.TempDir(), "missing.json")); err != ErrNoSession {
		t.Fatalf("want ErrNoSession, got %v", err)
	}
	if fi, err := os.Stat(path); err != nil || fi.Size() == 0 {
		t.Fatal("session file not written")
	}
}

// TestBasketChangeRefreshesStaleToken: the shop answers a stale form token
// with an HTML page; the client must fetch a fresh token and retry once.
func TestBasketChangeRefreshesStaleToken(t *testing.T) {
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Query().Get("cl") == "basket" {
			io.WriteString(w, `<html><form><input type="hidden" name="stoken" value="FRESH01"></form></html>`)
			return
		}
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			posts++
			if r.PostForm.Get("stoken") != "FRESH01" {
				io.WriteString(w, "<!DOCTYPE html><html><head></head><body>start page</body></html>")
				return
			}
			if r.PostForm.Get("aid") != "1000001MXST" || r.PostForm.Get("am") != "2" {
				t.Errorf("unexpected form %v", r.PostForm)
			}
			io.WriteString(w, `{"success":true,"productCount":2,"NewBruttoSum":2.38}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	sh := NewTestShop(srv.Client(), &Session{Stoken: "STALE00"}, srv.URL)
	r, err := sh.SetQuantity(context.Background(), "1000001MXST", 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if r.ProductCount != 2 || posts != 2 {
		t.Fatalf("result %+v after %d posts", r, posts)
	}
}

func TestLoggedInAndEnsure(t *testing.T) {
	logged := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if logged {
			io.WriteString(w, `<a class="btn button-service" aria-label="Mein Konto"></a>`)
		} else {
			io.WriteString(w, `<a class="btn button-service" aria-label="Anmelden"></a>`)
		}
	}))
	defer srv.Close()
	s := &Session{Cookies: []Cookie{{Name: "sid", Value: "v", Domain: "127.0.0.1", Path: "/"}}}
	sh := NewTestShop(srv.Client(), s, srv.URL)
	if err := sh.Ensure(context.Background()); err != ErrReloginNeeded {
		t.Fatalf("want ErrReloginNeeded, got %v", err)
	}
	logged = true
	if err := sh.Ensure(context.Background()); err != nil {
		t.Fatalf("want logged in, got %v", err)
	}
	if err := NewTestShop(srv.Client(), &Session{}, srv.URL).Ensure(context.Background()); err != ErrNoSession {
		t.Fatalf("want ErrNoSession, got %v", err)
	}
}

func TestReviewFixes(t *testing.T) {
	if c, ok := ParseCents("-0,50 €"); !ok || c != -50 {
		t.Fatalf("negative cents: %d", c)
	}
	if c, ok := ParseCents("-1.234,05"); !ok || c != -123405 {
		t.Fatalf("negative thousands: %d", c)
	}
	page := `<html><body><table><tr><th>Bestellnummer</th><th>Datum</th></tr></table>
<div class="orders"><div class="order-card"><span>Bestellnummer: 100234</span> <span>02.01.2030</span> <span>Summe 12,34 €</span></div>
<div class="order-card"><span>Bestellnummer: 100235</span> <span>03.01.2030</span> <span>Summe 5,00 €</span></div></div>
<footer>Versand 3,90 €</footer></body></html>`
	orders, _, err := ParseOrderHistory(page)
	if err != nil {
		t.Fatal(err)
	}
	if len(orders) != 2 || orders[0].Number != "100234" || orders[0].Total != "12,34 €" || orders[1].Number != "100235" || orders[1].Total != "5,00 €" {
		t.Fatalf("orders: %+v", orders)
	}
	if m := orderNrRe.FindStringSubmatch("Ihre Bestellnummer lautet 12345"); m == nil || m[1] != "12345" {
		t.Fatalf("order number: %v", m)
	}
	s := &Session{}
	s.ApplySetCookies([]setCookieUpdate{{Host: "m.edeka.shop", Cookie: &http.Cookie{Name: "wide", Value: "x", Domain: "shop"}}})
	if len(s.Cookies) != 0 {
		t.Fatal("a cookie scoped beyond edeka.shop was stored")
	}
	if loopbackWS("ws://evil.example/devtools") || !loopbackWS("ws://127.0.0.1:9222/devtools/browser/x") {
		t.Fatal("loopbackWS")
	}
}

func TestSaveMergedKeepsOtherRunsCookies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.json")
	a := &Session{Cookies: []Cookie{{Name: "sid", Value: "a", Domain: "m.edeka.shop", Path: "/"}}}
	if err := a.Save(path); err != nil {
		t.Fatal(err)
	}
	// another run stored a new slot cookie meanwhile
	b, _ := LoadSession(path)
	b.Cookies = append(b.Cookies, Cookie{Name: "mxpickupdate", Value: "2030-01-01", Domain: "m.edeka.shop", Path: "/"})
	if err := b.Save(path); err != nil {
		t.Fatal(err)
	}
	// this run only rotated sid
	upd := []setCookieUpdate{{Host: "m.edeka.shop", Cookie: &http.Cookie{Name: "sid", Value: "a2"}}}
	a.ApplySetCookies(upd)
	if err := a.SaveMerged(path, upd, "m", ""); err != nil {
		t.Fatal(err)
	}
	got, _ := LoadSession(path)
	vals := map[string]string{}
	for _, c := range got.Cookies {
		vals[c.Name] = c.Value
	}
	if vals["sid"] != "a2" || vals["mxpickupdate"] != "2030-01-01" || got.Market != "m" {
		t.Fatalf("merged session: %v market=%q", vals, got.Market)
	}
}

func TestWordMatch(t *testing.T) {
	cases := []struct {
		q, title string
		want     bool
	}{
		{"milch", "Weide Milch", true},
		{"milch", "Schwarzwaldmilch frische fettarme Milch", true},
		{"vollmilch", "Bio Vollmilch 3,8 %", true},
		{"__printing_press_invalid__", "GUT&GÜNSTIG Latte Espresso 250 ml", false},
		{"kaffee", "Eduscho Lieblingsbohnen ganze Bohnen 750 g", false},
	}
	for _, c := range cases {
		if got := WordMatch(c.q, c.title); got != c.want {
			t.Fatalf("WordMatch(%q, %q) = %v", c.q, c.title, got)
		}
	}
}

// TestSearchMarketsNearbyFallback: a postal code no market serves falls back to
// nearby postal codes (same three-digit block first) and marks the result.
func TestSearchMarketsNearbyFallback(t *testing.T) {
	var probed []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("fnc") != "searchMarketsByZipOrText" {
			http.NotFound(w, r)
			return
		}
		probed = append(probed, q.Get("search"))
		if q.Get("search") == "89231" {
			io.WriteString(w, `[{"mxactive":1,"mxdomain":"m1.edeka.shop","mxtitle":"EDEKA Test","mxzip":"89077","mxcity":"Ulm","mxpickup":1}]`)
			return
		}
		io.WriteString(w, `[]`)
	}))
	defer srv.Close()
	sh := NewTestShop(srv.Client(), &Session{}, srv.URL)
	ms, err := sh.SearchMarkets(context.Background(), "89233")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 || ms[0].Slug != "m1" || !ms[0].Nearby || ms[0].ServedZip != "89231" {
		t.Fatalf("markets %+v", ms)
	}
	if probed[0] != "89233" || len(probed) > MaxNearbyProbes+1 {
		t.Fatalf("probe order %v", probed)
	}
}

func TestNearbyProbeZips(t *testing.T) {
	got := nearbyProbeZips("89233", MaxNearbyProbes)
	want := []string{"89232", "89234", "89231", "89235", "89230", "89236"}
	if len(got) != MaxNearbyProbes || strings.Join(got[:6], ",") != strings.Join(want, ",") {
		t.Fatalf("got %v", got)
	}
	if tail := strings.Join(got[len(got)-4:], ","); tail != "89133,89333,89033,89433" {
		t.Fatalf("neighbour blocks %s", tail)
	}
	for _, z := range nearbyProbeZips("01001", MaxNearbyProbes) {
		if !IsZip(z) || z[:2] != "01" && z[:2] != "00" && z[:2] != "02" {
			t.Fatalf("bad probe %s", z)
		}
	}
}
