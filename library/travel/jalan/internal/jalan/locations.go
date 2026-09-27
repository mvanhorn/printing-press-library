package jalan

import (
	"strings"
)

// Location aliases deliberately resolve to source areas, not translated keywords.
type Location struct {
	AreaCode string   `json:"area_code"`
	NameJa   string   `json:"name_ja"`
	Aliases  []string `json:"aliases"`
	Kind     string   `json:"kind"`
	Scope    string   `json:"scope"`
	URL      string   `json:"url"`
}

// Prefecture codes and labels are the source homepage kenCd dropdown, verified
// 2026-09-27. English aliases are explicit; the IDs do not follow ISO ordering.
// Japanese short names remove only one final 県/都/府 suffix; 北海道 stays intact.
var locationCatalogue = []Location{
	{AreaCode: "010000", NameJa: "北海道", Aliases: []string{"Hokkaido", "北海道"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/010000/"},
	{AreaCode: "020000", NameJa: "青森県", Aliases: []string{"Aomori", "青森県", "青森"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/020000/"},
	{AreaCode: "030000", NameJa: "岩手県", Aliases: []string{"Iwate", "岩手県", "岩手"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/030000/"},
	{AreaCode: "040000", NameJa: "宮城県", Aliases: []string{"Miyagi", "宮城県", "宮城"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/040000/"},
	{AreaCode: "050000", NameJa: "秋田県", Aliases: []string{"Akita", "秋田県", "秋田"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/050000/"},
	{AreaCode: "060000", NameJa: "山形県", Aliases: []string{"Yamagata", "山形県", "山形"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/060000/"},
	{AreaCode: "070000", NameJa: "福島県", Aliases: []string{"Fukushima", "福島県", "福島"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/070000/"},
	{AreaCode: "080000", NameJa: "栃木県", Aliases: []string{"Tochigi", "栃木県", "栃木"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/080000/"},
	{AreaCode: "090000", NameJa: "群馬県", Aliases: []string{"Gunma", "群馬県", "群馬"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/090000/"},
	{AreaCode: "100000", NameJa: "茨城県", Aliases: []string{"Ibaraki", "茨城県", "茨城"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/100000/"},
	{AreaCode: "110000", NameJa: "埼玉県", Aliases: []string{"Saitama", "埼玉県", "埼玉"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/110000/"},
	{AreaCode: "120000", NameJa: "千葉県", Aliases: []string{"Chiba", "千葉県", "千葉"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/120000/"},
	{AreaCode: "130000", NameJa: "東京都", Aliases: []string{"Tokyo", "東京都", "東京"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/130000/"},
	{AreaCode: "140000", NameJa: "神奈川県", Aliases: []string{"Kanagawa", "神奈川県", "神奈川"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/140000/"},
	{AreaCode: "150000", NameJa: "山梨県", Aliases: []string{"Yamanashi", "山梨県", "山梨"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/150000/"},
	{AreaCode: "160000", NameJa: "長野県", Aliases: []string{"Nagano", "長野県", "長野"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/160000/"},
	{AreaCode: "170000", NameJa: "新潟県", Aliases: []string{"Niigata", "新潟県", "新潟"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/170000/"},
	{AreaCode: "180000", NameJa: "富山県", Aliases: []string{"Toyama", "富山県", "富山"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/180000/"},
	{AreaCode: "190000", NameJa: "石川県", Aliases: []string{"Ishikawa", "石川県", "石川"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/190000/"},
	{AreaCode: "200000", NameJa: "福井県", Aliases: []string{"Fukui", "福井県", "福井"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/200000/"},
	{AreaCode: "210000", NameJa: "静岡県", Aliases: []string{"Shizuoka", "静岡県", "静岡"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/210000/"},
	{AreaCode: "220000", NameJa: "岐阜県", Aliases: []string{"Gifu", "岐阜県", "岐阜"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/220000/"},
	{AreaCode: "230000", NameJa: "愛知県", Aliases: []string{"Aichi", "愛知県", "愛知"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/230000/"},
	{AreaCode: "240000", NameJa: "三重県", Aliases: []string{"Mie", "三重県", "三重"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/240000/"},
	{AreaCode: "250000", NameJa: "滋賀県", Aliases: []string{"Shiga", "滋賀県", "滋賀"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/250000/"},
	{AreaCode: "260000", NameJa: "京都府", Aliases: []string{"Kyoto", "京都府", "京都"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/260000/"},
	{AreaCode: "270000", NameJa: "大阪府", Aliases: []string{"Osaka", "大阪府", "大阪"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/270000/"},
	{AreaCode: "280000", NameJa: "兵庫県", Aliases: []string{"Hyogo", "兵庫県", "兵庫"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/280000/"},
	{AreaCode: "290000", NameJa: "奈良県", Aliases: []string{"Nara", "奈良県", "奈良"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/290000/"},
	{AreaCode: "300000", NameJa: "和歌山県", Aliases: []string{"Wakayama", "和歌山県", "和歌山"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/300000/"},
	{AreaCode: "310000", NameJa: "鳥取県", Aliases: []string{"Tottori", "鳥取県", "鳥取"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/310000/"},
	{AreaCode: "320000", NameJa: "島根県", Aliases: []string{"Shimane", "島根県", "島根"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/320000/"},
	{AreaCode: "330000", NameJa: "岡山県", Aliases: []string{"Okayama", "岡山県", "岡山"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/330000/"},
	{AreaCode: "340000", NameJa: "広島県", Aliases: []string{"Hiroshima", "広島県", "広島"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/340000/"},
	{AreaCode: "350000", NameJa: "山口県", Aliases: []string{"Yamaguchi", "山口県", "山口"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/350000/"},
	{AreaCode: "360000", NameJa: "徳島県", Aliases: []string{"Tokushima", "徳島県", "徳島"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/360000/"},
	{AreaCode: "370000", NameJa: "香川県", Aliases: []string{"Kagawa", "香川県", "香川"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/370000/"},
	{AreaCode: "380000", NameJa: "愛媛県", Aliases: []string{"Ehime", "愛媛県", "愛媛"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/380000/"},
	{AreaCode: "390000", NameJa: "高知県", Aliases: []string{"Kochi", "高知県", "高知"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/390000/"},
	{AreaCode: "400000", NameJa: "福岡県", Aliases: []string{"Fukuoka", "福岡県", "福岡"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/400000/"},
	{AreaCode: "410000", NameJa: "佐賀県", Aliases: []string{"Saga", "佐賀県", "佐賀"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/410000/"},
	{AreaCode: "420000", NameJa: "長崎県", Aliases: []string{"Nagasaki", "長崎県", "長崎"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/420000/"},
	{AreaCode: "430000", NameJa: "熊本県", Aliases: []string{"Kumamoto", "熊本県", "熊本"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/430000/"},
	{AreaCode: "440000", NameJa: "大分県", Aliases: []string{"Oita", "大分県", "大分"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/440000/"},
	{AreaCode: "450000", NameJa: "宮崎県", Aliases: []string{"Miyazaki", "宮崎県", "宮崎"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/450000/"},
	{AreaCode: "460000", NameJa: "鹿児島県", Aliases: []string{"Kagoshima", "鹿児島県", "鹿児島"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/460000/"},
	{AreaCode: "470000", NameJa: "沖縄県", Aliases: []string{"Okinawa", "沖縄県", "沖縄"}, Kind: "prefecture", Scope: "entire prefecture", URL: "https://www.jalan.net/470000/"},
	{AreaCode: "141600", NameJa: "箱根", Aliases: []string{"Hakone", "箱根"}, Kind: "large_area", Scope: "Jalan Hakone large area", URL: "https://www.jalan.net/140000/LRG_141600/"},
	{AreaCode: "138000", NameJa: "新宿・中野・杉並・吉祥寺", Aliases: []string{"Shinjuku", "新宿", "新宿・中野・杉並・吉祥寺"}, Kind: "large_area", Scope: "broader than Shinjuku: includes Nakano, Suginami and Kichijoji", URL: "https://www.jalan.net/130000/LRG_138000/"},
	{AreaCode: "136200", NameJa: "銀座・日本橋・東京駅周辺", Aliases: []string{"Tokyo Station", "Ginza", "東京駅", "銀座", "銀座・日本橋・東京駅周辺"}, Kind: "large_area", Scope: "Ginza, Nihonbashi and Tokyo Station surroundings", URL: "https://www.jalan.net/130000/LRG_136200/"},
	{AreaCode: "260500", NameJa: "京都駅周辺", Aliases: []string{"Kyoto Station", "京都駅", "京都駅周辺"}, Kind: "large_area", Scope: "Kyoto Station surroundings", URL: "https://www.jalan.net/260000/LRG_260500/"},
	{AreaCode: "440500", NameJa: "別府", Aliases: []string{"Beppu", "別府"}, Kind: "large_area", Scope: "Jalan Beppu large area", URL: "https://www.jalan.net/440000/LRG_440500/"},
	{AreaCode: "440600", NameJa: "湯布院", Aliases: []string{"Yufuin", "湯布院", "由布院"}, Kind: "large_area", Scope: "Jalan Yufuin large area", URL: "https://www.jalan.net/440000/LRG_440600/"},
	{AreaCode: "090200", NameJa: "草津・尻焼・花敷", Aliases: []string{"Kusatsu", "草津", "草津・尻焼・花敷"}, Kind: "large_area", Scope: "broader than Kusatsu: includes Shiriyaki and Hanashiki", URL: "https://www.jalan.net/090000/LRG_090200/"},
	{AreaCode: "281100", NameJa: "城崎・竹野・豊岡", Aliases: []string{"Kinosaki", "城崎", "城崎・竹野・豊岡"}, Kind: "large_area", Scope: "broader than Kinosaki: includes Takeno and Toyooka", URL: "https://www.jalan.net/280000/LRG_281100/"},
}

func aliasKey(value string) string { return strings.ToLower(strings.Join(strings.Fields(value), " ")) }
func resolveLocation(query string) (Location, error) {
	key := aliasKey(query)
	matches := []Location{}
	for _, location := range locationCatalogue {
		for _, alias := range location.Aliases {
			if aliasKey(alias) == key {
				matches = append(matches, location)
				break
			}
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return Location{}, usage("ambiguous_destination", "destination alias resolves to more than one source area", "Use stay locations --query to inspect areas and pass --area-code explicitly.")
	}
	return Location{}, usage("unknown_destination", "destination is not an explicit supported alias", "Use stay locations --query, or pass a six-digit Jalan --area-code; English keyword matching is not translated area search.")
}

func Locations(query string) (Response, error) {
	response := Response{Meta: map[string]any{"source": "Jalan public area catalogue", "source_url": "https://www.jalan.net/", "source_verified_at": "2026-09-27", "timezone": "Asia/Tokyo", "status": "ok", "coverage": "curated aliases and all 47 source prefectures", "query": query, "upstream_requests": 0, "elapsed_ms": 0, "cache_status": "catalogue", "warnings": []string{}}, Results: []any{}, Pagination: map[string]any{}, FetchFailures: []map[string]any{}}
	key := aliasKey(query)
	for _, location := range locationCatalogue {
		matched := key == "" || strings.Contains(aliasKey(location.NameJa), key) || strings.Contains(location.AreaCode, key)
		for _, alias := range location.Aliases {
			if strings.Contains(aliasKey(alias), key) {
				matched = true
			}
		}
		if matched {
			response.Results = append(response.Results, location)
		}
	}
	if len(response.Results) == 0 {
		response.Meta["status"] = "no_matches"
		response.Meta["next_action"] = "Use an explicit six-digit Jalan --area-code for unlisted large areas."
	}
	response.Pagination["returned_count"] = len(response.Results)
	response.Pagination["has_more"] = false
	return response, nil
}

func Capabilities() Response {
	return Response{Meta: map[string]any{"source": "Jalan anonymous Japanese public HTML", "source_url": sourceBaseURL, "source_verified_at": "2026-09-27", "timezone": "Asia/Tokyo", "status": "ok", "upstream_requests": 0, "elapsed_ms": 0, "cache_status": "catalogue", "warnings": []string{}}, Results: []any{map[string]any{
		"commands":                []string{"stay locations", "stay search", "stay property", "stay offers", "stay plan", "stay compare", "stay capabilities"},
		"coverage":                "Japanese accommodation; all prefectures through source area codes and explicit curated aliases",
		"inventory_requires_date": true, "same_occupancy_per_room": true, "children_categories": []string{"elementary", "infant_meals_bed", "infant_meals", "infant_bed", "infant_neither"},
		"filters":      []string{"meals", "lodging_type", "onsen", "outdoor_bath", "private_bath", "room_outdoor_bath", "non_smoking"},
		"bounds":       map[string]any{"results_per_page_max": 30, "search_source_pages_max": 2, "compare_alternatives_max": 5, "request_timeout_seconds": 20, "command_timeout_seconds": 60, "retry_max": 1, "response_bytes_max": maxResponseBytes, "concurrency_max": 2, "nights_max": 9, "rooms_max": 10, "adults_per_room_max": 8, "child_category_per_room_max": 5, "client_date_window_days": 365, "inventory_cache_age_max_seconds": 300},
		"price_policy": "Observed base quotes preserve source basis. Coupons, points and unresolved extra fees are separate; final payable may be unknown.",
		"limitations":  []string{"bounded observed subset; no exhaustive inventory or cheapest guarantee", "English aliases explicitly select source areas; arbitrary English keywords are unsupported", "no heterogeneous room occupancy", "no booking, account, coupon redemption or payment actions", "legacy API keys and other-language MCP are excluded"},
	}}, Pagination: map[string]any{"returned_count": 1, "has_more": false}, FetchFailures: []map[string]any{}}
}
