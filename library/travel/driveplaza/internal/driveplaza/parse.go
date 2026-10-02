package driveplaza

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/driveplaza/internal/cliutil"
	"golang.org/x/net/html"
)

func mustDocument(b []byte) *html.Node { n, _ := html.Parse(bytes.NewReader(b)); return n }

type match func(*html.Node) bool

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

func hasAttr(n *html.Node, key string) bool {
	if n == nil {
		return false
	}
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}
func tag(name string) match {
	return func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == name }
}
func byClass(name string) match {
	return func(n *html.Node) bool { return strings.Contains(" "+attr(n, "class")+" ", " "+name+" ") }
}
func byAttr(k, v string) match {
	return func(n *html.Node) bool { return n.Type == html.ElementNode && attr(n, k) == v }
}
func all(n *html.Node, m match) []*html.Node {
	out := []*html.Node{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil {
			return
		}
		if m(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}
func first(n *html.Node, m match) *html.Node {
	if n == nil {
		return nil
	}
	if m(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if f := first(c, m); f != nil {
			return f
		}
	}
	return nil
}
func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil || n.Type == html.CommentNode || (n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style")) {
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		if n.Data == "br" {
			b.WriteString(" | ")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return cliutil.CleanText(b.String())
}
func nullable(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
func boolptr(b bool) *bool { return &b }
func number(s string) *int {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	if !regexp.MustCompile(`^[0-9]+$`).MatchString(s) {
		return nil
	}
	n, e := strconv.Atoi(s)
	if e != nil {
		return nil
	}
	return &n
}

var durationPattern = regexp.MustCompile(`^(?:(\d+)h)?(?:(\d+)min)?$`)

func minutes(s string) *int {
	m := durationPattern.FindStringSubmatch(strings.ReplaceAll(s, " ", ""))
	if len(m) != 3 || (m[1] == "" && m[2] == "") {
		return nil
	}
	h, _ := strconv.Atoi(m[1])
	v, _ := strconv.Atoi(m[2])
	if v >= 60 {
		return nil
	}
	n := h*60 + v
	return &n
}
func distance(s string) *float64 {
	s = strings.TrimSpace(strings.TrimSuffix(s, "km"))
	n, e := strconv.ParseFloat(s, 64)
	if e != nil || n < 0 {
		return nil
	}
	return &n
}

func parseIC(b []byte, ja bool) ([]Interchange, error) {
	var root struct {
		XMLName xml.Name                                                    `xml:"NexcoIC"`
		Items   []struct{ Code, Name, Yomi, Type, RoadNo, RoadName string } `xml:"IcItem"`
	}
	if err := xml.Unmarshal(b, &root); err != nil {
		return nil, fmt.Errorf("expected public NexcoIC XML, source response changed: %w", err)
	}
	out := []Interchange{}
	for _, i := range root.Items {
		if !icID.MatchString(i.Code) || i.Name == "" {
			return nil, fmt.Errorf("source IC record has missing identity")
		}
		item := Interchange{ID: i.Code, Name: i.Name, Type: i.Type, RoadID: nullable(i.RoadNo), RoadName: nullable(i.RoadName)}
		if ja {
			item.NameJA = nullable(i.Name)
			item.Reading = nullable(i.Yomi)
		}
		out = append(out, item)
	}
	return out, nil
}
func parseRoads(b []byte) ([]Road, error) {
	doc := mustDocument(b)
	selectNode := first(doc, byAttr("name", "HIGHWAY"))
	if selectNode == nil {
		return nil, fmt.Errorf("source road catalog missing HIGHWAY select; source layout changed")
	}
	out := []Road{}
	seen := map[string]bool{}
	for _, o := range all(selectNode, tag("option")) {
		id := attr(o, "value")
		if !roadID.MatchString(id) || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, Road{ID: id, NameEN: text(o)})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("source road catalog contains no road IDs")
	}
	return out, nil
}
func parseFacilities(b []byte) ([]Facility, error) {
	doc := mustDocument(b)
	out := []Facility{}
	seen := map[string]bool{}
	for _, n := range all(doc, byAttr("name", "ITEM")) {
		id := attr(n, "value")
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		label := n.Parent
		for label != nil && label.Data != "label" {
			label = label.Parent
		}
		if label == nil {
			return nil, fmt.Errorf("facility ID%s has no label", id)
		}
		out = append(out, Facility{ID: id, Label: text(label), Group: strings.HasPrefix(id, "770")})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("source facility catalog missing ITEM inputs")
	}
	return out, nil
}

var sapaPath = regexp.MustCompile(`^/sapa/([0-9]{4}/[0-9]{7}/[12])/$`)

func stopPath(href string) string {
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if u.Host != "" && u.Host != "www.driveplaza.com" && u.Host != "en.driveplaza.com" {
		return ""
	}
	return u.Path
}

var parkingPattern = regexp.MustCompile(`Large[：:]\s*(\d+)\s*[／/]\s*Small[：:]\s*(\d+)`)
var iconPattern = regexp.MustCompile(`icon_shisetsu_(green|gray)_(\d{2})\.svg`)
var categories = map[string]string{"01": "food", "02": "souvenirs", "03": "gas", "04": "ev_charging", "05": "daily_supplies", "06": "rest_accommodation", "07": "aed_medical", "08": "accessibility", "09": "expressway_etc", "10": "children", "11": "pets"}

func parseStops(b []byte, en, jp string) ([]Stop, error) {
	doc := mustDocument(b)
	heading := text(first(doc, tag("h1")))
	if (heading != "Service area search results" && heading != "サービスエリア検索結果") || first(doc, byAttr("name", "HIGHWAY")) == nil {
		return nil, fmt.Errorf("source SA/PA result page contract missing; inspect official search")
	}
	out := []Stop{}
	seen := map[string]bool{}
	for _, box := range all(doc, byClass("box-sapa")) {
		a := first(first(box, byClass("ttl-sapaName")), tag("a"))
		m := sapaPath.FindStringSubmatch(stopPath(attr(a, "href")))
		if len(m) != 2 {
			return nil, fmt.Errorf("source SA/PA result box lacks a valid directional identity; source layout changed")
		}
		if text(a) == "" {
			return nil, fmt.Errorf("source SA/PA result box lacks a name; source layout changed")
		}
		id := m[1]
		if seen[id] {
			continue
		}
		seen[id] = true
		parts := strings.Split(id, "/")
		direction := "up"
		if parts[2] == "2" {
			direction = "down"
		}
		fac := map[string]*bool{}
		for _, category := range categories {
			fac[category] = nil
		}
		for _, img := range all(first(box, byClass("li-icons")), tag("img")) {
			m := iconPattern.FindStringSubmatch(attr(img, "src"))
			if len(m) != 3 {
				continue
			}
			category, ok := categories[m[2]]
			if !ok {
				continue
			}
			fac[category] = boolptr(m[1] == "green" && !byClass("none")(img.Parent))
		}
		p := parkingPattern.FindStringSubmatch(text(first(box, byClass("txt-info"))))
		var large, small *int
		if len(p) == 3 {
			large = number(p[1])
			small = number(p[2])
		}
		out = append(out, Stop{ID: id, NameEN: text(a), Direction: direction, RoadID: parts[0], RoadName: text(first(box, byClass("txt-road"))), URL: en + "/sapa/" + id + "/", URLJA: jp + "/sapa/" + id + "/", ParkingLarge: large, ParkingSmall: small, FacilityCategories: fac})
	}
	return out, nil
}
func parseDetail(b []byte, id, en, jp string) (StopDetail, error) {
	doc := mustDocument(b)
	name := text(first(doc, byClass("txt-title")))
	if name == "" || first(doc, byClass("facility-wrap")) == nil {
		return StopDetail{}, fmt.Errorf("source SA/PA detail identity or facility sections missing; source page changed")
	}
	parts := strings.Split(id, "/")
	direction := "up"
	if parts[2] == "2" {
		direction = "down"
	}
	out := StopDetail{ID: id, NameEN: name, Direction: direction, RoadID: parts[0], RoadName: text(first(doc, byClass("txt-way"))), URL: en + "/sapa/" + id + "/", URLJA: jp + "/sapa/" + id + "/", Sections: []FacilitySection{}, Nearby: []StopLink{}, HoursBasis: "Source weekday guidance, copied verbatim; holiday exceptions and current open status unknown."}
	for _, h := range all(first(doc, byClass("facility-wrap")), byClass("ttl-facility")) {
		var parts []string
		for p := h.NextSibling; p != nil; p = p.NextSibling {
			if p.Type == html.ElementNode && p.Data == "h3" {
				break
			}
			t := text(p)
			if t != "" {
				parts = append(parts, t)
			}
		}
		out.Sections = append(out.Sections, FacilitySection{Category: text(h), Text: strings.Join(parts, " | ")})
	}
	seen := map[string]bool{id: true}
	for _, a := range all(doc, tag("a")) {
		m := sapaPath.FindStringSubmatch(stopPath(attr(a, "href")))
		if len(m) != 2 || seen[m[1]] || text(a) == "" || strings.Contains(strings.ToLower(text(a)), "display") {
			continue
		}
		seen[m[1]] = true
		d := regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*km`).FindStringSubmatch(text(a.Parent))
		var km *float64
		if len(d) == 2 {
			km = distance(d[1])
		}
		out.Nearby = append(out.Nearby, StopLink{ID: m[1], Name: text(a), URL: en + "/sapa/" + m[1] + "/", DistanceKM: km})
		if len(out.Nearby) >= 8 {
			break
		}
	}
	return out, nil
}

func japaneseDetailName(b []byte) string {
	doc := mustDocument(b)
	if n := text(first(doc, byClass("txt-title"))); n != "" {
		return n
	}
	// Branded Pasar pages use a distinct layout; preserve their official
	// Japanese title instead of inventing a transliteration.
	n := attr(first(doc, byAttr("property", "og:title")), "content")
	if n == "" {
		return ""
	}
	return cliutil.CleanText(strings.Split(n, " | ")[0])
}

var blogPath = regexp.MustCompile(`^javascript:goSapaBlog\('(/sapa/[0-9]{4}/[0-9]{7}/[12]/)'\)$`)

func parseRoutes(b []byte, detail bool, en string) ([]RouteAlternative, error) {
	doc := mustDocument(b)
	table := first(doc, byClass("table-route"))
	if table == nil {
		return nil, fmt.Errorf("no source route alternatives; check exact English IC names, schedule and route restrictions at%s/dp/SearchTopEN", en)
	}
	// Summary rows correspond to immediate route panels. Nested ui-tabbox
	// elements are toll tabs within a route and must not consume an alternative.
	panels := []*html.Node{}
	if content := first(doc, byClass("js-doubleTabContent")); content != nil {
		for panel := content.FirstChild; panel != nil; panel = panel.NextSibling {
			if byClass("ui-tabbox")(panel) {
				panels = append(panels, panel)
			}
		}
	}
	out := []RouteAlternative{}
	for _, tr := range all(table, tag("tr")) {
		td := all(tr, tag("td"))
		if len(td) == 0 {
			continue
		}
		if len(td) != 3 {
			return nil, fmt.Errorf("source route summary column count changed")
		}
		fees := all(td[1], byClass("cell"))
		times := all(td[2], byClass("cell"))
		if len(fees) != 3 || len(times) != 3 {
			return nil, fmt.Errorf("source route toll/time columns changed")
		}
		i := len(out)
		r := RouteAlternative{ID: fmt.Sprintf("route-%d", i+1), StandardJPY: number(text(first(fees[0], tag("em")))), ETCJPY: number(text(first(fees[1], tag("em")))), ETC2JPY: number(text(first(fees[2], tag("em")))), IgnoringMinutes: minutes(text(times[0])), ConsideringMinutes: minutes(text(times[1])), DistanceKM: distance(text(times[2])), Warnings: []string{}}
		if i < len(panels) {
			panel := panels[i]
			for _, n := range all(panel, byClass("c-blockSkin03")) {
				if first(n, byClass("ui-carousel")) != nil || byClass("seamlessMsgArea")(n) {
					continue
				}
				t := text(n)
				if t != "" && len(t) < 1500 && !strings.Contains(t, "SAPA") {
					r.Warnings = append(r.Warnings, t)
				}
			}
			if detail {
				r.Stops = []StopLink{}
				r.ForecastURLs = []string{}
				seen := map[string]bool{}
				for _, a := range all(panel, tag("a")) {
					href := attr(a, "href")
					if m := blogPath.FindStringSubmatch(href); len(m) == 2 {
						p := m[1]
						if seen[p] {
							continue
						}
						seen[p] = true
						box := a.Parent
						for box != nil && !byClass("js_sapa_icon_area")(box) {
							box = box.Parent
						}
						r.Stops = append(r.Stops, StopLink{ID: strings.Trim(strings.TrimPrefix(p, "/sapa/"), "/"), Name: text(first(box, byClass("txt-sa"))), URL: en + p})
					}
					if u, e := url.Parse(href); e == nil && u.Scheme == "https" && (u.Host == "www.drivetraffic.jp" || u.Host == "en-www.drivetraffic.jp") && u.Query().Get("fctime") != "" && !seen[href] {
						seen[href] = true
						r.ForecastURLs = append(r.ForecastURLs, href)
					}
				}
			}
		}
		out = append(out, r)
		if len(out) > 5 {
			return nil, fmt.Errorf("source returned more than5 route alternatives; source contract changed")
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("source route summary contained no alternatives")
	}
	return out, nil
}

func parseNotices(b []byte) ([]Notice, *string, error) {
	var feed struct {
		XMLName xml.Name `xml:"rss"`
		Channel struct {
			Updated string `xml:"lastBuildDate"`
			Items   []struct {
				Title     string `xml:"title"`
				URL       string `xml:"link"`
				GUID      string `xml:"guid"`
				Published string `xml:"pubDate"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.Unmarshal(b, &feed); err != nil {
		return nil, nil, fmt.Errorf("expected official traffic RSS: %w", err)
	}
	parseDate := func(raw string) *string {
		for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC822Z} {
			if t, e := time.Parse(layout, raw); e == nil {
				return nullable(t.Format(time.RFC3339))
			}
		}
		return nil
	}
	out := []Notice{}
	for _, n := range feed.Channel.Items {
		if n.Title == "" || n.URL == "" {
			return nil, nil, fmt.Errorf("traffic notice missing title or link")
		}
		u, e := url.Parse(n.URL)
		if e != nil || u.Scheme != "https" || u.Host == "" {
			return nil, nil, fmt.Errorf("traffic notice contains invalid source URL")
		}
		id := n.GUID
		if id == "" {
			id = n.URL
		}
		out = append(out, Notice{ID: id, Title: cliutil.CleanText(n.Title), PublishedAt: parseDate(n.Published), PublishedRaw: n.Published, URL: n.URL})
	}
	return out, parseDate(feed.Channel.Updated), nil
}
