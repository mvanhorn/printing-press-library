// Package driveplaza extracts bounded planning facts from public NEXCO East pages.
package driveplaza

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/driveplaza/internal/cliutil"
)

const English = "https://en.driveplaza.com"
const Japanese = "https://www.driveplaza.com"
const maxBody = 2 << 20

var jst = time.FixedZone("JST", 9*60*60)
var roadID = regexp.MustCompile(`^[0-9]{4}$`)
var stopID = regexp.MustCompile(`^([0-9]{4})/([0-9]{7})/([12])$`)
var icID = regexp.MustCompile(`^[0-9]{7}$`)

// Client uses public HTTP only. One instance is scoped to one bounded command.
type Client struct {
	HTTP     *http.Client
	EN, JP   string
	limiter  *cliutil.AdaptiveLimiter
	requests int
	urls     []string
	warnings []string
}

func New(rate float64) *Client {
	if rate <= 0 || rate > 2 {
		rate = 2
	}
	return &Client{EN: English, JP: Japanese, HTTP: &http.Client{Timeout: 20 * time.Second}, limiter: cliutil.NewAdaptiveLimiter(rate), urls: []string{}, warnings: []string{}}
}

type Meta struct {
	Source      string   `json:"source"`
	Provider    string   `json:"provider"`
	RetrievedAt string   `json:"retrieved_at"`
	Timezone    string   `json:"timezone"`
	SourceURLs  []string `json:"source_urls"`
	Requests    int      `json:"upstream_requests"`
	Warnings    []string `json:"warnings"`
}
type Output[T any] struct {
	Meta    Meta `json:"meta"`
	Results T    `json:"results"`
}
type Page[T any] struct {
	Items          []T    `json:"items"`
	Total          int    `json:"total"`
	Offset         int    `json:"offset"`
	Limit          int    `json:"limit"`
	NextOffset     *int   `json:"next_offset"`
	ScannedItems   int    `json:"scanned_items"`
	Note           string `json:"note,omitempty"`
	ScanLimited    bool   `json:"scan_limited,omitempty"`
	MaxScanRecords int    `json:"max_scan_records,omitempty"`
}

func page[T any](items []T, limit, offset, scanned int) Page[T] {
	end := min(len(items), offset+limit)
	start := min(len(items), offset)
	out := Page[T]{Items: append([]T{}, items[start:end]...), Total: len(items), Offset: offset, Limit: limit, ScannedItems: scanned}
	if end < len(items) {
		out.NextOffset = &end
	}
	if len(out.Items) == 0 {
		out.Note = "No matching records in this source response; this does not establish current road or facility status."
	}
	return out
}
func (c *Client) meta() Meta {
	return Meta{Source: "live", Provider: "NEXCO East / Drive Plaza", RetrievedAt: time.Now().UTC().Format(time.RFC3339), Timezone: "Asia/Tokyo", SourceURLs: append([]string{}, c.urls...), Requests: c.requests, Warnings: append([]string{}, c.warnings...)}
}
func (c *Client) get(ctx context.Context, base, path string, q url.Values) ([]byte, error) {
	if c.requests >= 40 {
		return nil, fmt.Errorf("command reached the 40-request cap; reduce --limit")
	}
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	u := strings.TrimRight(base, "/") + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "driveplaza-pp-cli/0.1 public-readonly")
	c.requests++
	c.urls = append(c.urls, u)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w; retry or check the source page", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: u, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s; inspect the official source page", resp.StatusCode, u)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("read source response: %w", err)
	}
	if len(body) > maxBody {
		return nil, fmt.Errorf("source response exceeds 2 MiB cap; narrow the road or query")
	}
	c.limiter.OnSuccess()
	return body, nil
}
func (c *Client) partial(err error, identity string) error {
	var limited *cliutil.RateLimitError
	if errors.As(err, &limited) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return err
	}
	c.warnings = append(c.warnings, identity+": "+err.Error())
	return nil
}

// RouteOptions are quote assumptions, not eligibility guarantees.
type RouteOptions struct {
	From            string   `json:"from"`
	To              string   `json:"to"`
	At              string   `json:"at_jst"`
	Vehicle         string   `json:"vehicle"`
	Priority        string   `json:"priority"`
	TimeKind        string   `json:"time_kind"`
	Payment         string   `json:"payment"`
	Via             []string `json:"via"`
	ExcludeUrban    bool     `json:"exclude_urban_expressways"`
	ExcludeOrdinary bool     `json:"exclude_ordinary_roads"`
}

var vehicles = map[string]string{"light": "0", "standard": "1", "medium": "2", "large": "3", "extra-large": "4"}
var priorities = map[string]string{"distance": "1", "time": "2", "toll": "3"}
var timeKinds = map[string]string{"departure": "1", "arrival": "2"}

// ValidateRoute validates a real JST minute and preserves the form's 10-minute increments.
func ValidateRoute(o RouteOptions) (url.Values, error) {
	if strings.TrimSpace(o.From) == "" || strings.TrimSpace(o.To) == "" {
		return nil, fmt.Errorf("--from and --to require exact English IC names; resolve them with interchanges --language en")
	}
	if len(o.From) > 100 || len(o.To) > 100 || len(o.Via) > 5 {
		return nil, fmt.Errorf("--from/--to must be at most 100 bytes; --via accepts at most 5 waypoints")
	}
	t, err := time.ParseInLocation("2006-01-02T15:04", o.At, jst)
	if err != nil || t.Format("2006-01-02T15:04") != o.At || t.Minute()%10 != 0 {
		return nil, fmt.Errorf("--at must be YYYY-MM-DDTHH:MM in JST, with minutes00,10,20,30,40 or50")
	}
	v, ok := vehicles[o.Vehicle]
	if !ok {
		return nil, fmt.Errorf("--vehicle must be light, standard, medium, large or extra-large")
	}
	p, ok := priorities[o.Priority]
	if !ok {
		return nil, fmt.Errorf("--priority must be time, distance or toll")
	}
	k, ok := timeKinds[o.TimeKind]
	if !ok {
		return nil, fmt.Errorf("--time-kind must be departure or arrival")
	}
	if o.Payment != "standard" && o.Payment != "etc" && o.Payment != "etc2" {
		return nil, fmt.Errorf("--payment must be standard, etc or etc2; it selects a source column, not a discount formula")
	}
	q := url.Values{"startPlaceKana": {strings.TrimSpace(o.From)}, "arrivePlaceKana": {strings.TrimSpace(o.To)}, "searchYear": {t.Format("2006")}, "searchMonth": {fmt.Sprint(int(t.Month()))}, "searchDay": {fmt.Sprint(t.Day())}, "searchHour": {fmt.Sprint(t.Hour())}, "searchMinute": {fmt.Sprint(t.Minute())}, "kind": {k}, "carType": {v}, "priority": {p}, "selectickindflg": {"0"}}
	for i := 0; i < 5; i++ {
		key := "keiyuPlaceKana"
		if i > 0 {
			key += fmt.Sprint(i + 1)
		}
		q.Set(key, "")
		if i < len(o.Via) {
			if strings.TrimSpace(o.Via[i]) == "" || len(o.Via[i]) > 100 {
				return nil, fmt.Errorf("--via requires nonempty English IC names of at most100 bytes")
			}
			q.Set(key, strings.TrimSpace(o.Via[i]))
		}
	}
	if o.ExcludeUrban {
		q.Set("roadType1", "on")
	}
	if o.ExcludeOrdinary {
		q.Set("roadType2", "on")
	}
	return q, nil
}

func ValidPage(limit, offset int) error {
	if limit < 1 || limit > 30 || offset < 0 || offset > 10000 {
		return fmt.Errorf("--limit must be1..30 and --offset0..10000")
	}
	return nil
}

type Interchange struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	NameJA   *string `json:"name_ja"`
	Reading  *string `json:"reading_ja"`
	Type     string  `json:"source_type"`
	RoadID   *string `json:"road_id"`
	RoadName *string `json:"road_name"`
}
type ICOptions struct {
	Query, Language, Kind, Code, Road string
	Limit, Offset                     int
}

func (c *Client) Interchanges(ctx context.Context, o ICOptions) (Output[Page[Interchange]], error) {
	var out Output[Page[Interchange]]
	if err := ValidPage(o.Limit, o.Offset); err != nil {
		return out, err
	}
	if o.Language != "en" && o.Language != "ja" {
		return out, fmt.Errorf("--language must be ja or en")
	}
	if o.Kind != "start" && o.Kind != "arrive" {
		return out, fmt.Errorf("--kind must be start or arrive")
	}
	if o.Road != "" && !roadID.MatchString(o.Road) {
		return out, fmt.Errorf("--road must be a4-digit road ID from roads")
	}
	if o.Code != "" && (!icID.MatchString(o.Code) || o.Query != "") {
		return out, fmt.Errorf("use exactly one of --query or a7-digit --code")
	}
	if o.Code != "" && o.Road != "" {
		return out, fmt.Errorf("--road cannot be combined with --code because code lookup does not provide road identity; use --query")
	}
	if o.Code == "" && (strings.TrimSpace(o.Query) == "" || len(o.Query) > 100) {
		return out, fmt.Errorf("--query requires1..100 bytes, or use --code")
	}
	base, path, q := c.JP, "/community/icsearch_api.php", url.Values{"ic_type": {o.Kind}, "val_word": {o.Query}}
	if o.Language == "en" {
		base = c.EN
	}
	if o.Code != "" {
		base = c.JP
		path = "/community/icsearch_fromcode_api.php"
		q = url.Values{"val_iccode": {o.Code}}
	}
	b, err := c.get(ctx, base, path, q)
	if err != nil {
		return out, err
	}
	items, err := parseIC(b, o.Language == "ja" || o.Code != "")
	if err != nil {
		return out, err
	}
	scanned := len(items)
	filtered := []Interchange{}
	for _, ic := range items {
		if o.Road == "" || (ic.RoadID != nil && *ic.RoadID == o.Road) {
			filtered = append(filtered, ic)
		}
	}
	p := page(filtered, o.Limit, o.Offset, scanned)
	if o.Language == "en" && o.Code == "" {
		for i := range p.Items {
			ic := &p.Items[i]
			b, err := c.get(ctx, c.JP, "/community/icsearch_fromcode_api.php", url.Values{"val_iccode": {ic.ID}})
			if err != nil {
				if err = c.partial(err, "Japanese name for IC "+ic.ID); err != nil {
					return out, err
				}
				continue
			}
			ja, err := parseIC(b, true)
			if err == nil && len(ja) == 1 && ja[0].ID == ic.ID {
				ic.NameJA = ja[0].NameJA
				ic.Reading = ja[0].Reading
			} else {
				c.warnings = append(c.warnings, "Japanese name unavailable for IC "+ic.ID)
			}
		}
	}
	if o.Code != "" {
		c.warnings = append(c.warnings, "Code lookup is the Japanese endpoint; it does not provide an English route-entry name or road ID.")
	}
	out = Output[Page[Interchange]]{Meta: c.meta(), Results: p}
	return out, nil
}

type Road struct {
	ID     string  `json:"id"`
	NameEN string  `json:"name_en"`
	NameJA *string `json:"name_ja"`
}

func (c *Client) Roads(ctx context.Context, query string, limit, offset int) (Output[Page[Road]], error) {
	var out Output[Page[Road]]
	if err := ValidPage(limit, offset); err != nil {
		return out, err
	}
	b, err := c.get(ctx, c.EN, "/dp/SAPAServiceEN", nil)
	if err != nil {
		return out, err
	}
	en, err := parseRoads(b)
	if err != nil {
		return out, err
	}
	b, err = c.get(ctx, c.JP, "/dp/SAPAService", nil)
	if err != nil {
		if err = c.partial(err, "Japanese road catalog"); err != nil {
			return out, err
		}
	} else {
		ja, e := parseRoads(b)
		if e != nil {
			c.warnings = append(c.warnings, e.Error())
		} else {
			for i := range en {
				for _, j := range ja {
					if j.ID == en[i].ID {
						en[i].NameJA = &j.NameEN
						break
					}
				}
			}
		}
	}
	filtered := []Road{}
	for _, r := range en {
		ja := ""
		if r.NameJA != nil {
			ja = *r.NameJA
		}
		if contains(r.ID+" "+r.NameEN+" "+ja, query) {
			filtered = append(filtered, r)
		}
	}
	return Output[Page[Road]]{Meta: c.meta(), Results: page(filtered, limit, offset, len(en))}, nil
}

type Facility struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Group bool   `json:"category_selection"`
}

func (c *Client) Facilities(ctx context.Context) (Output[[]Facility], error) {
	var out Output[[]Facility]
	b, err := c.get(ctx, c.EN, "/dp/SAPAServiceEN", nil)
	if err != nil {
		return out, err
	}
	items, err := parseFacilities(b)
	if err != nil {
		return out, err
	}
	return Output[[]Facility]{Meta: c.meta(), Results: items}, nil
}

type Stop struct {
	ID                 string           `json:"id"`
	NameEN             string           `json:"name_en"`
	NameJA             *string          `json:"name_ja"`
	Direction          string           `json:"direction"`
	RoadID             string           `json:"road_id"`
	RoadName           string           `json:"road_name"`
	URL                string           `json:"url"`
	URLJA              string           `json:"url_ja"`
	ParkingLarge       *int             `json:"parking_large_spaces"`
	ParkingSmall       *int             `json:"parking_small_spaces"`
	FacilityCategories map[string]*bool `json:"facility_categories"`
}
type StopOptions struct {
	Road, Direction, Query string
	Facilities             []string
	Open24                 bool
	Limit, Offset          int
	MaxScan                int
}

var sapaCaveat = "Source warns that other operators' SA/PA data is dated 2006-03-31. Facility hours are weekday guidance and may vary on holidays. Availability is not an open-now guarantee."

func (c *Client) Stops(ctx context.Context, o StopOptions) (Output[Page[Stop]], error) {
	var out Output[Page[Stop]]
	if err := ValidPage(o.Limit, o.Offset); err != nil {
		return out, err
	}
	if o.MaxScan == 0 {
		o.MaxScan = 500
	}
	if o.MaxScan < 1 || o.MaxScan > 2000 {
		return out, fmt.Errorf("--max-scan-records must be 1..2000")
	}
	if !roadID.MatchString(o.Road) {
		return out, fmt.Errorf("--road requires a4-digit road ID from roads; nationwide scans are not supported")
	}
	if o.Direction != "both" && o.Direction != "up" && o.Direction != "down" {
		return out, fmt.Errorf("--direction must be up, down or both; these are the source's directional identities")
	}
	q := url.Values{"HIGHWAY": {o.Road}, "AREA": {""}, "arealist": {"0"}, "keiroCodeCSV": {""}, "startIcName": {""}, "arriveIcName": {""}}
	if o.Direction == "up" {
		q.Set("UP", "1")
	}
	if o.Direction == "down" {
		q.Set("DOWN", "1")
	}
	if o.Open24 {
		q.Set("TM24", "1")
	}
	if len(o.Facilities) > 10 {
		return out, fmt.Errorf("--facility accepts at most10 source facility IDs")
	}
	if len(o.Facilities) > 0 {
		catalog, err := c.Facilities(ctx)
		if err != nil {
			return out, err
		}
		valid := map[string]bool{}
		for _, f := range catalog.Results {
			valid[f.ID] = true
		}
		for _, f := range o.Facilities {
			if !valid[f] {
				return out, fmt.Errorf("unknown --facility %q; list supported IDs with sapa facilities", f)
			}
			q.Add("ITEM", f)
		}
	}
	b, err := c.get(ctx, c.EN, "/dp/SAPAServResEN", q)
	if err != nil {
		return out, err
	}
	items, err := parseStops(b, c.EN, c.JP)
	if err != nil {
		return out, err
	}
	scanLimited := len(items) > o.MaxScan
	if scanLimited {
		items = items[:o.MaxScan]
	}
	b, err = c.get(ctx, c.JP, "/dp/SAPAServRes", q)
	if err != nil {
		if err = c.partial(err, "Japanese SA/PA names"); err != nil {
			return out, err
		}
	} else {
		ja, e := parseStops(b, c.JP, c.JP)
		if e != nil {
			c.warnings = append(c.warnings, e.Error())
		} else {
			names := map[string]string{}
			for _, s := range ja {
				names[s.ID] = s.NameEN
			}
			for i := range items {
				if n, ok := names[items[i].ID]; ok {
					items[i].NameJA = &n
				}
			}
		}
	}
	filtered := []Stop{}
	for _, s := range items {
		ja := ""
		if s.NameJA != nil {
			ja = *s.NameJA
		}
		if (o.Direction == "both" || s.Direction == o.Direction) && s.RoadID == o.Road && contains(s.NameEN+" "+ja, o.Query) {
			filtered = append(filtered, s)
		}
	}
	c.warnings = append(c.warnings, sapaCaveat)
	p := page(filtered, o.Limit, o.Offset, len(items))
	p.MaxScanRecords = o.MaxScan
	p.ScanLimited = scanLimited
	if scanLimited {
		p.Note = fmt.Sprintf("Local filtering examined the first %d source records; total covers this scan only. Raise --max-scan-records to widen the scan.", o.MaxScan)
	}
	return Output[Page[Stop]]{Meta: c.meta(), Results: p}, nil
}

type FacilitySection struct {
	Category string `json:"category"`
	Text     string `json:"source_text"`
}
type StopLink struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	DistanceKM *float64 `json:"distance_km"`
}
type StopDetail struct {
	ID         string            `json:"id"`
	NameEN     string            `json:"name_en"`
	NameJA     *string           `json:"name_ja"`
	Direction  string            `json:"direction"`
	RoadID     string            `json:"road_id"`
	RoadName   string            `json:"road_name"`
	URL        string            `json:"url"`
	URLJA      string            `json:"url_ja"`
	Sections   []FacilitySection `json:"facility_sections"`
	Nearby     []StopLink        `json:"nearby_directional_stops"`
	HoursBasis string            `json:"hours_basis"`
}

func (c *Client) Detail(ctx context.Context, id string) (Output[StopDetail], error) {
	var out Output[StopDetail]
	if !stopID.MatchString(id) {
		return out, fmt.Errorf("--id must be road/area/direction, e.g.1040/1040021/1; direction1=up,2=down")
	}
	path := "/sapa/" + id + "/"
	b, err := c.get(ctx, c.EN, path, nil)
	if err != nil {
		return out, err
	}
	detail, err := parseDetail(b, id, c.EN, c.JP)
	if err != nil {
		return out, err
	}
	b, err = c.get(ctx, c.JP, path, nil)
	if err != nil {
		if err = c.partial(err, "Japanese SA/PA detail name"); err != nil {
			return out, err
		}
	} else {
		n := japaneseDetailName(b)
		if n != "" {
			detail.NameJA = &n
		} else {
			c.warnings = append(c.warnings, "Japanese detail name missing in source response")
		}
	}
	c.warnings = append(c.warnings, sapaCaveat)
	return Output[StopDetail]{Meta: c.meta(), Results: detail}, nil
}

type RouteAlternative struct {
	ID                 string     `json:"id"`
	StandardJPY        *int       `json:"standard_jpy"`
	ETCJPY             *int       `json:"etc_jpy"`
	ETC2JPY            *int       `json:"etc2_jpy"`
	SelectedJPY        *int       `json:"selected_jpy"`
	IgnoringMinutes    *int       `json:"ignoring_traffic_minutes"`
	ConsideringMinutes *int       `json:"considering_traffic_minutes"`
	DistanceKM         *float64   `json:"distance_km"`
	Warnings           []string   `json:"warnings"`
	Stops              []StopLink `json:"directional_stops,omitempty"`
	ForecastURLs       []string   `json:"forecast_urls,omitempty"`
}
type RouteQuote struct {
	Assumptions  RouteOptions       `json:"assumptions"`
	Alternatives []RouteAlternative `json:"alternatives"`
	URL          string             `json:"url"`
	TimingBasis  string             `json:"timing_basis"`
	TollBasis    string             `json:"toll_basis"`
}

func (c *Client) Route(ctx context.Context, o RouteOptions, detail bool) (Output[RouteQuote], error) {
	var out Output[RouteQuote]
	q, err := ValidateRoute(o)
	if err != nil {
		return out, err
	}
	b, err := c.get(ctx, c.EN, "/dp/SearchQuickEN", q)
	if err != nil {
		return out, err
	}
	routes, err := parseRoutes(b, detail, c.EN)
	if err != nil {
		return out, err
	}
	doc := mustDocument(b)
	for name, requested := range map[string]bool{"roadType1": o.ExcludeUrban, "roadType2": o.ExcludeOrdinary} {
		if !requested {
			continue
		}
		applied := false
		for _, n := range all(doc, byAttr("name", name)) {
			if attr(n, "type") == "checkbox" && hasAttr(n, "checked") {
				applied = true
			}
		}
		if !applied {
			return out, fmt.Errorf("source did not apply requested %s exclusion; no quote accepted", name)
		}
	}
	sourceDate := text(first(doc, byClass("txt-date")))
	sourceTime := text(first(doc, byClass("txt-time")))
	if sourceDate != strings.ReplaceAll(o.At[:10], "-", "/") {
		return out, fmt.Errorf("source returned date%q instead of requested%s; no quote accepted", sourceDate, o.At)
	}
	if sourceTime != o.At[11:] {
		return out, fmt.Errorf("source returned time%q instead of requested%s; no quote accepted", sourceTime, o.At)
	}
	if kind := text(first(doc, byClass("txt-departure"))); !strings.EqualFold(kind, o.TimeKind) {
		return out, fmt.Errorf("source returned time kind %q instead of %s; no quote accepted", kind, o.TimeKind)
	}
	selectVehicle := first(doc, byAttr("name", "carType"))
	vehicleApplied := false
	selectedLabel := ""
	for _, n := range all(selectVehicle, tag("option")) {
		if hasAttr(n, "selected") {
			selectedLabel = text(n)
			vehicleApplied = attr(n, "value") == vehicles[o.Vehicle]
			break
		}
	}
	if !vehicleApplied || selectedLabel == "" || text(first(doc, byClass("txt-type"))) != selectedLabel {
		return out, fmt.Errorf("source vehicle category does not confirm requested %s; no quote accepted", o.Vehicle)
	}
	for i := range routes {
		switch o.Payment {
		case "etc":
			routes[i].SelectedJPY = routes[i].ETCJPY
		case "etc2":
			routes[i].SelectedJPY = routes[i].ETC2JPY
		default:
			routes[i].SelectedJPY = routes[i].StandardJPY
		}
	}
	if o.Via == nil {
		o.Via = []string{}
	}
	quote := RouteQuote{Assumptions: o, Alternatives: routes, URL: c.EN + "/dp/SearchQuickEN?" + q.Encode(), TimingBasis: "Provider ignoring/considering traffic estimates for the requested JST schedule; considering traffic is not guaranteed live-now travel time.", TollBasis: "JPY source estimates. ETC/ETC2.0 depend on actual card/device eligibility, route and timing; no local discount formula or guaranteed final charge."}
	return Output[RouteQuote]{Meta: c.meta(), Results: quote}, nil
}

type Notice struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	PublishedAt  *string `json:"published_at"`
	PublishedRaw string  `json:"published_raw"`
	URL          string  `json:"url"`
	ActiveStatus *bool   `json:"active_restriction"`
}
type NoticePage struct {
	Page[Notice]
	FeedUpdated *string `json:"feed_updated_at"`
	Coverage    string  `json:"coverage"`
}

func (c *Client) Notices(ctx context.Context, query, since string, limit, offset int) (Output[NoticePage], error) {
	var out Output[NoticePage]
	if err := ValidPage(limit, offset); err != nil {
		return out, err
	}
	var minDate time.Time
	var err error
	if since != "" {
		minDate, err = time.ParseInLocation("2006-01-02", since, jst)
		if err != nil {
			return out, fmt.Errorf("--since must be a real YYYY-MM-DD JST date")
		}
	}
	b, err := c.get(ctx, c.JP, "/cms/news/traffic.xml", nil)
	if err != nil {
		return out, err
	}
	items, updated, err := parseNotices(b)
	if err != nil {
		return out, err
	}
	filtered := []Notice{}
	for _, n := range items {
		if !contains(n.Title, query) {
			continue
		}
		if since != "" {
			if n.PublishedAt == nil {
				continue
			}
			t, e := time.Parse(time.RFC3339, *n.PublishedAt)
			if e != nil || t.Before(minDate) {
				continue
			}
		}
		filtered = append(filtered, n)
	}
	return Output[NoticePage]{Meta: c.meta(), Results: NoticePage{Page: page(filtered, limit, offset, len(items)), FeedUpdated: updated, Coverage: "Official Drive Plaza traffic advisory feed. Titles include releases, postponements and plans; active restriction status is unknown. Feed is not a comprehensive live-closure inventory."}}, nil
}

type Handoff struct {
	Purpose  string `json:"purpose"`
	URL      string `json:"url"`
	Coverage string `json:"coverage"`
}

// Handoffs are observed canonical destinations and perform no navigation.
func Handoffs() []Handoff {
	return []Handoff{{"route_planner", English + "/dp/SearchTopEN", "Source route/toll estimates"}, {"rest_stop_search", English + "/dp/SAPAServiceEN", "Source directional SA/PA search"}, {"live_traffic", "https://en-www.drivetraffic.jp/map.html", "Check current information in the official map; this CLI does not extract live status"}, {"planned_restrictions", Japanese + "/traffic/roadinfo/schedule/", "Official operator links"}, {"east_construction", "https://www.drivetraffic.jp/construction-regulation/", "NEXCO East scheduled construction restrictions"}, {"east_etc_lanes", "https://www.drivetraffic.jp/lane/", "NEXCO East planned ETC-lane restrictions"}, {"central_construction", "https://www.c-nexco.co.jp/traffic/construction/", "NEXCO Central handoff"}, {"west_construction", "https://www.w-nexco.co.jp/traffic_info/construction/", "NEXCO West handoff"}}
}

func contains(haystack, query string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(strings.TrimSpace(query)))
}
