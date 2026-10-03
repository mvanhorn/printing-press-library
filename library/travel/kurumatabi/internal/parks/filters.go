package parks

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

//go:embed filters.json
var filterJSON []byte

type Filter struct {
	Value string `json:"value"`
	Label string `json:"label"`
}
type FilterCatalog struct {
	SourceURL  string              `json:"source_url"`
	ObservedAt string              `json:"observed_at"`
	Groups     map[string][]Filter `json:"groups"`
}

func Filters() FilterCatalog { var f FilterCatalog; _ = json.Unmarshal(filterJSON, &f); return f }

var aliases = map[string]map[string]string{
	"types":      {"rvpark": "2", "yypark": "3", "camp3000": "4", "3000camp": "4", "campjrva": "5", "jrvacamp": "5", "gourmet": "7", "minpark": "8", "train": "9", "kurumatabipark": "10"},
	"vehicles":   {"full": "1", "semi-full": "2", "bus": "3", "cab": "4", "van": "5", "trailer": "6", "kei": "7", "car": "8", "other": "10"},
	"facilities": {"garbage": "1", "electricity": "2", "water": "3", "generator": "4", "bath": "5", "pets": "6", "premium_benefits": "7", "toilet_24h": "8", "dump_station": "9", "dog_run": "10"},
}

func resolve(group, v string) (string, error) {
	if v == "" {
		return "", nil
	}
	candidate := v
	if group != "prefectures" && group != "periods" {
		if a := aliases[group][v]; a != "" {
			candidate = a
		}
	}
	for _, f := range Filters().Groups[group] {
		if candidate == f.Value || v == f.Label {
			return f.Value, nil
		}
	}
	return "", fmt.Errorf("unsupported %s value %q; run parks filters", group, v)
}
func QueryValues(q Query) (url.Values, error) {
	if q.MaxPages < 1 || q.MaxPages > 5 {
		return nil, fmt.Errorf("--max-scan-pages must be 1..5")
	}
	if q.Limit < 1 || q.Limit > 100 {
		return nil, fmt.Errorf("--limit must be 1..100")
	}
	v := url.Values{"from_side_search": {"on"}}
	for group, wire := range map[string]string{"prefectures": "area_pref", "types": "category[]", "vehicles": "vehicle_size_search[]", "periods": "availability_period[]"} {
		value := map[string]string{"prefectures": q.Prefecture, "types": q.Type, "vehicles": q.Vehicle, "periods": q.Period}[group]
		if value == "" {
			continue
		}
		value, e := resolve(group, value)
		if e != nil {
			return nil, e
		}
		v.Add(wire, value)
	}
	for _, f := range q.Facilities {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		value, e := resolve("facilities", f)
		if e != nil {
			return nil, e
		}
		v.Add("okonomi_search[]", value)
	}
	if len([]rune(q.Keyword)) > 100 {
		return nil, fmt.Errorf("--keyword must be at most 100 characters")
	}
	v.Set("src_word", q.Keyword)
	return v, nil
}
func VehicleLabel(v string) (string, error) {
	code, e := resolve("vehicles", v)
	if e != nil {
		return "", e
	}
	for _, f := range Filters().Groups["vehicles"] {
		if code == f.Value {
			switch f.Label {
			case "トラベルトレーラー":
				return "トレーラー", nil
			case "軽キャンピングカー":
				return "軽キャン", nil
			}
			return f.Label, nil
		}
	}
	return "", nil
}

func FacilityKey(value string) (string, error) {
	code, e := resolve("facilities", value)
	if e != nil {
		return "", e
	}
	return map[string]string{"1": "garbage", "2": "electricity", "3": "water", "4": "generator", "5": "bath", "6": "pets", "7": "premium_benefits", "8": "toilet_24h", "9": "dump_station", "10": "dog_run"}[code], nil
}
func PeriodLabel(value string) (string, error) {
	code, e := resolve("periods", value)
	if e != nil {
		return "", e
	}
	for _, f := range Filters().Groups["periods"] {
		if f.Value == code {
			return f.Label, nil
		}
	}
	return "", nil
}
