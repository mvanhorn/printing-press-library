package walkerplus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func hasClass(n *html.Node, class string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}
func nodes(n *html.Node, predicate func(*html.Node) bool) []*html.Node {
	out := []*html.Node{}
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if predicate(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(n)
	return out
}
func byClass(n *html.Node, class string) []*html.Node {
	return nodes(n, func(n *html.Node) bool { return hasClass(n, class) })
}
func nodeText(n *html.Node) string {
	var b strings.Builder
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		if n.Data == "br" || n.Data == "p" || n.Data == "div" {
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
		if n.Data == "p" || n.Data == "div" {
			b.WriteByte(' ')
		}
	}
	visit(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
func classText(n *html.Node, class string) string {
	found := byClass(n, class)
	if len(found) > 0 {
		return nodeText(found[0])
	}
	return ""
}
func parseDoc(b []byte) (*html.Node, error) { return html.Parse(bytes.NewReader(b)) }
func eventIDFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "event" && eventIDRE.MatchString(parts[1]) {
		return parts[1]
	}
	return ""
}

func emptyEvent(id, title, source string) Event {
	return Event{Timezone: "Asia/Tokyo", ID: id, TitleJA: title, SourceURL: source, DateCertainty: "unknown", Categories: []CatalogItem{}, Schedule: Schedule{OccurrenceDates: []string{}, ExcludedDates: []string{}, ClosedWeekdays: []string{}, Unresolved: []string{}}, Admission: Admission{Status: "unknown"}, OrganizerURLs: []string{}, Evidence: []Evidence{}, Sources: []Source{}}
}

func schemaEvents(doc *html.Node) ([]map[string]any, error) {
	out := []map[string]any{}
	var collect func(any)
	collect = func(v any) {
		switch t := v.(type) {
		case []any:
			for _, x := range t {
				collect(x)
			}
		case map[string]any:
			typ, _ := t["@type"].(string)
			if typ == "Event" {
				out = append(out, t)
			}
			if graph, ok := t["@graph"]; ok {
				collect(graph)
			}
		}
	}
	for _, script := range nodes(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "script" && attr(n, "type") == "application/ld+json"
	}) {
		var v any
		raw := nodeText(script)
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			if strings.Contains(raw, `"Event"`) {
				return nil, fmt.Errorf("Walkerplus Event JSON-LD shape invalid: %w", err)
			}
			continue
		}
		collect(v)
	}
	return out, nil
}
func stringField(m map[string]any, k string) string    { s, _ := m[k].(string); return s }
func object(m map[string]any, k string) map[string]any { out, _ := m[k].(map[string]any); return out }
func validSourceDate(s string) *string {
	if len(s) >= 10 {
		s = s[:10]
	}
	if _, err := parseDate(s); err == nil {
		return strptr(s)
	}
	return nil
}
func fillSchema(e *Event, s map[string]any) {
	e.StartDate = validSourceDate(stringField(s, "startDate"))
	e.EndDate = validSourceDate(stringField(s, "endDate"))
	e.Description = strptr(stringField(s, "description"))
	loc := object(s, "location")
	addr := object(loc, "address")
	if !e.venueFromDisplay && stringField(loc, "name") != "" {
		e.Location.Venue = strptr(stringField(loc, "name"))
	}
	e.Location.Address = strptr(stringField(addr, "streetAddress"))
	region, locality := stringField(addr, "addressRegion"), stringField(addr, "addressLocality")
	if region != "" {
		e.Location.PrefectureJA = strptr(region)
		e.Location.PrefectureCode = nil
	}
	if locality != "" {
		if value(e.Location.CityJA) != "" && value(e.Location.CityJA) != locality {
			e.Location.CityCode = nil
		}
		e.Location.CityJA = strptr(locality)
		if c, ok := lookup(knownCities, locality); ok {
			e.Location.CityCode = strptr(c.Code)
		}
	}
	if p, ok := lookup(prefectures, value(e.Location.PrefectureJA)); ok {
		e.Location.PrefectureCode = strptr(p.Code)
	}
	if s := stringField(s, "url"); s != "" {
		if u, err := url.Parse(s); err == nil && (u.Scheme == "https" || u.Scheme == "http") {
			e.OrganizerURLs = appendUnique(e.OrganizerURLs, s)
		}
	}
	setDateCertainty(e)
}
func setDateCertainty(e *Event) {
	e.DateCertainty = "unknown"
	if e.StartDate != nil && e.EndDate != nil {
		if *e.EndDate >= *e.StartDate {
			e.DateCertainty = "exact"
			y, _ := strconv.Atoi((*e.StartDate)[:4])
			e.EditionYear = &y
		}
	}
	if approximate(value(e.Schedule.Raw)) {
		e.DateCertainty = "approximate"
	}
}
func approximate(s string) bool {
	for _, word := range []string{"上旬", "中旬", "下旬", "見頃", "予定", "例年", "頃"} {
		if strings.Contains(s, word) {
			return true
		}
	}
	return false
}
func appendUnique(items []string, item string) []string {
	for _, s := range items {
		if s == item {
			return items
		}
	}
	return append(items, item)
}

type listing struct {
	events []Event
	next   *string
	total  *int
	years  []string
	cities []CatalogItem
}

func cityLinks(doc *html.Node) []CatalogItem {
	out := []CatalogItem{}
	seen := map[string]int{}
	for _, a := range nodes(doc, func(n *html.Node) bool { return n.Data == "a" }) {
		path := attr(a, "href")
		if m := areaPathRE.FindStringSubmatch(path); m != nil && len(m[1]) == 9 {
			name := nodeText(a)
			if name == "" {
				continue
			}
			item := CatalogItem{Code: m[1], NameJA: name, Aliases: nonempty(m[2]), Path: path, Kind: "city", PrefectureCode: strptr(m[1][:6])}
			if index, ok := seen[m[1]]; ok {
				if len(out[index].Aliases) == 0 && m[2] != "" {
					out[index] = item
				}
				continue
			}
			seen[m[1]] = len(out)
			out = append(out, item)
		}
	}
	return out
}

var totalRE = regexp.MustCompile(`全([0-9,]+)件`)
var yearRE = regexp.MustCompile(`([12][0-9]{3})年`)
var areaPathRE = regexp.MustCompile(`^/event_list/(ar[0-9]{4}(?:[0-9]{3})?)(?:/([^/]+))?/$`)
var categoryPathRE = regexp.MustCompile(`^/event_list/(eg[0-9]+)/$`)

func parseListing(p page) (listing, error) {
	doc, err := parseDoc(p.body)
	if err != nil {
		return listing{}, err
	}
	schemas, err := schemaEvents(doc)
	if err != nil {
		return listing{}, err
	}
	out := listing{events: []Event{}, years: []string{}, cities: []CatalogItem{}}
	cards := byClass(doc, "m-mainlist__item")
	for _, card := range cards {
		anchors := nodes(card, func(n *html.Node) bool { return n.Data == "a" && eventIDFromPath(attr(n, "href")) != "" })
		if len(anchors) == 0 {
			continue
		}
		id := eventIDFromPath(attr(anchors[0], "href"))
		title := classText(card, "m-mainlist-item__ttl")
		if title == "" {
			title = nodeText(anchors[0])
		}
		if title == "" {
			return listing{}, fmt.Errorf("Walkerplus listing card has no event title")
		}
		u, _ := url.Parse(p.source.URL)
		u.Path = "/event/" + id + "/"
		u.RawQuery = ""
		e := emptyEvent(id, title, u.String())
		e.Sources = append(e.Sources, p.source)
		e.Schedule.Raw = strptr(strings.TrimSpace(strings.ReplaceAll(classText(card, "m-mainlist-item-event__period"), "終了間近", "")))
		e.Location.Venue = strptr(classText(card, "m-mainlist-item-event__place"))
		for _, a := range nodes(card, func(n *html.Node) bool { return n.Data == "a" }) {
			path := attr(a, "href")
			if m := areaPathRE.FindStringSubmatch(path); m != nil {
				name := nodeText(a)
				if len(m[1]) == 6 {
					e.Location.PrefectureCode = strptr(m[1])
					e.Location.PrefectureJA = strptr(name)
				} else {
					e.Location.CityCode = strptr(m[1])
					e.Location.CityJA = strptr(name)
				}
			}
			if m := categoryPathRE.FindStringSubmatch(path); m != nil {
				code := categoryCode(m[1])
				e.Categories = append(e.Categories, CatalogItem{Code: code, NameJA: nodeText(a), Aliases: []string{}, Path: "/event_list/" + code + "/", Kind: "category"})
			}
		}
		matches := []map[string]any{}
		for _, s := range schemas {
			if stringField(s, "name") == title {
				venue := stringField(object(s, "location"), "name")
				if e.Location.Venue == nil || venue == "" || value(e.Location.Venue) == venue {
					matches = append(matches, s)
				}
			}
		}
		if len(matches) == 1 {
			fillSchema(&e, matches[0])
		}
		parseSchedule(&e)
		for _, tag := range byClass(card, "m-mainlist-item__tagsitem") {
			raw := nodeText(tag)
			if raw == "入場無料" || raw == "観覧無料" || raw == "参加無料" {
				applyAdmission(&e, raw, p.source.URL)
			}
			applyIndoor(&e, raw, p.source.URL)
		}
		if raw := value(e.Schedule.Raw); strings.Contains(raw, "開催中止") || strings.Contains(raw, "開催延期") {
			e.Cancellation = strptr(raw)
		}
		out.events = append(out.events, e)
		for _, m := range yearRE.FindAllStringSubmatch(value(e.Schedule.Raw), -1) {
			out.years = appendUnique(out.years, m[1])
		}
	}
	out.cities = cityLinks(doc)
	for _, n := range nodes(doc, func(n *html.Node) bool { return n.Data == "a" }) {
		path := attr(n, "href")
		if strings.Contains(attr(n, "rel"), "next") || (strings.Contains(attr(n, "class"), "pager") && strings.Contains(nodeText(n), "次")) {
			if strings.HasPrefix(path, "/event_list/") {
				out.next = strptr(path)
			}
		}
	}
	// The site uses a pager's numbered links without rel=next on some routes.
	current, _ := url.Parse(p.source.URL)
	currentPage := 1
	if strings.HasSuffix(current.Path, ".html") {
		s := current.Path[strings.LastIndex(current.Path, "/")+1:]
		currentPage, _ = strconv.Atoi(strings.TrimSuffix(s, ".html"))
	}
	for _, n := range nodes(doc, func(n *html.Node) bool { return n.Data == "a" }) {
		path := attr(n, "href")
		if strings.HasPrefix(path, "/event_list/") && strings.HasSuffix(path, "/"+strconv.Itoa(currentPage+1)+".html") && pageBase(path) == pageBase(current.Path) {
			out.next = strptr(path)
			break
		}
	}
	for _, n := range nodes(doc, func(n *html.Node) bool { return strings.Contains(attr(n, "class"), "pager") }) {
		if m := totalRE.FindStringSubmatch(nodeText(n)); m != nil {
			v, _ := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
			out.total = &v
			break
		}
	}
	if len(cards) == 0 {
		text := nodeText(doc)
		zero := out.total != nil && *out.total == 0
		for _, s := range []string{"イベントが見つかりません", "該当するイベントはありません", "イベントはありません", "該当するイベントがありません", "0件"} {
			zero = zero || strings.Contains(text, s)
		}
		if !zero {
			return listing{}, fmt.Errorf("Walkerplus listing shape changed: event cards absent")
		}
	}
	return out, nil
}
func nonempty(s string) []string {
	if s == "" {
		return []string{}
	}
	return []string{s}
}
func pageBase(s string) string {
	if strings.HasSuffix(s, ".html") {
		return s[:strings.LastIndex(s, "/")+1]
	}
	return s
}

func parseDetail(p page, e *Event) ([]string, error) {
	doc, err := parseDoc(p.body)
	if err != nil {
		return nil, err
	}
	schemas, err := schemaEvents(doc)
	if err != nil {
		return nil, err
	}
	h1 := nodes(doc, func(n *html.Node) bool { return n.Data == "h1" })
	rows := nodes(doc, func(n *html.Node) bool { return n.Data == "tr" && hasClass(n, "m-infotable__row") })
	if len(schemas) == 0 && len(rows) == 0 {
		return nil, fmt.Errorf("Walkerplus event shape changed: source facts absent")
	}
	if len(schemas) > 0 {
		if e.TitleJA == "" && len(h1) > 0 {
			e.TitleJA = nodeText(h1[0])
		}
		found := false
		for _, s := range schemas {
			if e.TitleJA == "" || e.TitleJA == stringField(s, "name") {
				if e.TitleJA == "" {
					e.TitleJA = stringField(s, "name")
				}
				fillSchema(e, s)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("Walkerplus detail title does not match event candidate")
		}
	}
	for _, row := range rows {
		th := nodes(row, func(n *html.Node) bool { return n.Data == "th" })
		td := nodes(row, func(n *html.Node) bool { return n.Data == "td" })
		if len(th) == 0 || len(td) == 0 {
			continue
		}
		key := nodeText(th[0])
		raw := nodeText(td[0])
		raw = strings.ReplaceAll(raw, "[地図]", "")
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		switch key {
		case "開催日":
			e.Schedule.Raw = strptr(raw)
			e.Evidence = append(e.Evidence, Evidence{Field: "schedule", Text: raw, SourceURL: p.source.URL})
		case "開催時間":
			e.Hours = strptr(raw)
		case "開催場所":
			e.Location.Venue = strptr(raw)
			e.venueFromDisplay = true
		case "住所":
			e.Location.Address = strptr(raw)
		case "交通アクセス":
			e.Access = strptr(raw)
		case "予約":
			e.ReservationText = strptr(raw)
			if strings.Contains(raw, "予約必須") {
				b := true
				e.ReservationRequired = &b
			} else if strings.Contains(raw, "予約不要") {
				b := false
				e.ReservationRequired = &b
			}
			e.Evidence = append(e.Evidence, Evidence{Field: "reservation_required", Text: raw, SourceURL: p.source.URL})
		case "料金":
			if !strings.Contains(raw, "こちらよりご確認") {
				applyAdmission(e, raw, p.source.URL)
			}
		case "荒天の場合":
			e.Weather = strptr(raw)
		}
		if key == "開催日" || key == "開催場所" || key == "荒天の場合" {
			applyIndoor(e, raw, p.source.URL)
		}
		if strings.Contains(key, "公式") {
			for _, a := range nodes(td[0], func(n *html.Node) bool { return n.Data == "a" }) {
				href := attr(a, "href")
				if strings.HasPrefix(href, "https://") || strings.HasPrefix(href, "http://") {
					e.OrganizerURLs = appendUnique(e.OrganizerURLs, href)
				}
			}
		}
	}
	// Event-local tags are authoritative; navigation categories are unrelated.
	tags := []CatalogItem{}
	for _, tag := range byClass(doc, "m-detailtag__tag") {
		for _, a := range nodes(tag, func(n *html.Node) bool { return n.Data == "a" }) {
			if m := categoryPathRE.FindStringSubmatch(attr(a, "href")); m != nil {
				code := categoryCode(m[1])
				tags = append(tags, CatalogItem{Code: code, NameJA: nodeText(a), Aliases: []string{}, Path: "/event_list/" + code + "/", Kind: "category"})
			}
		}
	}
	if len(tags) > 0 {
		e.Categories = tags
	}
	links := []string{}
	for _, a := range nodes(doc, func(n *html.Node) bool { return n.Data == "a" }) {
		href := attr(a, "href")
		if href == "/event/"+e.ID+"/data.html" || href == "/event/"+e.ID+"/price.html" {
			links = appendUnique(links, href)
		}
	}
	// Current-event notices only; related listing cards are independent events.
	for _, n := range nodes(doc, func(n *html.Node) bool {
		cl := attr(n, "class")
		candidate := strings.Contains(cl, "notice") || strings.Contains(cl, "caution") || strings.Contains(cl, "period")
		if !candidate {
			return false
		}
		current := strings.Contains(cl, "detail")
		for p := n.Parent; p != nil; p = p.Parent {
			pc := attr(p, "class")
			if strings.Contains(pc, "mainlist") || strings.Contains(pc, "related") {
				return false
			}
			current = current || strings.Contains(pc, "detailheader") || strings.Contains(pc, "detailmain")
		}
		return current
	}) {
		raw := nodeText(n)
		if currentCancellation(raw) {
			e.Cancellation = strptr(raw)
			e.Evidence = append(e.Evidence, Evidence{Field: "cancellation", Text: raw, SourceURL: p.source.URL})
		}
	}
	if currentCancellation(e.TitleJA) {
		e.Cancellation = strptr(e.TitleJA)
		e.Evidence = append(e.Evidence, Evidence{Field: "cancellation", Text: e.TitleJA, SourceURL: p.source.URL})
	}
	e.Sources = append(e.Sources, p.source)
	setDateCertainty(e)
	return links, nil
}

var priceRE = regexp.MustCompile(`([0-9][0-9,]*)円`)

func currentCancellation(raw string) bool {
	if strings.Contains(raw, "の場合") || strings.Contains(raw, "時は") || strings.Contains(raw, "際は") || strings.Contains(raw, "雨天中止") || strings.Contains(raw, "荒天中止") || strings.Contains(raw, "強風中止") {
		return false
	}
	return strings.Contains(raw, "開催中止") || strings.Contains(raw, "開催延期") || strings.Contains(raw, "中止にな")
}

func unqualifiedPrice(raw string) bool {
	clean := strings.NewReplacer("有料。", "", "有料．", "", "入場料", "", "入館料", "", "参加費", "", "入場", "", "入館", "").Replace(raw)
	clean = strings.Trim(clean, " 。．：: ")
	return priceRE.FindString(clean) == clean
}

func applyAdmission(e *Event, raw, source string) {
	e.Admission.Raw = strptr(raw)
	e.Admission.Status = "unknown"
	e.Admission.Price = nil
	if freeEntryWithOptionalPurchases(raw) {
		e.Admission.Status = "free"
		n := float64(0)
		e.Admission.Price = &n
		e.Admission.Currency = strptr("JPY")
	} else if (strings.HasPrefix(raw, "入場無料") || strings.HasPrefix(raw, "無料")) && strings.Contains(raw, "有料") {
		e.Admission.Status = "mixed"
	} else if strings.Contains(raw, "有料") || priceRE.MatchString(raw) {
		e.Admission.Status = "paid"
		if matches := priceRE.FindAllStringSubmatch(raw, -1); len(matches) == 1 && unqualifiedPrice(raw) {
			n, _ := strconv.ParseFloat(strings.ReplaceAll(matches[0][1], ",", ""), 64)
			e.Admission.Price = &n
			if n == 0 && !strings.Contains(raw, "有料") {
				conditional := false
				for _, term := range []string{"小学生", "未就学", "子供", "子ども", "幼児", "駐車", "障がい", "会員"} {
					conditional = conditional || strings.Contains(raw, term)
				}
				if conditional {
					e.Admission.Status = "unknown"
					e.Admission.Price = nil
				} else {
					e.Admission.Status = "free"
				}
			}
		}
		e.Admission.Currency = strptr("JPY")
		if matches := priceRE.FindAllStringSubmatch(raw, -1); !strings.Contains(raw, "有料") && len(matches) == 1 && matches[0][1] == "0" && !unqualifiedPrice(raw) {
			e.Admission.Status = "unknown"
		}
		if strings.Contains(raw, "割引") && !strings.Contains(raw, "有料") {
			e.Admission.Status = "unknown"
			e.Admission.Price = nil
		}
	} else if raw == "無料" || strings.HasPrefix(raw, "無料。") || strings.HasPrefix(raw, "入場無料") || strings.HasPrefix(raw, "入館無料") || strings.HasPrefix(raw, "観覧無料") || strings.HasPrefix(raw, "参加無料") {
		e.Admission.Status = "free"
		n := float64(0)
		e.Admission.Price = &n
		e.Admission.Currency = strptr("JPY")
	}
	if e.Admission.Status == "free" && (strings.Contains(raw, "一部有料") || strings.Contains(raw, "有料エリア") || strings.Contains(raw, "別途")) {
		e.Admission.Status = "mixed"
		e.Admission.Price = nil
	}
	e.Evidence = append(e.Evidence, Evidence{Field: "admission", Text: raw, SourceURL: source})
}

var optionalPurchasePaidRE = regexp.MustCompile("(?:購入|飲食|商品|物販|フード|ドリンク|グッズ)(?:代|費|料金)?(?:は|が)?有料")

func freeEntryWithOptionalPurchases(raw string) bool {
	if !strings.HasPrefix(raw, "入場無料") {
		return false
	}
	// Paid entry/venue fees, paid areas and restricted free entry stay guarded.
	for _, term := range []string{"入場料", "入園料", "入館料", "入山料", "拝観料", "施設利用料", "会場費", "参加費", "入場有料", "有料入場", "有料エリア", "エリア有料", "有料区域", "一部有料", "別途", "購入必須", "購入が必要", "購入が条件", "ワンドリンク", "オーダー制", "チケット", "小学生", "未就学", "子供", "子ども", "幼児", "駐車", "会員", "限定"} {
		if strings.Contains(raw, term) {
			return false
		}
	}
	// Every paid statement must explicitly refer to optional purchases/food.
	paid := strings.Count(raw, "有料")
	if paid == 0 || len(optionalPurchasePaidRE.FindAllStringIndex(raw, -1)) != paid {
		return false
	}
	for _, clause := range strings.FieldsFunc(raw, func(r rune) bool { return r == '。' || r == '、' || r == ';' || r == '；' }) {
		if priceRE.MatchString(clause) && !optionalPurchasePaidRE.MatchString(clause) {
			return false
		}
	}
	return true
}
func applyIndoor(e *Event, raw, source string) {
	if !strings.Contains(raw, "屋内") && !strings.Contains(raw, "室内") {
		return
	}
	conditional := strings.Contains(raw, "雨天時") || strings.Contains(raw, "雨天の場合") || strings.Contains(raw, "雨の場合") || strings.Contains(raw, "雨天は") || strings.Contains(raw, "雨天のみ") || strings.Contains(raw, "雨天に限") || strings.Contains(raw, "悪天候時") || strings.Contains(raw, "屋外")
	if conditional {
		e.Indoor = nil
		e.indoorBlocked = true
		return
	}
	if !e.indoorBlocked && (strings.Contains(raw, "屋内会場") || strings.Contains(raw, "屋内開催") || strings.Contains(raw, "屋内で開催") || strings.Contains(raw, "室内会場") || strings.Contains(raw, "室内開催")) {
		b := true
		e.Indoor = &b
		e.Evidence = append(e.Evidence, Evidence{Field: "indoor", Text: raw, SourceURL: source})
	}
}
