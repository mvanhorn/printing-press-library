// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package trip

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/cliutil"
	"golang.org/x/net/html"
	"golang.org/x/text/unicode/norm"
)

var refPattern = regexp.MustCompile(`^(spots|events)/([1-9][0-9]{0,8})$`)
var dayPattern = regexp.MustCompile(`^(\d{4})年(\d{1,2})月(\d{1,2})日(?:[（(][月火水木金土日][）)])?$`)
var rangePattern = regexp.MustCompile(`^(\d{4})年(\d{1,2})月(\d{1,2})日(?:[（(][月火水木金土日][）)])?\s*[〜～~]\s*(?:(\d{4})年)?(\d{1,2})月(\d{1,2})日(?:[（(][月火水木金土日][）)])?$`)
var datedPattern = regexp.MustCompile(`(\d{4})年(\d{1,2})月(\d{1,2})日`)
var agePattern = regexp.MustCompile(`対象年齢(?:は|[:：])?\s*(?:生後)?(\d{1,3})\s*(カ月|か月|ヶ月|ケ月|ヵ月|歳)\s*[〜～~]\s*(\d{1,3})\s*(歳|カ月|か月|ヶ月|ケ月|ヵ月)`)
var applicationPattern = regexp.MustCompile(`(?:申込|申し込み|申込み|応募|予約)(?:期間|受付期間)(?:は|[:：])?\s*(\d{4})年(\d{1,2})月(\d{1,2})日(?:[（(][月火水木金土日][）)])?\s*から\s*(?:(\d{4})年)?(\d{1,2})月(\d{1,2})日(?:[（(][月火水木金土日][）)])?\s*まで`)
var amenityPresentPattern = regexp.MustCompile(`あります|あり(?:[、。]|$)|完備(?:です|しています|している|しており|されて|$)|充実(?:しています|している|しており)|利用(?:できます|可(?:能|$))|(?:設置|対応)(?:しています|している|しており|されている|されています|済|可)`)
var capacityPattern = regexp.MustCompile(`定員(?:は|[:：])?\s*([0-9,]+)\s*名`)

func ParseReference(ref string) (kind, id string, err error) {
	if strings.HasPrefix(ref, BaseURL+"/") {
		ref = strings.TrimPrefix(ref, BaseURL+"/")
	}
	m := refPattern.FindStringSubmatch(ref)
	if m == nil {
		return "", "", fmt.Errorf("reference must be spots/8220 or events/8412 (a canonical Trip URL also works)")
	}
	return m[1], m[2], nil
}
func attr(n *html.Node, key string) string {
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
	f(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, f)
	}
}
func nodeText(n *html.Node) string {
	var b strings.Builder
	var visit func(*html.Node)
	visit = func(x *html.Node) {
		if x.Type == html.ElementNode {
			switch x.Data {
			case "script", "style", "iframe", "noscript", "template":
				return
			case "br":
				b.WriteByte('\n')
			}
		}
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(n)
	lines := strings.Split(b.String(), "\n")
	for i, x := range lines {
		lines[i] = cliutil.CleanText(x)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
func clip(s string, limit int) string {
	r := []rune(s)
	if len(r) > limit {
		return string(r[:limit]) + "…"
	}
	return s
}
func date(y, m, d string) string {
	yy, _ := strconv.Atoi(y)
	mm, _ := strconv.Atoi(m)
	dd, _ := strconv.Atoi(d)
	if yy < 1900 || yy > 2200 || mm < 1 || mm > 12 || dd < 1 || dd > 31 {
		return ""
	}
	t := time.Date(yy, time.Month(mm), dd, 0, 0, 0, 0, time.UTC)
	if t.Year() != yy || int(t.Month()) != mm || t.Day() != dd {
		return ""
	}
	return t.Format("2006-01-02")
}
func parseSchedule(raw string, now time.Time) Schedule {
	s := Schedule{Raw: clip(raw, 240), Precision: "unknown", Status: "unknown", Note: "Published dates do not establish individual operating days or live availability."}
	text := strings.ReplaceAll(norm.NFKC.String(raw), " ", "")
	if m := dayPattern.FindStringSubmatch(text); m != nil {
		s.Start = date(m[1], m[2], m[3])
		s.End = s.Start
		if s.Start != "" {
			s.Precision = "single_date"
		}
	}
	if m := rangePattern.FindStringSubmatch(text); m != nil {
		year := m[4]
		if year == "" {
			year = m[1]
		}
		s.Start = date(m[1], m[2], m[3])
		s.End = date(year, m[5], m[6])
		if s.Start != "" && s.End >= s.Start {
			s.Precision = "published_span"
		} else {
			s.Start = ""
			s.End = ""
		}
	}
	if s.Start != "" {
		today := now.In(time.FixedZone("JST", 9*3600)).Format("2006-01-02")
		switch {
		case s.End < today:
			s.Status = "ended"
		case s.Start > today:
			s.Status = "upcoming"
		default:
			s.Status = "published_span_current"
		}
	}
	return s
}
func safeURL(raw, base string) string {
	u, e := url.Parse(raw)
	if e != nil {
		return ""
	}
	b, _ := url.Parse(base)
	u = b.ResolveReference(u)
	if u.Scheme != "https" && u.Scheme != "http" {
		return ""
	}
	if u.User != nil {
		return ""
	}
	return u.String()
}

func ParseListing(raw []byte, kind, pageURL string, now time.Time) (Page, error) {
	out := Page{Records: make([]Record, 0), Areas: map[int]string{}}
	if kind != "spots" && kind != "events" {
		return out, fmt.Errorf("kind must be spots or events")
	}
	doc, err := html.Parse(strings.NewReader(string(raw)))
	if err != nil {
		return out, err
	}
	base, _ := url.Parse(pageURL)
	expectedIndex := false
	seen := map[string]bool{}
	walk(doc, func(n *html.Node) {
		if hasClass(n, "c-heading--h2_spot") || hasClass(n, "c-heading--h2_event") {
			expectedIndex = true
		}
		if n.Type != html.ElementNode || n.Data != "a" {
			return
		}
		href := safeURL(attr(n, "href"), pageURL)
		u, e := url.Parse(href)
		if e != nil || u.Host != base.Host {
			return
		}
		if u.Path == base.Path {
			v, _ := strconv.Atoi(u.Query().Get("page"))
			if v > out.LastPage {
				out.LastPage = v
			}
			current, _ := strconv.Atoi(base.Query().Get("page"))
			if current == 0 {
				current = 1
			}
			if v == current+1 {
				out.NextURL = href
			}
		}
		if m := regexp.MustCompile(`/` + kind + `/regions/[0-9]+/prefectures/([0-9]+)$`).FindStringSubmatch(u.Path); m != nil && u.RawQuery == "" {
			v, _ := strconv.Atoi(m[1])
			out.Areas[v] = href
		}
		if !hasClass(n, "c-list__link") {
			return
		}
		k, id, e := ParseReference(strings.TrimPrefix(u.Path, "/"))
		if e != nil || k != kind || seen[id] {
			return
		}
		seen[id] = true
		r := blankRecord(k, id, now)
		r.SourceURL = BaseURL + u.Path
		walk(n, func(x *html.Node) {
			if x.Type != html.ElementNode {
				return
			}
			if x.Data == "img" && r.Name == "" {
				r.Name = clip(attr(x, "alt"), 200)
			}
			if hasClass(x, "c-label") {
				t := nodeText(x)
				if t != "" {
					r.Tags = append(r.Tags, clip(t, 80))
				}
			}
			if x.Data == "i" && (hasClass(x, "icon-spot") || hasClass(x, "icon-date") || hasClass(x, "icon-event") || hasClass(x, "icon-spot_name")) {
				row := x.Parent
				for row != nil && row != n && row.Data != "div" {
					row = row.Parent
				}
				if row != nil && row != n {
					txt := nodeText(row)
					if hasClass(x, "icon-spot") {
						r.Location = clip(txt, 240)
					}
					if hasClass(x, "icon-event") || hasClass(x, "icon-spot_name") {
						r.Name = clip(txt, 200)
					}
					if hasClass(x, "icon-date") {
						r.Schedule = parseSchedule(txt, now)
					}
				}
			}

		})
		if kind == "spots" {
			r.Schedule.Status = "not_event"
		}
		if r.Name != "" {
			out.Records = append(out.Records, r)
		}
	})
	// Minimized test/capture samples use c-list without the full-page heading.
	if !expectedIndex {
		walk(doc, func(n *html.Node) { expectedIndex = expectedIndex || hasClass(n, "c-list") })
	}
	if !expectedIndex {
		return out, fmt.Errorf("Trip listing structure missing; refusing to treat this page as an empty catalog")
	}
	if out.LastPage == 0 {
		out.LastPage = 1
	}
	return out, nil
}
func addEvidence(r *Record, location, text string) string {
	text = clip(text, 180)
	for _, e := range r.Evidence {
		if e.Text == text {
			return e.ID
		}
	}
	if len(r.Evidence) >= 8 {
		return ""
	}
	id := fmt.Sprintf("e%d", len(r.Evidence)+1)
	r.Evidence = append(r.Evidence, Evidence{ID: id, Location: location, Text: text})
	return id
}
func appendID(ids []string, id string) []string {
	if id == "" {
		return ids
	}
	for _, v := range ids {
		if v == id {
			return ids
		}
	}
	return append(ids, id)
}

// amenityClause bounds a predicate to its amenity, retaining a shared
// enumeration such as "授乳室やおむつ替えスペースもあります". A following
// separately qualified amenity must not supply presence or absence for this one.
func amenityClause(text, keyword string) string {
	at := strings.Index(text, keyword)
	if at < 0 {
		return ""
	}
	tail := text[at:]
	end := len(tail)
	for _, separator := range []string{"。", ";", "；", "、", ",", "ですが", "ませんが", "ないが", "なく", "ものの", "一方", "しかし", "ただし"} {
		pos := strings.Index(tail, separator)
		if pos < 0 {
			continue
		}
		boundary := pos
		switch separator {
		case "ませんが":
			boundary += len("ません")
		case "ないが":
			boundary += len("ない")
		case "なく":
			boundary += len("なく")
		case "ですが":
			boundary += len("です")
		}
		// A comma directly after an amenity is a list separator. We can
		// conservatively preserve it until the next separately qualified noun.
		if (separator == "、" || separator == ",") && pos == len(keyword) {
			continue
		}
		if boundary < end {
			end = boundary
		}
	}
	for _, other := range []string{"屋内", "室内", "授乳室", "おむつ替え", "オムツ替え", "おむつ交換", "オムツ交換", "ベビーカー"} {
		if other == keyword {
			continue
		}
		pos := strings.Index(tail[len(keyword):], other)
		if pos < 0 {
			continue
		}
		pos += len(keyword)
		between := strings.TrimSpace(tail[len(keyword):pos])
		shared := between == "や" || between == "と" || between == "、" || between == "," || between == "・"
		if !shared && pos < end {
			end = pos
		}
	}
	return tail[:end]
}
func factStatement(text, keyword string) string {
	if !strings.Contains(text, keyword) {
		return ""
	}
	tail := amenityClause(text, keyword)
	// Off-site/conditional statements remain mentions. Assess only this
	// clause so qualifications about a different amenity cannot leak over.
	for _, term := range []string{"場合", "予定", "計画", "検討", "準備中", "可能性", "かもしれ", "近隣", "別施設", "周辺", "他施設"} {
		if strings.Contains(tail, term) {
			return "mentioned"
		}
	}
	// Also preserve an off-site prefix immediately before the amenity.
	prefix := text[:strings.Index(text, keyword)]
	if strings.Contains(prefix, "近隣") || strings.Contains(prefix, "周辺") || strings.Contains(prefix, "別施設") || strings.Contains(prefix, "他施設") {
		return "mentioned"
	}
	for _, term := range []string{"ありません", "ございません", "未設置", "不可", "なし", "はない", "がない", "はなく", "がなく", "できません", "できない", "利用できず", "設置されていない", "設置されておりません", "設置していません", "設置されていません", "設置しておりません", "設置していない", "対応していません", "対応しておりません", "対応していない", "ではありません"} {
		if strings.Contains(tail, term) {
			return "reported_absent"
		}
	}
	if strings.Contains(tail, "？") || strings.Contains(tail, "?") || strings.Contains(tail, "ますか") || strings.Contains(tail, "ありそう") || strings.Contains(tail, "充実していない") || strings.Contains(tail, "充実していません") {
		return "mentioned"
	}
	for _, term := range []string{"完備していません", "完備しておりません", "完備していない", "完備されていません"} {
		if strings.Contains(tail, term) {
			return "reported_absent"
		}
	}
	if amenityPresentPattern.MatchString(tail) {
		return "reported_present"
	}
	if keyword == "屋内" && (strings.Contains(tail, "エリア") || strings.Contains(tail, "ゾーン")) {
		return "reported_present"
	}
	return "mentioned"
}
func ageRangeAffirmed(text, matched string) bool {
	at := strings.Index(text, matched)
	if at < 0 {
		return false
	}
	tail := strings.TrimSpace(text[at+len(matched):])
	for _, neg := range []string{"ではありません", "ではない", "ではなく", "ではございません", "じゃありません", "とは限りません"} {
		if strings.HasPrefix(tail, neg) {
			return false
		}
	}
	return true
}
func absorbSentence(r *Record, text, location string) {
	normalized := norm.NFKC.String(text)
	for field, keys := range map[string][]string{"indoor": {"屋内", "室内"}, "nursing": {"授乳室"}, "changing": {"おむつ替え", "オムツ替え", "おむつ交換", "オムツ交換"}, "stroller": {"ベビーカー"}} {
		for _, key := range keys {
			state := factStatement(normalized, key)
			if state == "" {
				continue
			}
			id := addEvidence(r, location, text)
			if id == "" {
				continue
			}
			old := r.Amenities[field]
			if old.Status != "unknown" && old.Status != state {
				old.Status = "mentioned"
			} else {
				old.Status = state
			}
			old.EvidenceIDs = appendID(old.EvidenceIDs, id)
			r.Amenities[field] = old
			break
		}
	}
	if m := agePattern.FindStringSubmatch(normalized); m != nil && ageRangeAffirmed(normalized, m[0]) {
		min, _ := strconv.Atoi(m[1])
		max, _ := strconv.Atoi(m[3])
		if m[2] == "歳" {
			min *= 12
		}
		if m[4] == "歳" {
			if strings.Contains(normalized, m[3]+"歳未満") {
				max *= 12
			} else {
				max = (max + 1) * 12
			}
		} else {
			max++
		}
		if min >= 0 && min < max && max <= 240 {
			id := addEvidence(r, location, text)
			if id == "" {
				return
			}
			r.Age.Status = "published_range"
			r.Age.MinMonths = &min
			r.Age.MaxExclusiveMonths = &max
			r.Age.EvidenceIDs = appendID(r.Age.EvidenceIDs, id)
		}
	}
	if strings.Contains(normalized, "予約") || strings.Contains(normalized, "申込") || strings.Contains(normalized, "申し込み") || strings.Contains(normalized, "応募") {
		id := addEvidence(r, location, text)
		if id == "" {
			return
		}
		r.Booking.EvidenceIDs = appendID(r.Booking.EvidenceIDs, id)
		r.Booking.Status = "application_or_reservation_mentioned"
		if strings.Contains(normalized, "予約不要") || strings.Contains(normalized, "予約は不要") || strings.Contains(normalized, "申込不要") {
			r.Booking.Status = "not_required_statement"
		}
		if m := applicationPattern.FindStringSubmatch(normalized); m != nil {
			year := m[4]
			if year == "" {
				year = m[1]
			}
			start, end := date(m[1], m[2], m[3]), date(year, m[5], m[6])
			if start != "" && end >= start {
				r.Booking.ApplicationStart = start
				r.Booking.ApplicationEnd = end
			}
		}
	}
	if m := capacityPattern.FindStringSubmatch(normalized); m != nil {
		v, e := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
		if e == nil && v > 0 {
			id := addEvidence(r, location, text)
			if id != "" {
				r.Booking.CapacityPeople = &v
				r.Booking.EvidenceIDs = appendID(r.Booking.EvidenceIDs, id)
			}
		}
	}
	if strings.Contains(normalized, "抽選") {
		id := addEvidence(r, location, text)
		if id != "" {
			r.Booking.Lottery = Fact{Status: "lottery_mentioned", EvidenceIDs: []string{id}}
		}
	}
}
func ParseDetail(raw []byte, ref string, now time.Time) (Record, error) {
	kind, id, err := ParseReference(ref)
	if err != nil {
		return Record{}, err
	}
	r := blankRecord(kind, id, now)
	r.Detail = true
	doc, err := html.Parse(strings.NewReader(string(raw)))
	if err != nil {
		return r, err
	}
	found := false
	table := map[string]string{}
	paragraph := 0
	walk(doc, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		if n.Data == "link" && attr(n, "rel") == "canonical" {
			canonical := safeURL(attr(n, "href"), r.SourceURL)
			u, e := url.Parse(canonical)
			if e == nil && strings.Trim(u.Path, "/") != r.Ref {
				err = fmt.Errorf("source canonical URL differs from requested reference %s", r.Ref)
			}
		}
		if hasClass(n, "p-shared-basic_info") {
			found = true
			walk(n, func(row *html.Node) {
				if row.Data != "tr" {
					return
				}
				key, value := "", ""
				for cell := row.FirstChild; cell != nil; cell = cell.NextSibling {
					if cell.Data == "th" {
						key = nodeText(cell)
					}
					if cell.Data == "td" {
						value = nodeText(cell)
						if key == "公式URL" {
							walk(cell, func(a *html.Node) {
								if a.Data == "a" && r.OfficialURL == "" {
									r.OfficialURL = safeURL(attr(a, "href"), r.SourceURL)
								}
							})
						}
					}
				}
				if key != "" && value != "" {
					table[key] = value
				}
			})
		}
		if n.Data == "p" && hasClass(n, "p-shared-paragraph") {
			paragraph++
			text := nodeText(n)
			for _, sentence := range strings.FieldsFunc(text, func(r rune) bool { return r == '。' || r == '\n' }) {
				absorbSentence(&r, sentence, fmt.Sprintf("p.p-shared-paragraph[%d]", paragraph))
			}
		}
		if hasClass(n, "u-text--right") {
			text := nodeText(n)
			if strings.Contains(text, "公開日") && strings.Contains(text, "更新日") {
				dates := datedPattern.FindAllStringSubmatch(text, 2)
				if len(dates) == 2 {
					r.PublishedAt = date(dates[0][1], dates[0][2], dates[0][3])
					r.UpdatedAt = date(dates[1][1], dates[1][2], dates[1][3])
				}
			}
		}
	})
	if err != nil {
		return r, err
	}
	if !found {
		return r, fmt.Errorf("Trip basic-information table missing for %s; source shape changed or wrong page", ref)
	}
	r.Name = clip(table[map[string]string{"spots": "スポット名", "events": "イベント名"}[kind]], 200)
	if r.Name == "" {
		return r, fmt.Errorf("Trip source name missing for %s", ref)
	}
	r.Reading = clip(table["ふりがな"], 200)
	r.Address = clip(table["住所"], 400)
	r.Location = r.Address
	r.Access = clip(table["アクセス"], 500)
	r.Hours = clip(table["営業時間"], 500)
	if kind == "events" {
		r.Hours = clip(table["開催時間"], 500)
	}
	r.ClosedDays = clip(table["定休日"], 300)
	r.Fees.PublishedText = clip(table["料金"], 700)
	if strings.Contains(r.Fees.PublishedText, "円") {
		r.Fees.Currency = "JPY"
	}
	for _, line := range strings.Split(r.Fees.PublishedText, "\n") {
		if strings.HasPrefix(line, "子供:") || strings.HasPrefix(line, "子供：") || strings.HasPrefix(line, "子ども:") || strings.HasPrefix(line, "小人:") {
			r.Fees.Child = line
		}
		if strings.HasPrefix(line, "大人:") || strings.HasPrefix(line, "大人：") {
			r.Fees.Adult = line
		}
	}
	if kind == "events" {
		r.Schedule = parseSchedule(table["開催期間"], now)
	} else {
		r.Schedule.Status = "not_event"
	}
	for key, value := range table {
		if key == "対象年齢" || key == "予約" || key == "定員" {
			absorbSentence(&r, key+":"+value, "basic_information."+key)
		}
	}
	return r, nil
}
