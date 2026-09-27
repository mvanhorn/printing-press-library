package tenki

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	MinSeasonYear = 1900
	MaxSeasonYear = 2200
)

// ValidateSeasonYear bounds a requested year independently of source availability.
func ValidateSeasonYear(year int) error {
	if year < MinSeasonYear || year > MaxSeasonYear {
		return fmt.Errorf("seasonal year must be between %d and %d", MinSeasonYear, MaxSeasonYear)
	}
	return nil
}

func seasonalKind(kind string) (string, error) {
	if kind == "foliage" {
		kind = "kouyou"
	}
	if kind != "sakura" && kind != "kouyou" {
		return "", fmt.Errorf("seasonal kind must be sakura or kouyou, got %q", kind)
	}
	return kind, nil
}

var seasonYearRE = regexp.MustCompile(`\b(20[0-9]{2})\b`)

func seasonContext(doc *html.Node, requested int, now time.Time) (int, string, string, string) {
	main := id(doc, "main-column")
	if main == nil {
		main = doc
	}
	h := find(main, func(n *html.Node) bool { return n.Data == "h2" })
	year := 0
	if m := seasonYearRE.FindStringSubmatch(nodeText(h)); m != nil {
		year = atoi(m[1])
	}
	state, status := "active", "ok"
	message := nodeText(class(main, "off-season-box"))
	if strings.Contains(message, "更新は終了") || strings.Contains(message, "更新を終了") {
		state, status = "ended", "season_ended"
	} else if message != "" {
		state, status = "out_of_season", "out_of_season"
	}
	if year == 0 {
		state, status = "unknown", "unavailable"
	}
	if requested != year && year != 0 {
		status = "year_unavailable"
	}
	relation := "current"
	if year < now.In(JST).Year() {
		relation = "historical"
	}
	if year > now.In(JST).Year() {
		relation = "future"
	}
	if year == 0 {
		relation = "unknown"
	}
	return year, relation, state, status
}

func seasonalIssue(doc *html.Node) (time.Time, string) {
	main := id(doc, "main-column")
	if main == nil {
		main = doc
	}
	h := find(main, func(n *html.Node) bool { return n.Data == "h2" })
	t := find(h, func(n *html.Node) bool { return n.Data == "time" && attr(n, "datetime") != "" })
	if t == nil {
		t = class(main, "top-map-date-time")
	}
	raw := attr(t, "datetime")
	issue, _ := time.Parse(time.RFC3339, raw)
	return issue, raw
}

func labelValues(root *html.Node) map[string]string {
	result := map[string]string{}
	for _, dt := range all(root, func(n *html.Node) bool { return n.Data == "dt" || n.Data == "th" }) {
		for next := dt.NextSibling; next != nil; next = next.NextSibling {
			if next.Type != html.ElementNode {
				continue
			}
			if next.Data == "dd" || next.Data == "td" {
				result[nodeText(dt)] = nodeText(next)
			}
			break
		}
	}
	return result
}

func seasonalSpot(doc *html.Node, place Place, year int, issue time.Time) SeasonalSpot {
	spot := SeasonalSpot{Place: place, Year: year, Species: []string{}}
	info := id(doc, "section-info")
	if info == nil {
		return spot
	}
	spot.Condition = nodeText(class(info, "rank-telop"))
	if spot.Condition == "" {
		spot.Condition = nodeText(class(info, "bloom-telop"))
	}
	if spot.Condition == "" {
		spot.Condition = nodeText(class(info, "status-telop"))
	}
	spot.ReportRaw = nodeText(class(info, "heading-note"))
	// A day-only spot report never inherits the product's publication hour.
	if !issue.IsZero() && spot.ReportRaw != "" {
		if d := dateFromText(spot.ReportRaw, issue); !d.IsZero() {
			spot.ReportDate = d.Format("2006-01-02")
		} else if m := regexp.MustCompile(`([0-9]{1,2})日`).FindStringSubmatch(spot.ReportRaw); m != nil {
			d := dayClock(m[1]+"日00:00", issue)
			if !d.IsZero() {
				spot.ReportDate = d.Format("2006-01-02")
			}
		}
	}
	if report := find(info, func(n *html.Node) bool { return n.Data == "time" && attr(n, "datetime") != "" }); report != nil {
		if at, e := time.Parse(time.RFC3339, attr(report, "datetime")); e == nil {
			spot.ReportAt = stamp(at)
			spot.ReportDate = at.In(JST).Format("2006-01-02")
			spot.ReportRaw = nodeText(report)
		}
	}
	for label, value := range labelValues(info) {
		if strings.Contains(label, "種類") {
			for _, v := range regexp.MustCompile(`[、,，]`).Split(value, -1) {
				v = strings.TrimSpace(v)
				if v != "" && v != "情報なし" {
					spot.Species = append(spot.Species, v)
				}
			}
		}
		if strings.Contains(label, "例年") || strings.Contains(label, "平年") {
			spot.NormalPeriod = value
			continue
		}
		if strings.Contains(label, "見頃") || strings.Contains(label, "見ごろ") {
			if strings.Contains(value, "例年") || strings.Contains(value, "平年") {
				spot.NormalPeriod = strings.TrimSpace(strings.TrimPrefix(value, "例年の見頃："))
			} else if strings.Contains(label, "予想") {
				spot.PredictedBestPeriod = value
			}
		}
		if strings.Contains(label, "予想") {
			date := time.Time{}
			if m := fullDateRE.FindStringSubmatch(value); m != nil {
				date = validDate(atoi(m[1]), atoi(m[2]), atoi(m[3]))
			} else if m := monthDayRE.FindStringSubmatch(value); m != nil {
				date = validDate(year, atoi(m[1]), atoi(m[2]))
			}
			if date.IsZero() {
				continue
			}
			if strings.Contains(label, "開花") {
				spot.PredictedFloweringDate = date.Format("2006-01-02")
			}
			if strings.Contains(label, "満開") {
				spot.PredictedFullBloomDate = date.Format("2006-01-02")
			}
		}
	}
	return spot
}

func (c *Client) Seasonal(ctx context.Context, kind, target string, year int) (SeasonalResult, error) {
	kind, err := seasonalKind(kind)
	if err != nil {
		return SeasonalResult{}, err
	}
	if year == 0 {
		year = c.cfg.Now().In(JST).Year()
	}
	if err := ValidateSeasonYear(year); err != nil {
		return SeasonalResult{}, err
	}
	raw, err := canonicalPlaceURL(target)
	if err != nil {
		if strings.Contains(target, "://") {
			return SeasonalResult{}, err
		}
		list, err := c.SeasonalList(ctx, kind, target, year, 10, 2)
		if err != nil {
			return SeasonalResult{}, err
		}
		if len(list.Spots) == 0 {
			return SeasonalResult{Kind: kind, Year: list.Year, RequestedYear: year, YearRelation: list.YearRelation, UpdateState: list.UpdateState, Source: list.Source, Status: list.Status, Warnings: list.Warnings, Spot: SeasonalSpot{Species: []string{}}}, nil
		}
		if len(list.Spots) != 1 || list.Truncated {
			return SeasonalResult{}, errors.New("seasonal name is ambiguous; select a canonical spot URL")
		}
		raw = list.Spots[0].Place.URL
	}
	if !strings.HasPrefix(raw, "https://tenki.jp/"+kind+"/") {
		return SeasonalResult{}, errors.New("seasonal URL kind differs from requested kind")
	}
	body, source, err := c.fetch(ctx, raw, forecastTTL)
	if err != nil {
		return SeasonalResult{}, err
	}
	return c.parseSeasonal(kind, body, source, raw, year), nil
}

func (c *Client) parseSeasonal(kind, body string, source Source, raw string, requested int) SeasonalResult {
	doc := parseHTML(body)
	year, relation, state, status := seasonContext(doc, requested, c.cfg.Now())
	issue, issueRaw := seasonalIssue(doc)
	applyIssue(&source, issue, issueRaw, 36*time.Hour, c.cfg.Now())
	place := bodyPlace(doc, raw, kind)
	result := SeasonalResult{Kind: kind, Year: year, RequestedYear: requested, YearRelation: relation, UpdateState: state, Status: status, Place: place, Spot: seasonalSpot(doc, place, year, issue), Source: source, Warnings: sourceWarnings(source)}
	if state == "ended" {
		result.Warnings = append(result.Warnings, "Seasonal updates ended. Retained seasonal facts are separate from continuing municipal weather updates.")
	}
	if status == "year_unavailable" {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Requested year %d is unavailable; this page retains %d information.", requested, year))
	}
	result.Warnings = append(result.Warnings, "Typical viewing periods and peak-season photographs are not current reports or exact-date predictions.")
	return result
}

func (c *Client) SeasonalList(ctx context.Context, kind, query string, year, limit, maxPages int) (SeasonalListResult, error) {
	kind, err := seasonalKind(kind)
	if err != nil {
		return SeasonalListResult{}, err
	}
	limit, maxPages, err = bounds(limit, maxPages)
	if err != nil {
		return SeasonalListResult{}, err
	}
	if year == 0 {
		year = c.cfg.Now().In(JST).Year()
	}
	if err := ValidateSeasonYear(year); err != nil {
		return SeasonalListResult{}, err
	}
	result := SeasonalListResult{Kind: kind, RequestedYear: year, Spots: []SeasonalSpot{}, Warnings: []string{}, Status: "ok"}
	raw := "https://tenki.jp/" + kind + "/"
	if kind == "kouyou" && query != "" && c.cfg.SearchDirectory == "" {
		raw = "https://tenki.jp/kouyou/search/?" + url.Values{"keyword": {query}, "search_type": {"venue"}}.Encode()
	}
	if c.cfg.SearchDirectory != "" {
		raw = c.cfg.SearchDirectory
		if !strings.HasPrefix(raw, "https://tenki.jp/"+kind+"/") {
			return result, errors.New("seasonal --directory must match the selected kind")
		}
		u, e := url.Parse(raw)
		if e != nil || !regexp.MustCompile(`^/(?:sakura|kouyou)/(?:[0-9]+/){0,2}$`).MatchString(u.Path) {
			return result, errors.New("seasonal directory must identify an index, region or prefecture")
		}
	}
	if _, err = validateURL(raw); err != nil {
		return result, err
	}
	seen := map[string]bool{}
	for result.Pages < maxPages {
		body, source, err := c.fetch(ctx, raw, forecastTTL)
		if err != nil {
			return result, err
		}
		result.Pages++
		doc := parseHTML(body)
		sourceYear, relation, state, status := seasonContext(doc, year, c.cfg.Now())
		issue, ir := seasonalIssue(doc)
		applyIssue(&source, issue, ir, 36*time.Hour, c.cfg.Now())
		result.Year, result.YearRelation, result.UpdateState, result.Status, result.Source = sourceYear, relation, state, status, source
		if status == "year_unavailable" || status == "unavailable" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Requested year %d has no verified product in this directory (source year %d).", year, sourceYear))
			break
		}
		main := id(doc, "main-column")
		if main == nil {
			main = doc
		}
		for _, a := range all(main, func(n *html.Node) bool { return n.Data == "a" }) {
			u, err := canonicalPlaceURL(absoluteURL(attr(a, "href")))
			if err != nil || !strings.HasPrefix(u, "https://tenki.jp/"+kind+"/") || seen[u] {
				continue
			}
			seen[u] = true
			result.Scanned++
			name := imageAlt(a)
			if name == "" {
				name = nodeText(class(a, "name"))
			}
			if name == "" {
				name = nodeText(class(a, "text-box"))
			}
			if name == "" {
				name = nodeText(a)
			}
			if query != "" && !strings.Contains(name, query) && !strings.Contains(nodeText(a), query) {
				continue
			}
			condition := nodeText(class(a, "kouyou-bloom-box-rank-telop"))
			if condition == "" {
				condition = nodeText(class(a, "rank-telop"))
			}
			if condition == "" {
				condition = nodeText(class(a, "sakura-bloom-box-rank-telop"))
			}
			spot := SeasonalSpot{Place: Place{ID: strings.TrimPrefix(u, "https://tenki.jp/"), URL: u, Name: name, Kind: kind, Scope: "municipal"}, Year: sourceYear, Condition: condition, Species: []string{}}
			result.Spots = append(result.Spots, spot)
		}
		next := nextPage(doc, raw)
		if len(result.Spots) > limit {
			result.Spots = result.Spots[:limit]
			result.Truncated = true
		}
		if next == "" {
			break
		}
		if result.Pages >= maxPages || len(result.Spots) >= limit {
			result.Truncated = true
			break
		}
		raw = next
	}
	if len(result.Spots) == 0 && (result.Status == "ok" || (query != "" && result.Status == "season_ended")) {
		result.Status = "no_results"
	}
	result.Warnings = append(result.Warnings, sourceWarnings(result.Source)...)
	result.Warnings = append(result.Warnings, fmt.Sprintf("Scanned %d linked spots in %d bounded directory page(s); absence is not a nationwide catalog result.", result.Scanned, result.Pages))
	if result.UpdateState == "ended" {
		result.Warnings = append(result.Warnings, "Seasonal updates ended; fresh weather elsewhere on the page does not refresh seasonal information.")
	}
	return result, nil
}
