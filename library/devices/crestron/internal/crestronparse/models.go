package crestronparse

import (
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// ModelRow is one row of a Crestron model table — the shape returned by the
// VariantProduct, OptionalAccessories, and ReplacementProducts handlers.
type ModelRow struct {
	Model       string `json:"model"`
	Description string `json:"description,omitempty"`
	Price       string `json:"price,omitempty"`
	URL         string `json:"url,omitempty"`
	// InternalID is Crestron's numeric product id when the row exposes one.
	InternalID string `json:"internal_id,omitempty"`
}

// ParseModelTable reads the "Available Models" / accessories / replacements
// fragments. All three handlers render rows of class `sibling-row` whose parts
// carry dedicated classes: `.sibling-name` (model), `.sibling-number` (internal
// id), `.sibling-brand` (description), `.sibling-price` (price).
func ParseModelTable(body []byte) ([]ModelRow, error) {
	out := make([]ModelRow, 0)
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return out, err
	}

	seen := map[string]bool{}
	for _, row := range findAllByClass(doc, "sibling-row") {
		var m ModelRow
		if n := firstByClass(row, "sibling-name"); n != nil {
			m.Model = textOf(n)
		}
		if n := firstByClass(row, "sibling-number"); n != nil {
			m.InternalID = strings.TrimSpace(textOf(n))
		}
		if n := firstByClass(row, "sibling-brand"); n != nil {
			m.Description = textOf(n)
		}
		if n := firstByClass(row, "sibling-price"); n != nil {
			if v := attr(n, "data-model"); v != "" && m.InternalID == "" {
				m.InternalID = v
			}
			if t := textOf(n); t != "" {
				m.Price = t
			}
		}
		for _, a := range findAllTags(row, "a") {
			if h := attr(a, "href"); strings.Contains(h, "/Products/Catalog/") {
				m.URL = strings.TrimPrefix(h, "~")
				break
			}
		}
		// A header row repeats the column labels rather than a model.
		if m.Model == "" || strings.EqualFold(m.Model, "Model") || seen[m.Model] {
			continue
		}
		seen[m.Model] = true
		out = append(out, m)
	}
	return out, nil
}

// categoryMarkerRe is the definitive signal that a /Products/Catalog/ page is a
// category listing rather than a product detail page. Both page types embed
// schema.org JSON-LD of type Product — a category's JSON-LD names the category
// — so JSON-LD alone cannot tell them apart. Only category pages carry the
// inline request block that drives the product-tile endpoint.
var categoryMarkerRe = regexp.MustCompile(`documentId:\s*'\d+'`)

// IsCategoryPage reports whether a /Products/Catalog/ response is a category
// listing rather than a product detail page.
func IsCategoryPage(body []byte) bool {
	return categoryMarkerRe.Match(body)
}

// ParseSignedIn reports whether a Crestron.com header fragment or page shows an
// authenticated session. Crestron renders a "Sign In" link for anonymous
// visitors and replaces it with account links once signed in.
func ParseSignedIn(body []byte) bool {
	s := string(body)
	if strings.Contains(s, "Crestron Authentication - Sign In") {
		return false
	}
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return false
	}
	var sawSignIn, sawSignOut bool
	for _, a := range findAllTags(doc, "a") {
		h := strings.ToLower(attr(a, "href"))
		t := strings.ToLower(textOf(a))
		if strings.Contains(h, "logout") || strings.Contains(t, "sign out") {
			sawSignOut = true
		}
		if strings.Contains(h, "/login") || t == "sign in" {
			sawSignIn = true
		}
	}
	if sawSignOut {
		return true
	}
	return !sawSignIn
}
