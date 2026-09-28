package walkerplus

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var japaneseDateRE = regexp.MustCompile(`(?:([12][0-9]{3})年)?(?:([0-9]{1,2})月)?([0-9]{1,2})日`)
var exclusionRE = regexp.MustCompile(`(?:([12][0-9]{3})年)?([0-9]{1,2})月([0-9]{1,2})日(?:[（(][^）)]*[）)])?(?:は|を)?(?:除く|休館|休み|中止)`)

const weekdayListPattern = "(?:毎週)?[月火水木金土日](?:曜(?:日)?)?(?:(?:[・、/／ ]*|[ ]*(?:と|および|及び|ならびに|並びに)[ ]*)(?:毎週)?[月火水木金土日](?:曜(?:日)?)?)*"

var weeklyRE = regexp.MustCompile("毎週(" + weekdayListPattern + ")")
var dailyRE = regexp.MustCompile("毎日(?:[^曜]|$)|連日開催")
var bareWeeklyRE = regexp.MustCompile("毎(" + weekdayListPattern + "曜(?:日)?)(?:は|に|のみ|だけ)?(?:開催|実施|開館|営業)")
var onlyWeekdaysRE = regexp.MustCompile("(" + weekdayListPattern + ")(?:のみ|だけ)(?:開催|実施)")
var closedRE = regexp.MustCompile("(?:休館日|休園日|休業日|休み|定休日)[：: ]*(" + weekdayListPattern + ")|(" + weekdayListPattern + ")(?:[（(][^）)]*[）)])?[ ]*(?:は|が|を)?[ ]*(?:休館|休園|休業|休み|定休|除く)")
var nonExhaustiveWeekdaysRE = regexp.MustCompile("[月火水木金土日](?:曜(?:日)?)?[ ]*や[ ]*[月火水木金土日](?:曜(?:日)?)?")
var monthlyWeekdayRE = regexp.MustCompile("毎月(?:第?[0-9０-９一二三四五]+(?:[・、](?:第)?[0-9０-９一二三四五]+)*(?:週)?)?(?:[0-9０-９]+日|[月火水木金土日](?:曜(?:日)?)?)?(?:[（(][^）)]*[）)])?|第[0-9０-９一二三四五]+(?:[・、](?:第)?[0-9０-９一二三四五]+)*(?:週)?[月火水木金土日](?:曜(?:日)?)?")
var calendarWeekdayRE = regexp.MustCompile(japaneseDateRE.String() + "(?:[（(][月火水木金土日](?:曜(?:日)?)?[）)])?")
var weekdayNames = []string{"日", "月", "火", "水", "木", "金", "土"}

func weekdayTokens(names string) []time.Weekday {
	// 日 in the grammar suffix 曜日 is not a second Sunday token.
	names = strings.NewReplacer("毎週", "", "曜日", "", "曜", "").Replace(names)
	out := []time.Weekday{}
	for _, r := range names {
		for i, name := range weekdayNames {
			if string(r) == name && !containsWeekday(out, time.Weekday(i)) {
				out = append(out, time.Weekday(i))
			}
		}
	}
	return out
}

func adjacentNonExhaustiveWeekday(text string, start, end int) bool {
	return strings.HasSuffix(strings.TrimRight(text[:start], " "), "や") || strings.HasPrefix(strings.TrimLeft(text[end:], " "), "や")
}

func supportedWeekdayText(raw string) (string, bool) {
	var out strings.Builder
	last := 0
	unsupported := false
	for _, span := range monthlyWeekdayRE.FindAllStringIndex(raw, -1) {
		// 毎月曜 is an explicit weekday; 毎月第1日曜 is monthly/nth-week.
		if strings.HasPrefix(raw[span[0]:], "毎月曜") {
			continue
		}
		out.WriteString(raw[last:span[0]])
		out.WriteByte(' ')
		last = span[1]
		unsupported = true
	}
	out.WriteString(raw[last:])
	return out.String(), unsupported
}

func parseWeekdayRules(e *Event, raw string) {
	text, unsupported := supportedWeekdayText(raw)
	if unsupported {
		e.Schedule.Unresolved = appendUnique(e.Schedule.Unresolved, "monthly or nth-week recurrence is unsupported")
	}
	// Calendar dates/annotations are not standalone weekday recurrence tokens.
	text = calendarWeekdayRE.ReplaceAllString(text, " ")
	if nonExhaustiveWeekdaysRE.MatchString(text) {
		e.Schedule.Unresolved = appendUnique(e.Schedule.Unresolved, "non-exhaustive weekday list is unresolved")
	}
	var positive strings.Builder
	last := 0
	for _, m := range closedRE.FindAllStringSubmatchIndex(text, -1) {
		if adjacentNonExhaustiveWeekday(text, m[0], m[1]) {
			continue
		}
		if m[0] > 0 {
			r, _ := utf8.DecodeLastRuneInString(text[:m[0]])
			if unicode.IsDigit(r) {
				continue
			}
		}
		start, end := m[2], m[3]
		if start < 0 {
			start, end = m[4], m[5]
		}
		for _, day := range weekdayTokens(text[start:end]) {
			if !containsWeekday(e.Schedule.closed, day) {
				e.Schedule.closed = append(e.Schedule.closed, day)
				e.Schedule.ClosedWeekdays = append(e.Schedule.ClosedWeekdays, weekdayNames[int(day)]+"曜")
			}
		}
		positive.WriteString(text[last:m[0]])
		positive.WriteByte(' ')
		last = m[1]
	}
	positive.WriteString(text[last:])
	text = positive.String()
	if dailyRE.MatchString(text) {
		e.Schedule.daily = true
		e.Schedule.Recurrence = strptr("daily")
	}
	for _, pattern := range []*regexp.Regexp{weeklyRE, bareWeeklyRE, onlyWeekdaysRE} {
		for _, m := range pattern.FindAllStringSubmatchIndex(text, -1) {
			if adjacentNonExhaustiveWeekday(text, m[0], m[1]) {
				continue
			}
			for _, day := range weekdayTokens(text[m[2]:m[3]]) {
				if !containsWeekday(e.Schedule.weekdays, day) {
					e.Schedule.weekdays = append(e.Schedule.weekdays, day)
				}
			}
		}
	}
	if !e.Schedule.daily && len(e.Schedule.weekdays) > 0 {
		e.Schedule.Recurrence = strptr("weekly")
	}
}

func sourceDates(raw string, start, end *string) []string {
	result := []string{}
	year, month := 0, 0
	if start != nil {
		t, _ := parseDate(*start)
		year = t.Year()
		month = int(t.Month())
	}
	for _, m := range japaneseDateRE.FindAllStringSubmatch(raw, -1) {
		if m[1] != "" {
			year, _ = strconv.Atoi(m[1])
		}
		if m[2] != "" {
			month, _ = strconv.Atoi(m[2])
		}
		day, _ := strconv.Atoi(m[3])
		if year == 0 || month == 0 {
			continue
		}
		s := fmt.Sprintf("%04d-%02d-%02d", year, month, day)
		// Omitted years are resolved only within the explicit source envelope.
		if m[1] == "" && start != nil && end != nil && s < *start {
			next := fmt.Sprintf("%04d-%02d-%02d", year+1, month, day)
			if next >= *start && next <= *end {
				year++
				s = next
			}
		}
		if _, err := parseDate(s); err == nil {
			result = appendUnique(result, s)
		}
	}
	return result
}

var primaryRangeRE = regexp.MustCompile("^(.*?[～〜~]\\s*(?:[12][0-9]{3}年)?(?:[0-9]{1,2}月)?[0-9]{1,2}日)")

func parseSchedule(e *Event) {
	raw := value(e.Schedule.Raw)
	e.Schedule.OccurrenceDates = []string{}
	e.Schedule.ExcludedDates = []string{}
	e.Schedule.ClosedWeekdays = []string{}
	e.Schedule.Unresolved = []string{}
	e.Schedule.weekdays = nil
	e.Schedule.closed = nil
	e.Schedule.daily = false
	e.Schedule.Recurrence = nil
	if raw == "" {
		e.Schedule.Unresolved = append(e.Schedule.Unresolved, "source schedule is absent")
		return
	}
	dates := sourceDates(raw, e.StartDate, e.EndDate)
	if e.StartDate == nil && len(dates) > 0 {
		primary := dates
		if strings.ContainsAny(raw, "～〜~") {
			primary = nil
			if m := primaryRangeRE.FindStringSubmatch(raw); m != nil {
				primary = sourceDates(m[1], nil, nil)
			}
		}
		if len(primary) > 0 {
			e.StartDate = strptr(primary[0])
			e.EndDate = strptr(primary[len(primary)-1])
		}
	}
	setDateCertainty(e)
	if approximate(raw) {
		e.Schedule.Unresolved = append(e.Schedule.Unresolved, "source uses approximate or provisional dates")
	}
	holiday := strings.Contains(raw, "祝日") || strings.Contains(raw, "祝の場合") || strings.Contains(raw, "祝の場合は") || strings.Contains(raw, "祝日の場合") || strings.Contains(raw, "翌平日") || strings.Contains(raw, "祝を除")
	if holiday {
		e.Schedule.Unresolved = append(e.Schedule.Unresolved, "holiday-dependent schedule cannot be resolved without source holiday dates")
	}
	for _, qualifier := range []string{"不定休", "臨時休館", "臨時休業", "変更あり", "変更する場合", "休館する場合", "開催未定", "詳細は", "お問い合わせ"} {
		if strings.Contains(raw, qualifier) {
			e.Schedule.Unresolved = appendUnique(e.Schedule.Unresolved, "unsupported schedule qualifier: "+qualifier)
		}
	}
	parseWeekdayRules(e, raw)
	for _, m := range exclusionRE.FindAllStringSubmatch(raw, -1) {
		year := 0
		if e.EditionYear != nil {
			year = *e.EditionYear
		}
		if m[1] != "" {
			year, _ = strconv.Atoi(m[1])
		}
		month, _ := strconv.Atoi(m[2])
		day, _ := strconv.Atoi(m[3])
		s := fmt.Sprintf("%04d-%02d-%02d", year, month, day)
		if m[1] == "" && e.StartDate != nil && e.EndDate != nil && s < *e.StartDate {
			next := fmt.Sprintf("%04d-%02d-%02d", year+1, month, day)
			if next >= *e.StartDate && next <= *e.EndDate {
				s = next
			}
		}
		if _, err := parseDate(s); err == nil {
			e.Schedule.ExcludedDates = appendUnique(e.Schedule.ExcludedDates, s)
		}
	}
	hasRange := strings.ContainsAny(raw, "～〜~") || strings.Contains(raw, "から")
	if !hasRange && len(dates) > 0 && len(e.Schedule.closed) == 0 && len(e.Schedule.ExcludedDates) == 0 {
		e.Schedule.OccurrenceDates = dates
	}
	if len(e.Schedule.weekdays) == 0 && !e.Schedule.daily && len(e.Schedule.OccurrenceDates) == 0 && len(e.Schedule.closed) == 0 && len(e.Schedule.Unresolved) == 0 {
		e.Schedule.Unresolved = append(e.Schedule.Unresolved, "overall date range does not prove daily activity")
	}
}

func containsDate(items []string, s string) bool {
	for _, v := range items {
		if v == s {
			return true
		}
	}
	return false
}
func containsWeekday(items []time.Weekday, w time.Weekday) bool {
	for _, v := range items {
		if v == w {
			return true
		}
	}
	return false
}

func matchEvent(e Event, q Query, enriched bool) *Match {
	m := &Match{Timing: q.Timing, State: "possible", ConfirmedDays: []string{}, PossibleDays: []string{}, Reasons: []string{}}
	if q.From == "" {
		m.Reasons = append(m.Reasons, "no trip date filter")
		return m
	}
	from, _ := parseDate(q.From)
	to, _ := parseDate(q.To)
	if e.StartDate == nil || e.EndDate == nil {
		m.Reasons = append(m.Reasons, "event edition dates are unknown")
		return m
	}
	start, _ := parseDate(*e.StartDate)
	end, _ := parseDate(*e.EndDate)
	if end.Before(from) || start.After(to) {
		m.State = "excluded"
		m.Reasons = append(m.Reasons, "source edition envelope does not overlap requested dates")
		return m
	}
	if q.Timing == "starts" && (start.Before(from) || start.After(to)) {
		m.State = "excluded"
		return m
	}
	if q.Timing == "ends" && (end.Before(from) || end.After(to)) {
		m.State = "excluded"
		return m
	}
	first, last := from, to
	if start.After(first) {
		first = start
	}
	if end.Before(last) {
		last = end
	}
	m.EnvelopeOverlapDays = int(last.Sub(first)/(24*time.Hour)) + 1
	m.Reasons = append(m.Reasons, "source edition envelope overlaps requested dates")
	if q.Timing != "overlap" {
		m.Reasons = append(m.Reasons, "timing tests source envelope "+q.Timing+" boundary")
	}
	for day := first; !day.After(last); day = day.AddDate(0, 0, 1) {
		s := day.Format("2006-01-02")
		if !enriched || e.DateCertainty != "exact" {
			m.PossibleDays = append(m.PossibleDays, s)
			continue
		}
		if containsDate(e.Schedule.ExcludedDates, s) {
			continue
		}
		if len(e.Schedule.Unresolved) > 0 {
			m.PossibleDays = append(m.PossibleDays, s)
			continue
		}
		if containsWeekday(e.Schedule.closed, day.Weekday()) {
			continue
		}
		if len(e.Schedule.OccurrenceDates) > 0 {
			if containsDate(e.Schedule.OccurrenceDates, s) {
				m.ConfirmedDays = append(m.ConfirmedDays, s)
			}
			continue
		}
		if e.Schedule.daily {
			m.ConfirmedDays = append(m.ConfirmedDays, s)
			continue
		}
		if len(e.Schedule.weekdays) > 0 {
			if containsWeekday(e.Schedule.weekdays, day.Weekday()) {
				m.ConfirmedDays = append(m.ConfirmedDays, s)
			}
			continue
		}
		m.PossibleDays = append(m.PossibleDays, s)
	}
	if len(m.ConfirmedDays) > 0 {
		m.State = "confirmed"
		m.Reasons = append(m.Reasons, "displayed schedule confirms at least one requested day")
	} else if len(m.PossibleDays) == 0 {
		m.State = "excluded"
		m.Reasons = append(m.Reasons, "displayed schedule excludes requested days")
	} else {
		m.Reasons = append(m.Reasons, "activity on overlapping days remains unverified")
	}
	return m
}

func categoryMatches(items []CatalogItem, requested string) bool {
	if requested == "" {
		return true
	}
	groups := map[string][]string{"eg0055": {"eg0135", "eg0140"}, "eg0051": {"eg0101", "eg0102", "eg0103", "eg0104", "eg0105", "eg0130", "eg0131", "eg0133", "eg0134", "eg0141", "eg0144"}, "eg0052": {"eg0106", "eg0115", "eg0117", "eg0118", "eg0145"}, "eg0053": {"eg0107", "eg0108", "eg0109", "eg0110", "eg0111", "eg0114", "eg0140"}, "eg0054": {"eg0120", "eg0124", "eg0125", "eg0126", "eg0127"}, "eg0056": {"eg0123"}}
	for _, c := range items {
		if c.Code == requested || containsDate(groups[requested], c.Code) {
			return true
		}
	}
	return false
}
func matchesLocation(e Event, q Query) bool {
	if q.Prefecture != "" && value(e.Location.PrefectureCode) != q.Prefecture {
		return false
	}
	if q.City != "" && value(e.Location.CityCode) != q.City {
		return false
	}
	return true
}
func sortEvents(events []Event, q Query) {
	if q.Sort == "source" {
		return
	}
	sort.SliceStable(events, func(i, j int) bool {
		a, b := events[i], events[j]
		if q.Sort == "relevance" {
			ac := a.Match != nil && a.Match.State == "confirmed"
			bc := b.Match != nil && b.Match.State == "confirmed"
			if ac != bc {
				return ac
			}
		}
		av, bv := value(a.StartDate), value(b.StartDate)
		if q.Sort == "relevance" {
			if a.Match != nil {
				if len(a.Match.ConfirmedDays) > 0 {
					av = a.Match.ConfirmedDays[0]
				} else if len(a.Match.PossibleDays) > 0 {
					av = a.Match.PossibleDays[0]
				}
			}
			if b.Match != nil {
				if len(b.Match.ConfirmedDays) > 0 {
					bv = b.Match.ConfirmedDays[0]
				} else if len(b.Match.PossibleDays) > 0 {
					bv = b.Match.PossibleDays[0]
				}
			}
		}
		if q.Sort == "end" {
			av, bv = value(a.EndDate), value(b.EndDate)
		}
		if av == "" {
			av = "9999"
		}
		if bv == "" {
			bv = "9999"
		}
		if av != bv {
			return av < bv
		}
		if value(a.EndDate) != value(b.EndDate) {
			return value(a.EndDate) < value(b.EndDate)
		}
		return a.ID < b.ID
	})
}
