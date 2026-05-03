package vrbo

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/cliutil"
)

var (
	moneyRe    = regexp.MustCompile(`[$£€]\s*([0-9][0-9,]*(?:\.[0-9]{2})?)`)
	propertyRe = regexp.MustCompile(`/(?:h)?([0-9]{5,})\b`)
	bedroomRe  = regexp.MustCompile(`(?i)\b([0-9]+)\s*bedrooms?\b`)
	bedRe      = regexp.MustCompile(`(?i)\b([0-9]+)\s*beds?\b`)
	bathRe     = regexp.MustCompile(`(?i)\b([0-9]+(?:\.[0-9]+)?)\s*baths?\b`)
	sleepRe    = regexp.MustCompile(`(?i)\bsleeps?\s*([0-9]+)\b`)
)

func propertiesFromSearchHTML(data []byte) ([]Property, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if isBotChallenge(doc) {
		return nil, fmt.Errorf("VRBO bot challenge returned instead of search results")
	}
	var out []Property
	doc.Find(`[data-stid="lodging-card-responsive"]`).Each(func(i int, card *goquery.Selection) {
		p := propertyFromCard(card)
		if p.ID != "" || p.Title != "" {
			out = append(out, p)
		}
	})
	if len(out) == 0 {
		return nil, fmt.Errorf("VRBO lodging cards not found")
	}
	return dedupe(out), nil
}

func propertyFromCard(card *goquery.Selection) Property {
	text := clean(card.Text())
	title := clean(card.Find("h3").First().Text())
	title = strings.TrimPrefix(title, "Photo gallery for ")
	href, _ := card.Find("a[href]").First().Attr("href")
	abs := absoluteVRBOURL(href)
	id := normalizePropertyID(idFromVRBOURL(abs))
	p := Property{
		ID:                     id,
		Name:                   title,
		Title:                  title,
		URL:                    abs,
		PropertyManagementName: extractBrand(title),
		Raw:                    map[string]any{"text": text, "sponsored": card.Find(`[data-stid="sponsored-ad-badge"]`).Length() > 0},
	}
	p.Beds = firstInt(bedroomRe, text)
	if p.Beds == 0 {
		p.Beds = firstInt(bedRe, text)
	}
	p.Baths = firstFloat(bathRe, text)
	p.SleepsMax = firstInt(sleepRe, text)
	p.PriceBreakdown = priceBreakdownFromText(text)
	return p
}

func propertyFromDetailHTML(data []byte, id, rawURL string) (*Property, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if isBotChallenge(doc) {
		return nil, fmt.Errorf("VRBO bot challenge returned instead of property detail")
	}
	text := clean(doc.Text())
	title := clean(doc.Find("h1").First().Text())
	if title == "" {
		title = clean(doc.Find(`meta[property="og:title"]`).AttrOr("content", ""))
	}
	p := &Property{
		ID:                     normalizePropertyID(id),
		Name:                   title,
		Title:                  title,
		URL:                    rawURL,
		PropertyManagementName: extractBrand(title),
		Description:            clean(doc.Find(`meta[name="description"]`).AttrOr("content", "")),
		Raw:                    map[string]any{"text": text},
	}
	p.Beds = firstInt(bedroomRe, text)
	if p.Beds == 0 {
		p.Beds = firstInt(bedRe, text)
	}
	p.Baths = firstFloat(bathRe, text)
	p.SleepsMax = firstInt(sleepRe, text)
	p.PriceBreakdown = priceBreakdownFromText(text)
	if p.Title == "" && p.PriceBreakdown == nil {
		return nil, fmt.Errorf("property %s not found in detail HTML", id)
	}
	return p, nil
}

func priceBreakdownFromText(text string) *PriceBreakdown {
	amounts := moneyAmounts(text)
	if len(amounts) == 0 {
		return nil
	}
	sort.Float64s(amounts)
	pb := &PriceBreakdown{Currency: "USD", Fees: map[string]float64{}, Raw: text}
	pb.PerNight = amounts[0]
	pb.Total = amounts[len(amounts)-1]
	lower := strings.ToLower(text)
	for _, label := range []string{"cleaning", "service", "tax"} {
		if idx := strings.Index(lower, label); idx >= 0 {
			start, end := idx-120, idx+120
			if start < 0 {
				start = 0
			}
			if end > len(text) {
				end = len(text)
			}
			if amount := amountFromText(text[start:end]); amount > 0 {
				pb.Fees[label] = amount
			}
		}
	}
	return pb
}

func moneyAmounts(s string) []float64 {
	matches := moneyRe.FindAllStringSubmatch(s, -1)
	seen := map[float64]bool{}
	out := make([]float64, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		f, _ := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", ""), 64)
		if f > 0 && !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}

func extractBrand(title string) string {
	parts := strings.Split(title, "|")
	if len(parts) < 2 {
		return ""
	}
	last := strings.TrimSpace(parts[len(parts)-1])
	if i := strings.Index(strings.ToLower(last), " by "); i >= 0 {
		return strings.TrimSpace(last[i+4:])
	}
	return last
}

func absoluteVRBOURL(href string) string {
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if strings.HasPrefix(href, "/") {
		return baseURL + href
	}
	return baseURL + "/" + href
}

func idFromVRBOURL(raw string) string {
	if m := propertyRe.FindStringSubmatch(raw); len(m) > 1 {
		return m[1]
	}
	return ""
}

func normalizePropertyID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.TrimPrefix(id, "h")
	if fromURL := idFromVRBOURL(id); fromURL != "" {
		return fromURL
	}
	return id
}

func firstInt(re *regexp.Regexp, s string) int {
	if m := re.FindStringSubmatch(s); len(m) > 1 {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

func firstFloat(re *regexp.Regexp, s string) float64 {
	if m := re.FindStringSubmatch(s); len(m) > 1 {
		f, _ := strconv.ParseFloat(m[1], 64)
		return f
	}
	return 0
}

func isBotChallenge(doc *goquery.Document) bool {
	title := strings.ToLower(strings.TrimSpace(doc.Find("title").First().Text()))
	body := strings.ToLower(doc.Text())
	return strings.Contains(title, "bot or not") || strings.Contains(body, "captcha-pwa")
}

func propertiesFromRoot(root any) []Property {
	objects := findObjects(root, []string{"propertySearchListings", "propertyDetail", "headline", "listingKey"})
	var out []Property
	for _, obj := range objects {
		for _, item := range flattenPropertyCandidates(obj) {
			p := propertyFromMap(item)
			if p.ID != "" || p.Title != "" {
				out = append(out, p)
			}
		}
	}
	if len(out) == 0 {
		for _, obj := range findObjects(root, []string{"id", "name"}) {
			p := propertyFromMap(obj)
			if p.ID != "" && (p.Title != "" || p.Name != "") {
				out = append(out, p)
			}
		}
	}
	return dedupe(out)
}

func flattenPropertyCandidates(m map[string]any) []map[string]any {
	for _, k := range []string{"propertySearchListings", "listings", "properties"} {
		if arr, ok := m[k].([]any); ok {
			var out []map[string]any
			for _, item := range arr {
				out = append(out, asMap(item))
			}
			return out
		}
	}
	if pd := asMap(m["propertyDetail"]); len(pd) > 0 {
		return []map[string]any{pd}
	}
	return []map[string]any{m}
}

func propertyFromMap(m map[string]any) Property {
	p := Property{
		ID:             firstStringByKeys(m, "id", "propertyId", "listingId"),
		Name:           clean(firstStringByKeys(m, "name", "headline")),
		Title:          clean(firstStringByKeys(m, "headline", "title", "name")),
		AvgRatingValue: num(firstByKey(m, "avgRatingValue")),
		ReviewCount:    int(num(firstByKey(m, "reviewCount"))),
		City:           clean(firstStringByKeys(m, "city", "localizedCity", "location")),
		Raw:            m,
	}
	if p.ID != "" {
		p.URL = "https://www.vrbo.com/" + p.ID
	}
	p.Coordinate = coordFromAny(firstByKey(m, "geo"))
	if p.Coordinate == nil {
		p.Coordinate = coordFromAny(firstByKey(m, "location"))
	}
	p.PropertyManagementName = firstStringByKeys(m, "propertyManagement", "brandName")
	if pm := asMap(firstByKey(m, "propertyManagement")); len(pm) > 0 {
		p.PropertyManagementName = firstStringByKeys(pm, "name", "brandName")
	}
	if host := asMap(firstByKey(m, "host")); len(host) > 0 {
		p.HostName = firstStringByKeys(host, "displayName", "name")
		p.HostBio = firstStringByKeys(host, "aboutMe", "about", "description")
	}
	p.Description = firstStringByKeys(m, "description", "summary")
	p.Photos = collectURLs(m)
	p.Amenities = collectTexts(firstByKey(m, "amenities"))
	p.PriceBreakdown = priceBreakdownFromAny(m)
	p.Beds = int(num(firstByKey(m, "bedrooms")))
	if p.Beds == 0 {
		p.Beds = int(num(firstByKey(m, "beds")))
	}
	p.Baths = num(firstByKey(m, "bathrooms"))
	p.SleepsMax = int(num(firstByKey(m, "sleeps")))
	if p.SleepsMax == 0 {
		p.SleepsMax = int(num(firstByKey(m, "sleepsMax")))
	}
	return p
}

func priceBreakdownFromAny(root any) *PriceBreakdown {
	p := &PriceBreakdown{Currency: "USD", Fees: map[string]float64{}, Raw: root}
	for _, obj := range findObjects(root, []string{"feeType", "label", "amount"}) {
		label := strings.ToLower(firstStringByKeys(obj, "feeType", "label", "title"))
		amount := num(firstByKey(obj, "amount"))
		if amount == 0 {
			amount = amountFromText(firstStringByKeys(obj, "price", "formattedAmount", "value"))
		}
		switch {
		case strings.Contains(label, "clean"):
			p.Fees["cleaning"] += amount
		case strings.Contains(label, "service"):
			p.Fees["service"] += amount
		case strings.Contains(label, "tax"):
			p.Fees["tax"] += amount
		case strings.Contains(label, "subtotal"):
			p.Subtotal = amount
		case strings.Contains(label, "total"):
			p.Total = amount
		}
	}
	if p.Total == 0 {
		p.Total = amountFromText(firstStringByKeys(root, "totalPrice", "total"))
	}
	if p.PerNight == 0 {
		p.PerNight = amountFromText(firstStringByKeys(root, "perNightPrice", "averagePrice"))
	}
	return p
}

func findObjects(root any, keys []string) []map[string]any {
	var out []map[string]any
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for _, k := range keys {
				if _, ok := x[k]; ok {
					out = append(out, x)
					break
				}
			}
			for _, child := range x {
				walk(child)
			}
		case []any:
			for _, child := range x {
				walk(child)
			}
		}
	}
	walk(root)
	return out
}

func coordFromAny(v any) *Coord {
	m := asMap(v)
	if len(m) == 0 {
		return nil
	}
	lat := num(firstByKey(m, "latitude"))
	lng := num(firstByKey(m, "longitude"))
	if lat == 0 {
		lat = num(firstByKey(m, "lat"))
	}
	if lng == 0 {
		lng = num(firstByKey(m, "lng"))
	}
	if lat == 0 && lng == 0 {
		return nil
	}
	return &Coord{Latitude: lat, Longitude: lng}
}

func firstByKey(v any, key string) any {
	switch x := v.(type) {
	case map[string]any:
		if val, ok := x[key]; ok {
			return val
		}
		for _, val := range x {
			if found := firstByKey(val, key); found != nil {
				return found
			}
		}
	case []any:
		for _, val := range x {
			if found := firstByKey(val, key); found != nil {
				return found
			}
		}
	}
	return nil
}

func firstStringByKeys(v any, keys ...string) string {
	for _, key := range keys {
		if s := clean(str(firstByKey(v, key))); s != "" {
			return s
		}
	}
	return ""
}

func collectTexts(v any) []string {
	seen := map[string]bool{}
	var out []string
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			for _, key := range []string{"title", "subtitle", "text", "description", "name"} {
				if s := clean(str(t[key])); s != "" && len(s) < 200 && !seen[s] {
					seen[s] = true
					out = append(out, s)
				}
			}
			for _, v := range t {
				walk(v)
			}
		case []any:
			for _, v := range t {
				walk(v)
			}
		}
	}
	walk(v)
	return out
}

func collectURLs(v any) []string {
	seen := map[string]bool{}
	var out []string
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			for _, key := range []string{"url", "uri", "href"} {
				if s := str(t[key]); strings.HasPrefix(s, "http") && !seen[s] {
					seen[s] = true
					out = append(out, s)
				}
			}
			for _, v := range t {
				walk(v)
			}
		case []any:
			for _, v := range t {
				walk(v)
			}
		}
	}
	walk(v)
	return out
}

func dedupe(in []Property) []Property {
	seen := map[string]bool{}
	var out []Property
	for _, p := range in {
		key := p.ID
		if key == "" {
			key = p.Title
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
	}
	return out
}

func amountFromText(s string) float64 {
	m := moneyRe.FindStringSubmatch(s)
	if len(m) < 2 {
		return 0
	}
	f, _ := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", ""), 64)
	return f
}

func defaultInt(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func truncate(s string) string {
	if len(s) > 300 {
		return s[:300]
	}
	return s
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func str(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", x)
	}
}

func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(strings.Trim(x, "$,")), 64)
		return f
	default:
		return 0
	}
}

func clean(s string) string { return cliutil.CleanText(s) }
