// Copyright 2026 zjsng. Licensed under Apache-2.0.
package michi

import (
	"bytes"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/cliutil"
	"golang.org/x/net/html"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

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
func hasClass(n *html.Node, c string) bool {
	for _, x := range strings.Fields(attr(n, "class")) {
		if x == c {
			return true
		}
	}
	return false
}
func walk(n *html.Node, f func(*html.Node)) {
	if n == nil {
		return
	}
	f(n)
	for x := n.FirstChild; x != nil; x = x.NextSibling {
		walk(x, f)
	}
}
func first(n *html.Node, p func(*html.Node) bool) *html.Node {
	if n == nil {
		return nil
	}
	if p(n) {
		return n
	}
	for x := n.FirstChild; x != nil; x = x.NextSibling {
		if y := first(x, p); y != nil {
			return y
		}
	}
	return nil
}
func byClass(n *html.Node, c string) *html.Node {
	return first(n, func(x *html.Node) bool { return hasClass(x, c) })
}
func text(n *html.Node) string {
	var b strings.Builder
	walk(n, func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
			b.WriteByte(' ')
		} else if x.Data == "br" {
			b.WriteByte(' ')
		}
	})
	return cliutil.CleanText(b.String())
}
func document(body []byte) (*html.Node, error) {
	if len(body) > 5*1024*1024 {
		return nil, fmt.Errorf("provider HTML exceeds5 MiB bound")
	}
	d, e := html.Parse(bytes.NewReader(body))
	if e != nil {
		return nil, e
	}
	return d, nil
}

var stationPath = regexp.MustCompile(`^/stations/views/([0-9]+)$`)
var noticePath = regexp.MustCompile(`^/notices/views/([0-9]+)$`)
var iconPattern = regexp.MustCompile(`(?:^|/)facility([0-9]{2})(_off)?\.svg$`)

func pathID(href string, re *regexp.Regexp) string {
	u, e := url.Parse(href)
	if e != nil {
		return ""
	}
	if (u.Scheme != "" && u.Scheme != "https" && u.Scheme != "http") || (u.Host != "" && u.Host != "www.michi-no-eki.jp") || (u.Scheme != "" && u.Host == "") || u.User != nil {
		return ""
	}
	m := re.FindStringSubmatch(u.Path)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}
func iconEvidence(n *html.Node) map[string]string {
	out := map[string]string{}
	c := CatalogData().Facilities
	for _, x := range c {
		out[x.Slug] = "unknown"
	}
	walk(n, func(x *html.Node) {
		if x.Data != "img" {
			return
		}
		u, e := url.Parse(attr(x, "src"))
		if e != nil {
			return
		}
		m := iconPattern.FindStringSubmatch(u.Path)
		if len(m) != 3 {
			return
		}
		i, e := strconv.Atoi(m[1])
		if e != nil || i < 1 || i > len(c) {
			return
		}
		out[c[i-1].Slug] = "present"
		if m[2] == "_off" {
			out[c[i-1].Slug] = "not_listed"
		}
	})
	return out
}
func coordinate(lat, lon string) *Coordinates {
	a, e := strconv.ParseFloat(lat, 64)
	if e != nil {
		return nil
	}
	b, e := strconv.ParseFloat(lon, 64)
	if e != nil || math.IsNaN(a) || math.IsNaN(b) || math.IsInf(a, 0) || math.IsInf(b, 0) || a < -90 || a > 90 || b < -180 || b > 180 {
		return nil
	}
	return &Coordinates{a, b}
}

var countPattern = regexp.MustCompile(`([0-9,]+)件`)

func ParseSearch(body []byte, sourceURL, observedAt string) (Search, error) {
	d, e := document(body)
	if e != nil {
		return Search{}, e
	}
	list := byClass(d, "searchList")
	total := byClass(d, "searchTotal")
	if list == nil || total == nil {
		if list == nil && total == nil && nativeEmptyDataset(d, sourceURL) {
			return Search{Stations: []Station{}, SourceURL: sourceURL, ObservedAt: observedAt, SourceTotal: nil, SourceCards: 0, CoverageComplete: true, EmptyResultEvidence: "paired source map-data markers contain an empty dataset", Note: "Provider rendered an empty station/map dataset without publishing a result count.", Warnings: append([]string{}, evidenceWarnings...)}, nil
		}
		return Search{}, fmt.Errorf("provider search layout unrecognized; open the canonical source URL")
	}
	m := countPattern.FindStringSubmatch(text(total))
	if len(m) != 2 {
		return Search{}, fmt.Errorf("provider search count unrecognized")
	}
	count, e := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
	if e != nil {
		return Search{}, e
	}
	coords := map[string]*Coordinates{}
	walk(d, func(n *html.Node) {
		if hasClass(n, "js-data-box") {
			id := pathID(attr(n, "data-link"), stationPath)
			if id != "" {
				coords[id] = coordinate(attr(n, "data-lat"), attr(n, "data-lng"))
			}
		}
	})
	out := Search{Stations: []Station{}, SourceURL: sourceURL, ObservedAt: observedAt, SourceTotal: &count, Warnings: append([]string{}, evidenceWarnings...)}
	seen := map[string]bool{}
	walk(list, func(a *html.Node) {
		if a.Data != "a" {
			return
		}
		id := pathID(attr(a, "href"), stationPath)
		if id == "" || seen[id] {
			return
		}
		h := first(a, func(x *html.Node) bool { return x.Data == "h3" })
		if h == nil {
			return
		}
		s := emptyStation(id, sourceURL, observedAt)
		s.Name = text(h)
		location := text(byClass(a, "txt"))
		parts := strings.SplitN(location, " ", 2)
		if len(parts) > 0 {
			s.Prefecture = parts[0]
		}
		if len(parts) > 1 {
			s.Municipality = parts[1]
		}
		s.Facilities = iconEvidence(a)
		s.Coordinates = coords[id]
		seen[id] = true
		out.Stations = append(out.Stations, s)
	})
	out.SourceCards = len(out.Stations)
	if count > 0 && len(out.Stations) == 0 {
		return Search{}, fmt.Errorf("provider reported%d stations but no station cards parsed", count)
	}
	out.CoverageComplete = len(out.Stations) == count
	return out, nil
}
func parking(raw string) Parking {
	p := Parking{Raw: raw, Availability: "unknown", VehicleFit: "unknown"}
	patterns := []struct {
		re     string
		target **int
	}{{`大型[：:]\s*([0-9,]+)`, &p.Large}, {`普通車[：:]\s*([0-9,]+)`, &p.Cars}, {`身障者(?:用)?[：:]?\s*([0-9,]+)`, &p.Accessible}}
	for _, x := range patterns {
		m := regexp.MustCompile(x.re).FindStringSubmatch(raw)
		if len(m) == 2 {
			i, e := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
			if e == nil {
				v := i
				*x.target = &v
			}
		}
	}
	return p
}
func ParseStation(body []byte, id, sourceURL, observedAt string) (Station, error) {
	d, e := document(body)
	if e != nil {
		return Station{}, e
	}
	title := byClass(d, "viewTitle")
	if title == nil {
		return Station{}, fmt.Errorf("provider station layout unrecognized for ID %s; open %s", id, sourceURL)
	}
	s := emptyStation(id, sourceURL, observedAt)
	s.Prefecture = text(first(title, func(n *html.Node) bool { return n.Data == "span" }))
	s.Facilities = iconEvidence(byClass(d, "viewFacility"))
	main := first(d, func(n *html.Node) bool { return n.Data == "main" })
	walk(main, func(n *html.Node) {
		if n.Data != "dl" {
			return
		}
		key := text(first(n, func(x *html.Node) bool { return x.Data == "dt" }))
		dd := first(n, func(x *html.Node) bool { return x.Data == "dd" })
		value := text(dd)
		switch key {
		case "道の駅名":
			s.Name = value
		case "所在地":
			s.Address = value
		case "TEL":
			s.Phone = value
		case "駐車場":
			s.Parking = parking(value)
		case "営業時間":
			s.PublishedHours = value
		case "マップコード":
			s.MapCode = value
		case "ホームページ", "ホームページ2":
			walk(dd, func(a *html.Node) {
				if a.Data != "a" {
					return
				}
				u, e := url.Parse(attr(a, "href"))
				if e == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
					s.OperatorURLs = append(s.OperatorURLs, u.String())
				}
			})
		}
	})
	if s.Name == "" {
		return Station{}, fmt.Errorf("provider station name missing for ID %s", id)
	}
	walk(main, func(n *html.Node) {
		if n.Data != "iframe" {
			return
		}
		u, e := url.Parse(attr(n, "src"))
		if e != nil {
			return
		}
		q := strings.Split(u.Query().Get("q"), ",")
		if len(q) == 2 {
			s.Coordinates = coordinate(q[0], q[1])
		}
	})
	reg := byClass(d, "viewContent")
	if reg != nil {
		m := regexp.MustCompile(`＜[^＞]*登録[^＞]*＞`).FindString(text(reg))
		s.Registration = m
	}
	return s, nil
}
func dateISO(raw string) string {
	var y, m, d int
	if _, e := fmt.Sscanf(raw, "%d-%d-%d", &y, &m, &d); e != nil {
		if _, e = fmt.Sscanf(raw, "%d年%d月%d日", &y, &m, &d); e != nil {
			return "unknown"
		}
	}
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, jst)
	if t.Year() != y || int(t.Month()) != m || t.Day() != d {
		return "unknown"
	}
	return t.Format("2006-01-02")
}
func ParseNotices(body []byte, page int, sourceURL, observedAt string) ([]Notice, *int, error) {
	d, e := document(body)
	if e != nil {
		return nil, nil, e
	}
	list := byClass(d, "noticesList")
	if list == nil {
		return nil, nil, fmt.Errorf("provider notice list layout unrecognized")
	}
	out := []Notice{}
	seen := map[string]bool{}
	var entryErr error
	walk(list, func(a *html.Node) {
		if a.Data != "a" {
			return
		}
		id := pathID(attr(a, "href"), noticePath)
		if id == "" || seen[id] {
			return
		}
		t := first(a, func(n *html.Node) bool { return n.Data == "time" })
		date := dateISO(attr(t, "datetime"))
		if date == "unknown" {
			date = dateISO(text(t))
		}
		n := Notice{ID: id, Title: text(first(a, func(n *html.Node) bool { return n.Data == "p" })), Prefecture: text(first(a, func(n *html.Node) bool { return n.Data == "span" })), PublishedDate: date, Timezone: "Asia/Tokyo", URL: Origin + "/notices/views/" + id, SourceURL: sourceURL, ObservedAt: observedAt, StationIDs: []string{}}
		if n.Title == "" {
			entryErr = fmt.Errorf("provider notice list title layout unrecognized for ID %s", id)
			return
		}
		out = append(out, n)
		seen[id] = true
	})
	if entryErr != nil {
		return nil, nil, entryErr
	}
	var next *int
	walk(d, func(a *html.Node) {
		if a.Data != "a" {
			return
		}
		u, e := url.Parse(attr(a, "href"))
		if e != nil || u.Path != "/notices" {
			return
		}
		v, e := strconv.Atoi(u.Query().Get("page"))
		if e == nil && v > page && (next == nil || v < *next) {
			x := v
			next = &x
		}
	})
	return out, next, nil
}
func ParseNotice(body []byte, id, sourceURL, observedAt string) (Notice, error) {
	d, e := document(body)
	if e != nil {
		return Notice{}, e
	}
	article := byClass(d, "noticesView__content")
	if article == nil {
		return Notice{}, fmt.Errorf("provider notice layout unrecognized for ID %s", id)
	}
	n := Notice{ID: id, Title: text(first(article, func(n *html.Node) bool { return n.Data == "h3" })), PublishedDate: dateISO(text(byClass(article, "createdDate"))), Timezone: "Asia/Tokyo", URL: Origin + "/notices/views/" + id, SourceURL: sourceURL, ObservedAt: observedAt, StationIDs: []string{}}
	if n.Title == "" {
		return Notice{}, fmt.Errorf("provider notice title missing")
	}
	seen := map[string]bool{}
	paras := []string{}
	walk(article, func(x *html.Node) {
		if x.Data == "a" {
			sid := pathID(attr(x, "href"), stationPath)
			if sid != "" && !seen[sid] {
				n.StationIDs = append(n.StationIDs, sid)
				seen[sid] = true
			}
		}
		if x.Data == "p" && !hasClass(x, "createdDate") {
			paras = append(paras, text(x))
		}
	})
	runes := []rune(strings.Join(paras, " "))
	if len(runes) > 160 {
		runes = runes[:160]
		n.ExcerptTruncated = true
	}
	n.Excerpt = string(runes)
	return n, nil
}

// nativeEmptyDataset recognizes the provider's observed zero-result SSR contract,
// rather than interpreting a generic shell or missing search elements as empty.
func nativeEmptyDataset(d *html.Node, sourceURL string) bool {
	u, err := url.Parse(sourceURL)
	if err != nil || !strings.HasPrefix(u.Path, "/stations/search/") {
		return false
	}
	page := byClass(d, "pageStation")
	if page == nil || byClass(page, "searchMap") == nil || byClass(page, "searchTop") == nil {
		return false
	}
	recognized := false
	walk(page, func(n *html.Node) {
		if n.Type != html.CommentNode || strings.TrimSpace(n.Data) != "配列データ" {
			return
		}
		next := n.NextSibling
		for next != nil && next.Type == html.TextNode && strings.TrimSpace(next.Data) == "" {
			next = next.NextSibling
		}
		if next == nil || next.Type != html.ElementNode || next.Data != "div" || !strings.Contains(strings.ReplaceAll(attr(next, "style"), " ", ""), "display:none") {
			return
		}
		for child := next.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode || (child.Type == html.TextNode && strings.TrimSpace(child.Data) != "") {
				return
			}
		}
		close := next.NextSibling
		for close != nil && close.Type == html.TextNode && strings.TrimSpace(close.Data) == "" {
			close = close.NextSibling
		}
		if close != nil && close.Type == html.CommentNode && strings.TrimSpace(close.Data) == "配列データ" {
			recognized = true
		}
	})
	return recognized
}
