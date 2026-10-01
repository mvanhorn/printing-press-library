package cli

import (
	"fmt"
	"strings"
)

// Domain schemas keep projections checkable even when every row is empty or a
// nullable object is absent. An empty list is evidence of absence, not a wildcard.
type evidenceShape struct {
	fields map[string]*evidenceShape
	item   *evidenceShape
}

func shape(names string) *evidenceShape {
	s := &evidenceShape{fields: map[string]*evidenceShape{}}
	for _, n := range strings.Fields(names) {
		s.fields[n] = nil
	}
	return s
}
func listShape(s *evidenceShape) *evidenceShape { return &evidenceShape{item: s} }
func nest(s *evidenceShape, name string, child *evidenceShape) *evidenceShape {
	s.fields[name] = child
	return s
}
func domainShape(path string) *evidenceShape {
	source := shape("source_url retrieved_at cache_hit cache_age_seconds")
	season := shape("product year updates_ended ended_message_ja timezone year_basis")
	normal := shape("kind period_ja reference_years valid_date")
	location := shape("id name_ja name_kana prefecture city_ja address_ja latitude longitude elevation_m elevation_status url")
	forecastLocation := shape("id name_ja latitude longitude elevation_m elevation_status grid_resolution_m requested_name requested_latitude requested_longitude url")
	hourly := shape("kind valid_at issued_at temperature_c precipitation_mm precipitation_interval_hours precipitation_interval_alignment wind_speed_m_s wind_direction_code humidity_percent weather_code")
	daily := shape("kind valid_date timezone issued_at max_temperature_c min_temperature_c precipitation_probability_percent precipitation_mm weather_code")
	horizon := shape("hourly_first hourly_last daily_first daily_last")
	report := shape("kind status_ja status_code valid_date source_date_ja issued_at date_precision timezone report_age_days current_season comment_ja")
	switch path {
	case "places resolve":
		s := shape("query total next_offset coverage")
		nest(s, "items", listShape(shape("id name_ja kind latitude longitude elevation_m elevation_status url")))
		return nest(s, "source", source)
	case "weather forecast":
		s := shape("status requested_date timezone issued_at issue_time_status coverage")
		nest(s, "location", forecastLocation)
		nest(s, "hourly", listShape(hourly))
		nest(s, "daily", listShape(daily))
		nest(s, "observation", shape("kind valid_at issued_at name_ja temperature_c precipitation_mm precipitation_interval_hours wind_speed_m_s humidity_percent weather_code coverage"))
		nest(s, "horizon", horizon)
		return nest(s, "source", source)
	case "weather compare":
		row := shape("name status error")
		nest(row, "location", forecastLocation)
		nest(row, "daily", daily)
		nest(row, "horizon", horizon)
		nest(row, "source", source)
		nest(row, "checks", listShape(shape("criterion operator threshold value passes")))
		return nest(shape("date timezone partial comparison_basis"), "items", listShape(row))
	case "season search":
		row := shape("id name_ja name_kana prefecture city_ja latitude longitude elevation_m elevation_status url")
		nest(row, "report", shape("kind status_ja status_code valid_date date_precision report_age_days"))
		nest(row, "normal", shape("kind period_ja reference_years"))
		s := shape("query area total inventory_total next_offset coverage query_mode provider_fetched_at")
		nest(s, "items", listShape(row))
		nest(s, "season", season)
		return nest(s, "source", source)
	case "season show":
		s := shape("status provider_fetched_at")
		nest(s, "location", location)
		nest(s, "season", season)
		nest(s, "report", report)
		nest(s, "normal", normal)
		nest(s, "predictions", listShape(shape("event kind valid_date source_date_ja issued_at issue_time_status status date_precision source_enabled")))
		nest(s, "travel", shape("tree_species_ja access_train_ja access_car_ja opening_hours_ja fee_ja"))
		nest(s, "coverage", nest(shape("access report_visibility elevation historical_norm_is_forecast"), "member_only_products", listShape(nil)))
		return nest(s, "source", source)
	case "season compare":
		row := shape("id status error")
		nest(row, "location", location)
		nest(row, "season", season)
		nest(row, "report", report)
		nest(row, "normal", normal)
		nest(row, "source", source)
		nest(row, "check", shape("reason criterion operator threshold_days value_days peak_prediction_date basis"))
		return nest(shape("date timezone partial comparison_basis"), "items", listShape(row))
	}
	return nil
}
func validShape(s *evidenceShape, parts []string) bool {
	if len(parts) == 0 {
		return true
	}
	if s == nil {
		return false
	}
	if s.item != nil {
		return validShape(s.item, parts)
	}
	child, ok := s.fields[parts[0]]
	return ok && validShape(child, parts[1:])
}
func projectEvidence(value map[string]any, fields, path string) (map[string]any, error) {
	if fields == "" {
		return value, nil
	}
	schema := domainShape(path)
	var paths [][]string
	for _, f := range strings.Split(fields, ",") {
		f = strings.TrimSpace(f)
		if f == "" {
			return nil, fmt.Errorf("--select requires nonempty dotted field paths")
		}
		parts := strings.Split(strings.ToLower(f), ".")
		if !validShape(schema, parts) {
			return nil, fmt.Errorf("--select unknown field %q for %s; inspect unprojected JSON or command help", f, path)
		}
		paths = append(paths, parts)
	}
	projected := projectValue(value, paths)
	out, ok := projected.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("projection failed")
	}
	return out, nil
}
func projectValue(v any, paths [][]string) any {
	for _, p := range paths {
		if len(p) == 0 {
			return v
		}
	}
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		groups := map[string][][]string{}
		for _, p := range paths {
			if len(p) > 0 {
				groups[p[0]] = append(groups[p[0]], p[1:])
			}
		}
		for key, ps := range groups {
			if value, ok := x[key]; ok {
				out[key] = projectValue(value, ps)
			} else {
				out[key] = nil
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, value := range x {
			out[i] = projectValue(value, paths)
		}
		return out
	default:
		return v
	}
}
