// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package napcamp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/cliutil"
)

type Object = map[string]any
type Reader interface {
	GetNoCache(context.Context, string, map[string]string) (json.RawMessage, error)
}
type API struct{ Reader Reader }

const Origin = "https://www.nap-camp.com"
const Qualification = "Source starting-price evidence only; full group, tax, option and dated total are unknown."

var jst = time.FixedZone("JST", 9*3600)
var idRE = regexp.MustCompile("^[1-9][0-9]{0,10}$")
var prefRE = regexp.MustCompile("^[a-z_]+$")

func ValidateID(id string) error {
	if !idRE.MatchString(id) {
		return fmt.Errorf("ID %q must be a positive numeric campsite or plan ID", id)
	}
	return nil
}
func ValidateMonth(month string) (time.Time, error) {
	t, e := time.ParseInLocation("2006-01", month, jst)
	if e != nil || t.Format("2006-01") != month {
		return time.Time{}, fmt.Errorf("--month must be YYYY-MM in JST")
	}
	return t, nil
}
func ValidateDates(in, out string) error {
	if in == "" && out == "" {
		return nil
	}
	if in == "" || out == "" {
		return errors.New("--check-in and --check-out must be supplied together")
	}
	a, e := time.ParseInLocation("2006-01-02", in, jst)
	if e != nil || a.Format("2006-01-02") != in {
		return errors.New("--check-in must be a real YYYY-MM-DD JST date")
	}
	b, e := time.ParseInLocation("2006-01-02", out, jst)
	if e != nil || b.Format("2006-01-02") != out || !b.After(a) {
		return errors.New("--check-out must be a real JST date after --check-in")
	}
	if b.Sub(a) > 31*24*time.Hour {
		return errors.New("a search stay is bounded to 31 nights")
	}
	return nil
}

// Nap Camp supplies rich-text rules as HTML strings. Keep their text and
// paragraph boundaries without exposing markup in agent-facing evidence.
var sourceBlockRE = regexp.MustCompile(`(?is)<(?:script|style)\b[^>]*>.*?</(?:script|style)\s*>`)
var sourceBreakRE = regexp.MustCompile(`(?i)<br\s*/?>|</(?:p|div|li|tr|h[1-6])\s*>`)
var sourceTagRE = regexp.MustCompile(`(?s)</?[A-Za-z][^>]*>|<![^>]*>`)

func text(v any) string {
	s, _ := v.(string)
	s = cliutil.CleanText(s)
	s = sourceBlockRE.ReplaceAllString(s, "")
	s = sourceBreakRE.ReplaceAllString(s, "\n")
	s = sourceTagRE.ReplaceAllString(s, "")
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}
func object(v any) Object {
	m, _ := v.(map[string]any)
	if m == nil {
		return Object{}
	}
	return m
}
func list(v any) []any {
	a, _ := v.([]any)
	if a == nil {
		return []any{}
	}
	return a
}
func number(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case json.Number:
		n, e := x.Float64()
		return n, e == nil
	case string:
		n, e := strconv.ParseFloat(x, 64)
		return n, e == nil
	}
	return 0, false
}
func integer(v any) int { n, _ := number(v); return int(n) }
func id(v any) string {
	n, ok := number(v)
	if !ok || n <= 0 || math.Trunc(n) != n || math.IsNaN(n) || math.IsInf(n, 0) {
		return ""
	}
	return strconv.FormatInt(int64(n), 10)
}
func tri(v any) string {
	n, ok := number(v)
	if !ok {
		return "unknown"
	}
	if n == 1 {
		return "yes"
	}
	if n == 0 {
		return "no"
	}
	return "unknown"
}
func limitedText(v any, field string, truncated *[]string) string {
	s := text(v)
	r := []rune(s)
	if len(r) > 4000 {
		*truncated = append(*truncated, field)
		return string(r[:4000])
	}
	return s
}
func sourcePrice(v any, calendar bool) Object {
	m := object(v)
	p := Object{"currency": "JPY", "qualifier": "starting_price", "full_total": nil, "qualification": Qualification}
	for _, k := range []string{"guideline", "base", "unit"} {
		n, ok := number(m[k])
		if !ok || n <= 0 {
			p[k+"_jpy"] = nil
		} else {
			p[k+"_jpy"] = n
		}
	}
	p["source_tax_flag"] = m["tax_flg"]
	if calendar {
		p["zero_price_meaning"] = "unknown, including nonaccepting dates; never free"
	}
	return p
}

var vehicleLabels = map[int]string{24: "乗用車", 25: "トレーラー", 26: "キャンピングカー", 89: "バイク"}
var facilityLabels = map[int]string{43: "AC電源", 44: "バリアフリー", 48: "シャワー", 49: "お風呂", 50: "ランドリー", 51: "ウォッシュレット式トイレ", 52: "売店・自動販売機", 53: "レストラン・食堂", 56: "給湯", 57: "炊事棟", 58: "ゴミ捨て場", 47: "ペットOK"}

func labels(v any, dictionary map[int]string) []Object {
	rows := make([]Object, 0)
	for _, n := range list(v) {
		key := integer(n)
		label, ok := dictionary[key]
		if !ok {
			label = "unknown source label"
		}
		rows = append(rows, Object{"id": key, "label": label, "scope": "facility", "fees": "unknown"})
	}
	return rows
}
func Canonical(c Object) string {
	p := text(c["prefecture_name_en"])
	i := id(c["id"])
	if !prefRE.MatchString(p) || i == "" {
		return ""
	}
	return Origin + "/" + p + "/" + i
}

func NormalizeCampsite(raw Object) Object {
	trunc := make([]string, 0)
	masters := object(raw["master_list"])
	c := Object{"id": id(raw["id"]), "name": text(raw["name"]), "prefecture": text(raw["prefecture_name"]), "prefecture_slug": text(raw["prefecture_name_en"]), "region": text(raw["region_name"]), "area": text(raw["area_name"]), "canonical_url": Canonical(raw), "source_url": Origin + "/api/campsite/" + id(raw["id"]), "vehicle_categories": labels(masters["available_vehicle"], vehicleLabels), "facility_labels": labels(masters["equipment"], facilityLabels), "price_range_source": text(raw["price_range"]), "full_dated_total": nil}
	for dst, src := range map[string]string{"season": "season_info", "closed_days": "holiday_info", "fees": "charge_info", "facilities_text": "site_info", "parking": "parking_info", "rules": "rule_info", "check_in": "checkin_info", "check_out": "checkout_info"} {
		c[dst] = limitedText(raw[src], dst, &trunc)
	}
	c["text_truncated_fields"] = trunc
	c["category_qualification"] = "Facility labels do not prove every pitch or every vehicle size is accepted."
	return c
}

func NormalizePitch(raw Object, campsite Object) Object {
	b := object(raw["basic_info"])
	trunc := make([]string, 0)
	memo := text(b["site_is_prohibited_vehicle_other_memo"])
	var capacity any
	if n, ok := number(b["site_num"]); ok && n > 0 && math.Trunc(n) == n {
		capacity = int(n)
	}
	u := text(campsite["canonical_url"])
	if u != "" {
		u += "/plans/" + id(raw["id"])
	}
	p := Object{"id": id(raw["id"]), "name": text(raw["site_name"]), "canonical_url": u, "source_url": Origin + "/api/campsite/" + fmt.Sprint(campsite["id"]) + "/plans/" + id(raw["id"]), "type": text(b["master_site_type"]), "capacity": capacity, "pitch_area_source": text(b["site_scale"]), "area_units": "source square metres when specified", "vehicle_entry": tri(b["site_is_prohibited_vehicle_type"]), "vehicle_memo": memo, "power": tri(b["site_ac_type"]), "power_memo": text(b["site_ac_memo"]), "pets": tri(b["site_pet_type"]), "pets_memo": text(b["site_pet_memo"]), "check_in": text(b["check_in"]), "check_in_limit": text(b["check_in_limit"]), "check_out": text(b["check_out"]), "price": sourcePrice(raw["price"], false), "vehicle_dimensions": Object{"length_m": nil, "width_m": nil, "height_m": nil, "status": "unknown", "qualification": "Pitch area alone cannot establish vehicle clearance."}}
	p["description"] = limitedText(raw["site_main"], "description", &trunc)
	p["facilities_text"] = limitedText(raw["facilities"], "facilities_text", &trunc)
	payment := object(raw["payment"])
	other := object(raw["other"])
	p["fees"] = limitedText(payment["rsvmast_base_charge_memo"], "fees", &trunc)
	p["cancellation"] = limitedText(payment["rsvmast_base_cancel_memo"], "cancellation", &trunc)
	p["rules"] = limitedText(other["rsvmast_base_sonota_memo"], "rules", &trunc)
	p["sales_timing"] = text(raw["sales_timing"])
	p["text_truncated_fields"] = trunc
	p["qualification"] = "Confirm the selected pitch, campervan category, physical dimensions, season, fees and dated booking conditions at the source."
	return p
}

func (a API) get(ctx context.Context, path string, q map[string]string, dst any) error {
	raw, e := a.Reader.GetNoCache(ctx, path, q)
	if e != nil {
		return fmt.Errorf("Nap Camp source GET %s: %w", path, e)
	}
	if len(raw) > 4*1024*1024 {
		return errors.New("source response exceeds the 4 MiB planning bound; narrow the query")
	}
	if e = json.Unmarshal(raw, dst); e != nil {
		return fmt.Errorf("source contract at %s: %w", path, e)
	}
	return nil
}
func (a API) Campsite(ctx context.Context, cid string) (Object, error) {
	var raw Object
	if e := a.get(ctx, "/api/campsite/"+cid, nil, &raw); e != nil {
		return nil, e
	}
	if id(raw["id"]) != cid || text(raw["name"]) == "" {
		return nil, fmt.Errorf("source returned no campsite object for ID %s", cid)
	}
	return NormalizeCampsite(raw), nil
}
func (a API) Pitch(ctx context.Context, cid, pid string, c Object) (Object, error) {
	var raw Object
	if e := a.get(ctx, "/api/campsite/"+cid+"/plans/"+pid, nil, &raw); e != nil {
		return nil, e
	}
	if id(raw["id"]) != pid || text(raw["site_name"]) == "" {
		return nil, fmt.Errorf("source returned no plan object for ID %s", pid)
	}
	return NormalizePitch(raw, c), nil
}
func (a API) Plans(ctx context.Context, cid string, c Object, limit int) (Object, error) {
	var raw Object
	if e := a.get(ctx, "/api/campsite/"+cid+"/plans", nil, &raw); e != nil {
		return nil, e
	}
	v, ok := raw["list"].([]any)
	if !ok {
		return nil, errors.New("source plans object has no list array")
	}
	rows := make([]Object, 0)
	for _, x := range v {
		if len(rows) >= limit {
			break
		}
		p := NormalizePitch(object(x), c)
		rows = append(rows, Object{"id": p["id"], "name": p["name"], "canonical_url": p["canonical_url"], "type": p["type"], "capacity": p["capacity"], "pitch_area_source": p["pitch_area_source"], "vehicle_entry": p["vehicle_entry"], "vehicle_memo": p["vehicle_memo"], "power": p["power"], "pets": p["pets"], "price": p["price"]})
	}
	return Object{"plans": rows, "source_count": len(v), "returned": len(rows), "truncated": len(v) > len(rows)}, nil
}

type Query struct {
	RegionID, PrefectureID, AreaID, Page, Limit int
	Filters                                     []int
	Keyword, CheckIn, CheckOut                  string
}

func (a API) Discover(ctx context.Context, q Query) (Object, error) {
	ids := []string{"26"}
	seen := map[int]bool{26: true}
	for _, n := range q.Filters {
		if !seen[n] {
			ids = append(ids, strconv.Itoa(n))
			seen[n] = true
		}
	}
	p := map[string]string{"other": strings.Join(ids, ","), "sort_id": "21", "page_id": strconv.Itoa(q.Page), "count": strconv.Itoa(q.Limit)}
	for k, n := range map[string]int{"region_id": q.RegionID, "prefecture_id": q.PrefectureID, "area_id": q.AreaID} {
		if n > 0 {
			p[k] = strconv.Itoa(n)
		}
	}
	if q.Keyword != "" {
		p["freeword"] = q.Keyword
	}
	if q.CheckIn != "" {
		p["check_in"] = q.CheckIn
		p["check_out"] = q.CheckOut
	}
	var raw Object
	if e := a.get(ctx, "/api/search", p, &raw); e != nil {
		return nil, e
	}
	v, ok := raw["result"].([]any)
	if !ok {
		return nil, errors.New("source search object has no result array")
	}
	rows := make([]Object, 0)
	for _, x := range v {
		if len(rows) >= q.Limit {
			break
		}
		c := object(x)
		plans := make([]Object, 0)
		for _, x := range list(c["plans"]) {
			if len(plans) >= 3 {
				break
			}
			p := NormalizePitch(object(x), Object{"id": id(c["id"]), "canonical_url": Canonical(c)})
			plans = append(plans, Object{"id": p["id"], "name": p["name"], "canonical_url": p["canonical_url"], "capacity": p["capacity"], "power": p["power"], "vehicle_entry": p["vehicle_entry"], "price": p["price"]})
		}
		rev := object(object(c["review"])["reviewsInfo"])
		rows = append(rows, Object{"id": id(c["id"]), "name": text(c["name"]), "prefecture": text(c["prefecture_name"]), "area": text(c["area_name"]), "canonical_url": Canonical(c), "campervan_entry_category": true, "rating_source": object(rev["points"])["total"], "review_count": rev["total_count"], "plan_previews": plans})
	}
	info := object(raw["info"])
	return Object{"observed_at": Now(), "time_zone": "Asia/Tokyo", "source_url": Origin + "/api/search", "query": p, "source_total": info["total_count"], "source_page": info["current_page_id"], "has_next_page": info["is_next_page"], "returned": len(rows), "results": rows, "coverage": "One explicitly requested source page; Japanese listings, not a complete offline catalog.", "check_in": q.CheckIn, "check_out": q.CheckOut, "inventory": "unknown; dated search membership is not a guarantee of vacancy", "qualification": "Campervan-entry category only. Inspect each selected pitch and physical vehicle dimensions."}, nil
}
func Now() string { return time.Now().UTC().Format(time.RFC3339) }
func (a API) Catalog(ctx context.Context, kind string, limit int) (Object, error) {
	rows := make([]Object, 0)
	sourceCount := 0
	if kind == "filters" {
		var raw []Object
		if e := a.get(ctx, "/api/master", nil, &raw); e != nil {
			return nil, e
		}
		if raw == nil {
			return nil, errors.New("source master must be an array, not null")
		}
		for _, g := range raw {
			entries, ok := g["master_list"].([]any)
			if !ok || id(g["id"]) == "" || text(g["type_name"]) == "" {
				return nil, errors.New("source master group has no valid id/name/master_list array")
			}
			for _, x := range entries {
				entry := object(x)
				if id(entry["id"]) == "" || text(entry["ja"]) == "" {
					return nil, errors.New("source master item has no valid id/name")
				}

				sourceCount++
				if len(rows) < limit {
					m := object(x)
					rows = append(rows, Object{"id": m["id"], "name": text(m["ja"]), "group_id": g["id"], "group_name": text(g["type_name"])})
				}
			}
		}
	} else {
		var raw Object
		if e := a.get(ctx, "/api/locations", nil, &raw); e != nil {
			return nil, e
		}
		regions, ok := raw["list_r"].([]any)
		if !ok {
			return nil, errors.New("source locations has no list_r array")
		}
		for _, x := range regions {
			r := object(x)
			basic := object(r["basic"])
			if id(r["id"]) == "" || text(basic["ja"]) == "" || text(basic["en"]) == "" {
				return nil, errors.New("source region has no valid id/name/slug")
			}

			if kind == "regions" {
				sourceCount++
				if len(rows) < limit {
					b := object(r["basic"])
					rows = append(rows, Object{"id": r["id"], "name": text(b["ja"]), "slug": text(b["en"]), "source_count": object(r["extension"])["count"]})
				}
			} else {
				prefectures, ok := r["list_p"].([]any)
				if !ok {
					return nil, errors.New("source region has no list_p array")
				}
				for _, x := range prefectures {
					p := object(x)
					basic := object(p["basic"])
					if id(p["id"]) == "" || text(basic["ja"]) == "" || text(basic["en"]) == "" {
						return nil, errors.New("source prefecture has no valid id/name/slug")
					}

					sourceCount++
					if len(rows) < limit {
						b := object(p["basic"])
						rows = append(rows, Object{"id": p["id"], "name": text(b["ja"]), "slug": text(b["en"]), "region_id": b["region_id"], "source_count": object(p["extension"])["count"]})
					}
				}
			}
		}
	}
	return Object{"observed_at": Now(), "kind": kind, "results": rows, "returned": len(rows), "source_count": sourceCount, "truncated": sourceCount > len(rows), "qualification": "Source facility-category labels and counts; inspect selected pitches."}, nil
}

func NormalizeCalendar(raw []Object) ([]Object, error) {
	if raw == nil {
		return nil, errors.New("source calendar must be an array, not null")
	}
	days := make([]Object, 0, len(raw))
	seen := map[string]bool{}
	names := map[int]string{0: "準備中", 1: "受付中", 2: "残りわずか", 3: "受付終了", 4: "キャンセル発生通知対象"}
	for _, r := range raw {
		d := list(r["date"])
		if len(d) != 3 {
			return nil, errors.New("calendar date must contain year, month, day")
		}
		for _, part := range d {
			n, ok := number(part)
			if !ok || math.Trunc(n) != n || math.IsNaN(n) || math.IsInf(n, 0) {
				return nil, errors.New("calendar year/month/day must be whole numbers")
			}
		}
		y, m, n := integer(d[0]), integer(d[1]), integer(d[2])
		t := time.Date(y, time.Month(m), n, 0, 0, 0, 0, jst)
		if t.Year() != y || int(t.Month()) != m || t.Day() != n {
			return nil, errors.New("source calendar contains an invalid date")
		}
		date := t.Format("2006-01-02")
		if seen[date] {
			return nil, fmt.Errorf("duplicate source calendar date %s", date)
		}
		seen[date] = true
		status, known := number(r["status"])
		code := -1
		if known && math.Trunc(status) == status && !math.IsNaN(status) && !math.IsInf(status, 0) {
			code = int(status)
		}
		name, ok := names[code]
		if !ok {
			name = "unknown source status"
		}
		days = append(days, Object{"date": date, "time_zone": "Asia/Tokyo", "source_status": code, "raw_source_status": r["status"], "source_label": name, "acceptance_candidate": code == 1 || code == 2, "vacancy": "unknown", "price": sourcePrice(r["price"], true)})
	}
	sort.Slice(days, func(i, j int) bool { return days[i]["date"].(string) < days[j]["date"].(string) })
	return days, nil
}
func (a API) Calendar(ctx context.Context, cid, pid, month string) ([]Object, error) {
	var raw []Object
	if e := a.get(ctx, "/api/campsite/"+cid+"/plans/"+pid+"/reservation", map[string]string{"month": month}, &raw); e != nil {
		return nil, e
	}
	if len(raw) > 62 {
		return nil, errors.New("source calendar exceeds two-month 62-day bound")
	}
	return NormalizeCalendar(raw)
}
func CalendarView(days []Object, month string, limit int) Object {
	out := make([]Object, 0)
	total := 0
	for _, d := range days {
		if strings.HasPrefix(fmt.Sprint(d["date"]), month+"-") {
			total++
			if len(out) < limit {
				out = append(out, d)
			}
		}
	}
	return Object{"requested_month": month, "time_zone": "Asia/Tokyo", "observed_at": Now(), "source_days_in_month": total, "returned": len(out), "truncated": total > len(out), "days": out, "qualification": "Source acceptance/starting prices only; inventory and full dated total unknown."}
}

type Requirements struct {
	People                int
	Power, Pets           bool
	Length, Width, Height float64
}

func Fit(c, p Object, r Requirements) Object {
	checks := make([]Object, 0)
	add := func(k, state, e string) {
		checks = append(checks, Object{"requirement": k, "status": state, "evidence": e})
	}
	v := text(p["vehicle_entry"])
	memo := text(p["vehicle_memo"])
	s := "unknown"
	if v == "no" {
		s = "contradiction"
	}
	add("campervan_entry", s, "Selected-pitch entry="+v+"; source memo="+memo+". Facility campervan category does not establish this pitch's vehicle category.")
	add("vehicle_dimensions", "unknown", fmt.Sprintf("Vehicle requested %.2f m length × %.2f m width × %.2f m height. Source provides no validated vehicle clearance; pitch area is insufficient.", r.Length, r.Width, r.Height))
	if r.People > 0 {
		state := "unknown"
		e := "source capacity unknown"
		if n, ok := number(p["capacity"]); ok {
			state = "verified"
			if r.People > int(n) {
				state = "contradiction"
			}
			e = fmt.Sprintf("%d requested; source capacity %d, check group-wide rules", r.People, int(n))
		}
		add("group_capacity", state, e)
	}
	for _, q := range []struct {
		Want bool
		Key  string
	}{{r.Power, "power"}, {r.Pets, "pets"}} {
		if q.Want {
			state := "unknown"
			if p[q.Key] == "yes" {
				state = "verified"
			} else if p[q.Key] == "no" {
				state = "contradiction"
			}
			add(q.Key, state, "selected-pitch source value="+fmt.Sprint(p[q.Key]))
		}
	}
	add("full_dated_total", "unknown", Qualification)
	add("season_and_rules", "unknown", "Confirm operating season and all selected-plan/group/vehicle restrictions at the canonical source.")
	verdict := "needs_confirmation"
	for _, x := range checks {
		if x["status"] == "contradiction" {
			verdict = "contradiction"
		}
	}
	campSummary := Object{"id": c["id"], "name": c["name"], "canonical_url": c["canonical_url"], "source_url": c["source_url"], "season": c["season"], "category_qualification": c["category_qualification"]}
	pitchSummary := Object{}
	for _, key := range []string{"id", "name", "canonical_url", "source_url", "capacity", "pitch_area_source", "vehicle_entry", "vehicle_memo", "vehicle_dimensions", "power", "power_memo", "pets", "pets_memo", "check_in", "check_in_limit", "check_out", "price"} {
		pitchSummary[key] = p[key]
	}
	return Object{"campsite": campSummary, "pitch": pitchSummary, "checks": checks, "verdict": verdict, "observed_at": Now(), "qualification": "This is an evidence checklist, not vehicle fit certification or booking availability. Read pitch inspect for full rules and fees."}
}
func Windows(days []Object, month string, nights, limit int) (Object, error) {
	start, e := ValidateMonth(month)
	if e != nil {
		return nil, e
	}
	if nights < 1 || nights > 14 {
		return nil, errors.New("--nights must be 1..14")
	}
	if limit < 1 || limit > 50 {
		return nil, errors.New("--limit must be 1..50")
	}
	byDate := map[string]Object{}
	dates := make([]string, 0)
	for _, d := range days {
		key := fmt.Sprint(d["date"])
		byDate[key] = d
		dates = append(dates, key)
	}
	windows := make([]Object, 0)
	unknown := map[string]bool{}
	count := 0
	for t := start; t.Before(start.AddDate(0, 1, 0)); t = t.AddDate(0, 0, 1) {
		valid := true
		states := make([]Object, 0)
		var sum float64
		allPrices := true
		for n := 0; n < nights; n++ {
			date := t.AddDate(0, 0, n).Format("2006-01-02")
			d, ok := byDate[date]
			if !ok {
				unknown[date] = true
				valid = false
				continue
			}
			if d["acceptance_candidate"] != true {
				valid = false
			}
			p := object(d["price"])
			price, ok := number(p["guideline_jpy"])
			if !ok {
				allPrices = false
			} else {
				sum += price
			}
			states = append(states, Object{"date": date, "source_status": d["source_status"], "source_label": d["source_label"], "starting_price_jpy": p["guideline_jpy"]})
		}
		if valid {
			count++
			if len(windows) < limit {
				var price any
				if allPrices {
					price = sum
				}
				windows = append(windows, Object{"check_in": t.Format("2006-01-02"), "check_out": t.AddDate(0, 0, nights).Format("2006-01-02"), "nights": nights, "night_evidence": states, "sum_starting_prices_jpy": price, "full_total": nil, "vacancy": "unknown"})
			}
		}
	}
	missing := make([]string, 0, len(unknown))
	for x := range unknown {
		missing = append(missing, x)
	}
	sort.Strings(missing)
	return Object{"requested_start_month": month, "source_dates": dates, "candidate_windows": windows, "candidate_count": count, "returned": len(windows), "truncated": count > len(windows), "unknown_dates": missing, "time_zone": "Asia/Tokyo", "observed_at": Now(), "qualification": "Evaluate stay nights excluding checkout. Acceptance candidates are not guaranteed vacancy; sum of starting prices is not a full quote."}, nil
}

type Snapshot struct {
	SchemaVersion  int      `json:"schema_version"`
	ObservedAt     string   `json:"observed_at"`
	Campsite       Object   `json:"campsite"`
	Pitch          Object   `json:"pitch"`
	Calendar       []Object `json:"calendar"`
	RequestedMonth string   `json:"requested_month"`
	SourceURLs     []string `json:"source_urls"`
}

func (a API) Snapshot(ctx context.Context, cid, pid, month string) (Snapshot, error) {
	s := Snapshot{SchemaVersion: 1, ObservedAt: Now(), Calendar: make([]Object, 0), SourceURLs: []string{Origin + "/api/campsite/" + cid, Origin + "/api/campsite/" + cid + "/plans/" + pid}, RequestedMonth: month}
	c, e := a.Campsite(ctx, cid)
	if e != nil {
		return s, e
	}
	p, e := a.Pitch(ctx, cid, pid, c)
	if e != nil {
		return s, e
	}
	s.Campsite = c
	s.Pitch = p
	if month != "" {
		days, e := a.Calendar(ctx, cid, pid, month)
		if e != nil {
			return s, e
		}
		for _, d := range days {
			if strings.HasPrefix(fmt.Sprint(d["date"]), month+"-") {
				s.Calendar = append(s.Calendar, d)
			}
		}
		s.SourceURLs = append(s.SourceURLs, Origin+"/api/campsite/"+cid+"/plans/"+pid+"/reservation?month="+month)
	}
	return s, nil
}
func SaveSnapshot(path string, s Snapshot) error {
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return e
	}
	if len(b) > 2*1024*1024 {
		return errors.New("snapshot exceeds 2 MiB bound")
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".nap-snapshot-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(0600); e != nil {
		_ = f.Close() // Preserve the primary chmod/write/sync failure.
		return e
	}
	if _, e = f.Write(append(b, '\n')); e != nil {
		_ = f.Close() // Preserve the primary chmod/write/sync failure.
		return e
	}
	if e = f.Sync(); e != nil {
		_ = f.Close() // Preserve the primary chmod/write/sync failure.
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if e = os.Link(tmp, path); e != nil {
		return fmt.Errorf("save snapshot without overwriting %q: %w", path, e)
	}
	return nil
}
func LoadSnapshot(path string) (Snapshot, error) {
	s := Snapshot{}
	f, e := os.Open(path) // #nosec G304 -- Caller explicitly selects a snapshot file; read is capped at 2 MiB, schema checked, and never executed.
	if e != nil {
		return s, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, 2*1024*1024+1))
	if e != nil {
		return s, e
	}
	if len(b) > 2*1024*1024 {
		return s, errors.New("snapshot exceeds 2 MiB bound")
	}
	if e = json.Unmarshal(b, &s); e != nil {
		return s, e
	}
	if s.SchemaVersion != 1 || s.Campsite["id"] == nil || s.Pitch["id"] == nil || s.ObservedAt == "" {
		return s, errors.New("expected schema_version=1 Nap Camp snapshot with IDs and observed_at")
	}
	if s.Calendar == nil {
		s.Calendar = make([]Object, 0)
	}
	return s, nil
}
func Diff(a, b Snapshot) (Object, error) {
	if a.SchemaVersion != 1 || b.SchemaVersion != 1 || fmt.Sprint(a.Campsite["id"]) != fmt.Sprint(b.Campsite["id"]) || fmt.Sprint(a.Pitch["id"]) != fmt.Sprint(b.Pitch["id"]) {
		return nil, errors.New("observations must have schema_version=1 and identical campsite/pitch IDs")
	}
	changes := make([]Object, 0)
	compare := func(prefix string, x, y Object) {
		keys := map[string]bool{}
		for k := range x {
			keys[k] = true
		}
		for k := range y {
			keys[k] = true
		}
		ks := make([]string, 0)
		for k := range keys {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			if !reflect.DeepEqual(x[k], y[k]) {
				changes = append(changes, Object{"field": prefix + "." + k, "before": x[k], "after": y[k]})
			}
		}
	}
	compare("campsite", a.Campsite, b.Campsite)
	compare("pitch", a.Pitch, b.Pitch)
	old, new := map[string]Object{}, map[string]Object{}
	for _, d := range a.Calendar {
		old[fmt.Sprint(d["date"])] = d
	}
	for _, d := range b.Calendar {
		new[fmt.Sprint(d["date"])] = d
	}
	onlyBefore, onlyAfter := make([]string, 0), make([]string, 0)
	for date, d := range old {
		if n, ok := new[date]; ok {
			compare("calendar."+date, d, n)
		} else {
			onlyBefore = append(onlyBefore, date)
		}
	}
	for date := range new {
		if _, ok := old[date]; !ok {
			onlyAfter = append(onlyAfter, date)
		}
	}
	sort.Strings(onlyBefore)
	sort.Strings(onlyAfter)
	sort.Slice(changes, func(i, j int) bool { return fmt.Sprint(changes[i]["field"]) < fmt.Sprint(changes[j]["field"]) })
	return Object{"campsite_id": a.Campsite["id"], "pitch_id": a.Pitch["id"], "before_observed_at": a.ObservedAt, "after_observed_at": b.ObservedAt, "changes": changes, "dates_only_before": onlyBefore, "dates_only_after": onlyAfter, "coverage_changed": a.RequestedMonth != b.RequestedMonth || len(onlyBefore) > 0 || len(onlyAfter) > 0, "qualification": "Differences between saved observations; current inventory and full totals remain unknown."}, nil
}
