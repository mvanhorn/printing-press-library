package repark

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mvanhorn/printing-press-library/library/travel/repark/internal/cliutil"
	"golang.org/x/net/html"
)

const MaxBodyBytes = 2 * 1024 * 1024

var ErrNotFound = errors.New("parking source did not find the requested lot")
var ErrCalculatorUnavailable = errors.New("source calculator unavailable for this lot")

type Client struct {
	http        *http.Client
	limiter     *cliutil.AdaptiveLimiter
	origin      string
	requests    int
	maxRequests int
	pages       []string
	now         func() time.Time
}

// New creates a bounded client. It makes no request or filesystem changes.
func New(timeout time.Duration, rate float64, maxRequests int) *Client {
	if timeout <= 0 || timeout > 20*time.Second {
		timeout = 20 * time.Second
	}
	if rate < 0 {
		rate = 2
	}
	c := &Client{origin: Origin, limiter: cliutil.NewAdaptiveLimiter(rate), maxRequests: maxRequests, now: time.Now, pages: []string{}}
	c.http = &http.Client{Timeout: timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 2 {
			return fmt.Errorf("source redirect limit exceeded")
		}
		u, _ := url.Parse(c.origin)
		if req.URL.Host != u.Host || req.URL.Scheme != u.Scheme || !allowedPath(req.URL.Path) {
			return fmt.Errorf("source redirect left supported public Repark interfaces")
		}
		return c.beforeRequest(req.Context(), req.URL.String())
	}}
	return c
}

func allowedPath(p string) bool {
	return p == "/parking_user/time/freeword/" || p == "/parking_user/time/map.html" || p == "/parking_user/time/result/detail/" || p == "/parking_user/time/result/calculation/" || p == "/ajax/time_markers.json"
}
func (c *Client) beforeRequest(ctx context.Context, u string) error {
	if c.requests >= c.maxRequests {
		return fmt.Errorf("source request budget (%d) exhausted", c.maxRequests)
	}
	if err := c.limiter.Wait(ctx); err != nil {
		return err
	}
	c.requests++
	c.pages = append(c.pages, u)
	return nil
}
func (c *Client) fetch(ctx context.Context, path string, form url.Values) ([]byte, string, error) {
	u, err := url.Parse(c.origin + path)
	if err != nil || !allowedPath(u.Path) {
		return nil, "", fmt.Errorf("unsupported source interface")
	}
	if err = c.beforeRequest(ctx, u.String()); err != nil {
		return nil, "", err
	}
	method := http.MethodGet
	var body io.Reader
	if form != nil {
		method = http.MethodPost
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "repark-pp-cli/0.1.0 (read-only public parking lookup)")
	req.Header.Set("Accept", "application/json,text/html")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("Repark request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return nil, "", &cliutil.RateLimitError{URL: u.String(), RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, "", ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("Repark returned HTTP %d for %s", resp.StatusCode, u.Path)
	}
	if resp.ContentLength > MaxBodyBytes {
		return nil, "", fmt.Errorf("source response exceeds %d bytes; reduce --radius", MaxBodyBytes)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxBodyBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > MaxBodyBytes {
		return nil, "", fmt.Errorf("source response exceeds %d bytes; reduce --radius", MaxBodyBytes)
	}
	c.limiter.OnSuccess()
	return data, resp.Request.URL.String(), nil
}

type Meta struct {
	Source        string   `json:"source"`
	ObservedAt    string   `json:"observed_at"`
	Requests      int      `json:"requests"`
	RequestBudget int      `json:"request_budget"`
	SourceURLs    []string `json:"source_urls"`
	Coverage      string   `json:"coverage"`
	Notes         []string `json:"notes"`
}

func (c *Client) Meta() Meta {
	return Meta{Source: "live", ObservedAt: stamp(c.now()), Requests: c.requests, RequestBudget: c.maxRequests, SourceURLs: c.pages, Coverage: "provider coverage unverified", Notes: []string{
		"Occupancy includes compact and size-restricted four-wheel bays and excludes motorcycles; available does not establish fit for the remaining bays.",
		"Source import_date and updated_at are preserved raw; their timezone and relation to occupancy measurement are unverified.",
		"Displayed amounts are preserved in JPY. tax_included is null unless the source explicitly states tax inclusion; no tax is added.",
	}}
}

func (c *Client) Detail(ctx context.Context, value string) (Lot, error) {
	id, err := CanonicalID(value)
	if err != nil {
		return Lot{}, err
	}
	b, _, err := c.fetch(ctx, "/parking_user/time/result/detail/?park="+url.QueryEscape(id), nil)
	if err != nil {
		return Lot{}, err
	}
	return parseDetail(b, id, c.now())
}

type Options struct {
	RadiusM          int
	Limit            int
	MaxScanRecords   int
	AvailableOnly    bool
	WithinLimitsOnly bool
	Vehicle          Vehicle
	ExcludeID        string
}

func ValidateOptions(o Options) error {
	if o.RadiusM < 50 || o.RadiusM > 2000 {
		return fmt.Errorf("--radius must be 50..2000 metres")
	}
	if o.Limit < 1 || o.Limit > 50 {
		return fmt.Errorf("--limit must be 1..50 lots")
	}
	if o.MaxScanRecords < 1 || o.MaxScanRecords > 1000 {
		return fmt.Errorf("--max-scan-records must be 1..1000")
	}
	if err := ValidateVehicle(o.Vehicle); err != nil {
		return err
	}
	if o.WithinLimitsOnly && o.Vehicle.HeightM == nil && o.Vehicle.WidthM == nil && o.Vehicle.LengthM == nil && o.Vehicle.WeightT == nil {
		return fmt.Errorf("--within-limits-only requires at least one supplied vehicle dimension")
	}
	return nil
}

type Discovery struct {
	Anchor          *Coordinates `json:"anchor"`
	AnchorKind      string       `json:"anchor_kind"`
	Query           string       `json:"query,omitempty"`
	ResolvedPlace   string       `json:"source_resolved_place,omitempty"`
	Candidates      []Candidate  `json:"source_candidates,omitempty"`
	Status          string       `json:"status"`
	RadiusM         int          `json:"radius_m"`
	Results         []Lot        `json:"lots"`
	SourceRows      int          `json:"source_rows"`
	ScannedRecords  int          `json:"scanned_records"`
	MaxScanRecords  int          `json:"max_scan_records"`
	ScanTruncated   bool         `json:"scan_truncated"`
	OutputTruncated bool         `json:"output_truncated"`
	Note            string       `json:"note,omitempty"`
}
type Candidate struct {
	Name      string `json:"name"`
	SourceURL string `json:"source_url"`
}

func rangeValue(p Coordinates, radius int) string {
	dy := float64(radius) / 110500
	dx := float64(radius) / (110500 * math.Cos(p.Latitude*math.Pi/180))
	return fmt.Sprintf("C%.8f,%.8fN%.8fW%.8fS%.8fE%.8f", p.Latitude, p.Longitude, p.Latitude+dy, p.Longitude-dx, p.Latitude-dy, p.Longitude+dx)
}

func (c *Client) Nearby(ctx context.Context, p Coordinates, o Options) (Discovery, error) {
	if err := ValidateCoordinates(p); err != nil {
		return Discovery{}, err
	}
	if err := ValidateOptions(o); err != nil {
		return Discovery{}, err
	}
	q := url.Values{"range": {rangeValue(p, o.RadiusM)}}
	body, _, err := c.fetch(ctx, "/ajax/time_markers.json?"+q.Encode(), nil)
	if err != nil {
		return Discovery{}, err
	}
	lots, totalRows, err := parseMarkersBounded(body, c.now(), o.MaxScanRecords)
	if err != nil {
		return Discovery{}, err
	}
	v := Discovery{Anchor: &p, AnchorKind: "explicit_coordinates", Status: "ok", RadiusM: o.RadiusM, Results: []Lot{}, SourceRows: totalRows, MaxScanRecords: o.MaxScanRecords, ScanTruncated: totalRows > o.MaxScanRecords}
	seen := map[string]bool{}
	for i, l := range lots {
		if i >= o.MaxScanRecords {
			v.ScanTruncated = true
			break
		}
		v.ScannedRecords++
		if l.ID == o.ExcludeID || seen[l.ID] {
			continue
		}
		seen[l.ID] = true
		d := DistanceM(p, *l.Coordinates)
		if d > o.RadiusM {
			continue
		}
		l.DistanceM = &d
		l.Fit = AssessFit(l.Limits, o.Vehicle)
		if o.AvailableOnly && l.Occupancy.Category != "available" && l.Occupancy.Category != "crowded" {
			continue
		}
		if o.WithinLimitsOnly && l.Fit.Status != "within_supplied_published_limits" {
			continue
		}
		v.Results = append(v.Results, l)
	}
	sort.SliceStable(v.Results, func(i, j int) bool {
		if *v.Results[i].DistanceM == *v.Results[j].DistanceM {
			return v.Results[i].ID < v.Results[j].ID
		}
		return *v.Results[i].DistanceM < *v.Results[j].DistanceM
	})
	if len(v.Results) > o.Limit {
		v.OutputTruncated = true
		v.Results = v.Results[:o.Limit]
	}
	if len(v.Results) == 0 {
		v.Note = "No matching lots in this bounded source window. Refine the query, change filters, or increase --radius / --max-scan-records within their limits."
	}
	return v, nil
}

func (c *Client) Search(ctx context.Context, query string, o Options) (Discovery, error) {
	query = strings.TrimSpace(query)
	if err := ValidateQuery(query); err != nil {
		return Discovery{}, err
	}
	if err := ValidateOptions(o); err != nil {
		return Discovery{}, err
	}
	body, final, err := c.fetch(ctx, "/parking_user/time/freeword/?"+url.Values{"st": {"1"}, "word": {query}}.Encode(), nil)
	if err != nil {
		return Discovery{}, err
	}
	n, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return Discovery{}, err
	}
	u, _ := url.Parse(final)
	la := u.Query().Get("lat")
	lo := u.Query().Get("lon")
	if la == "" {
		la = input(n, "lat")
	}
	if lo == "" {
		lo = input(n, "lon")
	}
	lat := numeric(la)
	lon := numeric(lo)
	if lat == nil || lon == nil {
		v := Discovery{Query: query, AnchorKind: "source_unresolved", Status: "needs_refinement", RadiusM: o.RadiusM, Results: []Lot{}, MaxScanRecords: o.MaxScanRecords, Note: "The provider did not establish a unique coordinate anchor. Refine the Japanese place/address or use parking nearby with explicit coordinates."}
		seen := map[string]bool{}
		for _, a := range nodes(n, func(x *html.Node) bool { return x.Data == "a" }) {
			href := attr(a, "href")
			p, err := url.Parse(href)
			if err != nil {
				continue
			}
			if p.IsAbs() && (p.Scheme != "https" || p.Host != "www.repark.jp") {
				continue
			}
			if p.Path != "/parking_user/time/freeword/" && p.Path != "/parking_user/time/map.html" {
				continue
			}
			if len(p.RawQuery) == 0 || seen[href] {
				continue
			}
			name := clean(text(a))
			if name == "" {
				continue
			}
			seen[href] = true
			if !p.IsAbs() {
				href = Origin + href
			}
			v.Candidates = append(v.Candidates, Candidate{Name: name, SourceURL: href})
			if len(v.Candidates) >= 10 {
				break
			}
		}
		return v, nil
	}
	v, err := c.Nearby(ctx, Coordinates{*lat, *lon}, o)
	if err != nil {
		return v, err
	}
	v.Query = query
	v.AnchorKind = "source_resolved_named_place"
	v.ResolvedPlace = clean(input(n, "freeword"))
	if v.ResolvedPlace == "" {
		v.ResolvedPlace = u.Query().Get("plc")
	}
	return v, nil
}

func ValidateQuery(query string) error {
	query = strings.TrimSpace(query)
	if utf8.RuneCountInString(query) < 1 || utf8.RuneCountInString(query) > 72 || strings.ContainsAny(query, "\r\n\x00") {
		return fmt.Errorf("search query must contain 1..72 characters of a named place, station or address")
	}
	return nil
}

type Quote struct {
	ID                          string   `json:"id"`
	Name                        string   `json:"name"`
	Bay                         int      `json:"bay"`
	Start                       string   `json:"start_jst"`
	End                         string   `json:"end_jst"`
	DurationMinutes             int      `json:"duration_minutes"`
	AmountJPY                   int      `json:"estimated_amount_jpy"`
	TaxIncluded                 *bool    `json:"tax_included"`
	DiscountsIncluded           bool     `json:"discounts_included"`
	FinalBilledChargeGuaranteed bool     `json:"final_billed_charge_guaranteed"`
	SourceText                  string   `json:"source_result_text"`
	SourceCautions              []string `json:"source_cautions"`
	RateBasis                   string   `json:"rate_basis"`
	SourceURL                   string   `json:"source_url"`
	ObservedAt                  string   `json:"observed_at"`
}

func ValidateQuote(bay int, start, end, now time.Time) error {
	if bay < 1 || bay > 999 {
		return fmt.Errorf("--bay must be an explicit source bay number from 1..999; confirm the marked physical bay")
	}
	if !end.After(start) {
		return fmt.Errorf("--end must be later than --start (JST)")
	}
	if end.Sub(start) > 48*time.Hour {
		return fmt.Errorf("quote interval exceeds the provider's usual 48-hour stay limit; contact the lot using its source page")
	}
	last := now.In(JST).AddDate(0, 0, 365)
	last = time.Date(last.Year(), last.Month(), last.Day(), 23, 59, 0, 0, JST)
	if end.After(last) {
		return fmt.Errorf("--end is outside the provider calculator's 365-day date window")
	}
	return nil
}

func (c *Client) Quote(ctx context.Context, value string, bay int, start, end time.Time) (Quote, error) {
	if err := ValidateQuote(bay, start, end, c.now()); err != nil {
		return Quote{}, err
	}
	l, err := c.Detail(ctx, value)
	if err != nil {
		return Quote{}, err
	}
	if l.CalculatorURL == "" {
		return Quote{}, ErrCalculatorUnavailable
	}
	path := "/parking_user/time/result/calculation/?park=" + l.ID
	body, _, err := c.fetch(ctx, path, nil)
	if err != nil {
		return Quote{}, err
	}
	n, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return Quote{}, err
	}
	var form *html.Node
	for _, f := range nodes(n, func(x *html.Node) bool { return x.Data == "form" }) {
		if attr(f, "name") == "nowtime_calculation" {
			form = f
		}
	}
	if form == nil || input(form, "func") != "settime" {
		return Quote{}, ErrCalculatorUnavailable
	}
	pk := input(form, "pkid")
	actualID, _ := strconv.Atoi(strings.TrimPrefix(l.ID, "REP"))
	pkNum, _ := strconv.Atoi(pk)
	if pkNum != actualID {
		return Quote{}, fmt.Errorf("source calculator lot identity does not match %s", l.ID)
	}
	bayAllowed := false
	for _, s := range nodes(form, func(x *html.Node) bool { return x.Data == "select" && attr(x, "name") == "settime-pksno" }) {
		for _, o := range nodes(s, func(x *html.Node) bool { return x.Data == "option" }) {
			if attr(o, "value") == strconv.Itoa(bay) {
				bayAllowed = true
			}
		}
	}
	if !bayAllowed {
		return Quote{}, fmt.Errorf("source calculator does not offer bay %d", bay)
	}
	values := url.Values{"func": {"settime"}, "pkid": {pk}, "settime-pksno": {strconv.Itoa(bay)}}
	for key, t := range map[string]time.Time{"start": start.In(JST), "end": end.In(JST)} {
		values.Set("settime-"+key+"Date", t.Format("2006/01/02"))
		values.Set("settime-"+key+"Hour", t.Format("15"))
		values.Set("settime-"+key+"Minute", t.Format("04"))
	}
	body, _, err = c.fetch(ctx, path, values)
	if err != nil {
		return Quote{}, err
	}
	return parseQuote(body, l.ID, l.Name, bay, start.In(JST), end.In(JST), c.now())
}

func parseQuote(body []byte, id, name string, bay int, start, end, observed time.Time) (Quote, error) {
	n, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return Quote{}, err
	}
	amounts := nodes(n, func(x *html.Node) bool { return hasClass(x, "fs40") && hasClass(x, "orange") })
	if len(amounts) != 1 {
		return Quote{}, fmt.Errorf("source calculator returned no unambiguous estimate; confirm bay and source conditions")
	}
	a := number(clean(text(amounts[0])))
	if a == nil {
		return Quote{}, fmt.Errorf("source calculator amount was not a JPY integer")
	}
	parents := amounts[0]
	for parents.Parent != nil && parents.Data != "p" {
		parents = parents.Parent
	}
	source := clean(text(parents))
	jp := func(t time.Time) string {
		return fmt.Sprintf("%d年%d月%d日%d時%d分", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute())
	}
	if !strings.Contains(source, jp(start)+"から") || !strings.Contains(source, jp(end)+"まで") {
		return Quote{}, fmt.Errorf("source calculator echoed an interval different from the requested JST interval")
	}
	q := Quote{ID: id, Name: name, Bay: bay, Start: stamp(start), End: stamp(end), DurationMinutes: int(end.Sub(start) / time.Minute), AmountJPY: *a, SourceText: source, SourceCautions: []string{}, RateBasis: "current source rates; historical and future tariff changes unknown", SourceURL: Origin + "/parking_user/time/result/calculation/?park=" + id, ObservedAt: stamp(observed)}
	for _, p := range byClass(n, "result_attention") {
		for _, s := range nodes(p, func(x *html.Node) bool { return x.Data == "span" }) {
			if v := clean(text(s)); v != "" {
				q.SourceCautions = append(q.SourceCautions, v)
			}
		}
	}
	if len(q.SourceCautions) == 0 {
		return Quote{}, fmt.Errorf("source calculator response lacked its estimate conditions")
	}
	if strings.Contains(source, "税込") {
		q.TaxIncluded = boolp(true)
	}
	return q, nil
}
