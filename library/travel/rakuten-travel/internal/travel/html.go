package travel

import (
	"bytes"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

func integer(n int) string { return strconv.Itoa(n) }
func attr(n *html.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func hasClass(n *html.Node, class string) bool {
	for _, x := range strings.Fields(attr(n, "class")) {
		if x == class {
			return true
		}
	}
	return false
}
func descendants(n *html.Node, predicate func(*html.Node) bool) []*html.Node {
	if n == nil {
		return []*html.Node{}
	}
	out := []*html.Node{}
	var visit func(*html.Node)
	visit = func(x *html.Node) {
		if predicate(x) {
			out = append(out, x)
		}
		for child := x.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(n)
	return out
}
func first(n *html.Node, predicate func(*html.Node) bool) *html.Node {
	if n == nil {
		return nil
	}
	if predicate(n) {
		return n
	}
	for x := n.FirstChild; x != nil; x = x.NextSibling {
		if found := first(x, predicate); found != nil {
			return found
		}
	}
	return nil
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
		for ch := x.FirstChild; ch != nil; ch = ch.NextSibling {
			visit(ch)
		}
	}
	visit(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
func rawText(n *html.Node) string {
	var b strings.Builder
	for x := n.FirstChild; x != nil; x = x.NextSibling {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
	}
	return b.String()
}
func boundedText(s string, limit int) string {
	r := []rune(s)
	if len(r) > limit {
		return string(r[:limit]) + "…"
	}
	return s
}
func pointer(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	s = boundedText(strings.TrimSpace(s), 16000)
	return &s
}
func nodeByClass(n *html.Node, class string) *html.Node {
	return first(n, func(x *html.Node) bool { return hasClass(x, class) })
}
func byLocate(n *html.Node, loc string) *html.Node {
	return first(n, func(x *html.Node) bool { return attr(x, "data-locate") == loc })
}
func parseDOM(d document) (*html.Node, error) {
	root, err := html.Parse(bytes.NewReader(d.Body))
	if err != nil {
		return nil, parseFailure(d.Source.URL, "invalid document")
	}
	return root, nil
}
func checkDocument(body []byte, raw string) error {
	if len(bytes.TrimSpace(body)) == 0 {
		return parseFailure(raw, "empty document")
	}
	lower := strings.ToLower(string(body))
	title := ""
	if m := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`).FindStringSubmatch(lower); len(m) > 1 {
		title = m[1]
	}
	for _, marker := range []string{"just a moment", "access denied", "robot check", "captcha", "security check"} {
		if strings.Contains(title, marker) {
			return sourceError("challenge", raw, 200, "upstream returned a challenge page", nil)
		}
	}
	if strings.Contains(lower, "cf-chl-") || strings.Contains(lower, "verify you are human") || strings.Contains(lower, "人間であることを確認") {
		return sourceError("challenge", raw, 200, "upstream returned a challenge page", nil)
	}
	if !strings.Contains(lower, "<") {
		return parseFailure(raw, "response is not HTML")
	}
	return nil
}
func resolveSource(raw, href string) string {
	base, e := url.Parse(raw)
	if e != nil {
		return ""
	}
	u, e := url.Parse(href)
	if e != nil {
		return ""
	}
	u = base.ResolveReference(u)
	u.Fragment = ""
	if !validSourceURL(u.String()) {
		return ""
	}
	return u.String()
}

var propertyPath = regexp.MustCompile(`^/HOTEL/([0-9]{1,12})/([0-9]{1,12})\.html$`)

func propertyID(raw string) string {
	u, e := url.Parse(raw)
	if e != nil {
		return ""
	}
	m := propertyPath.FindStringSubmatch(u.Path)
	if len(m) == 3 && m[1] == m[2] {
		return m[1]
	}
	return ""
}
func sourceNext(root *html.Node, d document, key string, page int) bool {
	for _, a := range descendants(root, func(n *html.Node) bool { return n.Data == "a" }) {
		resolved := resolveSource(d.Source.URL, attr(a, "href"))
		u, e := url.Parse(resolved)
		if e != nil || resolved == "" {
			continue
		}
		base, _ := url.Parse(d.Source.URL)
		if u.Host != base.Host || u.Path != base.Path {
			continue
		}
		v := u.Query()
		p, e := strconv.Atoi(v.Get(key))
		if e != nil || p != page+1 {
			continue
		}
		if key == "f_page_no" && v.Get("f_flg") != "PLAN" {
			continue
		}
		return true
	}
	return false
}
func sourceTotal(root *html.Node) *int {
	if n := nodeByClass(root, "plan-number__total-count"); n != nil {
		if count, e := strconv.Atoi(strings.ReplaceAll(text(n), ",", "")); e == nil && count >= 0 {
			return &count
		}
	}
	m := regexp.MustCompile(`([0-9,]+)\s*件中`).FindStringSubmatch(text(root))
	if len(m) == 2 {
		count, e := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
		if e == nil {
			return &count
		}
	}
	return nil
}
func blankHotel(id string) Hotel {
	return Hotel{ID: id, URL: "https://travel.rakuten.co.jp/HOTEL/" + id + "/" + id + ".html", Access: []string{}, Parking: []string{}, HotelAmenities: []string{}, RoomAmenities: []string{}, Notes: []string{}, CancellationPolicy: []string{}, CoordinatesReason: "Source coordinate datum and units are unverified; coordinates are not inferred."}
}
func rating(root *html.Node) *Rating {
	var score float64
	for _, class := range []string{"rating", "average", "avrgNum", "hotel-list__review-average", "review-average", "cstmrEvl", "ratePoint"} {
		n := nodeByClass(root, class)
		if n != nil {
			m := regexp.MustCompile(`(?:^|\s)([0-5]\.[0-9]{1,2})(?:\s|$)`).FindStringSubmatch(text(n))
			if len(m) == 2 {
				score, _ = strconv.ParseFloat(m[1], 64)
				break
			}
		}
	}
	if score <= 0 || score > 5 {
		return nil
	}
	r := &Rating{Source: "Rakuten Travel", Score: score}
	m := regexp.MustCompile(`([0-9,]+)\s*件`).FindStringSubmatch(text(root))
	if len(m) == 2 {
		count, e := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
		if e == nil {
			r.ReviewCount = &count
		}
	}
	return r
}
