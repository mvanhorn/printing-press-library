package jreast

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// The catalogue contains source-observed identity only, never operational status.
//
//go:embed catalogue.json
var catalogueJSON []byte

type catalogueEntry struct {
	JA string `json:"ja"`
	EN string `json:"en"`
}

var catalogue = func() map[string]map[string]catalogueEntry {
	v := map[string]map[string]catalogueEntry{}
	if e := json.Unmarshal(catalogueJSON, &v); e != nil {
		panic(e)
	}
	return v
}()

var enTime = regexp.MustCompile(`Current as of:\s*(\d{2}/\d{2}/\d{4})\s+at\s+(\d{2}:\d{2})`)
var jaTime = regexp.MustCompile(`(\d{4})年\s*(\d{1,2})月\s*(\d{1,2})日\s*(\d{1,2})時\s*(\d{1,2})分\s*現在`)

func pageState(doc *html.Node, language, source string, now time.Time, staleAfter time.Duration) SourceState {
	st := SourceState{URL: source, Language: language, ObservedAt: now.In(JST).Format(time.RFC3339), Freshness: "missing_timestamp", ReportingState: ReportingCoverage(now).ReportingState}
	s := Normalize(text(doc))
	var tm time.Time
	if language == "en" {
		if m := enTime.FindStringSubmatch(text(doc)); len(m) > 0 {
			tm, _ = time.ParseInLocation("01/02/2006 15:04", m[1]+" "+m[2], JST)
		}
	} else {
		if m := jaTime.FindStringSubmatch(s); len(m) > 0 {
			tm, _ = time.ParseInLocation("2006 1 2 15 4", strings.Join(m[1:], " "), JST)
		}
	}
	if !tm.IsZero() {
		st.UpdatedAt = tm.Format(time.RFC3339)
		age := int64(now.Sub(tm).Seconds())
		st.AgeSeconds = &age
		st.Freshness = "fresh"
		if age < -120 {
			st.Freshness = "future_timestamp"
		} else if now.Sub(tm) > staleAfter {
			st.Freshness = "stale"
		}
	}
	if strings.Contains(s, "情報提供時間は") || strings.Contains(s, "information is provided from") || strings.Contains(s, "outside the service hours") {
		st.ReportingState = "outside_reporting_hours"
	}
	return st
}

func statusKind(label string) string {
	s := Normalize(label)
	switch {
	case strings.Contains(s, "平常運転") || strings.Contains(s, "normal operation"):
		return "normal_label"
	case strings.Contains(s, "一部運休") || strings.Contains(s, "partial cancellation"):
		return "partial_cancellation"
	case strings.Contains(s, "直通運転中止") || strings.Contains(s, "through service cancelled") || strings.Contains(s, "through service suspended"):
		return "through_service_stopped"
	case strings.Contains(s, "運転見合わせ") || strings.Contains(s, "operation suspended"):
		return "suspended"
	case strings.Contains(s, "運休") || strings.Contains(s, "service cancelled"):
		return "cancelled"
	case strings.Contains(s, "遅延") || strings.Contains(s, "delay"):
		return "delayed"
	case strings.Contains(s, "お知らせ") || strings.Contains(s, "notice"):
		return "notice"
	default:
		return "unknown"
	}
}

func nativeID(n *html.Node, region Region, language, source string, name string) string {
	for _, a := range walk(n, func(n *html.Node) bool { return n.Data == "a" }) {
		u, e := url.Parse(attr(a, "href"))
		if e == nil {
			for _, q := range []string{"lineid", "group"} {
				if id := u.Query().Get(q); id != "" {
					return id
				}
			}
		}
	}
	for _, s := range walk(n, func(n *html.Node) bool { return class(n, "rosen_color") || class(n, "traininfo-routes__line") }) {
		for _, v := range strings.Fields(attr(s, "class")) {
			if v != "rosen_color" && v != "traininfo-routes__line" {
				return v
			}
		}
	}
	if u, e := url.Parse(source); e == nil && u.Query().Get("group") != "" {
		return u.Query().Get("group")
	}
	if language == "en" {
		for id, c := range catalogue[region.ID] {
			if Normalize(c.EN) == Normalize(name) {
				return id
			}
		}
	}
	return ""
}

// ParseRegion extracts source-native rows, rejecting shells/challenge pages instead of empty success.
func ParseRegion(body []byte, r Region, language, source string, now time.Time, staleAfter time.Duration) (Page, error) {
	doc, e := html.Parse(strings.NewReader(string(body)))
	if e != nil {
		return Page{}, fmt.Errorf("JR East HTML: %w", e)
	}
	p := Page{Region: r, State: pageState(doc, language, source, now, staleAfter), rows: make([]row, 0)}
	boxes := walk(doc, func(n *html.Node) bool {
		if language == "ja" {
			return class(n, "traininfo-routes__table__item")
		}
		return class(n, "rosenBox")
	})
	if len(boxes) == 0 && language == "en" {
		boxes = walk(doc, func(n *html.Node) bool { return n.Data == "tr" && firstClass(n, "status") != nil })
	}
	if len(boxes) > 512 {
		return Page{}, fmt.Errorf("JR East row scan exceeds 512 rows")
	}
	for _, b := range boxes {
		nameNode := firstClass(b, "traininfo-routes__name")
		if nameNode == nil {
			nameNode = firstClass(b, "name")
		}
		if nameNode == nil {
			hs := walk(b, func(n *html.Node) bool { return n.Data == "th" })
			if len(hs) > 0 {
				nameNode = hs[0]
			}
		}
		name := text(nameNode)
		if name == "" {
			continue
		}
		status := firstClass(b, "traininfo-routes__status")
		if status == nil {
			status = firstClass(b, "traininfo-line-info__status")
		}
		if status == nil {
			status = firstClass(b, "status")
		}
		label := ""
		if status != nil {
			if imgs := walk(status, func(n *html.Node) bool { return n.Data == "img" && attr(n, "alt") != "" }); len(imgs) > 0 {
				label = attr(imgs[0], "alt")
			} else {
				sp := walk(status, func(n *html.Node) bool { return n.Data == "span" })
				if len(sp) > 0 {
					label = text(sp[0])
				} else {
					label = text(status)
				}
			}
		}
		note := firstClass(b, "traininfo-routes__note")
		if note == nil {
			note = firstClass(b, "status_Text")
		}
		if note == nil && b.Data == "tr" {
			note = firstClass(b, "mt10")
		}
		groupID, groupName := group(b)
		code := ""
		for _, im := range walk(b, func(n *html.Node) bool { return n.Data == "img" }) {
			a := attr(im, "alt")
			if len(a) >= 2 && len(a) <= 3 {
				code = a
				break
			}
		}
		id := nativeID(b, r, language, source, name)
		link := source
		for _, a := range walk(b, func(n *html.Node) bool { return n.Data == "a" }) {
			u, e := url.Parse(attr(a, "href"))
			if e == nil && (u.Query().Get("lineid") != "" || u.Query().Get("group") != "") {
				base, _ := url.Parse(Origin)
				link = base.ResolveReference(u).String()
				break
			}
		}
		p.rows = append(p.rows, row{ID: id, Name: name, Code: code, GroupID: groupID, GroupName: groupName, Status: statusKind(label), Label: label, Text: text(note), URL: link, Language: language})
	}
	p.State.ScannedRows = len(p.rows)
	closedMessage := strings.Contains(text(doc), "情報提供時間は") || strings.Contains(strings.ToLower(text(doc)), "outside the service hours") || strings.Contains(strings.ToLower(text(doc)), "information is provided from")
	if len(p.rows) == 0 && !closedMessage {
		return Page{}, fmt.Errorf("JR East source has no recognizable status rows; source changed, errored or returned a shell")
	}
	return p, nil
}

func addUnique(a []string, s string) []string {
	for _, v := range a {
		if v == s {
			return a
		}
	}
	return append(a, s)
}
func kindSet(a []row) []string {
	out := make([]string, 0)
	for _, r := range a {
		out = addUnique(out, r.Status)
	}
	sort.Strings(out)
	return out
}

// JoinRegions matches translated rows by observed names/native IDs, never by changing row position.
func JoinRegions(jp, ep Page, now time.Time) Snapshot {
	s := Snapshot{Region: jp.Region, Sources: []SourceState{jp.State, ep.State}, Lines: make([]Line, 0), Warnings: make([]string, 0)}
	positions := map[string]int{}
	jaRows := map[string][]row{}
	enRows := map[string][]row{}
	for _, r := range ep.rows {
		if r.ID != "" {
			enRows[r.ID] = append(enRows[r.ID], r)
		}
	}
	for _, r := range jp.rows {
		if r.ID == "" {
			s.Warnings = append(s.Warnings, "Japanese source row has no native identity: "+r.Name)
			continue
		}
		jaRows[r.ID] = append(jaRows[r.ID], r)
		i, ok := positions[r.ID]
		if !ok {
			e := catalogue[jp.Region.ID][r.ID]
			i = len(s.Lines)
			positions[r.ID] = i
			s.Lines = append(s.Lines, Line{IdentitySource: "live_source", ID: jp.Region.ID + ":" + r.ID, SourceID: r.ID, Region: jp.Region.ID, NameJA: r.Name, NameEN: e.EN, Code: r.Code, Groups: make([]Group, 0), Statuses: make([]string, 0), Notices: make([]Notice, 0), SourceJA: r.URL, SourceEN: ep.State.URL})
		}
		l := &s.Lines[i]
		l.Statuses = addUnique(l.Statuses, r.Status)
		if r.GroupID != "" {
			found := false
			for _, g := range l.Groups {
				if g.ID == r.GroupID {
					found = true
				}
			}
			if !found {
				l.Groups = append(l.Groups, Group{ID: r.GroupID, NameJA: r.GroupName})
			}
		}
		if r.Text != "" || strings.Contains(jp.State.URL, "express.aspx") {
			n := facts(r)
			if strings.Contains(jp.State.URL, "express.aspx") {
				n.ServiceName = r.Name
			}
			l.Notices = append(l.Notices, n)
		}
	}
	for i := range s.Lines {
		l := &s.Lines[i]
		en := enRows[l.SourceID]
		if len(en) > 0 {
			l.EnglishMapped = true
			l.NameEN = en[0].Name
			l.SourceEN = en[0].URL
			for j := range l.Groups {
				for _, r := range en {
					if r.GroupID == l.Groups[j].ID {
						l.Groups[j].NameEN = r.GroupName
						break
					}
				}
			}
			jaKinds := kindSet(jaRows[l.SourceID])
			enKinds := kindSet(en)
			l.LanguageConflict = strings.Join(jaKinds, ",") != strings.Join(enKinds, ",")
			for _, r := range en {
				if r.Text != "" || strings.Contains(ep.State.URL, "express.aspx") {
					n := facts(r)
					if strings.Contains(ep.State.URL, "express.aspx") {
						n.ServiceName = r.Name
					}
					l.Notices = append(l.Notices, n)
				}
			}
		} else {
			s.Warnings = append(s.Warnings, "English identity not matched for "+l.ID)
		}
		l.Assessment = Assessment(*l, s.Sources, now)
		l.NoticeFactCount = len(l.Notices)
	}
	return s
}

var sectionJA = regexp.MustCompile(`([一-龯ぁ-んァ-ヶA-Za-z0-9・ー]+)[～〜~]([一-龯ぁ-んァ-ヶA-Za-z0-9・ー]+?)(?:駅)?間`)
var sectionEN = regexp.MustCompile(`(?i)between\s+([^.,;]+?)\s+and\s+([^.,;]+?)\s+(?:station|stations)`)
var dateJA = regexp.MustCompile(`(?:\d{4}年)?\d{1,2}月\d{1,2}日(?:[（(][^）)]*[）)])?(?:[～〜]\d{1,2}日(?:[（(][^）)]*[）)])?)?`)
var workRE = regexp.MustCompile(`工事|作業|保守`)
var clockJA = regexp.MustCompile(`(\d{1,2})時(?:(\d{1,2})分)?(?:頃)?(?:から|～|〜)(\d{1,2})時(?:(\d{1,2})分)?`)
var yearJA = regexp.MustCompile(`(\d{4})年`)
var causeJA = regexp.MustCompile(`([^、。]{1,55}?)(?:の影響|のため)`)

// A single year only applies when every reported date explicitly names it.
// A later endpoint's year must not be applied to an earlier yearless date.
func commonCalendarYear(expressions []string) *int {
	if len(expressions) == 0 {
		return nil
	}
	year := 0
	for _, expression := range expressions {
		m := yearJA.FindStringSubmatch(expression)
		if len(m) == 0 || !strings.HasPrefix(expression, m[0]) {
			return nil
		}
		y, _ := strconv.Atoi(m[1])
		if year != 0 && year != y {
			return nil
		}
		year = y
	}
	return &year
}

func facts(r row) Notice {
	s := strings.TrimSpace(r.Text)
	n := Notice{GroupID: r.GroupID, Status: r.Status, Label: r.Label, Language: r.Language, Direction: "unknown", Sections: make([]Section, 0), Dates: make([]string, 0), Replacement: "unknown", AffectedScope: "unknown", SourceURL: r.URL}
	norm := Normalize(s)
	if r.Language == "ja" {
		switch {
		case strings.Contains(norm, "上下線"):
			n.Direction = "both"
		case strings.Contains(norm, "内・外回り"):
			n.Direction = "inner_and_outer"
		case strings.Contains(norm, "内回り"):
			n.Direction = "inner"
		case strings.Contains(norm, "外回り"):
			n.Direction = "outer"
		case strings.Contains(norm, "上り"):
			n.Direction = "inbound"
		case strings.Contains(norm, "下り"):
			n.Direction = "outbound"
		}
		// NFKC folds width but preserves Japanese station/date text.
		full := normText(s)
		if strings.Contains(full, "全区間") {
			n.AffectedScope = "all_sections"
		}
		if m := regexp.MustCompile(`運転再開まで少なくとも(\d{1,2})[かヶ]月`).FindStringSubmatch(full); len(m) > 0 {
			v, _ := strconv.Atoi(m[1])
			n.ResumeMinimumMonths = &v
			n.ResumeEstimate = true
		}
		for _, m := range sectionJA.FindAllStringSubmatch(full, 8) {
			n.Sections = append(n.Sections, Section{From: m[1], To: strings.TrimSuffix(m[2], "駅"), Language: "ja"})
		}
		if m := causeJA.FindStringSubmatch(full); len(m) > 0 {
			n.Cause = cut(m[1], 55)
		}
		n.Planned = r.Status == "notice" && workRE.MatchString(full) && strings.Contains(full, "月")
		n.Dates = dateJA.FindAllString(full, 8)
		if n.Dates == nil {
			n.Dates = make([]string, 0)
		}
		n.Year = commonCalendarYear(dateJA.FindAllString(full, -1))
		if m := clockJA.FindStringSubmatch(full); len(m) > 0 {
			a, _ := strconv.Atoi(m[1])
			am, _ := strconv.Atoi(m[2])
			b, _ := strconv.Atoi(m[3])
			bm, _ := strconv.Atoi(m[4])
			if a < 24 && b < 24 && am < 60 && bm < 60 {
				n.LocalStart = fmt.Sprintf("%02d:%02d", a, am)
				n.LocalEnd = fmt.Sprintf("%02d:%02d", b, bm)
				n.Approximate = strings.Contains(full, "頃")
			}
		}
		if strings.Contains(full, "代行輸送は行いません") {
			n.Replacement = "not_provided"
		} else if strings.Contains(full, "調整") || strings.Contains(full, "確保") {
			n.Replacement = "being_arranged_not_confirmed"
		} else if strings.Contains(full, "バスによる代行") {
			n.Replacement = "bus_reported"
		} else if strings.Contains(full, "タクシーによる代行") {
			n.Replacement = "taxi_reported"
		}
	} else {
		switch {
		case strings.Contains(norm, "inbound and outbound"):
			n.Direction = "both"
		case strings.Contains(norm, "inbound"):
			n.Direction = "inbound"
		case strings.Contains(norm, "outbound"):
			n.Direction = "outbound"
		}
		for _, m := range sectionEN.FindAllStringSubmatch(s, 8) {
			n.Sections = append(n.Sections, Section{From: m[1], To: m[2], Language: "en"})
		}
		if strings.Contains(norm, "typhoon") {
			n.Cause = "Typhoon"
		}
	}
	return n
}

func normText(s string) string { // Width folding only: preserve names' original casing.
	return strings.Map(func(r rune) rune {
		if r >= '０' && r <= '９' {
			return '0' + r - '０'
		}
		return r
	}, s)
}

// ParseAreas reads inert document.write fragments; it does not execute source JavaScript.
func ParseAreas(body []byte, now time.Time) ([]Area, SourceState, error) {
	re := regexp.MustCompile(`document\.write\('((?:[^'\\]|\\.)*)'\);`)
	var b strings.Builder
	for _, m := range re.FindAllStringSubmatch(string(body), 16) {
		s := strings.ReplaceAll(m[1], `\'`, `'`)
		b.WriteString(strings.ReplaceAll(s, `\\`, `\`))
	}
	doc, e := html.Parse(strings.NewReader(b.String()))
	if e != nil {
		return nil, SourceState{}, e
	}
	out := make([]Area, 0)
	for _, a := range walk(doc, func(n *html.Node) bool { return n.Data == "a" }) {
		u, _ := url.Parse(attr(a, "href"))
		if u == nil {
			continue
		}
		for _, r := range Regions() {
			if u.String() == r.SourceEN {
				label := ""
				im := walk(a, func(n *html.Node) bool { return n.Data == "img" })
				if len(im) > 0 {
					label = attr(im[0], "alt")
				}
				out = append(out, Area{Region: r, Status: statusKind(label), Label: label})
			}
		}
	}
	if len(out) != 5 {
		return nil, SourceState{}, fmt.Errorf("JR East summary does not contain exactly five recognized service areas")
	}
	st := SourceState{URL: Origin + "/train_info/e/infotop.aspx", Language: "en", ObservedAt: now.In(JST).Format(time.RFC3339), Freshness: "missing_timestamp", ReportingState: ReportingCoverage(now).ReportingState, ScannedRows: len(out)}
	return out, st, nil
}
