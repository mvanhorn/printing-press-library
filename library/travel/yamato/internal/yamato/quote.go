// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package yamato

import (
	"context"
	"fmt"
	"golang.org/x/net/html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type QuoteInput struct {
	Service, Origin, Destination, Airport, Date, DateKind string
	Size                                                  int
}
type Rate struct {
	Size        int `json:"size"`
	CashlessJPY int `json:"cashless_jpy"`
	CashJPY     int `json:"cash_jpy"`
}
type Quote struct {
	Service             string   `json:"service"`
	Origin              string   `json:"origin"`
	Destination         string   `json:"destination"`
	Airport             *Airport `json:"airport"`
	InputDate           string   `json:"input_date"`
	DateKind            string   `json:"date_kind"`
	CalendarKind        string   `json:"calendar_kind"`
	CalendarDate        string   `json:"calendar_date"`
	DeliveryTimeJP      string   `json:"delivery_time_jp,omitempty"`
	Guaranteed          bool     `json:"guaranteed"`
	SelectedRate        Rate     `json:"selected_rate"`
	Rates               []Rate   `json:"rates,omitempty"`
	Currency            string   `json:"currency"`
	TaxIncluded         bool     `json:"tax_included"`
	RateBasis           string   `json:"rate_basis"`
	IncludedFees        []string `json:"included_fees"`
	Discounts           string   `json:"discounts"`
	Acceptance          string   `json:"acceptance"`
	CounterCutoffTime   any      `json:"counter_cutoff_time"`
	ExceptionalCalendar any      `json:"exceptional_calendar"`
	Warnings            []string `json:"warnings"`
	HandoffURL          string   `json:"handoff_url"`
}

var jpDate = regexp.MustCompile(`(\d{4})年(\d{1,2})月(\d{1,2})日`)
var zip = regexp.MustCompile(`^\d{7}$`)
var sizeLabel = regexp.MustCompile(`^(60|80|100|120|140|160|180|200)サイズ$`)

func (in QuoteInput) Validate(now time.Time) error {
	if !zip.MatchString(strings.ReplaceAll(in.Origin, "-", "")) {
		return fmt.Errorf("--origin must be a seven-digit Japanese postal code")
	}
	if !ValidSize(in.Size) {
		return fmt.Errorf("--size must be 60,80,100,120,140,160,180 or 200; use parcel to determine the greater dimension/weight category")
	}
	if in.Service != "takkyubin" && in.Service != "airport" && in.Service != "roundtrip" && in.Service != "airport-roundtrip" {
		return fmt.Errorf("--service must be takkyubin, airport, roundtrip or airport-roundtrip; use same-day for separate limited-area evidence")
	}
	if strings.HasPrefix(in.Service, "airport") {
		if in.Airport == "" || in.Destination != "" {
			return fmt.Errorf("airport services require --airport ID from airports and no --destination")
		}
	} else if !zip.MatchString(strings.ReplaceAll(in.Destination, "-", "")) || in.Airport != "" {
		return fmt.Errorf("postal services require --destination seven-digit postal code and no --airport")
	}
	if in.DateKind != "ship" && in.DateKind != "delivery" && in.DateKind != "boarding" && in.DateKind != "use" {
		return fmt.Errorf("--date-kind must be ship, delivery (TA-Q-BIN), boarding (airport), or use (roundtrip)")
	}
	if in.DateKind != "ship" && ((in.Service == "takkyubin" && in.DateKind != "delivery") || (strings.HasPrefix(in.Service, "airport") && in.DateKind != "boarding") || (in.Service == "roundtrip" && in.DateKind != "use")) {
		return fmt.Errorf("--date-kind %s does not match service %s", in.DateKind, in.Service)
	}
	loc := time.FixedZone("JST", 9*60*60)
	d, e := time.ParseInLocation("2006-01-02", in.Date, loc)
	if e != nil {
		return fmt.Errorf("--date must be a real YYYY-MM-DD calendar date in Japan")
	}
	today := now.In(loc).Format("2006-01-02")
	if in.Date < today {
		return fmt.Errorf("--date must be today (%s JST) or later; this source returns current rates, not historical rates", today)
	}
	if d.After(now.In(loc).AddDate(1, 0, 0)) {
		return fmt.Errorf("--date must be within the next year; upstream calendar may impose a narrower range")
	}
	return nil
}
func (c *Client) Quote(ctx context.Context, in QuoteInput, detail bool) (Quote, error) {
	q := Quote{Service: in.Service, Origin: strings.ReplaceAll(in.Origin, "-", ""), Destination: strings.ReplaceAll(in.Destination, "-", ""), InputDate: in.Date, DateKind: in.DateKind, Currency: "JPY", TaxIncluded: true, IncludedFees: []string{}, RateBasis: "one-way per parcel, source list tariff", Discounts: "conditional drop-off/member/digital discounts not applied; packaging and other optional charges not included", Acceptance: "unknown: valid source quote is not shipment, counter or hotel acceptance", Warnings: []string{"Estimated dates are not guaranteed. Counter shipping cutoff times vary and are not closing hours.", "Weather, holidays, disrupted transport and some islands may alter displayed dates; air-restricted or unclear contents can delay delivery up to about one week.", "Confirm hotel reception and contents/packaging eligibility before sending; some convenience stores cannot accept size200."}}
	endpoint := "TakkyubinSmp"
	act := "J_RKTKJS0010_SMP"
	link := "TK"
	if in.Service == "roundtrip" {
		endpoint = "LeisureTakkyubinSmp"
		act = "J_RKLTJS0010_SMP"
		link = "LT"
		q.RateBasis = "roundtrip per parcel; source roundtrip discount included"
		q.IncludedFees = append(q.IncludedFees, "source roundtrip reduction included; do not subtract another 200 JPY")
		q.Warnings = append(q.Warnings, "Return service within one month of outbound dropoff; confirm lodging accepts both legs.")
	}
	if strings.HasPrefix(in.Service, "airport") {
		endpoint = "KuukouTakkyubinSmp"
		act = "J_RKKTJS0010_SMP"
		link = "KT"
		aa, e := c.Airports(ctx)
		if e != nil {
			return q, e
		}
		var chosen *Airport
		for _, a := range aa {
			if a.ID == in.Airport {
				v := a
				chosen = &v
				break
			}
		}
		if chosen == nil {
			return q, fmt.Errorf("--airport %q not present in live source airport list; run airports", in.Airport)
		}
		q.Airport = chosen
		q.Destination = chosen.NameJP
		q.IncludedFees = append(q.IncludedFees, "outbound airport fee 660 JPY included")
		q.Warnings = append(q.Warnings, "No time-zone delivery or cash-on-delivery when sending to an airport. Airport delivery occurs by the day before boarding; use source deadline.")
		if in.Service == "airport-roundtrip" {
			q.RateBasis = "roundtrip per parcel; source roundtrip discount and outbound airport fee included"
			q.IncludedFees = append(q.IncludedFees, "source roundtrip reduction included; do not subtract another 200 JPY")
			q.Warnings = append(q.Warnings, "Return service within one month; Kansai return airport fee 660 JPY applies and must be confirmed against source tariff.")
		}
	}
	d, _ := time.Parse("2006-01-02", in.Date)
	kind := "PARA_DELIVERY_SEARCH"
	if in.DateKind != "ship" {
		kind = "PARA_CARRY_SEARCH"
	}
	dest := q.Destination
	if q.Airport != nil {
		dest = q.Airport.ID
	}
	form := url.Values{"ACTID": {act}, "PARA_STA": {q.Origin}, "PARA_END": {dest}, "PARA_YEAR": {strconv.Itoa(d.Year())}, "PARA_MONTH": {strconv.Itoa(int(d.Month()))}, "PARA_DAY": {strconv.Itoa(d.Day())}, "PARA_SEARCH_KBN": {kind}, "BTN_EXEC_SLEVEL": {string([]byte{0x8c, 0x9f, 0x8d, 0xf5})}}
	q.HandoffURL = Date + "MainSmp?LINK=" + link
	if q.Airport == nil {
		if _, e := c.Fetch(ctx, q.HandoffURL, nil, 2<<20); e != nil {
			return q, e
		}
	}
	b, e := c.Fetch(ctx, Date+endpoint, form, 2<<20)
	if e != nil {
		return q, e
	}
	return ParseQuote(b, in, q, detail)
}
func ParseQuote(b []byte, in QuoteInput, q Quote, detail bool) (Quote, error) {
	doc, e := Parse(b)
	if e != nil {
		return q, e
	}
	var tables []*html.Node
	for _, table := range Nodes(doc, "table") {
		comparison := false
		for parent := table.Parent; parent != nil; parent = parent.Parent {
			for _, class := range strings.Fields(Attr(parent, "class")) {
				if class == "openable" {
					comparison = true
				}
			}
		}
		// The expandable other-product comparisons contain independent rates
		// and dates. They are not this submitted product's primary result.
		if comparison {
			continue
		}
		// Desktop/mobile layout tables may contain entire nested calendars and
		// multiple tariffs. Only actual leaf tables have meaningful row columns.
		if len(Nodes(table, "table")) == 1 {
			tables = append(tables, table)
		}
	}
	if len(tables) == 0 {
		return q, fmt.Errorf("Yamato returned no quote table (invalid postal code/calendar or source changed); open %s", q.HandoffURL)
	}
	label := "発送締切日"
	if in.DateKind == "ship" {
		label = "お届け予定日"
		if strings.HasPrefix(in.Service, "airport") {
			label = "ご搭乗可能日"
		}
		if in.Service == "roundtrip" {
			label = "ご利用可能日"
		}
	}
	var dr [][]string
	for _, table := range tables {
		rows := Rows(table)
		if len(rows) >= 2 && len(rows[0]) >= 1 && rows[0][0] == label {
			dr = rows
			break
		}
	}
	var dates []string
	if len(dr) >= 2 && len(dr[1]) >= 1 {
		dates = jpDate.FindStringSubmatch(dr[1][0])
	}
	if dates == nil {
		return q, fmt.Errorf("Yamato returned no calendar result; check route/date or open %s", q.HandoffURL)
	}
	y, _ := strconv.Atoi(dates[1])
	m, _ := strconv.Atoi(dates[2])
	day, _ := strconv.Atoi(dates[3])
	dt := time.Date(y, time.Month(m), day, 0, 0, 0, 0, time.FixedZone("JST", 32400))
	if dt.Year() != y || int(dt.Month()) != m || dt.Day() != day {
		return q, fmt.Errorf("invalid source calendar date")
	}
	q.CalendarDate = dt.Format("2006-01-02")
	if in.DateKind != "ship" {
		q.CalendarKind = "shipping_deadline_date"
	} else {
		q.CalendarKind = "estimated_delivery_date"
		if strings.HasPrefix(in.Service, "airport") {
			q.CalendarKind = "earliest_boarding_date"
		}
		if in.Service == "roundtrip" {
			q.CalendarKind = "earliest_use_date"
		}
	}
	if len(dr) > 1 && len(dr[1]) > 1 {
		q.DeliveryTimeJP = dr[1][1]
	}
	var priceTables [][]Rate
	for _, table := range tables {
		var rates []Rate
		rows := Rows(table)
		for _, row := range rows {
			if len(row) != 3 {
				continue
			}
			mt := sizeLabel.FindStringSubmatch(strings.ReplaceAll(row[0], " ", ""))
			if mt == nil {
				continue
			}
			size, _ := strconv.Atoi(mt[1])
			cashless, e1 := parseYen(row[1])
			cash, e2 := parseYen(row[2])
			if e1 != nil || e2 != nil {
				return q, fmt.Errorf("Yamato rate value changed for size%d; refuse an ambiguous quote", size)
			}
			rates = append(rates, Rate{size, cashless, cash})
		}
		if len(rates) > 0 {
			if len(rows[0]) != 3 || rows[0][0] != "サイズ" || rows[0][1] != "キャッシュレス決済" || rows[0][2] != "現金" {
				return q, fmt.Errorf("Yamato rate column labels changed; no ambiguous cash/cashless tariff returned")
			}
			priceTables = append(priceTables, rates)
		}
	}
	ix := 0
	if in.Service == "roundtrip" || in.Service == "airport-roundtrip" {
		ix = 1
	}
	if len(priceTables) <= ix {
		return q, fmt.Errorf("requested product source rate table absent; open %s", q.HandoffURL)
	}
	rates := priceTables[ix]
	seen := map[int]bool{}
	for _, r := range rates {
		if seen[r.Size] {
			return q, fmt.Errorf("duplicate source rate size")
		}
		seen[r.Size] = true
		if r.Size == in.Size {
			q.SelectedRate = r
		}
	}
	if len(seen) != len(Sizes) || q.SelectedRate.Size == 0 {
		return q, fmt.Errorf("incomplete source rate table; no guessed prices returned")
	}
	if detail {
		q.Rates = rates
	}
	return q, nil
}
func parseYen(s string) (int, error) {
	s = strings.NewReplacer(",", "", "円", "", " ", "", "\u00a0", "").Replace(s)
	v, e := strconv.Atoi(s)
	if e != nil || v <= 0 {
		return 0, fmt.Errorf("invalid JPY value")
	}
	return v, nil
}
