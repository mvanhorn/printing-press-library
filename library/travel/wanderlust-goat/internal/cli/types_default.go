package cli

import "strings"

// defaultIncludedTypesFromCriteria maps free-text criteria to Google Places
// API (New) type filters when the user does not pass --type. Without this,
// the seed call returns generic "anything-near-here" candidates including
// schools, electronics stores, museums, parks. With it, "high-end seafood"
// narrows the seed to seafood/sushi/japanese restaurants.
//
// Returns nil when no rule matches; the caller falls back to the default
// Google Places behavior (no filter).
func defaultIncludedTypesFromCriteria(criteria, identity string) []string {
	body := strings.ToLower(criteria + " " + identity)
	hits := map[string]bool{}
	add := func(types ...string) {
		for _, t := range types {
			hits[t] = true
		}
	}
	// Order matters only for readability; we union into a set.
	if anyOf(body, "seafood", "sushi", "kaiseki", "kappo", "uni", "ikura", "fish market", "raw fish", "sashimi") {
		add("sushi_restaurant", "seafood_restaurant", "japanese_restaurant")
	}
	if anyOf(body, "ramen", "noodle", "udon", "soba", "tsukemen") {
		add("ramen_restaurant", "japanese_restaurant")
	}
	if anyOf(body, "coffee", "kissaten", "espresso", "barista", "pour-over", "pour over", "café", "cafe", "third wave", "specialty coffee") {
		add("cafe", "coffee_shop")
	}
	if anyOf(body, "kbbq", "korean bbq", "yakiniku", "barbecue", "bbq") {
		add("barbecue_restaurant", "korean_restaurant")
	}
	if anyOf(body, "korean", "kimchi", "bibimbap", "tteokbokki") {
		add("korean_restaurant")
	}
	if anyOf(body, "bistro", "natural wine", "wine bar", "tapas", "small plates") {
		add("wine_bar", "bar", "restaurant")
	}
	if anyOf(body, "bakery", "patisserie", "pastry", "boulangerie", "viennoiserie") {
		add("bakery")
	}
	if anyOf(body, "cocktail", "bar ", " bar,", "speakeasy", "whisky", "whiskey") {
		add("bar")
	}
	if anyOf(body, "dim sum", "chinese", "cantonese", "szechuan", "sichuan") {
		add("chinese_restaurant")
	}
	if anyOf(body, "italian", "pizza", "pasta", "trattoria", "osteria") {
		add("italian_restaurant", "pizza_restaurant")
	}
	if anyOf(body, "thai", "vietnamese", "pho", "banh mi", "indian", "curry house") {
		add("thai_restaurant", "vietnamese_restaurant", "indian_restaurant", "restaurant")
	}
	if anyOf(body, "viewpoint", "scenic", "vista", "panorama", "lookout", "photo spot", "photogenic", "shoot", "photographer") {
		add("tourist_attraction")
	}
	if anyOf(body, "historic", "heritage", "meiji", "edo", "warehouse", "old building", "monument", "shrine", "temple", "castle") {
		add("historical_landmark", "tourist_attraction")
	}
	if anyOf(body, "museum", "gallery", "art center", "art gallery") {
		add("museum", "art_gallery")
	}
	if anyOf(body, "vintage clothing", "vintage shop", "thrift", "antique", "record store", "bookstore", "stationery") {
		add("clothing_store", "store", "book_store")
	}
	if anyOf(body, "morning market", "fish market", "market") {
		add("market", "tourist_attraction")
	}
	// Default if nothing else matched but user is clearly food-oriented.
	if len(hits) == 0 && anyOf(body, "food", "eat", "dinner", "lunch", "breakfast", "amazing", "high-end", "michelin", "chef", "omakase", "tasting menu") {
		add("restaurant")
	}
	if len(hits) == 0 {
		return nil
	}
	out := make([]string, 0, len(hits))
	for t := range hits {
		out = append(out, t)
	}
	return out
}

func anyOf(s string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}
