package travel

import (
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

func parseAreas(d document, parent string) (AreaResult, error) {
	root, e := parseDOM(d)
	if e != nil {
		return AreaResult{}, e
	}
	out := AreaResult{Status: StatusOK, Areas: []Area{}, Source: d.Source}
	seen := map[string]bool{}
	path := regexp.MustCompile(`^/yado/([a-z]{2,20})/([A-Za-z0-9_]{1,20})\.html$`)
	for _, a := range descendants(root, func(n *html.Node) bool { return n.Data == "a" }) {
		raw := resolveSource(d.Source.URL, attr(a, "href"))
		u, e := url.Parse(raw)
		if e != nil || raw == "" {
			continue
		}
		m := path.FindStringSubmatch(u.Path)
		if len(m) != 3 {
			continue
		}
		id := m[1]
		if parent == "" {
			if m[2] != "map" {
				continue
			}
		} else {
			if m[1] != parent || m[2] == "map" || m[2] == "map_s" {
				continue
			}
			id = m[1] + "/" + m[2]
		}
		name := text(a)
		if name == "" || seen[id] {
			continue
		}
		seen[id] = true
		area := Area{ID: id, Name: boundedText(name, 200), URL: raw}
		if parent != "" {
			area.Parent = pointer(parent)
		}
		out.Areas = append(out.Areas, area)
		if len(out.Areas) >= 1000 {
			break
		}
	}
	if len(out.Areas) == 0 {
		return out, parseFailure(d.Source.URL, "no source area directory links found")
	}
	_, _, out.Page = window(len(out.Areas), 0, len(out.Areas), false, 1, "area_links", len(out.Areas), len(out.Areas), nil)
	return out, nil
}
func cardAncestor(a *html.Node) *html.Node {
	var fallback *html.Node
	for n := a.Parent; n != nil; n = n.Parent {
		if n.Data != "body" && (hasClass(n, "hotelBox") || hasClass(n, "hotel-list") || hasClass(n, "hotel-list__item")) {
			return n
		}
		if fallback == nil && (n.Data == "dl" || n.Data == "li") {
			fallback = n
		}
		if attr(n, "id") == "result" {
			break
		}
	}
	if fallback != nil {
		return fallback
	}
	return a.Parent
}
func parseHotelSearch(d document, q HotelQuery) (HotelSearchResult, error) {
	root, e := parseDOM(d)
	if e != nil {
		return HotelSearchResult{}, e
	}
	if q.Query != "" {
		echo := first(root, func(n *html.Node) bool { return n.Data == "input" && attr(n, "name") == "f_query" })
		if echo == nil || attr(echo, "value") != q.Query {
			return HotelSearchResult{}, sourceError("query_mismatch", d.Source.URL, 0, "keyword query echo is missing or differs", nil)
		}
	}
	out := HotelSearchResult{Status: StatusOK, Hotels: []Hotel{}, Query: q, Source: d.Source}
	seen := map[string]bool{}
	hotels := []Hotel{}
	for _, a := range descendants(root, func(n *html.Node) bool { return n.Data == "a" }) {
		raw := resolveSource(d.Source.URL, attr(a, "href"))
		id := propertyID(raw)
		if id == "" || seen[id] || text(a) == "" {
			continue
		}
		heading := false
		for n := a.Parent; n != nil && n != root; n = n.Parent {
			if n.Data == "h2" || n.Data == "h3" {
				heading = true
				break
			}
			if n.Data == "div" || n.Data == "li" {
				break
			}
		}
		if !heading {
			continue
		}
		seen[id] = true
		h := blankHotel(id)
		h.Name = pointer(text(a))
		card := cardAncestor(a)
		h.Rating = rating(card)
		if m := regexp.MustCompile(`\[住所\]\s*(.*?)(?:宿泊プラン|$)`).FindStringSubmatch(text(card)); len(m) > 1 {
			h.Address = pointer(boundedText(m[1], 400))
		}
		hotels = append(hotels, h)
		if len(hotels) >= 2000 {
			return out, parseFailure(d.Source.URL, "hotel list exceeds the bounded parser limit")
		}
	}
	if len(hotels) == 0 {
		result := first(root, func(n *html.Node) bool { return attr(n, "id") == "result" })
		if result == nil {
			result = first(root, func(n *html.Node) bool {
				return attr(n, "id") == "notFound" && first(n, func(x *html.Node) bool { return attr(x, "id") == "messageArea" }) != nil
			})
		}
		t := text(result)
		total := sourceTotal(result)
		explicit := strings.Contains(t, "宿泊施設が見つかりません") || strings.Contains(t, "宿泊施設は見つかりません") || strings.Contains(t, "条件に該当する施設はありません") || strings.Contains(t, "該当する施設が見つかりません")
		if result == nil || !(explicit || total != nil && *total == 0) {
			return out, parseFailure(d.Source.URL, "hotel result structure is missing or not explicitly empty")
		}
		out.Status = StatusNoMatches
	}
	next := sourceNext(root, d, "f_next", q.Page)
	if q.Area != "" {
		next = areaNext(root, d, q)
	}
	start, end, page := window(len(hotels), q.Offset, q.Limit, next, q.Page, "hotels", len(hotels), len(hotels), sourceTotal(root))
	out.Hotels = append(out.Hotels, hotels[start:end]...)
	out.Page = page
	return out, nil
}
func detailValues(n *html.Node) []string {
	dd := first(n, func(x *html.Node) bool { return x.Data == "dd" || x.Data == "td" })
	if dd == nil {
		dd = n
	}
	values := []string{}
	lis := descendants(dd, func(x *html.Node) bool { return x.Data == "li" })
	if len(lis) > 0 {
		for _, li := range lis {
			if t := text(li); t != "" {
				values = append(values, boundedText(t, 4000))
				if len(values) >= 100 {
					break
				}
			}
		}
	} else if t := text(dd); t != "" {
		values = append(values, boundedText(t, 16000))
	}
	return values
}
func parseHotel(d document, id string, details bool) (Hotel, error) {
	root, e := parseDOM(d)
	if e != nil {
		return Hotel{}, e
	}
	h := blankHotel(id)
	identity := false
	for _, script := range descendants(root, func(n *html.Node) bool { return n.Data == "script" }) {
		matches := regexp.MustCompile(`(?s)hotelBasicInfo\s*=\s*\{.{0,1000}?hotelNo\s*:\s*"([0-9]{1,12})"`).FindStringSubmatch(rawText(script))
		if len(matches) > 1 {
			if matches[1] != id {
				return h, sourceError("query_mismatch", d.Source.URL, 0, "property hotelNo differs from requested ID", nil)
			}
			identity = true
		}
	}
	for _, link := range descendants(root, func(n *html.Node) bool { return n.Data == "link" && attr(n, "rel") == "canonical" }) {
		u, e := url.Parse(resolveSource(d.Source.URL, attr(link, "href")))
		if e == nil && u.Path != "" {
			expected := "/HOTEL/" + id + "/" + id
			if u.Path != expected+".html" && u.Path != expected+"_std.html" {
				return h, sourceError("query_mismatch", d.Source.URL, 0, "canonical property ID differs", nil)
			}
			identity = true
		}
	}
	hero := first(root, func(n *html.Node) bool { return attr(n, "id") == "RthNameArea" })
	if hero != nil {
		h.Name = pointer(text(hero))
		for _, a := range descendants(hero, func(n *html.Node) bool { return n.Data == "a" }) {
			self := propertyID(resolveSource(d.Source.URL, attr(a, "href")))
			if self != "" {
				if self != id {
					return h, sourceError("query_mismatch", d.Source.URL, 0, "property hero ID differs", nil)
				}
				identity = true
			}
		}
	}
	header := first(root, func(n *html.Node) bool { return attr(n, "id") == "hotel-info" })
	if header != nil {
		h.Rating = rating(header)
	}
	if !identity {
		return h, parseFailure(d.Source.URL, "canonical/self property identity is missing")
	}
	count := 0
	for _, n := range descendants(root, func(n *html.Node) bool { return n.Data == "li" || n.Data == "tr" }) {
		loc := attr(n, "data-locate")
		label := text(first(n, func(x *html.Node) bool { return x.Data == "dt" || x.Data == "th" }))
		values := detailValues(n)
		if len(values) == 0 {
			continue
		}
		switch {
		case loc == "hotel-address" || label == "住所":
			h.Address = pointer(strings.ReplaceAll(values[0], "地図を見る", ""))
			if m := regexp.MustCompile(`〒\s*([0-9]{3}-[0-9]{4})`).FindStringSubmatch(values[0]); len(m) > 1 {
				h.PostalCode = pointer(m[1])
			}
			count++
		case loc == "hotel-tel" || label == "TEL" || label == "電話番号":
			h.Phone = pointer(values[0])
			count++
		case loc == "hotel-access" || label == "交通アクセス":
			h.Access = values
			count++
		case loc == "hotel-park" || label == "駐車場":
			h.Parking = values
			count++
		case loc == "hotel-facilities" || label == "館内設備":
			h.HotelAmenities = values
			count++
		case loc == "hotel-room-facilities" || strings.HasPrefix(label, "部屋設備"):
			h.RoomAmenities = values
			count++
		case loc == "hotel-attention" || label == "条件・注意事項":
			h.Notes = values
			count++
		case loc == "hotel-cancelPolicy" || label == "キャンセルポリシー":
			h.CancellationPolicy = values
			h.PolicyCaveat = pointer(text(byLocate(n, "hotel-planCancelPolicy")))
			count++
		}
	}
	if h.Address == nil {
		if n := byLocate(root, "hotel-address"); n != nil {
			h.Address = pointer(strings.ReplaceAll(text(n), "地図を見る", ""))
		}
	}
	if details {
		if count == 0 {
			return h, parseFailure(d.Source.URL, "facilities/policies structure is missing")
		}
	} else if !identity || h.Name == nil {
		return h, parseFailure(d.Source.URL, "hotel identity/name is missing")
	}
	return h, nil
}
func areaNext(root *html.Node, d document, q HotelQuery) bool {
	want := "/ds/yado/" + q.Area + "-p" + integer(q.Page+1)
	for _, a := range descendants(root, func(n *html.Node) bool { return n.Data == "a" }) {
		u, e := url.Parse(resolveSource(d.Source.URL, attr(a, "href")))
		if e == nil && u.Hostname() == "search.travel.rakuten.co.jp" && u.Path == want {
			return true
		}
	}
	return false
}
