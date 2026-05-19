package scraper

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/mvanhorn/printing-press-library/library/commerce/tennis-warehouse/internal/cliutil"
)

// ParseUsedCatalog parses /usedcatpage.html?ccode=X — a list of used MODELS
// stocked for a brand. Each card carries a data-pcode attribute (the model
// SKU) and price data. Returns a list of pcode + summary tuples; full specs
// require a subsequent fetch of /orderusedproduct.html?pcode=...
func ParseUsedCatalog(html string) ([]UsedModel, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	var out []UsedModel
	seen := make(map[string]bool)
	// Cards use data-code (NOT data-pcode) for the model SKU; data-gtm_impression_code
	// is the same value. Filter to elements that ALSO carry product-impression metadata
	// so we don't accidentally grab unit-level rows.
	doc.Find("[data-code][data-gtm_impression_code]").Each(func(i int, sel *goquery.Selection) {
		pcode, _ := sel.Attr("data-code")
		if pcode == "" || seen[pcode] {
			return
		}
		seen[pcode] = true
		m := UsedModel{PCode: pcode}
		if v, ok := sel.Attr("data-prod_name"); ok {
			m.Model = cliutil.CleanText(strings.TrimSpace(v))
		}
		if v, ok := sel.Attr("data-gtm_impression_brand"); ok {
			m.Brand = strings.TrimSpace(v)
		}
		if v, ok := sel.Attr("data-price_low"); ok {
			m.PriceLow = parseFloat(v)
		}
		if v, ok := sel.Attr("data-price_high"); ok {
			m.PriceHigh = parseFloat(v)
		}
		if v, ok := sel.Attr("data-old_price_low"); ok {
			m.MSRP = parseFloat(v)
		}
		if m.PriceLow == 0 {
			if v, ok := sel.Attr("data-gtm_impression_price"); ok {
				m.PriceLow = parseFloat(v)
				m.PriceHigh = m.PriceLow
			}
		}
		// Resolve the detail URL — the surrounding <a> usually carries it.
		linkAttr := ""
		sel.Find("a[href*=orderusedproduct]").EachWithBreak(func(i int, a *goquery.Selection) bool {
			if h, ok := a.Attr("href"); ok {
				linkAttr = collapseWhitespace(h)
				return false
			}
			return true
		})
		if linkAttr == "" {
			anc := sel.ParentsFiltered("a[href*=orderusedproduct]").First()
			if h, ok := anc.Attr("href"); ok {
				linkAttr = collapseWhitespace(h)
			}
		}
		if linkAttr != "" {
			if !strings.HasPrefix(linkAttr, "http") {
				linkAttr = "https://www.tennis-warehouse.com" + linkAttr
			}
			m.URL = linkAttr
		} else {
			m.URL = "https://www.tennis-warehouse.com/orderusedproduct.html?pcode=" + pcode
		}
		// Image.
		sel.Find("img[src]").EachWithBreak(func(i int, img *goquery.Selection) bool {
			if src, _ := img.Attr("src"); src != "" {
				m.ImageURL = src
				return false
			}
			return true
		})
		out = append(out, m)
	})
	return out, nil
}

// ParseRacquetCatalog parses /{Brand}racquets.html — a list of CURRENT
// racquets for a brand. Each card carries data-code (SKU) plus data-gtm_impression_*
// attributes (brand, price, name). Inline description text often includes mini-specs.
func ParseRacquetCatalog(html, brand string) ([]Racquet, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	var out []Racquet
	seen := make(map[string]bool)
	doc.Find("[data-code][data-gtm_impression_code]").Each(func(i int, sel *goquery.Selection) {
		sku, _ := sel.Attr("data-code")
		if sku == "" || seen[sku] {
			return
		}
		seen[sku] = true
		r := Racquet{SKU: sku}
		if v, ok := sel.Attr("data-prod_name"); ok {
			r.Model = cliutil.CleanText(strings.TrimSpace(v))
		}
		if r.Model == "" {
			if v, ok := sel.Attr("data-gtm_impression_name"); ok {
				r.Model = cliutil.CleanText(strings.TrimSpace(v))
			}
		}
		if v, ok := sel.Attr("data-gtm_impression_brand"); ok {
			r.Brand = strings.TrimSpace(v)
		}
		if r.Brand == "" {
			r.Brand = strings.Title(brand)
		}
		if v, ok := sel.Attr("data-gtm_impression_price"); ok {
			r.Price = parseFloat(v)
		}
		if v, ok := sel.Attr("data-price_low"); ok && r.Price == 0 {
			r.Price = parseFloat(v)
		}
		if v, ok := sel.Attr("data-old_price_low"); ok {
			r.MSRP = parseFloat(v)
		}
		// Resolve the detail URL from a descpageRC link.
		sel.Find(`a[href*="descpageRC"]`).EachWithBreak(func(i int, a *goquery.Selection) bool {
			h, _ := a.Attr("href")
			h = collapseWhitespace(h)
			if h == "" {
				return true
			}
			if !strings.HasPrefix(h, "http") {
				h = "https://www.tennis-warehouse.com" + h
			}
			r.URL = h
			return false
		})
		if r.URL == "" {
			// Build a fallback (the brand-uppercase + SKU suffix). Detail
			// pages also live at /<Brand>_<Model_Name>/descpageRC<BRAND>-<sku>.html;
			// without the model-name slug we can't fully reconstruct it.
			r.URL = "https://www.tennis-warehouse.com/descpageRC" + strings.ToUpper(r.Brand) + "-" + sku + ".html"
		}
		// Image.
		sel.Find("img[src]").EachWithBreak(func(i int, img *goquery.Selection) bool {
			if src, _ := img.Attr("src"); src != "" {
				r.ImageURL = src
				return false
			}
			return true
		})
		// Inline mini-specs in the card text.
		txt := sel.Text()
		if m := reHeadSize.FindStringSubmatch(txt); len(m) > 1 {
			r.HeadSizeIn2, _ = strconv.ParseFloat(m[1], 64)
		}
		if m := reStringPat.FindStringSubmatch(txt); len(m) > 1 {
			r.StringPattern = m[1]
		}
		// Status flag inference.
		if _, ok := sel.Attr("data-newitem"); ok {
			r.Status = "new"
		}
		if _, ok := sel.Attr("data-reduced"); ok && r.Status == "" {
			r.Status = "reduced"
		}
		if _, ok := sel.Attr("data-closeout"); ok {
			r.Status = "closeout"
		}
		out = append(out, r)
	})
	return out, nil
}

var reHeadSize = regexp.MustCompile(`Headsize:\s*(\d{2,3}(?:\.\d+)?)\s*in`)
var reStringPat = regexp.MustCompile(`String Pattern:\s*(\d{1,2}x\d{1,2})`)
var reDescpage = regexp.MustCompile(`descpageRC[A-Z]+-([A-Z0-9-]+)\.html`)

func skuFromDescpageHref(href string) string {
	m := reDescpage.FindStringSubmatch(href)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

func collapseWhitespace(s string) string {
	parts := strings.Fields(s)
	return strings.Join(parts, " ")
}
