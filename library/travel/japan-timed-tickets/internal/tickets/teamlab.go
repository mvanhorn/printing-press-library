// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package tickets

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// TeamLabConfig holds the parts of /api/v1/configurations the CLI reads.
type TeamLabConfig struct {
	CalendarPeriod string // YYYY-MM, the last bookable month
}

// ParseTeamLabConfig reads the configuration list.
func ParseTeamLabConfig(raw []byte) (TeamLabConfig, error) {
	var items []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return TeamLabConfig{}, fmt.Errorf("configurations: %w", err)
	}
	cfg := TeamLabConfig{}
	for _, it := range items {
		if it.Key == "Calendar_Period" {
			cfg.CalendarPeriod = strings.TrimSpace(it.Value)
		}
	}
	return cfg, nil
}

// LastDayOfPeriod returns the last date of a YYYY-MM month.
func LastDayOfPeriod(period string) (Date, bool) {
	t, err := time.ParseInLocation("2006-01", period, JST)
	if err != nil {
		return Date{}, false
	}
	return DateOf(t.AddDate(0, 1, -1)), true
}

var reJANotice = regexp.MustCompile(`(\d{1,2})月(\d{1,2})日\s*(\d{1,2})時(\d{1,2})分`)

// ParseTeamLabNotice reads the Japanese "unpublished_calendar_notice" text,
// for example "10月30日 12時00分発売予定". The year is the first one that puts
// the occurrence nearest to now (see InferYear).
func ParseTeamLabNotice(raw []byte, now time.Time) (text string, at *time.Time, tentative bool, err error) {
	var items []struct {
		Key  string `json:"key"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return "", nil, false, fmt.Errorf("texts: %w", err)
	}
	for _, it := range items {
		if it.Key != "unpublished_calendar_notice" {
			continue
		}
		text = strings.TrimSpace(StripControl(halfWidth(it.Text)))
		break
	}
	if text == "" {
		return "", nil, false, nil
	}
	tentative = strings.Contains(text, "予定") || strings.Contains(strings.ToLower(text), "tentative")
	m := reJANotice.FindStringSubmatch(text)
	if m == nil {
		return text, nil, tentative, nil
	}
	mo, _ := strconv.Atoi(m[1])
	dd, _ := strconv.Atoi(m[2])
	hh, _ := strconv.Atoi(m[3])
	mi, _ := strconv.Atoi(m[4])
	ref := now.In(JST)
	cand := time.Date(InferYear(mo, ref.Year(), int(ref.Month())), time.Month(mo), dd, hh, mi, 0, 0, JST)
	return text, &cand, tentative, nil
}

// ParseTeamLabStatuses maps stocks-statuses records to day statuses.
func ParseTeamLabStatuses(raw []byte) (map[string]DayStatus, error) {
	var resp struct {
		Records []struct {
			Date        string `json:"date"`
			StockStatus string `json:"stock_status"`
		} `json:"records"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("stocks-statuses: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("stocks-statuses: %s", resp.Error)
	}
	out := map[string]DayStatus{}
	for _, r := range resp.Records {
		if len(r.Date) < 10 {
			continue
		}
		key := r.Date[:10]
		if t, err := time.Parse(time.RFC3339, r.Date); err == nil {
			key = DateOf(t).String()
		}
		switch r.StockStatus {
		case "":
			out[key] = DayAvailable
		case "few":
			out[key] = DayFew
		case "sold_out":
			out[key] = DaySoldOut
		case "closed":
			out[key] = DayClosed
		default:
			out[key] = DayUnknown
		}
	}
	return out, nil
}

// TeamLabProduct is one product on a date.
type TeamLabProduct struct {
	ID       int    `json:"id"`
	UUID     string `json:"uuid"`
	Name     string `json:"name"`
	SaleType string `json:"sale_type"`
	Few      bool   `json:"is_few_stock"`
	SoldOut  bool   `json:"is_sold_out"`
}

// ParseTeamLabProducts reads /api/v1/products/search2.
func ParseTeamLabProducts(raw []byte) ([]TeamLabProduct, error) {
	var resp struct {
		Products []TeamLabProduct `json:"products"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("products: %w", err)
	}
	return resp.Products, nil
}

// ParseTeamLabSlots reads /api/v1/products/stocks time spans.
func ParseTeamLabSlots(raw []byte, date, product string) ([]Slot, error) {
	var resp struct {
		TimeSpans []struct {
			StartTime  string `json:"start_time"`
			StartStr   string `json:"start_str"`
			EndStr     string `json:"end_str"`
			StockCount *int   `json:"stock_count"`
			IsFew      bool   `json:"is_few_stock"`
			Options    []struct {
				Name    string `json:"name"`
				AddCost *int   `json:"add_cost"`
			} `json:"options"`
		} `json:"time_spans"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("stocks: %w", err)
	}
	out := make([]Slot, 0, len(resp.TimeSpans))
	for _, ts := range resp.TimeSpans {
		status := SlotAvailable
		switch {
		case ts.StockCount != nil && *ts.StockCount <= 0:
			status = SlotSoldOut
		case ts.IsFew:
			status = SlotFew
		case ts.StockCount == nil:
			status = SlotUnknown
		}
		var adult *int
		for _, o := range ts.Options {
			if strings.EqualFold(o.Name, "Adults") || strings.HasPrefix(strings.ToLower(o.Name), "adult") {
				adult = o.AddCost
				break
			}
		}
		startJST := "" // empty when unparsable, so the past-slot filter keeps the slot
		if t, err := time.Parse(time.RFC3339, ts.StartTime); err == nil {
			startJST = FormatJST(t)
		}
		out = append(out, Slot{Date: date, Product: StripControl(product), Start: StripControl(ts.StartStr), End: StripControl(ts.EndStr), StartJST: startJST, Status: status, Stock: ts.StockCount, AdultPriceJPY: adult})
	}
	return out, nil
}

func teamLabAPI(host, path string, q url.Values) string {
	u := "https://" + host + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	return u
}

func readTeamLabJSON(ctx context.Context, f *Fetcher, sd *SightData, from, to Date, slotDates []Date, now func() time.Time) {
	host := sd.Sight.Host
	raw, err := sd.read(ctx, f, teamLabAPI(host, "/api/v1/configurations", nil), true, now)
	if err != nil {
		sd.warn("ticket-site configuration unavailable: " + err.Error())
		return
	}
	cfg, err := ParseTeamLabConfig(raw)
	if err != nil {
		sd.warn(err.Error())
		return
	}
	last, ok := LastDayOfPeriod(cfg.CalendarPeriod)
	if ok {
		sd.CalendarUntil = &last
		sd.RuleConfirmed = true
	}
	if raw, err := sd.read(ctx, f, teamLabAPI(host, "/api/v1/texts", url.Values{"lang": {"ja"}}), true, now); err != nil {
		sd.warn("release notice unavailable: " + err.Error())
	} else {
		text, at, tentative, perr := ParseTeamLabNotice(raw, now())
		if perr != nil {
			sd.warn(perr.Error())
		} else if text != "" {
			rel := &Release{Text: text, SourceURL: sd.Sight.TicketSiteURL, Tentative: tentative, OnSale: at, Confidence: ConfAnnounced}
			if ok {
				rel.Covers = last.AddDays(1).Month()
			}
			if at == nil {
				rel.Confidence = ConfEstimated
			}
			sd.Release = rel
		}
	}
	if !ok {
		sd.warn("ticket site did not publish a calendar period; dates are unknown")
		return
	}
	qTo := to
	if last.Before(qTo) {
		qTo = last
	}
	if !qTo.Before(from) {
		readTeamLabDays(ctx, f, sd, from, qTo, now)
	}
	for _, d := range slotDates {
		if last.Before(d) {
			continue
		}
		if st, ok := sd.Days[d.String()]; ok && st.Status == DayClosed {
			continue
		}
		raw, err := sd.read(ctx, f, teamLabAPI(host, "/api/v1/products/search2", url.Values{"date": {d.String()}, "lang": {"en"}}), true, now)
		if err != nil {
			sd.warn(fmt.Sprintf("products for %s unavailable: %v", d, err))
			continue
		}
		products, err := ParseTeamLabProducts(raw)
		if err != nil {
			sd.warn(err.Error())
			continue
		}
		for _, p := range products {
			if p.SaleType != "time" || p.UUID == "" {
				continue
			}
			raw, err := sd.read(ctx, f, teamLabAPI(host, "/api/v1/products/stocks", url.Values{"date": {d.String()}, "product_id": {p.UUID}, "lang": {"en"}}), true, now)
			if err != nil {
				sd.warn(fmt.Sprintf("slots for %s %s unavailable: %v", d, p.Name, err))
				continue
			}
			slots, err := ParseTeamLabSlots(raw, d.String(), p.Name)
			if err != nil {
				sd.warn(err.Error())
				continue
			}
			sd.Slots = append(sd.Slots, slots...)
		}
	}
}

// readTeamLabDays reads the day calendar for from..to (inside the period).
func readTeamLabDays(ctx context.Context, f *Fetcher, sd *SightData, from, to Date, now func() time.Time) {
	raw, err := sd.read(ctx, f, teamLabAPI(sd.Sight.Host, "/api/v1/products/stocks-statuses", url.Values{"from": {from.String()}, "to": {to.String()}}), true, now)
	if err != nil {
		sd.warn("calendar status unavailable: " + err.Error())
		return
	}
	statuses, err := ParseTeamLabStatuses(raw)
	if err != nil {
		sd.warn(err.Error())
		return
	}
	FillTeamLabDays(sd.Days, statuses, from, to)
}

// FillTeamLabDays stores calendar statuses. A date inside the range that the
// calendar omits has no ticket products (seen on non-operating days); the
// source gives no reason, so it stays unknown.
func FillTeamLabDays(days map[string]Day, statuses map[string]DayStatus, from, to Date) {
	for key, st := range statuses {
		days[key] = Day{Date: key, Status: st, Reason: DefaultReason(st)}
	}
	for _, d := range DateRange(from, to) {
		if _, ok := days[d.String()]; !ok {
			days[d.String()] = Day{Date: d.String(), Status: DayUnknown, Reason: "the ticket calendar lists no tickets for this date (possibly a non-operating day; the source gives no reason)"}
		}
	}
}
