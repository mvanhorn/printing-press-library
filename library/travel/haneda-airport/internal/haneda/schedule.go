package haneda

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

type rawSchedule struct {
	Start    string `json:"運航開始日"`
	End      string `json:"運航終了日"`
	Airlines []struct {
		Code   string `json:"ＡＬコード"`
		Name   string `json:"ＡＬ英名称"`
		NameJA string `json:"ＡＬ和名称"`
		Number string `json:"便名"`
	} `json:"航空会社"`
	STD             string            `json:"ＳＴＤ"`
	STA             string            `json:"ＳＴＡ"`
	OriginCode      string            `json:"出発地空港コード"`
	OriginName      string            `json:"出発地空港英名称"`
	OriginJA        string            `json:"出発地空港和名称"`
	DestinationCode string            `json:"行先地空港コード"`
	DestinationName string            `json:"行先地空港英名称"`
	DestinationJA   string            `json:"行先地空港和名称"`
	Days            map[string]string `json:"運航曜日"`
}
type rawScheduleFeed struct {
	Updated string        `json:"last_upd"`
	Start   string        `json:"start"`
	End     string        `json:"end"`
	Flights []rawSchedule `json:"flight_info"`
}

var dayNames = []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
var dayKeys = []string{"日曜日", "月曜日", "火曜日", "水曜日", "木曜日", "金曜日", "土曜日"}

func normalizeSchedule(raw rawSchedule, kind, direction string, airports []Airport, airlines []Airline) (Schedule, error) {
	s := Schedule{Kind: kind, Direction: direction, Weekdays: []string{}, ListedFlights: []ListedFlight{}, HanedaTimezone: "Asia/Tokyo", SourceURL: Origin + "/en/flight/monthlyFlightSchedule.html"}
	start, err := parseDate(raw.Start)
	if err != nil {
		return s, err
	}
	end, err := parseDate(raw.End)
	if err != nil {
		return s, err
	}
	if end.Before(start) {
		return s, fmt.Errorf("schedule period end precedes start")
	}
	s.PeriodStart = start.Format("2006-01-02")
	s.PeriodEnd = end.Format("2006-01-02")
	for i, k := range dayKeys {
		if raw.Days[k] == "1" || raw.Days["毎日"] == "1" {
			s.Weekdays = append(s.Weekdays, dayNames[i])
		}
	}
	if len(s.Weekdays) == 0 {
		return s, fmt.Errorf("schedule has no interpretable operating weekdays")
	}
	for _, a := range raw.Airlines {
		meta := findAirline(a.Code, airlines)
		prefix := meta.Prefix
		if prefix == "" {
			prefix = strings.TrimSpace(a.Code)
		}
		n, err := NormalizeNumber(prefix + strings.TrimLeft(a.Number, "0"))
		if err != nil {
			return s, err
		}
		name, ja := meta.Name, meta.NameJA
		if name == "" {
			name = a.Name
		}
		if ja == "" {
			ja = a.NameJA
		}
		s.ListedFlights = append(s.ListedFlights, ListedFlight{Number: n, AirlineCode: a.Code, Name: name, NameJA: ja, URL: meta.URL})
	}
	if len(s.ListedFlights) == 0 {
		return s, fmt.Errorf("schedule has no listed airline")
	}
	code, name, ja := raw.DestinationCode, raw.DestinationName, raw.DestinationJA
	if direction == "departure" {
		s.HanedaTime = ptr(raw.STD)
		s.OtherAirportTime = ptr(raw.STA)
	} else {
		code, name, ja = raw.OriginCode, raw.OriginName, raw.OriginJA
		s.HanedaTime = ptr(raw.STA)
		s.OtherAirportTime = ptr(raw.STD)
	}
	if kind == "domestic" {
		z := "Asia/Tokyo"
		s.OtherAirportTimezone = &z
	}
	if code != "" {
		s.Airport = findAirport(code, airports)
		if s.Airport == nil {
			s.Airport = &Airport{Code: code, Name: name, NameJA: ja, Kind: kind}
		}
	}
	for _, clock := range []*string{s.HanedaTime, s.OtherAirportTime} {
		if clock != nil {
			if _, err := time.Parse("15:04", *clock); err != nil {
				return s, fmt.Errorf("invalid schedule time %q", *clock)
			}
		}
	}
	key := kind + "|" + direction + "|" + raw.Start + "|" + raw.End + "|" + s.ListedFlights[0].Number + "|" + raw.STD + "|" + raw.STA + "|" + code + "|" + strings.Join(s.Weekdays, ",")
	h := sha256.Sum256([]byte(key))
	s.ID = "hnd-schedule:" + hex.EncodeToString(h[:12])
	return s, nil
}

// FetchSchedules reads published periods/weekday rules, keeping them separate from status and inventory.
func (c *Client) FetchSchedules(ctx context.Context, q Query, now time.Time) (ScheduleResult, error) {
	r := ScheduleResult{ObservedAt: now.In(JST).Format(time.RFC3339), RequestedDate: q.Date, Sources: []SourceInfo{}, Schedules: []Schedule{}, Limit: q.Limit, Offset: q.Offset, Notes: []string{"Published schedules may change without notice and do not establish current flight status or bookable seats.", "scheduled_haneda_time is JST. Other-airport times remain raw published clocks; international timezone and date rollover are not inferred.", "Feed windows and per-row operating periods are preserved; no schedule is fabricated beyond published coverage."}}
	if err := ValidateQuery(q, now, false); err != nil {
		return r, err
	}
	if strings.HasPrefix(q.Flight, "hnd:") {
		return r, fmt.Errorf("schedule --flight accepts a listed flight number; use flights detail for a board hnd: ID")
	}
	matches := []Schedule{}
	for _, kind := range kinds(q.Kind) {
		airports, airlines, err := c.catalogs(ctx, kind)
		if err != nil {
			return r, err
		}
		for _, direction := range directions(q.Direction) {
			filename := "hdacfdsc"
			if direction == "arrival" {
				filename = "hdacfasc"
			}
			path := "/app_resource/flight/data/" + catalogCode(kind) + "/" + filename + ".json"
			var feed rawScheduleFeed
			if err = c.request(ctx, path, nil, &feed); err != nil {
				return r, err
			}
			if feed.Flights == nil || feed.Start == "" || feed.End == "" {
				return r, fmt.Errorf("monthly schedule has missing flight_info or coverage window")
			}
			r.Sources = append(r.Sources, SourceInfo{Kind: kind, Direction: direction, URL: Origin + path, UpdatedAt: parseTimestamp(feed.Updated), TimestampSemantics: "last_upd is the published feed timestamp", SourceTotal: len(feed.Flights), PeriodStart: feed.Start, PeriodEnd: feed.End})
			if q.Date != "" {
				start, err := parseDate(strings.Fields(feed.Start)[0])
				if err != nil {
					return r, err
				}
				end, err := parseDate(strings.Fields(feed.End)[0])
				if err != nil {
					return r, err
				}
				day, _ := parseDate(q.Date)
				if day.Before(start) || day.After(end) {
					return r, fmt.Errorf("--date is outside the published %s %s schedule window %s through %s", kind, direction, start.Format("2006-01-02"), end.Format("2006-01-02"))
				}
			}
			for _, raw := range feed.Flights {
				if r.ScannedRecords >= q.MaxScan {
					r.ScanCapHit = true
					continue
				}
				r.ScannedRecords++
				s, err := normalizeSchedule(raw, kind, direction, airports, airlines)
				if err != nil {
					return r, err
				}
				f := Flight{Kind: kind, Direction: direction, ListedFlights: s.ListedFlights, Airport: s.Airport}
				if s.Airport != nil {
					f.SourceAreaName = s.Airport.Name
				}
				filters := q
				filters.Status = ""
				filters.Terminal = ""
				filters.RolloverOnly = false
				filters.ServiceDayOnly = false
				if !matchesFlight(f, filters) {
					continue
				}
				if q.Date != "" {
					day, _ := parseDate(q.Date)
					active := false
					for _, weekday := range s.Weekdays {
						if weekday == dayNames[int(day.Weekday())] {
							active = true
						}
					}
					if q.Date < s.PeriodStart || q.Date > s.PeriodEnd || !active {
						continue
					}
				}
				matches = append(matches, s)
			}
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].PeriodStart == matches[j].PeriodStart {
			return matches[i].ID < matches[j].ID
		}
		return matches[i].PeriodStart < matches[j].PeriodStart
	})
	r.TotalMatches = len(matches)
	start := q.Offset
	if start > len(matches) {
		start = len(matches)
	}
	end := start + q.Limit
	if end > len(matches) {
		end = len(matches)
	}
	r.Schedules = append([]Schedule{}, matches[start:end]...)
	if end < len(matches) {
		v := end
		r.NextOffset = &v
	}
	r.Budget = c.Budget()
	if r.ScanCapHit {
		r.Notes = append(r.Notes, "The local scan cap was reached; raise --max-scan-records to examine remaining published rows.")
	}
	if len(r.Schedules) == 0 {
		r.Notes = append(r.Notes, "No matching published rows in the examined coverage; absence is not a seat or cancellation determination.")
	}
	return r, nil
}
