// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/japan47go/internal/cliutil"
	"golang.org/x/net/html"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const BaseURL = "https://www.japan47go.travel"

var uuidRE = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)
var emailRE = regexp.MustCompile(`(?i)[a-z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-z0-9.-]+\.[a-z]{2,}`)
var privateRE = regexp.MustCompile(`ガイド数|平均年齢|スタッフ|担当者|プロフィール|連絡先|メール|電話|ＦＡＸ|FAX|Email|E-mail`)
var phoneRE = regexp.MustCompile(`\b0\d{1,4}[-ー]\d{1,4}[-ー]\d{3,4}\b`)

func text(s string, n int) string {
	s = cliutil.CleanText(s)
	s = emailRE.ReplaceAllString(s, "[contact omitted]")
	s = phoneRE.ReplaceAllString(s, "[contact omitted]")
	lines := strings.Split(s, "\n")
	out := []string{}
	for _, l := range lines {
		if !privateRE.MatchString(l) {
			out = append(out, l)
		}
	}
	rs := []rune(strings.TrimSpace(strings.Join(out, "\n")))
	if len(rs) > n {
		rs = rs[:n]
	}
	return string(rs)
}
func pointer(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func ID(s string) (string, error) {
	if strings.Contains(s, "://") {
		u, e := url.Parse(s)
		if e != nil || u.Scheme != "https" || u.Host != "www.japan47go.travel" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return "", fmt.Errorf("ID URL must be a canonical https://www.japan47go.travel/ja/detail/UUID URL")
		}
		s = strings.TrimPrefix(u.Path, "/ja/detail/")
	}
	if !uuidRE.MatchString(s) {
		return "", fmt.Errorf("ID must be a lowercase source UUID or canonical JAPAN47GO detail URL")
	}
	return s, nil
}

// PageProps extracts only the bounded __NEXT_DATA__ script from source HTML.
func PageProps(body []byte) (json.RawMessage, error) {
	z := html.NewTokenizer(bytes.NewReader(body))
	found := false
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt == html.StartTagToken {
			t := z.Token()
			if t.Data == "script" {
				for _, a := range t.Attr {
					if a.Key == "id" && a.Val == "__NEXT_DATA__" {
						found = true
					}
				}
			}
		} else if found && tt == html.TextToken {
			var v struct {
				Props struct {
					PageProps json.RawMessage `json:"pageProps"`
				} `json:"props"`
			}
			if e := json.Unmarshal(z.Text(), &v); e != nil {
				return nil, fmt.Errorf("invalid __NEXT_DATA__: %w", e)
			}
			if len(v.Props.PageProps) == 0 {
				return nil, fmt.Errorf("source page has no pageProps")
			}
			return v.Props.PageProps, nil
		}
	}
	return nil, fmt.Errorf("source page has no __NEXT_DATA__ structured content")
}

var negativeSameDay = regexp.MustCompile(`当日[^\n・、,;；]{0,24}(?:不可|禁止|できません|出来ません|できない|出来ない|受け付けません|受け付けていません|事前予約が必要)`)
var negativeNoReservation = regexp.MustCompile(`予約不要(?:ではありません|ではない|ではなく|でない)`)
var conditionalRequestOption = regexp.MustCompile(`ただし|場合|限り|条件|要確認|応相談|要予約`)
var leadingWeatherConnector = regexp.MustCompile(`^\s*(?:ただし|また|なお|但し|尚)[、,]?\s*`)
var independentWeatherCancellation = regexp.MustCompile(`^\s*雨天の場合は中止\s*$`)
var separateExpenses = regexp.MustCompile(`(?:交通費|入場料|保険料|資料代|弁当代|食事代|昼食代|宿泊費)[^\n。、,;；]{0,12}別途|別途[^\n。、,;；]{0,12}(?:交通費|入場料|保険料|資料代|弁当代|食事代|昼食代|宿泊費)`)

// Only a whole standalone rain-cancellation clause is known to constrain operation.
// Unexplained continuation conditions still leave the request ambiguous.
func hasRequestOptionCondition(original string) bool {
	for _, clause := range strings.FieldsFunc(original, func(r rune) bool { return r == '。' || r == '\n' || r == ';' || r == '；' }) {
		if !conditionalRequestOption.MatchString(clause) {
			continue
		}
		if !independentWeatherCancellation.MatchString(leadingWeatherConnector.ReplaceAllString(clause, "")) {
			return true
		}
	}
	return false
}

func ParseRequest(s string) Request {
	r := Request{Status: "unknown", LeadTimes: []LeadTime{}, Options: []string{}, Original: ""}
	p := strings.Index(s, "【予約期限】")
	if p < 0 {
		return r
	}
	v := s[p+len("【予約期限】"):]
	if i := strings.Index(v, "【"); i >= 0 {
		v = v[:i]
	}
	r.Original = text(v, 400)
	ambiguousQuantity := regexp.MustCompile(`[0-9]+(?:\.[0-9]+|\s*[-～〜/]\s*[0-9]+)\s*(?:日|週間|週|ヶ月|か月|ケ月|カ月|月)\s*前`)
	if ambiguousQuantity.MatchString(r.Original) {
		r.Status = "ambiguous"
		return r
	}
	re := regexp.MustCompile(`([0-9]+)\s*(日|週間|週|ヶ月|か月|ケ月|カ月|月)\s*前`)
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(r.Original, -1) {
		n, _ := strconv.Atoi(m[1])
		if n < 1 || n > 366 {
			continue
		}
		unit := "days"
		if strings.HasPrefix(m[2], "週") {
			unit = "weeks"
		} else if m[2] != "日" {
			unit = "months"
		}
		k := fmt.Sprintf("%d%s", n, unit)
		if !seen[k] {
			r.LeadTimes = append(r.LeadTimes, LeadTime{n, unit, m[0]})
			seen[k] = true
		}
	}
	negDay := negativeSameDay.MatchString(r.Original)
	negNone := negativeNoReservation.MatchString(r.Original)
	for _, o := range []string{"予約不要", "当日"} {
		if strings.Contains(r.Original, o) && !(o == "当日" && negDay) && !(o == "予約不要" && negNone) {
			r.Options = append(r.Options, o)
		}
	}
	if len(r.Options) > 0 && (negDay || negNone || hasRequestOptionCondition(r.Original)) {
		r.Status = "ambiguous"
		return r
	}
	if len(r.LeadTimes)+len(r.Options) == 1 {
		r.Status = "known"
	} else if len(r.LeadTimes)+len(r.Options) > 1 {
		r.Status = "ambiguous"
	}
	return r
}

// Keep broad cost references for evidence, but only affirmative obligations
// affect cost compatibility. A waiver applies to its matching cost clause.
func hasSeparateExpenseObligation(original string) bool {
	for _, clause := range strings.FieldsFunc(original, func(r rune) bool { return r == '。' || r == '\n' || r == '、' || r == ',' || r == ';' || r == '；' }) {
		for _, match := range separateExpenses.FindAllStringIndex(clause, -1) {
			tail := strings.TrimSpace(clause[match[1]:])
			if strings.HasPrefix(tail, "不要") || strings.HasPrefix(tail, "は不要") {
				continue
			}
			return true
		}
	}
	return false
}

func ParsePrice(raw, description string) Price {
	original := text(raw, 700)
	desc := text(description, 1600)
	feeLines := []string{}
	numeric := regexp.MustCompile(`([0-9][0-9,]*)\s*円([～〜~]?)`)
	for _, l := range strings.Split(desc, "\n") {
		if strings.Contains(l, "料金") || numeric.MatchString(l) || strings.Contains(l, "無料") || strings.Contains(l, "実費") || separateExpenses.MatchString(l) {
			feeLines = append(feeLines, l)
		}
	}
	for _, line := range feeLines {
		if !strings.Contains(original, line) {
			original = text(original+"\n"+line, 700)
		}
	}
	p := Price{Status: "unknown", Amounts: []Amount{}, Qualifiers: []string{}, Original: original}
	all := original + "\n" + strings.Join(feeLines, "\n")
	free := strings.Contains(all, "無料")
	separate := hasSeparateExpenseObligation(all)
	expenses := strings.Contains(all, "実費") || separate
	paid := strings.Contains(all, "有料")
	matches := numeric.FindAllStringSubmatch(original, -1)
	for _, m := range matches {
		n, e := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
		if e != nil || n > 100000000 {
			continue
		}
		q := "stated"
		if m[2] != "" {
			q = "from"
		}
		var unit *string
		personUnit := strings.Contains(original, "1人") || strings.Contains(original, "一人") || strings.Contains(original, "1名") || strings.Contains(original, "お一人")
		groupUnit := strings.Contains(original, "1組") || strings.Contains(original, "一組")
		if len(matches) == 1 && personUnit && !groupUnit {
			v := "per_person"
			unit = &v
		} else if len(matches) == 1 && groupUnit && !personUnit {
			v := "per_group"
			unit = &v
		}
		p.Amounts = append(p.Amounts, Amount{n, q, unit, m[0]})
		if n > 0 {
			paid = true
		}
	}
	if expenses {
		p.Qualifiers = append(p.Qualifiers, "expenses")
	}
	if strings.Contains(all, "ガイド料") {
		p.Qualifiers = append(p.Qualifiers, "guide_fee")
	}
	if free && separate {
		p.Status = "expenses"
	} else if free && (paid || expenses) {
		p.Status = "ambiguous"
	} else if free {
		p.Status = "free"
	} else if expenses && !paid {
		p.Status = "expenses"
	} else if paid {
		p.Status = "paid"
	}
	return p
}
func ParseDurations(s string) ([]Duration, []int) {
	out := []Duration{}
	mins := []int{}
	seen := map[string]bool{}
	minutes := func(value, unit, half string) int {
		n, _ := strconv.Atoi(value)
		if unit == "時間" {
			n *= 60
			if half != "" {
				n += 30
			}
		}
		return n
	}
	add := func(lo, hi int, original string, isRange bool) {
		if lo < 1 || hi < lo || hi > 1440 {
			return
		}
		key := fmt.Sprintf("%d:%d", lo, hi)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, Duration{lo, hi, text(original, 240)})
		if lo == hi && !isRange {
			mins = append(mins, lo)
		}
	}
	// Consume the whole explicitly unit-qualified range before exact durations,
	// so real 30分～2時間 evidence cannot become two course alternatives.
	ranges := regexp.MustCompile(`([0-9]+)\s*(時間|分)(半)?\s*(?:[-～〜]|から)\s*([0-9]+)\s*(時間|分)(半)?`)
	scalars := regexp.MustCompile(`([0-9]+)(?:\s*[-～〜]\s*([0-9]+))?\s*(時間|分)(半)?`)
	for _, line := range strings.Split(text(s, 1800), "\n") {
		for _, m := range ranges.FindAllStringSubmatch(line, -1) {
			add(minutes(m[1], m[2], m[3]), minutes(m[4], m[5], m[6]), line, true)
		}
		remainder := ranges.ReplaceAllString(line, "")
		for _, m := range scalars.FindAllStringSubmatch(remainder, -1) {
			lo := minutes(m[1], m[3], m[4])
			hi := lo
			if m[2] != "" {
				hi = minutes(m[2], m[3], m[4])
			}
			add(lo, hi, line, m[2] != "")
		}
	}
	sort.Ints(mins)
	return out, mins
}
func ParseMinimumParty(s string) (*int, string) {
	re := regexp.MustCompile(`(?:最低催行人数|最少催行人数|最小催行人数|最低人数)\s*[:：]?\s*([0-9]+)\s*(?:名|人)`)
	matches := re.FindAllStringSubmatch(text(s, 2000), -1)
	var value *int
	ev := []string{}
	for _, m := range matches {
		n, _ := strconv.Atoi(m[1])
		if n < 1 || n > 10000 {
			continue
		}
		ev = append(ev, m[0])
		if value != nil && *value != n {
			return nil, text(strings.Join(ev, "; "), 400)
		}
		value = &n
	}
	return value, text(strings.Join(ev, "; "), 400)
}
func sourceDate(s string) *string {
	if len(s) < 10 {
		return nil
	}
	v := s[:10]
	if _, e := time.Parse("2006-01-02", v); e != nil {
		return nil
	}
	return &v
}
func ParseDetail(props []byte, id string, at time.Time) (Service, error) {
	var page struct {
		Article struct {
			Type string `json:"articleType"`
			Slug string `json:"slug"`
			Data struct {
				Slug        string `json:"slug"`
				Name        string `json:"name"`
				NameCommon  string `json:"nameCommon"`
				Description string `json:"description"`
				Notice      string `json:"importantNotice"`
				Notes       string `json:"notes"`
				Opening     string `json:"openingDateNotes"`
				Closed      *bool  `json:"closed"`
				Updated     string `json:"updateDate"`
				Start       string `json:"startDate"`
				End         string `json:"endDate"`
				Access      string `json:"access"`
				Accessible  string `json:"accessible"`
				Language    string `json:"multilingual"`
				Location    struct {
					Prefecture struct {
						Label string `json:"label"`
					} `json:"prefecture"`
					City struct {
						Label string `json:"label"`
					} `json:"city"`
				} `json:"location"`
				Price struct {
					Value string `json:"price"`
					Notes string `json:"priceNotes"`
				} `json:"price"`
				Category struct {
					ID   int    `json:"id2"`
					Text string `json:"text2"`
				} `json:"categoryTag"`
				URLs []struct {
					URL string `json:"url"`
				} `json:"url"`
			} `json:"data"`
		} `json:"article"`
	}
	if e := json.Unmarshal(props, &page); e != nil {
		return Service{}, fmt.Errorf("invalid detail structure: %w", e)
	}
	a := page.Article.Data
	if page.Article.Slug != id || a.Slug != id || (a.Name == "" && a.NameCommon == "") {
		return Service{}, fmt.Errorf("source detail UUID/name mismatch or missing article")
	}
	if page.Article.Type != "location" && page.Article.Type != "event" {
		return Service{}, &UnsupportedError{Type: page.Article.Type}
	}
	name := a.Name
	if name == "" {
		name = a.NameCommon
	}
	description := text(a.Description, 1800)
	extra := text(a.Notice+"\n"+a.Notes, 600)
	min, ev := ParseMinimumParty(description + "\n" + extra)
	dur, minutes := ParseDurations(description)
	kind := "experience"
	if a.Category.ID == 617 {
		kind = "guide"
	} else if page.Article.Type == "event" {
		kind = "event"
	}
	s := Service{ID: id, NameJA: text(name, 200), Kind: kind, PrefectureJA: text(a.Location.Prefecture.Label, 80), CityJA: text(a.Location.City.Label, 100), SourceURL: BaseURL + "/ja/detail/" + id, SourceUpdatedAt: pointer(a.Updated), ObservedAt: at.UTC().Format(time.RFC3339Nano), SourceClosed: a.Closed, Availability: "unknown", Request: ParseRequest(a.Opening), Price: ParsePrice(a.Price.Value+"\n"+a.Price.Notes, description), MinimumParty: min, PartyEvidence: ev, Durations: dur, DurationsMinutes: minutes, Schedule: Schedule{StartDate: sourceDate(a.Start), EndDate: sourceDate(a.End), Original: text(a.Opening+"\n"+extra, 900), Operation: "unknown"}, DescriptionEvidence: description, AccessEvidence: text(a.Access, 500), AccessibilityEvidence: text(a.Accessible, 500), LanguageEvidence: text(a.Language, 300), OrganizationURLs: []string{}, Transport: "live"}
	for _, v := range a.URLs {
		u, e := url.Parse(v.URL)
		if e == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" && u.User == nil && !emailRE.MatchString(v.URL) && len(v.URL) < 512 && len(s.OrganizationURLs) < 5 {
			s.OrganizationURLs = append(s.OrganizationURLs, v.URL)
		}
	}
	return s, nil
}

type UnsupportedError struct{ Type string }

func (e *UnsupportedError) Error() string {
	return "unsupported JAPAN47GO article type " + e.Type + "; local service location/event records are supported"
}
func ParseListing(props []byte, expected int, at time.Time) ([]Candidate, Coverage, error) {
	var p struct {
		Items []struct {
			Slug       string `json:"slug"`
			Name       string `json:"name"`
			Type       string `json:"articleType"`
			Updated    string `json:"updateDate"`
			Prefecture struct {
				Label string `json:"label"`
			} `json:"prefecture"`
			City struct {
				Label string `json:"label"`
			} `json:"city"`
			Category struct {
				ID int `json:"id2"`
			} `json:"categoryTag"`
		} `json:"items"`
		Page *struct {
			Total int `json:"totalCnt"`
			Size  int `json:"perPage"`
			Pages int `json:"totalPageCnt"`
			No    int `json:"pageNo"`
		} `json:"pageInfo"`
	}
	if e := json.Unmarshal(props, &p); e != nil {
		return nil, Coverage{}, fmt.Errorf("invalid listing: %w", e)
	}
	if p.Page == nil || p.Page.No != expected || p.Page.Size < 1 || p.Page.Size > 100 || p.Page.Total < 0 || p.Page.Pages < 0 || len(p.Items) > p.Page.Size {
		return nil, Coverage{}, fmt.Errorf("source pagination missing or inconsistent; refusing a false complete result")
	}
	out := []Candidate{}
	for _, v := range p.Items {
		if _, e := ID(v.Slug); e != nil || v.Name == "" {
			return nil, Coverage{}, fmt.Errorf("source listing contains invalid UUID/name")
		}
		k := "experience"
		if v.Category.ID == 617 {
			k = "guide"
		} else if v.Type == "event" {
			k = "event"
		}
		out = append(out, Candidate{ID: v.Slug, NameJA: text(v.Name, 200), Kind: k, PrefectureJA: text(v.Prefecture.Label, 80), CityJA: text(v.City.Label, 100), SourceURL: BaseURL + "/ja/detail/" + v.Slug, SourceUpdatedAt: pointer(v.Updated), ObservedAt: at.UTC().Format(time.RFC3339Nano)})
	}
	return out, Coverage{MatchingTotal: p.Page.Total, TotalPages: p.Page.Pages, PageSize: p.Page.Size}, nil
}
