package smartex

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

const WestPDF = "https://global.jr-central.co.jp/en/info/timetable/_pdf/shinkansen_west_bound2603.pdf"
const EastPDF = "https://global.jr-central.co.jp/en/info/timetable/_pdf/shinkansen_east_bound2603.pdf"

type Stop struct {
	StationID string `json:"station_id"`
	Departure string `json:"departure_jst,omitempty"`
	Arrival   string `json:"arrival_jst,omitempty"`
}
type BasicService struct {
	Train      string `json:"train"`
	Number     int    `json:"number"`
	Stops      []Stop `json:"stops,omitempty"`
	SourceURL  string `json:"source_url"`
	SourcePage int    `json:"source_pdf_page"`
}

func depart(id, t string) Stop             { return Stop{StationID: id, Departure: t} }
func terminal(id, t string) Stop           { return Stop{StationID: id, Arrival: t} }
func interchange(id, arr, dep string) Stop { return Stop{StationID: id, Arrival: arr, Departure: dep} }

// Curated, visually verified regular examples from the public basic timetable.
// Departure-only intermediate rows are never relabelled as arrival times.
var basicServices = []BasicService{
	{"nozomi", 1, []Stop{depart("tokyo", "06:00"), depart("shinagawa", "06:07"), depart("shin-yokohama", "06:18"), interchange("nagoya", "07:34", "07:35"), depart("kyoto", "08:09"), interchange("shin-osaka", "08:22", "08:24"), depart("shin-kobe", "08:37"), depart("okayama", "09:10"), depart("fukuyama", "09:26"), depart("hiroshima", "09:50"), depart("kokura", "10:36"), terminal("hakata", "10:52")}, WestPDF, 2},
	{"nozomi", 101, []Stop{depart("nagoya", "06:20"), depart("kyoto", "06:55"), interchange("shin-osaka", "07:09", "07:11"), depart("shin-kobe", "07:24"), depart("okayama", "07:56"), depart("fukuyama", "08:13"), depart("hiroshima", "08:37"), depart("kokura", "09:24"), terminal("hakata", "09:39")}, WestPDF, 1},
	{"hikari", 631, []Stop{depart("tokyo", "06:21"), depart("shinagawa", "06:28"), depart("shin-yokohama", "06:39"), depart("odawara", "06:56"), depart("hamamatsu", "07:49"), interchange("nagoya", "08:08", "08:13"), depart("gifu-hashima", "08:30"), depart("maibara", "08:48"), depart("kyoto", "09:08"), terminal("shin-osaka", "09:21")}, WestPDF, 2},
	{"hikari", 681, []Stop{depart("shin-osaka", "06:06"), depart("shin-kobe", "06:19"), depart("nishi-akashi", "06:27"), depart("himeji", "06:38"), depart("aioi", "06:46"), depart("okayama", "07:02"), depart("shin-kurashiki", "07:12"), depart("fukuyama", "07:23"), depart("mihara", "07:33"), depart("higashi-hiroshima", "07:44"), depart("hiroshima", "08:00"), depart("shin-iwakuni", "08:14"), depart("tokuyama", "08:26"), depart("shin-yamaguchi", "08:39"), depart("shin-shimonoseki", "08:54"), depart("kokura", "09:03"), terminal("hakata", "09:19")}, WestPDF, 1},
	{"kodama", 801, []Stop{depart("tokyo", "06:30"), depart("shinagawa", "06:37"), depart("shin-yokohama", "06:48"), depart("odawara", "07:05"), depart("atami", "07:14"), depart("mishima", "07:26"), depart("shin-fuji", "07:39"), depart("shizuoka", "07:52"), depart("kakegawa", "08:08"), depart("hamamatsu", "08:23"), depart("toyohashi", "08:39"), depart("mikawa-anjo", "08:56"), terminal("nagoya", "09:06")}, WestPDF, 2},
	{"kodama", 931, []Stop{depart("okayama", "06:10"), depart("shin-kurashiki", "06:21"), depart("fukuyama", "06:33"), depart("shin-onomichi", "06:41"), depart("mihara", "06:48"), depart("higashi-hiroshima", "06:59"), depart("hiroshima", "07:11"), depart("shin-iwakuni", "07:26"), depart("tokuyama", "07:39"), depart("shin-yamaguchi", "07:57"), depart("asa", "08:07"), depart("shin-shimonoseki", "08:17"), depart("kokura", "08:26"), terminal("hakata", "08:42")}, WestPDF, 1},
	{"mizuho", 601, []Stop{depart("shin-osaka", "06:00"), depart("shin-kobe", "06:13"), depart("himeji", "06:29"), depart("okayama", "06:51"), depart("hiroshima", "07:26"), depart("kokura", "08:13"), interchange("hakata", "08:28", "08:30"), depart("kumamoto", "09:03"), terminal("kagoshima-chuo", "09:46")}, WestPDF, 1},
	{"sakura", 401, []Stop{depart("hiroshima", "06:43"), depart("tokuyama", "07:05"), depart("shin-yamaguchi", "07:18"), depart("shin-shimonoseki", "07:32"), depart("kokura", "07:41"), interchange("hakata", "07:56", "07:58"), depart("shin-tosu", "08:12"), depart("kurume", "08:16"), depart("kumamoto", "08:37"), depart("shin-yatsushiro", "08:49"), depart("shin-minamata", "09:03"), depart("izumi", "09:10"), depart("sendai", "09:22"), terminal("kagoshima-chuo", "09:34")}, WestPDF, 1},
	{"tsubame", 303, []Stop{depart("kumamoto", "06:11"), depart("shin-yatsushiro", "06:23"), depart("shin-minamata", "06:37"), depart("izumi", "06:44"), depart("sendai", "06:56"), terminal("kagoshima-chuo", "07:08")}, WestPDF, 1},
	{"tsubame", 307, []Stop{depart("hakata", "06:10"), depart("shin-tosu", "06:24"), depart("kurume", "06:29"), depart("chikugo-funagoya", "06:36"), depart("shin-omuta", "06:43"), depart("shin-tamana", "06:51"), depart("kumamoto", "07:01"), depart("shin-yatsushiro", "07:13"), depart("shin-minamata", "07:27"), depart("izumi", "07:34"), depart("sendai", "07:46"), terminal("kagoshima-chuo", "07:58")}, WestPDF, 1},
	{"nozomi", 2, []Stop{depart("hakata", "06:00"), depart("kokura", "06:18"), depart("shin-yamaguchi", "06:36"), depart("hiroshima", "07:08"), depart("okayama", "07:44"), depart("shin-kobe", "08:16"), interchange("shin-osaka", "08:28", "08:30"), depart("kyoto", "08:45"), interchange("nagoya", "09:19", "09:20"), terminal("shin-yokohama", "10:37"), depart("shinagawa", "10:49"), terminal("tokyo", "10:57")}, EastPDF, 2},
	{"nozomi", 230, []Stop{depart("shin-osaka", "06:00"), depart("kyoto", "06:14"), interchange("nagoya", "06:48", "06:49"), terminal("shin-yokohama", "08:05"), depart("shinagawa", "08:16"), terminal("tokyo", "08:23")}, EastPDF, 1},
	{"hikari", 634, []Stop{depart("shin-osaka", "06:15"), depart("kyoto", "06:29"), interchange("nagoya", "07:02", "07:03"), depart("shizuoka", "08:12"), terminal("shin-yokohama", "09:20"), depart("shinagawa", "09:31"), terminal("tokyo", "09:39")}, EastPDF, 1},
	{"kodama", 800, []Stop{depart("nagoya", "06:42"), depart("mikawa-anjo", "07:00"), depart("toyohashi", "07:15"), depart("hamamatsu", "07:30"), depart("kakegawa", "07:45"), depart("shizuoka", "08:06"), depart("shin-fuji", "08:22"), depart("mishima", "08:36"), depart("atami", "08:44"), depart("odawara", "08:56"), terminal("shin-yokohama", "09:11"), depart("shinagawa", "09:22"), terminal("tokyo", "09:30")}, EastPDF, 1},
	{"sakura", 740, []Stop{depart("kumamoto", "06:08"), depart("shin-tamana", "06:18"), depart("shin-omuta", "06:26"), depart("chikugo-funagoya", "06:32"), depart("kurume", "06:39"), depart("shin-tosu", "06:44"), interchange("hakata", "06:57", "06:59"), depart("kokura", "07:16"), depart("shin-shimonoseki", "07:24"), depart("shin-yamaguchi", "07:38"), depart("hiroshima", "08:10"), depart("fukuyama", "08:34"), depart("okayama", "08:51"), depart("shin-kobe", "09:24"), terminal("shin-osaka", "09:37")}, EastPDF, 2},
}

type TimetableMatch struct {
	BasicService
	From                   string `json:"from"`
	To                     string `json:"to"`
	Departure              string `json:"departure_jst"`
	Arrival                any    `json:"arrival_jst"`
	DestinationDeparture   any    `json:"destination_departure_jst"`
	DurationMinutes        any    `json:"duration_minutes"`
	ServiceOnRequestedDate any    `json:"operates_on_requested_date"`
	Inventory              any    `json:"inventory"`
}

func BasicMatches(from, to, train, after string, limit int, detail bool) ([]TimetableMatch, error) {
	f, e := Resolve(from)
	if e != nil {
		return nil, e
	}
	t, e := Resolve(to)
	if e != nil {
		return nil, e
	}
	if f.ID == t.ID {
		return nil, fmt.Errorf("--from and --to must differ")
	}
	if train != "" && !ValidTrain(train) {
		return nil, fmt.Errorf("unsupported --train category")
	}
	if after != "" {
		if _, e = time.Parse("15:04", after); e != nil {
			return nil, fmt.Errorf("--after must be HH:MM JST")
		}
	}
	result := []TimetableMatch{}
	for _, s := range basicServices {
		if train != "" && s.Train != train {
			continue
		}
		i, j := -1, -1
		for k, v := range s.Stops {
			if v.StationID == f.ID {
				i = k
			}
			if v.StationID == t.ID {
				j = k
			}
		}
		if i < 0 || j <= i || s.Stops[i].Departure == "" || s.Stops[i].Departure < after {
			continue
		}
		dep, dest := s.Stops[i].Departure, s.Stops[j]
		m := TimetableMatch{BasicService: s, From: f.ID, To: t.ID, Departure: dep}
		if dest.Arrival != "" {
			m.Arrival = dest.Arrival
			a, _ := time.Parse("15:04", dest.Arrival)
			d, _ := time.Parse("15:04", dep)
			m.DurationMinutes = int(a.Sub(d).Minutes())
		} else {
			m.DestinationDeparture = dest.Departure
		}
		if !detail {
			m.Stops = nil
		}
		result = append(result, m)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Departure < result[j].Departure })
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (c *Client) Timetable(ctx context.Context, from, to, date, train, after string, limit int, detail, offline bool) (map[string]any, error) {
	if date != "" {
		if _, e := ParseDate(date); e != nil {
			return nil, e
		}
	}
	matches, e := BasicMatches(from, to, train, after, limit, detail)
	if e != nil {
		return nil, e
	}
	var check any
	var compatible any
	publications := []string{}
	if !offline {
		got := c.Source(ctx, Sources[4], true)
		check = got
		if got.Error != "" {
			return nil, fmt.Errorf("timetable source check failed: %w; use --offline for the explicitly dated snapshot", got.Err)
		}
		west, east := false, false
		for _, l := range got.Links {
			publications = appendUnique(publications, strings.Split(l.URL, "#")[0])
			west = west || strings.Split(l.URL, "#")[0] == WestPDF
			east = east || strings.Split(l.URL, "#")[0] == EastPDF
		}
		compatible = west && east
		if compatible == false {
			matches = []TimetableMatch{}
		}
	}
	notes := []string{"Limited curated regular service examples, visually verified against2026-03-14 basic JR timetable; not every train or stop is represented", "Schedules, extras and operating calendars depend on date; requested-date operation and inventory are unknown", "Intermediate timetable rows may show departure only; arrival and duration stay null in that case", "Nozomi is all-reserved in major peak periods; confirm current date/class in official booking"}
	if compatible == false {
		notes = append(notes, "Official PDF links changed; old example rows suppressed; use the current publication links")
	}
	if len(matches) == 0 {
		notes = append(notes, "No match in the curated examples does not mean no trains; consult the complete official PDFs or booking site")
	}
	return map[string]any{"from": from, "to": to, "requested_date_jst": date, "timezone": "Asia/Tokyo", "effective_from": "2026-03-14", "coverage": "curated_basic_examples", "complete": false, "snapshot_matches_current_pdf_links": compatible, "offline": offline, "source_check": check, "publications": publications, "snapshot_publications": []string{WestPDF, EastPDF}, "services": matches, "notes": notes, "upstream_requests": c.requests, "booking_url": BookingURL}, nil
}
