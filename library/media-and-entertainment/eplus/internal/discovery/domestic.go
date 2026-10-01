package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/net/html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var Genres = map[string]string{"concert": "100", "sports": "500", "event": "200", "theatre": "400", "classical": "600", "art": "300", "anime": "800", "film": "700"}
var Regions = map[string]string{"hokkaido-tohoku": "01", "kanto": "05", "hokushinetsu": "09", "tokai": "10", "kansai": "11", "chugoku-shikoku": "12", "kyushu-okinawa": "19", "overseas": "99"}

type SearchOptions struct {
	Keyword, Artist, Venue, Location, From, To, Category, Region string
	Limit, Pages, Page                                           int
}
type Result struct {
	Data []Row `json:"data"`
	Meta Row   `json:"meta"`
}

func result(c *Client, data []Row, obs []Observation, total any, partial bool, warnings []string) Result {
	return Result{data, Row{"observations": obs, "checked_at": nowUTC(), "source_total": total, "returned": len(data), "partial": partial, "warnings": warnings, "stats": c.Stats()}}
}
func nowUTC() string { return timeNow().UTC().Format("2006-01-02T15:04:05Z") }

// timeNow is a test seam for source freshness, not sale-status inference.
var timeNow = func() time.Time { return time.Now() }

func ValidateSearch(o SearchOptions) error {
	if o.Limit < 1 || o.Limit > 100 {
		return fmt.Errorf("limit must be 1..100")
	}
	if o.Pages < 1 || o.Pages > 5 || o.Page < 1 || o.Page > 50 {
		return fmt.Errorf("pages must be 1..5 and page 1..50")
	}
	if e := ValidateDate(o.From); e != nil {
		return e
	}
	if e := ValidateDate(o.To); e != nil {
		return e
	}
	if o.From != "" && o.To != "" && o.From > o.To {
		return fmt.Errorf("from must be on or before to")
	}
	if len([]rune(o.Keyword))+len([]rune(o.Artist)) > 40 {
		return fmt.Errorf("combined keyword and artist must be at most 40 characters")
	}
	if o.Category != "" && Genres[o.Category] == "" {
		return fmt.Errorf("unknown category %q", o.Category)
	}
	if o.Region != "" && Regions[o.Region] == "" {
		return fmt.Errorf("unknown region %q", o.Region)
	}
	return nil
}
func domesticParams(o SearchOptions) url.Values {
	q := url.Values{"block": {"true"}}
	k := strings.TrimSpace(o.Keyword + " " + o.Artist)
	if k != "" {
		q.Set("keyword", k)
	}
	if o.From != "" {
		q.Set("koen_from_filter", strings.ReplaceAll(o.From, "-", ""))
	}
	if o.To != "" {
		q.Set("koen_to_filter", strings.ReplaceAll(o.To, "-", ""))
	}
	if o.Category != "" {
		q.Set("p_genre_filter", Genres[o.Category])
	}
	if o.Region != "" {
		q.Set("chiho_filter", Regions[o.Region])
	}
	return q
}

var publicTokenRE = regexp.MustCompile(`var APIV3_TOKEN\s*=\s*\{\s*'X-APIToken'\s*:\s*'([^']+)'\s*\}`)

func (c *Client) apiPage(ctx context.Context, n *html.Node, page int) (Row, []Observation, error) {
	obs := []Observation{}
	uri, params := "", ""
	for _, x := range nodes(n, func(x *html.Node) bool { return x.Data == "input" }) {
		switch attr(x, "id") {
		case "seemoreuri1":
			uri = attr(x, "value")
		case "seemoreparam1":
			params = attr(x, "value")
		}
	}
	if uri != "/v3/koen/keyword" && uri != "/v3/koen" {
		return nil, obs, fmt.Errorf("public search pagination contract missing")
	}
	q, e := url.ParseQuery(params)
	if e != nil {
		return nil, obs, fmt.Errorf("pagination parameters invalid")
	}
	q.Set("shutoku_kensu", "20")
	q.Set("shutoku_start_ichi", strconv.Itoa((page-1)*20+1))
	b, ob, e := c.fetch(ctx, "https://eplus.jp/s/eplus/js/property.js", nil, false)
	obs = append(obs, ob)
	if e != nil {
		return nil, obs, e
	}
	m := publicTokenRE.FindSubmatch(b)
	if m == nil {
		return nil, obs, fmt.Errorf("public website pagination token contract changed")
	}
	b, ob, e = c.fetch(ctx, "https://api.eplus.jp"+uri+"?"+q.Encode(), map[string]string{"X-APIToken": string(m[1]), "Referer": "https://eplus.jp/"}, true)
	obs = append(obs, ob)
	if e != nil {
		return nil, obs, e
	}
	var envelope Row
	if e = json.Unmarshal(b, &envelope); e != nil {
		return nil, obs, fmt.Errorf("pagination response is not JSON")
	}
	if envelope["error"] != nil {
		return nil, obs, fmt.Errorf("pagination source error")
	}
	d := obj(envelope["data"])
	if _, ok := d["record_list"]; !ok {
		return nil, obs, fmt.Errorf("pagination record_list missing")
	}
	return d, obs, nil
}
func (c *Client) DomesticSearch(ctx context.Context, o SearchOptions) (Result, error) {
	if e := ValidateSearch(o); e != nil {
		return Result{}, e
	}
	defaultDate := false
	if strings.TrimSpace(o.Keyword+o.Artist) == "" && o.From == "" && o.To == "" && o.Category == "" {
		o.From = timeNow().In(jst).Format("2006-01-02")
		defaultDate = true
	}
	u := "https://eplus.jp/sf/search?" + domesticParams(o).Encode()
	b, ob, e := c.Fetch(ctx, u)
	if e != nil {
		return Result{}, e
	}
	n, e := document(b)
	if e != nil {
		return Result{}, e
	}
	d, e := searchJSON(n)
	if e != nil {
		return Result{}, e
	}
	out := []Row{}
	obs := []Observation{ob}
	warnings := []string{}
	seen := map[string]bool{}
	partial := false
	total := d["so_kensu"]
	fetched := 0
	scannedPages := 0
	for p := o.Page; p < o.Page+o.Pages; p++ {
		if p > 1 {
			if e = pause(ctx, 300*time.Millisecond); e != nil {
				return Result{}, e
			}
			var more []Observation
			d, more, e = c.apiPage(ctx, n, p)
			obs = append(obs, more...)
			if e != nil {
				if len(out) == 0 {
					return Result{}, e
				}
				partial = true
				warnings = append(warnings, e.Error())
				break
			}
		}
		scannedPages++
		records := list(d["record_list"])
		fetched += len(records)
		for _, x := range records {
			row := domesticRecord(obj(x))
			id := str(row["id"])
			if seen[id] {
				continue
			}
			seen[id] = true
			if !matches(row, o) {
				continue
			}
			out = append(out, row)
			if len(out) >= o.Limit {
				break
			}
		}
		if len(out) >= o.Limit || len(records) < 20 {
			break
		}
	}
	count, _ := total.(float64)
	if float64(fetched+(o.Page-1)*20) < count || len(out) >= o.Limit && count > float64(len(out)) {
		partial = true
	}
	if o.Venue != "" || o.Location != "" || o.Region != "" {
		warnings = append(warnings, "region/venue/location match only the bounded fetched pages; source_total is before those local filters")
	}
	r := result(c, out, obs, total, partial, warnings)
	r.Meta["source"] = "domestic"
	if defaultDate {
		r.Meta["default_from"] = o.From
	}
	r.Meta["page"] = o.Page
	r.Meta["pages_requested"] = o.Pages
	r.Meta["search_url"] = u
	r.Meta["scanned_records"] = fetched
	r.Meta["scanned_pages"] = scannedPages
	if len(out) == 0 && partial {
		r.Meta["note"] = "no match within the scanned pages; raise --pages or refine --keyword/--region"
	}
	return r, nil
}
func matches(r Row, o SearchOptions) bool {
	date, end := str(r["date"]), str(r["date_end"])
	if end == "" {
		end = date
	}
	if o.From != "" && (end == "" || end < o.From) {
		return false
	}
	if o.To != "" && (date == "" || date > o.To) {
		return false
	}
	if o.Region != "" && !inRegion(r, o.Region) {
		return false
	}
	if o.Venue != "" && !strings.Contains(strings.ToLower(str(obj(r["venue"])["name"])), strings.ToLower(o.Venue)) {
		return false
	}
	if o.Location != "" && !strings.Contains(strings.ToLower(str(r["region"])+" "+str(obj(r["venue"])["name"])), strings.ToLower(o.Location)) {
		return false
	}
	return true
}
func domesticRecord(x Row) Row {
	r := newSession()
	r["url"] = absolute("https://eplus.jp", str(x["koen_detail_url_pc"]))
	event := eventID(str(r["url"]))
	if event == "" {
		event = str(x["kogyo_code"]) + str(x["kogyo_sub_code"])
	}
	r["event_id"] = event
	r["id"] = event + "/" + str(x["koen_code"])
	r["source"] = "domestic"
	r["source_event_code"] = nullable(str(x["kogyo_code"]))
	r["source_sub_code"] = nullable(str(x["kogyo_sub_code"]))
	if sid := sessionID(str(r["url"])); sid != "" {
		r["id"] = sid
	}
	r["source_performance_code"] = nullable(str(x["koen_code"]))
	k := obj(x["kanren_kogyo_sub"])
	r["name"] = nullable(strings.TrimSpace(str(k["kogyo_name_1"]) + " " + str(k["kogyo_name_2"])))
	date := parseDate(str(x["koenbi_term"]))
	r["date"] = nullable(date)
	if term := str(x["koenbi_term"]); len(term) > 8 {
		ds := regexp.MustCompile(`\d{8}`).FindAllString(term, -1)
		if len(ds) > 1 {
			r["date"] = nullable(parseDate(ds[0]))
			r["date_end"] = nullable(parseDate(ds[len(ds)-1]))
			date = str(r["date"])
		}
	}
	r["doors_at"] = at(date, str(x["kaijo_time"]))
	r["start_at"] = at(date, str(x["kaien_time"]))
	r["end_at"] = at(date, str(x["shuen_time"]))
	v := obj(x["kanren_venue"])
	r["venue"] = Row{"id": nullable(str(v["venue_code"])), "name": nullable(str(v["venue_name"])), "url": nullable(venueURL(str(v["venue_code"])))}
	r["region"] = nullable(str(v["todofuken_name"]))
	r["prefecture_code"] = nullable(str(v["todofuken_code"]))
	sales := []Row{}
	for _, s := range list(x["kanren_uketsuke_koen_list"]) {
		a := obj(s)
		kind := saleKind(str(a["hambai_hoho_label"]), str(a["hambai_hoho_kubun"]))
		handled, _ := a["eplus_toriatsukai_ari_flag"].(bool)
		cancelled, _ := a["kyuen_flag"].(bool)
		status, inventory := saleState("", str(a["uketsuke_status"]), kind, cancelled, handled)
		sale := newSale(str(r["id"])+"/"+str(a["uketsuke_info_code"]), str(a["uketsuke_name_pc"]), kind, status, inventory, compactDateTime(str(a["uketsuke_start_datetime"])), compactDateTime(str(a["uketsuke_end_datetime"])), r["url"])
		sale["raw_status"] = a["uketsuke_status"]
		sale["source_round_code"] = a["uketsuke_info_code"]
		sale["source_kind"] = a["hambai_hoho_label"]
		sale["booking_url_kind"] = "detail_handoff"
		sales = append(sales, sale)
	}
	r["sales"] = sales
	return r
}
func venueURL(id string) string {
	if id == "" {
		return ""
	}
	return "https://eplus.jp/sf/venue/" + id
}
func (c *Client) DomesticDetail(ctx context.Context, id string) (Result, error) {
	u, e := DomesticURL(id)
	if e != nil {
		return Result{}, e
	}
	b, ob, e := c.Fetch(ctx, u)
	if e != nil {
		return Result{}, e
	}
	n, e := document(b)
	if e != nil {
		return Result{}, e
	}
	rows, e := parseDomesticDetail(n, u)
	if e != nil {
		return Result{}, e
	}
	r := result(c, rows, []Observation{ob}, len(rows), false, []string{"prices, fees and conditions not displayed on this public detail surface remain unknown; follow each booking link for the final terms"})
	r.Meta["source"] = "domestic"
	return r, nil
}

var prefectureNames = []string{"北海道", "青森県", "岩手県", "宮城県", "秋田県", "山形県", "福島県", "茨城県", "栃木県", "群馬県", "埼玉県", "千葉県", "東京都", "神奈川県", "新潟県", "富山県", "石川県", "福井県", "山梨県", "長野県", "岐阜県", "静岡県", "愛知県", "三重県", "滋賀県", "京都府", "大阪府", "兵庫県", "奈良県", "和歌山県", "鳥取県", "島根県", "岡山県", "広島県", "山口県", "徳島県", "香川県", "愛媛県", "高知県", "福岡県", "佐賀県", "長崎県", "熊本県", "大分県", "宮崎県", "鹿児島県", "沖縄県"}

func inRegion(r Row, region string) bool {
	code, _ := strconv.Atoi(str(r["prefecture_code"]))
	if code == 0 {
		for i, n := range prefectureNames {
			if str(r["region"]) == n {
				code = i + 1
				break
			}
		}
	}
	switch region {
	case "hokkaido-tohoku":
		return code >= 1 && code <= 7
	case "kanto":
		return code >= 8 && code <= 14
	case "hokushinetsu":
		return code >= 15 && code <= 20
	case "tokai":
		return code >= 21 && code <= 24
	case "kansai":
		return code >= 25 && code <= 30
	case "chugoku-shikoku":
		return code >= 31 && code <= 39
	case "kyushu-okinawa":
		return code >= 40 && code <= 47
	case "overseas":
		return code == 99 || str(r["region"]) == "海外"
	}
	return false
}
