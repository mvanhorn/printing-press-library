package store

import "testing"

func TestSaveLoadProfileAndData(t *testing.T) {
	s := New(t.TempDir())
	if err := s.SaveProfile(Profile{Name: "demo", ShopifyShop: "demo.myshopify.com", GAProperty: "123"}); err != nil {
		t.Fatal(err)
	}
	p, err := s.GetProfile("demo")
	if err != nil {
		t.Fatal(err)
	}
	if p.ShopifyShop != "demo.myshopify.com" || p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		t.Fatalf("bad profile: %#v", p)
	}
	d := Fixture("demo")
	if err := s.SaveData(d); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.LoadData("demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Products) != 3 || len(loaded.Pages) != 2 || loaded.Source != "embedded-shopify-commerce-fixture" {
		t.Fatalf("bad fixture load: %#v", loaded)
	}
	if !loaded.Storefront.StructuredData || loaded.Storefront.Answerability == 0 {
		t.Fatalf("missing GEO fixture fields: %#v", loaded.Storefront)
	}
	for _, p := range loaded.Products {
		if p.Handle == "" || p.Revenue == 0 {
			t.Fatalf("bad product fixture: %#v", p)
		}
		if !p.Source.Shopify.Synced || !p.Source.GA4.Synced || !p.Source.GSC.Synced || !p.Source.Ahrefs.Synced || !p.Source.Klaviyo.Synced {
			t.Fatalf("source evidence not mirrored: %#v", p.Source)
		}
	}
}
