// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package ticket

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/toretabi/internal/cliutil"
	"golang.org/x/net/html"
	"golang.org/x/text/unicode/norm"
)

const Origin = "https://www.toretabi.jp"
const EvidenceRunes = 160
const MaxEvidence = 24

var idRE = regexp.MustCompile(`^[a-z]{2,12}_[0-9]{3}$`)
var jpDate = regexp.MustCompile(`([0-9]{4})年([0-9]{1,2})月([0-9]{1,2})日`)
var rangeRE = regexp.MustCompile(`([0-9]{4})年([0-9]{1,2})月([0-9]{1,2})日[～〜~－-](?:([0-9]{4})年)?(?:([0-9]{1,2})月)?([0-9]{1,2})日`)
var spanRE = regexp.MustCompile(`([0-9]{1,2})月([0-9]{1,2})日[～〜~－-](?:([0-9]{1,2})月)?([0-9]{1,2})日`)
var amountRE = regexp.MustCompile(`([0-9][0-9,]*(?:\.[0-9]+)?)円`)
var daysRE = regexp.MustCompile(`([0-9]+)日間`)

func ValidID(id string) bool { return idRE.MatchString(id) }
func clip(s string) Evidence {
	s = cliutil.CleanText(s)
	r := []rune(s)
	e := Evidence{TextJA: s}
	if len(r) > EvidenceRunes {
		e.TextJA = string(r[:EvidenceRunes])
		e.Truncated = true
	}
	return e
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
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, f)
	}
}
func first(n *html.Node, f func(*html.Node) bool) *html.Node {
	var out *html.Node
	walk(n, func(x *html.Node) {
		if out == nil && f(x) {
			out = x
		}
	})
	return out
}
func text(n *html.Node) string {
	var b strings.Builder
	var rec func(*html.Node)
	rec = func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		if x.Type == html.ElementNode && (x.Data == "script" || x.Data == "style") {
			return
		}
		if x.Data == "br" {
			b.WriteString("\n")
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			rec(c)
		}
		if x.Data == "p" || x.Data == "li" {
			b.WriteString("\n")
		}
	}
	if n != nil {
		rec(n)
	}
	return cliutil.CleanText(b.String())
}
func lines(n *html.Node) []string {
	var b strings.Builder
	walk(n, func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		if x.Data == "br" {
			b.WriteString("\n")
		}
	})
	out := []string{}
	for _, s := range strings.Split(b.String(), "\n") {
		s = cliutil.CleanText(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
func strptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func unique(xs []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, s := range xs {
		if s != "" && !seen[s] {
			out = append(out, s)
			seen[s] = true
		}
	}
	return out
}
func sourceID(raw string) (string, bool) {
	u, e := url.Parse(raw)
	if e != nil {
		return "", false
	}
	if u.Host != "" && u.Host != "www.toretabi.jp" {
		return "", false
	}
	p := strings.TrimPrefix(u.Path, "/ticket/")
	p = strings.TrimSuffix(p, ".html")
	return p, ValidID(p) && u.RawQuery == "" && u.Fragment == "" && u.Path == "/ticket/"+p+".html"
}

// ParseListing accepts only native ticket-list elements, never footer/article links.
func ParseListing(body []byte, observed string) (Listing, error) {
	doc, e := html.Parse(strings.NewReader(string(body)))
	if e != nil {
		return Listing{}, e
	}
	list := first(doc, func(n *html.Node) bool { return n.Data == "ul" && hasClass(n, "nav-index-04") })
	if list == nil {
		return Listing{}, fmt.Errorf("ticket listing selector missing; source contract changed")
	}
	out := Listing{Tickets: []Summary{}, Areas: []CatalogEntry{}, Types: []CatalogEntry{}, ObservedAt: observed}
	walk(doc, func(n *html.Node) {
		if n.Data != "select" {
			return
		}
		name := attr(n, "name")
		if name != "area" && name != "class" {
			return
		}
		walk(n, func(x *html.Node) {
			if x.Data == "option" && attr(x, "value") != "" {
				v := CatalogEntry{Code: attr(x, "value"), LabelJA: text(x)}
				if name == "area" {
					out.Areas = append(out.Areas, v)
				} else {
					out.Types = append(out.Types, v)
				}
			}
		})
	})
	seen := map[string]bool{}
	walk(list, func(n *html.Node) {
		if n.Data != "a" {
			return
		}
		id, ok := sourceID(attr(n, "href"))
		if !ok || seen[id] {
			return
		}
		name := text(first(n, func(x *html.Node) bool { return x.Data == "b" }))
		if name == "" {
			return
		}
		seen[id] = true
		s := Summary{ID: id, NameJA: name, SourceURL: Origin + "/ticket/" + id + ".html", ObservedAt: observed, AreasJA: []string{}, TagsJA: []string{}}
		walk(n, func(x *html.Node) {
			if x.Data == "li" {
				v := text(x)
				if hasClass(x, "type1") {
					s.TypeJA = strptr(v)
				} else if hasClass(x, "type2") {
					s.AreasJA = append(s.AreasJA, v)
				} else if hasClass(x, "type3") {
					s.SeasonJA = strptr(v)
				} else if v != "" {
					s.TagsJA = append(s.TagsJA, v)
				}
			}
		})
		out.Tickets = append(out.Tickets, s)
	})
	if len(out.Areas) == 0 || len(out.Types) == 0 {
		return Listing{}, fmt.Errorf("ticket filter catalog missing; source contract changed")
	}
	return out, nil
}
func date(y, m, d string) (string, bool) {
	yi, _ := strconv.Atoi(y)
	mi, _ := strconv.Atoi(m)
	di, _ := strconv.Atoi(d)
	t := time.Date(yi, time.Month(mi), di, 0, 0, 0, 0, time.UTC)
	if yi < 1900 || yi > 2200 || int(t.Month()) != mi || t.Day() != di {
		return "", false
	}
	return t.Format("2006-01-02"), true
}
func period(raw string) Period {
	original := raw
	raw = norm.NFKC.String(raw)
	p := Period{Evidence: clip(original), Intervals: []Interval{}, ExplicitDates: []string{}, YearRound: strings.Contains(raw, "通年")}
	for _, v := range jpDate.FindAllStringSubmatchIndex(raw, -1) {
		part := raw[v[0]:v[1]]
		s := jpDate.FindStringSubmatch(part)
		iso, ok := date(s[1], s[2], s[3])
		if !ok {
			continue
		}
		p.ExplicitDates = append(p.ExplicitDates, iso)
		tail := raw[v[1]:]
		if strings.HasPrefix(tail, "まで") && p.End == nil {
			p.End = strptr(iso)
		}
		tail = regexp.MustCompile(`^[（(][^）)]{1,10}[）)]`).ReplaceAllString(tail, "")
		if (strings.HasPrefix(tail, "から") || strings.HasPrefix(tail, "より")) && p.Start == nil {
			p.Start = strptr(iso)
		}
	}
	for _, s := range rangeRE.FindAllStringSubmatch(raw, -1) {
		a, ok := date(s[1], s[2], s[3])
		if !ok {
			continue
		}
		ey, em := s[4], s[5]
		der := "explicit_years"
		if ey == "" {
			ey = s[1]
			der = "end_year_inherited_from_range_start"
		}
		if em == "" {
			em = s[2]
		}
		b, ok := date(ey, em, s[6])
		if ok && b < a && s[4] == "" {
			y, _ := strconv.Atoi(ey)
			b, ok = date(strconv.Itoa(y+1), em, s[6])
		}
		if ok && a <= b {
			p.Intervals = append(p.Intervals, Interval{Start: a, End: b, Derivation: der})
		}
	}
	if len(p.Intervals) == 1 && p.Start == nil && p.End == nil {
		p.Start = strptr(p.Intervals[0].Start)
		p.End = strptr(p.Intervals[0].End)
	}
	for _, q := range []string{"土曜", "土休日", "日曜", "金曜", "休日", "祝日", "のみ", "除く", "利用開始", "有効期間開始", "ヶ月前", "カ月前", "か月前", "前日", "当日", "一部", "限定"} {
		if strings.Contains(raw, q) {
			p.Conditional = true
		}
	}
	p.ExplicitDates = unique(p.ExplicitDates)
	return p
}
func money(raw, unit string) []Money {
	out := []Money{}
	seen := map[string]bool{}
	for _, m := range amountRE.FindAllStringSubmatch(raw, -1) {
		v := strings.ReplaceAll(m[1], ",", "")
		if !seen[v] {
			out = append(out, Money{Amount: v, Currency: "JPY", Unit: unit, Evidence: clip(raw)})
			seen[v] = true
		}
	}
	return out
}
func addEvidence(dst *[]Evidence, raw string, truncated *bool) {
	if len(*dst) >= MaxEvidence {
		*truncated = true
		return
	}
	v := clip(raw)
	if v.TextJA == "" {
		return
	}
	for _, old := range *dst {
		if old.TextJA == v.TextJA {
			return
		}
	}
	*dst = append(*dst, v)
	if v.Truncated {
		*truncated = true
	}
}

// ParseDetail separates fare rows from benefits and keeps operator links as handoffs.
func ParseDetail(body []byte, id, observed string) (Ticket, error) {
	if !ValidID(id) {
		return Ticket{}, fmt.Errorf("invalid ticket ID %q", id)
	}
	doc, e := html.Parse(strings.NewReader(string(body)))
	if e != nil {
		return Ticket{}, e
	}
	h := first(doc, func(n *html.Node) bool { return n.Data == "h1" && hasClass(n, "title-01") })
	name := text(h)
	main := first(doc, func(n *html.Node) bool {
		return n.Data == "div" && hasClass(n, "content-cell") && attr(n, "data-area") != ""
	})
	if name == "" || main == nil {
		return Ticket{}, fmt.Errorf("ticket detail selectors missing; source contract changed")
	}
	t := Ticket{Summary: Summary{ID: id, NameJA: name, SourceURL: Origin + "/ticket/" + id + ".html", ObservedAt: observed, AreasJA: []string{}, TagsJA: []string{}}, Sales: period(""), Use: period(""), Price: Price{Status: "unknown", Amounts: []Money{}, Reason: "No explicitly labelled ticket-price row in Toretabi evidence."}, Benefits: []Money{}, Eligibility: []Evidence{}, PurchaseChannels: []Evidence{}, Supplements: []Evidence{}, Exceptions: []Evidence{}, BlackoutSpans: []MonthDaySpan{}, Conditions: []Evidence{}, OperatorURLs: []string{}, InventoryStatus: "unknown", OperatorVerification: "required", Unsupported: []string{"live_seats", "tourist_train_timetables", "pass_savings", "booking"}, Transport: "live"}
	walk(main, func(n *html.Node) {
		if n.Data != "tr" {
			return
		}
		th := first(n, func(x *html.Node) bool { return x.Data == "th" })
		td := first(n, func(x *html.Node) bool { return x.Data == "td" })
		label, v := text(th), text(td)
		switch label {
		case "発売期間":
			t.Sales = period(v)
		case "利用期間":
			t.Use = period(v)
		case "有効期間":
			t.Validity = clip(v)
			if m := daysRE.FindStringSubmatch(v); m != nil {
				d, _ := strconv.Atoi(m[1])
				t.ValidityDays = &d
			}
		case "ねだん", "発売額", "価格", "おねだん", "料金":
			t.Price.Amounts = money(v, "source_labelled_ticket_price")
			if len(t.Price.Amounts) > 0 {
				t.Price.Status = "published"
				t.Price.Reason = "Fare basis and passenger category remain in original evidence."
			}
		}
	})
	walk(main, func(n *html.Node) {
		if n.Data == "p" && !hasClass(n, "btn-01") {
			if first(n, func(x *html.Node) bool { return x.Data == "img" }) != nil {
				return
			}
			ls := lines(n)
			if len(ls) == 0 {
				return
			}
			if t.Description.TextJA == "" {
				t.Description = clip(strings.Join(ls, " "))
			}
			for _, line := range ls {
				for _, part := range strings.SplitAfter(line, "。") {
					part = cliutil.CleanText(part)
					if part == "" {
						continue
					}
					addEvidence(&t.Conditions, part, &t.EvidenceTruncated)
					if strings.Contains(part, "うりば") || strings.Contains(part, "発売はありません") || strings.Contains(part, "のみの発売") || strings.Contains(part, "発売しています") || strings.Contains(part, "窓口") || strings.Contains(part, "券売機") {
						addEvidence(&t.PurchaseChannels, part, &t.EvidenceTruncated)
					}
					if strings.Contains(part, "歳") || strings.Contains(part, "証明") || strings.Contains(part, "搭乗") || strings.Contains(part, "対象") || strings.Contains(part, "限定") {
						addEvidence(&t.Eligibility, part, &t.EvidenceTruncated)
					}
					if strings.Contains(part, "別途") || strings.Contains(part, "別に") || strings.Contains(part, "料金が必要") || strings.Contains(part, "特急券") || (strings.Contains(part, "指定席券") && !strings.Contains(part, "券売機")) {
						addEvidence(&t.Supplements, part, &t.EvidenceTruncated)
					}
					if strings.Contains(part, "できません") || strings.Contains(part, "なれません") || strings.Contains(part, "除く") || strings.Contains(part, "ただし") || strings.Contains(part, "終了") || strings.Contains(part, "限り") {
						addEvidence(&t.Exceptions, part, &t.EvidenceTruncated)
					}
					if strings.Contains(part, "ご利用券") || strings.Contains(part, "円分") {
						for _, v := range money(part, "benefit_not_ticket_price") {
							if len(t.Benefits) < 12 {
								t.Benefits = append(t.Benefits, v)
							}
						}
					}
					if strings.Contains(part, "利用できません") || strings.Contains(part, "利用になれません") {
						for _, v := range spanRE.FindAllStringSubmatch(part, -1) {
							sm, _ := strconv.Atoi(v[1])
							sd, _ := strconv.Atoi(v[2])
							em := sm
							if v[3] != "" {
								em, _ = strconv.Atoi(v[3])
							}
							ed, _ := strconv.Atoi(v[4])
							if sm >= 1 && sm <= 12 && em >= 1 && em <= 12 && sd >= 1 && sd <= 31 && ed >= 1 && ed <= 31 {
								t.BlackoutSpans = append(t.BlackoutSpans, MonthDaySpan{StartMonth: sm, StartDay: sd, EndMonth: em, EndDay: ed, Year: nil, Evidence: clip(part)})
							}
						}
					}
				}
			}
		}
		if n.Data == "p" && hasClass(n, "btn-01") {
			walk(n, func(x *html.Node) {
				if x.Data != "a" {
					return
				}
				u, e := url.Parse(attr(x, "href"))
				if e == nil && u.Scheme == "https" && u.Host != "" && u.Host != "www.toretabi.jp" && u.User == nil {
					t.OperatorURLs = append(t.OperatorURLs, u.String())
				}
			})
		}
	})
	t.OperatorURLs = unique(t.OperatorURLs)
	for i := range t.Eligibility {
		raw := t.Eligibility[i].TextJA
		scope := "unspecified_condition"
		if strings.Contains(raw, "ご利用券") || strings.Contains(raw, "機内販売") {
			scope = "benefit"
		} else if strings.Contains(raw, "25歳") || strings.Contains(raw, "U25") {
			scope = "purchase_age"
		}
		t.Eligibility[i].Scope = &scope
	}
	if t.Sales.Evidence.TextJA == "" && t.Use.Evidence.TextJA == "" && t.Description.TextJA == "" {
		return Ticket{}, fmt.Errorf("ticket facts missing; source contract changed")
	}
	if len(t.BlackoutSpans) > 0 {
		t.Use.Conditional = true
	}
	return t, nil
}

// CheckPeriod checks published date evidence only; it never establishes inventory or eligibility.
func CheckPeriod(p Period, on string) DateCheck {
	out := DateCheck{Date: strptr(on), State: "unknown", Reason: "No date requested or no sufficient explicit date evidence.", Evidence: p.Evidence}
	if on == "" {
		return out
	}
	if p.End != nil && on > *p.End {
		out.State = "closed"
		out.Reason = "After the explicit published period end."
		return out
	}
	if p.Start != nil && on < *p.Start {
		out.State = "not_started"
		out.Reason = "Before the explicit published period start."
		return out
	}
	inside := false
	for _, v := range p.Intervals {
		if on >= v.Start && on <= v.End {
			inside = true
		}
	}
	if len(p.Intervals) > 0 && !inside && p.Start == nil && p.End == nil {
		out.State = "outside_published_intervals"
		out.Reason = "Outside all explicit published intervals."
		return out
	}
	if p.Conditional {
		out.State = "conditional"
		out.Reason = "Published weekday, holiday, sale lead-time or blackout rules need manual/operator confirmation."
		return out
	}
	if inside || p.YearRound || (p.End != nil || p.Start != nil) {
		out.State = "within_published_window"
		out.Reason = "Within the known published bounds; missing bounds, other conditions and operator freshness remain unconfirmed."
	}
	return out
}
func Compare(t Ticket, useOn, asOf string) Comparison {
	c := Comparison{OperatorSalesCheck: CheckPeriod(period(""), asOf), OperatorUseCheck: CheckPeriod(period(""), useOn), OperatorEditionState: "not_checked", Ticket: t, SalesCheck: CheckPeriod(t.Sales, asOf), UseCheck: CheckPeriod(t.Use, useOn), EditionState: "undated", ConfirmedEligibility: "unknown"}
	if t.Use.End != nil {
		c.EditionState = "published_dated_window"
		if asOf != "" && asOf > *t.Use.End {
			c.EditionState = "archived_use_window"
		}
	}
	if t.Operator != nil {
		c.OperatorSalesCheck = CheckPeriod(t.Operator.Sales, asOf)
		c.OperatorUseCheck = CheckPeriod(t.Operator.Use, useOn)
		c.OperatorEditionState = operatorEdition(*t.Operator, asOf)
	}
	return c
}
