// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// Package jbo reads anonymous Japan Bus Online pages. Sessions are memory-only.
package jbo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-bus-online/internal/cliutil"
	"golang.org/x/net/html"
)

const Origin = "https://japanbusonline.com"
const maxBody = 6 << 20

// MaintenanceError distinguishes a temporary provider outage from a parser or
// inventory result. The provider may serve this notice with HTTP 200.
type MaintenanceError struct {
	StartJST string
	EndJST   string
}

func (e *MaintenanceError) Error() string {
	return fmt.Sprintf("Japan Bus Online scheduled maintenance from %s to %s; retry after the published maintenance window", e.StartJST, e.EndJST)
}

var JST = time.FixedZone("JST", 9*3600)
var digits = regexp.MustCompile(`^[0-9]+$`)
var detailPath = regexp.MustCompile(`^/(en|zh-tw|zh-cn|ko)/Detail/([0-9]+)/([01])/([0-9]+)/([0-9]+)/?`)
var priceRE = regexp.MustCompile(`JPY\s*([0-9,]+)`)
var dateRE = regexp.MustCompile(`\b([0-9]{1,2}/[0-9]{1,2}/[0-9]{4})\b`)
var timeRE = regexp.MustCompile(`([0-9]{1,2}):([0-9]{2})`)

type Client struct {
	HTTP           *http.Client
	Base, Language string
	Requests       int
	Bytes          int64
	Started        time.Time
	limiter        *cliutil.AdaptiveLimiter
}

func New(language string, rateLimits ...float64) (*Client, error) {
	if language != "en" {
		return nil, errors.New("only the English (en) public surface is verified")
	}
	if len(rateLimits) > 0 && (math.IsNaN(rateLimits[0]) || math.IsInf(rateLimits[0], 0)) {
		return nil, errors.New("rate-limit must be finite; use -1 for auto, 0 to disable, or a positive requests-per-second ceiling")
	}
	jar, _ := cookiejar.New(nil)
	limiter := cliutil.NewAdaptiveLimiterAuto(2)
	if len(rateLimits) > 0 && rateLimits[0] >= 0 {
		limiter = cliutil.NewAdaptiveLimiter(rateLimits[0])
	}
	return &Client{HTTP: &http.Client{Jar: jar, Timeout: 25 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		if req.URL.Scheme != "https" || req.URL.Hostname() != "japanbusonline.com" && req.URL.Hostname() != "www.japanbusonline.com" {
			return errors.New("unexpected source redirect")
		}
		return nil
	}}, Base: Origin, Language: language, Started: time.Now(), limiter: limiter}, nil
}
func (c *Client) Get(ctx context.Context, path string) (*html.Node, string, error) {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return nil, "", errors.New("invalid source path")
	}
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Base+path, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "JapanBusOnlineReadOnlyCLI/1.0")
	req.Header.Set("Accept", "text/html")
	if strings.Contains(path, "/DetailAjax/") {
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
	}
	c.Requests++
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("source network error: %w", err)
	}
	defer resp.Body.Close()
	if remaining, resetAt, ok := cliutil.ParseRateLimitHeaders(resp.Header); ok {
		c.limiter.ObserveHeaders(remaining, resetAt)
	}
	if resp.StatusCode == 429 {
		c.limiter.OnRateLimit()
		return nil, "", &cliutil.RateLimitError{URL: c.Base + path, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("Japan Bus Online HTTP %d at %s", resp.StatusCode, path)
	}
	c.limiter.OnSuccess()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	c.Bytes += int64(len(b))
	if err != nil {
		return nil, "", err
	}
	if len(b) > maxBody {
		return nil, "", errors.New("source response exceeded 6MiB bound")
	}
	if len(b) == 0 {
		return nil, "", errors.New("source returned empty HTML; public session may have expired; retry command")
	}
	doc, err := html.Parse(strings.NewReader(string(b)))
	if err != nil {
		return nil, "", err
	}
	if m := regexp.MustCompile(`(?i)website is currently under maintenance from\s*([0-9/]+ [0-9:]+)\s*to\s*([0-9/]+ [0-9:]+)`).FindStringSubmatch(text(doc)); len(m) > 0 {
		start, end := m[1]+" JST", m[2]+" JST"
		if t, e := time.ParseInLocation("01/02/2006 15:04", m[1], JST); e == nil {
			start = t.Format(time.RFC3339)
		}
		if t, e := time.ParseInLocation("01/02/2006 15:04", m[2], JST); e == nil {
			end = t.Format(time.RFC3339)
		}
		return nil, "", &MaintenanceError{StartJST: start, EndJST: end}
	}
	return doc, string(b), nil
}
func (c *Client) Metadata(source string) map[string]any {
	return map[string]any{"source_url": c.Base + source, "fetched_at": time.Now().UTC().Format(time.RFC3339), "timezone": "Asia/Tokyo", "language": c.Language, "upstream_requests": c.Requests, "response_bytes": c.Bytes, "elapsed_ms": time.Since(c.Started).Milliseconds()}
}
func walk(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for x := n.FirstChild; x != nil; x = x.NextSibling {
		walk(x, fn)
	}
}
func attr(n *html.Node, k string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, k) {
			return a.Val
		}
	}
	return ""
}
func has(n *html.Node, k string) bool {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, k) {
			return true
		}
	}
	return false
}
func cls(n *html.Node, k string) bool {
	for _, x := range strings.Fields(attr(n, "class")) {
		if x == k {
			return true
		}
	}
	return false
}
func all(n *html.Node, p func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	walk(n, func(x *html.Node) {
		if p(x) {
			out = append(out, x)
		}
	})
	return out
}
func text(n *html.Node) string {
	var b strings.Builder
	var f func(*html.Node)
	f = func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
			b.WriteByte(' ')
		}
		if x.Data == "script" || x.Data == "style" {
			return
		}
		for y := x.FirstChild; y != nil; y = y.NextSibling {
			f(y)
		}
	}
	f(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
func byID(n *html.Node, id string) *html.Node {
	a := all(n, func(x *html.Node) bool { return attr(x, "id") == id })
	if len(a) == 0 {
		return nil
	}
	return a[0]
}
func intVal(s string) int { v, _ := strconv.Atoi(strings.ReplaceAll(s, ",", "")); return v }
func prices(s string) []int {
	out := []int{}
	for _, v := range priceRE.FindAllStringSubmatch(s, -1) {
		out = append(out, intVal(v[1]))
	}
	return out
}
func nameJA(s string) any {
	if regexp.MustCompile(`[ぁ-んァ-ン一-龯]`).MatchString(s) {
		return s
	}
	return nil
}
func canonical(path string) string { return Origin + path }
func ValidID(id string) error {
	if !digits.MatchString(id) || len(id) > 20 {
		return errors.New("route ID must contain 1-20 digits")
	}
	return nil
}
func ParseDate(s string) (time.Time, error) {
	t, e := time.ParseInLocation("2006-01-02", s, JST)
	if e != nil || t.Format("2006-01-02") != s {
		return t, errors.New("date must be a valid YYYY-MM-DD JST service day")
	}
	return t, nil
}
func DateISO(s string) string {
	for _, fmtstr := range []string{"20060102", "1/2/2006"} {
		if t, e := time.ParseInLocation(fmtstr, s, JST); e == nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}

// Source 24+ notation is relative to service day. Lower clocks use an explicit date.
func Timestamp(day, clock string) (string, error) {
	d, e := ParseDate(day)
	if e != nil {
		return "", e
	}
	m := timeRE.FindStringSubmatch(clock)
	if len(m) == 0 {
		return "", fmt.Errorf("invalid source time %q", clock)
	}
	h, min := intVal(m[1]), intVal(m[2])
	if h > 71 || min > 59 {
		return "", errors.New("invalid source clock")
	}
	return d.Add(time.Duration(h*60+min) * time.Minute).Format(time.RFC3339), nil
}
func Clock(raw string) string {
	if len(raw) == 4 && digits.MatchString(raw) {
		return raw[:2] + ":" + raw[2:]
	}
	return raw
}

type Route struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	NameJA any    `json:"name_ja"`
	URL    string `json:"url"`
}

func ParseRoutes(doc *html.Node, lang string) ([]Route, error) {
	var out []Route
	seen := map[string]bool{}
	re := regexp.MustCompile(`^/` + regexp.QuoteMeta(lang) + `/CourseSearch/([0-9]+)(?:\?|$)`)
	for _, a := range all(doc, func(n *html.Node) bool { return n.Data == "a" }) {
		m := re.FindStringSubmatch(attr(a, "href"))
		if len(m) == 0 || seen[m[1]] {
			continue
		}
		name := text(a)
		if name == "" {
			continue
		}
		seen[m[1]] = true
		out = append(out, Route{m[1], name, nameJA(name), canonical("/" + lang + "/CourseSearch/" + m[1])})
	}
	if len(out) == 0 {
		return nil, errors.New("route catalog shape changed or access page returned; inspect canonical source")
	}
	return out, nil
}
func (c *Client) Routes(ctx context.Context, query string, offset, limit int) (map[string]any, error) {
	path := "/" + c.Language + "/AllRouteList"
	doc, _, e := c.Get(ctx, path)
	if e != nil {
		return nil, e
	}
	rs, e := ParseRoutes(doc, c.Language)
	if e != nil {
		return nil, e
	}
	filtered := []Route{}
	for _, r := range rs {
		if strings.Contains(strings.ToLower(r.Name+" "+r.ID), strings.ToLower(query)) {
			filtered = append(filtered, r)
		}
	}
	total := len(filtered)
	start := min(offset, total)
	end := min(start+limit, total)
	out := c.Metadata(path)
	out["query"] = query
	out["total"] = total
	out["offset"] = offset
	out["limit"] = limit
	out["has_more"] = end < total
	out["routes"] = filtered[start:end]
	return out, nil
}

type StopSchedule struct {
	ScheduleRow int      `json:"schedule_row"`
	Name        string   `json:"name"`
	NameJA      any      `json:"name_ja"`
	MapURL      string   `json:"map_url,omitempty"`
	Times       []string `json:"source_times"`
	DayOffsets  []any    `json:"day_offsets"`
}
type Direction struct {
	Direction       int            `json:"direction"`
	DepArea         string         `json:"departure_area_id"`
	ArrArea         string         `json:"arrival_area_id"`
	Title           string         `json:"title"`
	URL             string         `json:"booking_url"`
	AdvertisedFares []int          `json:"advertised_fares_jpy"`
	Stops           []StopSchedule `json:"schedule_stops"`
}

func ParseDirections(doc *html.Node, route, lang string) ([]Direction, error) {
	out := []Direction{}
	seen := map[int]bool{}
	for _, a := range all(doc, func(n *html.Node) bool { return n.Data == "a" }) {
		m := detailPath.FindStringSubmatch(attr(a, "href"))
		if len(m) == 0 || m[2] != route {
			continue
		}
		dir := intVal(m[3])
		if seen[dir] {
			continue
		}
		box := a
		for box.Parent != nil && attr(box, "id") != route+m[3] {
			box = box.Parent
		}
		if attr(box, "id") != route+m[3] {
			continue
		}
		seen[dir] = true
		d := Direction{Direction: dir, DepArea: m[4], ArrArea: m[5], URL: canonical("/" + lang + "/Detail/" + route + "/" + m[3] + "/" + m[4] + "/" + m[5] + "/"), Stops: []StopSchedule{}}
		head := all(box, func(n *html.Node) bool { return n.Data == "h1" })
		if len(head) > 0 {
			d.Title = text(head[0])
		}
		strong := all(box, func(n *html.Node) bool { return cls(n, "text_strong") })
		for _, x := range strong {
			if p := prices(text(x)); len(p) > 0 {
				d.AdvertisedFares = p
			}
		}
		tables := all(box, func(n *html.Node) bool { return n.Data == "table" && cls(n, "time_table") })
		for _, t := range tables {
			prev := []int{}
			for i, row := range all(t, func(n *html.Node) bool { return n.Data == "tr" }) {
				ths := all(row, func(n *html.Node) bool { return n.Data == "th" })
				if len(ths) == 0 {
					continue
				}
				spans := all(ths[0], func(n *html.Node) bool { return n.Data == "span" && cls(n, "text_blk12") })
				nm := text(ths[0])
				if len(spans) > 0 {
					nm = text(spans[0])
				}
				st := StopSchedule{ScheduleRow: i + 1, Name: nm, NameJA: nameJA(nm), Times: []string{}, DayOffsets: []any{}}
				maps := all(ths[0], func(n *html.Node) bool { return n.Data == "a" })
				if len(maps) > 0 {
					st.MapURL = attr(maps[0], "href")
				}
				for j, td := range all(row, func(n *html.Node) bool { return n.Data == "td" }) {
					raw := text(td)
					st.Times = append(st.Times, raw)
					m := timeRE.FindStringSubmatch(raw)
					if len(m) == 0 {
						st.DayOffsets = append(st.DayOffsets, nil)
						continue
					}
					minute := intVal(m[1])*60 + intVal(m[2])
					for len(prev) <= j {
						prev = append(prev, -1)
					}
					for minute < prev[j] {
						minute += 1440
					}
					prev[j] = minute
					st.DayOffsets = append(st.DayOffsets, minute/1440)
				}
				d.Stops = append(d.Stops, st)
			}
		}
		out = append(out, d)
	}
	if len(out) == 0 {
		return nil, errors.New("route ID not found or direction layout changed")
	}
	return out, nil
}
func (c *Client) Route(ctx context.Context, route string) (map[string]any, []Direction, error) {
	if e := ValidID(route); e != nil {
		return nil, nil, e
	}
	path := "/" + c.Language + "/CourseSearch/" + route
	doc, _, e := c.Get(ctx, path)
	if e != nil {
		return nil, nil, e
	}
	ds, e := ParseDirections(doc, route, c.Language)
	if e != nil {
		return nil, nil, e
	}
	out := c.Metadata(path)
	out["route_id"] = route
	out["kind"] = "published_schedule_not_inventory"
	out["currency"] = "JPY"
	out["stop_identity_note"] = "schedule_row is a display row; quote returns source stop IDs scoped to route/direction/service"
	out["directions"] = ds
	return out, ds, nil
}

type Availability struct {
	Status     string `json:"status"`
	Raw        string `json:"source_text"`
	Exact      any    `json:"exact_seats"`
	LowerBound any    `json:"seats_lower_bound"`
}

func AvailabilityOf(raw string) Availability {
	a := Availability{Status: "unknown", Raw: raw}
	s := strings.ToLower(raw)
	if strings.Contains(s, "sold out") {
		a.Status = "sold_out"
		a.Exact = 0
		return a
	}
	if strings.Contains(s, "not available") {
		a.Status = "unavailable_unknown_reason"
		return a
	}
	if strings.Contains(s, "not on sale") {
		a.Status = "not_on_sale"
		return a
	}
	if strings.Contains(s, "more than 5") {
		a.Status = "available"
		a.LowerBound = 6
		return a
	}
	r := regexp.MustCompile(`([0-9]+)\s+seats?\s+left`).FindStringSubmatch(s)
	if len(r) > 0 {
		n, err := strconv.Atoi(r[1])
		if err != nil {
			return a
		}
		if n > 0 {
			a.LowerBound = n
			a.Status = "available"
		} else {
			a.Exact = 0
			a.Status = "sold_out"
		}
	}
	return a
}

type Service struct {
	ID           string       `json:"service_id"`
	Route        string       `json:"route_id"`
	Direction    int          `json:"direction"`
	DepDate      string       `json:"departure_date"`
	ArrDate      string       `json:"arrival_date"`
	DepTime      string       `json:"departure_source_time"`
	ArrTime      string       `json:"arrival_source_time"`
	Departure    string       `json:"departure_jst"`
	Arrival      string       `json:"arrival_jst"`
	FirstStop    string       `json:"first_stop"`
	LastStop     string       `json:"last_stop"`
	Availability Availability `json:"availability"`
	FromFare     any          `json:"from_fare_jpy"`
}

func ParseServices(doc *html.Node) ([]Service, string, []string, error) {
	out := []Service{}
	seen := map[string]bool{}
	effective := ""
	if n := byID(doc, "SelectDate"); n != nil {
		effective = DateISO(attr(n, "value"))
	}
	window := []string{}
	for _, n := range all(doc, func(n *html.Node) bool { return cls(n, "text_strong") }) {
		// Only the provider's labeled bookable-days element establishes this
		// window. Other highlighted notices may contain unrelated dates.
		raw := text(n)
		if !strings.Contains(strings.ToLower(raw), "bookable days of operation") {
			continue
		}
		dates := dateRE.FindAllString(raw, -1)
		if len(dates) != 2 {
			continue
		}
		start, end := DateISO(dates[0]), DateISO(dates[1])
		if start != "" && end != "" && start <= end {
			window = []string{start, end}
			break
		}
	}
	for _, n := range all(doc, func(n *html.Node) bool { return attr(n, "data-route") != "" && attr(n, "data-depdate") != "" }) {
		s := Service{ID: attr(n, "data-route"), Route: attr(n, "data-coursecd"), Direction: intVal(attr(n, "data-updownflg")), DepDate: DateISO(attr(n, "data-depdate")), ArrDate: DateISO(attr(n, "data-arrdate")), DepTime: Clock(attr(n, "data-deptime")), ArrTime: Clock(attr(n, "data-arrtime"))}
		key := s.ID + s.DepDate + s.DepTime
		if seen[key] {
			continue
		}
		seen[key] = true
		var e error
		s.Departure, e = Timestamp(s.DepDate, s.DepTime)
		if e != nil {
			return nil, "", nil, e
		}
		s.Arrival, e = Timestamp(s.ArrDate, s.ArrTime)
		if e != nil {
			return nil, "", nil, e
		}
		nm := []string{}
		raw := text(n)
		for _, col := range all(n, func(x *html.Node) bool { return cls(x, "text_mobile") }) {
			v := text(col)
			if v == "" || timeRE.MatchString(v) || v == "To" || v == "→" || strings.Contains(v, "JPY") {
				continue
			}
			if strings.Contains(strings.ToLower(v), "seat") || strings.Contains(strings.ToLower(v), "available") || strings.Contains(strings.ToLower(v), "sold out") || strings.Contains(strings.ToLower(v), "sale") {
				s.Availability = AvailabilityOf(v)
				continue
			}
			nm = append(nm, v)
		}
		if len(nm) > 0 {
			s.FirstStop = nm[0]
		}
		if len(nm) > 1 {
			s.LastStop = nm[1]
		}
		if s.Availability.Status == "" {
			s.Availability = AvailabilityOf(raw)
		}
		p := prices(raw)
		if len(p) > 0 {
			s.FromFare = p[0]
		}
		out = append(out, s)
	}
	if effective == "" {
		return nil, "", nil, errors.New("dated inventory layout changed: no effective source date")
	}
	return out, effective, window, nil
}
func (c *Client) Services(ctx context.Context, route string, dir int, date string) (map[string]any, []Service, string, error) {
	t, e := ParseDate(date)
	if e != nil {
		return nil, nil, "", e
	}
	_, ds, e := c.Route(ctx, route)
	if e != nil {
		return nil, nil, "", e
	}
	var d *Direction
	for i := range ds {
		if ds[i].Direction == dir {
			d = &ds[i]
		}
	}
	if d == nil {
		return nil, nil, "", errors.New("direction not offered for this route")
	}
	path := "/" + c.Language + "/Detail/" + route + "/" + strconv.Itoa(dir) + "/" + d.DepArea + "/" + d.ArrArea + "/" + t.Format("20060102")
	doc, _, e := c.Get(ctx, path)
	if e != nil {
		return nil, nil, "", e
	}
	sv, effective, window, e := ParseServices(doc)
	if e != nil {
		return nil, nil, "", e
	}
	out := c.Metadata(path)
	out["route_id"] = route
	out["direction"] = dir
	out["requested_date"] = date
	out["effective_source_date"] = effective
	out["sales_window"] = window
	out["status"] = "inventory_reported"
	out["fare_basis"] = "one-way headline from fare, not a stop-pair or party quote"
	out["booking_url"] = canonical(path)
	if len(sv) == 0 {
		out["status"] = "no_services_reported"
	}
	if effective != date {
		out["status"] = "source_date_substituted"
		out["warning"] = "Source selected a different date; no inventory returned for requested day"
		sv = []Service{}
	}
	if len(window) >= 2 && (date < window[0] || date > window[1]) {
		out["status"] = "not_on_sale_or_outside_window"
		sv = []Service{}
	}
	if effective == date {
		for _, s := range sv {
			if s.Route != route || s.Direction != dir || s.DepDate != date {
				return nil, nil, "", fmt.Errorf("source service %s identity mismatch: requested route %s direction %d date %s, received route %s direction %d date %s; inspect canonical source", s.ID, route, dir, date, s.Route, s.Direction, s.DepDate)
			}
		}
	}
	out["services"] = sv
	return out, sv, path, nil
}
