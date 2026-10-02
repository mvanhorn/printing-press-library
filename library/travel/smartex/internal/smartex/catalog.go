// Package smartex implements public, read-only Shinkansen planning.
package smartex

import (
	"fmt"
	"strings"
	"unicode"
)

const AsOf = "2026-10-02"
const BookingURL = "https://shinkansen2.jr-central.co.jp/RSV_P/smart_en_index.htm"
const TimetableURL = "https://global.jr-central.co.jp/en/info/timetable/"
const FareURL = "https://unchin-navi.jp/cgi-bin/plusex/tokai_exic.cgi"

type Source struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	Kind string `json:"kind"`
	AsOf string `json:"as_of,omitempty"`
}

var Sources = []Source{
	{"service", "https://smart-ex.jp/en/product/plan/service/", "official_policy", AsOf},
	{"window", "https://smart-ex.jp/en/reservation/useful/accept_time/", "official_policy", AsOf},
	{"advance-jp", "https://smart-ex.jp/reservation/useful/pre_time/", "official_policy", AsOf},
	{"fare", "https://unchin-navi.jp/", "dated_adult_fare_calculator", ""},
	{"timetable", TimetableURL, "published_basic_timetable", "2026-03-14"},
	{"baggage", "https://global.jr-central.co.jp/en/info/oversized-baggage/", "official_policy", AsOf},
	{"baggage-reservation", "https://smart-ex.jp/en/entraining/oversized-baggage/", "official_policy", AsOf},
	{"change", "https://smart-ex.jp/en/reservation/change/", "official_policy", AsOf},
	{"refund", "https://smart-ex.jp/en/reservation/guide/cancel/", "official_policy", AsOf},
	{"boarding", "https://smart-ex.jp/en/entraining/", "official_policy", AsOf},
	{"nozomi", "https://global.jr-central.co.jp/en/nozomi/", "official_policy", AsOf},
	{"window-jp", "https://smart-ex.jp/reservation/useful/accept_time/", "official_policy", AsOf},
	{"baggage-reservation-jp", "https://smart-ex.jp/reservation/reserve_smart/oversized-baggage/", "official_policy", AsOf},
}

type Station struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Japanese  string   `json:"japanese"`
	Index     int      `json:"corridor_order"`
	Corridors []string `json:"corridors"`
	Note      string   `json:"note,omitempty"`
}

// The order and names follow the official 2026-03-14 JR timetable station rows.
var stations = []Station{
	{ID: "tokyo", Name: "Tokyo", Japanese: "東京"},
	{ID: "shinagawa", Name: "Shinagawa", Japanese: "品川"},
	{ID: "shin-yokohama", Name: "Shin-Yokohama", Japanese: "新横浜"},
	{ID: "odawara", Name: "Odawara", Japanese: "小田原"},
	{ID: "atami", Name: "Atami", Japanese: "熱海"},
	{ID: "mishima", Name: "Mishima", Japanese: "三島"},
	{ID: "shin-fuji", Name: "Shin-Fuji", Japanese: "新富士"},
	{ID: "shizuoka", Name: "Shizuoka", Japanese: "静岡"},
	{ID: "kakegawa", Name: "Kakegawa", Japanese: "掛川"},
	{ID: "hamamatsu", Name: "Hamamatsu", Japanese: "浜松"},
	{ID: "toyohashi", Name: "Toyohashi", Japanese: "豊橋"},
	{ID: "mikawa-anjo", Name: "Mikawa-Anjo", Japanese: "三河安城"},
	{ID: "nagoya", Name: "Nagoya", Japanese: "名古屋"},
	{ID: "gifu-hashima", Name: "Gifu-Hashima", Japanese: "岐阜羽島"},
	{ID: "maibara", Name: "Maibara", Japanese: "米原"},
	{ID: "kyoto", Name: "Kyoto", Japanese: "京都"},
	{ID: "shin-osaka", Name: "Shin-Osaka", Japanese: "新大阪"},
	{ID: "shin-kobe", Name: "Shin-Kobe", Japanese: "新神戸"},
	{ID: "nishi-akashi", Name: "Nishi-Akashi", Japanese: "西明石"},
	{ID: "himeji", Name: "Himeji", Japanese: "姫路"},
	{ID: "aioi", Name: "Aioi", Japanese: "相生"},
	{ID: "okayama", Name: "Okayama", Japanese: "岡山"},
	{ID: "shin-kurashiki", Name: "Shin-Kurashiki", Japanese: "新倉敷"},
	{ID: "fukuyama", Name: "Fukuyama", Japanese: "福山"},
	{ID: "shin-onomichi", Name: "Shin-Onomichi", Japanese: "新尾道"},
	{ID: "mihara", Name: "Mihara", Japanese: "三原"},
	{ID: "higashi-hiroshima", Name: "Higashi-Hiroshima", Japanese: "東広島"},
	{ID: "hiroshima", Name: "Hiroshima", Japanese: "広島"},
	{ID: "shin-iwakuni", Name: "Shin-Iwakuni", Japanese: "新岩国"},
	{ID: "tokuyama", Name: "Tokuyama", Japanese: "徳山"},
	{ID: "shin-yamaguchi", Name: "Shin-Yamaguchi", Japanese: "新山口"},
	{ID: "asa", Name: "Asa", Japanese: "厚狭"},
	{ID: "shin-shimonoseki", Name: "Shin-Shimonoseki", Japanese: "新下関"},
	{ID: "kokura", Name: "Kokura", Japanese: "小倉"},
	{ID: "hakata", Name: "Hakata (Fukuoka)", Japanese: "博多"},
	{ID: "shin-tosu", Name: "Shin-Tosu", Japanese: "新鳥栖"},
	{ID: "kurume", Name: "Kurume", Japanese: "久留米"},
	{ID: "chikugo-funagoya", Name: "Chikugo-Funagoya", Japanese: "筑後船小屋"},
	{ID: "shin-omuta", Name: "Shin-Omuta", Japanese: "新大牟田"},
	{ID: "shin-tamana", Name: "Shin-Tamana", Japanese: "新玉名"},
	{ID: "kumamoto", Name: "Kumamoto", Japanese: "熊本"},
	{ID: "shin-yatsushiro", Name: "Shin-Yatsushiro", Japanese: "新八代"},
	{ID: "shin-minamata", Name: "Shin-Minamata", Japanese: "新水俣"},
	{ID: "izumi", Name: "Izumi", Japanese: "出水"},
	{ID: "sendai", Name: "Sendai (Kagoshima)", Japanese: "川内", Note: "Kyushu station 川内; not Sendai 仙台 on the Tohoku Shinkansen"},
	{ID: "kagoshima-chuo", Name: "Kagoshima-Chuo", Japanese: "鹿児島中央"},
}

func init() {
	for i := range stations {
		stations[i].Index = i
		if i <= 16 {
			stations[i].Corridors = append(stations[i].Corridors, "tokaido")
		}
		if i >= 16 && i <= 34 {
			stations[i].Corridors = append(stations[i].Corridors, "sanyo")
		}
		if i >= 34 {
			stations[i].Corridors = append(stations[i].Corridors, "kyushu")
		}
	}
}

func normalize(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || strings.ContainsRune("-_().", r) {
			return -1
		}
		return unicode.ToLower(r)
	}, strings.TrimSpace(s))
}

func Resolve(name string) (Station, error) {
	n := normalize(name)
	if n == "" {
		return Station{}, fmt.Errorf("station is required; use stations --query to find a station ID")
	}
	for _, s := range stations {
		if n == normalize(s.ID) || n == normalize(s.Name) || n == normalize(s.Japanese) {
			return s, nil
		}
	}
	if n == "fukuoka" {
		return Station{}, fmt.Errorf("use station ID hakata (博多) for Fukuoka's Shinkansen station")
	}
	if n == "osaka" || n == "大阪" {
		return Station{}, fmt.Errorf("Osaka and Shin-Osaka are distinct stations; use shin-osaka (新大阪) for this Shinkansen CLI")
	}
	return Station{}, fmt.Errorf("unsupported station %q; scope is Tokyo–Kagoshima-Chuo; use stations --query %q", name, name)
}

func Stations(query string, limit int) ([]Station, int) {
	out := []Station{}
	for _, s := range stations {
		if query == "" || strings.Contains(normalize(s.ID+" "+s.Name+" "+s.Japanese), normalize(query)) {
			out = append(out, s)
		}
	}
	total := len(out)
	if limit < len(out) {
		out = out[:limit]
	}
	return out, total
}

type Route struct {
	From                  Station   `json:"from"`
	To                    Station   `json:"to"`
	Direction             string    `json:"direction"`
	Corridors             []string  `json:"corridors"`
	Stations              []Station `json:"stations,omitempty"`
	TrainCategories       []string  `json:"train_categories"`
	ThroughTrainConfirmed bool      `json:"through_train_confirmed"`
	Inventory             any       `json:"inventory"`
	Notes                 []string  `json:"notes"`
	Source                Source    `json:"source"`
}

func PlanRoute(from, to string, detail bool) (Route, error) {
	f, err := Resolve(from)
	if err != nil {
		return Route{}, err
	}
	t, err := Resolve(to)
	if err != nil {
		return Route{}, err
	}
	if f.ID == t.ID {
		return Route{}, fmt.Errorf("--from and --to must be different stations")
	}
	r := Route{From: f, To: t, Direction: "westbound", Corridors: []string{}, TrainCategories: []string{}, Source: Sources[4], Notes: []string{"Train categories describe corridor coverage; stops, through services and seats require timetable/booking confirmation", "smartEX covers Shinkansen stations only; conventional-line fares are separate"}}
	lo, hi := f.Index, t.Index
	if lo > hi {
		lo, hi = hi, lo
		r.Direction = "eastbound"
	}
	if lo < 16 {
		r.Corridors = append(r.Corridors, "tokaido")
		r.TrainCategories = append(r.TrainCategories, "nozomi", "hikari", "kodama")
	}
	if lo < 34 && hi > 16 {
		r.Corridors = append(r.Corridors, "sanyo")
		if lo >= 16 {
			r.TrainCategories = append(r.TrainCategories, "nozomi", "hikari", "kodama")
		}
		r.TrainCategories = append(r.TrainCategories, "mizuho", "sakura")
	}
	if hi > 34 {
		r.Corridors = append(r.Corridors, "kyushu")
		if hi <= 34 || lo >= 34 {
			r.TrainCategories = nil
		}
		r.TrainCategories = appendUnique(r.TrainCategories, "mizuho", "sakura", "tsubame")
	}
	if detail {
		step := 1
		if f.Index > t.Index {
			step = -1
		}
		for i := f.Index; ; i += step {
			r.Stations = append(r.Stations, stations[i])
			if i == t.Index {
				break
			}
		}
	}
	return r, nil
}

func appendUnique(a []string, v ...string) []string {
	for _, s := range v {
		found := false
		for _, x := range a {
			if x == s {
				found = true
			}
		}
		if !found {
			a = append(a, s)
		}
	}
	return a
}
