package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/immovlan"
)

func fp(v float64) *float64 { return &v }
func ip(v int) *int         { return &v }

func openTestVlan(t *testing.T) *Store {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "immovlan.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	for i := 0; i < 2; i++ {
		if err := db.EnsureVlanSchema(context.Background()); err != nil {
			t.Fatalf("schema must be idempotent: %v", err)
		}
	}
	return db
}

func TestAddrKeyAndPhotoHash(t *testing.T) {
	cases := map[string]string{
		"Rue Thiéfry 54":                       "1030|thiefry|54",
		"AVENUE EMILE MAX 57":                  "1030|emile max|57",
		"Avenue Émile Max, 57 A":               "1030|emile max|57",
		"Chaussée de Haecht 120":               "1030|de haecht|120",
		"Rue sans numéro":                      "",
		"Boulevard Anspach 1/3":                "1030|anspach|1",
		"Rue de la Poste 12-14":                "1030|de la poste|12",
		"Chaussée de Waterloo 1234 bte 5":      "1030|de waterloo|1234",
		"Avenue Louise 0":                      "",
		"Avenue du 11 Novembre 12":             "1030|du 11novembre|12",
		"Place du 4 Septembre 3":               "1030|du 4septembre|3",
		"Rue Neuve 57bis":                      "1030|neuve|57",
		"Rue Neuve 57 A":                       "1030|neuve|57",
		"Rue Royale 12a":                       "1030|royale|12",
		"Avenue du 1er Mai 12":                 "1030|du 1er mai|12",
		"Place du 4 Mai 12":                    "1030|du 4mai|12",
		"Avenue de la 2e Armée Britannique 12": "1030|de la 2e armee britannique|12",
		"1ste Straat 12":                       "1030|1ste|12",
	}
	for in, want := range cases {
		if got := AddrKey("1030", in); got != want {
			t.Errorf("AddrKey(%q) = %q, want %q", in, got, want)
		}
	}
	h1 := PhotoHash([]string{"https://api-image.immovlan.be/v1/property/VBE1/images/abc.jpg/Large", "https://x/def.jpg"})
	h2 := PhotoHash([]string{"https://api-image.immovlan.be/v1/property/VBE2/images/ABC.jpg?x=1", "https://y/def.jpg/Medium"})
	h3 := PhotoHash([]string{"https://api-image.immovlan.be/v1/property/VBE3/images/zzz.jpg/Large", "https://y/def.jpg/Medium"})
	if h1 == "" || h1 != h2 || h1 == h3 || PhotoHash(nil) != "" {
		t.Errorf("photo hash must depend on file names only, ignoring the size segment: %q %q %q", h1, h2, h3)
	}
}

func TestUpsertDetailAndQueries(t *testing.T) {
	ctx := context.Background()
	db := openTestVlan(t)
	t0 := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	l := immovlan.Listing{ID: "VBE1", Deal: immovlan.DealSale, Type: "maison", PostalCode: "1030", Locality: "Schaerbeek", Price: fp(500000), Surface: fp(200), Bedrooms: ip(4), EPC: "F", EPCBand: "bad", SellerType: "estateAgents", Agency: "Agence X", AgencyID: "42"}
	if err := db.UpsertVlanListings(ctx, []immovlan.Listing{l}, t0); err != nil {
		t.Fatal(err)
	}
	l.Price = fp(480000)
	if err := db.UpsertVlanListings(ctx, []immovlan.Listing{l}, t0.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	hist, err := db.VlanPriceHistory(ctx, "vbe1")
	if err != nil || len(hist) != 2 || hist[1].Price != 480000 {
		t.Fatalf("history = %+v %v", hist, err)
	}
	rented := true
	d := immovlan.Detail{Listing: l, Condition: "À rénover", Rented: &rented, CadastralIncome: fp(1200), Year: ip(1910), Software: "Whise", Photos: []string{"https://api-image.immovlan.be/v1/property/VBE1/images/a.jpg"}}
	d.Street = "Rue Thiéfry 54"
	if err := db.SaveVlanDetail(ctx, d, t0.Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	rows, err := db.QueryVlanListings(ctx, ListingFilter{Deal: immovlan.DealSale, PostalCodes: []string{"BE-1030"}, EPC: []string{"f"}, Rented: &rented})
	if err != nil || len(rows) != 1 {
		t.Fatalf("query = %d rows, %v", len(rows), err)
	}
	r := rows[0]
	if r.Condition != "À rénover" || r.Year == nil || *r.Year != 1910 || r.Software != "Whise" || r.AddrKey != "1030|thiefry|54" || r.PhotoHash == "" || r.PricePerSqm == nil || *r.PricePerSqm != 2400 {
		t.Errorf("stored detail = %+v", r)
	}
	// a thinner detail page (no type, no price, no photos) must not erase what the card knew
	thin := immovlan.Detail{Listing: immovlan.Listing{ID: "VBE1"}}
	if err := db.SaveVlanDetail(ctx, thin, t0.Add(49*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if rows, _ := db.QueryVlanListings(ctx, ListingFilter{IDs: []string{"VBE1"}}); len(rows) != 1 || rows[0].Type != "maison" || rows[0].Price == nil || *rows[0].Price != 480000 || rows[0].PostalCode != "1030" || len(rows[0].Photos) != 1 || rows[0].Private {
		t.Errorf("thin detail clobbered the row: %+v", rows)
	}
	if rows, _ := db.QueryVlanListings(ctx, ListingFilter{MissingAny: []string{"RENTED"}}); len(rows) != 0 {
		t.Errorf("rented is set, MissingAny must skip it: %d", len(rows))
	}
	if rows, _ := db.QueryVlanListings(ctx, ListingFilter{MissingAny: []string{"geo"}}); len(rows) != 1 {
		t.Errorf("geo is missing, MissingAny must return it: %d", len(rows))
	}
	if _, err := db.QueryVlanListings(ctx, ListingFilter{MissingAny: []string{"bogus"}}); err == nil {
		t.Error("unknown missing field must error")
	}
	if n, err := db.MarkGone(ctx, []string{"VBE1"}, t0, t0); err != nil || n != 0 {
		t.Fatalf("MarkGone must skip a listing seen after the cutoff: %d %v", n, err)
	}
	if rows, _ := db.QueryVlanListings(ctx, ListingFilter{DetailBefore: t0.Add(1 * time.Hour).UTC().Format(time.RFC3339)}); len(rows) != 0 {
		t.Errorf("DetailBefore must exclude a page read later: %d", len(rows))
	}
	if rows, _ := db.QueryVlanListings(ctx, ListingFilter{DetailBefore: t0.Add(100 * time.Hour).UTC().Format(time.RFC3339), OldestDetailFirst: true}); len(rows) != 1 {
		t.Errorf("DetailBefore must keep an older read: %d", len(rows))
	}
	if n, err := db.MarkGone(ctx, []string{"VBE1"}, t0.Add(72*time.Hour), t0.Add(72*time.Hour)); err != nil || n != 1 {
		t.Fatalf("MarkGone = %d %v", n, err)
	}
	if rows, _ := db.QueryVlanListings(ctx, ListingFilter{}); len(rows) != 0 {
		t.Error("gone listing must be hidden by default")
	}
	if rows, _ := db.QueryVlanListings(ctx, ListingFilter{IncludeGone: true}); len(rows) != 1 || rows[0].GoneAt == "" {
		t.Error("IncludeGone must return it with gone_at")
	}
	if pcs, _ := db.LocalPostcodesFor(ctx, "schaerbeek"); len(pcs) != 1 || pcs[0] != "1030" {
		t.Errorf("LocalPostcodesFor = %v", pcs)
	}
	// a stale hash recipe is recomputed from the stored pictures on the next schema check
	if _, err := db.db.Exec(`UPDATE vlan_listings SET photo_hash = 'stale'; DELETE FROM vlan_meta`); err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureVlanSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if rows, _ := db.QueryVlanListings(ctx, ListingFilter{IncludeGone: true}); len(rows) != 1 || rows[0].PhotoHash != PhotoHash(d.Photos) {
		t.Errorf("photo hash migration did not run: %+v", rows)
	}
}

func TestSavedSearchesAndSeen(t *testing.T) {
	ctx := context.Background()
	db := openTestVlan(t)
	crit := immovlan.Criteria{Types: []string{"maison"}, Deal: immovlan.DealSale, PostalCodes: []string{"1030"}}
	if err := db.SaveSearch(ctx, "s", crit); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := db.RecordSearchRun(ctx, "s", map[string]*float64{"A": fp(1), "B": fp(2)}, nil, now); err != nil {
		t.Fatal(err)
	}
	seen, _ := db.SeenIDs(ctx, "s")
	if len(seen) != 2 || seen["A"].LastPrice == nil || *seen["A"].LastPrice != 1 {
		t.Fatalf("seen = %v", seen)
	}
	if err := db.RecordSearchRun(ctx, "s", map[string]*float64{"A": fp(1)}, []string{"B"}, now); err != nil {
		t.Fatal(err)
	}
	if seen, _ := db.SeenIDs(ctx, "s"); len(seen) != 1 {
		t.Error("left listing must be forgotten")
	}
	reordered := crit
	reordered.PostalCodes = []string{"1030"}
	reordered.Types = []string{"maison"}
	_ = db.SaveSearch(ctx, "s", reordered)
	if ss, _ := db.GetSearch(ctx, "s"); ss.LastRunAt == "" {
		t.Error("re-saving identical criteria must keep the watch baseline")
	}
	crit.MaxPrice = 100
	_ = db.SaveSearch(ctx, "s", crit)
	if ss, _ := db.GetSearch(ctx, "s"); ss.LastRunAt != "" {
		t.Error("criteria change must reset last_run_at")
	}
	if ok, _ := db.DeleteSearch(ctx, "s"); !ok {
		t.Error("delete should report true")
	}
	_ = db.SetHidden(ctx, "A", true, "x")
	_ = db.SetShortlist(ctx, "A", true, "note")
	if h, _ := db.HiddenSet(ctx); !h["A"] {
		t.Error("hidden set")
	}
	if sl, _ := db.ListShortlist(ctx); len(sl) != 1 || sl[0].Note != "note" {
		t.Error("shortlist")
	}
}
