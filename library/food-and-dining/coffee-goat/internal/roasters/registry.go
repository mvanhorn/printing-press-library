// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

// Package roasters is the curated 24-roaster static registry that
// drives every multi-source command (sync, search, twin, watch,
// producer, etc.). Each row carries the slug used everywhere in the
// store, the display name shown to humans, an ISO country code, the
// storefront transport identifier, and the sync URL that the matching
// source adapter knows how to read.
//
// This is a `// pp:novel-static-reference` table: the data is
// hand-curated from the scoping conversation's reachability probes
// and the prior-brief Tier table, not synthesized at runtime. Source:
// research/prior-brief.md "Reachability Probes" (24/24 reachable).
package roasters

// Roaster identifies one of the 24 specialty-coffee storefronts the
// CLI aggregates over. Slug is the immutable identifier used in
// every store table; Filters carries source-specific JSON-encoded
// hints (e.g. Shopify product_type narrowing for Black & White).
type Roaster struct {
	Slug      string
	Name      string
	Country   string
	Transport string
	SyncURL   string
	Filters   map[string]string
}

// Transport identifiers — one per storefront family. The matching
// adapter package under internal/source/<transport>/ owns the fetch
// path.
const (
	TransportShopify      = "shopify"
	TransportWooCommerce  = "woocommerce"
	TransportSquareOnline = "square-online"
	TransportSnipcart     = "snipcart"
)

var registry = []Roaster{
	// Tier-1 Shopify (21 storefronts confirmed 200 in scoping probes)
	{Slug: "glitch", Name: "Glitch Coffee", Country: "JP", Transport: TransportShopify, SyncURL: "https://glitchcoffeeroasters.myshopify.com/products.json"},
	{Slug: "leaves", Name: "Leaves Coffee Roasters", Country: "JP", Transport: TransportShopify, SyncURL: "https://leaves-coffee-roasters.myshopify.com/products.json"},
	{Slug: "prodigal", Name: "Prodigal Coffee", Country: "US", Transport: TransportShopify, SyncURL: "https://prodigalcoffee.com/products.json"},
	{Slug: "black-and-white", Name: "Black & White Coffee", Country: "US", Transport: TransportShopify, SyncURL: "https://blackwhiteroasters.com/products.json", Filters: map[string]string{"product_type": "Coffee"}},
	{Slug: "onyx", Name: "Onyx Coffee Lab", Country: "US", Transport: TransportShopify, SyncURL: "https://onyxcoffeelab.com/products.json"},
	{Slug: "sey", Name: "Sey Coffee", Country: "US", Transport: TransportShopify, SyncURL: "https://seycoffee.com/products.json"},
	{Slug: "tim-wendelboe", Name: "Tim Wendelboe", Country: "NO", Transport: TransportShopify, SyncURL: "https://www.timwendelboe.no/products.json"},
	{Slug: "hydrangea", Name: "Hydrangea Coffee", Country: "US", Transport: TransportShopify, SyncURL: "https://hydrangea.coffee/products.json"},
	{Slug: "passenger", Name: "Passenger Coffee", Country: "US", Transport: TransportShopify, SyncURL: "https://passengercoffee.com/products.json"},
	{Slug: "george-howell", Name: "George Howell Coffee", Country: "US", Transport: TransportShopify, SyncURL: "https://georgehowellcoffee.com/products.json"},
	{Slug: "heart", Name: "Heart Roasters", Country: "US", Transport: TransportShopify, SyncURL: "https://www.heartroasters.com/products.json"},
	{Slug: "saint-frank", Name: "Saint Frank Coffee", Country: "US", Transport: TransportShopify, SyncURL: "https://saintfrankcoffee.com/products.json"},
	{Slug: "verve", Name: "Verve Coffee Roasters", Country: "US", Transport: TransportShopify, SyncURL: "https://www.vervecoffee.com/products.json"},
	{Slug: "proud-mary", Name: "Proud Mary Coffee", Country: "AU", Transport: TransportShopify, SyncURL: "https://proudmarycoffee.com/products.json"},
	{Slug: "square-mile", Name: "Square Mile Coffee", Country: "GB", Transport: TransportShopify, SyncURL: "https://shop.squaremilecoffee.com/products.json"},
	{Slug: "april", Name: "April Coffee Roasters", Country: "DK", Transport: TransportShopify, SyncURL: "https://aprilcoffeeroasters.com/products.json"},
	{Slug: "coffee-collective", Name: "Coffee Collective", Country: "DK", Transport: TransportShopify, SyncURL: "https://coffeecollective.dk/products.json"},
	{Slug: "manhattan", Name: "Manhattan Coffee Roasters", Country: "NL", Transport: TransportShopify, SyncURL: "https://manhattancoffeeroasters.com/products.json"},
	{Slug: "friedhats", Name: "Friedhats", Country: "NL", Transport: TransportShopify, SyncURL: "https://www.friedhats.com/products.json"},
	{Slug: "the-barn", Name: "The Barn", Country: "DE", Transport: TransportShopify, SyncURL: "https://www.thebarn.de/products.json"},
	{Slug: "la-cabra", Name: "La Cabra Coffee", Country: "DK", Transport: TransportShopify, SyncURL: "https://lacabra.dk/products.json"},

	// Tier-1b WooCommerce (1 storefront)
	{Slug: "mame", Name: "Mame Coffee", Country: "CH", Transport: TransportWooCommerce, SyncURL: "https://mame.coffee/wp-json/wc/store/v1/products"},

	// Tier-2 sniffed (2 storefronts; HTML scrape adapters not built in this phase)
	{Slug: "loquat", Name: "Loquat Coffee", Country: "US", Transport: TransportSquareOnline, SyncURL: "https://www.loquatcoffee.com/"},
	{Slug: "dak", Name: "DAK Coffee Roasters", Country: "NL", Transport: TransportSnipcart, SyncURL: "https://dakcoffeeroasters.com/"},
}

// All returns every roaster in the registry. Callers may inspect but
// must not mutate the returned slice; the registry is a singleton.
func All() []Roaster {
	out := make([]Roaster, len(registry))
	copy(out, registry)
	return out
}

// BySlug returns the roaster matching slug, or (zero-value, false)
// when slug is unknown. Used by commands that take a `<roaster>`
// argument to validate it before hitting the store.
func BySlug(slug string) (Roaster, bool) {
	for _, r := range registry {
		if r.Slug == slug {
			return r, true
		}
	}
	return Roaster{}, false
}

// ShopifyOnly returns the subset of roasters whose Transport is
// "shopify". Used by the Shopify source adapter's bulk-sync entrypoint
// and by --source shopify CLI invocations.
func ShopifyOnly() []Roaster {
	var out []Roaster
	for _, r := range registry {
		if r.Transport == TransportShopify {
			out = append(out, r)
		}
	}
	return out
}

// Count returns the number of registered roasters. Exists so tests
// and commands can assert the registry size without leaking the
// internal slice.
func Count() int {
	return len(registry)
}
