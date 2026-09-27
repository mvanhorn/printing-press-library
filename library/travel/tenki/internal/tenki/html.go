package tenki

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

func parseHTML(body string) *html.Node { n, _ := html.Parse(strings.NewReader(body)); return n }
func attr(n *html.Node, key string) string {
	if n != nil {
		for _, a := range n.Attr {
			if a.Key == key {
				return a.Val
			}
		}
	}
	return ""
}
func hasClass(n *html.Node, name string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == name {
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
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		walk(child, f)
	}
}
func find(n *html.Node, pred func(*html.Node) bool) *html.Node {
	if n == nil {
		return nil
	}
	if pred(n) {
		return n
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if v := find(child, pred); v != nil {
			return v
		}
	}
	return nil
}
func class(n *html.Node, name string) *html.Node {
	return find(n, func(n *html.Node) bool { return hasClass(n, name) })
}
func id(n *html.Node, name string) *html.Node {
	return find(n, func(n *html.Node) bool { return attr(n, "id") == name })
}
func all(n *html.Node, pred func(*html.Node) bool) []*html.Node {
	result := []*html.Node{}
	walk(n, func(n *html.Node) {
		if pred(n) {
			result = append(result, n)
		}
	})
	return result
}
func childElements(n *html.Node, tag string) []*html.Node {
	result := []*html.Node{}
	if n != nil {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && (tag == "" || c.Data == tag) {
				result = append(result, c)
			}
		}
	}
	return result
}
func nodeText(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var add func(*html.Node)
	add = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style" || n.Data == "rt") {
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			add(c)
		}
	}
	add(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
func rawText(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	walk(n, func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
	})
	return b.String()
}
func imageAlt(n *html.Node) string {
	img := find(n, func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == "img" })
	return attr(img, "alt")
}
func past(n *html.Node) bool {
	return find(n, func(n *html.Node) bool {
		return hasClass(n, "past") || hasClass(n, "gray") || hasClass(n, "grey") || strings.Contains(attr(n, "src"), "_past.")
	}) != nil
}

var numericRE = regexp.MustCompile(`^\s*([-+]?[0-9]+(?:\.[0-9]+)?)\s*(?:℃|°C|%|％|mm/h|mm|㎜|m/s|m)?\s*$`)

func number(s string) *float64 {
	m := numericRE.FindStringSubmatch(strings.ReplaceAll(s, ",", ""))
	if m == nil {
		return nil
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return nil
	}
	return &v
}
func valueAt(nodes []*html.Node, i int) *float64 {
	if i < 0 || i >= len(nodes) {
		return nil
	}
	return number(nodeText(nodes[i]))
}
func altAt(nodes []*html.Node, i int) string {
	if i < 0 || i >= len(nodes) {
		return ""
	}
	return imageAlt(nodes[i])
}
func kindAt(nodes []*html.Node, i int) string {
	if i >= 0 && i < len(nodes) && past(nodes[i]) {
		return "estimated_actual"
	}
	return "forecast"
}

var announceRE = regexp.MustCompile(`announce_datetime:([0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2})`)
var nameRE = regexp.MustCompile(`name:([^\s]+)\s+jiscode:`)

func forecastIssue(doc *html.Node) (time.Time, string) {
	var issue time.Time
	raw := ""
	walk(doc, func(n *html.Node) {
		if !issue.IsZero() || n.Type != html.CommentNode {
			return
		}
		if !strings.Contains(n.Data, "forecast/") {
			return
		}
		m := announceRE.FindStringSubmatch(n.Data)
		if m == nil {
			return
		}
		t, err := time.ParseInLocation("2006-01-02 15:04:05", m[1], JST)
		if err == nil {
			issue, raw = t, m[1]
		}
	})
	return issue, raw
}

var fullDateRE = regexp.MustCompile(`([0-9]{4})年\s*([0-9]{1,2})月\s*([0-9]{1,2})日`)
var monthDayRE = regexp.MustCompile(`([0-9]{1,2})月\s*([0-9]{1,2})日`)
var dayTimeRE = regexp.MustCompile(`([0-9]{1,2})日\s*([0-9]{1,2}):([0-9]{2})`)

func atoi(s string) int { v, _ := strconv.Atoi(s); return v }
func dateFromText(s string, anchor time.Time) time.Time {
	if m := fullDateRE.FindStringSubmatch(s); m != nil {
		return validDate(atoi(m[1]), atoi(m[2]), atoi(m[3]))
	}
	if m := monthDayRE.FindStringSubmatch(s); m != nil {
		d := validDate(anchor.Year(), atoi(m[1]), atoi(m[2]))
		if d.IsZero() {
			return d
		}
		if d.Before(anchor.AddDate(0, -6, 0)) {
			d = d.AddDate(1, 0, 0)
		}
		if d.After(anchor.AddDate(0, 6, 0)) {
			d = d.AddDate(-1, 0, 0)
		}
		return d
	}
	return time.Time{}
}
func validDate(y, m, d int) time.Time {
	v := time.Date(y, time.Month(m), d, 0, 0, 0, 0, JST)
	if v.Year() != y || int(v.Month()) != m || v.Day() != d {
		return time.Time{}
	}
	return v
}
func dayClock(raw string, anchor time.Time) time.Time {
	m := dayTimeRE.FindStringSubmatch(raw)
	if m == nil {
		return time.Time{}
	}
	if atoi(m[2]) > 23 || atoi(m[3]) > 59 {
		return time.Time{}
	}
	best := time.Time{}
	bestDelta := 1000 * time.Hour
	for month := -1; month <= 1; month++ {
		a := time.Date(anchor.Year(), anchor.Month(), 1, 0, 0, 0, 0, JST).AddDate(0, month, 0)
		d := validDate(a.Year(), int(a.Month()), atoi(m[1]))
		if d.IsZero() {
			continue
		}
		d = d.Add(time.Duration(atoi(m[2]))*time.Hour + time.Duration(atoi(m[3]))*time.Minute)
		delta := anchor.Sub(d)
		if delta < 0 {
			delta = -delta
		}
		if delta < bestDelta {
			best, bestDelta = d, delta
		}
	}
	return best
}

func bodyPlace(doc *html.Node, canonical, kind string) Place {
	u := strings.TrimPrefix(canonical, "https://tenki.jp")
	p := Place{ID: strings.Trim(u, "/"), URL: canonical, Kind: kind, Scope: "municipal"}
	p.NameSource = "place_page"
	bread := class(doc, "breadcrumb-navi")
	for _, a := range all(bread, func(n *html.Node) bool { return n.Data == "a" }) {
		url := absoluteURL(attr(a, "href"))
		name := nodeText(a)
		if url == canonical {
			p.Name = name
		}
		if url == canonical && kind == "municipality" {
			p.Name = name
		}
		if strings.HasSuffix(name, "県") || strings.HasSuffix(name, "都") || strings.HasSuffix(name, "府") || name == "北海道" {
			p.Prefecture = name
		}
	}
	if p.Name == "" && kind == "municipality" {
		walk(doc, func(n *html.Node) {
			if p.Name != "" || n.Type != html.CommentNode || !strings.Contains(n.Data, "forecast/") {
				return
			}
			if m := nameRE.FindStringSubmatch(n.Data); m != nil {
				p.Name = m[1]
			}
		})
	}
	if p.Name == "" {
		h := find(id(doc, "main-column"), func(n *html.Node) bool { return n.Data == "h2" })
		if h == nil {
			h = find(doc, func(n *html.Node) bool { return n.Data == "h2" })
		}
		p.Name = nodeText(h)
		for _, suffix := range []string{"の天気・登山情報", "の紅葉見頃時期・天気情報", "の桜開花・満開情報", "の天気"} {
			if i := strings.Index(p.Name, suffix); i >= 0 {
				p.Name = p.Name[:i]
				break
			}
		}
	}
	if kind == "municipality" {
		p.ForecastReferenceURL, p.ForecastReferenceName = canonical, p.Name
		return p
	}
	main := id(doc, "main-column")
	if main == nil {
		main = doc
	}
	var reference string
	for _, a := range all(main, func(n *html.Node) bool { return n.Data == "a" }) {
		raw := absoluteURL(attr(a, "href"))
		if !municipalityPath.MatchString(strings.TrimPrefix(raw, "https://tenki.jp")) {
			continue
		}
		if ref, err := canonicalPlaceURL(raw); err == nil {
			reference = ref
			if attr(a, "id") == "forecast-thumbnail-link" || attr(a, "id") == "forecast-10days-link-btn" || attr(a, "id") == "2week-more-btn" || kind == "mountain" {
				break
			}
		}
	}
	p.ForecastReferenceURL = reference
	if kind == "mountain" {
		p.Scope = "foothill"
		p.ElevationM = number(nodeText(class(main, "value")))
		h := find(main, func(n *html.Node) bool { return n.Data == "h3" && strings.HasPrefix(nodeText(n), "天気予報") })
		p.ForecastReferenceName = strings.Trim(nodeText(class(h, "subtitle")), "()（） ")
	} else {
		walk(main, func(n *html.Node) {
			if p.ForecastReferenceName != "" || n.Type != html.CommentNode || !strings.Contains(n.Data, "forecast/") {
				return
			}
			if m := regexp.MustCompile(`name:([^\s]+)`).FindStringSubmatch(n.Data); m != nil && m[1] != "jiscode:" {
				p.ForecastReferenceName = m[1]
			}
		})
	}
	return p
}
