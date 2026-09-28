package navitime

import (
	"net/url"
	"regexp"
	"strings"
	"time"
)

var japan = time.FixedZone("Asia/Tokyo", 9*60*60)
var stationPattern = regexp.MustCompile(`^\d{8}$`)
var spotPattern = regexp.MustCompile(`^\d{5}-[A-Za-z0-9][A-Za-z0-9-]*$`)
var passPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func parseRef(raw string) (kind, id string, err error) {
	if stationPattern.MatchString(raw) {
		return "station", raw, nil
	}
	kind, id, ok := strings.Cut(raw, ":")
	if !ok || (kind != "station" && kind != "spot") || (kind == "station" && !stationPattern.MatchString(id)) || (kind == "spot" && !spotPattern.MatchString(id)) {
		return "", "", &ArgumentError{"use an explicit station:ID or spot:ID reference returned by places search"}
	}
	return kind, id, nil
}

func ValidateQuery(q Query) error { _, _, err := queryParams(q); return err }

func queryParams(q Query) (url.Values, string, error) {
	fk, fi, err := parseRef(q.From)
	if err != nil {
		return nil, "", err
	}
	tk, ti, err := parseRef(q.To)
	if err != nil {
		return nil, "", err
	}
	modes := []struct {
		value, mode string
		dateOnly    bool
	}{{q.DepartAt, "start-time", false}, {q.ArriveBy, "goal-time", false}, {q.FirstOn, "first-operation", true}, {q.LastOn, "last-operation", true}}
	count := 0
	mode := ""
	date := ""
	for _, m := range modes {
		if m.value == "" {
			continue
		}
		count++
		mode = m.mode
		var t time.Time
		if m.dateOnly {
			t, err = time.ParseInLocation("2006-01-02", m.value, japan)
		} else {
			t, err = time.ParseInLocation("2006-01-02T15:04", m.value, japan)
			if err != nil {
				t, err = time.Parse(time.RFC3339, m.value)
			}
		}
		if err != nil {
			return nil, "", &ArgumentError{"use YYYY-MM-DD for first/last or an explicit ISO date and time for depart/arrive"}
		}
		if !m.dateOnly && (t.Second() != 0 || t.Nanosecond() != 0) {
			return nil, "", &ArgumentError{"timetable query precision is whole minutes"}
		}
		date = t.In(japan).Format("2006-01-02T15:04")
	}
	if count != 1 {
		return nil, "", &ArgumentError{"exactly one of depart-at, arrive-by, first-on, or last-on is required"}
	}
	if q.Pass != "" && !passPattern.MatchString(q.Pass) {
		return nil, "", &ArgumentError{"supply one source-advertised pass ID; multiple passes require source membership"}
	}
	p := url.Values{"date_time": {date}, "search_time_mode": {mode}}
	if fk == "spot" {
		p.Set("startCode", fi)
	} else {
		p.Set("start", fi)
	}
	if tk == "spot" {
		p.Set("goalCode", ti)
	} else {
		p.Set("goal", ti)
	}
	if q.Pass != "" {
		p.Set("pass_list", q.Pass)
	}
	return p, mode, nil
}
