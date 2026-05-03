package vrbo

import "testing"

func TestPropertiesFromSearchHTMLCards(t *testing.T) {
	html := []byte(`
		<article data-stid="lodging-card-responsive">
			<a href="/h12345678?adults=2"><h3>Photo gallery for Near Ski Resorts | Basque Lodge by AvantStay</h3></a>
			<div data-stid="sponsored-ad-badge">Sponsored</div>
			<p>$533 nightly $1,350 total 6 bedrooms 8 beds 3.5 baths Sleeps 12</p>
		</article>
	`)
	props, err := propertiesFromSearchHTML(html)
	if err != nil {
		t.Fatal(err)
	}
	if len(props) != 1 {
		t.Fatalf("len(props) = %d, want 1", len(props))
	}
	p := props[0]
	if p.ID != "12345678" {
		t.Fatalf("ID = %q", p.ID)
	}
	if p.Title != "Near Ski Resorts | Basque Lodge by AvantStay" {
		t.Fatalf("Title = %q", p.Title)
	}
	if p.PropertyManagementName != "AvantStay" {
		t.Fatalf("PropertyManagementName = %q", p.PropertyManagementName)
	}
	if p.PriceBreakdown == nil || p.PriceBreakdown.PerNight != 533 || p.PriceBreakdown.Total != 1350 {
		t.Fatalf("PriceBreakdown = %#v", p.PriceBreakdown)
	}
	if p.Beds != 6 || p.Baths != 3.5 || p.SleepsMax != 12 {
		t.Fatalf("counts = beds %d baths %.1f sleeps %d", p.Beds, p.Baths, p.SleepsMax)
	}
}
