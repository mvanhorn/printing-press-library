// Package registry provides the seeded directory of verified Grade-A UCP merchants.
package registry

// Merchant is one entry in the UCP merchant registry.
type Merchant struct {
	Domain      string
	Grade       string // "A" or "B"
	Category    string // "pet", "fashion", "beauty", etc.
	HasRopeToys bool   // only true for known dog-toy stores
	Notes       string
}

// Default returns the seeded registry of verified Grade-A UCP merchants.
// Source: agent-team-analyzer Lane D probes, 2026-05-24.
// All are Shopify-hosted with `transport: mcp + embedded`, anonymous /products.json.
// Exception: kongcompany.com is Grade B (no catalog search capability).
func Default() []Merchant {
	return []Merchant{
		// Pet / Dog (the user's target)
		{Domain: "bark.co", Grade: "A", Category: "pet/dog", HasRopeToys: true, Notes: "Rope/tug toys confirmed in catalog (Corn Dog Tug, Lickin' Links, etc.)"},
		{Domain: "ruffwear.com", Grade: "A", Category: "pet/dog", HasRopeToys: true, Notes: "Dog gear brand; carries rope/tug toys"},
		{Domain: "sitstay.com", Grade: "A", Category: "pet/dog", HasRopeToys: true, Notes: "Specialty dog retailer; broad toy selection"},
		{Domain: "earthrated.com", Grade: "A", Category: "pet/eco", HasRopeToys: false, Notes: "Eco pet supplies; primarily bags/wipes"},
		{Domain: "kongcompany.com", Grade: "B", Category: "pet/dog", HasRopeToys: true, Notes: "KONG dog toys; checkout-only (no catalog search capability)"},
		// Food / Coffee
		{Domain: "checkout.coffeecircle.com", Grade: "A", Category: "food/coffee", Notes: "Coffee; products.json returns 500 (headless theme)"},
		{Domain: "taftcoffee.com", Grade: "A", Category: "food/coffee"},
		// Fashion
		{Domain: "everlane.com", Grade: "A", Category: "fashion"},
		{Domain: "skims.com", Grade: "A", Category: "fashion"},
		{Domain: "fashionnova.com", Grade: "A", Category: "fashion"},
		{Domain: "forever21.com", Grade: "A", Category: "fashion"},
		{Domain: "chubbiesshorts.com", Grade: "A", Category: "fashion"},
		{Domain: "untuckit.com", Grade: "A", Category: "fashion"},
		{Domain: "statelymen.com", Grade: "A", Category: "fashion"},
		{Domain: "lesbenjamins.com", Grade: "A", Category: "fashion"},
		// Footwear
		{Domain: "allbirds.com", Grade: "A", Category: "footwear"},
		{Domain: "keenfootwear.com", Grade: "A", Category: "footwear"},
		{Domain: "stevemadden.com", Grade: "A", Category: "footwear"},
		{Domain: "rothys.com", Grade: "A", Category: "footwear"},
		// Activewear
		{Domain: "gymshark.com", Grade: "A", Category: "activewear", Notes: "via us.checkout.gymshark.com"},
		{Domain: "aloyoga.com", Grade: "A", Category: "activewear"},
		{Domain: "outdoorvoices.com", Grade: "A", Category: "activewear"},
		{Domain: "ripcurl.com", Grade: "A", Category: "activewear"},
		{Domain: "quiksilver.com", Grade: "A", Category: "activewear"},
		{Domain: "billabong.com", Grade: "A", Category: "activewear"},
		// Beauty
		{Domain: "glossier.com", Grade: "A", Category: "beauty"},
		{Domain: "kyliecosmetics.com", Grade: "A", Category: "beauty"},
		{Domain: "fentybeauty.com", Grade: "A", Category: "beauty"},
		{Domain: "olaplex.com", Grade: "A", Category: "beauty"},
		{Domain: "moroccanoil.com", Grade: "A", Category: "beauty"},
		{Domain: "peachandlily.com", Grade: "A", Category: "beauty"},
		{Domain: "thebodyshop.com", Grade: "A", Category: "beauty"},
		{Domain: "sokoglam.com", Grade: "A", Category: "beauty"},
		{Domain: "colourpop.com", Grade: "A", Category: "beauty"},
		{Domain: "terrebleu.ca", Grade: "A", Category: "beauty"},
		{Domain: "korendy.com.tr", Grade: "A", Category: "beauty"},
		// Grooming / Personal Care
		{Domain: "harrys.com", Grade: "A", Category: "grooming"},
		{Domain: "dollarshaveclub.com", Grade: "A", Category: "grooming"},
		{Domain: "nativecos.com", Grade: "A", Category: "personal-care"},
		// Home / Furniture / Bedding
		{Domain: "casper.com", Grade: "A", Category: "home/mattress"},
		{Domain: "tuftandneedle.com", Grade: "A", Category: "home/mattress"},
		{Domain: "brooklinen.com", Grade: "A", Category: "home/bedding"},
		{Domain: "bollandbranch.com", Grade: "A", Category: "home/bedding"},
		{Domain: "pier1.com", Grade: "A", Category: "home/furniture"},
		{Domain: "burrow.com", Grade: "A", Category: "home/furniture"},
		{Domain: "publicgoods.com", Grade: "A", Category: "household"},
		{Domain: "grove.co", Grade: "A", Category: "household"},
		// Luggage / Travel
		{Domain: "monos.com", Grade: "A", Category: "luggage"},
		{Domain: "awaytravel.com", Grade: "A", Category: "luggage"},
		// Kids / Baby
		{Domain: "kytebaby.com", Grade: "A", Category: "baby"},
		{Domain: "primary.com", Grade: "A", Category: "kids-apparel"},
		{Domain: "mattel.com", Grade: "A", Category: "toys"},
		// Jewelry / Accessories
		{Domain: "mejuri.com", Grade: "A", Category: "jewelry"},
		{Domain: "puravidabracelets.com", Grade: "A", Category: "accessories"},
		{Domain: "flavus.com", Grade: "A", Category: "accessories"},
		// Outdoor / Sports
		{Domain: "decathlon.com", Grade: "A", Category: "sports/outdoors"},
		{Domain: "kelty.com", Grade: "A", Category: "outdoor-gear"},
		// Audio / Electronics / Misc
		{Domain: "rayconglobal.com", Grade: "A", Category: "audio"},
		{Domain: "nothing.tech", Grade: "A", Category: "electronics"},
		{Domain: "fender.com", Grade: "A", Category: "music"},
		{Domain: "tonal.com", Grade: "A", Category: "fitness"},
		{Domain: "hungryminds.com", Grade: "A", Category: "publishing"},
	}
}
