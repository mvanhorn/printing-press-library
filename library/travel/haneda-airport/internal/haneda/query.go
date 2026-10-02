package haneda

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

var flightNumberPattern = regexp.MustCompile(`^[A-Z0-9]{2,3}[0-9]{1,5}$`)
var flightLetterPattern = regexp.MustCompile(`[A-Z]`)
var numberPartsPattern = regexp.MustCompile(`^([A-Z0-9]{2,3}?)([0-9]+)$`)

// NormalizeNumber accepts conventional spaces/hyphens and preserves significant source digits.
func NormalizeNumber(s string) (string, error) {
	n := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(s), " ", ""), "-", ""))
	if !flightNumberPattern.MatchString(n) || !flightLetterPattern.MatchString(n) {
		return "", fmt.Errorf("flight must be a full flight number such as NH849, 6J21, or JL91")
	}
	return n, nil
}

// ParseIdentity reverses the stable source-primary identity without interpreting it as an operator.
func ParseIdentity(s string) (kind, direction, date, number string, err error) {
	p := strings.Split(s, ":")
	if len(p) != 5 || p[0] != "hnd" {
		err = fmt.Errorf("flight ID must have the hnd:kind:direction:YYYYMMDD:flight form")
		return
	}
	kind, direction = p[1], p[2]
	if (kind != "domestic" && kind != "international") || (direction != "departure" && direction != "arrival") {
		err = fmt.Errorf("flight ID has an invalid kind/direction")
		return
	}
	d, e := time.ParseInLocation("20060102", p[3], JST)
	if e != nil {
		err = fmt.Errorf("flight ID has an invalid service date")
		return
	}
	date = d.Format("2006-01-02")
	number, err = NormalizeNumber(p[4])
	return
}

// ValidateQuery separates strict syntax/output budgets from optional live source date coverage.
func ValidateQuery(q Query, now time.Time, live bool) error {
	if q.Kind != "domestic" && q.Kind != "international" && q.Kind != "all" {
		return fmt.Errorf("--kind must be domestic, international, or all")
	}
	if q.Direction != "departure" && q.Direction != "arrival" && q.Direction != "both" {
		return fmt.Errorf("--direction must be departure, arrival, or both")
	}
	if q.Limit < 1 || q.Limit > 200 {
		return fmt.Errorf("--limit must be between 1 and 200")
	}
	if q.Offset < 0 || q.Offset > 20000 {
		return fmt.Errorf("--offset must be between 0 and 20000")
	}
	if q.MaxScan < 1 || q.MaxScan > 20000 {
		return fmt.Errorf("--max-scan-records must be between 1 and 20000")
	}
	if len(q.Flight) > 120 || len(q.Airline) > 120 || len(q.Destination) > 120 || len(q.Status) > 120 || len(q.Terminal) > 20 {
		return fmt.Errorf("filter text exceeds its bounded length")
	}
	for _, v := range []string{q.Airline, q.Destination, q.Status} {
		if v != "" && fold(v) == "" {
			return fmt.Errorf("filters must contain an airport, airline, or status name/code")
		}
	}
	if q.Status != "" {
		for _, token := range strings.Split(q.Status, ",") {
			if fold(token) == "" {
				return fmt.Errorf("--status must not contain empty comma-separated values or punctuation-only tokens")
			}
		}
	}
	if q.Terminal != "" && q.Terminal != "T1" && q.Terminal != "T2" && q.Terminal != "T3" {
		return fmt.Errorf("--terminal must be T1, T2, or T3")
	}
	if q.Flight != "" {
		if strings.HasPrefix(q.Flight, "hnd:") {
			if _, _, _, _, err := ParseIdentity(q.Flight); err != nil {
				return err
			}
		} else if _, err := NormalizeNumber(q.Flight); err != nil {
			return err
		}
	}
	if q.Date == "" {
		return nil
	}
	d, err := time.ParseInLocation("2006-01-02", q.Date, JST)
	if err != nil {
		return fmt.Errorf("--date must be an exact valid YYYY-MM-DD date")
	}
	if live {
		today, _ := time.ParseInLocation("2006-01-02", now.In(JST).Format("2006-01-02"), JST)
		minimum := today
		if q.Kind == "international" {
			minimum = today.AddDate(0, 0, -1)
		}
		if d.Before(minimum) || d.After(today.AddDate(0, 3, 0)) {
			return fmt.Errorf("--date is outside source coverage: domestic/all start today JST, international starts yesterday JST, through three calendar months")
		}
	}
	return nil
}

func matchesFlight(f Flight, q Query) bool {
	if q.Kind != "all" && f.Kind != q.Kind {
		return false
	}
	if q.Direction != "both" && f.Direction != q.Direction {
		return false
	}
	if q.ServiceDayOnly && f.ServiceDate != q.Date {
		return false
	}
	if q.Terminal != "" && (f.Terminal == nil || *f.Terminal != q.Terminal) {
		return false
	}
	if q.Flight != "" {
		matched := strings.EqualFold(f.ID, q.Flight)
		n, _ := NormalizeNumber(q.Flight)
		for _, a := range f.ListedFlights {
			if equalNumber(a.Number, n) {
				matched = true
			}
		}
		if !matched {
			return false
		}
	}
	if q.Airline != "" {
		matched := false
		for _, a := range f.ListedFlights {
			if strings.EqualFold(a.AirlineCode, q.Airline) || strings.HasPrefix(a.Number, strings.ToUpper(q.Airline)) || contains(a.Name, q.Airline) || contains(a.NameJA, q.Airline) {
				matched = true
			}
		}
		if !matched {
			return false
		}
	}
	if q.Destination != "" {
		matched := contains(f.SourceAreaName, q.Destination)
		if a := f.Airport; a != nil {
			matched = matched || strings.EqualFold(a.Code, q.Destination) || strings.EqualFold(a.SearchValue, q.Destination) || contains(a.Name, q.Destination) || contains(a.NameJA, q.Destination)
		}
		if !matched {
			return false
		}
	}
	if q.Status != "" && !strings.EqualFold(strings.TrimSpace(q.Status), "all") {
		matched := false
		for _, s := range strings.Split(q.Status, ",") {
			s = strings.ToLower(strings.TrimSpace(s))
			if fold(s) == "" {
				continue
			}
			if s == "all" {
				matched = true
				break
			}
			if s == "cancelled" {
				s = "canceled"
			}
			if (s == "unknown" && !f.Status.Known) || fold(s) == fold(f.Status.Category) || fold(s) == fold(f.Status.Text) {
				matched = true
			}
		}
		if !matched {
			return false
		}
	}
	if q.RolloverOnly {
		crossed := f.ServiceDate != q.Date
		if f.ScheduledAt != nil {
			crossed = crossed || (*f.ScheduledAt)[:10] != f.ServiceDate
		}
		if f.ScheduledAt != nil && f.RevisedAt != nil {
			crossed = crossed || (*f.ScheduledAt)[:10] != (*f.RevisedAt)[:10]
		}
		if !crossed {
			return false
		}
	}
	return true
}
func contains(hay, needle string) bool {
	return needle != "" && strings.Contains(fold(hay), fold(needle))
}
func equalNumber(a, b string) bool {
	if a == b {
		return true
	}
	pa, pb := numberPartsPattern.FindStringSubmatch(a), numberPartsPattern.FindStringSubmatch(b)
	return len(pa) == 3 && len(pb) == 3 && pa[1] == pb[1] && strings.TrimLeft(pa[2], "0") == strings.TrimLeft(pb[2], "0")
}

// FilterBoard preserves source order within each codeshare group while paging matching services.
func FilterBoard(r BoardResult, q Query, now time.Time) BoardResult {
	matches := []Flight{}
	for _, f := range r.Flights {
		if matchesFlight(f, q) {
			if !q.IncludeFacilities {
				f.Facilities = nil
			}
			matches = append(matches, f)
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].ScheduledAt == nil {
			return false
		}
		if matches[j].ScheduledAt == nil {
			return true
		}
		if *matches[i].ScheduledAt == *matches[j].ScheduledAt {
			return matches[i].ID < matches[j].ID
		}
		return *matches[i].ScheduledAt < *matches[j].ScheduledAt
	})
	r.TotalMatches = len(matches)
	r.Limit = q.Limit
	r.Offset = q.Offset
	r.NextOffset = nil
	start := q.Offset
	if start > len(matches) {
		start = len(matches)
	}
	end := start + q.Limit
	if end > len(matches) {
		end = len(matches)
	}
	r.Flights = append([]Flight{}, matches[start:end]...)
	if end < len(matches) {
		n := end
		r.NextOffset = &n
	}
	if observed, err := time.Parse(time.RFC3339, r.ObservedAt); err == nil {
		r.SnapshotAgeSeconds = int64(now.Sub(observed).Seconds())
		if r.SnapshotAgeSeconds < 0 {
			r.SnapshotAgeSeconds = 0
		}
		r.Stale = r.SnapshotAgeSeconds > 300
	}
	if r.ScanCapHit {
		r.Notes = append(append([]string{}, r.Notes...), "The local scan cap was reached; totals describe examined records only. Raise --max-scan-records to widen the scan.")
	}
	if len(r.Flights) == 0 {
		r.Notes = append(append([]string{}, r.Notes...), "No matching service groups in the examined source scope; an empty board does not prove the absence of delays or flights outside that coverage.")
	}
	return r
}
