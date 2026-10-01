package evidence

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var idRE = regexp.MustCompile(`^[0-9]{1,8}$`)
var slugRE = regexp.MustCompile(`^[a-z]{2,16}$`)
var mdRE = regexp.MustCompile(`^(?:(20[0-9]{2})(?:年|[-/]))?([0-9]{1,2})(?:月|/|-)([0-9]{1,2})(?:日|日時点)?$`)
var sakuraStatusRE = regexp.MustCompile(`(?s)<h3\b[^>]*class="kaikaStatus__title"[^>]*>(.*?)</h3>`)

func productOK(p string) error {
	if p != "sakura" && p != "koyo" {
		return fail(2, "--product must be sakura or koyo")
	}
	return nil
}
func dateEvidence(v any, year int) any {
	s := strings.TrimSpace(str(v))
	m := mdRE.FindStringSubmatch(s)
	if len(m) < 4 {
		return nil
	}
	if m[1] != "" {
		explicit, _ := strconv.Atoi(m[1])
		if explicit != year {
			return nil
		}
	}
	month, _ := strconv.Atoi(m[2])
	day, _ := strconv.Atoi(m[3])
	d := time.Date(year, time.Month(month), day, 0, 0, 0, 0, JST)
	if int(d.Month()) != month || d.Day() != day {
		return nil
	}
	return d.Format("2006-01-02")
}
func reportAge(date any) any {
	if date == nil {
		return nil
	}
	d, e := ParseDate(str(date))
	if e != nil {
		return nil
	}
	now := time.Now().In(JST)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, JST)
	return int(today.Sub(d).Hours() / 24)
}
func (c *Client) SeasonSearch(ctx context.Context, product, area, query string, limit, offset int) (map[string]any, error) {
	if e := productOK(product); e != nil {
		return nil, e
	}
	if e := bounds(limit, offset); e != nil {
		return nil, e
	}
	query = strings.TrimSpace(query)
	if len([]rune(query)) > 100 {
		return nil, fail(2, "--query is bounded to 100 characters")
	}
	if area == "" && query == "" {
		return nil, fail(2, "supply --area kyoto or --query with Japanese place words")
	}
	if area != "" && !slugRE.MatchString(area) {
		return nil, fail(2, "--area must be a Weathernews prefecture/region slug such as kyoto, hokkaido or kanto")
	}
	raw := "https://weathernews.jp/" + product + "/search_result.html"
	if area != "" {
		raw = "https://weathernews.jp/" + product + "/area/" + area + "/"
	}
	f, e := c.Get(ctx, raw, time.Hour)
	if e != nil {
		return nil, e
	}
	season, e := SeasonState(f.Body, product)
	if e != nil {
		return nil, e
	}
	data, e := Nuxt(f.Body)
	if e != nil {
		return nil, e
	}
	var rows []map[string]any
	var providerFetched any
	inventoryFound := false
	for _, entry := range data {
		v := object(entry)
		d := object(v["data"])
		var pl any
		if k := object(d["spotKanaSortedList"]); k != nil {
			pl = k["pointlist"]
		} else {
			pl = d["pointlist"]
		}
		if pl == nil {
			continue
		}
		inventoryFound = true
		providerFetched = providerTime(v["fetchedAt"])
		switch x := pl.(type) {
		case []any:
			for _, z := range x {
				rows = append(rows, object(z))
			}
		case map[string]any:
			for _, z := range x {
				if list, ok := z.([]any); ok {
					for _, a := range list {
						rows = append(rows, object(a))
					}
				}
			}
		default:
			return nil, fail(5, "Weathernews inventory schema changed")
		}
	}
	if !inventoryFound {
		return nil, fail(5, "Weathernews seasonal inventory is missing; use a prefecture or keyword search")
	}
	seen := map[string]bool{}
	items := []any{}
	terms := strings.Fields(strings.ToLower(query))
	for _, r := range rows {
		id := str(first(r, "spotid", "lcid"))
		name := str(first(r, "pointname", "name"))
		if !idRE.MatchString(id) || name == "" {
			return nil, fail(5, "season inventory identity changed")
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		hay := strings.ToLower(name + " " + str(first(r, "pointname_kana", "name_kana")) + " " + str(r["cityname"]) + " " + str(first(r, "address", "addr")) + " " + str(first(r, "pref", "pref_en")))
		match := true
		for _, term := range terms {
			if !strings.Contains(hay, term) {
				match = false
			}
		}
		if !match {
			continue
		}
		var reportedDate any
		if product == "koyo" && text(r["obsmon"]) != nil && text(r["obsday"]) != nil {
			reportedDate = dateEvidence(str(r["obsmon"])+"/"+str(r["obsday"]), season["year"].(int))
		}
		items = append(items, map[string]any{"id": id, "name_ja": name, "name_kana": text(first(r, "pointname_kana", "name_kana")), "prefecture": text(first(r, "pref", "pref_en")), "city_ja": text(r["cityname"]), "latitude": number(first(r, "latd", "lat")), "longitude": number(first(r, "lond", "lon")), "elevation_m": nil, "elevation_status": "not_provided", "url": "https://weathernews.jp/" + product + "/spot/" + id + "/", "report": map[string]any{"kind": "observation", "status_ja": text(r["rank_txt"]), "status_code": number(first(r, "obsrank", "rank")), "valid_date": reportedDate, "date_precision": "day_or_unknown", "report_age_days": reportAge(reportedDate)}, "normal": map[string]any{"kind": "historical_norm", "period_ja": text(r["reinen"]), "reference_years": nil}})
	}
	sortItems(items)
	page, next := paginate(items, limit, offset)
	return map[string]any{"season": season, "query": query, "area": text(area), "items": page, "total": len(items), "inventory_total": len(seen), "next_offset": next, "coverage": "public inventory summaries; detail retrieved lazily; member radar excluded", "query_mode": "all terms must occur in source name/kana/city/address/prefecture; no fuzzy universal relevance score", "provider_fetched_at": providerFetched, "source": provenance(f)}, nil
}
func (c *Client) SeasonShow(ctx context.Context, product, id string) (map[string]any, error) {
	if e := productOK(product); e != nil {
		return nil, e
	}
	if !idRE.MatchString(id) {
		return nil, fail(2, "--id must be the numeric source spot ID from season search")
	}
	f, e := c.Get(ctx, "https://weathernews.jp/"+product+"/spot/"+id+"/", time.Hour)
	if e != nil {
		return nil, e
	}
	season, e := SeasonState(f.Body, product)
	if e != nil {
		return nil, e
	}
	data, e := Nuxt(f.Body)
	if e != nil {
		return nil, e
	}
	var detail, report map[string]any
	var providerFetched any
	for _, v := range data {
		r := object(v)
		if str(r["spotid"]) == id {
			detail = r
		}
		d := object(r["data"])
		if d != nil {
			if _, ok := d["obsrank"]; ok {
				report = d
				providerFetched = providerTime(r["fetchedAt"])
			}
			if _, ok := d["rankText"]; ok {
				report = d
				providerFetched = providerTime(r["fetchedAt"])
			}
		}
	}
	if detail == nil {
		return nil, fail(3, "Weathernews seasonal spot %s not found", id)
	}
	if report == nil {
		return nil, fail(5, "Weathernews seasonal report schema changed")
	}
	year := season["year"].(int)
	visibility := "public"
	if report["showContent"] == false {
		visibility = "source_hidden"
	}
	var status, valid any
	code := number(report["obsrank"])
	if product == "koyo" {
		status = text(report["rankText"])
		valid = dateEvidence(report["obs"], year)
		code = number(report["rank"])
	} else {
		m := sakuraStatusRE.FindSubmatch(f.Body)
		if len(m) == 2 {
			status = text(clean(string(m[1])))
		}
		valid = dateEvidence(report["obsday"], year)
	}
	if visibility == "source_hidden" {
		status = nil
		valid = nil
		code = nil
	}
	predictions := []any{}
	fields := []struct{ field, label, flag string }{{"migoro", "peak", "showMigoroPrediction"}, {"rakuyoStart", "leaf_fall_start", "showRakuyoStartPrediction"}}
	if product == "sakura" {
		fields = []struct{ field, label, flag string }{{"kaika", "bloom_start", "prediction"}, {"gobu", "half_bloom", "prediction"}, {"mankai", "full_bloom", "prediction"}, {"fubuki", "petal_fall", "prediction"}}
	}
	for _, x := range fields {
		date := dateEvidence(report[x.field], year)
		enabled, _ := report[x.flag].(bool)
		state := "not_available"
		if visibility == "source_hidden" {
			state = "not_publicly_displayed"
			date = nil
			enabled = false
		}
		if season["updates_ended"] == true {
			state = "season_ended"
		} else if enabled && date != nil {
			state = "available"
		}
		predictions = append(predictions, map[string]any{"event": x.label, "kind": predictionKind(enabled), "valid_date": date, "source_date_ja": text(report[x.field]), "issued_at": nil, "issue_time_status": "not_provided", "status": state, "date_precision": "day", "source_enabled": enabled})
	}
	name := str(first(detail, "name", "spotname"))
	normal := text(first(detail, "reinennomigoro", "reinen"))
	state := "available"
	if season["updates_ended"] == true {
		state = "season_ended"
	}
	return map[string]any{"season": season, "status": state, "location": map[string]any{"id": id, "name_ja": name, "name_kana": text(detail["name_kana"]), "prefecture": text(first(detail, "pref", "prefname")), "city_ja": text(detail["cityname"]), "address_ja": text(first(detail, "address", "addr")), "latitude": number(first(detail, "latd", "lat")), "longitude": number(first(detail, "lond", "lon")), "elevation_m": nil, "elevation_status": "not_provided", "url": f.URL}, "report": map[string]any{"kind": "observation", "status_ja": status, "status_code": code, "valid_date": valid, "source_date_ja": text(first(report, "obs", "obsday")), "issued_at": nil, "date_precision": "day_or_unknown", "timezone": "Asia/Tokyo", "report_age_days": reportAge(valid), "current_season": season["updates_ended"] != true, "comment_ja": text(report["comment"])}, "predictions": predictions, "normal": map[string]any{"kind": "historical_norm", "period_ja": normal, "reference_years": nil, "valid_date": nil}, "travel": map[string]any{"tree_species_ja": text(detail["kind"]), "access_train_ja": text(first(detail, "access_train", "train1")), "access_car_ja": text(first(detail, "access_car", "mycar1")), "opening_hours_ja": text(first(detail, "period", "term")), "fee_ja": text(first(detail, "price", "fee"))}, "coverage": map[string]any{"access": "public", "report_visibility": visibility, "member_only_products": []string{"season radar and other subscription-only coverage are excluded"}, "elevation": "not supplied by selected public source", "historical_norm_is_forecast": false}, "provider_fetched_at": providerFetched, "source": provenance(f)}, nil
}
func PeakCheck(detail map[string]any, date string, tolerance int) (string, map[string]any) {
	season := object(detail["season"])
	target, e := ParseDate(date)
	if e != nil {
		return "unknown", map[string]any{"reason": "invalid_date"}
	}
	if season["updates_ended"] == true {
		return "unknown", map[string]any{"reason": "season_ended"}
	}
	year := int(*num(season["year"]))
	if target.Year() != year {
		return "unknown", map[string]any{"reason": "season_year_mismatch"}
	}
	preds, _ := detail["predictions"].([]any)
	for _, p := range preds {
		r := object(p)
		event := str(r["event"])
		if (event != "peak" && event != "full_bloom") || r["status"] != "available" {
			continue
		}
		peak, e := ParseDate(str(r["valid_date"]))
		if e != nil {
			continue
		}
		delta := int(math.Abs(target.Sub(peak).Hours() / 24))
		check := map[string]any{"criterion": "days_from_source_peak_prediction", "operator": "<=", "threshold_days": tolerance, "value_days": delta, "peak_prediction_date": r["valid_date"], "basis": "forecast date proximity, not an observation or guarantee of color"}
		if delta <= tolerance {
			return "meets", check
		}
		return "does_not_meet", check
	}
	return "unknown", map[string]any{"reason": "peak_prediction_not_available"}
}
func (c *Client) CompareSeason(ctx context.Context, product string, ids []string, date string, tolerance int) (map[string]any, error) {
	if e := productOK(product); e != nil {
		return nil, e
	}
	if len(ids) < 2 || len(ids) > 5 {
		return nil, fail(2, "season compare requires 2–5 comma-separated --ids")
	}
	if _, e := ParseDate(date); e != nil {
		return nil, e
	}
	if tolerance < 0 || tolerance > 45 {
		return nil, fail(2, "--max-days-from-peak must be 0–45 days")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !idRE.MatchString(id) || seen[id] {
			return nil, fail(2, "--ids must contain distinct numeric source IDs")
		}
		seen[id] = true
	}
	items := []any{}
	partial := false
	for _, id := range ids {
		d, e := c.SeasonShow(ctx, product, id)
		if e != nil {
			partial = true
			items = append(items, map[string]any{"id": id, "status": "error", "error": e.Error()})
			continue
		}
		status, check := PeakCheck(d, date, tolerance)
		items = append(items, map[string]any{"location": d["location"], "season": d["season"], "status": status, "check": check, "report": d["report"], "normal": d["normal"], "source": d["source"]})
	}
	return map[string]any{"date": date, "timezone": "Asia/Tokyo", "items": items, "partial": partial, "comparison_basis": fmt.Sprintf("caller maximum %d days from published peak prediction; normals never substitute missing forecasts", tolerance)}, nil
}

func predictionKind(enabled bool) string {
	if enabled {
		return "forecast"
	}
	return "unclassified_source_date"
}
