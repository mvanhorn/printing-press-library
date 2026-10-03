// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package hgj

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
)

var resultCountRE = regexp.MustCompile(`^([0-9][0-9,]*) results? found$`)

func attr(n *xhtml.Node, k string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, k) {
			return a.Val
		}
	}
	return ""
}
func hasClass(n *xhtml.Node, c string) bool {
	for _, v := range strings.Fields(attr(n, "class")) {
		if v == c {
			return true
		}
	}
	return false
}
func nodeText(n *xhtml.Node) string {
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n == nil {
			return
		}
		if n.Type == xhtml.ElementNode && (n.Data == "script" || n.Data == "style" || n.Data == "svg" || n.Data == "iframe") {
			return
		}
		if n.Type == xhtml.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
func findAll(n *xhtml.Node, pred func(*xhtml.Node) bool) []*xhtml.Node {
	out := []*xhtml.Node{}
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n == nil {
			return
		}
		if pred(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}
func first(n *xhtml.Node, pred func(*xhtml.Node) bool) *xhtml.Node {
	a := findAll(n, pred)
	if len(a) == 0 {
		return nil
	}
	return a[0]
}
func tag(t string) func(*xhtml.Node) bool {
	return func(n *xhtml.Node) bool { return n.Type == xhtml.ElementNode && n.Data == t }
}
func parseDoc(body []byte) (*xhtml.Node, error) { return xhtml.Parse(bytes.NewReader(body)) }
func badge(p *Place, n *xhtml.Node) {
	for _, t := range findAll(n, tag("time")) {
		month := attr(t, "datetime")
		if monthRE.MatchString(month) && strings.Contains(nodeText(t.Parent), "HGJ Verified") {
			p.Verification = Verification{State: Reported, Label: "HGJ Verified", Month: month, SourceURL: p.SourceURL}
			return
		}
	}
}

// ParseSearch uses actual listing cards; recommendation/navigation links never become results.
func ParseSearch(body []byte, kind, requestURL, observed string, limit int) (SearchResult, error) {
	if kind != Restaurant && kind != Prayer {
		return SearchResult{}, fmt.Errorf("kind must be restaurant or prayer")
	}
	out := SearchResult{Results: []Place{}, SourceURL: requestURL, ObservedAt: observed, SourceTotal: -1, Note: "Discovery cards may hide condition icons. Multiple source filters do not prove all requirements; inspect full details and use plan match."}
	doc, err := parseDoc(body)
	if err != nil {
		return out, err
	}
	main := first(doc, tag("main"))
	if main == nil {
		return out, fmt.Errorf("source search format changed: missing main content")
	}
	for _, p := range findAll(doc, tag("p")) {
		if m := resultCountRE.FindStringSubmatch(nodeText(p)); m != nil {
			out.SourceTotal, _ = strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
			break
		}
		if nodeText(p) == "No results found." {
			for _, class := range strings.Fields(attr(p, "class")) {
				// The source's explicit empty state is a deferred result-summary
				// paragraph, not a numeric counter or a generic page sentence.
				if strings.HasPrefix(class, "SearchResults-module__") && strings.HasSuffix(class, "__resultSummary") {
					out.SourceTotal = 0
					break
				}
			}
			if out.SourceTotal == 0 {
				break
			}
		}
	}
	seen := map[string]bool{}
	prefix := "/restaurant/"
	if kind == Prayer {
		prefix = "/pray/"
	}
	pins, names := rscFacts(doc)
	// React SSR streams deferred card fragments into hidden blocks after </main>.
	// The archive-box class is the observed listing-card boundary; carousels have different classes.
	cards := findAll(doc, func(n *xhtml.Node) bool { return n.Data == "a" && hasClass(n, "archive-box") })
	if len(cards) == 0 {
		cards = findAll(main, tag("a"))
	}
	for _, a := range cards {
		href := attr(a, "href")
		u, e := url.Parse(href)
		if e != nil || u.IsAbs() || u.RawQuery != "" || !strings.HasPrefix(u.Path, prefix) {
			continue
		}
		id := strings.TrimPrefix(u.Path, prefix)
		if !numericID.MatchString(id) || seen[id] {
			continue
		}
		heading := first(a, tag("h3"))
		if heading == nil {
			continue
		}
		p := emptyPlace(kind, id, "card", observed)
		p.Name = nodeText(heading)
		if p.Name == "" {
			return out, fmt.Errorf("source listing has empty name for %s", id)
		}
		p.HiddenConditions = strings.Contains(" "+nodeText(a)+" ", " + ")
		for _, i := range findAll(a, tag("img")) {
			setReported(&p, attr(i, "alt"))
		}
		badge(&p, a)
		p.Coordinates = pins[kind+":"+id]
		p.NameJapanese = names[kind+":"+id]
		if kind == Prayer {
			if strings.Contains(nodeText(a), "Prayer Space") {
				p.PrayerType = "space"
			} else if strings.Contains(nodeText(a), "Mosque") {
				p.PrayerType = "mosque"
			}
		}
		seen[id] = true
		out.Results = append(out.Results, p)
	}
	out.ParsedCount = len(out.Results)
	if out.SourceTotal < 0 {
		return out, fmt.Errorf("source search format changed: no explicit result count")
	}
	if out.SourceTotal == 0 && out.ParsedCount > 0 {
		return out, fmt.Errorf("source search format changed: zero reported results but %d listing cards parsed", out.ParsedCount)
	}
	if out.SourceTotal > 0 && out.ParsedCount == 0 {
		return out, fmt.Errorf("source reported %d results but no listing cards parsed", out.SourceTotal)
	}
	if limit <= 0 {
		return out, fmt.Errorf("limit must be positive")
	}
	if len(out.Results) > limit {
		out.Results = out.Results[:limit]
	}
	out.ReturnedCount = len(out.Results)
	out.Truncated = out.ReturnedCount < out.SourceTotal
	return out, nil
}

type ldPlace struct {
	ID      string `json:"@id"`
	URL     string `json:"url"`
	Name    string `json:"name"`
	Type    any    `json:"@type"`
	Price   string `json:"priceRange"`
	Address struct {
		Street string `json:"streetAddress"`
		Region string `json:"addressRegion"`
		Postal string `json:"postalCode"`
	} `json:"address"`
	Geo       *CoordinatesLD `json:"geo"`
	Amenities []struct {
		Name  string `json:"name"`
		Value any    `json:"value"`
	} `json:"amenityFeature"`
}
type CoordinatesLD struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

// ParseDetail requires the canonical entity's JSON-LD, so recommendations cannot contaminate evidence.
func ParseDetail(body []byte, kind, id, observed string) (Place, error) {
	p := emptyPlace(kind, id, "detail", observed)
	if _, err := CanonicalURL(kind, id); err != nil {
		return p, err
	}
	doc, err := parseDoc(body)
	if err != nil {
		return p, err
	}
	var ld *ldPlace
	for _, s := range findAll(doc, func(n *xhtml.Node) bool { return n.Data == "script" && attr(n, "type") == "application/ld+json" }) {
		if s.FirstChild == nil {
			continue
		}
		var value any
		if json.Unmarshal([]byte(s.FirstChild.Data), &value) != nil {
			continue
		}
		var walk func(any)
		walk = func(v any) {
			switch x := v.(type) {
			case map[string]any:
				if x["@id"] == p.SourceURL || x["url"] == p.SourceURL {
					raw, _ := json.Marshal(x)
					var d ldPlace
					if json.Unmarshal(raw, &d) == nil && d.Name != "" {
						ld = &d
					}
				}
				if g, ok := x["@graph"]; ok {
					walk(g)
				}
			case []any:
				for _, e := range x {
					walk(e)
				}
			}
		}
		walk(value)
	}
	if ld == nil {
		return p, fmt.Errorf("source detail format changed: canonical JSON-LD missing for %s", p.SourceURL)
	}
	main := first(doc, tag("main"))
	if main == nil {
		return p, fmt.Errorf("source detail format changed: missing main")
	}
	h1 := first(main, tag("h1"))
	if h1 == nil || nodeText(h1) != ld.Name {
		return p, fmt.Errorf("source detail identity mismatch for %s", p.SourceURL)
	}
	p.Name = ld.Name
	p.Prefecture = ld.Address.Region
	p.Address = strings.TrimSpace(ld.Address.Street + " " + ld.Address.Region + " " + ld.Address.Postal)
	if ld.Geo != nil && ld.Geo.Latitude != nil && ld.Geo.Longitude != nil {
		g := &Coordinates{*ld.Geo.Latitude, *ld.Geo.Longitude}
		if validCoordinates(g) {
			p.Coordinates = g
		}
	}
	for _, a := range ld.Amenities {
		if v, ok := a.Value.(bool); ok {
			if v {
				setReported(&p, a.Name)
			} else if key, e := NormalizeCondition(a.Name); e == nil {
				c := p.Conditions[key]
				if c.State != Inapplicable {
					c.State = ReportedNegative
					c.Provenance = "HGJ JSON-LD explicit false value"
					p.Conditions[key] = c
				}
			}
		}
	}
	info := h1.Parent
	for info != nil && !hasClass(info, "info-text") {
		info = info.Parent
	}
	if info == nil {
		info = h1.Parent
	}
	badge(&p, info)
	if c := first(info, func(n *xhtml.Node) bool { return hasClass(n, "category") }); c != nil {
		p.Category = nodeText(c)
		if kind == Prayer {
			if strings.Contains(p.Category, "Prayer Space") {
				p.PrayerType = "space"
			} else if strings.Contains(p.Category, "Mosque") {
				p.PrayerType = "mosque"
			}
		}
	}
	for _, h := range findAll(info, tag("h2")) {
		if nodeText(h) != "Weekly opening hours" {
			continue
		}
		next := h.NextSibling
		for next != nil && next.Type != xhtml.ElementNode {
			next = next.NextSibling
		}
		if next != nil && next.Data == "dl" {
			for _, dt := range findAll(next, tag("dt")) {
				dd := dt.NextSibling
				for dd != nil && dd.Data != "dd" {
					dd = dd.NextSibling
				}
				if dd != nil {
					p.WeeklyHours = append(p.WeeklyHours, Hours{nodeText(dt), nodeText(dd)})
				}
			}
		}
	}
	if len(p.WeeklyHours) > 0 {
		p.HoursState = Reported
	}
	if kind == Prayer {
		if n := first(info, func(n *xhtml.Node) bool { return hasClass(n, "whitespace-pre-line") }); n != nil {
			v := nodeText(n)
			if v != "" {
				r := []rune(v)
				if len(r) > 2000 {
					v = string(r[:2000])
					p.AccessNotesTruncated = true
				}
				p.AccessNotes = []string{v}
				p.AccessState = Reported
				if regexp.MustCompile(`(?i)(hours?\s*:|[0-2]?[0-9]:[0-5][0-9]|AM[0-9]|PM[0-9])`).MatchString(v) {
					p.HoursNotes = []string{v}
					p.HoursState = Reported
				}
			}
		}
	}
	_, names := rscFacts(doc)
	p.NameJapanese = names[kind+":"+id]
	p.Certification.LabelState = p.Conditions["certified"].State
	if kind == Restaurant && ld.Price != "" {
		p.PriceRange = ld.Price
		p.PriceScope = "Published directory price range; not a dated quote."
	}
	return p, nil
}

// RSC facts are selectively projected in memory; signed images/session data are never returned or persisted.
func rscFacts(doc *xhtml.Node) (map[string]*Coordinates, map[string]string) {
	coords := map[string]*Coordinates{}
	names := map[string]string{}
	var stream strings.Builder
	for _, s := range findAll(doc, tag("script")) {
		if s.FirstChild == nil {
			continue
		}
		v := s.FirstChild.Data
		if !strings.HasPrefix(v, "self.__next_f.push(") {
			continue
		}
		v = strings.TrimSuffix(strings.TrimPrefix(v, "self.__next_f.push("), ")")
		var a []json.RawMessage
		if json.Unmarshal([]byte(v), &a) != nil || len(a) != 2 {
			continue
		}
		var channel int
		var chunk string
		if json.Unmarshal(a[0], &channel) == nil && channel == 1 && json.Unmarshal(a[1], &chunk) == nil {
			stream.WriteString(chunk)
		}
	}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if alias, ok := x["aliasId"].(float64); ok {
				kind := ""
				if x["category"] == "shop" {
					kind = Restaurant
				} else if x["category"] == "prayer_space" || x["category"] == "prayer" {
					kind = Prayer
				}
				if x["shopNameJapanese"] != nil {
					kind = Restaurant
				}
				id := strconv.FormatInt(int64(alias), 10)
				if kind != "" {
					if c, ok := x["coordinate"].(map[string]any); ok {
						lat, lok := c["lat"].(float64)
						lon, rok := c["lng"].(float64)
						g := &Coordinates{lat, lon}
						if lok && rok && validCoordinates(g) {
							coords[kind+":"+id] = g
						}
					}
					if name, ok := x["shopNameJapanese"].(string); ok {
						names[kind+":"+id] = html.UnescapeString(name)
					}
				}
			}
			for _, e := range x {
				walk(e)
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	for _, line := range strings.Split(stream.String(), "\n") {
		_, data, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		var x any
		if json.Unmarshal([]byte(data), &x) == nil {
			walk(x)
		}
	}
	return coords, names
}
