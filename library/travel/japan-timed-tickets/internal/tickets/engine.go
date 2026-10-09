// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package tickets

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-timed-tickets/internal/cliutil"
)

// MaxRangeDays bounds one command's visit-date range.
const MaxRangeDays = 62

// MaxSlotDates bounds how many dates --slots may read.
const MaxSlotDates = 7

// RequestBudget estimates the requests one command needs, with room for
// retries, so a wide --slots read is not cut short part way.
func RequestBudget(sights []Sight, slotDates int) int {
	n := 0
	for _, s := range sights {
		switch s.Kind {
		case KindGhibli:
			n += 3
		case KindShibuyaSky:
			n += 4
		case KindTeamLabDMM:
			n += 1 + slotDates
		case KindTeamLabJSON:
			n += 3 + 3*slotDates // search2 + up to two timed products per date
		}
	}
	return 2 * n
}

// Collect reads every selected sight concurrently (one limiter per host).
// slotDates lists dates whose slot stock should be read (teamLab only).
// A sight whose requests partly fail keeps what was read; a sight with no
// readable source becomes a FetchFailure, and its rows fall back to the
// stored rules.
func Collect(ctx context.Context, f *Fetcher, sights []Sight, from, to Date, slotDates []Date, now func() time.Time) (map[string]*SightData, []FetchFailure) {
	results, errs := cliutil.FanoutRun(ctx, sights, func(s Sight) string { return s.ID }, func(ctx context.Context, s Sight) (*SightData, error) {
		sd := newSightData(s)
		switch s.Kind {
		case KindGhibli:
			readGhibli(ctx, f, sd, from, to, now)
		case KindShibuyaSky:
			readSky(ctx, f, sd, now)
		case KindTeamLabJSON:
			readTeamLabJSON(ctx, f, sd, from, to, slotDates, now)
		case KindTeamLabDMM:
			readDMM(ctx, f, sd, from, to, slotDates, now)
		}
		return sd, nil
	}, cliutil.WithConcurrency(len(sights)))
	out := map[string]*SightData{}
	for _, r := range results {
		out[r.Source] = r.Value
	}
	failures := make([]FetchFailure, 0)
	for _, e := range errs {
		failures = append(failures, FetchFailure{Sight: e.Source, Error: e.Err.Error()})
	}
	for _, s := range sights {
		sd, ok := out[s.ID]
		if !ok {
			continue
		}
		if !sd.anySourceOK() {
			msg := sd.firstFailure()
			if msg == "" {
				msg = "no source could be read"
			}
			failures = append(failures, FetchFailure{Sight: s.ID, Error: msg, RateLimited: sd.RateLimited})
		} else if sd.RateLimited {
			sd.warn("some requests were rate limited (HTTP 429); results for this sight may be incomplete")
		}
	}
	return out, failures
}

func (sd *SightData) anySourceOK() bool {
	for _, src := range sd.Sources {
		if src.OK {
			return true
		}
	}
	return false
}

// FetchFailure records a sight that could not be read.
type FetchFailure struct {
	Sight       string `json:"sight"`
	Error       string `json:"error"`
	RateLimited bool   `json:"rate_limited"`
}

// IsRateLimited reports whether any failure was a throttle (HTTP 429).
func IsRateLimited(failures []FetchFailure) bool {
	for _, f := range failures {
		if f.RateLimited {
			return true
		}
	}
	return false
}

// OnsaleRow is one sight x visit date answer.
type OnsaleRow struct {
	Sight          string      `json:"sight"`
	NameEN         string      `json:"name_en"`
	NameJA         string      `json:"name_ja"`
	VisitDate      string      `json:"visit_date"`
	OnSaleJST      *string     `json:"on_sale_jst"`
	OnSaleLocal    *string     `json:"on_sale_local"`
	OnSaleEarliest *string     `json:"on_sale_earliest_day"`
	OnSaleLatest   *string     `json:"on_sale_latest_day"`
	SaleEndsJST    *string     `json:"sale_ends_jst"`
	Confidence     string      `json:"confidence"`
	SaleBasis      string      `json:"sale_basis"`
	Window         string      `json:"window"`
	State          string      `json:"state"`
	Reason         string      `json:"reason"`
	DayStatus      DayStatus   `json:"day_status"`
	Channel        string      `json:"channel"`
	HandoffURL     string      `json:"handoff_url"`
	Sunset         *SunsetPlan `json:"sunset"`
}

// Row states.
const (
	StateBookNow       = "book-now"
	StateFew           = "few"
	StateOpensAt       = "opens-at"
	StateClosed        = "closed"
	StateSoldOut       = "sold-out"
	StateNoGeneralSale = "no-general-sale"
	StateUnknown       = "unknown"
)

func strPtr(s string) *string { return &s }

// BuildOnsale assembles one row per sight and visit date, sorted by when the
// traveler must act.
func BuildOnsale(data map[string]*SightData, sights []Sight, from, to Date, now time.Time, loc *time.Location) []OnsaleRow {
	rows := make([]OnsaleRow, 0)
	dates := DateRange(from, to)
	for _, s := range sights {
		sd := data[s.ID]
		if sd == nil {
			continue
		}
		for _, d := range dates {
			rows = append(rows, buildRow(sd, d, now, loc))
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		ki, kj := actionKey(rows[i]), actionKey(rows[j])
		if ki != kj {
			return ki < kj
		}
		if rows[i].VisitDate != rows[j].VisitDate {
			return rows[i].VisitDate < rows[j].VisitDate
		}
		return rows[i].Sight < rows[j].Sight
	})
	return rows
}

// actionKey orders rows: act-now states first, then by sale moment, then unknowns.
func actionKey(r OnsaleRow) string {
	switch r.State {
	case StateFew:
		return "0"
	case StateBookNow:
		return "1"
	case StateOpensAt:
		if r.OnSaleJST != nil {
			return "2" + *r.OnSaleJST
		}
		if r.OnSaleEarliest != nil {
			return "2" + *r.OnSaleEarliest
		}
		return "3"
	case StateUnknown:
		return "4"
	}
	return "5"
}

func buildRow(sd *SightData, d Date, now time.Time, loc *time.Location) OnsaleRow {
	s := sd.Sight
	row := OnsaleRow{Sight: s.ID, NameEN: s.NameEN, NameJA: s.NameJA, VisitDate: d.String(), Channel: s.Channels[0].Name, HandoffURL: s.HandoffURL, DayStatus: DayUnknown}
	day, hasDay := sd.Days[d.String()]
	if hasDay {
		row.DayStatus = day.Status
	}
	setMoment := func(t time.Time) {
		row.OnSaleJST = strPtr(FormatJST(t))
		if loc != nil {
			row.OnSaleLocal = strPtr(t.In(loc).Format(time.RFC3339))
		}
	}
	switch s.Kind {
	case KindGhibli:
		moment := GhibliSaleMoment(d)
		row.Confidence, row.SaleBasis = ConfExact, "official rule: 10:00 JST on the 10th of the previous month (confirmed on the museum page)"
		if !sd.RuleConfirmed {
			row.Confidence, row.SaleBasis = ConfEstimated, "rule from "+s.RuleCheckedOn+"; the live page did not confirm it in this run"
		}
		if o, ok := sd.SaleOverrides[d.Month()]; ok {
			moment = o
			row.Confidence, row.SaleBasis = ConfAnnounced, "Lawson Ticket Ghibli page notice for this entry month"
		}
		w := sd.windowFor(d)
		if w != nil {
			moment = *w.Opens
			row.Confidence, row.SaleBasis = ConfAnnounced, "Lawson Ticket listing "+w.Label+" ("+w.Status+")"
			if w.Closes != nil {
				row.SaleEndsJST = strPtr(FormatJST(*w.Closes))
			}
		}
		setMoment(moment)
		switch {
		case hasDay && day.Status == DayClosed:
			row.State, row.Reason = StateClosed, day.Reason
		case hasDay && day.Status == DayNoGeneralSale:
			row.State, row.Reason = StateNoGeneralSale, day.Reason
		case now.Before(moment):
			row.Window, row.State, row.Reason = WindowNotYetOpen, StateOpensAt, "general sale opens at the moment above (Lawson Ticket)"
		case w != nil && w.Closes != nil && now.After(*w.Closes):
			row.Window, row.State, row.Reason = WindowEnded, StateUnknown, "Lawson Ticket sale window has ended"
		default:
			row.Window, row.State, row.Reason = WindowOpen, StateUnknown, "sale open since "+FormatJST(moment)+"; per-date stock needs a Lawson Ticket member login, so it is not read"
		}
		if row.Window == "" {
			if now.Before(moment) {
				row.Window = WindowNotYetOpen
			} else {
				row.Window = WindowOpen
			}
		}
	case KindShibuyaSky:
		moment := SkySaleMoment(d)
		setMoment(moment)
		row.Confidence, row.SaleBasis = ConfExact, "official FAQ: 00:00 JST two weeks before the entry date (confirmed in this run)"
		if !sd.RuleConfirmed {
			row.Confidence, row.SaleBasis = ConfEstimated, "rule from "+s.RuleCheckedOn+"; the live FAQ did not confirm it in this run"
		}
		if plan, ok := PlanSunset(d, loc); ok {
			row.Sunset = plan
		}
		if now.Before(moment) {
			row.Window, row.State, row.Reason = WindowNotYetOpen, StateOpensAt, "web sale opens at the moment above; target the sunset slots"
		} else {
			row.Window, row.State, row.Reason = WindowOpen, StateUnknown, "web sale open; slot stock is behind a virtual waiting room that the CLI does not join"
		}
	case KindTeamLabJSON, KindTeamLabDMM:
		buildTeamLabRow(sd, &row, d, day, hasDay, now, setMoment)
	}
	return row
}

// windowFor returns the Lawson Ticket listing window that covers d (the last
// matching listing wins), or nil.
func (sd *SightData) windowFor(d Date) *SaleWindow {
	var found *SaleWindow
	for i := range sd.Windows {
		w := &sd.Windows[i]
		if w.FirstVisit <= d.String() && d.String() <= w.LastVisit && w.Opens != nil {
			found = w
		}
	}
	return found
}

func buildTeamLabRow(sd *SightData, row *OnsaleRow, d Date, day Day, hasDay bool, now time.Time, setMoment func(time.Time)) {
	if sd.CalendarUntil == nil {
		row.Window, row.State, row.Confidence = WindowUnknown, StateUnknown, ConfUnknown
		row.SaleBasis, row.Reason = "ticket calendar could not be read", "source unavailable in this run"
		return
	}
	until := *sd.CalendarUntil
	if !until.Before(d) {
		row.Window, row.Confidence = WindowOpen, ConfExact
		row.SaleBasis = "on sale now: inside the published calendar (through " + until.String() + ")"
		switch {
		case !hasDay:
			row.State, row.Reason = StateUnknown, "date not in the calendar data"
		case day.Status == DayAvailable:
			row.State, row.Reason = StateBookNow, DefaultReason(day.Status)
		case day.Status == DayFew:
			row.State, row.Reason = StateFew, DefaultReason(day.Status)
		case day.Status == DaySoldOut:
			row.State, row.Reason = StateSoldOut, DefaultReason(day.Status)
		case day.Status == DayClosed:
			row.State, row.Reason = StateClosed, day.Reason
		case day.Reason != "":
			row.State, row.Reason = StateUnknown, day.Reason
		default:
			row.State, row.Reason = StateUnknown, "calendar status: "+string(day.Status)
		}
		return
	}
	row.Window = WindowNotYetOpen
	rel := sd.Release
	if rel == nil {
		row.State, row.Confidence = StateUnknown, ConfUnknown
		row.SaleBasis, row.Reason = "no release announcement found", "month not released yet (calendar ends "+until.String()+")"
		return
	}
	if rel.Covers != "" && rel.Covers != d.Month() {
		row.State, row.Confidence = StateUnknown, ConfUnknown
		row.SaleBasis = "next announced release covers " + rel.Covers + ": " + rel.Text
		row.Reason = "this month is not announced yet (calendar ends " + until.String() + ")"
		return
	}
	row.SaleBasis = "ticket site notice: " + rel.Text
	row.Confidence = rel.Confidence
	if rel.OnSale != nil {
		setMoment(*rel.OnSale)
		if now.After(*rel.OnSale) {
			row.State, row.Reason = StateUnknown, "announced sale moment has passed but the calendar has not been extended yet"
			return
		}
		row.State, row.Reason = StateOpensAt, "next calendar release"
		if rel.Tentative {
			row.Reason += " (tentative)"
		}
		return
	}
	if rel.EarliestDay != "" {
		row.OnSaleEarliest = strPtr(rel.EarliestDay)
		row.OnSaleLatest = strPtr(rel.LatestDay)
		if rel.LatestDay < DateOf(now).String() {
			row.State, row.Reason = StateUnknown, "the estimated release window has passed but the calendar has not been extended yet"
			return
		}
		row.State, row.Reason = StateOpensAt, "release expected in this date range; exact time not published"
		return
	}
	row.State, row.Reason = StateUnknown, "release announced without a date"
}

// Party is a traveling group.
type Party struct {
	Adults   int `json:"adults"`
	Children int `json:"children"`
	Infants  int `json:"infants"`
}

// Total counts everyone; slot fit is conservative and counts infants too.
func (p Party) Total() int { return p.Adults + p.Children + p.Infants }

// ParseParty parses "adults=2,children=1,infants=1".
func ParseParty(s string) (*Party, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	p := &Party{}
	for _, part := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			return nil, fmt.Errorf("invalid --party %q: use adults=N,children=N,infants=N", s)
		}
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n < 0 || n > 50 {
			return nil, fmt.Errorf("invalid --party count %q: use a whole number from 0 to 50, as in adults=N,children=N,infants=N", part)
		}
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "adults", "adult", "a":
			p.Adults = n
		case "children", "child", "kids", "c":
			p.Children = n
		case "infants", "infant", "i":
			p.Infants = n
		default:
			return nil, fmt.Errorf("unknown --party key %q: use adults=N,children=N,infants=N", k)
		}
	}
	if p.Total() == 0 {
		return nil, errors.New("--party must include at least one person, as in adults=N,children=N,infants=N")
	}
	return p, nil
}

// PartyNote returns channel rules that matter for the party at a sight.
func PartyNote(s Sight, p *Party) string {
	if p == nil {
		return ""
	}
	switch s.Kind {
	case KindShibuyaSky:
		if p.Children+p.Infants > 0 {
			return "children and infants: tickets only at the 14F counter on the day; the web sale covers adults (12+) only"
		}
	case KindGhibli:
		note := ""
		if p.Infants > 0 {
			note = "ages 3 and under enter free without a ticket; buy tickets for the others in one Lawson Ticket order"
		}
		if p.Adults+p.Children > ghibliMaxTickets {
			if note != "" {
				note += "; "
			}
			note += fmt.Sprintf("Lawson Ticket allows up to %d tickets per account per month", ghibliMaxTickets)
		}
		return note
	case KindTeamLabJSON, KindTeamLabDMM:
		return "slot fit counts every person, including free infants (conservative)"
	}
	return ""
}

// ApplyParty keeps the slots that can hold the party. A slot whose stock is
// known and large enough gets FitsParty true. A slot with unknown stock is
// kept with FitsParty nil, because the CLI cannot tell. Slots with too little
// stock are dropped. fit counts the slots with FitsParty true.
func ApplyParty(slots []Slot, p *Party) (kept []Slot, fit int) {
	if p == nil {
		return slots, 0
	}
	kept = make([]Slot, 0, len(slots))
	for _, sl := range slots {
		if sl.Stock == nil {
			kept = append(kept, sl)
			continue
		}
		if *sl.Stock >= p.Total() {
			fits := true
			sl.FitsParty = &fits
			kept = append(kept, sl)
			fit++
		}
	}
	return kept, fit
}
