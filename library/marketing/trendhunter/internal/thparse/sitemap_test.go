package thparse

import "testing"

func TestParseSitemapKinds(t *testing.T) {
	body := []byte(`<?xml version="1.0"?><urlset>
		<url><loc>https://www.trendhunter.com/trends/ai-clone</loc><lastmod>2026-05-13</lastmod></url>
		<url><loc>https://www.trendhunter.com/megatrend/workplace-ai</loc></url>
		<url><loc>https://www.trendhunter.com/tech</loc></url>
		<url><loc>https://www.trendhunter.com/about</loc></url>
	</urlset>`)
	entries, err := ParseSitemap(body)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{entries[0].Kind, entries[1].Kind, entries[2].Kind, entries[3].Kind}
	want := []string{"trend", "megatrend", "category", "other"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("kind[%d]=%q want %q", i, got[i], want[i])
		}
	}
}
