// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: website resolution (UUID or domain/name) and shared Umami API
// helpers used by the hand-written analysis commands.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/marketing/umami/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/umami/internal/store"
)

// umamiSite is the subset of the website object the analysis commands need.
type umamiSite struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Domain string `json:"domain"`
}

// fetchAllWebsites lists every website the account can access, paginating
// through /api/websites until the reported count is reached (capped at 50
// pages = 10k sites as a runaway guard).
func fetchAllWebsites(ctx context.Context, c *client.Client) ([]umamiSite, error) {
	const pageSize = 200
	var all []umamiSite
	for page := 1; page <= 50; page++ {
		data, err := c.Get(ctx, "/api/websites", map[string]string{
			"pageSize":     strconv.Itoa(pageSize),
			"page":         strconv.Itoa(page),
			"includeTeams": "true",
		})
		if err != nil {
			return nil, fmt.Errorf("listing websites (page %d): %w", page, err)
		}
		var envelope struct {
			Data  []umamiSite `json:"data"`
			Count int         `json:"count"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			return nil, fmt.Errorf("parsing websites list (page %d): %w", page, err)
		}
		all = append(all, envelope.Data...)
		if len(envelope.Data) < pageSize || (envelope.Count > 0 && len(all) >= envelope.Count) {
			break
		}
	}
	return all, nil
}

// resolveWebsiteID accepts a website UUID, domain, or display name and
// returns the UUID. Falls back to UMAMI_DEFAULT_WEBSITE when arg is empty.
func resolveWebsiteID(ctx context.Context, c *client.Client, arg string) (string, error) {
	if arg == "" {
		arg = os.Getenv("UMAMI_DEFAULT_WEBSITE")
	}
	if arg == "" {
		return "", usageErr(fmt.Errorf("website is required: pass a website UUID or domain, or set UMAMI_DEFAULT_WEBSITE"))
	}
	if store.IsUUID(arg) {
		return arg, nil
	}
	sites, err := fetchAllWebsites(ctx, c)
	if err != nil {
		return "", err
	}
	needle := strings.ToLower(strings.TrimPrefix(arg, "www."))
	for _, s := range sites {
		domain := strings.ToLower(strings.TrimPrefix(s.Domain, "www."))
		if domain == needle || strings.EqualFold(s.Name, arg) {
			return s.ID, nil
		}
	}
	var known []string
	for _, s := range sites {
		known = append(known, s.Domain)
	}
	return "", notFoundErr(fmt.Errorf("no website matches %q; known domains: %s", arg, strings.Join(known, ", ")))
}

// umamiStats is the flat v3 /stats response.
type umamiStats struct {
	Pageviews  float64 `json:"pageviews"`
	Visitors   float64 `json:"visitors"`
	Visits     float64 `json:"visits"`
	Bounces    float64 `json:"bounces"`
	Totaltime  float64 `json:"totaltime"`
	Comparison *struct {
		Pageviews float64 `json:"pageviews"`
		Visitors  float64 `json:"visitors"`
		Visits    float64 `json:"visits"`
		Bounces   float64 `json:"bounces"`
		Totaltime float64 `json:"totaltime"`
	} `json:"comparison"`
}

func getStats(ctx context.Context, c *client.Client, websiteID string, w periodWindow, compare string) (umamiStats, error) {
	params := map[string]string{
		"startAt": strconv.FormatInt(w.StartMs, 10),
		"endAt":   strconv.FormatInt(w.EndMs, 10),
	}
	if compare != "" {
		params["compare"] = compare
	}
	var out umamiStats
	data, err := c.Get(ctx, "/api/websites/"+websiteID+"/stats", params)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, fmt.Errorf("parsing stats: %w", err)
	}
	return out, nil
}

// umamiMetricRow is one {x, y} row from /metrics.
type umamiMetricRow struct {
	X string  `json:"x"`
	Y float64 `json:"y"`
}

func getMetrics(ctx context.Context, c *client.Client, websiteID, metricType string, w periodWindow, limit int) ([]umamiMetricRow, error) {
	params := map[string]string{
		"type":    metricType,
		"startAt": strconv.FormatInt(w.StartMs, 10),
		"endAt":   strconv.FormatInt(w.EndMs, 10),
		"limit":   strconv.Itoa(limit),
	}
	data, err := c.Get(ctx, "/api/websites/"+websiteID+"/metrics", params)
	if err != nil {
		return nil, err
	}
	var rows []umamiMetricRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("parsing %s metrics: %w", metricType, err)
	}
	return rows, nil
}

// getAllMetrics drains a metrics dimension with limit/offset pagination so
// busy days (>page rows) are captured completely, not just the top slice.
// Capped at 20 pages (10k rows per day/type) as a runaway guard.
func getAllMetrics(ctx context.Context, c *client.Client, websiteID, metricType string, w periodWindow) ([]umamiMetricRow, error) {
	const page = 500
	var all []umamiMetricRow
	for offset := 0; offset < 20*page; offset += page {
		params := map[string]string{
			"type":    metricType,
			"startAt": strconv.FormatInt(w.StartMs, 10),
			"endAt":   strconv.FormatInt(w.EndMs, 10),
			"limit":   strconv.Itoa(page),
			"offset":  strconv.Itoa(offset),
		}
		data, err := c.Get(ctx, "/api/websites/"+websiteID+"/metrics", params)
		if err != nil {
			return nil, err
		}
		var rows []umamiMetricRow
		if err := json.Unmarshal(data, &rows); err != nil {
			return nil, fmt.Errorf("parsing %s metrics (offset %d): %w", metricType, offset, err)
		}
		all = append(all, rows...)
		if len(rows) < page {
			break
		}
	}
	return all, nil
}

// seriesPoint is one {x, y} bucket from /pageviews.
type seriesPoint struct {
	X string  `json:"x"`
	Y float64 `json:"y"`
}

// getDailySeries returns per-day pageviews and sessions series.
func getDailySeries(ctx context.Context, c *client.Client, websiteID string, w periodWindow, timezone string) (pageviews, sessions []seriesPoint, err error) {
	if timezone == "" {
		timezone = "UTC"
	}
	params := map[string]string{
		"startAt":  strconv.FormatInt(w.StartMs, 10),
		"endAt":    strconv.FormatInt(w.EndMs, 10),
		"unit":     "day",
		"timezone": timezone,
	}
	data, err := c.Get(ctx, "/api/websites/"+websiteID+"/pageviews", params)
	if err != nil {
		return nil, nil, err
	}
	var envelope struct {
		Pageviews []seriesPoint `json:"pageviews"`
		Sessions  []seriesPoint `json:"sessions"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, nil, fmt.Errorf("parsing pageviews series: %w", err)
	}
	return envelope.Pageviews, envelope.Sessions, nil
}

// fetchFailure records one failed fan-out call for partial-failure reporting.
type fetchFailure struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

// siteStatsResult pairs a site with its fetched stats (or error).
type siteStatsResult struct {
	Site  umamiSite
	Stats umamiStats
	Err   error
}

// fanoutStats fetches stats for every site concurrently (bounded to 5 in
// flight) and preserves per-site errors for partial-failure accounting.
func fanoutStats(ctx context.Context, c *client.Client, sites []umamiSite, w periodWindow, compare string) []siteStatsResult {
	results := make([]siteStatsResult, len(sites))
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup
	for i, site := range sites {
		wg.Add(1)
		go func(i int, site umamiSite) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			stats, err := getStats(ctx, c, site.ID, w, compare)
			results[i] = siteStatsResult{Site: site, Stats: stats, Err: err}
		}(i, site)
	}
	wg.Wait()
	return results
}

// fanoutMetricsTypes fetches several metric dimensions for one site
// concurrently and preserves per-dimension errors.
func fanoutMetricsTypes(ctx context.Context, c *client.Client, websiteID string, w periodWindow, types []string, limit int) (map[string][]umamiMetricRow, []fetchFailure) {
	out := make(map[string][]umamiMetricRow, len(types))
	errs := make([]error, len(types))
	rowsByIdx := make([][]umamiMetricRow, len(types))
	var wg sync.WaitGroup
	for i, mt := range types {
		wg.Add(1)
		go func(i int, mt string) {
			defer wg.Done()
			rows, err := getMetrics(ctx, c, websiteID, mt, w, limit)
			rowsByIdx[i] = rows
			errs[i] = err
		}(i, mt)
	}
	wg.Wait()
	failures := make([]fetchFailure, 0)
	for i, mt := range types {
		if errs[i] != nil {
			failures = append(failures, fetchFailure{ID: "metrics:" + mt, Error: errs[i].Error()})
			continue
		}
		out[mt] = rowsByIdx[i]
	}
	return out, failures
}

// googleDomainRe matches google.<tld> and google.<tld>.<cc> shapes
// (google.com, google.fr, google.co.uk, google.com.br) without matching
// attacker-controlled lookalikes such as google.evil.example.
var googleDomainRe = regexp.MustCompile(`^google\.[a-z]{2,4}(\.[a-z]{2})?$`)

// isGoogleDomain reports whether a referrer domain belongs to Google.
func isGoogleDomain(domain string) bool {
	d := strings.ToLower(strings.TrimPrefix(domain, "www."))
	return googleDomainRe.MatchString(d) || strings.HasSuffix(d, ".google.com")
}

// pctChange returns the percent change from prev to cur (0 when prev is 0).
func pctChange(cur, prev float64) float64 {
	if prev == 0 {
		return 0
	}
	return (cur - prev) / prev * 100
}

// splitStatsResults separates successes from failures for partial-failure
// accounting; callers must exclude failures from every denominator.
func splitStatsResults(results []siteStatsResult) (ok []siteStatsResult, failures []fetchFailure) {
	ok = make([]siteStatsResult, 0, len(results))
	failures = make([]fetchFailure, 0)
	for _, r := range results {
		if r.Err != nil {
			failures = append(failures, fetchFailure{ID: r.Site.Domain, Error: r.Err.Error()})
			continue
		}
		ok = append(ok, r)
	}
	return ok, failures
}
