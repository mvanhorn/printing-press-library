package cli

import (
	"encoding/json"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/crestron/internal/crestronparse"
)

// crestronExtract routes a Crestron.com HTML response through the parser that
// understands that specific page, falling back to the generic extractor for
// anything unrecognized.
//
// Why this exists: Crestron has no API, so every endpoint returns a full
// server-rendered page. The generic `page` extractor returns site chrome —
// nav banners and "Learn More" links — rather than the result rows a caller
// asked for. Each branch below keys off the request path, which is in scope at
// every generated call site.
//
// Regen note: the generated endpoint commands call this instead of
// extractHTMLResponse. That is a one-line swap per command; `regen-merge`
// reports it as reviewable drift. The parsers themselves live in
// internal/crestronparse and are untouched by regeneration.
func crestronExtract(data []byte, path string, opts htmlExtractionOptions) ([]byte, error) {
	switch {
	case strings.HasPrefix(path, "/Support/Search-Results"):
		page, err := crestronparse.ParseSearchResults(data)
		if err != nil {
			break
		}
		return json.Marshal(page)

	case strings.HasPrefix(path, "/CMSPages/ProductSubcategoryItemTemplate.aspx"):
		products, total, err := crestronparse.ParseProductTiles(data)
		if err != nil {
			break
		}
		return json.Marshal(struct {
			Products []crestronparse.Product `json:"products"`
			Total    int                     `json:"total"`
			Count    int                     `json:"count"`
		}{Products: products, Total: total, Count: len(products)})

	case strings.HasPrefix(path, "/Handlers/ResourceHandler.ashx"):
		assets, err := crestronparse.ParseAssets(data)
		if err != nil {
			break
		}
		return json.Marshal(struct {
			Assets []crestronparse.Asset `json:"assets"`
			Count  int                   `json:"count"`
		}{Assets: assets, Count: len(assets)})

	case strings.HasPrefix(path, "/Handlers/VariantProduct.ashx"),
		strings.HasPrefix(path, "/Handlers/OptionalAccessoriesHandler.ashx"),
		strings.HasPrefix(path, "/Handlers/ReplacementProductsHandler.ashx"):
		models, err := crestronparse.ParseModelTable(data)
		if err != nil {
			break
		}
		return json.Marshal(struct {
			Models []crestronparse.ModelRow `json:"models"`
			Count  int                      `json:"count"`
		}{Models: models, Count: len(models)})

	case strings.HasPrefix(path, "/Software-Firmware/"):
		fr, err := crestronparse.ParseFirmwareRelease(data)
		if err != nil {
			break
		}
		return json.Marshal(fr)

	case strings.HasPrefix(path, "/sitemap"):
		paths, err := crestronparse.ParseCatalogPaths(data)
		if err != nil {
			break
		}
		return json.Marshal(struct {
			Categories []string `json:"categories"`
			Count      int      `json:"count"`
		}{Categories: paths, Count: len(paths)})

	case strings.HasPrefix(path, "/Products/Catalog/"):
		// Category pages and product pages share the same path prefix AND both
		// embed schema.org JSON-LD of type Product — a category's JSON-LD names
		// the category. So JSON-LD presence cannot disambiguate them. Only
		// category pages carry the inline request block, so test that first.
		if crestronparse.IsCategoryPage(data) {
			if c, err := crestronparse.ParseCategoryPage(data, opts.BaseURL); err == nil && c.DocumentID != "" {
				return json.Marshal(c)
			}
		}
		if p, err := crestronparse.ParseProductPage(data, opts.BaseURL); err == nil && p.Model != "" {
			specs, _ := crestronparse.ParseSpecTable(data)
			assets, _ := crestronparse.ParseAssets(data)
			return json.Marshal(struct {
				crestronparse.Product
				Specs  []crestronparse.SpecSection `json:"specs,omitempty"`
				Assets []crestronparse.Asset       `json:"assets,omitempty"`
			}{Product: p, Specs: specs, Assets: assets})
		}

	case strings.HasPrefix(path, "/handlers/Header.ashx"),
		strings.HasPrefix(path, "/Handlers/header.ashx"):
		signedIn := crestronparse.ParseSignedIn(data)
		return json.Marshal(struct {
			SignedIn bool `json:"signed_in"`
		}{SignedIn: signedIn})
	}

	// Unrecognized path, or a parser that could not read this body: fall back
	// to the generic extractor rather than returning nothing.
	return extractHTMLResponse(data, opts)
}
