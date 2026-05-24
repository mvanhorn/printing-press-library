// Copyright 2026 bobeglz. Licensed under Apache-2.0. See LICENSE.
package rappi

import (
	"encoding/json"
	"testing"
)

func TestExtractJSONLDBlocks(t *testing.T) {
	html := []byte(`<html><head>
<script type="application/ld+json">{"@type":"Brand","name":"El Farolito"}</script>
<script type="application/ld+json">
{"@type":"Restaurant","name":"El Farolito","url":"https://www.rappi.com.mx/restaurantes/10000295-el-farolito"}
</script>
</head></html>`)
	blocks := ExtractJSONLDBlocks(html)
	if len(blocks) != 2 {
		t.Fatalf("want 2 blocks, got %d", len(blocks))
	}
	var m map[string]any
	if err := json.Unmarshal(blocks[0], &m); err != nil {
		t.Fatalf("parse block 0: %v", err)
	}
	if m["@type"] != "Brand" {
		t.Errorf("block 0 @type = %v, want Brand", m["@type"])
	}
}

func TestParseRestaurant(t *testing.T) {
	html := []byte(`<html><script type="application/ld+json">{
		"@type":"Restaurant",
		"@id":"https://www.rappi.com.mx/restaurantes/10000295-el-farolito",
		"name":"El Farolito",
		"url":"https://www.rappi.com.mx/restaurantes/10000295-el-farolito",
		"image":"https://images.rappi.com.mx/restaurants_background/foo.png",
		"servesCuisine":["Tacos","Mexicana"],
		"address":{"@type":"PostalAddress","streetAddress":"ALTATA No. 19. COL. HIPODROMO CONDESA, CUAUHTEMOC."},
		"openingHoursSpecification":[{"dayOfWeek":"https://schema.org/Tuesday","opens":"12:00:00","closes":"23:59:00"}],
		"geo":{"@type":"GeoCoordinates","latitude":19.406346,"longitude":-99.172858},
		"aggregateRating":{"@type":"AggregateRating","ratingCount":64,"ratingValue":4.1,"bestRating":5}
	}</script></html>`)
	r := ParseRestaurant(html)
	if r == nil {
		t.Fatal("ParseRestaurant returned nil")
	}
	if r.ID != "10000295" {
		t.Errorf("ID = %q, want 10000295", r.ID)
	}
	if r.Name != "El Farolito" {
		t.Errorf("Name = %q", r.Name)
	}
	if len(r.ServesCuisine) != 2 || r.ServesCuisine[0] != "Tacos" {
		t.Errorf("ServesCuisine = %v", r.ServesCuisine)
	}
	if r.Latitude < 19 || r.Latitude > 20 {
		t.Errorf("Latitude = %f", r.Latitude)
	}
	if r.RatingValue < 4 || r.RatingValue > 5 {
		t.Errorf("RatingValue = %f", r.RatingValue)
	}
	if r.RatingCount != 64 {
		t.Errorf("RatingCount = %d", r.RatingCount)
	}
	if len(r.OpeningHours) != 1 {
		t.Errorf("OpeningHours len = %d", len(r.OpeningHours))
	} else if r.OpeningHours[0].DayOfWeek != "Tuesday" {
		t.Errorf("OpeningHours[0].DayOfWeek = %q", r.OpeningHours[0].DayOfWeek)
	}
	if r.Neighborhood != "HIPODROMO CONDESA" {
		t.Errorf("Neighborhood = %q", r.Neighborhood)
	}
}

func TestParseRestaurantList(t *testing.T) {
	html := []byte(`<html>
<script type="application/ld+json">{"@type":"ItemList","itemListElement":[
  {"@type":"ListItem","position":1,"name":"El Farolito","url":"https://www.rappi.com.mx/restaurantes/10000295-el-farolito"}
]}</script>
<script type="application/ld+json">{"@type":"ItemList","itemListElement":[
  {"@type":"ListItem","position":1,"item":{"@type":"Restaurant","name":"El Farolito","url":"https://www.rappi.com.mx/restaurantes/10000295-el-farolito","servesCuisine":"Tacos","aggregateRating":{"ratingValue":4.9,"reviewCount":1955}}}
]}</script>
</html>`)
	rows := ParseRestaurantList(html, "ciudad-de-mexico", "tacos")
	if len(rows) != 1 {
		t.Fatalf("ParseRestaurantList len = %d", len(rows))
	}
	row := rows[0]
	if row.ID != "10000295" {
		t.Errorf("ID = %q", row.ID)
	}
	if row.RatingValue != 4.9 {
		t.Errorf("Rating = %f", row.RatingValue)
	}
	if row.RatingCount != 1955 {
		t.Errorf("ReviewCount = %d", row.RatingCount)
	}
	if row.City != "ciudad-de-mexico" || row.Category != "tacos" {
		t.Errorf("City/Category propagation failed: %q / %q", row.City, row.Category)
	}
}

func TestParseStoreList(t *testing.T) {
	html := []byte(`<html>
	<script type="application/ld+json">{"@type":"ItemList","itemListElement":[
	  {"@type":"ListItem","position":1,"name":"La Comer","url":"https://www.rappi.com.mx/tiendas/1930163835-la-comer"},
  {"@type":"ListItem","position":2,"name":"Chedraui","url":"https://www.rappi.com.mx/tiendas/1930102470-chedraui"}
]}</script>
</html>`)
	stores := ParseStoreList(html, "market", "ciudad-de-mexico")
	if len(stores) != 2 {
		t.Fatalf("len = %d", len(stores))
	}
	if stores[0].ID != "1930163835" || stores[0].Name != "La Comer" {
		t.Errorf("Store[0] = %+v", stores[0])
	}
	if stores[0].StoreType != "market" || stores[0].City != "ciudad-de-mexico" {
		t.Errorf("Store[0] tag propagation failed: %+v", stores[0])
	}
}

func TestParseStore(t *testing.T) {
	html := []byte(`<html>
	<script type="application/ld+json">{"@context":"http://schema.org/","@type":"Store","name":"La Comer","image":"https://images.rappi.com.mx/marketplace/la_comer.jpg","url":"https://www.rappi.com.mx/tiendas/1930163835-la-comer","geo":{"@type":"GeoCoordinates","latitude":19.37513545,"longitude":-99.16711854}}</script>
	</html>`)
	store := ParseStore(html)
	if store == nil {
		t.Fatal("ParseStore returned nil")
	}
	if store.ID != "1930163835" || store.Name != "La Comer" {
		t.Fatalf("store identity = %+v", store)
	}
	if store.Latitude == 0 || store.Longitude == 0 {
		t.Fatalf("store coordinates missing: %+v", store)
	}
}

func TestExtractNeighborhood(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"ALTATA No. 19. COL. HIPODROMO CONDESA, CUAUHTEMOC.", "HIPODROMO CONDESA"},
		{"Av. Insurgentes Sur 123 Col. Roma Norte, CDMX", "Roma Norte"},
		{"Some address with no col marker", ""},
		{"", ""},
	}
	for _, tc := range cases {
		got := extractNeighborhood(tc.in)
		if got != tc.want {
			t.Errorf("extractNeighborhood(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestEnums(t *testing.T) {
	if c := CityBySlug("ciudad-de-mexico"); c == nil || c.Name != "Ciudad de México" {
		t.Errorf("CityBySlug(ciudad-de-mexico) = %+v", c)
	}
	if c := CityBySlug("does-not-exist"); c != nil {
		t.Errorf("CityBySlug(does-not-exist) = %+v, want nil", c)
	}
	if c := RestaurantCategoryBySlug("tacos"); c == nil || c.Spanish != "Tacos" {
		t.Errorf("RestaurantCategoryBySlug(tacos) = %+v", c)
	}
	if s := StoreTypeBySlug("market"); s == nil || s.Spanish != "Supermercados" {
		t.Errorf("StoreTypeBySlug(market) = %+v", s)
	}
}
