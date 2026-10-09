// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package locker

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// ---- small DOM helpers ----

func hasClass(n *html.Node, class string) bool {
	if n.Type != html.ElementNode {
		return false
	}
	for _, a := range n.Attr {
		if a.Key == "class" {
			for _, c := range strings.Fields(a.Val) {
				if c == class {
					return true
				}
			}
		}
	}
	return false
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func findAll(n *html.Node, pred func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if pred(x) {
			out = append(out, x)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

func findFirst(n *html.Node, pred func(*html.Node) bool) *html.Node {
	if n == nil {
		return nil
	}
	if pred(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if f := findFirst(c, pred); f != nil {
			return f
		}
	}
	return nil
}

func byClass(class string) func(*html.Node) bool {
	return func(n *html.Node) bool { return hasClass(n, class) }
}

func byTag(tag string) func(*html.Node) bool {
	return func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == tag }
}

// text returns the node text; <br> becomes a newline.
func text(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		switch {
		case x.Type == html.TextNode:
			b.WriteString(x.Data)
		case x.Type == html.ElementNode && x.Data == "br":
			b.WriteString("\n")
		case x.Type == html.ElementNode && (x.Data == "script" || x.Data == "style"):
			return
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func clean(s string) string {
	s = strings.ReplaceAll(s, " ", " ")
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.Join(strings.Fields(l), " ")
		if l != "" {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

var (
	clIDRe   = regexp.MustCompile(`/cl/(\d+)`)
	mapQRe   = regexp.MustCompile(`[?&]q=(-?[0-9.]+),(-?[0-9.]+)`)
	markerRe = regexp.MustCompile(`marker=(-?[0-9.]+)%2C(-?[0-9.]+)`)
	digitsRe = regexp.MustCompile(`[0-9,]+`)
	csrfRe   = regexp.MustCompile(`X-CSRF-Token',\s*"([0-9a-fA-F]+)"`)
)

func parseFloatPtr(s string) *float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}

// parseSizes reads the size rows inside a price-list container.
func parseSizes(n *html.Node) []Size {
	sizes := []Size{}
	for _, name := range findAll(n, byClass("size-name")) {
		ul := name.Parent
		detail := findFirst(ul, byClass("detail"))
		sizes = append(sizes, NewSize(clean(text(name)), clean(text(detail))))
	}
	return sizes
}

// ParseListing parses the "nearest-wrap" blocks used by search, GPS nearest,
// area pages and the detail page neighbour list.
func ParseListing(page []byte, baseURL string) ([]Locker, error) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, err
	}
	out := []Locker{}
	for _, wrap := range findAll(doc, byClass("nearest-wrap")) {
		l, ok := parseWrap(wrap, baseURL)
		if ok {
			out = append(out, l)
		}
	}
	return out, nil
}

func parseWrap(wrap *html.Node, baseURL string) (Locker, bool) {
	var l Locker
	title := findFirst(wrap, byClass("title"))
	a := findFirst(title, byTag("a"))
	if a == nil {
		return l, false
	}
	m := clIDRe.FindStringSubmatch(attr(a, "href"))
	if m == nil {
		return l, false
	}
	l.ID = m[1]
	l.URL = strings.TrimRight(baseURL, "/") + "/cl/" + l.ID
	l.Name = clean(text(a))
	if note := clean(text(findFirst(wrap, byClass("txt")))); note != "" {
		l.Note = strPtr(note)
	}
	if img := findFirst(findFirst(wrap, byClass("nearest-head")), func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "img" && strings.HasSuffix(attr(n, "src"), "_icon.png")
	}); img != nil {
		l.ServiceJA = attr(img, "alt")
	}
	if dw := findFirst(wrap, byClass("distance-wrap")); dw != nil {
		if d := findFirst(dw, byClass("distance")); d != nil {
			if dm := digitsRe.FindString(text(d)); dm != "" {
				l.DistanceM = atoiPtr(dm)
			}
		}
	}
	l.Service, l.WalkUp = ServiceFromJA(l.ServiceJA)
	if btn := findFirst(wrap, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "a" && strings.Contains(attr(n, "href"), "maps.google.com")
	}); btn != nil {
		href := attr(btn, "href")
		l.MapURL = strPtr(href)
		if mm := mapQRe.FindStringSubmatch(href); mm != nil {
			l.Lat, l.Lon = parseFloatPtr(mm[1]), parseFloatPtr(mm[2])
		}
	}
	l.Sizes = []Size{}
	if pl := findFirst(wrap, byClass("price-list")); pl != nil {
		l.Sizes = parseSizes(pl)
	}
	l.SizesKnown = len(l.Sizes) > 0
	fields := map[string]string{}
	if ld := findFirst(wrap, byClass("locker-detail")); ld != nil {
		for _, li := range findAll(ld, byTag("li")) {
			spans := findAll(li, byTag("span"))
			if len(spans) < 2 {
				continue
			}
			k := strings.TrimSuffix(strings.TrimSpace(clean(text(spans[0]))), "：")
			fields[k] = text(spans[len(spans)-1])
		}
	}
	fillFields(&l, fields["支払方法"], fields["両替機"], fields["利用時間"])
	return l, true
}

func fillFields(l *Locker, payment, change, hours string) {
	l.Payment, l.PaymentJA, l.PaymentKnown, l.ICCard = ParsePayment(payment)
	l.ChangeMachine, l.ChangeMachineRaw = ParseChangeMachine(clean(change))
	l.Hours = ParseHours(clean(hours))
}

// Detail is a locker detail page.
type Detail struct {
	Locker
	Breadcrumb []string `json:"breadcrumb"`
	Neighbours []Locker `json:"neighbours"`
}

// ParseDetail parses /cl/<id>.
func ParseDetail(page []byte, id, baseURL string) (Detail, error) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return Detail{}, err
	}
	d := Detail{Breadcrumb: []string{}, Neighbours: []Locker{}}
	d.ID = id
	d.URL = strings.TrimRight(baseURL, "/") + "/cl/" + id
	if h1 := findFirst(doc, byTag("h1")); h1 != nil {
		d.Name = clean(text(h1))
	}
	if tw := findFirst(doc, byClass("page-title-wrap")); tw != nil {
		if img := findFirst(tw, byTag("img")); img != nil {
			d.ServiceJA = attr(img, "alt")
		}
	}
	if bc := findFirst(doc, byClass("breadcrumb")); bc != nil {
		for _, li := range findAll(bc, byTag("li")) {
			if t := clean(text(li)); t != "" {
				d.Breadcrumb = append(d.Breadcrumb, t)
			}
		}
	}
	if ifr := findFirst(doc, byTag("iframe")); ifr != nil {
		if mm := markerRe.FindStringSubmatch(attr(ifr, "src")); mm != nil {
			d.Lat, d.Lon = parseFloatPtr(mm[1]), parseFloatPtr(mm[2])
			d.MapURL = strPtr(fmt.Sprintf("https://maps.google.com/maps?q=%s,%s", mm[1], mm[2]))
		}
	}
	table := findFirst(doc, byClass("detail-table"))
	if table == nil {
		return d, fmt.Errorf("locker %s: detail table missing (page shape changed?)", id)
	}
	cells := map[string]*html.Node{}
	for _, tr := range findAll(table, byTag("tr")) {
		th := findFirst(tr, byTag("th"))
		td := findFirst(tr, byTag("td"))
		if th != nil && td != nil {
			cells[clean(text(th))] = td
		}
	}
	if td := cells["サービス"]; td != nil && d.ServiceJA == "" {
		d.ServiceJA = clean(text(td))
	}
	d.Service, d.WalkUp = ServiceFromJA(d.ServiceJA)
	d.Sizes = []Size{}
	if td := cells["サイズ・料金"]; td != nil {
		d.Sizes = parseSizes(td)
	}
	d.SizesKnown = len(d.Sizes) > 0
	if td := cells["備考"]; td != nil {
		if n := clean(text(td)); n != "" {
			d.Note = strPtr(n)
		}
	}
	fillFields(&d.Locker, text(cells["支払方法"]), text(cells["両替機"]), text(cells["利用時間"]))
	if nw := findFirst(doc, byClass("nearest-all-wrap")); nw != nil {
		for _, wrap := range findAll(nw, byClass("nearest-wrap")) {
			if l, ok := parseWrap(wrap, baseURL); ok {
				d.Neighbours = append(d.Neighbours, l)
			}
		}
	}
	return d, nil
}

// ---- source calls ----

// Search runs the keyword search and follows "もっと見る" pages until limit.
func (c *Client) Search(ctx context.Context, keyword string, limit, maxPages int) ([]Locker, bool, error) {
	u := c.NaviBase + "/search?q=" + url.QueryEscape(keyword)
	page, err := c.fetch(ctx, u, nil)
	if err != nil {
		return nil, false, err
	}
	rows, err := ParseListing(page, c.NaviBase)
	if err != nil {
		return nil, false, err
	}
	more := len(rows) >= 10
	tok := ""
	if m := csrfRe.FindSubmatch(page); m != nil {
		tok = string(m[1])
	}
	for p := 1; more && len(rows) < limit && p < maxPages && tok != ""; p++ {
		form := url.Values{"cnt": {strconv.Itoa(len(rows))}, "keyword": {keyword}}
		frag, err := c.postForm(ctx, c.NaviBase+"/search/search_more", form, map[string]string{"X-CSRF-Token": tok, "Referer": u})
		if err != nil {
			return rows, more, err
		}
		next, err := ParseListing(frag, c.NaviBase)
		if err != nil {
			return rows, more, err
		}
		rows = append(rows, next...)
		more = len(next) >= 10
	}
	if len(rows) > limit {
		rows, more = rows[:limit], true
	}
	return rows, more, nil
}

// Nearest runs the site's GPS search for a coordinate (about 100 rows).
func (c *Client) Nearest(ctx context.Context, lat, lon float64) ([]Locker, error) {
	gps := c.NaviBase + "/search/gps/"
	page, err := c.fetch(ctx, gps, nil)
	if err != nil {
		return nil, err
	}
	m := csrfRe.FindSubmatch(page)
	if m == nil {
		return nil, fmt.Errorf("GPS search page has no CSRF token (page shape changed?)")
	}
	form := url.Values{
		"location_lat": {strconv.FormatFloat(lat, 'f', 6, 64)},
		"location_lon": {strconv.FormatFloat(lon, 'f', 6, 64)},
	}
	frag, err := c.postForm(ctx, gps+"nearest_cl", form, map[string]string{"X-CSRF-Token": string(m[1]), "Referer": gps})
	if err != nil {
		return nil, err
	}
	return ParseListing(frag, c.NaviBase)
}

// Detail fetches /cl/<id>.
func (c *Client) Detail(ctx context.Context, id string) (Detail, error) {
	page, err := c.fetch(ctx, c.NaviBase+"/cl/"+url.PathEscape(id), nil)
	if err != nil {
		return Detail{}, err
	}
	return ParseDetail(page, id, c.NaviBase)
}
