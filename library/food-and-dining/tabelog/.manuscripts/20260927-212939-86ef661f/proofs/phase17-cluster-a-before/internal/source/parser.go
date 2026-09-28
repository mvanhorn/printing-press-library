package source

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
	"tabelog-pp-cli/internal/domain"
)

var restaurantPath = regexp.MustCompile(`^/en/([a-z-]+)/(A\d{4})/(A\d{6})/(\d{7,10})/$`)
var yenNumbers = regexp.MustCompile(`[0-9][0-9,]*`)
var stationPattern = regexp.MustCompile(`^(.+?)\s+([0-9,]+)m(?:\s|$)`)

func RestaurantURL(raw string) (string, string, domain.Area, error) {
	if e := ValidateSourceURL(raw); e != nil {
		return "", "", domain.Area{}, e
	}
	u, _ := url.Parse(raw)
	u.RawQuery = ""
	u.Fragment = ""
	u.Path = strings.TrimRight(u.Path, "/") + "/"
	m := restaurantPath.FindStringSubmatch(u.Path)
	if len(m) == 0 {
		return "", "", domain.Area{}, fail("usage", "show requires a canonical English restaurant URL, or an already fetched ID")
	}
	return u.String(), m[4], domain.Area{Prefecture: m[1], Area1: m[2], Area2: m[3], Verified: true}, nil
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func class(n *html.Node, want string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == want {
			return true
		}
	}
	return false
}
func walk(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}
func first(n *html.Node, fn func(*html.Node) bool) *html.Node {
	var out *html.Node
	walk(n, func(x *html.Node) {
		if out == nil && fn(x) {
			out = x
		}
	})
	return out
}
func text(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var visit func(*html.Node)
	visit = func(x *html.Node) {
		if x.Type == html.ElementNode && (x.Data == "script" || x.Data == "style") {
			return
		}
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
			b.WriteByte(' ')
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
func rawScript(n *html.Node) string {
	var b strings.Builder
	walk(n, func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
	})
	return b.String()
}
func byClass(n *html.Node, c string) *html.Node {
	return first(n, func(x *html.Node) bool { return class(x, c) })
}
func number(s string) *int {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	n, e := strconv.Atoi(s)
	if e != nil {
		return nil
	}
	return &n
}
func decimal(s string) *float64 {
	v, e := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if e != nil {
		return nil
	}
	return &v
}
func value(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return nil
	}
	return &s
}
func categories(s string) []string {
	if strings.TrimSpace(s) == "-" {
		return make([]string, 0)
	}
	out := make([]string, 0)
	depth, start := 0, 0
	for i, c := range s {
		if c == '(' {
			depth++
		}
		if c == ')' && depth > 0 {
			depth--
		}
		if c == ',' && depth == 0 {
			if v := strings.TrimSpace(s[start:i]); v != "" {
				out = append(out, v)
			}
			start = i + 1
		}
	}
	if v := strings.TrimSpace(s[start:]); v != "" {
		out = append(out, v)
	}
	return out
}

func ParseBudget(s, provenance string) domain.Budget {
	b := domain.Budget{Raw: strings.TrimSpace(s), Source: provenance}
	if b.Raw == "" {
		b.Raw = "-"
	}
	nums := yenNumbers.FindAllString(b.Raw, -1)
	if len(nums) >= 2 {
		b.MinJPY = number(nums[0])
		b.MaxJPY = number(nums[1])
	} else if len(nums) == 1 {
		if strings.HasPrefix(b.Raw, "～") || strings.HasPrefix(b.Raw, "~") || strings.Contains(strings.ToLower(b.Raw), "less") {
			b.MaxJPY = number(nums[0])
		} else if strings.Contains(b.Raw, "-") || strings.HasSuffix(b.Raw, "～") {
			b.MinJPY = number(nums[0])
		} else {
			b.MinJPY = number(nums[0])
			b.MaxJPY = number(nums[0])
		}
	}
	return b
}

func IsChallenge(body []byte) bool {
	doc, e := html.Parse(bytes.NewReader(body))
	if e != nil {
		return false
	}
	// A real listing/detail/zero-result surface takes priority over incidental
	// words in venue names, scripts, or a static-map signature.
	if byClass(doc, "list-rst__rst-name-target") != nil || byClass(doc, "rstinfo-table__address") != nil || byClass(doc, "rstlist-notfound") != nil {
		return false
	}
	title := strings.ToLower(text(first(doc, func(n *html.Node) bool { return n.Data == "title" })))
	for _, s := range []string{"just a moment", "access denied", "bot or not", "pardon our interruption", "captcha", "このページを表示することができません"} {
		if strings.Contains(title, s) {
			return true
		}
	}
	if first(doc, func(n *html.Node) bool {
		return attr(n, "id") == "challenge-form" || strings.Contains(attr(n, "src"), "/cdn-cgi/challenge-platform/")
	}) != nil {
		return true
	}
	return false
}

type Listing struct {
	Items     []domain.Restaurant
	NextURL   string
	Condition string
	Title     string
	Document  *html.Node
}

func ParseListing(body []byte, sourceURL string, t time.Time) (Listing, error) {
	doc, e := html.Parse(bytes.NewReader(body))
	if e != nil {
		return Listing{}, fail("parser_drift", "cannot parse source listing HTML")
	}
	out := Listing{Items: make([]domain.Restaurant, 0), Document: doc, Title: text(first(doc, func(n *html.Node) bool { return n.Data == "title" })), Condition: text(byClass(doc, "list-condition"))}
	seen := map[string]bool{}
	var parseErr error
	walk(doc, func(card *html.Node) {
		if !class(card, "list-rst") || parseErr != nil {
			return
		}
		a := byClass(card, "list-rst__rst-name-target")
		if a == nil {
			parseErr = fail("parser_drift", "restaurant card lost its identity anchor")
			return
		}
		canonical, id, area, e := RestaurantURL(attr(a, "href"))
		if e != nil {
			parseErr = fail("parser_drift", "restaurant card has an invalid English identity URL")
			return
		}
		if seen[id] {
			return
		}
		seen[id] = true
		r := domain.Restaurant{ID: id, Name: text(a), URL: canonical, SourceURL: sourceURL, Area: area, Categories: make([]string, 0), FetchedAt: t, Surface: "listing", Sections: []string{"listing"}, Evidence: map[string]string{}, LunchBudget: ParseBudget("-", "listing"), DinnerBudget: ParseBudget("-", "listing")}
		r.Rating = decimal(text(byClass(card, "list-rst__rating-val")))
		walk(card, func(n *html.Node) {
			if n.Data == "a" && strings.Contains(attr(n, "href"), "/dtlrvwlst/") && r.ReviewCount == nil {
				r.ReviewCount = number(text(n))
			}
		})
		ag := text(byClass(card, "list-rst__area-genre"))
		parts := strings.SplitN(ag, "/", 2)
		if m := stationPattern.FindStringSubmatch(strings.TrimSpace(parts[0])); len(m) > 0 {
			r.NearestStation = m[1]
			r.NearestStationDistanceM = number(m[2])
		}
		if len(parts) == 2 {
			r.Categories = categories(parts[1])
		}
		walk(card, func(n *html.Node) {
			label := attr(n, "aria-label")
			if label == "Average dinner price" || label == "Average lunch price" {
				v := text(byClass(n.Parent, "c-rating-v3__val"))
				b := ParseBudget(v, "listing")
				if label == "Average dinner price" {
					r.DinnerBudget = b
				} else {
					r.LunchBudget = b
				}
			}
			if class(n, "list-rst__holiday-icon") {
				r.Closures = value(text(n.Parent))
			}
			if class(n, "list-rst__award-tooltip") {
				if s := text(n); s != "" {
					r.Awards = append(r.Awards, s)
				}
			}
		})
		// Facilities remain concise source tags, not inferred suitability.
		if f := byClass(card, "list-rst__search-word"); f != nil {
			walk(f, func(n *html.Node) {
				if class(n, "list-rst__search-word-item") {
					if s := text(n); s != "" {
						r.Facilities = append(r.Facilities, s)
					}
				}
			})
		}
		for _, field := range []string{"hours", "payment", "reservation", "address", "transportation"} {
			r.Evidence[field] = "detail_not_fetched"
		}
		setEvidence(&r)
		if r.Name == "" {
			parseErr = fail("parser_drift", "restaurant card has an empty name")
			return
		}
		out.Items = append(out.Items, r)
	})
	if parseErr != nil {
		return Listing{}, parseErr
	}
	walk(doc, func(n *html.Node) {
		if out.NextURL != "" || n.Data != "a" {
			return
		}
		if strings.HasPrefix(text(n), "Next 20") {
			h := attr(n, "href")
			base, _ := url.Parse(sourceURL)
			u, e := base.Parse(h)
			if e != nil || ValidateSourceURL(u.String()) != nil || !strings.Contains(u.Path, "/rstLst/") {
				parseErr = fail("parser_drift", "unsafe or unrecognized next-page link")
				return
			}
			out.NextURL = u.String()
		}
	})
	if parseErr != nil {
		return Listing{}, parseErr
	}
	if len(out.Items) == 0 {
		visible := strings.ToLower(text(doc))
		if out.Condition == "" || (!strings.Contains(visible, "no restaurants") && !strings.Contains(visible, "no results") && !strings.Contains(visible, "0 restaurants") && !strings.Contains(visible, "no matching")) {
			return Listing{}, fail("parser_drift", "source page has no recognizable restaurant cards or verified zero-result state")
		}
	}
	return out, nil
}

func setEvidence(r *domain.Restaurant) {
	r.Evidence["rating"] = "source_unknown"
	if r.Rating != nil {
		r.Evidence["rating"] = "known"
	}
	r.Evidence["review_count"] = "source_unknown"
	if r.ReviewCount != nil {
		r.Evidence["review_count"] = "known"
	}
	for key, b := range map[string]domain.Budget{"lunch_budget": r.LunchBudget, "dinner_budget": r.DinnerBudget} {
		r.Evidence[key] = "source_unknown"
		if b.MinJPY != nil || b.MaxJPY != nil {
			r.Evidence[key] = "known"
		}
	}
}

func ParseDetail(body []byte, sourceURL string, t time.Time) (domain.Restaurant, error) {
	canonical, id, area, e := RestaurantURL(sourceURL)
	if e != nil {
		return domain.Restaurant{}, e
	}
	doc, e := html.Parse(bytes.NewReader(body))
	if e != nil {
		return domain.Restaurant{}, fail("parser_drift", "cannot parse detail HTML")
	}
	r := domain.Restaurant{ID: id, URL: canonical, SourceURL: sourceURL, Area: area, Categories: make([]string, 0), FetchedAt: t, Surface: "detail", Sections: []string{"identity", "budgets", "practical", "facilities"}, Evidence: map[string]string{}, LunchBudget: ParseBudget("-", "listed"), DinnerBudget: ParseBudget("-", "listed")}
	var schema map[string]any
	var visitJSON func(any)
	visitJSON = func(v any) {
		switch x := v.(type) {
		case []any:
			for _, i := range x {
				visitJSON(i)
			}
		case map[string]any:
			typ := fmt.Sprint(x["@type"])
			if strings.Contains(typ, "Restaurant") && schema == nil {
				schema = x
			}
			if g, ok := x["@graph"]; ok {
				visitJSON(g)
			}
		}
	}
	walk(doc, func(n *html.Node) {
		if n.Data == "script" && attr(n, "type") == "application/ld+json" {
			var v any
			if json.Unmarshal([]byte(rawScript(n)), &v) == nil {
				visitJSON(v)
			}
		}
	})
	if schema != nil {
		if s, ok := schema["name"].(string); ok {
			r.Name = s
		}
		if ar, ok := schema["aggregateRating"].(map[string]any); ok {
			r.Rating = decimal(fmt.Sprint(ar["ratingValue"]))
			count := ar["ratingCount"]
			if count == nil {
				count = ar["reviewCount"]
			}
			r.ReviewCount = number(fmt.Sprint(count))
		}
	}
	rows := map[string]*html.Node{}
	walk(doc, func(n *html.Node) {
		if n.Data != "tr" {
			return
		}
		th := first(n, func(c *html.Node) bool { return c.Data == "th" })
		td := first(n, func(c *html.Node) bool { return c.Data == "td" })
		if th != nil && td != nil {
			rows[strings.ToLower(text(th))] = td
		}
	})
	if r.Name == "" {
		r.Name = text(rows["restaurant name"])
	}
	if r.Name == "" || rows["address"] == nil || rows["business hours"] == nil {
		return domain.Restaurant{}, fail("parser_drift", "detail page lost required restaurant identity/practical table structure")
	}
	if n := rows["categories"]; n != nil {
		r.Categories = categories(text(n))
	}
	for label, set := range map[string]func(*string){"reservation availability": func(v *string) { r.Reservation = v }, "address": func(v *string) { r.Address = v }, "transportation": func(v *string) { r.Transportation = v }, "business hours": func(v *string) { r.Hours = v }, "payment methods": func(v *string) { r.Payment = v }, "service charge & fee": func(v *string) { r.ServiceCharge = v }} {
		set(value(text(rows[label])))
	}
	if address := byClass(rows["address"], "rstinfo-table__address"); address != nil {
		r.Address = value(text(address))
	}
	for label, node := range rows {
		if label == "average price" || label == "average price (based on reviews)" {
			walk(node, func(n *html.Node) {
				meal := attr(n, "aria-label")
				if meal != "Dinner" && meal != "Lunch" {
					return
				}
				em := first(n.Parent, func(c *html.Node) bool { return c.Data == "em" })
				provenance := "listed"
				if label != "average price" {
					provenance = "reviews"
				}
				b := ParseBudget(text(em), provenance)
				if provenance == "listed" {
					if meal == "Dinner" {
						r.DinnerBudget = b
					} else {
						r.LunchBudget = b
					}
				} else {
					if meal == "Dinner" {
						r.ReviewDinnerBudget = &b
					} else {
						r.ReviewLunchBudget = &b
					}
				}
			})
		}
	}
	// The concise station fact uses the source header, never first digits in access prose.
	for _, cn := range []string{"rstinfo-table__access", "rstinfo-table__access-info", "rst-header-area-genre"} {
		if n := byClass(doc, cn); n != nil {
			if m := stationPattern.FindStringSubmatch(text(n)); len(m) > 0 {
				r.NearestStation = m[1]
				r.NearestStationDistanceM = number(m[2])
				break
			}
		}
	}
	if n := first(doc, func(n *html.Node) bool {
		return attr(n, "data-rst-name") != "" && attr(n, "data-nearest-station-name") != ""
	}); n != nil {
		r.NearestStation = attr(n, "data-nearest-station-name")
	}
	// Actual English access table separates the canonical station distance into its own paragraph.
	if r.NearestStationDistanceM == nil && rows["transportation"] != nil {
		walk(rows["transportation"], func(n *html.Node) {
			if n.Data != "p" {
				return
			}
			s := text(n)
			re := regexp.MustCompile(`^([0-9,]+) meters from (.+?)\.?$`)
			if m := re.FindStringSubmatch(s); len(m) > 0 {
				r.NearestStation = m[2]
				r.NearestStationDistanceM = number(m[1])
			}
		})
	}
	stationRoute := regexp.MustCompile(`/R[0-9]+/`)
	walk(doc, func(n *html.Node) {
		if n.Data != "a" || !class(n, "linktree__parent-target") || !stationRoute.MatchString(attr(n, "href")) {
			return
		}
		label := text(n)
		short := strings.TrimSpace(strings.TrimSuffix(label, "Sta."))
		if strings.HasSuffix(label, "Sta.") && (r.NearestStation == "" || strings.EqualFold(short, r.NearestStation)) {
			r.NearestStation = label
		}
	})
	for field, v := range map[string]*string{"hours": r.Hours, "payment": r.Payment, "reservation": r.Reservation, "address": r.Address, "transportation": r.Transportation} {
		r.Evidence[field] = "source_unknown"
		if v != nil {
			r.Evidence[field] = "known"
		}
	}
	setEvidence(&r)
	for _, label := range []string{"space/facilities", "non-smoking/smoking", "private rooms", "parking", "food", "drink", "service"} {
		if s := text(rows[label]); s != "" && s != "-" {
			r.Facilities = append(r.Facilities, label+": "+s)
		}
	}
	title := strings.ToLower(text(first(doc, func(n *html.Node) bool { return n.Data == "title" })))
	visible := text(doc)
	lower := strings.ToLower(visible)
	if strings.Contains(title, "[relocated]") || strings.Contains(lower, "this is information from before the relocation") {
		r.Status = value("relocated")
		r.SourceWarnings = append(r.SourceWarnings, "This is information from before the relocation.")
	}
	if strings.Contains(title, "[closed]") || strings.Contains(lower, "this restaurant is now closed") {
		r.Status = value("closed")
		r.SourceWarnings = append(r.SourceWarnings, "The source identifies this restaurant as closed.")
	}
	return r, nil
}
