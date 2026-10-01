package omakase

import (
	"fmt"
	"golang.org/x/net/html"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var idRE = regexp.MustCompile(`^[a-z]{2}[0-9]{6}$`)
var cardRE = regexp.MustCompile(`^/(?:en/|ja/)?r/([a-z]{2}[0-9]{6})$`)
var amountRE = regexp.MustCompile(`(?:JPY\s*|[¥￥]\s*)([0-9][0-9,]*)|([0-9][0-9,]*)\s*(?:円|JPY|yen)`)
var percentRE = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*[%％]`)

func ValidateID(s string) error {
	if !idRE.MatchString(s) {
		return fmt.Errorf("restaurant ID must be a source slug such as hc778124")
	}
	return nil
}
func LocalePath(locale string) (string, error) {
	switch locale {
	case "en":
		return "/en", nil
	case "ja":
		return "", nil
	default:
		return "", fmt.Errorf("--lang must be en or ja")
	}
}
func strptr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func class(n *html.Node, key string) bool {
	for _, s := range strings.Fields(attr(n, "class")) {
		if s == key {
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
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, f)
	}
}
func first(n *html.Node, p func(*html.Node) bool) *html.Node {
	var out *html.Node
	walk(n, func(v *html.Node) {
		if out == nil && p(v) {
			out = v
		}
	})
	return out
}
func nodes(n *html.Node, p func(*html.Node) bool) []*html.Node {
	out := []*html.Node{}
	walk(n, func(v *html.Node) {
		if p(v) {
			out = append(out, v)
		}
	})
	return out
}
func byclass(n *html.Node, s string) *html.Node {
	return first(n, func(v *html.Node) bool { return class(v, s) })
}
func text(n *html.Node) string {
	var b strings.Builder
	var read func(*html.Node)
	read = func(v *html.Node) {
		if v == nil {
			return
		}
		if v.Type == html.ElementNode {
			switch v.Data {
			case "script", "style", "iframe":
				return
			case "br":
				b.WriteByte('\n')
			}
		}
		if v.Type == html.TextNode {
			b.WriteString(v.Data)
		}
		for c := v.FirstChild; c != nil; c = c.NextSibling {
			read(c)
		}
		if v.Type == html.ElementNode {
			switch v.Data {
			case "p", "div", "tr", "h1", "h2", "h3", "li":
				b.WriteByte('\n')
			}
		}
	}
	read(n)
	lines := []string{}
	for _, s := range strings.Split(b.String(), "\n") {
		s = strings.Join(strings.Fields(s), " ")
		if s != "" {
			lines = append(lines, s)
		}
	}
	return strings.Join(lines, "\n")
}
func one(n *html.Node) string { return strings.Join(strings.Fields(text(n)), " ") }
func doc(body []byte) (*html.Node, error) {
	s := strings.ToLower(string(body))
	for _, bad := range []string{"<title>just a moment", "<title>attention required", "cf-error-details", "cf-chl-widget"} {
		if strings.Contains(s, bad) {
			return nil, fmt.Errorf("OMAKASE returned a protection page; retry later in the canonical website")
		}
	}
	return html.Parse(strings.NewReader(string(body)))
}
func tableRows(n *html.Node) map[string]string {
	out := map[string]string{}
	walk(n, func(v *html.Node) {
		if v.Type != html.ElementNode || v.Data != "tr" {
			return
		}
		cells := []*html.Node{}
		for c := v.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
				cells = append(cells, c)
			}
		}
		if len(cells) == 2 {
			out[one(cells[0])] = text(cells[1])
		}
	})
	return out
}
func lookup(rows map[string]string, keys ...string) *string {
	for _, k := range keys {
		if v := rows[k]; v != "" {
			return strptr(v)
		}
	}
	return nil
}
func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func ParsePage(body []byte, locale string) (Page, error) {
	root, e := doc(body)
	if e != nil {
		return Page{}, e
	}
	p := Page{Results: []Summary{}, Filters: map[string][]Option{}}
	seen := map[string]bool{}
	for _, a := range nodes(root, func(v *html.Node) bool { return v.Data == "a" }) {
		m := cardRE.FindStringSubmatch(attr(a, "href"))
		if m == nil || seen[m[1]] {
			continue
		}
		h := first(a, func(v *html.Node) bool { return v.Data == "h3" })
		if h == nil {
			continue
		}
		name := one(h)
		if name == "" {
			continue
		}
		seen[m[1]] = true
		row := Summary{ID: m[1], Name: name, URL: Origin + "/en/r/" + m[1], SeatState: "unknown"}
		if locale == "ja" {
			row.NameJA = &name
			row.URL = Origin + "/r/" + m[1]
		}
		detail := byclass(a, "c-restaurant_item_detail")
		sp := first(detail, func(v *html.Node) bool { return v.Data == "span" })
		parts := strings.SplitN(one(sp), "/", 2)
		if len(parts) == 2 {
			row.Cuisine = strptr(parts[0])
			row.Area = strptr(parts[1])
		}
		p.Results = append(p.Results, row)
	}
	for _, s := range nodes(root, func(v *html.Node) bool { return v.Data == "select" }) {
		key := attr(s, "name")
		if key != "area" && key != "cuisine" {
			continue
		}
		opts := []Option{}
		for _, o := range nodes(s, func(v *html.Node) bool { return v.Data == "option" }) {
			if val := attr(o, "value"); val != "" {
				opts = append(opts, Option{val, one(o)})
			}
		}
		p.Filters[key] = opts
	}
	for _, a := range nodes(root, func(v *html.Node) bool { return v.Data == "a" && attr(v, "rel") == "next" }) {
		u := attr(a, "href")
		if strings.HasPrefix(u, "/en/r/page/") || strings.HasPrefix(u, "/r/page/") {
			p.Next = strptr(Origin + u)
		}
	}
	totalRE := regexp.MustCompile(`(?:Search result|検索結果)\s*[0-9,]+\s*[-〜~]\s*[0-9,]+\s*/\s*([0-9,]+)`)
	if m := totalRE.FindStringSubmatch(one(root)); m != nil {
		v, _ := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
		p.Total = &v
	}
	if len(p.Filters["area"]) == 0 || len(p.Filters["cuisine"]) == 0 {
		return Page{}, fmt.Errorf("OMAKASE catalogue structure missing (login, error, or source changed)")
	}
	if len(p.Results) == 0 && (p.Total == nil || *p.Total != 0) {
		t := strings.ToLower(one(root))
		if !strings.Contains(t, "no restaurants") && !strings.Contains(t, "見つかりません") {
			return Page{}, fmt.Errorf("OMAKASE returned no recognizable restaurant cards")
		}
	}
	return p, nil
}

func ParsePrice(raw string) Price {
	p := Price{Currency: "JPY", Basis: "unknown", Raw: raw}
	m := amountRE.FindStringSubmatch(raw)
	if m != nil {
		s := m[1]
		if s == "" {
			s = m[2]
		}
		v, _ := strconv.Atoi(strings.ReplaceAll(s, ",", ""))
		p.Amount = &v
	}
	matches := amountRE.FindAllStringSubmatch(raw, -1)
	if len(matches) == 2 && (strings.Contains(raw, " - ") || strings.Contains(raw, "～") || strings.Contains(raw, "〜")) {
		last := matches[1][1]
		if last == "" {
			last = matches[1][2]
		}
		v, err := strconv.Atoi(strings.ReplaceAll(last, ",", ""))
		if err == nil {
			p.MaximumAmount = &v
		}
	}
	low := strings.ToLower(raw)
	if strings.Contains(low, "guest") || strings.Contains(raw, "名") || strings.Contains(raw, "人") {
		p.Basis = "per_guest"
	}
	if strings.Contains(low, "tax included") || strings.Contains(low, "tax and service charge included") || strings.Contains(raw, "税込") || strings.Contains(raw, "税サ込") || strings.Contains(raw, "税・サ込") {
		v := true
		p.TaxIncluded = &v
	}
	if strings.Contains(low, "tax excluded") || strings.Contains(low, "tax and service charge excluded") || strings.Contains(raw, "税抜") {
		v := false
		p.TaxIncluded = &v
	}
	if strings.Contains(low, "service charge included") || strings.Contains(raw, "サービス料込") || strings.Contains(raw, "税サ込") || strings.Contains(raw, "税・サ込") {
		v := true
		p.ServiceChargeIncluded = &v
	}
	if strings.Contains(low, "service charge excluded") {
		v := false
		p.ServiceChargeIncluded = &v
	}
	p.Minimum = p.MaximumAmount != nil || strings.Contains(raw, "～") || strings.Contains(raw, "〜") || strings.Contains(low, "from ")
	p.Variable = p.Minimum || p.MaximumAmount != nil || strings.Contains(low, "market") || strings.Contains(low, "vary") || strings.Contains(raw, "時価")
	return p
}
func percentage(s string) *float64 {
	m := percentRE.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	v, e := strconv.ParseFloat(m[1], 64)
	if e != nil {
		return nil
	}
	return &v
}

func ParseRelease(current, next, frequency *string) Release {
	r := Release{State: "unknown", CurrentPeriod: current, NextRoundRaw: next, Timezone: "Asia/Tokyo", MaximumFrequency: frequency}
	n := deref(next)
	if n != "" {
		low := strings.ToLower(n)
		if strings.Contains(low, "tbd") || strings.Contains(low, "undecided") || strings.Contains(n, "未定") {
			r.State = "undetermined"
		} else if strings.Contains(low, "irregular") || strings.Contains(n, "不定期") {
			r.State = "irregular"
		} else {
			r.State = "schedule_text"
		}
	}
	clean := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(n, "JST(UTC+9)", ""), "JST (UTC+9)", ""))
	clean = strings.Join(strings.Fields(clean), " ")
	jst := time.FixedZone("JST", 9*3600)
	for _, layout := range []string{"Jan 2, 2006, 15:04", "January 2, 2006, 15:04", "2006年1月2日 15:04", "2006/1/2 15:04", "2006-01-02 15:04"} {
		if d, e := time.ParseInLocation(layout, clean, jst); e == nil {
			v := d.Format(time.RFC3339)
			r.NextRoundAt = &v
			r.State = "scheduled"
			break
		}
	}
	if r.State == "unknown" && (strings.Contains(strings.ToLower(deref(current)), "irregular") || strings.Contains(deref(current), "不定期")) {
		r.State = "irregular"
	}
	return r
}

// ActionState uses only the reservation action, not descriptive restaurant rules.
func ActionState(raw string) (method, access string) {
	s := strings.ToLower(raw)
	if strings.Contains(s, "log in to check availability") || strings.Contains(raw, "空席状況を確認するには") || strings.Contains(raw, "ログインして空き枠") {
		return "unknown", "login_required"
	}
	if strings.Contains(s, "waitlist") || strings.Contains(raw, "キャンセル待ち") {
		return "waitlist", "unknown"
	}
	if strings.Contains(s, "request reservation") || strings.Contains(s, "reservation request") || strings.Contains(raw, "予約リクエスト") {
		return "request", "unknown"
	}
	if strings.Contains(s, "lottery") || strings.Contains(s, "raffle") || strings.Contains(raw, "抽選") {
		return "lottery", "unknown"
	}
	return "unknown", "unknown"
}

func ParseDetail(body []byte, id, locale string) (Detail, error) {
	if e := ValidateID(id); e != nil {
		return Detail{}, e
	}
	root, e := doc(body)
	if e != nil {
		return Detail{}, e
	}
	h := first(root, func(v *html.Node) bool { return v.Data == "h1" && class(v, "p-r_title") })
	if h == nil || one(h) == "" {
		return Detail{}, fmt.Errorf("OMAKASE restaurant structure missing (login, error, or source changed)")
	}
	d := Detail{Summary: Summary{ID: id, Name: one(h), URL: Origin + "/en/r/" + id, SeatState: "unknown"}, Courses: []Course{}, Cancellation: []Cancellation{}, Information: map[string]string{}, Evidence: []Evidence{}, Warnings: []string{}}
	if locale == "ja" {
		d.NameJA = strptr(d.Name)
		d.URL = Origin + "/r/" + id
	}
	info := byclass(root, "p-r_main_subInfo")
	spans := nodes(info, func(v *html.Node) bool { return v.Data == "span" })
	if len(spans) > 0 {
		d.Location = strptr(one(spans[0]))
	}
	if len(spans) > 1 {
		d.Cuisine = strptr(one(spans[1]))
	}
	d.ReservationRules = strptr(text(byclass(root, "p-r_text")))
	for _, upper := range nodes(root, func(v *html.Node) bool { return class(v, "p-r_course_upper") }) {
		h := first(upper, func(v *html.Node) bool { return v.Data == "h3" })
		raw := one(byclass(upper, "p-r_course_price"))
		if h != nil {
			d.Courses = append(d.Courses, Course{Name: one(h), Price: ParsePrice(raw)})
		}
	}
	d.CourseNotes = strptr(text(byclass(root, "p-r_course_notice")))
	notes := deref(d.CourseNotes)
	d.ServiceCharge = Charge{}
	if strings.Contains(strings.ToLower(notes), "service") || strings.Contains(notes, "サービス") {
		for _, line := range strings.Split(notes, "\n") {
			if strings.Contains(strings.ToLower(line), "service") || strings.Contains(line, "サービス") {
				d.ServiceCharge = Charge{percentage(line), strptr(line)}
				break
			}
		}
	}
	for i := range d.Courses {
		if strings.Contains(strings.ToLower(notes), "vary") || strings.Contains(notes, "変動") || strings.Contains(notes, "時価") || (strings.Contains(notes, "上下") && (strings.Contains(notes, "お値段") || strings.Contains(notes, "価格"))) {
			d.Courses[i].Price.Variable = true
		}
	}
	d.ReservationFee = Fee{Currency: "JPY", Basis: "unknown"}
	reserve := byclass(root, "p-r_reserve")
	rows := tableRows(reserve)
	d.Release = ParseRelease(lookup(rows, "Current reservation period", "現在の予約受付期間", "予約期間"), lookup(rows, "Next round opens", "次回枠の受付開始日時", "次回予約開始"), lookup(rows, "Maximum frequency", "最大予約頻度", "予約頻度"))
	action := byclass(reserve, "p-r_reserve_action_reserve")
	d.ActionRaw = strptr(one(action))
	d.BookingMethod, d.Access = ActionState(deref(d.ActionRaw))
	if d.Access == "unknown" {
		for _, a := range nodes(action, func(v *html.Node) bool { return v.Data == "a" }) {
			if strings.Contains(attr(a, "href"), "/users/sign_in") {
				d.Access = "login_required"
			}
		}
	}
	for _, n := range nodes(reserve, func(v *html.Node) bool { return class(v, "rsv_notice") }) {
		raw := one(n)
		if strings.Contains(strings.ToLower(raw), "reservation fee") || strings.Contains(raw, "予約手数料") || (strings.Contains(raw, "一席") && strings.Contains(raw, "手数料")) {
			p := ParsePrice(raw)
			d.ReservationFee.Amount = p.Amount
			d.ReservationFee.Raw = strptr(raw)
			if strings.Contains(raw, "/seat") || strings.Contains(raw, "一席") {
				d.ReservationFee.Basis = "per_seat"
			}
		}
	}
	for _, table := range nodes(reserve, func(v *html.Node) bool { return v.Data == "table" }) {
		heading := one(first(table, func(v *html.Node) bool { return v.Data == "thead" }))
		if strings.Contains(strings.ToLower(heading), "cancellation") || strings.Contains(heading, "キャンセル") {
			walk(table, func(v *html.Node) {
				if v.Data != "tr" {
					return
				}
				cells := nodes(v, func(n *html.Node) bool { return n.Data == "td" })
				if len(cells) == 2 {
					d.Cancellation = append(d.Cancellation, Cancellation{one(cells[0]), percentage(one(cells[1]))})
				}
			})
		}
	}
	d.CancellationNote = strptr(text(byclass(reserve, "p-r_reserve_cpDesc")))
	d.Information = tableRows(byclass(root, "p-r-list"))
	// Region from visible location is distinct from provider's broad search area.
	if d.Location != nil {
		parts := strings.Split(*d.Location, ",")
		if len(parts) > 1 {
			d.Area = strptr(parts[len(parts)-1])
		}
	}
	if len(d.Courses) == 0 {
		d.Warnings = append(d.Warnings, "No public courses listed")
	}
	if d.Access == "login_required" {
		d.Warnings = append(d.Warnings, "Exact seats require login; public release schedule is not availability")
	}
	return d, nil
}

func ParseMembership(body []byte) (Membership, error) {
	root, e := doc(body)
	if e != nil {
		return Membership{}, e
	}
	t := one(root)
	if !strings.Contains(t, "Premium") || !strings.Contains(t, "4,980") {
		return Membership{}, fmt.Errorf("OMAKASE Premium page structure changed")
	}
	m := Membership{URL: Origin + "/en/premium/green", GoldEligibility: "unknown", Features: []string{}, Sections: map[string]string{}}
	if strings.Contains(strings.ToLower(t), "invitation") {
		m.GoldEligibility = "invitation_only"
	}
	// Extract the published price plans, keeping values unknown if the exact source terms disappear.
	if regexp.MustCompile(`4,980\s*Yen\(tax inc\.\)/mo`).MatchString(t) {
		v := 4980
		m.MonthlyJPY = &v
	}
	if regexp.MustCompile(`49,980\s*Yen\(tax inc\.\)/yr`).MatchString(t) {
		v := 49980
		m.AnnualJPY = &v
	}
	for _, h := range nodes(root, func(v *html.Node) bool { return v.Data == "h3" }) {
		title := one(h)
		if title == "" {
			continue
		}
		section := title
		for n := h.NextSibling; n != nil; n = n.NextSibling {
			if n.Data == "h2" || n.Data == "h3" {
				break
			}
			s := text(n)
			if s != "" {
				section += "\n" + s
			}
			if len(section) > 1600 {
				break
			}
		}
		m.Sections[title] = section
		if len(m.Features) < 12 {
			m.Features = append(m.Features, title)
		}
	}
	return m, nil
}

// NameMatches is a literal, case-insensitive name substring check. It prevents
// upstream fuzzy search from presenting unrelated restaurants as exact matches.
func NameMatches(name, query string) bool {
	fold := func(s string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return -1
			}
			return unicode.ToLower(r)
		}, s)
	}
	return strings.Contains(fold(name), fold(query))
}
