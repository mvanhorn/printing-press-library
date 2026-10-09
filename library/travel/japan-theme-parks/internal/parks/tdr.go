// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package parks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Ticket sale states decoded from the TDR calendar.
const (
	StatusOnSale       = "on_sale"
	StatusFewLeft      = "few_left"
	StatusSoldOut      = "sold_out"
	StatusNotYetOnSale = "not_yet_on_sale"
	StatusNoOnlineSale = "no_online_sale"
	StatusUnknown      = "unknown"
)

// StatusSymbols maps a decoded status to the official calendar legend.
var StatusSymbols = map[string]string{
	StatusOnSale:       "○ 販売中",
	StatusFewLeft:      "△ チケット残りわずか",
	StatusSoldOut:      "X 売り切れ",
	StatusNoOnlineSale: "ー オンラインでの販売なし",
	StatusNotYetOnSale: "販売前 (not yet on sale)",
	StatusUnknown:      "unknown",
}

// StatusAliases lets users pass short status names to filters.
var StatusAliases = map[string]string{
	"on-sale": StatusOnSale, "on_sale": StatusOnSale, "onsale": StatusOnSale, "available": StatusOnSale,
	"few": StatusFewLeft, "few-left": StatusFewLeft, "few_left": StatusFewLeft,
	"sold-out": StatusSoldOut, "sold_out": StatusSoldOut, "soldout": StatusSoldOut,
	"not-yet": StatusNotYetOnSale, "not-yet-on-sale": StatusNotYetOnSale, "not_yet_on_sale": StatusNotYetOnSale,
	"no-online": StatusNoOnlineSale, "no-online-sale": StatusNoOnlineSale, "no_online_sale": StatusNoOnlineSale,
	"unknown": StatusUnknown,
}

// ErrTDRStructure marks a TDR page whose calendar structure is not the one
// this parser knows. Callers must fail loudly instead of returning empty data.
var ErrTDRStructure = errors.New("TDR ticket calendar structure changed")

// ErrNoTicketPopup means the page has no `var ticketPopup` line. A month
// can look like this before TDR publishes it; dates treats it as unpublished
// until the published sale rule says the month is on sale, and as a structure
// change after that.
var ErrNoTicketPopup = fmt.Errorf("%w: `var ticketPopup` not found in the page", ErrTDRStructure)

var ticketPopupRe = regexp.MustCompile(`(?m)var\s+ticketPopup\s*=\s*(.*?);?\s*$`)

// ExtractTicketPopup returns the raw JSON assigned to `var ticketPopup` in a
// TDR ticket calendar page.
func ExtractTicketPopup(page []byte) (json.RawMessage, error) {
	m := ticketPopupRe.FindSubmatch(page)
	if m == nil {
		return nil, ErrNoTicketPopup
	}
	raw := bytes.TrimSpace(m[1])
	raw = bytes.TrimSuffix(raw, []byte(";"))
	if !json.Valid(raw) {
		return nil, fmt.Errorf("%w: `var ticketPopup` is not valid JSON", ErrTDRStructure)
	}
	return json.RawMessage(raw), nil
}

type tdrOpenTime struct {
	Open  string `json:"open"`
	Close string `json:"close"`
}

type tdrTicketState struct {
	PassFlg    bool   `json:"passFlg"`
	ClassPopup string `json:"classPopup"`
}

type tdrTicket struct {
	Name    string         `json:"name"`
	Ticket  tdrTicketState `json:"ticket"`
	AgeText string         `json:"ageText"`
}

type tdrGroup struct {
	Name string          `json:"name"`
	List json.RawMessage `json:"list"`
}

type tdrDay struct {
	OpenTime json.RawMessage `json:"openTime"`
	Info     json.RawMessage `json:"info"`
}

// TDRTicket is one ticket type on one date at one park.
type TDRTicket struct {
	ID           string         `json:"id"`
	GroupID      string         `json:"group_id"`
	GroupNameJA  string         `json:"group_name_ja"`
	GroupNameEN  string         `json:"group_name_en,omitempty"`
	NameJA       string         `json:"name_ja"`
	NameEN       string         `json:"name_en,omitempty"`
	Status       string         `json:"status"`
	StatusLegend string         `json:"status_legend"`
	RawClass     string         `json:"raw_class"`
	PricesYen    map[string]int `json:"prices_yen"`
	PriceText    string         `json:"price_text_ja"`
}

// TDRDay is one park on one date.
type TDRDay struct {
	Date    string      `json:"date"`
	Park    string      `json:"park"`
	Open    string      `json:"open"`
	Close   string      `json:"close"`
	Tickets []TDRTicket `json:"tickets"`
}

// TDRCalendar maps "YYYY-MM-DD" -> park code (tdl/tds) -> day.
type TDRCalendar map[string]map[string]*TDRDay

// ParseTicketPopup decodes a JA (or EN) ticketPopup document. An empty JSON
// array means the month is not published yet and returns an empty calendar.
func ParseTicketPopup(raw json.RawMessage) (TDRCalendar, error) {
	cal := TDRCalendar{}
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("[]")) || bytes.Equal(trimmed, []byte("{}")) {
		return cal, nil
	}
	var top map[string]map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	if err := dec.Decode(&top); err != nil {
		return nil, fmt.Errorf("%w: ticketPopup is not a date->park object: %v", ErrTDRStructure, err)
	}
	for ymd, byPark := range top {
		date, err := ymdToISO(ymd)
		if err != nil {
			return nil, fmt.Errorf("%w: unexpected date key %q", ErrTDRStructure, ymd)
		}
		for parkCode, rawDay := range byPark {
			var d tdrDay
			dd := json.NewDecoder(bytes.NewReader(rawDay))
			dd.UseNumber()
			if err := dd.Decode(&d); err != nil {
				return nil, fmt.Errorf("%w: day %s/%s: %v", ErrTDRStructure, date, parkCode, err)
			}
			day := &TDRDay{Date: date, Park: parkCode, Tickets: []TDRTicket{}}
			var ot tdrOpenTime
			if len(d.OpenTime) > 0 && json.Unmarshal(d.OpenTime, &ot) == nil {
				day.Open, day.Close = padClock(ot.Open), padClock(ot.Close)
			}
			groups, err := decodeObjectOrEmpty[tdrGroup](d.Info)
			if err != nil {
				return nil, fmt.Errorf("%w: day %s/%s info: %v", ErrTDRStructure, date, parkCode, err)
			}
			for _, gid := range slices.Sorted(maps.Keys(groups)) {
				g := groups[gid]
				tickets, err := decodeObjectOrEmpty[tdrTicket](g.List)
				if err != nil {
					return nil, fmt.Errorf("%w: day %s/%s group %s: %v", ErrTDRStructure, date, parkCode, gid, err)
				}
				for _, tid := range sortTicketIDs(slices.Sorted(maps.Keys(tickets))) {
					t := tickets[tid]
					st := DecodeStatus(t.Ticket.PassFlg, t.Ticket.ClassPopup)
					day.Tickets = append(day.Tickets, TDRTicket{
						ID: tid, GroupID: gid, GroupNameJA: cleanRemote(g.Name), NameJA: cleanRemote(t.Name),
						Status: st, StatusLegend: StatusSymbols[st], RawClass: t.Ticket.ClassPopup,
						PricesYen: ParsePrices(cleanRemote(t.AgeText)), PriceText: cleanRemote(t.AgeText),
					})
				}
			}
			if cal[date] == nil {
				cal[date] = map[string]*TDRDay{}
			}
			cal[date][parkCode] = day
		}
	}
	return cal, nil
}

// MergeEnglish copies English ticket and group names from an EN calendar.
func (c TDRCalendar) MergeEnglish(en TDRCalendar) {
	for date, byPark := range c {
		for park, day := range byPark {
			enDay := en[date][park]
			if enDay == nil {
				continue
			}
			// Ticket ids repeat across groups, so match on group and ticket id.
			byID := map[string]TDRTicket{}
			for _, t := range enDay.Tickets {
				byID[t.GroupID+"/"+t.ID] = t
			}
			for i := range day.Tickets {
				if e, ok := byID[day.Tickets[i].GroupID+"/"+day.Tickets[i].ID]; ok {
					day.Tickets[i].NameEN = e.NameJA
					day.Tickets[i].GroupNameEN = e.GroupNameJA
				}
			}
		}
	}
}

// DecodeStatus maps the calendar's CSS classes to a sale state. The legend on
// the official page is ○ on sale, △ few left, X sold out, ー no online sale.
func DecodeStatus(passFlg bool, class string) string {
	tokens := map[string]bool{}
	for _, t := range strings.Fields(class) {
		tokens[t] = true
	}
	switch {
	case tokens["is-none"]:
		return StatusSoldOut
	case tokens["is-notSales"]:
		return StatusNotYetOnSale
	case passFlg:
		return StatusNoOnlineSale
	case tokens["is-few"]:
		return StatusFewLeft
	case tokens["conversion"] && len(tokens) == 1:
		return StatusOnSale
	default:
		return StatusUnknown
	}
}

var priceRe = regexp.MustCompile(`(大人|中人|小人|Adult|Junior|Child)\s*[：:]\s*[￥¥]?\s*([0-9][0-9,]*)`)

var priceLabels = map[string]string{
	"大人": "adult", "中人": "junior", "小人": "child",
	"Adult": "adult", "Junior": "junior", "Child": "child",
}

// ParsePrices reads adult/junior/child yen amounts from a TDR ageText line.
func ParsePrices(text string) map[string]int {
	out := map[string]int{}
	for _, m := range priceRe.FindAllStringSubmatch(text, -1) {
		n, err := strconv.Atoi(strings.ReplaceAll(m[2], ",", ""))
		if err != nil {
			continue
		}
		out[priceLabels[m[1]]] = n
	}
	return out
}

// SaleOpensAt applies the published TDR rule: tickets for a date go on sale
// at 14:00 JST on the same day two months earlier; when that day does not
// exist, sales start on the 1st of the following month at 14:00.
func SaleOpensAt(date time.Time) time.Time {
	y, m, d := date.Date()
	ty, tm := y, m-2
	if tm < 1 {
		tm += 12
		ty--
	}
	first := time.Date(ty, tm, 1, 14, 0, 0, 0, Tokyo)
	lastDay := first.AddDate(0, 1, -1).Day()
	if d > lastDay {
		return first.AddDate(0, 1, 0)
	}
	return time.Date(ty, tm, d, 14, 0, 0, 0, Tokyo)
}

// SaleRuleBasis is shown next to every rule-derived sale-open time.
const SaleRuleBasis = "Published TDR rule (tokyodisneyresort.jp/ticket/): tickets for a date go on sale daily from 14:00 JST two months ahead; if that day does not exist, from the 1st of the next month at 14:00. Rule-derived, not observed; sales can stop or restart without notice."

// MonthsBetween lists YYYYMM strings from a through b inclusive.
func MonthsBetween(a, b time.Time) []string {
	out := []string{}
	cur := time.Date(a.Year(), a.Month(), 1, 0, 0, 0, 0, Tokyo)
	end := time.Date(b.Year(), b.Month(), 1, 0, 0, 0, 0, Tokyo)
	for !cur.After(end) {
		out = append(out, cur.Format("200601"))
		cur = cur.AddDate(0, 1, 0)
	}
	return out
}

func ymdToISO(ymd string) (string, error) {
	t, err := time.ParseInLocation("20060102", ymd, Tokyo)
	if err != nil {
		return "", err
	}
	return t.Format("2006-01-02"), nil
}

func decodeObjectOrEmpty[T any](raw json.RawMessage) (map[string]T, error) {
	out := map[string]T{}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("[]")) {
		return out, nil
	}
	if err := json.Unmarshal(trimmed, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// sortTicketIDs orders T1, T2, T8, T10 numerically.
func sortTicketIDs(ids []string) []string {
	sort.SliceStable(ids, func(i, j int) bool {
		ni, ei := strconv.Atoi(strings.TrimLeft(ids[i], "TG"))
		nj, ej := strconv.Atoi(strings.TrimLeft(ids[j], "TG"))
		if ei == nil && ej == nil {
			return ni < nj
		}
		return ids[i] < ids[j]
	})
	return ids
}

var clockRe = regexp.MustCompile(`^([0-9]{1,2}):([0-5][0-9])$`)

// padClock turns "9:00" into "09:00" so TDR and ThemeParks.wiki hours match.
// "24:00" (midnight close) is kept. Anything that is not a clock time
// returns "" (unknown), never raw text.
func padClock(s string) string {
	m := clockRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return ""
	}
	h, _ := strconv.Atoi(m[1])
	if h > 24 || (h == 24 && m[2] != "00") {
		return ""
	}
	return fmt.Sprintf("%02d:%s", h, m[2])
}
