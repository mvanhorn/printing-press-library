// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package tickets

import (
	"context"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	ghibliTicketsURL  = "https://www.ghibli-museum.jp/en/tickets/"
	lawsonGhibliURL   = "https://l-tike.com/ghibli/"
	lawsonGhibliQuery = "三鷹の森ジブリ美術館"
	// ghibliMaxTickets is the Lawson Ticket limit per account per month.
	ghibliMaxTickets = 6
)

var (
	reComment     = regexp.MustCompile(`(?s)<!--.*?-->`)
	reScriptStyle = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	reTag         = regexp.MustCompile(`<[^>]+>`)
	reMonthHeader = regexp.MustCompile(`<h3>\s*(January|February|March|April|May|June|July|August|September|October|November|December)\s+(\d{4})\s*</h3>`)
	reCell        = regexp.MustCompile(`(?s)<td class="([^"]*)">\s*(\d{1,2})?\s*</td>`)
	reGhibliRule  = regexp.MustCompile(`(?i)10\s*a\.?m\.?\s*\(JST\)\s*on the 10th of each month`)
)

var monthNames = map[string]int{"January": 1, "February": 2, "March": 3, "April": 4, "May": 5, "June": 6, "July": 7, "August": 8, "September": 9, "October": 10, "November": 11, "December": 12}

// htmlText strips comments, scripts, styles, tags and control characters and
// returns trimmed non-empty lines.
func htmlText(raw string) []string {
	s := reComment.ReplaceAllString(raw, "")
	s = reScriptStyle.ReplaceAllString(s, "")
	s = reTag.ReplaceAllString(s, "\n")
	s = StripControl(html.UnescapeString(s))
	out := make([]string, 0)
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// cellText returns the clean text of one HTML fragment: tags removed,
// entities decoded, control characters dropped, spaces trimmed.
func cellText(s string) string {
	return strings.TrimSpace(StripControl(html.UnescapeString(reTag.ReplaceAllString(s, ""))))
}

// halfWidth converts full-width digits and slashes to ASCII.
func halfWidth(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= '０' && r <= '９':
			return '0' + (r - '０')
		case r == '／':
			return '/'
		case r == '：':
			return ':'
		}
		return r
	}, s)
}

// ParseGhibliCalendar reads the museum calendar. It returns closed and known
// (open or closed) dates keyed YYYY-MM-DD, and whether the published sale rule
// text is present.
func ParseGhibliCalendar(page string) (closed map[string]bool, known map[string]bool, ruleFound bool) {
	closed = map[string]bool{}
	known = map[string]bool{}
	ruleFound = reGhibliRule.MatchString(page)
	locs := reMonthHeader.FindAllStringSubmatchIndex(page, -1)
	for i, loc := range locs {
		month := monthNames[page[loc[2]:loc[3]]]
		year, _ := strconv.Atoi(page[loc[4]:loc[5]])
		end := len(page)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		section := page[loc[1]:end]
		if j := strings.Index(section, "</table>"); j >= 0 {
			section = section[:j]
		}
		for _, m := range reCell.FindAllStringSubmatch(section, -1) {
			if m[2] == "" {
				continue
			}
			day, _ := strconv.Atoi(m[2])
			key := Date{year, month, day}.String()
			known[key] = true
			if strings.Contains(m[1], "close") {
				closed[key] = true
			}
		}
	}
	return closed, known, ruleFound
}

var (
	reLawsonBlock     = regexp.MustCompile(`(\d{4})/(\d{1,2})/(\d{1,2})\([^)]*\)\s*～\s*(\d{1,2})/(\d{1,2})\([^)]*\)\s*入場分`)
	reLawsonOverride  = regexp.MustCompile(`※\s*(\d{1,2})月入場券は\s*(\d{1,2})/(\d{1,2})\([^)]*\)\s*発売`)
	reLawsonCitizen   = regexp.MustCompile(`※\s*((?:\d{1,2}/\d{1,2}\([^)]*\)[・、,\s]*)+)は市民デーの為`)
	reMonthDayPairs   = regexp.MustCompile(`(\d{1,2})/(\d{1,2})`)
	reLawsonGeneralTm = regexp.MustCompile(`毎月(\d{1,2})日(\d{1,2}):(\d{2})より、翌月入場分`)
)

// LawsonGhibliNotes holds exceptions from the Lawson Ticket Ghibli page.
type LawsonGhibliNotes struct {
	// SaleDate maps an entry month (YYYY-MM) to its announced sale date.
	SaleDate map[string]Date
	// NoGeneralSale maps a visit date to the published reason.
	NoGeneralSale map[string]string
	// GeneralHour and GeneralMinute come from "毎月10日10:00より" when present.
	GeneralHour, GeneralMinute int
	RuleFound                  bool
}

// ParseLawsonGhibli reads the Lawson Ticket Ghibli page (comments removed).
func ParseLawsonGhibli(page string) LawsonGhibliNotes {
	notes := LawsonGhibliNotes{SaleDate: map[string]Date{}, NoGeneralSale: map[string]string{}, GeneralHour: 10}
	lines := htmlText(page)
	joined := halfWidth(strings.Join(lines, "\n"))
	if m := reLawsonGeneralTm.FindStringSubmatch(strings.ReplaceAll(joined, "\n", "")); m != nil {
		notes.RuleFound = m[1] == "10"
		notes.GeneralHour, _ = strconv.Atoi(m[2])
		notes.GeneralMinute, _ = strconv.Atoi(m[3])
	}
	// Walk the text, tracking the current entry-month block.
	blockYear, blockMonth := 0, 0
	text := strings.ReplaceAll(joined, "\n", " ")
	idx := 0
	for idx < len(text) {
		nextBlock := reLawsonBlock.FindStringSubmatchIndex(text[idx:])
		end := len(text)
		if nextBlock != nil {
			end = idx + nextBlock[0]
		}
		segment := text[idx:end]
		if blockYear > 0 {
			for _, m := range reLawsonOverride.FindAllStringSubmatch(segment, -1) {
				em, _ := strconv.Atoi(m[1])
				sm, _ := strconv.Atoi(m[2])
				sd, _ := strconv.Atoi(m[3])
				ey := InferYear(em, blockYear, blockMonth)
				sy := InferYear(sm, ey, em)
				notes.SaleDate[Date{ey, em, 1}.Month()] = Date{sy, sm, sd}
			}
			for _, m := range reLawsonCitizen.FindAllStringSubmatch(segment, -1) {
				for _, p := range reMonthDayPairs.FindAllStringSubmatch(m[1], -1) {
					mm, _ := strconv.Atoi(p[1])
					dd, _ := strconv.Atoi(p[2])
					y := InferYear(mm, blockYear, blockMonth)
					notes.NoGeneralSale[Date{y, mm, dd}.String()] = "市民デー (citizens' day): no general sale"
				}
			}
		}
		if nextBlock == nil {
			break
		}
		m := text[idx+nextBlock[0] : idx+nextBlock[1]]
		sub := reLawsonBlock.FindStringSubmatch(m)
		blockYear, _ = strconv.Atoi(sub[1])
		blockMonth, _ = strconv.Atoi(sub[2])
		idx += nextBlock[1]
	}
	return notes
}

var (
	reResultTitle  = regexp.MustCompile(`(?s)class="ResultBox__title">(.*?)</h3>`)
	reResultDates  = regexp.MustCompile(`(\d{4})/(\d{1,2})/(\d{1,2})\([^)]*\)\s*～\s*(\d{4})/(\d{1,2})/(\d{1,2})\([^)]*\)`)
	reResultStatus = regexp.MustCompile(`(?s)class="[^"]*ResultBox__status[^"]*">\s*(.*?)\s*</p>`)
	reResultWindow = regexp.MustCompile(`(\d{4})/(\d{1,2})/(\d{1,2})\([^)]*\)\s*(\d{1,2}):(\d{2})\s*～\s*(\d{4})/(\d{1,2})/(\d{1,2})\([^)]*\)\s*(\d{1,2}):(\d{2})`)
)

// ParseLawsonSearch reads Lawson Ticket search results for the museum.
func ParseLawsonSearch(page string) []SaleWindow {
	page = reComment.ReplaceAllString(page, "")
	out := make([]SaleWindow, 0)
	parts := strings.Split(page, `<div class="ResultBox boxContents`)
	for i, body := range parts {
		if i == 0 {
			continue
		}
		if j := strings.Index(body, `id="layout_search_result_artist"`); j >= 0 {
			body = body[:j]
		}
		title := ""
		if m := reResultTitle.FindStringSubmatch(body); m != nil {
			title = cellText(m[1])
		}
		if !strings.Contains(title, "ジブリ美術館") {
			continue
		}
		bodyHW := halfWidth(body)
		dm := reResultDates.FindStringSubmatch(bodyHW)
		wm := reResultWindow.FindStringSubmatch(bodyHW)
		if dm == nil || wm == nil {
			continue
		}
		atoi := func(s string) int { v, _ := strconv.Atoi(s); return v }
		first := Date{atoi(dm[1]), atoi(dm[2]), atoi(dm[3])}
		last := Date{atoi(dm[4]), atoi(dm[5]), atoi(dm[6])}
		opens := Date{atoi(wm[1]), atoi(wm[2]), atoi(wm[3])}.Time(atoi(wm[4]), atoi(wm[5]))
		closes := Date{atoi(wm[6]), atoi(wm[7]), atoi(wm[8])}.Time(atoi(wm[9]), atoi(wm[10]))
		status := ""
		if m := reResultStatus.FindStringSubmatch(body); m != nil {
			status = cellText(m[1])
		}
		out = append(out, SaleWindow{FirstVisit: first.String(), LastVisit: last.String(), Opens: &opens, Closes: &closes, Status: status, Label: halfWidth(title), SourceURL: lawsonSearchURL()})
	}
	return out
}

func lawsonSearchURL() string {
	return "https://l-tike.com/search/?keyword=" + url.QueryEscape(lawsonGhibliQuery)
}

// GhibliSaleMoment applies the published rule: 10:00 JST on the 10th of the
// month before the visit month.
func GhibliSaleMoment(visit Date) time.Time {
	first := time.Date(visit.Y, time.Month(visit.M), 1, 10, 0, 0, 0, JST)
	prev := first.AddDate(0, -1, 0)
	return time.Date(prev.Year(), prev.Month(), 10, 10, 0, 0, 0, JST)
}

func readGhibli(ctx context.Context, f *Fetcher, sd *SightData, from, to Date, now func() time.Time) {
	if page, err := sd.read(ctx, f, ghibliTicketsURL, false, now); err != nil {
		sd.warn("museum calendar unavailable: " + err.Error())
	} else {
		closed, known, rule := ParseGhibliCalendar(string(page))
		sd.RuleConfirmed = rule
		if !rule {
			sd.warn("the museum page no longer shows the '10 a.m. (JST) on the 10th of each month' rule text; sale moments are estimated")
		}
		if len(known) == 0 {
			sd.warn("the museum calendar had no dates; the page layout may have changed")
		}
		for _, d := range DateRange(from, to) {
			key := d.String()
			switch {
			case closed[key]:
				sd.Days[key] = Day{Date: key, Status: DayClosed, Reason: "museum closed (official calendar)"}
			case known[key]:
				sd.Days[key] = Day{Date: key, Status: DayUnknown, Reason: "museum open; per-date stock needs a Lawson Ticket member login"}
			}
		}
	}
	if lawson, err := sd.read(ctx, f, lawsonGhibliURL, false, now); err != nil {
		sd.warn("Lawson Ticket Ghibli page unavailable; monthly exceptions not checked: " + err.Error())
	} else {
		notes := ParseLawsonGhibli(string(lawson))
		if !notes.RuleFound {
			sd.warn("the Lawson Ticket Ghibli page no longer shows the monthly 10th-of-the-month sale notice")
		}
		for month, d := range notes.SaleDate {
			sd.SaleOverrides[month] = d.Time(notes.GeneralHour, notes.GeneralMinute)
		}
		for key, reason := range notes.NoGeneralSale {
			if d, err := ParseDate(key); err == nil && !d.Before(from) && !to.Before(d) {
				if cur, ok := sd.Days[key]; !ok || cur.Status != DayClosed {
					sd.Days[key] = Day{Date: key, Status: DayNoGeneralSale, Reason: reason}
				}
			}
		}
	}
	if search, err := sd.read(ctx, f, lawsonSearchURL(), false, now); err != nil {
		sd.warn("Lawson Ticket listing unavailable; sale windows not checked: " + err.Error())
	} else {
		sd.Windows = ParseLawsonSearch(string(search))
	}
}
