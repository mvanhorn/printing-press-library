package store

import "testing"

func TestSaveLoadProfileAndData(t *testing.T) {
	s := New(t.TempDir())
	if err := s.SaveProfile(Profile{Name: "demo", SiteURL: "https://example.com"}); err != nil {
		t.Fatal(err)
	}
	p, err := s.GetProfile("demo")
	if err != nil {
		t.Fatal(err)
	}
	if p.SiteURL != "https://example.com" || p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
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
	if len(loaded.Pages) != 4 || loaded.Source != "embedded-fixture" {
		t.Fatalf("bad fixture load: %#v", loaded)
	}
	for _, p := range loaded.Pages {
		if p.URL == "/best-blue-ray-ripper" {
			t.Fatalf("fixture contains unrelated page: %#v", p)
		}
		if p.Sources.GSC.Clicks != p.Clicks || p.Sources.GA4.Revenue != p.Revenue || p.Sources.Ahrefs.RefDomains != p.RefDomains {
			t.Fatalf("source fields not mirrored for %s: %#v", p.URL, p.Sources)
		}
	}
}
