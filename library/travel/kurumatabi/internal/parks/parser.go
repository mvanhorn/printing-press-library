package parks

import (
	"bytes"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/cliutil"
	"golang.org/x/net/html"
	"golang.org/x/text/unicode/norm"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var idRE = regexp.MustCompile(`^([a-z][a-z0-9_-]*)/([0-9]+)$`)
var linkRE = regexp.MustCompile(`^/park/([a-z][a-z0-9_-]*)/([0-9]+)\.html$`)
var countRE = regexp.MustCompile(`件数\s*([0-9,]+)件`)
var amountRE = regexp.MustCompile(`([0-9][0-9,]*(?:\.[0-9]+)?)\s*円`)
var dateRE = regexp.MustCompile(`掲載内容は\s*([0-9]{4})年([0-9]{1,2})月([0-9]{1,2})日`)

func NormalizeID(value string) (string, error) {
	if strings.Contains(value, "://") {
		u, e := url.Parse(value)
		if e != nil || u.Scheme != "https" || u.Host != "www.kurumatabi.com" || u.RawQuery != "" || u.Fragment != "" {
			return "", fmt.Errorf("use a canonical https://www.kurumatabi.com/park/kind/id.html URL or kind/id")
		}
		m := linkRE.FindStringSubmatch(u.Path)
		if m == nil {
			return "", fmt.Errorf("invalid canonical park URL")
		}
		value = m[1] + "/" + m[2]
	}
	if !idRE.MatchString(value) {
		return "", fmt.Errorf("park ID must be kind/numeric-id, for example rvpark/1086")
	}
	return value, nil
}
func walk(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}
func attr(n *html.Node, k string) string {
	for _, a := range n.Attr {
		if a.Key == k {
			return a.Val
		}
	}
	return ""
}
func hasClass(n *html.Node, c string) bool {
	for _, v := range strings.Fields(attr(n, "class")) {
		if v == c {
			return true
		}
	}
	return false
}
func all(n *html.Node, p func(*html.Node) bool) []*html.Node {
	out := []*html.Node{}
	walk(n, func(v *html.Node) {
		if p(v) {
			out = append(out, v)
		}
	})
	return out
}
func first(n *html.Node, p func(*html.Node) bool) *html.Node {
	v := all(n, p)
	if len(v) > 0 {
		return v[0]
	}
	return nil
}
func tag(n *html.Node, t string) bool { return n.Type == html.ElementNode && n.Data == t }
func textOf(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	walk(n, func(v *html.Node) {
		if v.Type == html.TextNode {
			b.WriteString(v.Data)
			b.WriteByte(' ')
		}
	})
	return strings.Join(strings.Fields(cliutil.CleanText(b.String())), " ")
}
func short(s string) string {
	r := []rune(s)
	if len(r) > 4000 {
		return string(r[:4000]) + " [truncated]"
	}
	return s
}
func addUnique(xs []string, s string) []string {
	if s == "" {
		return xs
	}
	for _, x := range xs {
		if x == s {
			return xs
		}
	}
	return append(xs, s)
}
func ParseDimensions(raw string) Dimensions {
	d := Dimensions{Raw: raw, Scope: "published_parking_space"}
	v := norm.NFKC.String(raw)
	for label, dst := range map[string]**float64{"長さ": &d.LengthM, "幅": &d.WidthM, "高さ": &d.HeightM} {
		r := regexp.MustCompile(regexp.QuoteMeta(label) + `\s*([0-9]+(?:\.[0-9]+)?)\s*(cm|m)(?:\s|×|$)`)
		m := r.FindStringSubmatch(v)
		if m != nil {
			n, e := strconv.ParseFloat(m[1], 64)
			if e == nil && n > 0 {
				if m[2] == "cm" {
					n /= 100
				}
				*dst = &n
			}
		}
	}
	d.HeightUnrestricted = regexp.MustCompile(`高さ\s*無制限`).MatchString(v)
	return d
}
func coordinates(n *html.Node) *Coordinates {
	for _, a := range all(n, func(v *html.Node) bool { return tag(v, "a") }) {
		u, e := url.Parse(attr(a, "href"))
		if e != nil {
			continue
		}
		q := u.Query().Get("q")
		m := regexp.MustCompile(`^(-?[0-9]+(?:\.[0-9]+)?),\s*(-?[0-9]+(?:\.[0-9]+)?)`).FindStringSubmatch(q)
		if m == nil {
			continue
		}
		lat, e1 := strconv.ParseFloat(m[1], 64)
		lon, e2 := strconv.ParseFloat(m[2], 64)
		if e1 == nil && e2 == nil && lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180 {
			return &Coordinates{lat, lon}
		}
	}
	return nil
}
func facilityValue(raw string) Facility {
	v := norm.NFKC.String(raw)
	f := Facility{Status: "unknown", Fee: "unknown", Evidence: []string{short(raw)}}
	switch {
	case strings.HasPrefix(v, "なし"), strings.HasPrefix(v, "不可"), strings.Contains(v, "・なし"), strings.Contains(v, "・不可"):
		f.Status = "no"
	case strings.HasPrefix(v, "あり"), strings.HasPrefix(v, "可"), strings.Contains(v, "OK"):
		f.Status = "yes"
	}
	if f.Status == "yes" {
		f.Fee = feeValue(v)
	}
	return f
}
func iconKey(src, alt string) string {
	switch {
	case strings.Contains(src, "toilet_24") || strings.Contains(src, "facility_toilet"):
		return "toilet_24h"
	case strings.Contains(src, "facility_ac"):
		return "electricity"
	case strings.Contains(src, "suidou") || strings.Contains(src, "facility_water"):
		return "water"
	case strings.Contains(src, "dump"):
		return "dump_station"
	case strings.Contains(src, "refuse"):
		return "garbage"
	case strings.Contains(src, "coinrandory") || strings.Contains(src, "laundry"):
		return "laundry"
	case strings.Contains(src, "generator"):
		return "generator"
	case strings.Contains(src, "facility_premium"):
		return "premium_benefits"
	case strings.Contains(src, "facility_pet"):
		return "pets"
	case strings.Contains(src, "bath_shower") || strings.Contains(src, "bath_shawer") || strings.Contains(alt, "シャワー"):
		return "shower"
	case strings.Contains(src, "facility_hotspring") || strings.Contains(src, "bath_hotspring"):
		return "bath"
	case strings.Contains(src, "bath_") && !strings.Contains(src, "hotspring"):
		return "bath"
	case strings.Contains(src, "kitchen"):
		return "kitchen"
	case strings.Contains(src, "dogrun"):
		return "dog_run"
	case strings.Contains(src, "wifi"):
		return "wifi"
	}
	return ""
}
func parseIcons(p *Park, n *html.Node) {
	for _, im := range all(n, func(v *html.Node) bool { return tag(v, "img") && strings.Contains(attr(v, "src"), "/images/ico/") }) {
		src, alt := attr(im, "src"), attr(im, "alt")
		disabled := strings.Contains(src, "_off.")
		p.Icons = append(p.Icons, Icon{alt, src, disabled})
		key := iconKey(src, alt)
		if key == "" {
			continue
		}
		f := facilityValue(alt)
		f.Evidence = []string{alt + " [icon=" + src + "]"}
		if disabled || strings.Contains(src, "toilet_24no") || strings.Contains(alt, "利用時間制限") || strings.Contains(alt, "なし") || strings.Contains(alt, "不可") {
			f.Status = "no"
			f.Fee = "unknown"
		} else if strings.Contains(alt, "現地相談") {
			f.Status = "unknown"
		} else if f.Status == "unknown" {
			f.Status = "yes"
		}
		if disabled && !strings.Contains(alt, "なし") && !strings.Contains(alt, "不可") {
			p.Warnings = addUnique(p.Warnings, "legacy_icon_off_overrides_positive_alt: "+alt)
		}
		if f.Status != "no" {
			f.Fee = feeValue(alt)
		}
		previous := p.Facilities[key]
		if len(previous.Evidence) > 0 {
			for _, evidence := range f.Evidence {
				previous.Evidence = addUnique(previous.Evidence, evidence)
			}
			if previous.Status != f.Status {
				if key == "bath" && (previous.Status == "yes" || f.Status == "yes") {
					// Indoor baths and onsen are narrower observations of the
					// generic bath category; one explicit positive is sufficient.
					previous.Status = "yes"
				} else {
					previous.Status = "unknown"
					p.Warnings = addUnique(p.Warnings, "conflicting_icon_evidence: "+key)
				}
			}
			if previous.Fee != f.Fee {
				previous.Fee = "unknown"
			}
			f = previous
		}
		p.Facilities[key] = f
		if key == "dump_station" && !disabled {
			for label, k := range map[string]string{"ブラック": "black_water", "グレー": "grey_water"} {
				if status := wastePermission(alt, label); status != "unknown" {
					p.Facilities[k] = Facility{Status: status, Fee: "unknown", Evidence: []string{alt}}
				}
			}
		}

	}
}
func parseVehicles(n *html.Node, class string) []string {
	out := []string{}
	ul := first(n, func(v *html.Node) bool { return tag(v, "ul") && hasClass(v, class) })
	if ul == nil {
		return out
	}
	for _, li := range all(ul, func(v *html.Node) bool { return tag(v, "li") }) {
		if hasClass(li, "off") || hasClass(li, "disabled") {
			continue
		}
		out = addUnique(out, textOf(li))
	}
	return out
}
func parseLinks(p *Park, n *html.Node) {
	for _, a := range all(n, func(v *html.Node) bool { return tag(v, "a") }) {
		v := attr(a, "href")
		switch {
		case strings.HasPrefix(v, "tel:"):
			if !strings.Contains(v, "000-0000-0000") {
				p.Booking.Phones = addUnique(p.Booking.Phones, strings.TrimPrefix(v, "tel:"))
			}
		case strings.HasPrefix(v, "mailto:"):
			p.Booking.Emails = addUnique(p.Booking.Emails, strings.TrimPrefix(v, "mailto:"))
		case strings.Contains(textOf(a), "ネット予約"):
			u, e := url.Parse(v)
			if e == nil && (u.Scheme == "https" || u.Scheme == "http") {
				p.Booking.URLs = addUnique(p.Booking.URLs, v)
			}
		}
	}
}
func ParseSearch(body []byte, now time.Time) (SearchResult, error) {
	doc, e := html.Parse(bytes.NewReader(body))
	if e != nil {
		return SearchResult{}, e
	}
	t := textOf(doc)
	m := countRE.FindStringSubmatch(t)
	if m == nil {
		return SearchResult{}, fmt.Errorf("search markup changed or provider returned a shell: no result-count evidence")
	}
	total, _ := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
	out := SearchResult{Meta: Meta{Source: "live", SourceURL: Origin + "/park/search.php", ObservedAt: now.In(JST).Format(time.RFC3339), ProviderTotal: &total, ScannedPages: 1, ProviderPagesComplete: !strings.Contains(t, "次へ")}, Results: []Park{}}
	for _, card := range all(doc, func(n *html.Node) bool { return tag(n, "div") && hasClass(n, "numBoxs") }) {
		a := first(card, func(n *html.Node) bool { return tag(n, "a") && linkRE.MatchString(attr(n, "href")) })
		if a == nil {
			continue
		}
		mm := linkRE.FindStringSubmatch(attr(a, "href"))
		p := newPark(mm[1]+"/"+mm[2], now)
		p.Name = textOf(a)
		p.Type = mm[1]
		p.SourceLevel = "search_card"
		p.TypeLabel = textOf(first(card, func(n *html.Node) bool { return hasClass(n, "label") }))
		ct := textOf(card)
		adr := regexp.MustCompile(`【所在地】\s*(.*?)\s*【TEL】`).FindStringSubmatch(ct)
		if adr != nil {
			p.Address = adr[1]
		}
		p.Coordinates = coordinates(card)
		p.Vehicles = parseVehicles(card, "cartype")
		p.Dimensions = ParseDimensions(textOf(first(card, func(n *html.Node) bool { return tag(n, "p") && strings.Contains(textOf(n), "駐車可能サイズ") })))
		parseIcons(&p, card)
		if first(card, func(n *html.Node) bool { return hasClass(n, "labelpre") }) != nil {
			p.Facilities["premium_benefits"] = Facility{Status: "yes", Fee: "unknown", Evidence: []string{"プレミアム会員特典 badge"}}
		}
		parseLinks(&p, card)
		if strings.Contains(ct, "当日空き予約可") {
			yes := true
			p.Booking.SameDayAccepted = &yes
		}
		out.Results = append(out.Results, p)
	}
	if total > 0 && len(out.Results) == 0 {
		return SearchResult{}, fmt.Errorf("search markup changed: provider reports %d records but no park cards parsed", total)
	}
	out.Meta.ScannedRecords = len(out.Results)
	out.Meta.ReturnedRecords = len(out.Results)
	return out, nil
}
func ParseDetail(id string, body []byte, now time.Time) (Park, error) {
	id, e := NormalizeID(id)
	if e != nil {
		return Park{}, e
	}
	doc, e := html.Parse(bytes.NewReader(body))
	if e != nil {
		return Park{}, e
	}
	p := newPark(id, now)
	p.Type = strings.Split(id, "/")[0]
	p.SourceLevel = "detail"
	heading := first(doc, func(n *html.Node) bool { return tag(n, "h3") && hasClass(n, "commonIcoTitle") })
	if heading != nil {
		for c := heading.FirstChild; c != nil; c = c.NextSibling {
			if !tag(c, "span") {
				continue
			}
			if hasClass(c, "label") {
				p.TypeLabel = textOf(c)
			} else if !hasClass(c, "ico") && p.Name == "" {
				p.Name = textOf(c)
			}
		}
	}
	if p.Name == "" {
		return Park{}, fmt.Errorf("park %s not found or detail markup changed: no facility name", id)
	}
	for _, dl := range all(doc, func(n *html.Node) bool { return tag(n, "dl") }) {
		dt := first(dl, func(n *html.Node) bool { return tag(n, "dt") })
		dd := first(dl, func(n *html.Node) bool { return tag(n, "dd") })
		label, v := textOf(dt), short(textOf(dd))
		if label == "" || dd == nil {
			continue
		}
		if old := p.Sections[label]; old != "" && v != "" {
			p.Sections[label] = old + " | " + v
		} else {
			p.Sections[label] = v
		}
	}
	p.Address = p.Sections["所在地"]
	p.Coordinates = coordinates(doc)
	p.Vehicles = parseVehicles(doc, "cartypedetail")
	p.AvailabilityPeriods = parseVehicles(doc, "availableperiod")
	p.Dimensions = ParseDimensions(p.Sections["駐車場"])
	parseIcons(&p, firstOrDoc(doc, func(n *html.Node) bool {
		return tag(n, "dd") && strings.Contains(textOf(n), "スタンダード会員特典")
	}))
	// The feature definition list contains the icons; parse all icons only once when no member badges exist.
	if len(p.Icons) == 0 {
		parseIcons(&p, doc)
	}
	mappings := map[string]string{"電源の有無": "electricity", "水道": "water", "ダンプステーション": "dump_station", "ゴミ処理対応": "garbage", "コインランドリー": "laundry", "発電機の使用": "generator", "ペット連れ": "pets", "Wi-Fiの利用": "wifi", "炊事場": "kitchen", "ドッグランの利用": "dog_run"}
	for label, key := range mappings {
		if v := p.Sections[label]; v != "" {
			f := facilityValue(v)
			if f.Status != "unknown" {
				setDetailFacility(&p, key, f)
			} else {
				old := p.Facilities[key]
				old.Evidence = addUnique(old.Evidence, v)
				// A conditional price section cannot preserve an unqualified free icon.
				if strings.Contains(v, "無料") || strings.Contains(v, "有料") || strings.Contains(v, "含まれ") {
					old.Fee = feeValue(v)
				}
				p.Facilities[key] = old
			}
			if key == "dump_station" {
				for label, k := range map[string]string{"ブラック": "black_water", "グレー": "grey_water"} {
					status := wastePermission(v, label)
					if status != "unknown" {
						setDetailFacility(&p, k, Facility{Status: status, Fee: "unknown", Evidence: []string{v}})
					} else if f.Status == "no" && p.Facilities[k].Status == "yes" {
						// No station does not prove that all disposal is forbidden, but
						// it invalidates a positive disposal claim derived only from its icon.
						setDetailFacility(&p, k, Facility{Status: "unknown", Fee: "unknown", Evidence: []string{v}})
					}
				}
			}

		}
	}
	if v := p.Sections["トイレ"]; v != "" {
		f := Facility{Status: toiletHoursStatus(v), Fee: "unknown", Evidence: []string{v}}
		if f.Status == "unknown" && !hasToiletHoursEvidence(v) {
			f.Status = p.Facilities["toilet_24h"].Status
		}
		setDetailFacility(&p, "toilet_24h", f)
	}
	cond := p.Sections["利用条件"]
	p.Membership.Evidence = cond
	p.Membership.Status = classifyMembership(cond)

	for _, dl := range all(doc, func(n *html.Node) bool { return tag(n, "dl") }) {
		if textOf(first(dl, func(n *html.Node) bool { return tag(n, "dt") })) != "利用料金" {
			continue
		}
		dd := first(dl, func(n *html.Node) bool { return tag(n, "dd") })
		feeul := first(dd, func(n *html.Node) bool { return tag(n, "ul") && hasClass(n, "fee") })
		if feeul != nil {
			for _, li := range all(feeul, func(n *html.Node) bool { return tag(n, "li") }) {
				raw := textOf(li)
				if raw == "" {
					continue
				}
				aud := textOf(first(li, func(n *html.Node) bool { return tag(n, "span") }))
				p.Tariffs = append(p.Tariffs, tariff(aud, raw))
			}
		}
		if len(p.Tariffs) == 0 {
			pv := textOf(first(dd, func(n *html.Node) bool { return tag(n, "p") }))
			if pv != "" {
				p.Tariffs = append(p.Tariffs, tariff("unspecified", pv))
			}
		}
	}
	parseLinks(&p, firstOrDoc(doc, func(n *html.Node) bool { return tag(n, "main") }))
	p.Booking.Terms = p.Sections["予約"]
	if strings.Contains(p.Booking.Terms, "当日空き予約可") {
		yes := true
		p.Booking.SameDayAccepted = &yes
	}
	dm := dateRE.FindStringSubmatch(textOf(doc))
	if dm != nil {
		y, _ := strconv.Atoi(dm[1])
		mo, _ := strconv.Atoi(dm[2])
		day, _ := strconv.Atoi(dm[3])
		p.SourceUpdated = fmt.Sprintf("%04d-%02d-%02d", y, mo, day)
	}
	return p, nil
}
func firstOrDoc(doc *html.Node, p func(*html.Node) bool) *html.Node {
	n := first(doc, p)
	if n == nil {
		return doc
	}
	return n
}
func tariff(aud, raw string) Tariff {
	v := Tariff{Audience: aud, Description: raw, Currency: "JPY", Kind: "published_tariff_not_dated_total", Raw: raw}
	m := amountRE.FindStringSubmatch(norm.NFKC.String(raw))
	if m != nil {
		n, e := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", ""), 64)
		if e == nil {
			v.AmountJPY = &n
		}
	} else if strings.HasPrefix(raw, "無料") {
		zero := 0.0
		v.AmountJPY = &zero
	}
	return v
}

// Fee evidence is conservative when the same section qualifies or contradicts a fee.
func feeValue(raw string) string {
	v := norm.NFKC.String(raw)
	paid := strings.Contains(v, "有料")
	free := strings.Contains(v, "無料")
	negativeIncluded := strings.Contains(v, "含まれていません") || strings.Contains(v, "含まれません") || strings.Contains(v, "含まれない") || strings.Contains(v, "含まれていない")
	included := strings.Contains(v, "含まれ") && !negativeIncluded
	negativeFree := strings.Contains(v, "無料では") || strings.Contains(v, "無料でない") || strings.Contains(v, "無料じゃない")
	if free && negativeFree {
		free = false
	}
	if paid && free || paid && included || free && strings.Contains(v, "のみ") {
		return "unknown"
	}
	if included {
		return "included"
	}
	if paid {
		return "paid"
	}
	if free {
		return "free"
	}
	return "unknown"
}
func wastePermission(raw, label string) string {
	v := norm.NFKC.String(raw)
	prefix := regexp.QuoteMeta(label) + `\s*(?:排水|水|処理)?\s*[:：]?\s*`
	if regexp.MustCompile(prefix + `(?:不可|禁止|対応不可|処理不可|なし)`).MatchString(v) {
		return "no"
	}
	if regexp.MustCompile(prefix + `(?:OK|可|可能|対応可|対応可能|のみ)`).MatchString(v) {
		return "yes"
	}
	if regexp.MustCompile(`ブラック[・/、]グレー\s*(?:OK|可|可能)`).MatchString(v) {
		return "yes"
	}
	return "unknown"
}

// Detail restrictions take precedence over an icon, while both remain citable.
func setDetailFacility(p *Park, key string, detail Facility) {
	old := p.Facilities[key]
	if old.Status != "unknown" && old.Status != "" && old.Status != detail.Status {
		p.Warnings = addUnique(p.Warnings, "detail_conflict_overrides_icon: "+key)
	}
	for _, evidence := range old.Evidence {
		detail.Evidence = addUnique(detail.Evidence, evidence)
	}
	p.Facilities[key] = detail
}

var toiletHourWordRE = regexp.MustCompile(`[0-9]{1,2}時`)

var toiletHoursRE = regexp.MustCompile(`([0-9]{1,2}):([0-9]{2})\s*[〜~～–-]\s*([0-9]{1,2}):([0-9]{2})`)

func hasToiletHoursEvidence(raw string) bool {
	v := norm.NFKC.String(raw)
	return toiletHoursRE.MatchString(v) || toiletHourWordRE.MatchString(v) || strings.Contains(v, "24時間") || toiletNegativeEvidence(v) || strings.Contains(v, "時間制限")
}

func toiletHoursStatus(raw string) string {
	v := norm.NFKC.String(raw)
	yes := strings.Contains(v, "24時間利用可") || strings.Contains(v, "24時間利用可能")
	no := toiletNegativeEvidence(v) || strings.Contains(v, "時間制限") && !strings.Contains(v, "時間制限なし") && !strings.Contains(v, "時間制限はありません")
	fullRange := false
	for _, m := range toiletHoursRE.FindAllStringSubmatch(v, -1) {
		a, _ := strconv.Atoi(m[1])
		b, _ := strconv.Atoi(m[2])
		c, _ := strconv.Atoi(m[3])
		d, _ := strconv.Atoi(m[4])
		if a > 23 || b > 59 || c > 24 || d > 59 || c == 24 && d != 0 {
			return "unknown"
		}
		if a == 0 && b == 0 && c == 24 && d == 0 {
			fullRange = true
		} else {
			no = true
		}
	}
	if (yes || fullRange) && no {
		return "unknown"
	}
	if no {
		return "no"
	}
	if yes || fullRange {
		return "yes"
	}
	return "unknown"
}

func toiletNegativeEvidence(v string) bool {
	v = strings.Trim(strings.Join(strings.Fields(norm.NFKC.String(v)), ""), "()")
	if strings.HasPrefix(v, "なし") || strings.HasPrefix(v, "無し") || strings.HasPrefix(v, "ありません") {
		return true
	}
	for _, term := range []string{"利用不可", "使用不可", "利用できません", "利用できない", "利用出来ません", "利用出来ない", "利用可能ではありません", "閉鎖", "トイレはありません", "トイレがありません"} {
		if strings.Contains(v, term) {
			return true
		}
	}
	return false
}

var premiumRequiredRE = regexp.MustCompile(`プレミアム会員(?:証)?(?:の|を)?(?:ご)?(?:提示(?:が|は|を)?)?(?:\()?((?:限定|専用|必須|必要|以上|のみ|に限る))`)

func premiumMembershipRequired(raw string) bool {
	v := strings.Join(strings.Fields(norm.NFKC.String(raw)), "")
	for _, bounds := range premiumRequiredRE.FindAllStringIndex(v, -1) {
		suffix := v[bounds[1]:]
		negated := false
		for _, term := range []string{"では", "じゃ", "でない", "ありません", "不要"} {
			if strings.HasPrefix(suffix, term) {
				negated = true
			}
		}
		if !negated {
			return true
		}
	}
	return false
}

var membershipWaiverRE = regexp.MustCompile(`(?:会員証|会員資格|会員登録)(?:の|を)?(?:提示(?:する)?)?(?:は|が|も)?(?:不要|必要(?:は)?ありません|必要ない|必要ではない|必須では(?:ありません|ない))|会員(?:限定|専用)では(?:ありません|ない)`)

func classifyMembership(raw string) string {
	v := strings.Join(strings.Fields(norm.NFKC.String(raw)), "")
	waiver := membershipWaiverRE.MatchString(v)
	unrestricted := strings.Contains(v, "特になし") || strings.Contains(v, "会員でなく") || strings.Contains(v, "非会員も") || strings.Contains(v, "非会員でも")
	if waiver || unrestricted {
		rest := membershipWaiverRE.ReplaceAllString(v, "")
		// A tier-only waiver says nothing definitive about other admission tiers.
		if waiver && (strings.Contains(v, "プレミアム会員") || strings.Contains(v, "スタンダード会員")) {
			return "unknown"
		}
		for _, qualifier := range []string{"ただし", "ですが", "場合", "会員証", "会員限定", "会員専用", "必須", "必要", "のみ"} {
			if strings.Contains(rest, qualifier) {
				return "unknown"
			}
		}
		return "not_required_stated"
	}
	if strings.Contains(v, "会員証") || strings.Contains(v, "会員限定") || strings.Contains(v, "会員専用") {
		if strings.Contains(v, "割引") && !strings.Contains(v, "限定") && !strings.Contains(v, "専用") {
			return "unknown"
		}
		return "required"
	}
	return "unknown"
}
