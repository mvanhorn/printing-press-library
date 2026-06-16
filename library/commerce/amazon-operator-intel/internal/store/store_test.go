package store

import "testing"

func TestSaveLoadProfileAndFixture(t *testing.T) {
	s := New(t.TempDir())
	if err := s.SaveProfile(Profile{Name: "demo", MarketplaceID: "ATVPDKIKX0DER", AdsProfileID: "321", TargetACOS: .25}); err != nil {
		t.Fatal(err)
	}
	p, err := s.GetProfile("demo")
	if err != nil {
		t.Fatal(err)
	}
	if p.MarketplaceID != "ATVPDKIKX0DER" || p.AdsProfileID != "321" || p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		t.Fatalf("bad profile: %#v", p)
	}
	d := Fixture("demo")
	if len(d.SKUs) < 10 || len(d.Campaigns) < 5 || len(d.SearchTerms) < 6 || len(d.BundleSignals) < 2 || len(d.VendorDeductions) < 2 {
		t.Fatalf("fixture too thin: %#v", d)
	}
	if err := s.SaveData(d); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.LoadData("demo")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Source != "embedded-fixture" || len(loaded.PurchaseOrders) < 2 {
		t.Fatalf("bad fixture load: %#v", loaded)
	}
	foundSuppressed, foundLaunch := false, false
	for _, sku := range loaded.SKUs {
		if sku.Suppressed && len(sku.Defects) > 0 {
			foundSuppressed = true
		}
		if sku.SKU == "FIXTURE-SKU" && sku.Source.Seller.Present && sku.Source.Ads.Present && sku.Source.Listings.Present {
			foundLaunch = true
		}
	}
	if !foundSuppressed || !foundLaunch {
		t.Fatalf("fixture missing suppressed or launch evidence")
	}
}
