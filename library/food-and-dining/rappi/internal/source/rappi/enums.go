// Copyright 2026 bobeglz. Licensed under Apache-2.0. See LICENSE.
package rappi

import "strings"

// City is a Mexican city Rappi serves with a baked-in default centroid
// coordinate. The centroid is the canonical lat/lng used by `restaurants
// near` and other geo-scoped commands when --lat/--lng aren't provided.
type City struct {
	Slug      string  `json:"slug"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	// State is the Mexican state for filtering and display.
	State string `json:"state"`
}

// Cities is the closed enum of supported MX cities. Values came from
// Phase 1 research; coordinates target each city's historic center.
var Cities = []City{
	{Slug: "ciudad-de-mexico", Name: "Ciudad de México", Latitude: 19.4326, Longitude: -99.1332, State: "CDMX"},
	{Slug: "guadalajara", Name: "Guadalajara", Latitude: 20.6597, Longitude: -103.3496, State: "Jalisco"},
	{Slug: "monterrey", Name: "Monterrey", Latitude: 25.6866, Longitude: -100.3161, State: "Nuevo León"},
	{Slug: "puebla", Name: "Puebla", Latitude: 19.0414, Longitude: -98.2063, State: "Puebla"},
	{Slug: "queretaro", Name: "Querétaro", Latitude: 20.5888, Longitude: -100.3899, State: "Querétaro"},
	{Slug: "leon", Name: "León", Latitude: 21.1250, Longitude: -101.6859, State: "Guanajuato"},
	{Slug: "tijuana", Name: "Tijuana", Latitude: 32.5149, Longitude: -117.0382, State: "Baja California"},
	{Slug: "merida", Name: "Mérida", Latitude: 20.9670, Longitude: -89.5926, State: "Yucatán"},
	{Slug: "naucalpan", Name: "Naucalpan", Latitude: 19.4796, Longitude: -99.2386, State: "México"},
	{Slug: "coyoacan", Name: "Coyoacán", Latitude: 19.3467, Longitude: -99.1617, State: "CDMX"},
}

// CityBySlug returns the City entry for a slug, or nil if not in the
// closed enum.
func CityBySlug(slug string) *City {
	slug = strings.ToLower(strings.TrimSpace(slug))
	for i := range Cities {
		if Cities[i].Slug == slug {
			return &Cities[i]
		}
	}
	return nil
}

// CitySlugs returns just the slug strings in their canonical order.
func CitySlugs() []string {
	out := make([]string, 0, len(Cities))
	for _, c := range Cities {
		out = append(out, c.Slug)
	}
	return out
}

// RestaurantCategory is a closed Rappi restaurant cuisine slug + Spanish
// display name. Discovered during browser-sniff from the CDMX restaurants
// page; verified against rappi.com.mx/restaurantes navigation.
type RestaurantCategory struct {
	Slug    string `json:"slug"`
	Spanish string `json:"name_es"`
	English string `json:"name_en"`
}

// RestaurantCategories is the closed enum of cuisine slugs. Filterable
// by `restaurants list-category` and used as input for `restaurants
// multi-category`, `restaurants top`, etc.
var RestaurantCategories = []RestaurantCategory{
	{Slug: "hamburguesas", Spanish: "Hamburguesas", English: "Burgers"},
	{Slug: "pizza", Spanish: "Pizza", English: "Pizza"},
	{Slug: "sushi", Spanish: "Sushi", English: "Sushi"},
	{Slug: "tacos", Spanish: "Tacos", English: "Tacos"},
	{Slug: "pollo", Spanish: "Pollo", English: "Chicken"},
	{Slug: "mexicana", Spanish: "Mexicana", English: "Mexican"},
	{Slug: "saludables", Spanish: "Saludables", English: "Healthy"},
	{Slug: "postres", Spanish: "Postres", English: "Desserts"},
	{Slug: "cafe", Spanish: "Café", English: "Coffee"},
	{Slug: "pasteleria", Spanish: "Pastelería", English: "Pastry"},
}

// RestaurantCategoryBySlug returns the category for a slug, or nil.
func RestaurantCategoryBySlug(slug string) *RestaurantCategory {
	slug = strings.ToLower(strings.TrimSpace(slug))
	for i := range RestaurantCategories {
		if RestaurantCategories[i].Slug == slug {
			return &RestaurantCategories[i]
		}
	}
	return nil
}

// StoreType is a closed Rappi store-type slug + display name.
type StoreType struct {
	Slug    string `json:"slug"`
	Spanish string `json:"name_es"`
	English string `json:"name_en"`
}

// StoreTypes is the closed enum of /tiendas/tipo/<slug> values.
var StoreTypes = []StoreType{
	{Slug: "market", Spanish: "Supermercados", English: "Supermarkets"},
	{Slug: "farmatodo", Spanish: "Farmacias", English: "Pharmacies"},
	{Slug: "liquor", Spanish: "Licorerías", English: "Liquor stores"},
	{Slug: "express", Spanish: "Express", English: "Convenience"},
	{Slug: "rappimall-parent", Spanish: "RappiMall", English: "Marketplace"},
}

// StoreTypeBySlug returns the store type for a slug, or nil.
func StoreTypeBySlug(slug string) *StoreType {
	slug = strings.ToLower(strings.TrimSpace(slug))
	for i := range StoreTypes {
		if StoreTypes[i].Slug == slug {
			return &StoreTypes[i]
		}
	}
	return nil
}
