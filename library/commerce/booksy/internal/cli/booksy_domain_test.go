// Copyright 2026 Max Tomago and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import "testing"

func sampleBusiness() *bkBusiness {
	return &bkBusiness{
		ID:   297360,
		Name: "Barber Ivan | Wola",
		ServiceCategories: []bkServiceCategory{
			{
				Name: "Strzyżenie",
				Services: []bkService{
					{
						Name: "Strzyżenie | Haircut",
						Variants: []bkVariant{
							{ID: 20193554, Price: 110, Duration: 30, ServicePrice: "110,00 zł"},
						},
					},
					{
						Name: "Strzyżenie BuzzCut",
						Variants: []bkVariant{
							{ID: 19851442, Price: 80, Duration: 20, ServicePrice: "80,00 zł+"},
						},
					},
				},
			},
			{
				Name: "Broda",
				Services: []bkService{
					{
						Name: "Trymowanie brody",
						Variants: []bkVariant{
							{ID: 111, Price: 50, Duration: 15, ServicePrice: "50,00 zł"},
						},
					},
				},
			},
		},
	}
}

func TestFlattenServices(t *testing.T) {
	b := sampleBusiness()
	all := flattenServices(b, "")
	if len(all) != 3 {
		t.Fatalf("expected 3 flattened rows, got %d", len(all))
	}
	if all[0].VariantID != 20193554 || all[0].PriceLabel != "110,00 zł" {
		t.Errorf("unexpected first row: %+v", all[0])
	}
}

func TestFlattenServicesFilter(t *testing.T) {
	b := sampleBusiness()
	// English "haircut" must match Polish "Strzyżenie" via the stem map,
	// and must NOT match the beard service.
	rows := flattenServices(b, "haircut")
	if len(rows) != 2 {
		t.Fatalf("expected 2 haircut rows, got %d: %+v", len(rows), rows)
	}
	for _, r := range rows {
		if r.Category == "Broda" {
			t.Errorf("beard service leaked into haircut filter: %+v", r)
		}
	}
}

func TestCheapestMatching(t *testing.T) {
	b := sampleBusiness()
	best := cheapestMatching(b, "haircut")
	if best == nil {
		t.Fatal("expected a cheapest haircut match, got nil")
	}
	if best.VariantID != 19851442 || best.Price != 80 {
		t.Errorf("expected cheapest haircut = BuzzCut (80), got %+v", best)
	}
	if none := cheapestMatching(b, "massage"); none != nil {
		t.Errorf("expected no match for massage, got %+v", none)
	}
}

func TestServiceMatches(t *testing.T) {
	cases := []struct {
		text, query string
		want        bool
	}{
		{"Strzyżenie | Haircut", "haircut", true},
		{"Strzyzenie meskie", "haircut", true}, // deaccented input
		{"Trymowanie brody", "beard", true},
		{"Trymowanie brody", "haircut", false},
		{"anything", "", true},
	}
	for _, c := range cases {
		if got := serviceMatches(c.text, c.query); got != c.want {
			t.Errorf("serviceMatches(%q,%q)=%v want %v", c.text, c.query, got, c.want)
		}
	}
}

func TestDeaccentLower(t *testing.T) {
	if got := deaccentLower("StrzyŻeNie Łódź"); got != "strzyzenie lodz" {
		t.Errorf("deaccentLower mismatch: %q", got)
	}
}
