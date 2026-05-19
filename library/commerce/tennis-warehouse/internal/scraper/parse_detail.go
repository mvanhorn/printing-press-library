package scraper

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/mvanhorn/printing-press-library/library/commerce/tennis-warehouse/internal/cliutil"
)

// specs come as <td class="SpecsLt|SpecsDk">Label: value</td> — label and
// value live in the SAME td separated by ": ". Some cells (Grip Size) carry
// a link or have no value; we keep the label with an empty value.
func specMap(doc *goquery.Document) map[string]string {
	out := make(map[string]string)
	doc.Find("td.SpecsLt, td.SpecsDk").Each(func(i int, td *goquery.Selection) {
		t := cliutil.CleanText(strings.TrimSpace(td.Text()))
		if t == "" {
			return
		}
		idx := strings.Index(t, ":")
		if idx <= 0 {
			return
		}
		label := strings.TrimSpace(t[:idx])
		value := strings.TrimSpace(t[idx+1:])
		if label != "" {
			out[strings.ToLower(label)] = value
		}
	})
	return out
}

var reFloatPart = regexp.MustCompile(`-?\d+(\.\d+)?`)

func parseFloatHead(s string) float64 {
	m := reFloatPart.FindString(s)
	if m == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(m, 64)
	return f
}

func parseIntHead(s string) int {
	m := reFloatPart.FindString(s)
	if m == "" {
		return 0
	}
	v, _ := strconv.Atoi(strings.TrimSuffix(m, "."))
	if v == 0 {
		f, _ := strconv.ParseFloat(m, 64)
		return int(f)
	}
	return v
}

// applySpecsTo copies values from a spec map onto common spec fields.
// out must be addressable (pointer to a struct embedding the same fields).
func applySpecsTo(m map[string]string, set func(field, value string)) {
	for k, v := range m {
		set(k, v)
	}
}

// ParseRacquetDetail parses a single new-racquet detail page (descpageRC...html).
// SKU and URL must be supplied by the caller because they're not always present
// as data attributes on the detail page itself.
func ParseRacquetDetail(html, sku, url, brand string) (*Racquet, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	r := &Racquet{
		SKU:        sku,
		URL:        url,
		Brand:      brand,
		LastSeenAt: time.Now().UTC(),
	}
	// Title: the H1 usually carries the model name, or the document title's
	// leading segment. Fall back to og:title meta.
	if h1 := doc.Find("h1").First(); h1.Length() > 0 {
		r.Model = cliutil.CleanText(strings.TrimSpace(h1.Text()))
	}
	if r.Model == "" {
		if og, exists := doc.Find(`meta[property="og:title"]`).Attr("content"); exists {
			r.Model = cliutil.CleanText(strings.TrimSpace(og))
		}
	}
	// Brand: try to derive from the H1 or product description if not given.
	if r.Brand == "" {
		r.Brand = brandFromModel(r.Model)
	}
	// Price: first $-prefixed value on the page is generally the asking price.
	doc.Find(`[class*="price"], [class*="Price"]`).EachWithBreak(func(i int, sel *goquery.Selection) bool {
		t := strings.TrimSpace(sel.Text())
		if t == "" {
			return true
		}
		if v := parsePriceFromText(t); v > 0 {
			r.Price = v
			return false
		}
		return true
	})
	if r.Price == 0 {
		// Fallback: scan body text for first $NN.NN pattern.
		r.Price = parsePriceFromText(doc.Find("body").Text())
	}
	// Image: first product image with an SKU-like name.
	doc.Find("img").EachWithBreak(func(i int, img *goquery.Selection) bool {
		src, _ := img.Attr("src")
		if src != "" && strings.Contains(src, sku) {
			r.ImageURL = src
			return false
		}
		return true
	})
	// Specs.
	specs := specMap(doc)
	applyCommonSpecs(specs, &r.HeadSizeIn2, &r.StrungWeight, &r.UnstrungOz,
		&r.Balance, &r.Swingweight, &r.Stiffness, &r.BeamWidth,
		&r.StringPattern, &r.LengthIn, &r.Composition,
		&r.PowerLevel, &r.StrokeStyle)
	return r, nil
}

// ParseUsedDetail parses a used-model detail page (/orderusedproduct.html?pcode=...).
// Returns the model record plus the list of individual physical units (Grade A/B/C/Unused).
func ParseUsedDetail(html, pcode, url, brand string) (*UsedModel, []UsedUnit, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, nil, err
	}
	now := time.Now().UTC()
	m := &UsedModel{
		PCode:       pcode,
		URL:         url,
		Brand:       brand,
		FirstSeenAt: now,
		LastSeenAt:  now,
	}
	if h1 := doc.Find("h1").First(); h1.Length() > 0 {
		m.Model = cliutil.CleanText(strings.TrimSpace(h1.Text()))
	}
	if m.Brand == "" {
		m.Brand = brandFromModel(m.Model)
	}
	// Image.
	doc.Find("img").EachWithBreak(func(i int, img *goquery.Selection) bool {
		src, _ := img.Attr("src")
		if src != "" && strings.Contains(src, pcode) {
			m.ImageURL = src
			return false
		}
		return true
	})
	// Specs.
	specs := specMap(doc)
	applyCommonSpecs(specs, &m.HeadSizeIn2, &m.StrungWeight, &m.UnstrungOz,
		&m.Balance, &m.Swingweight, &m.Stiffness, &m.BeamWidth,
		&m.StringPattern, &m.LengthIn, &m.Composition,
		&m.PowerLevel, &m.StrokeStyle)
	// Individual unit rows. Unit rows live in tr.subproduct[data-code];
	// grade and grip size live in the row text, not as attributes.
	var units []UsedUnit
	doc.Find(`tr.subproduct[data-code]`).Each(func(i int, tr *goquery.Selection) {
		code, _ := tr.Attr("data-code")
		if code == "" {
			return
		}
		// Skip rows that are themselves the parent model record (data-code matches pcode).
		if code == pcode {
			return
		}
		raw := strings.Join(strings.Fields(tr.Text()), " ")
		u := UsedUnit{
			StockCode:   code,
			PCode:       pcode,
			FirstSeenAt: now,
			LastSeenAt:  now,
		}
		if gm := reGrade.FindStringSubmatch(raw); len(gm) > 1 {
			u.Grade = strings.TrimSpace(gm[1])
		}
		if u.Grade == "" {
			return
		}
		u.Price = parsePriceFromText(raw)
		if u.Price <= 0 {
			return
		}
		if g := reGrip.FindString(raw); g != "" {
			u.GripSize = strings.TrimSpace(strings.TrimSuffix(g, `"`))
		}
		if strings.Contains(raw, "Sold Out") {
			u.Notes = "sold_out"
		}
		units = append(units, u)
	})
	m.UnitCount = len(units)
	// Aggregate price low/high.
	if len(units) > 0 {
		m.PriceLow = units[0].Price
		m.PriceHigh = units[0].Price
		for _, u := range units {
			if u.Price < m.PriceLow {
				m.PriceLow = u.Price
			}
			if u.Price > m.PriceHigh {
				m.PriceHigh = u.Price
			}
		}
	}
	return m, units, nil
}

// Grip size in HTML appears as `4 3/8"` or `4_3/8` depending on rendering.
var reGrip = regexp.MustCompile(`\b\d[_ ]\d/\d"?\b`)

// Grade label inside row text: `Grade: Grade A` or `Grade : Grade A`.
var reGrade = regexp.MustCompile(`Grade\s*:\s*(Grade [ABC]|Unused)`)

func applyCommonSpecs(m map[string]string, headSize, strungWt, unstrungWt *float64,
	balance *string, swingweight, stiffness *int, beamWidth *string,
	stringPattern *string, lengthIn *float64, composition *string,
	powerLevel, strokeStyle *string,
) {
	if v, ok := m["head size"]; ok {
		*headSize = parseFloatHead(v)
	}
	if v, ok := m["strung weight"]; ok {
		*strungWt = parseFloatHead(v)
	}
	if v, ok := m["unstrung weight"]; ok {
		*unstrungWt = parseFloatHead(v)
	}
	if v, ok := m["balance"]; ok {
		*balance = v
	}
	if v, ok := m["swingweight"]; ok {
		*swingweight = parseIntHead(v)
	}
	if v, ok := m["stiffness"]; ok {
		*stiffness = parseIntHead(v)
	}
	if v, ok := m["beam width"]; ok {
		*beamWidth = v
	}
	if v, ok := m["string pattern"]; ok {
		*stringPattern = normalizePattern(v)
	}
	if v, ok := m["length"]; ok {
		*lengthIn = parseFloatHead(v)
	}
	if v, ok := m["composition"]; ok {
		*composition = v
	}
	if v, ok := m["power level"]; ok {
		*powerLevel = v
	}
	if v, ok := m["stroke style"]; ok {
		*strokeStyle = v
	}
}

func brandFromModel(model string) string {
	low := strings.ToLower(model)
	for _, b := range []string{"wilson", "babolat", "head", "yonex", "tecnifibre", "dunlop", "prince", "volkl", "prokennex", "solinco", "mizuno", "lacoste"} {
		if strings.Contains(low, b) {
			return strings.Title(b)
		}
	}
	return ""
}

var rePrice = regexp.MustCompile(`\$(\d{1,4}(?:,\d{3})*(?:\.\d{1,2})?)`)

// normalizePattern reduces verbose string pattern descriptions like
// "16 Mains / 19 Crosses ..." or "16x19 (no shared holes)" to the canonical
// "16x19" form. Returns the original string when it can't find both numbers.
var rePatternPair = regexp.MustCompile(`(\d{1,2})\s*(?:x|×|Mains?\s*/\s*)\s*(\d{1,2})`)

func normalizePattern(raw string) string {
	if raw == "" {
		return ""
	}
	if m := rePatternPair.FindStringSubmatch(raw); len(m) == 3 {
		return m[1] + "x" + m[2]
	}
	return strings.Fields(raw)[0]
}

func parsePriceFromText(s string) float64 {
	m := rePrice.FindStringSubmatch(s)
	if len(m) < 2 {
		return 0
	}
	clean := strings.ReplaceAll(m[1], ",", "")
	v, _ := strconv.ParseFloat(clean, 64)
	return v
}
