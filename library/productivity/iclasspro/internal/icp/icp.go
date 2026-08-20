// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

// Package icp holds the pure domain logic behind the iClassPro CLI's novel
// commands: normalizing portal payloads, diffing catalog snapshots, computing
// fill trends, evaluating registration windows, linting catalog quality, and
// rendering RFC 5545 calendars.
//
// Nothing here performs I/O. Every function is deterministic given its inputs,
// which is what makes the novel commands testable without a live portal.
package icp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// MediaBase is the prefix iClassPro relative image paths resolve against.
const MediaBase = "https://app.iclasspro.com/media/"

// PortalBase is the customer-portal origin used to build registration links.
const PortalBase = "https://portal.iclasspro.com/"

// Kind distinguishes the two catalog entity families the portal exposes.
const (
	KindClass = "class"
	KindCamp  = "camp"
)

// Slot is one recurring or one-off meeting time.
type Slot struct {
	Date      string `json:"date,omitempty"` // YYYY-MM-DD when known
	DayNumber int    `json:"day_number,omitempty"`
	DayName   string `json:"day_name,omitempty"`
	StartTime string `json:"start_time,omitempty"`
	EndTime   string `json:"end_time,omitempty"`
}

// Entity is the normalized shape shared by classes and camps. The portal names
// the same concept differently between the two families (minAgeYear vs minAge,
// allowWaitlist vs allowToRequestCampThatIsFull), so normalizing once here keeps
// every downstream command from re-learning both dialects.
type Entity struct {
	Kind        string `json:"kind"`
	Account     string `json:"account"`
	LocationID  int    `json:"location_id"`
	ID          int    `json:"id"`
	Name        string `json:"name"`
	ProgramID   int    `json:"program_id,omitempty"`
	ProgramName string `json:"program_name,omitempty"`
	TypeID      int    `json:"type_id,omitempty"`
	LevelID     int    `json:"level_id,omitempty"`

	MinAge int `json:"min_age,omitempty"`
	MaxAge int `json:"max_age,omitempty"`

	Openings       int  `json:"openings"`
	FutureOpenings int  `json:"future_openings,omitempty"`
	HasOpenings    bool `json:"has_openings"`
	ShowOpenings   bool `json:"show_openings"`
	AllowWaitlist  bool `json:"allow_waitlist"`

	StartDate string `json:"start_date,omitempty"`
	EndDate   string `json:"end_date,omitempty"`
	RegStart  string `json:"registration_start_date,omitempty"`
	RegEnd    string `json:"registration_end_date,omitempty"`

	RegExpired     bool `json:"registration_expired,omitempty"`
	ProgramDeleted bool `json:"program_deleted,omitempty"`

	RoomName    string   `json:"room_name,omitempty"`
	Instructors []string `json:"instructors,omitempty"`
	Description string   `json:"description,omitempty"`
	Image       string   `json:"image,omitempty"`

	Slots     []Slot `json:"slots,omitempty"`
	PortalURL string `json:"portal_url,omitempty"`

	// Detailed records whether this entity came from a detail endpoint. The
	// list endpoints omit description, blocks, roomName, and instructors
	// entirely, so a rule that fires on their absence would flag every record
	// from a list sync. Quality rules that depend on those fields consult this
	// flag first.
	Detailed bool `json:"detailed,omitempty"`
}

// Status renders the availability state a human cares about.
func (e Entity) Status() string {
	switch {
	case e.Openings > 0:
		return fmt.Sprintf("open (%d)", e.Openings)
	case e.FutureOpenings > 0:
		return fmt.Sprintf("future (%d)", e.FutureOpenings)
	case e.AllowWaitlist:
		return "waitlist"
	default:
		return "full"
	}
}

// Key is the stable cross-snapshot identity of an entity.
func (e Entity) Key() string {
	return fmt.Sprintf("%s/%s/%d", e.Account, e.Kind, e.ID)
}

// MediaURL resolves a relative portal image path to an absolute URL. Values that
// are already absolute, or empty, are returned unchanged.
func MediaURL(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return p
	}
	return MediaBase + strings.TrimPrefix(p, "/")
}

// PortalURL builds the customer-portal deep link for an entity.
func PortalURL(account, kind string, id int) string {
	leaf := "class-details"
	if kind == KindCamp {
		leaf = "camp-details"
	}
	return fmt.Sprintf("%s%s/%s/%d", PortalBase, account, leaf, id)
}

// ---------- normalization ----------

func num(m map[string]any, keys ...string) int {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case float64:
			return int(t)
		case int:
			return t
		case json.Number:
			if n, err := t.Int64(); err == nil {
				return int(n)
			}
		case string:
			if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
				return n
			}
		}
	}
	return 0
}

func str(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
	}
	return ""
}

// boolean tolerates the portal's habit of returning 1/0 for some flags and true/false
// for others (allowWebRegistration is an int on classes and a bool on camp detail).
func boolean(m map[string]any, keys ...string) bool {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case bool:
			return t
		case float64:
			return t != 0
		case string:
			b, err := strconv.ParseBool(strings.TrimSpace(t))
			if err == nil {
				return b
			}
		}
	}
	return false
}

func strSlice(m map[string]any, key string) []string {
	out := make([]string, 0)
	raw, ok := m[key].([]any)
	if !ok {
		return out
	}
	for _, v := range raw {
		switch t := v.(type) {
		case string:
			if strings.TrimSpace(t) != "" {
				out = append(out, t)
			}
		case map[string]any:
			if n := str(t, "name", "firstName"); n != "" {
				last := str(t, "lastName")
				if last != "" {
					n += " " + last
				}
				out = append(out, n)
			}
		}
	}
	return out
}

func slots(m map[string]any) []Slot {
	out := make([]Slot, 0)
	raw, ok := m["schedule"].([]any)
	if !ok {
		return out
	}
	for _, v := range raw {
		row, ok := v.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, Slot{
			Date:      normalizeDate(str(row, "sqlDate", "date")),
			DayNumber: num(row, "dayNumber", "dayInt"),
			DayName:   str(row, "dayName"),
			StartTime: str(row, "startTime"),
			EndTime:   str(row, "endTime"),
		})
	}
	return out
}

// normalizeDate reduces the portal's mixed date encodings ("2026-09-19",
// "2026-09-19 00:00:00", RFC3339) to a bare YYYY-MM-DD.
func normalizeDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.IndexAny(s, " T"); i > 0 {
		s = s[:i]
	}
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// dates pulls the concrete meeting dates for an entity: camps carry them in
// blocks[].sqlDate, classes in availableDates[].
func dates(m map[string]any) []string {
	out := make([]string, 0)
	seen := map[string]bool{}
	add := func(s string) {
		d := normalizeDate(s)
		if d != "" && !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	if raw, ok := m["blocks"].([]any); ok {
		for _, v := range raw {
			if row, ok := v.(map[string]any); ok {
				add(str(row, "sqlDate", "date"))
			}
		}
	}
	for _, key := range []string{"availableDates", "availableDays"} {
		if raw, ok := m[key].([]any); ok {
			for _, v := range raw {
				if s, ok := v.(string); ok {
					add(s)
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

// NormalizeClass converts a raw /classes or /classes/{id} record into an Entity.
func NormalizeClass(m map[string]any, account string, locationID int) Entity {
	e := Entity{
		Kind:           KindClass,
		Account:        account,
		LocationID:     locationID,
		ID:             num(m, "id"),
		Name:           strings.TrimSpace(str(m, "name")),
		ProgramID:      num(m, "programId"),
		LevelID:        num(m, "levelId"),
		MinAge:         num(m, "minAgeYear"),
		MaxAge:         num(m, "maxAgeYear"),
		Openings:       num(m, "openings"),
		FutureOpenings: num(m, "futureOpenings"),
		ShowOpenings:   boolean(m, "showOpenings"),
		AllowWaitlist:  boolean(m, "allowWaitlist"),
		StartDate:      normalizeDate(str(m, "startDate")),
		EndDate:        normalizeDate(str(m, "endDate")),
		Description:    str(m, "description"),
		Instructors:    strSlice(m, "instructors"),
		Slots:          slots(m),
	}
	e.HasOpenings = e.Openings > 0
	if d, ok := m["dates"].(map[string]any); ok {
		e.RegStart = normalizeDate(str(d, "regStart"))
		e.RegEnd = normalizeDate(str(d, "regEnd"))
		if e.StartDate == "" {
			e.StartDate = normalizeDate(str(d, "start"))
		}
		if e.EndDate == "" {
			e.EndDate = normalizeDate(str(d, "end"))
		}
	}
	e.PortalURL = PortalURL(account, KindClass, e.ID)
	_, e.Detailed = m["description"]
	e.assignDates(dates(m))
	return e
}

// NormalizeCamp converts a raw /camps or /camps/{id} record into an Entity.
func NormalizeCamp(m map[string]any, account string, locationID int) Entity {
	e := Entity{
		Kind:           KindCamp,
		Account:        account,
		LocationID:     locationID,
		ID:             num(m, "id"),
		Name:           strings.TrimSpace(str(m, "name")),
		ProgramID:      num(m, "programId"),
		ProgramName:    str(m, "programName"),
		TypeID:         num(m, "typeId"),
		MinAge:         num(m, "minAge"),
		MaxAge:         num(m, "maxAge"),
		Openings:       num(m, "openings"),
		ShowOpenings:   boolean(m, "showOpenings"),
		AllowWaitlist:  boolean(m, "allowToRequestCampThatIsFull"),
		StartDate:      normalizeDate(str(m, "startDate")),
		EndDate:        normalizeDate(str(m, "endDate")),
		RegStart:       normalizeDate(str(m, "registrationStartDate")),
		RegEnd:         normalizeDate(str(m, "registrationEndDate")),
		RegExpired:     boolean(m, "campRegisterExpired"),
		ProgramDeleted: boolean(m, "programIsDeleted"),
		RoomName:       str(m, "roomName"),
		Description:    str(m, "description"),
		Image:          MediaURL(str(m, "image")),
		Instructors:    strSlice(m, "instructors"),
		Slots:          slots(m),
	}
	e.HasOpenings = boolean(m, "hasOpenings") || e.Openings > 0
	e.PortalURL = PortalURL(account, KindCamp, e.ID)
	_, hasDesc := m["description"]
	_, hasBlocks := m["blocks"]
	e.Detailed = hasDesc || hasBlocks
	e.assignDates(dates(m))
	return e
}

// assignDates attaches concrete dates to slots that only carry a weekday, so the
// calendar renderer has something to anchor a VEVENT to.
func (e *Entity) assignDates(ds []string) {
	if len(ds) == 0 {
		// List endpoints omit blocks and availableDates, but every record
		// carries startDate (and often endDate). Expand that span instead of
		// leaving the entity undated, which would silently drop it from the
		// calendar. When slots declare a weekday, only matching days are
		// emitted, so a Saturday camp running across a week yields Saturdays.
		ds = e.spanDates()
	}
	if len(ds) == 0 {
		return
	}
	if len(e.Slots) == 0 {
		for _, d := range ds {
			e.Slots = append(e.Slots, Slot{Date: d})
		}
		return
	}
	// Slots that already carry a date are left alone. Undated slots are expanded
	// across every known date, which is how a weekly class becomes N events.
	expanded := make([]Slot, 0, len(e.Slots))
	for _, s := range e.Slots {
		if s.Date != "" {
			expanded = append(expanded, s)
			continue
		}
		for _, d := range ds {
			c := s
			c.Date = d
			expanded = append(expanded, c)
		}
	}
	e.Slots = expanded
}

// spanDates expands startDate..endDate into concrete days, honoring the weekday
// of any schedule slot. The span is capped so a mis-set end date cannot generate
// an unbounded event list.
func (e Entity) spanDates() []string {
	start, ok := parseDay(e.StartDate)
	if !ok {
		return nil
	}
	end, hasEnd := parseDay(e.EndDate)
	if !hasEnd || end.Before(start) {
		end = start
	}
	const maxSpanDays = 366
	if end.Sub(start).Hours()/24 > maxSpanDays {
		end = start.AddDate(0, 0, maxSpanDays)
	}

	weekdays := map[int]bool{}
	for _, s := range e.Slots {
		if s.DayNumber > 0 {
			weekdays[s.DayNumber] = true
		}
	}

	out := make([]string, 0)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if len(weekdays) > 0 && !weekdays[int(d.Weekday())+1] {
			continue
		}
		out = append(out, d.Format("2006-01-02"))
	}
	// A weekday filter that matches nothing in the span (bad data upstream)
	// should still anchor the entity to its start date rather than drop it.
	if len(out) == 0 {
		out = append(out, e.StartDate)
	}
	return out
}

// ---------- registration windows ----------

// WindowState classifies where "now" sits relative to an entity's registration window.
type WindowState string

const (
	WindowUpcoming WindowState = "upcoming" // registration has not opened yet
	WindowClosing  WindowState = "closing"  // open now, closes inside the horizon
	WindowExpired  WindowState = "expired"
	WindowOpen     WindowState = "open"
	WindowUnknown  WindowState = "unknown"
)

// Window is one registration-window finding.
type Window struct {
	Entity    Entity      `json:"entity"`
	State     WindowState `json:"state"`
	Date      string      `json:"date,omitempty"`
	DaysAway  int         `json:"days_away"`
	PortalURL string      `json:"portal_url,omitempty"`
}

func parseDay(s string) (time.Time, bool) {
	s = normalizeDate(s)
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func dayDiff(from, to time.Time) int {
	return int(to.Sub(from).Hours() / 24)
}

// OpensSoon returns entities whose registration opens, or closes, inside the next
// `days` days. Entities with no registration dates at all are omitted rather than
// guessed at: the portal leaves those blank for always-open classes.
func OpensSoon(ents []Entity, now time.Time, days int) []Window {
	out := make([]Window, 0)
	if days < 0 {
		days = 0
	}
	today := now.Truncate(24 * time.Hour)
	for _, e := range ents {
		if e.ProgramDeleted {
			continue
		}
		start, hasStart := parseDay(e.RegStart)
		end, hasEnd := parseDay(e.RegEnd)
		switch {
		case hasStart && start.After(today):
			if d := dayDiff(today, start); d <= days {
				out = append(out, Window{Entity: e, State: WindowUpcoming, Date: e.RegStart, DaysAway: d, PortalURL: e.PortalURL})
			}
		case e.RegExpired || (hasEnd && end.Before(today)):
			// Expired windows are reported only when explicitly flagged upstream,
			// so a long-past camp does not flood the result.
			if e.RegExpired {
				out = append(out, Window{Entity: e, State: WindowExpired, Date: e.RegEnd, DaysAway: 0, PortalURL: e.PortalURL})
			}
		case hasEnd:
			if d := dayDiff(today, end); d >= 0 && d <= days {
				out = append(out, Window{Entity: e, State: WindowClosing, Date: e.RegEnd, DaysAway: d, PortalURL: e.PortalURL})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DaysAway != out[j].DaysAway {
			return out[i].DaysAway < out[j].DaysAway
		}
		return out[i].Entity.Name < out[j].Entity.Name
	})
	return out
}

// ---------- drift ----------

// ChangeKind labels one catalog difference between two snapshots.
type ChangeKind string

const (
	ChangeAdded    ChangeKind = "added"
	ChangeRemoved  ChangeKind = "removed"
	ChangeDeleted  ChangeKind = "marked_deleted"
	ChangeRetimed  ChangeKind = "retimed"
	ChangeRenamed  ChangeKind = "renamed"
	ChangeOpenings ChangeKind = "openings_changed"
)

// Change is one drift finding.
type Change struct {
	Kind      ChangeKind `json:"kind"`
	Key       string     `json:"key"`
	Account   string     `json:"account"`
	EntityID  int        `json:"entity_id"`
	Entity    string     `json:"entity_kind"`
	Name      string     `json:"name"`
	From      string     `json:"from,omitempty"`
	To        string     `json:"to,omitempty"`
	PortalURL string     `json:"portal_url,omitempty"`
}

func slotSignature(e Entity) string {
	parts := make([]string, 0, len(e.Slots))
	for _, s := range e.Slots {
		parts = append(parts, fmt.Sprintf("%s|%s|%s|%s", s.Date, s.DayName, s.StartTime, s.EndTime))
	}
	sort.Strings(parts)
	return strings.Join(parts, ";")
}

// Diff compares two catalog snapshots and returns every difference that matters
// to someone maintaining a schedule. It is deliberately conservative: an entity
// missing from `cur` is reported as removed only if `cur` is non-empty, so an
// empty or failed sync cannot masquerade as a mass deletion.
func Diff(prev, cur []Entity) []Change {
	out := make([]Change, 0)
	prevBy := make(map[string]Entity, len(prev))
	for _, e := range prev {
		prevBy[e.Key()] = e
	}
	curBy := make(map[string]Entity, len(cur))
	for _, e := range cur {
		curBy[e.Key()] = e
	}

	keys := make([]string, 0, len(curBy))
	for k := range curBy {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		c := curBy[k]
		p, existed := prevBy[k]
		if !existed {
			out = append(out, Change{Kind: ChangeAdded, Key: k, Account: c.Account, EntityID: c.ID, Entity: c.Kind, Name: c.Name, To: c.Status(), PortalURL: c.PortalURL})
			continue
		}
		if !p.ProgramDeleted && c.ProgramDeleted {
			out = append(out, Change{Kind: ChangeDeleted, Key: k, Account: c.Account, EntityID: c.ID, Entity: c.Kind, Name: c.Name, PortalURL: c.PortalURL})
		}
		if p.Name != c.Name {
			out = append(out, Change{Kind: ChangeRenamed, Key: k, Account: c.Account, EntityID: c.ID, Entity: c.Kind, Name: c.Name, From: p.Name, To: c.Name, PortalURL: c.PortalURL})
		}
		if ps, cs := slotSignature(p), slotSignature(c); ps != cs {
			out = append(out, Change{Kind: ChangeRetimed, Key: k, Account: c.Account, EntityID: c.ID, Entity: c.Kind, Name: c.Name, From: ps, To: cs, PortalURL: c.PortalURL})
		}
		if p.Openings != c.Openings {
			out = append(out, Change{
				Kind: ChangeOpenings, Key: k, Account: c.Account, EntityID: c.ID, Entity: c.Kind, Name: c.Name,
				From: strconv.Itoa(p.Openings), To: strconv.Itoa(c.Openings), PortalURL: c.PortalURL,
			})
		}
	}

	if len(cur) > 0 {
		pkeys := make([]string, 0, len(prevBy))
		for k := range prevBy {
			pkeys = append(pkeys, k)
		}
		sort.Strings(pkeys)
		for _, k := range pkeys {
			if _, still := curBy[k]; still {
				continue
			}
			p := prevBy[k]
			out = append(out, Change{Kind: ChangeRemoved, Key: k, Account: p.Account, EntityID: p.ID, Entity: p.Kind, Name: p.Name, From: p.Status(), PortalURL: p.PortalURL})
		}
	}
	return out
}

// ---------- fill rate ----------

// Sample is one observation of an entity's openings at a point in time.
type Sample struct {
	Key        string
	Account    string
	Kind       string
	EntityID   int
	Name       string
	Openings   int
	ObservedAt time.Time
}

// Trend summarizes how an entity's openings moved across the observed window.
type Trend struct {
	Key          string  `json:"key"`
	Account      string  `json:"account"`
	EntityKind   string  `json:"entity_kind"`
	EntityID     int     `json:"entity_id"`
	Name         string  `json:"name"`
	First        int     `json:"first_openings"`
	Last         int     `json:"last_openings"`
	Delta        int     `json:"delta"`
	Samples      int     `json:"samples"`
	SpanHours    float64 `json:"span_hours"`
	PerDay       float64 `json:"seats_filled_per_day"`
	Direction    string  `json:"direction"`
	ProjectedETA string  `json:"projected_full_date,omitempty"`
}

// FillRates groups samples by entity and computes the direction and velocity of
// fill. Entities with fewer than two observations are skipped: a single sample
// carries no trend, and reporting one as "flat" would be a fabrication.
func FillRates(samples []Sample) []Trend {
	byKey := map[string][]Sample{}
	for _, s := range samples {
		byKey[s.Key] = append(byKey[s.Key], s)
	}
	out := make([]Trend, 0, len(byKey))
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		g := byKey[k]
		if len(g) < 2 {
			continue
		}
		sort.SliceStable(g, func(i, j int) bool { return g[i].ObservedAt.Before(g[j].ObservedAt) })
		first, last := g[0], g[len(g)-1]
		span := last.ObservedAt.Sub(first.ObservedAt).Hours()
		delta := last.Openings - first.Openings
		t := Trend{
			Key: k, Account: last.Account, EntityKind: last.Kind, EntityID: last.EntityID,
			Name: last.Name, First: first.Openings, Last: last.Openings,
			Delta: delta, Samples: len(g), SpanHours: round2(span),
		}
		switch {
		case delta < 0:
			t.Direction = "filling"
		case delta > 0:
			t.Direction = "emptying"
		default:
			t.Direction = "flat"
		}
		if span > 0 && delta < 0 {
			perDay := float64(-delta) / (span / 24)
			t.PerDay = round2(perDay)
			if perDay > 0 && last.Openings > 0 {
				daysLeft := float64(last.Openings) / perDay
				t.ProjectedETA = last.ObservedAt.Add(time.Duration(daysLeft * 24 * float64(time.Hour))).Format("2006-01-02")
			}
		}
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].PerDay != out[j].PerDay {
			return out[i].PerDay > out[j].PerDay
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}

// ---------- lint ----------

// Finding is one catalog-quality problem.
type Finding struct {
	Rule      string `json:"rule"`
	Severity  string `json:"severity"`
	Account   string `json:"account"`
	Entity    string `json:"entity_kind"`
	EntityID  int    `json:"entity_id"`
	Name      string `json:"name"`
	Detail    string `json:"detail"`
	PortalURL string `json:"portal_url,omitempty"`
}

// Lint evaluates catalog hygiene rules against synced entities. Rules only fire
// on evidence present in the record — nothing here infers a problem from absence
// of a field the portal never populates for that entity family.
func Lint(ents []Entity, now time.Time) []Finding {
	out := make([]Finding, 0)
	today := now.Truncate(24 * time.Hour)
	for _, e := range ents {
		add := func(rule, sev, detail string) {
			out = append(out, Finding{
				Rule: rule, Severity: sev, Account: e.Account, Entity: e.Kind,
				EntityID: e.ID, Name: e.Name, Detail: detail, PortalURL: e.PortalURL,
			})
		}
		if e.ProgramDeleted {
			add("deleted_but_listed", "error", "upstream marks the program deleted but it is still returned by the catalog")
		}
		if strings.TrimSpace(e.Name) == "" {
			add("missing_name", "error", "entity has no name")
		}
		// These two rules read fields that only the detail endpoints return.
		// Firing them on list-sourced records would flag every camp in the
		// catalog for a field the response never contained.
		if e.Kind == KindCamp && e.Detailed {
			if strings.TrimSpace(e.Description) == "" {
				add("missing_description", "warning", "camp has no description; it will render blank on a website")
			}
			if strings.TrimSpace(e.Image) == "" {
				add("missing_image", "info", "camp has no flyer image")
			}
		}
		if end, ok := parseDay(e.RegEnd); ok && end.Before(today) && !e.RegExpired {
			add("stale_registration_window", "warning",
				fmt.Sprintf("registration window closed on %s but the record is not flagged expired", e.RegEnd))
		}
		if e.Openings == 0 && !e.AllowWaitlist && e.FutureOpenings == 0 {
			add("full_without_waitlist", "info", "no openings, no future openings, and waitlist is disabled — this is a dead end for a customer")
		}
		if len(e.Slots) == 0 {
			add("no_schedule", "warning", "no schedule slots; a calendar export will skip this entity")
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		rank := map[string]int{"error": 0, "warning": 1, "info": 2}
		if rank[out[i].Severity] != rank[out[j].Severity] {
			return rank[out[i].Severity] < rank[out[j].Severity]
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// ---------- ICS ----------

var timeLayouts = []string{"3:04PM", "3:04 PM", "03:04PM", "15:04", "3:04pm", "15:04:05"}

// parseClock accepts the several clock encodings the portal mixes across
// endpoints and returns hour and minute.
func parseClock(s string) (int, int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, false
	}
	up := strings.ToUpper(strings.ReplaceAll(s, " ", ""))
	for _, l := range timeLayouts {
		layout := strings.ToUpper(strings.ReplaceAll(l, " ", ""))
		if t, err := time.Parse(layout, up); err == nil {
			return t.Hour(), t.Minute(), true
		}
	}
	return 0, 0, false
}

func icsEscape(s string) string {
	r := strings.NewReplacer("\\", "\\\\", ";", "\\;", ",", "\\,", "\n", "\\n", "\r", "")
	return r.Replace(s)
}

// foldLine applies RFC 5545 line folding at 75 octets.
func foldLine(s string) string {
	const limit = 73
	if len(s) <= limit {
		return s
	}
	var b strings.Builder
	for len(s) > limit {
		b.WriteString(s[:limit])
		b.WriteString("\r\n ")
		s = s[limit:]
	}
	b.WriteString(s)
	return b.String()
}

// RenderICS produces an RFC 5545 calendar with one VEVENT per dated slot.
// Entities with no dated slot contribute no events; the count of skipped
// entities is returned so the caller can report the gap honestly rather than
// silently emitting a short calendar.
func RenderICS(ents []Entity, stamp time.Time) (string, int) {
	var b strings.Builder
	w := func(line string) {
		b.WriteString(foldLine(line))
		b.WriteString("\r\n")
	}
	w("BEGIN:VCALENDAR")
	w("VERSION:2.0")
	w("PRODID:-//iclasspro-pp-cli//iClassPro catalog export//EN")
	w("CALSCALE:GREGORIAN")
	w("METHOD:PUBLISH")

	dtstamp := stamp.UTC().Format("20060102T150405Z")
	skipped := 0
	for _, e := range ents {
		emitted := 0
		for i, s := range e.Slots {
			day, ok := parseDay(s.Date)
			if !ok {
				continue
			}
			sh, sm, hasStart := parseClock(s.StartTime)
			eh, em, hasEnd := parseClock(s.EndTime)

			w("BEGIN:VEVENT")
			w(fmt.Sprintf("UID:%s-%d-%d@iclasspro-pp-cli", strings.ReplaceAll(e.Key(), "/", "-"), i, day.Unix()))
			w("DTSTAMP:" + dtstamp)
			if hasStart {
				start := time.Date(day.Year(), day.Month(), day.Day(), sh, sm, 0, 0, time.UTC)
				w("DTSTART:" + start.Format("20060102T150405"))
				if hasEnd {
					end := time.Date(day.Year(), day.Month(), day.Day(), eh, em, 0, 0, time.UTC)
					if !end.After(start) {
						end = start.Add(time.Hour)
					}
					w("DTEND:" + end.Format("20060102T150405"))
				} else {
					w("DTEND:" + start.Add(time.Hour).Format("20060102T150405"))
				}
			} else {
				w("DTSTART;VALUE=DATE:" + day.Format("20060102"))
				w("DTEND;VALUE=DATE:" + day.AddDate(0, 0, 1).Format("20060102"))
			}
			w("SUMMARY:" + icsEscape(e.Name))
			desc := describeForCalendar(e)
			if desc != "" {
				w("DESCRIPTION:" + icsEscape(desc))
			}
			if e.RoomName != "" {
				w("LOCATION:" + icsEscape(e.RoomName))
			}
			if e.PortalURL != "" {
				w("URL:" + e.PortalURL)
			}
			w("END:VEVENT")
			emitted++
		}
		if emitted == 0 {
			skipped++
		}
	}
	w("END:VCALENDAR")
	return b.String(), skipped
}

func describeForCalendar(e Entity) string {
	parts := make([]string, 0, 4)
	if e.ProgramName != "" {
		parts = append(parts, "Program: "+e.ProgramName)
	}
	if e.MinAge > 0 || e.MaxAge > 0 {
		parts = append(parts, fmt.Sprintf("Ages %d-%d", e.MinAge, e.MaxAge))
	}
	parts = append(parts, "Availability: "+e.Status())
	if len(e.Instructors) > 0 {
		parts = append(parts, "Instructors: "+strings.Join(e.Instructors, ", "))
	}
	if e.PortalURL != "" {
		parts = append(parts, "Register: "+e.PortalURL)
	}
	return strings.Join(parts, "\n")
}

// ---------- cross-tenant compare ----------

// CompareRow is one account's aggregate for a comparison bucket.
type CompareRow struct {
	Account       string  `json:"account"`
	Bucket        string  `json:"bucket"`
	Entities      int     `json:"entities"`
	TotalOpenings int     `json:"total_openings"`
	WithOpenings  int     `json:"with_openings"`
	Full          int     `json:"full"`
	AvgOpenings   float64 `json:"avg_openings"`
	MinAge        int     `json:"min_age,omitempty"`
	MaxAge        int     `json:"max_age,omitempty"`
}

// Compare aggregates entities per account and bucket. The bucket is the program
// name when the portal supplies one and the entity kind otherwise, because
// program names are tenant-specific and will not line up across gyms.
func Compare(ents []Entity) []CompareRow {
	type agg struct {
		row CompareRow
	}
	byKey := map[string]*agg{}
	order := make([]string, 0)
	for _, e := range ents {
		bucket := e.ProgramName
		if strings.TrimSpace(bucket) == "" {
			bucket = e.Kind
		}
		k := e.Account + "\x00" + bucket
		a, ok := byKey[k]
		if !ok {
			a = &agg{row: CompareRow{Account: e.Account, Bucket: bucket, MinAge: e.MinAge, MaxAge: e.MaxAge}}
			byKey[k] = a
			order = append(order, k)
		}
		a.row.Entities++
		a.row.TotalOpenings += e.Openings
		if e.Openings > 0 {
			a.row.WithOpenings++
		} else {
			a.row.Full++
		}
		if e.MinAge > 0 && (a.row.MinAge == 0 || e.MinAge < a.row.MinAge) {
			a.row.MinAge = e.MinAge
		}
		if e.MaxAge > a.row.MaxAge {
			a.row.MaxAge = e.MaxAge
		}
	}
	sort.Strings(order)
	out := make([]CompareRow, 0, len(order))
	for _, k := range order {
		r := byKey[k].row
		if r.Entities > 0 {
			r.AvgOpenings = round2(float64(r.TotalOpenings) / float64(r.Entities))
		}
		out = append(out, r)
	}
	return out
}
