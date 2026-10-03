// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// Package snowjapan projects public source facts without retaining report prose.
package snowjapan

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/snowjapan/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/snowjapan/internal/cliutil"
	"golang.org/x/net/html"
)

const MaxBody = 5 << 20
const MaxRecords = 1500

type Fact map[string]any

type Client struct {
	HTTP      *http.Client
	SiteBase  string
	ChartBase string
	limiter   *cliutil.AdaptiveLimiter
}

func New() *Client {
	return &Client{
		HTTP: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 4 {
				return fmt.Errorf("source redirect limit exceeded")
			}
			if !trustedHost(req.URL) {
				return fmt.Errorf("source redirect left trusted public hosts")
			}
			return nil
		}},
		SiteBase:  "https://www.snowjapan.com",
		ChartBase: "https://flo.uri.sh",
		limiter:   cliutil.NewAdaptiveLimiter(2),
	}
}

// NewWithRateLimit honors a slower operator rate and retains the source cap.
func NewWithRateLimit(rate float64) *Client {
	c := New()
	if rate > 0 && rate < 2 {
		c.limiter = cliutil.NewAdaptiveLimiter(rate)
	}
	return c
}

func trustedHost(u *url.URL) bool {
	if u.Scheme != "https" || u.User != nil {
		return false
	}
	switch u.Host {
	case "www.snowjapan.com", "snowjapan.com", "www.snowjp.com", "flo.uri.sh":
		return true
	}
	return false
}

func (c *Client) fetch(ctx context.Context, target string) ([]byte, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "snowjapan-pp-cli/0.1 (read-only factual planning)")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: target, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &client.APIError{Method: http.MethodGet, Path: req.URL.Path, StatusCode: resp.StatusCode, Body: "public SnowJapan source did not return a published record"}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxBody+1))
	if err != nil {
		return nil, err
	}
	if len(body) > MaxBody {
		return nil, fmt.Errorf("source body exceeds %d bytes", MaxBody)
	}
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "html") {
		return nil, fmt.Errorf("source changed content type; expected public HTML")
	}
	c.limiter.OnSuccess()
	return body, nil
}

func stamp() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func walk(n *html.Node, f func(*html.Node)) {
	f(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, f)
	}
}
func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func text(n *html.Node) string {
	var b strings.Builder
	walk(n, func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
			b.WriteByte(' ')
		}
	})
	return cliutil.CleanText(b.String())
}
func doc(body []byte) (*html.Node, error) { return html.Parse(bytes.NewReader(body)) }

func warmup(body []byte) (map[string]any, error) {
	d, err := doc(body)
	if err != nil {
		return nil, err
	}
	var raw string
	walk(d, func(n *html.Node) {
		if n.Data == "script" && attr(n, "id") == "wix-warmup-data" && n.FirstChild != nil {
			raw = n.FirstChild.Data
		}
	})
	if raw == "" {
		return nil, fmt.Errorf("source schema changed: wix-warmup-data is absent")
	}
	var out map[string]any
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("decoding source SSR: %w", err)
	}
	return out, nil
}

var chartID = regexp.MustCompile(`data-src="visualisation/([0-9]{1,12})"`)

func findCharts(v any, ids map[string]bool) {
	switch x := v.(type) {
	case map[string]any:
		for _, a := range x {
			findCharts(a, ids)
		}
	case []any:
		for _, a := range x {
			findCharts(a, ids)
		}
	case string:
		if strings.Contains(x, "flourish-embed") {
			for _, m := range chartID.FindAllStringSubmatch(x, -1) {
				ids[m[1]] = true
			}
		}
	}
}

func declaration(body []byte, name string, out any) error {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*=\s*`)
	at := re.FindIndex(body)
	if at == nil {
		return fmt.Errorf("source schema changed: %s declaration absent", name)
	}
	raw := body[at[1]:]
	if len(raw) == 0 || (raw[0] != '{' && raw[0] != '[') {
		return fmt.Errorf("source declaration %s is not a data container", name)
	}
	// Accept only literal new Date(integer) outside quoted JSON strings.
	// This is syntax normalization, never Javascript evaluation.
	var b bytes.Buffer
	inString, escaped := false, false
	depth := 0
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if inString {
			b.WriteByte(ch)
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			b.WriteByte(ch)
			continue
		}
		if bytes.HasPrefix(raw[i:], []byte("new Date(")) {
			start := i + 9
			j := start
			for j < len(raw) && raw[j] >= '0' && raw[j] <= '9' {
				j++
			}
			if j == start || j >= len(raw) || raw[j] != ')' || j-start > 15 {
				return fmt.Errorf("unsupported source date literal")
			}
			b.Write(raw[start:j])
			i = j
			continue
		}
		if ch == '{' || ch == '[' {
			depth++
		} else if ch == '}' || ch == ']' {
			depth--
		}
		b.WriteByte(ch)
		if depth == 0 {
			break // Only normalize this declaration; trailing scripts are unrelated.
		}
	}
	dec := json.NewDecoder(&b)
	dec.UseNumber()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("source declaration %s is not supported data: %w", name, err)
	}
	return nil
}

func (c *Client) chart(ctx context.Context, path string) ([]byte, error) {
	page, err := c.fetch(ctx, c.SiteBase+path)
	if err != nil {
		return nil, err
	}
	w, err := warmup(page)
	if err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	findCharts(w, ids)
	if len(ids) != 1 {
		return nil, fmt.Errorf("source chart schema changed: expected one published visualization, found %d", len(ids))
	}
	var id string
	for k := range ids {
		id = k
	}
	return c.fetch(ctx, c.ChartBase+"/visualisation/"+id+"/embed?auto=1")
}

func number(v any) (float64, bool) {
	switch x := v.(type) {
	case json.Number:
		f, e := x.Float64()
		return f, e == nil
	case float64:
		return x, true
	}
	return 0, false
}
func numeric(v any) any {
	if n, ok := number(v); ok {
		return n
	}
	return nil
}
func stringVal(v any) string { s, _ := v.(string); return cliutil.CleanText(s) }
func pathID(source string) (string, error) {
	u, e := url.Parse(source)
	if e != nil || !trustedHost(u) || u.Host == "flo.uri.sh" || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("source contains an untrusted resort URL")
	}
	id := strings.TrimPrefix(u.Path, "/ski-areas-in-japan/")
	if id == u.Path || !validResortID(id) {
		return "", fmt.Errorf("source contains an invalid canonical resort path")
	}
	return id, nil
}

var resortIDPattern = regexp.MustCompile(`^(?:[a-z0-9]+(?:-[a-z0-9]+)*/){2}[a-z0-9]+(?:-[a-z0-9]+)*$`)

func validResortID(id string) bool { return len(id) < 220 && resortIDPattern.MatchString(id) }
func canonical(id string) string   { return "https://www.snowjapan.com/ski-areas-in-japan/" + id }

func parseCatalog(body []byte) ([]Fact, error) {
	var columns map[string]map[string]any
	if e := declaration(body, "_Flourish_data_column_names", &columns); e != nil {
		return nil, e
	}
	want := []string{"City", "Prefecture", "Top (m)", "Base (m)", "Vertical (m)", "Lifts (#)", "Courses (#)", "Longest (m)", "Steepest (°)", "Link"}
	got, _ := columns["data"]["popup_metadata"].([]any)
	if len(got) != len(want) {
		return nil, fmt.Errorf("source catalog columns changed")
	}
	for i, s := range want {
		if got[i] != s {
			return nil, fmt.Errorf("source catalog column %d changed", i)
		}
	}
	var data map[string][]struct {
		Names  []string `json:"nest_columns"`
		Values []any    `json:"popup_metadata"`
	}
	if e := declaration(body, "_Flourish_data", &data); e != nil {
		return nil, e
	}
	rows := data["data"]
	if len(rows) == 0 || len(rows) > MaxRecords {
		return nil, fmt.Errorf("source catalog returned an invalid row count")
	}
	out := make([]Fact, 0, len(rows))
	seen := map[string]bool{}
	now := stamp()
	for _, r := range rows {
		if len(r.Names) != 1 || len(r.Values) != 10 {
			return nil, fmt.Errorf("source catalog row shape changed")
		}
		originalLink := stringVal(r.Values[9])
		sourceLink := originalLink
		// The source-owned chart's exact staging-host typo was verified against
		// SnowJapan's public Gunma directory on 2026-10-03. Never rewrite a
		// general foreign hostname or send a request to it.
		if sourceLink == "https://www.ushihadoko.com/ski-areas-in-japan/gunma-prefecture/tsumagoi-village/manza-onsen" {
			sourceLink = "https://www.snowjapan.com/ski-areas-in-japan/gunma-prefecture/tsumagoi-village/manza-onsen"
		}
		id, e := pathID(sourceLink)
		if e != nil {
			return nil, e
		}
		if seen[id] {
			return nil, fmt.Errorf("source catalog contains duplicate canonical id %s", id)
		}
		seen[id] = true
		f := Fact{"id": id, "name": stringVal(r.Names[0]), "town": stringVal(r.Values[0]), "prefecture": stringVal(r.Values[1]), "source_url": canonical(id), "observed_at": now, "projection": "catalog-v1", "lift_operation_status": "unknown", "directory_status": "listed_active_area"}
		f["catalog_observed_at"] = now
		if sourceLink != originalLink {
			f["original_chart_link"] = originalLink
			f["link_reconciled_from"] = "https://www.snowjapan.com/ski-areas-in-japan/gunma-prefecture"
		}
		keys := []string{"peak_m", "base_m", "vertical_m", "installed_lifts", "courses", "longest_course_m", "steepest_degrees"}
		for i, k := range keys {
			f[k] = numeric(r.Values[i+2])
		}
		if f["name"] == "" || f["town"] == "" || f["prefecture"] == "" {
			return nil, fmt.Errorf("source catalog identity missing")
		}
		out = append(out, f)
	}
	return out, nil
}

func (c *Client) Catalog(ctx context.Context) ([]Fact, error) {
	b, e := c.chart(ctx, "/insights/japan-ski-areas-statistics")
	if e != nil {
		return nil, e
	}
	return parseCatalog(b)
}

func records(body []byte) (map[string]any, error) {
	w, e := warmup(body)
	if e != nil {
		return nil, e
	}
	v := any(w)
	for _, k := range []string{"appsWarmupData", "dataBinding", "dataStore", "recordsByCollectionId", "Ski-Areas"} {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("source resort record schema changed")
		}
		v = m[k]
	}
	m, ok := v.(map[string]any)
	if !ok || len(m) == 0 || len(m) > MaxRecords {
		return nil, fmt.Errorf("source resort records missing")
	}
	return m, nil
}

func projectRecord(v map[string]any, id string) (Fact, error) {
	f := Fact{"id": id, "name": stringVal(v["title"]), "name_japanese": stringVal(v["nameJapanese"]), "town": stringVal(v["mapLocation"]), "prefecture": stringVal(v["prefecture"]), "popular_region": stringVal(v["popularRegion"]), "source_url": canonical(id), "location_url": canonical(id) + "/location", "observed_at": stamp(), "projection": "detail-v1", "lift_operation_status": "unknown", "planned_window": stringVal(v["plannedSeason"]), "information_status": stringVal(v["status"]), "upcoming_dates_confirmed": false}
	f["detail_observed_at"] = f["observed_at"]
	for out, in := range map[string]string{"peak_m": "maxElevation", "base_m": "minElevation", "vertical_m": "vertical", "installed_lifts": "lifts", "courses": "courses", "longest_course_m": "longestCourse", "steepest_degrees": "steepestCourse", "beginner_percent": "beginner", "intermediate_percent": "intermediate", "advanced_percent": "advanced", "latitude": "lat", "longitude": "long"} {
		f[out] = numeric(v[in])
	}
	lifts := Fact{}
	for out, in := range map[string]string{"ropeway": "ropeway", "gondola": "gondola", "quad": "quadLift", "triple": "triple", "pair": "pair", "single": "single", "surface_or_other": "other"} {
		lifts[out] = numeric(v[in])
	}
	f["installed_lift_types"] = lifts
	if addressDoc, e := doc([]byte(stringVal(v["fullAddress"]))); e == nil {
		f["address"] = text(addressDoc)
	}
	if strings.HasPrefix(stringVal(v["dailyReportLink"]), "/daily-snow-and-weather-reports/") {
		f["regional_report_url"] = "https://www.snowjapan.com" + stringVal(v["dailyReportLink"])
	}
	if d, ok := v["_updatedDate"].(map[string]any); ok {
		f["source_record_updated_at"] = stringVal(d["$date"])
	}
	if f["name"] == "" || f["peak_m"] == nil || f["installed_lifts"] == nil {
		return nil, fmt.Errorf("source resort factual fields changed")
	}
	for _, key := range []string{"beginner_percent", "intermediate_percent", "advanced_percent"} {
		if n, ok := number(f[key]); ok && (n < 0 || n > 100) {
			return nil, fmt.Errorf("source terrain percentage out of range")
		}
	}
	return f, nil
}

func (c *Client) Inspect(ctx context.Context, id string) (Fact, error) {
	if !validResortID(id) {
		return nil, fmt.Errorf("resort id must be an exact canonical path suffix from resorts search")
	}
	b, e := c.fetch(ctx, c.SiteBase+"/ski-areas-in-japan/"+id)
	if e != nil {
		return nil, e
	}
	m, e := records(b)
	if e != nil {
		return nil, e
	}
	for _, x := range m {
		r, ok := x.(map[string]any)
		if !ok {
			continue
		}
		recordID, err := detailRecordID(r)
		if err == nil && recordID == id {
			return projectRecord(r, id)
		}
	}
	return nil, fmt.Errorf("source detail did not identify the requested resort")
}

func detailRecordID(r map[string]any) (string, error) {
	var identity string
	for _, field := range []string{"link-copy-of-ski-areas-title-2", "link-copy-of-ski-areas-title"} {
		link := stringVal(r[field])
		if link == "" {
			continue
		}
		if strings.HasPrefix(link, "/") {
			link = "https://www.snowjapan.com" + link
		}
		// Wix's companion location link refers to the same area. Both
		// published identity links must agree when they are present.
		link = strings.TrimSuffix(link, "/location")
		candidate, err := pathID(link)
		if err != nil || (identity != "" && identity != candidate) {
			return "", fmt.Errorf("source detail contains a contradictory resort identity")
		}
		identity = candidate
	}
	if identity == "" {
		return "", fmt.Errorf("source detail contains no resort identity")
	}
	return identity, nil
}

var seasonPattern = regexp.MustCompile(`^([0-9]{4})-([0-9]{4})$`)

func ValidateSeason(season string) error {
	return validateSeasonAt(season, time.Now().UTC())
}

func validateSeasonAt(season string, now time.Time) error {
	m := seasonPattern.FindStringSubmatch(season)
	if m == nil {
		return fmt.Errorf("--season must be YYYY-YYYY")
	}
	a, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[2])
	latestStart := now.Year() - 1
	if now.Month() < time.September {
		latestStart--
	}
	if b != a+1 || a < 2021 || a > latestStart {
		return fmt.Errorf("--season requires a completed published winter; future seasons are unsupported")
	}
	return nil
}

func (c *Client) Seasons(ctx context.Context, season string) ([]Fact, error) {
	if e := ValidateSeason(season); e != nil {
		return nil, e
	}
	b, e := c.chart(ctx, "/insights/"+season+"-ski-season-dates-sort")
	if e != nil {
		return nil, e
	}
	var columns map[string]map[string][]string
	if e = declaration(b, "_Flourish_data_column_names", &columns); e != nil {
		return nil, e
	}
	want := []string{"Ski area", "Town", "Prefecture", "Open", "Close", "Days"}
	if strings.Join(columns["rows"]["columns"], "|") != strings.Join(want, "|") {
		return nil, fmt.Errorf("historical source columns changed")
	}
	var data map[string][]struct {
		Values []any `json:"columns"`
	}
	if e = declaration(b, "_Flourish_data", &data); e != nil {
		return nil, e
	}
	rows := data["rows"]
	if len(rows) == 0 || len(rows) > MaxRecords {
		return nil, fmt.Errorf("historical source row count invalid")
	}
	out := make([]Fact, 0, len(rows))
	seen := map[string]bool{}
	now := stamp()
	for _, r := range rows {
		if len(r.Values) != 6 {
			return nil, fmt.Errorf("historical row shape changed")
		}
		d, e := doc([]byte(stringVal(r.Values[0])))
		if e != nil {
			return nil, e
		}
		var source, name string
		walk(d, func(n *html.Node) {
			if n.Data == "a" && source == "" {
				source = attr(n, "href")
				name = text(n)
			}
		})
		id, e := pathID(source)
		if e != nil {
			return nil, e
		}
		unit := name
		// The source can publish conflicting rows with the same name/link
		// but different municipalities. Retain both as evidence; their
		// shared resort join remains ambiguous.
		identity := id + "\x00" + unit + "\x00" + stringVal(r.Values[1])
		if seen[identity] {
			return nil, fmt.Errorf("historical source has duplicate resort operating-unit label %s", unit)
		}
		seen[identity] = true
		a, ok := number(r.Values[3])
		if !ok {
			return nil, fmt.Errorf("historical opening timestamp missing")
		}
		z, ok := number(r.Values[4])
		if !ok {
			return nil, fmt.Errorf("historical closing timestamp missing")
		}
		first, last := time.UnixMilli(int64(a)).UTC(), time.UnixMilli(int64(z)).UTC()
		span, ok := number(r.Values[5])
		if !ok || first.After(last) || int(last.Sub(first).Hours()/24)+1 != int(span) {
			return nil, fmt.Errorf("historical source dates and span disagree")
		}
		year, _ := strconv.Atoi(season[:4])
		windowStart := time.Date(year, 9, 1, 0, 0, 0, 0, time.UTC)
		windowEnd := windowStart.AddDate(1, 0, 0)
		evidenceState := "recorded_span"
		if first.Before(windowStart) || !last.Before(windowEnd) {
			evidenceState = "dates_outside_requested_winter"
		}
		key := sha256.Sum256([]byte(identity))
		out = append(out, Fact{"id": season + ":" + id + ":" + fmt.Sprintf("%x", key[:8]), "resort_id": id, "name": name, "operating_unit_label": unit, "town": stringVal(r.Values[1]), "prefecture": stringVal(r.Values[2]), "season": season, "first_recorded_day": first.Format("2006-01-02"), "last_recorded_day": last.Format("2006-01-02"), "span_days": int(span), "endpoint_evidence_state": evidenceState, "continuous_operation": "unknown", "source_url": canonical(id), "chart_url": "https://www.snowjapan.com/insights/" + season + "-ski-season-dates-sort", "observed_at": now, "projection": "season-v1"})
	}
	return out, nil
}

var datedSlug = regexp.MustCompile(`^([a-z]+(?:-[a-z]+)*)-now-([0-9]{1,2})(?:st|nd|rd|th)-([a-z]+)-([0-9]{4})$`)
var months = map[string]time.Month{"january": 1, "february": 2, "march": 3, "april": 4, "may": 5, "june": 6, "july": 7, "august": 8, "september": 9, "october": 10, "november": 11, "december": 12}

func reportIdentity(slug string) (string, string, error) {
	m := datedSlug.FindStringSubmatch(slug)
	if m == nil {
		return "", "", fmt.Errorf("use an exact dated report slug from reports list")
	}
	day, _ := strconv.Atoi(m[2])
	year, _ := strconv.Atoi(m[4])
	month, ok := months[m[3]]
	t := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	if !ok || year < 2012 || year > time.Now().Year() || t.Month() != month || t.Day() != day {
		return "", "", fmt.Errorf("invalid report date")
	}
	return m[1], t.Format("2006-01-02"), nil
}

func (c *Client) Reports(ctx context.Context) ([]Fact, error) {
	b, e := c.fetch(ctx, c.SiteBase+"/")
	if e != nil {
		return nil, e
	}
	d, e := doc(b)
	if e != nil {
		return nil, e
	}
	out := make([]Fact, 0, 12)
	seen := map[string]bool{}
	now := stamp()
	walk(d, func(n *html.Node) {
		if n.Data != "a" {
			return
		}
		u, e := url.Parse(attr(n, "href"))
		if e != nil {
			return
		}
		if u.Host == "" {
			u, _ = url.Parse(c.SiteBase + u.Path)
		}
		if !trustedHost(u) || !strings.HasPrefix(u.Path, "/daily-snow-and-weather-reports/") {
			return
		}
		id := strings.TrimPrefix(u.Path, "/daily-snow-and-weather-reports/")
		region, date, e := reportIdentity(id)
		if e != nil || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, Fact{"id": id, "name": text(n), "region": region, "report_date": date, "source_url": "https://www.snowjapan.com" + u.Path, "observed_at": now, "projection": "report-metadata-v1", "coverage": "named_region_reporter_base_or_town", "lift_operation_status": "unknown"})
	})
	if len(out) == 0 || len(out) > 30 {
		return nil, fmt.Errorf("source latest report metadata schema changed")
	}
	return out, nil
}

var snowFigure = regexp.MustCompile(`(?i)^(New snowfall at base|Snowfall at base this season)\s+([0-9]+(?:\.[0-9]+)?)\s*cm$`)

func (c *Client) Report(ctx context.Context, id string) (Fact, error) {
	region, date, e := reportIdentity(id)
	if e != nil {
		return nil, e
	}
	b, e := c.fetch(ctx, c.SiteBase+"/daily-snow-and-weather-reports/"+id)
	if e != nil {
		return nil, e
	}
	d, e := doc(b)
	if e != nil {
		return nil, e
	}
	f := Fact{"id": id, "region": region, "report_date": date, "source_url": "https://www.snowjapan.com/daily-snow-and-weather-reports/" + id, "observed_at": stamp(), "projection": "report-observations-v1", "new_snow_cm": nil, "season_snowfall_cm": nil, "measurement_location": "reporter_base_or_town", "new_snow_period": "since_previous_report", "lift_operation_status": "unknown"}
	var heading string
	walk(d, func(n *html.Node) {
		if n.Data == "h1" && heading == "" {
			heading = text(n)
		}
		if n.Data != "h6" {
			return
		}
		m := snowFigure.FindStringSubmatch(text(n))
		if m == nil {
			return
		}
		v, _ := strconv.ParseFloat(m[2], 64)
		if strings.EqualFold(m[1], "New snowfall at base") {
			f["new_snow_cm"] = v
		} else {
			f["season_snowfall_cm"] = v
		}
	})
	if !strings.Contains(strings.ToLower(heading), strings.ReplaceAll(region, "-", " ")+" now:") {
		return nil, fmt.Errorf("dated source page does not identify the requested region")
	}
	dateHeading := regexp.MustCompile(`(?i):\s*([0-9]{1,2})(?:st|nd|rd|th)?\s+([a-z]+)\s+([0-9]{4})`).FindStringSubmatch(heading)
	if dateHeading == nil {
		return nil, fmt.Errorf("dated source page does not expose a verifiable report date")
	}
	day, _ := strconv.Atoi(dateHeading[1])
	year, _ := strconv.Atoi(dateHeading[3])
	month := months[strings.ToLower(dateHeading[2])]
	if time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Format("2006-01-02") != date {
		return nil, fmt.Errorf("source report date differs from the requested dated identity")
	}
	f["name"] = heading
	// Absence is unknown, including preseason pages; zero is retained only if published.
	return f, nil
}
