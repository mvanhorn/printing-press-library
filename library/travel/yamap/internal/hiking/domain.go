package hiking

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Row preserves nulls and exact source IDs. Scalars never coerce absent fields to zero.
type Row = map[string]any

func object(v any) Row { m, _ := v.(map[string]any); return m }
func number(v any) (float64, bool) {
	switch n := v.(type) {
	case json.Number:
		f, e := n.Float64()
		return f, e == nil
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}
func Num(v any) any {
	if f, ok := number(v); ok {
		return f
	}
	return nil
}
func Str(v any) string { s, _ := v.(string); return s }
func ID(v any) string {
	if n, ok := v.(json.Number); ok {
		return n.String()
	}
	if f, ok := number(v); ok {
		return strconv.FormatFloat(f, 'f', 0, 64)
	}
	return Str(v)
}
func Stamp(v any) any {
	if n, ok := number(v); ok && n > 0 {
		return time.Unix(int64(n), 0).UTC().Format(time.RFC3339)
	}
	return nil
}
func JapanDate(v any) any {
	if n, ok := number(v); ok && n > 0 {
		return time.Unix(int64(n), 0).In(time.FixedZone("Asia/Tokyo", 9*3600)).Format("2006-01-02")
	}
	return nil
}
func Excerpt(s string, limit int) (string, bool) {
	r := []rune(s)
	if len(r) <= limit {
		return s, false
	}
	return string(r[:limit]), true
}

func nullableExcerpt(v any, limit int) (any, any) {
	s, ok := v.(string)
	if !ok {
		return nil, nil
	}
	return Excerpt(s, limit)
}
func prefs(v any) []string {
	if v == nil {
		return nil
	}
	a, _ := v.([]any)
	out := []string{}
	for _, v := range a {
		m := object(v)
		s := Str(m["full_name"])
		if s == "" {
			s = Str(m["name"])
		}
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
func loc(v any) any {
	a, _ := v.([]any)
	if len(a) != 2 {
		return nil
	}
	lon, ok1 := number(a[0])
	lat, ok2 := number(a[1])
	if !ok1 || !ok2 || lon < -180 || lon > 180 || lat < -90 || lat > 90 {
		return nil
	}
	return Row{"longitude": lon, "latitude": lat}
}
func boolean(v any) any {
	if b, ok := v.(bool); ok {
		return b
	}
	return nil
}

func caution(v any) any {
	m := object(v)
	if m == nil {
		return nil
	}
	body, cut := nullableExcerpt(m["body"], 600)
	return Row{"text": body, "text_truncated": cut, "source_name": m["source_name"], "source_url": m["source_url"], "published_since": Stamp(m["publish_since"]), "authority": "publisher_relay; linked authority not verified by this CLI"}
}
func cautions(v any) Row {
	if v == nil {
		return Row{"items": nil, "source_count": nil, "truncated": nil, "authority": "publisher_notice_field_unavailable"}
	}
	a, _ := v.([]any)
	out := []any{}
	for i, x := range a {
		if i == 5 {
			break
		}
		out = append(out, caution(x))
	}
	return Row{"items": out, "source_count": len(a), "truncated": len(a) > 5, "authority": "publisher_relay; inspect linked source and date"}
}

func ParseID(s, resource string) (string, error) {
	if u, e := url.Parse(s); e == nil && u.Scheme != "" {
		if u.Scheme != "https" || u.Host != "yamap.com" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return "", fmt.Errorf("use a positive source ID or https://yamap.com/%s/ID", resource)
		}
		p := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(p) != 2 || p[0] != resource {
			return "", fmt.Errorf("expected YAMAP %s URL", resource)
		}
		s = p[1]
	}
	if s == "" || len(s) > 18 {
		return "", fmt.Errorf("source ID must contain 1–18 digits")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return "", fmt.Errorf("source ID must contain only digits")
		}
	}
	n, e := strconv.ParseUint(s, 10, 64)
	if e != nil || n == 0 {
		return "", fmt.Errorf("source ID must be positive")
	}
	return strconv.FormatUint(n, 10), nil
}

func Summary(kind string, m Row, detail bool) Row {
	id := ID(m["id"])
	resource := map[string]string{"mountain": "mountains", "route": "model-courses", "report": "activities", "map": "maps"}[kind]
	r := Row{"id": id, "kind": kind, "url": "https://yamap.com/" + resource + "/" + id, "updated_at": Stamp(m["updated_at"])}
	switch kind {
	case "mountain":
		r["name"] = m["name"]
		r["name_hira"] = m["name_hira"]
		r["altitude_m"] = Num(m["altitude"])
		r["location"] = loc(m["coord"])
		r["prefectures"] = prefs(m["prefectures"])
		r["description"] = m["short_description"]
		if detail {
			r["description"], r["description_truncated"] = nullableExcerpt(m["description"], 1200)
			r["publisher_attention"] = m["attention_info"]
			r["publisher_caution"] = caution(m["mountain_caution"])
			r["primary_map"] = mapRef(object(m["primary_map"]))
			r["ai_description_included"] = false
			r["official_closure_status"] = nil
		}
	case "route":
		r["name"] = m["name"]
		r["route_kind"] = "planned_model_course"
		r["metrics"] = Row{"distance_m": Num(m["distance"]), "ascent_m": Num(m["cumulative_up"]), "descent_m": Num(m["cumulative_down"]), "standard_time_seconds": Num(m["course_time"])}
		r["prefectures"] = prefs(m["prefectures"])
		r["map"] = mapRef(object(m["map"]))
		r["difficulty_source"] = m["difficulty_level"]
		if detail {
			r["description"], r["description_truncated"] = nullableExcerpt(m["description"], 1200)
			r["publisher_route_flags"] = Row{"closed_route_passed_through": boolean(m["is_closed_route_passed_through"]), "dashed_route_passed_through": boolean(m["is_dashed_route_passed_through"])}
			r["official_closure_status"] = nil
			r["trail_open"] = nil
			r["geometry_included"] = false
		}
	case "report":
		r["title"] = m["title"]
		r["route_kind"] = "activity_log_summary"
		r["is_planned"] = boolean(m["is_planned"])
		if b, ok := m["is_planned"].(bool); ok {
			if b {
				r["route_kind"] = "planned_activity"
			} else {
				r["route_kind"] = "recorded_trip"
			}
		}
		r["activity_date_jst"] = JapanDate(m["start_at"])
		r["started_at"] = Stamp(m["start_at"])
		r["finished_at"] = Stamp(m["finish_at"])
		r["published_at"] = Stamp(m["public_at"])
		r["map"] = mapRef(object(m["map"]))
		r["activity_type"] = object(m["activity_type"])["name"]
		metrics := Row{"distance_m": Num(m["distance"]), "ascent_m": Num(m["cumulative_up"]), "descent_m": Num(m["cumulative_down"]), "elapsed_seconds": Num(m["duration"]), "active_seconds": nil, "rest_seconds": nil, "moving_seconds": nil, "source": "legacy_activity_summary"}
		if whole := object(m["activity_whole_section"]); detail && whole != nil {
			metrics = Row{"distance_m": Num(whole["distance"]), "ascent_m": Num(whole["cumulative_up"]), "descent_m": Num(whole["cumulative_down"]), "elapsed_seconds": Num(whole["total_time"]), "active_seconds": Num(whole["active_time"]), "rest_seconds": Num(whole["rest_time"]), "moving_seconds": nil, "source": "activity_whole_section", "active_definition": "source non-rest time; rest auto-detected for stops of at least 3 minutes; exact moving time unavailable"}
		}
		r["metrics"] = metrics
		if detail {
			r["observation_text"], r["observation_truncated"] = nullableExcerpt(m["description"], 1600)
			r["evidence_kind"] = "contributor_observation"
			r["track_included"] = false
			r["has_source_points"] = boolean(m["has_points"])
			r["official_closure_status"] = nil
			r["trail_open"] = nil
		}
	case "map":
		r["name"] = m["name"]
		r["version"] = m["version"]
		r["prefectures"] = prefs(m["prefectures"])
		r["bounds_lon_lat"] = m["bound"]
		r["center"] = loc(m["coord"])
		r["in_japan"] = boolean(m["in_japan"])
		r["deprecated"] = boolean(m["is_deprecated"])
		r["coverage_kind"] = "publisher_map_area"
		r["offline_navigation_included"] = false
		r["track_coverage_verified"] = false
		if detail {
			r["contained_area_names"] = m["contain_area"]
			r["publisher_cautions"] = cautions(m["mountain_cautions"])
			r["official_closure_status"] = nil
			r["trail_open"] = nil
			r["download_restrictions"] = "YAMAP account and plan rules apply; this CLI only links to source map pages"
		}
	}
	return r
}
func mapRef(m Row) any {
	if m == nil || ID(m["id"]) == "" {
		return nil
	}
	id := ID(m["id"])
	return Row{"id": id, "name": m["name"], "url": "https://yamap.com/maps/" + id}
}

func Extract(obj Row, key string) ([]Row, error) {
	v, ok := obj[key]
	if !ok {
		return nil, &SourceError{Kind: "schema", Message: "missing source field " + key}
	}
	a, ok := v.([]any)
	if !ok {
		return nil, &SourceError{Kind: "schema", Message: "expected array in " + key}
	}
	out := []Row{}
	for _, v := range a {
		m := object(v)
		if m == nil || ID(m["id"]) == "" {
			return nil, &SourceError{Kind: "schema", Message: "result missing source ID"}
		}
		out = append(out, m)
	}
	return out, nil
}
func Detail(obj Row, key, id string) (Row, error) {
	m := object(obj[key])
	if m == nil || ID(m["id"]) != id {
		return nil, &SourceError{Kind: "schema", Message: "detail ID missing or differs from request"}
	}
	return m, nil
}

func Recent(m Row, since, now time.Time) bool {
	n, ok := number(m["start_at"])
	if !ok || n <= 0 {
		return false
	}
	t := time.Unix(int64(n), 0)
	b, known := m["is_planned"].(bool)
	return known && !b && !t.Before(since) && !t.After(now)
}
func Since(s string, days int, now time.Time) (time.Time, error) {
	if s != "" {
		t, e := time.ParseInLocation("2006-01-02", s, time.FixedZone("Asia/Tokyo", 9*3600))
		if e != nil || t.After(now) {
			return time.Time{}, fmt.Errorf("--since must be a past or current YYYY-MM-DD Japan date")
		}
		return t, nil
	}
	if days < 1 || days > 366 {
		return time.Time{}, fmt.Errorf("--days must be 1–366")
	}
	return now.AddDate(0, 0, -days), nil
}

func Project(row Row, fields string) (Row, error) {
	if fields == "" {
		return row, nil
	}
	out := Row{}
	for _, path := range strings.Split(fields, ",") {
		path = strings.TrimSpace(path)
		if path == "" || !utf8.ValidString(path) {
			return nil, fmt.Errorf("--select requires non-empty comma-separated fields")
		}
		parts := strings.Split(path, ".")
		src := row
		dst := out
		for i, p := range parts {
			v, exists := src[p]
			if !exists {
				return nil, fmt.Errorf("unknown --select field %q", path)
			}
			if i == len(parts)-1 {
				dst[p] = v
				break
			}
			child := object(v)
			if child == nil {
				return nil, fmt.Errorf("--select field %q is not an object", p)
			}
			if dst[p] == nil {
				dst[p] = Row{}
			}
			next := object(dst[p])
			if next == nil {
				return nil, fmt.Errorf("overlapping --select fields")
			}
			dst = next
			src = child
		}
	}
	return out, nil
}

func Compare(route, report Row) (Row, error) {
	planned, known := report["is_planned"].(bool)
	if !known || planned {
		return nil, &SourceError{Kind: "usage", Message: "comparison requires a recorded trip with source is_planned:false; planned or unknown activities cannot be labeled recorded evidence"}
	}
	r := Summary("route", route, true)
	a := Summary("report", report, true)
	delta := Row{}
	rm := object(r["metrics"])
	am := object(a["metrics"])
	for _, k := range []string{"distance_m", "ascent_m", "descent_m"} {
		x, ok1 := number(rm[k])
		y, ok2 := number(am[k])
		if ok1 && ok2 {
			delta[k] = y - x
		} else {
			delta[k] = nil
		}
	}
	return Row{"planned_route": r, "recorded_trip": a, "recorded_minus_planned": delta, "route_equivalence_verified": false, "time_comparison": "standard route time and recorded elapsed/active time have different definitions; no pace or safety inference", "official_closure_status": nil}, nil
}
