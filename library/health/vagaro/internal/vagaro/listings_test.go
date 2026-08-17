// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package vagaro

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseListings(t *testing.T) {
	html := `<html><head>
<script type="application/ld+json">
{"@type":"ItemList","itemListElement":[
  {"@type":"ListItem","position":1,"item":{"@type":"HealthAndBeautyBusiness","name":"Central Barber","url":"https://www.vagaro.com/centralbarber","telephone":"(206) 555-0100","priceRange":"$$","address":{"streetAddress":"123 Pine St","addressLocality":"Seattle","addressRegion":"WA","postalCode":"98101"},"aggregateRating":{"ratingValue":"4.9","ratingCount":"212"}}},
  {"@type":"ListItem","position":2,"item":{"@type":"HealthAndBeautyBusiness","name":"Rudy&#39;s Barbershop","url":"https://www.vagaro.com/rudysbarbershop?ref=x","priceRange":"$","address":{"addressLocality":"Seattle","addressRegion":"wa"},"aggregateRating":{"ratingValue":4.2,"ratingCount":88}}}
]}
</script></head><body></body></html>`
	rows := ParseListings(html)
	require.Len(t, rows, 2)

	assert.Equal(t, "Central Barber", rows[0].Name)
	assert.Equal(t, "centralbarber", rows[0].Slug)
	assert.Equal(t, "(206) 555-0100", rows[0].Phone)
	assert.Equal(t, "$$", rows[0].PriceRange)
	assert.Equal(t, 4.9, rows[0].Rating)
	assert.Equal(t, 212, rows[0].ReviewCount)
	assert.Equal(t, "Seattle", rows[0].City)
	assert.Equal(t, "WA", rows[0].State)
	assert.Contains(t, rows[0].Address, "123 Pine St")

	// Entity-decoded name, slug stripped of query string, region upper-cased.
	assert.Equal(t, "Rudy's Barbershop", rows[1].Name)
	assert.Equal(t, "rudysbarbershop", rows[1].Slug)
	assert.Equal(t, "WA", rows[1].State)
	assert.Equal(t, 4.2, rows[1].Rating)
	assert.Equal(t, 88, rows[1].ReviewCount)
}

func TestParseListings_dedupeAndEmpty(t *testing.T) {
	assert.Empty(t, ParseListings(`<html>no ld json here</html>`))

	// Same slug across two blocks is de-duplicated (first seen wins).
	html := `<script type="application/ld+json">[{"item":{"name":"A","url":"https://www.vagaro.com/a"}}]</script>
<script type="application/ld+json">[{"item":{"name":"A dup","url":"https://www.vagaro.com/a"}}]</script>`
	rows := ParseListings(html)
	require.Len(t, rows, 1)
	assert.Equal(t, "A", rows[0].Name)
}

func TestParsePriceTextCents(t *testing.T) {
	tests := []struct {
		in     string
		want   int
		wantOK bool
	}{
		{"$52.00", 5200, true},
		{"$52", 5200, true},
		{"From $120.50", 12050, true},
		{"$8.5", 850, true},
		{"Free", 0, false},
		{"Varies", 0, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, ok := ParsePriceTextCents(tt.in)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestParseSlotDateTime(t *testing.T) {
	dt, ok := ParseSlotDateTime("24 Jul 2026", "10:00 AM")
	require.True(t, ok)
	assert.Equal(t, 2026, dt.Year())
	assert.Equal(t, 10, dt.Hour())
	assert.Equal(t, 0, dt.Minute())

	dt2, ok := ParseSlotDateTime("Fri Jul-24-2026", "1:15 PM")
	require.True(t, ok)
	assert.Equal(t, 13, dt2.Hour())
	assert.Equal(t, 15, dt2.Minute())

	// Parseable date but unparseable time yields date at midnight.
	dt3, ok := ParseSlotDateTime("24 Jul 2026", "not a time")
	require.True(t, ok)
	assert.Equal(t, 0, dt3.Hour())

	// Unparseable date fails.
	_, ok = ParseSlotDateTime("", "10:00 AM")
	assert.False(t, ok)
}

func TestSlugFromURL(t *testing.T) {
	assert.Equal(t, "centralbarber", slugFromURL("https://www.vagaro.com/centralbarber"))
	assert.Equal(t, "centralbarber", slugFromURL("https://www.vagaro.com/centralbarber/"))
	assert.Equal(t, "centralbarber", slugFromURL("https://www.vagaro.com/us02/centralbarber?ref=1"))
	assert.Equal(t, "", slugFromURL(""))
}
