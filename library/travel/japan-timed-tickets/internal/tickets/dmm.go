// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package tickets

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	dmmBase     = "https://teamlabplanets.dmm.com"
	dmmEntryURL = dmmBase + "/ticket"
	// dmmTicketENURL is the English store page used as the handoff link.
	dmmTicketENURL = dmmBase + "/en/ticket"
)

var (
	reDMMCell    = regexp.MustCompile(`(?s)<td class="(p-purchaseCalendar_[A-Za-z]+[^"]*)">\s*<input type="radio" name="calendar" value="(\d{8})"(.*?)</td>`)
	reDMMPrice   = regexp.MustCompile(`p-purchaseCalendar_itemPrice">\s*([\d,]+)`)
	reDMMToken   = regexp.MustCompile(`/ticket/admission_date/([a-f0-9]{16,64})`)
	reDMMRelease = regexp.MustCompile(`(\d{1,2})月のチケットは\s*(\d{1,2})月\s*(上旬|中旬|下旬|末頃|末|(\d{1,2})日)[^。<\n]*?販売予定`)
)

// DMMCalendar is the parsed DMM purchase calendar.
type DMMCalendar struct {
	Days    map[string]Day
	Last    *Date
	Release *Release
}

// ParseDMMCalendar reads the DMM admission-date page.
func ParseDMMCalendar(page string, now time.Time) DMMCalendar {
	cal := DMMCalendar{Days: map[string]Day{}}
	var last Date
	for _, m := range reDMMCell.FindAllStringSubmatch(page, -1) {
		class, raw, inner := m[1], m[2], m[3]
		y, _ := strconv.Atoi(raw[:4])
		mo, _ := strconv.Atoi(raw[4:6])
		dd, _ := strconv.Atoi(raw[6:8])
		d := Date{y, mo, dd}
		key := d.String()
		day := Day{Date: key}
		switch {
		case strings.HasPrefix(class, "p-purchaseCalendar_noStock"):
			day.Status, day.Reason = DayClosed, "休館日 closed (official store calendar)"
		case strings.HasPrefix(class, "p-purchaseCalendar_close"):
			day.Status, day.Reason = DayPast, "not selectable (past date)"
		case strings.Contains(inner, "is-soldOut") || strings.Contains(class, "soldOut"):
			day.Status = DaySoldOut
		case strings.Contains(inner, "is-few") || strings.Contains(class, "is-few"):
			day.Status = DayFew
		case strings.HasPrefix(class, "p-purchaseCalendar_available"):
			day.Status = DayAvailable
		default:
			day.Status, day.Reason = DayUnknown, "unrecognized calendar cell: "+StripControl(class)
		}
		if pm := reDMMPrice.FindStringSubmatch(inner); pm != nil {
			if v, err := strconv.Atoi(strings.ReplaceAll(pm[1], ",", "")); err == nil {
				day.PriceFrom = intPtr(v)
			}
		}
		cal.Days[key] = day
		if last.Before(d) {
			last = d
		}
	}
	if last.Y > 0 {
		cal.Last = &last
	}
	text := halfWidth(strings.Join(htmlText(page), " "))
	if m := reDMMRelease.FindStringSubmatch(text); m != nil {
		cal.Release = dmmRelease(m, now)
	}
	return cal
}

func dmmRelease(m []string, now time.Time) *Release {
	coverMonth, _ := strconv.Atoi(m[1])
	saleMonth, _ := strconv.Atoi(m[2])
	ref := now.In(JST)
	y := InferYear(saleMonth, ref.Year(), int(ref.Month()))
	cy := InferYear(coverMonth, y, saleMonth)
	first := Date{y, saleMonth, 1}
	lastDay := DateOf(first.Time(12, 0).AddDate(0, 1, -1))
	lo, hi := 1, lastDay.D
	switch m[3] {
	case "上旬":
		lo, hi = 1, 10
	case "中旬":
		lo, hi = 11, 20
	case "下旬", "末頃", "末":
		lo, hi = 21, lastDay.D
	default:
		if m[4] != "" {
			v, _ := strconv.Atoi(m[4])
			lo, hi = v, v
		}
	}
	rel := &Release{
		Covers:      Date{cy, coverMonth, 1}.Month(),
		EarliestDay: Date{y, saleMonth, lo}.String(),
		LatestDay:   Date{y, saleMonth, hi}.String(),
		Confidence:  ConfEstimated,
		Text:        strings.TrimSpace(m[0]),
		SourceURL:   dmmEntryURL,
		Tentative:   true,
	}
	if lo == hi {
		rel.Confidence = ConfAnnounced
	}
	return rel
}

// ParseDMMSlots reads the DMM admission_time_list JSON.
func ParseDMMSlots(raw []byte, date Date) ([]Slot, error) {
	var resp struct {
		TicketTypes map[string]struct {
			TicketTypeID int `json:"ticket_type_id"`
			EntryTimes   []struct {
				Start    string `json:"start_time"`
				End      string `json:"end_time"`
				MaxStock *int   `json:"max_stock"`
				InStock  *int   `json:"instock"`
				Info     struct {
					Attributes []struct {
						Name struct {
							EN string `json:"text_en"`
						} `json:"attribute_name"`
						UnitPrice string `json:"unit_price"`
					} `json:"attribute_infos"`
				} `json:"ticket_infos"`
			} `json:"entry_times"`
		} `json:"ticket_types"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("admission_time_list: %w", err)
	}
	keys := make([]string, 0, len(resp.TicketTypes))
	for k := range resp.TicketTypes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]Slot, 0)
	for _, k := range keys {
		tt := resp.TicketTypes[k]
		product := dmmProductName(tt.TicketTypeID)
		for _, e := range tt.EntryTimes {
			status := SlotAvailable
			switch {
			case e.InStock == nil:
				status = SlotUnknown
			case *e.InStock <= 0:
				status = SlotSoldOut
			}
			var adult *int
			for _, a := range e.Info.Attributes {
				if strings.HasPrefix(strings.ToLower(a.Name.EN), "adult") {
					if v, err := strconv.Atoi(a.UnitPrice); err == nil {
						adult = intPtr(v)
					}
					break
				}
			}
			startJST := ""
			if hh, mm, ok := parseHHMM(e.Start); ok {
				startJST = FormatJST(date.Time(hh, mm))
			}
			out = append(out, Slot{Date: date.String(), Product: product, Start: StripControl(e.Start), End: StripControl(e.End), StartJST: startJST, Status: status, Stock: e.InStock, Capacity: e.MaxStock, AdultPriceJPY: adult})
		}
	}
	return out, nil
}

func dmmProductName(id int) string {
	switch id {
	case 1:
		return "Entrance Pass"
	case 3:
		return "Premium Pass"
	}
	return fmt.Sprintf("ticket type %d", id)
}

func parseHHMM(s string) (int, int, bool) {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) < 2 {
		return 0, 0, false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	return h, m, err1 == nil && err2 == nil
}

func readDMM(ctx context.Context, f *Fetcher, sd *SightData, from, to Date, slotDates []Date, now func() time.Time) {
	// The store issues a calendar page token through redirects; keep the final URL.
	final, page, err := f.FinalURL(ctx, dmmEntryURL)
	if err != nil {
		sd.failed(dmmEntryURL, err, now())
		sd.warn("DMM store calendar unavailable: " + err.Error())
		return
	}
	sd.addSource(dmmEntryURL, true, "", now())
	cal := ParseDMMCalendar(string(page), now())
	sd.CalendarUntil = cal.Last
	sd.Release = cal.Release
	sd.RuleConfirmed = cal.Last != nil
	if cal.Last == nil {
		sd.warn("DMM store calendar had no dates; the page layout may have changed")
	}
	for _, d := range DateRange(from, to) {
		if day, ok := cal.Days[d.String()]; ok {
			sd.Days[d.String()] = day
		}
	}
	if len(slotDates) == 0 {
		return
	}
	m := reDMMToken.FindStringSubmatch(final)
	if m == nil {
		sd.warn("DMM store did not issue a calendar page token; slot stock unavailable")
		return
	}
	listURL := fmt.Sprintf("%s/ticket/admission_time_list/%s", dmmBase, m[1])
	for _, d := range slotDates {
		day, ok := sd.Days[d.String()]
		if !ok || day.Status == DayClosed || day.Status == DayPast {
			continue
		}
		raw, err := f.PostFormRead(ctx, listURL, url.Values{"entry_date": {fmt.Sprintf("%04d%02d%02d", d.Y, d.M, d.D)}}, final)
		if err != nil {
			sd.failed(dmmBase+"/ticket/admission_time_list", err, now())
			sd.warn(fmt.Sprintf("slots for %s unavailable: %v", d, err))
			continue
		}
		sd.addSource(dmmBase+"/ticket/admission_time_list", true, "read-only time list for "+d.String(), now())
		slots, err := ParseDMMSlots(raw, d)
		if err != nil {
			sd.warn(err.Error())
			continue
		}
		sd.Slots = append(sd.Slots, slots...)
	}
}
