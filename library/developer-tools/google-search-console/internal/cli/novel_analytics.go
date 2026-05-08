// Novel transcendence commands rooted in the search_analytics_rows table:
// book, cannibalize, decay, opportunity, momentum, new-queries, territory,
// appearance. Each one runs a SQL query the live API cannot express
// (cross-time, cross-page, cross-property aggregates) and emits a typed
// envelope with --json/--select/--csv friendly shape.
package cli

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func timeParse(layout, value string) (time.Time, error) { return time.Parse(layout, value) }

// -------- book: cross-property mover board --------------------------------

func newBookCmd(flags *rootFlags) *cobra.Command {
	cf := commonFlags{}
	var top int
	cmd := &cobra.Command{
		Use:   "book",
		Short: "Cross-property mover board: top click deltas across every verified site",
		Long: strings.TrimSpace(`
One report covering every verified property: top pages and queries by absolute
click delta in the last window, with per-site rollup rows. Devin's whole Friday
client email collapses into one command.
`),
		Example:     "  google-search-console-pp-cli book --window 7d --top 25 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStoreFromFlag(cmd.Context(), cf.db)
			if err != nil {
				return err
			}
			defer s.Close()
			ok, err := requireStoreData(s, "")
			if err != nil {
				return err
			}
			if !ok {
				return emitEmpty(cmd, flags, "no data in local store")
			}
			days := parseWindow(cf.window, 7)
			start, end := dateRange(days)
			priorEnd := mustAddDays(start, -1)
			priorStart := mustAddDays(priorEnd, -days+1)

			rows, err := s.DB().QueryContext(cmd.Context(), `
WITH curr AS (
  SELECT site_url, page, query, SUM(clicks) AS c
  FROM search_analytics_rows
  WHERE date BETWEEN ? AND ?
  GROUP BY site_url, page, query
),
prev AS (
  SELECT site_url, page, query, SUM(clicks) AS c
  FROM search_analytics_rows
  WHERE date BETWEEN ? AND ?
  GROUP BY site_url, page, query
)
SELECT COALESCE(curr.site_url, prev.site_url) AS site_url,
       COALESCE(curr.page, prev.page)         AS page,
       COALESCE(curr.query, prev.query)       AS query,
       COALESCE(curr.c, 0)                    AS curr_clicks,
       COALESCE(prev.c, 0)                    AS prev_clicks,
       COALESCE(curr.c, 0) - COALESCE(prev.c, 0) AS click_delta
FROM curr LEFT JOIN prev USING (site_url, page, query)
UNION
SELECT prev.site_url, prev.page, prev.query, 0, prev.c, -prev.c
FROM prev LEFT JOIN curr USING (site_url, page, query) WHERE curr.page IS NULL
ORDER BY ABS(click_delta) DESC LIMIT ?`,
				start, end, priorStart, priorEnd, top)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			data, err := rowsToMaps(rows)
			if err != nil {
				return apiErr(err)
			}
			// Per-site rollup.
			rollup := map[string]float64{}
			for _, r := range data {
				site, _ := r["site_url"].(string)
				delta, _ := toFloat(r["click_delta"])
				rollup[site] += delta
			}
			rollupRows := []map[string]any{}
			for site, d := range rollup {
				rollupRows = append(rollupRows, map[string]any{"site_url": site, "total_click_delta": d})
			}
			return emit(cmd, flags, map[string]any{
				"window":      fmt.Sprintf("%dd", days),
				"date_range":  fmt.Sprintf("%s..%s", start, end),
				"prior_range": fmt.Sprintf("%s..%s", priorStart, priorEnd),
				"top":         top,
				"rows":        data,
				"rollup":      rollupRows,
			})
		},
	}
	cmd.Flags().StringVar(&cf.site, "site", "", "Site URL (e.g. sc-domain:example.com). Required.")
	cmd.Flags().StringVar(&cf.window, "window", "7d", "Analysis window: Nd, Nw, or Nm.")
	cmd.Flags().StringVar(&cf.db, "db", "", "SQLite path (default ~/.config/google-search-console-pp-cli/store.sqlite).")
	cmd.Flags().IntVar(&top, "top", 25, "Maximum movers to return.")
	return cmd
}

// -------- cannibalize: same-site, same-query competition ------------------

func newCannibalizeCmd(flags *rootFlags) *cobra.Command {
	cf := commonFlags{}
	var top int
	var minImpressions float64
	cmd := &cobra.Command{
		Use:   "cannibalize",
		Short: "Find queries where 2+ pages on the same site compete for the same intent",
		Long: strings.TrimSpace(`
GROUP BY (site, query) HAVING COUNT(DISTINCT page) > 1, with impression split
and CTR drag annotations. Single-call API cannot express this — needs the
local store.
`),
		Example:     "  google-search-console-pp-cli cannibalize --site sc-domain:example.com --min-impressions 100 --top 20 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := requireSite(cf.site); err != nil {
				return err
			}
			s, err := openStoreFromFlag(cmd.Context(), cf.db)
			if err != nil {
				return err
			}
			defer s.Close()
			ok, err := requireStoreData(s, cf.site)
			if err != nil {
				return err
			}
			if !ok {
				return emitEmpty(cmd, flags, fmt.Sprintf("no data for site %s", cf.site))
			}
			days := parseWindow(cf.window, 28)
			start, end := dateRange(days)
			rows, err := s.DB().QueryContext(cmd.Context(), `
WITH per_pair AS (
  SELECT query, page, SUM(clicks) AS clicks, SUM(impressions) AS impressions
  FROM search_analytics_rows
  WHERE site_url = ? AND date BETWEEN ? AND ? AND query <> '' AND page <> ''
  GROUP BY query, page
),
totals AS (
  SELECT query, COUNT(DISTINCT page) AS contender_count,
         SUM(impressions) AS total_impressions, SUM(clicks) AS total_clicks
  FROM per_pair GROUP BY query
)
SELECT t.query AS query, t.contender_count, t.total_impressions, t.total_clicks,
       (SELECT GROUP_CONCAT(p.page, '|') FROM per_pair p
          WHERE p.query = t.query ORDER BY p.impressions DESC) AS contender_pages
FROM totals t
WHERE t.contender_count >= 2 AND t.total_impressions >= ?
ORDER BY t.total_impressions DESC LIMIT ?`,
				cf.site, start, end, minImpressions, top)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			data, err := rowsToMaps(rows)
			if err != nil {
				return apiErr(err)
			}
			// Split contender_pages into a real array.
			for _, r := range data {
				if v, ok := r["contender_pages"].(string); ok {
					r["contender_pages"] = splitNonEmpty(v, "|")
				}
			}
			return emit(cmd, flags, map[string]any{
				"site":       cf.site,
				"window":     fmt.Sprintf("%dd", days),
				"date_range": fmt.Sprintf("%s..%s", start, end),
				"rows":       data,
			})
		},
	}
	cmd.Flags().StringVar(&cf.site, "site", "", "Site URL (e.g. sc-domain:example.com). Required.")
	cmd.Flags().StringVar(&cf.window, "window", "28d", "Analysis window: Nd, Nw, or Nm.")
	cmd.Flags().StringVar(&cf.db, "db", "", "SQLite path (default ~/.config/google-search-console-pp-cli/store.sqlite).")
	cmd.Flags().IntVar(&top, "top", 20, "Maximum cannibalized queries to return.")
	cmd.Flags().Float64Var(&minImpressions, "min-impressions", 100, "Skip queries below this total impression count.")
	return cmd
}

// -------- decay: linear-fit slope for each query --------------------------

func newDecayCmd(flags *rootFlags) *cobra.Command {
	cf := commonFlags{}
	var minLoss float64
	cmd := &cobra.Command{
		Use:   "decay",
		Short: "Queries whose impressions or clicks have steadily declined over the window",
		Long: strings.TrimSpace(`
Linear-fit slope per (site, query) over the window's daily rows; ranks by
absolute click loss. Catches gradual erosion before it becomes a missing-traffic
emergency. Live API has no slope endpoint.
`),
		Example:     "  google-search-console-pp-cli decay --site sc-domain:example.com --window 12w --min-loss 100 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := requireSite(cf.site); err != nil {
				return err
			}
			s, err := openStoreFromFlag(cmd.Context(), cf.db)
			if err != nil {
				return err
			}
			defer s.Close()
			ok, err := requireStoreData(s, cf.site)
			if err != nil {
				return err
			}
			if !ok {
				return emitEmpty(cmd, flags, fmt.Sprintf("no data for site %s", cf.site))
			}
			days := parseWindow(cf.window, 84) // 12 weeks default
			start, end := dateRange(days)
			rows, err := s.DB().QueryContext(cmd.Context(), `
SELECT query, date, SUM(clicks) AS clicks, SUM(impressions) AS impressions
FROM search_analytics_rows
WHERE site_url = ? AND date BETWEEN ? AND ? AND query <> ''
GROUP BY query, date
ORDER BY query, date`,
				cf.site, start, end)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type series struct {
				dates  []float64 // day index
				clicks []float64
			}
			perQuery := map[string]*series{}
			startEpoch := dateToEpochDays(start)
			for rows.Next() {
				var query, date string
				var clicks, impressions float64
				if err := rows.Scan(&query, &date, &clicks, &impressions); err != nil {
					return apiErr(err)
				}
				ser, ok := perQuery[query]
				if !ok {
					ser = &series{}
					perQuery[query] = ser
				}
				ser.dates = append(ser.dates, float64(dateToEpochDays(date)-startEpoch))
				ser.clicks = append(ser.clicks, clicks)
			}
			out := []map[string]any{}
			for q, ser := range perQuery {
				if len(ser.dates) < 5 {
					continue // need enough points to fit a slope
				}
				slope, intercept := linearFit(ser.dates, ser.clicks)
				lossEstimate := math.Abs(slope) * float64(days)
				if slope >= 0 || lossEstimate < minLoss {
					continue
				}
				totalClicks := 0.0
				for _, c := range ser.clicks {
					totalClicks += c
				}
				out = append(out, map[string]any{
					"query":          q,
					"slope_per_day":  round3(slope),
					"intercept":      round3(intercept),
					"window_days":    days,
					"data_points":    len(ser.dates),
					"total_clicks":   totalClicks,
					"estimated_loss": round3(lossEstimate),
				})
			}
			sortByFloatDesc(out, "estimated_loss")
			return emit(cmd, flags, map[string]any{
				"site": cf.site, "window": fmt.Sprintf("%dd", days),
				"date_range": fmt.Sprintf("%s..%s", start, end),
				"rows":       out,
			})
		},
	}
	cmd.Flags().StringVar(&cf.site, "site", "", "Site URL (e.g. sc-domain:example.com). Required.")
	cmd.Flags().StringVar(&cf.window, "window", "12w", "Analysis window: Nd, Nw, or Nm.")
	cmd.Flags().StringVar(&cf.db, "db", "", "SQLite path (default ~/.config/google-search-console-pp-cli/store.sqlite).")
	cmd.Flags().Float64Var(&minLoss, "min-loss", 100, "Filter out queries whose estimated click loss over the window is below this.")
	return cmd
}

// -------- opportunity: positions 4-20 with baseline -----------------------

func newOpportunityCmd(flags *rootFlags) *cobra.Command {
	cf := commonFlags{}
	var newSince string
	var minImpressions float64
	cmd := &cobra.Command{
		Use:   "opportunity",
		Short: "Quick-wins (positions 4-20, high impression / low CTR) joined to prior snapshots for entry-date baseline",
		Long: strings.TrimSpace(`
Today's opportunity zone (positions 4-20, high-impression / low-CTR pages)
joined to prior daily snapshots, so each row carries the date the page entered
the zone. Beats single-snapshot "quick wins" tools by distinguishing fresh
opportunities from chronic ones.
`),
		Example:     "  google-search-console-pp-cli opportunity --site sc-domain:example.com --new-since 14d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := requireSite(cf.site); err != nil {
				return err
			}
			s, err := openStoreFromFlag(cmd.Context(), cf.db)
			if err != nil {
				return err
			}
			defer s.Close()
			ok, err := requireStoreData(s, cf.site)
			if err != nil {
				return err
			}
			if !ok {
				return emitEmpty(cmd, flags, fmt.Sprintf("no data for site %s", cf.site))
			}
			days := parseWindow(cf.window, 28)
			start, end := dateRange(days)
			newWindow := parseWindow(newSince, 14)
			cutoff := mustAddDays(end, -newWindow+1)

			// Today's opportunity set.
			rows, err := s.DB().QueryContext(cmd.Context(), `
SELECT page, SUM(clicks) AS clicks, SUM(impressions) AS impressions,
       AVG(position) AS avg_position
FROM search_analytics_rows
WHERE site_url = ? AND date BETWEEN ? AND ? AND page <> ''
GROUP BY page
HAVING avg_position BETWEEN 4 AND 20 AND impressions >= ?
ORDER BY impressions DESC`,
				cf.site, start, end, minImpressions)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type oppRow struct {
				Page        string  `json:"page"`
				Clicks      float64 `json:"clicks"`
				Impressions float64 `json:"impressions"`
				AvgPosition float64 `json:"avg_position"`
				CTR         float64 `json:"ctr"`
				EnteredZone string  `json:"entered_zone"`
				IsNew       bool    `json:"is_new"`
			}
			var data []oppRow
			for rows.Next() {
				var r oppRow
				if err := rows.Scan(&r.Page, &r.Clicks, &r.Impressions, &r.AvgPosition); err != nil {
					return apiErr(err)
				}
				if r.Impressions > 0 {
					r.CTR = r.Clicks / r.Impressions
				}
				data = append(data, r)
			}
			// Compute entry date per page (earliest date the page first
			// landed inside positions 4-20 for this site).
			for i := range data {
				var first string
				err := s.DB().QueryRowContext(cmd.Context(), `
SELECT MIN(date) FROM (
  SELECT date, AVG(position) AS pos
  FROM search_analytics_rows
  WHERE site_url = ? AND page = ?
  GROUP BY date
  HAVING pos BETWEEN 4 AND 20
)`, cf.site, data[i].Page).Scan(&first)
				if err == nil {
					data[i].EnteredZone = first
					data[i].IsNew = first >= cutoff
				}
			}
			return emit(cmd, flags, map[string]any{
				"site":       cf.site,
				"window":     fmt.Sprintf("%dd", days),
				"date_range": fmt.Sprintf("%s..%s", start, end),
				"new_since":  fmt.Sprintf("%dd", newWindow),
				"new_cutoff": cutoff,
				"rows":       data,
			})
		},
	}
	cmd.Flags().StringVar(&cf.site, "site", "", "Site URL (e.g. sc-domain:example.com). Required.")
	cmd.Flags().StringVar(&cf.window, "window", "28d", "Analysis window: Nd, Nw, or Nm.")
	cmd.Flags().StringVar(&cf.db, "db", "", "SQLite path (default ~/.config/google-search-console-pp-cli/store.sqlite).")
	cmd.Flags().StringVar(&newSince, "new-since", "14d", "Mark pages that entered the opportunity zone within this window as new.")
	cmd.Flags().Float64Var(&minImpressions, "min-impressions", 100, "Filter out pages below this impression threshold.")
	return cmd
}

// -------- momentum: page-level rolling-vs-baseline ------------------------

func newMomentumCmd(flags *rootFlags) *cobra.Command {
	cf := commonFlags{}
	var vs string
	var minLift float64
	cmd := &cobra.Command{
		Use:   "momentum",
		Short: "Pages whose recent clicks differ sharply from their trailing baseline",
		Long: strings.TrimSpace(`
Compares the page's recent rolling-window clicks against a longer trailing
baseline. Both directions: rising stars (lift > min-lift) and collapsing pages
(lift < -min-lift).
`),
		Example:     "  google-search-console-pp-cli momentum --site sc-domain:example.com --window 7d --vs 28d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := requireSite(cf.site); err != nil {
				return err
			}
			s, err := openStoreFromFlag(cmd.Context(), cf.db)
			if err != nil {
				return err
			}
			defer s.Close()
			ok, err := requireStoreData(s, cf.site)
			if err != nil {
				return err
			}
			if !ok {
				return emitEmpty(cmd, flags, fmt.Sprintf("no data for site %s", cf.site))
			}
			recent := parseWindow(cf.window, 7)
			baseline := parseWindow(vs, 28)
			recentStart, recentEnd := dateRange(recent)
			baseEnd := mustAddDays(recentStart, -1)
			baseStart := mustAddDays(baseEnd, -baseline+1)
			rows, err := s.DB().QueryContext(cmd.Context(), `
WITH recent AS (
  SELECT page, SUM(clicks) AS c FROM search_analytics_rows
  WHERE site_url = ? AND date BETWEEN ? AND ? AND page <> ''
  GROUP BY page
),
base AS (
  SELECT page, SUM(clicks) AS c FROM search_analytics_rows
  WHERE site_url = ? AND date BETWEEN ? AND ? AND page <> ''
  GROUP BY page
)
SELECT COALESCE(recent.page, base.page) AS page,
       COALESCE(recent.c, 0) AS recent_clicks,
       COALESCE(base.c, 0) AS baseline_clicks
FROM recent LEFT JOIN base USING(page)
UNION
SELECT base.page, 0, base.c FROM base LEFT JOIN recent USING(page) WHERE recent.page IS NULL`,
				cf.site, recentStart, recentEnd, cf.site, baseStart, baseEnd)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			out := []map[string]any{}
			for rows.Next() {
				var page string
				var rc, bc float64
				if err := rows.Scan(&page, &rc, &bc); err != nil {
					return apiErr(err)
				}
				baselineDaily := bc / float64(baseline)
				expected := baselineDaily * float64(recent)
				lift := 0.0
				if expected > 0 {
					lift = (rc - expected) / expected
				} else if rc > 0 {
					lift = math.Inf(1) // brand-new page
				}
				if math.IsInf(lift, 1) || math.Abs(lift) >= minLift {
					out = append(out, map[string]any{
						"page":            page,
						"recent_clicks":   rc,
						"baseline_clicks": bc,
						"expected":        round3(expected),
						"lift":            sanitizeFloat(lift),
						"direction":       directionOf(lift),
					})
				}
			}
			sortByFloatDesc(out, "lift")
			return emit(cmd, flags, map[string]any{
				"site":            cf.site,
				"recent_window":   fmt.Sprintf("%dd", recent),
				"baseline_window": fmt.Sprintf("%dd", baseline),
				"date_range":      fmt.Sprintf("%s..%s vs %s..%s", recentStart, recentEnd, baseStart, baseEnd),
				"rows":            out,
			})
		},
	}
	cmd.Flags().StringVar(&cf.site, "site", "", "Site URL (e.g. sc-domain:example.com). Required.")
	cmd.Flags().StringVar(&cf.window, "window", "7d", "Analysis window: Nd, Nw, or Nm.")
	cmd.Flags().StringVar(&cf.db, "db", "", "SQLite path (default ~/.config/google-search-console-pp-cli/store.sqlite).")
	cmd.Flags().StringVar(&vs, "vs", "28d", "Baseline window length to compare against.")
	cmd.Flags().Float64Var(&minLift, "min-lift", 0.5, "Only show pages whose absolute lift is at least this (e.g. 0.5 = ±50%).")
	return cmd
}

// -------- new-queries (with --lost) ---------------------------------------

func newNewQueriesCmd(flags *rootFlags) *cobra.Command {
	cf := commonFlags{}
	var minImpressions float64
	var lost bool
	cmd := &cobra.Command{
		Use:   "new-queries",
		Short: "Queries that appeared in the recent window and were absent from the prior trailing window",
		Long: strings.TrimSpace(`
LEFT JOIN against the prior window — anti-join requires persistent state.
--lost inverts the predicate to surface queries that vanished.
`),
		Example:     "  google-search-console-pp-cli new-queries --site sc-domain:example.com --window 7d --min-impressions 50 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := requireSite(cf.site); err != nil {
				return err
			}
			s, err := openStoreFromFlag(cmd.Context(), cf.db)
			if err != nil {
				return err
			}
			defer s.Close()
			ok, err := requireStoreData(s, cf.site)
			if err != nil {
				return err
			}
			if !ok {
				return emitEmpty(cmd, flags, fmt.Sprintf("no data for site %s", cf.site))
			}
			days := parseWindow(cf.window, 7)
			start, end := dateRange(days)
			priorEnd := mustAddDays(start, -1)
			priorStart := mustAddDays(priorEnd, -days+1)

			var rows *sql.Rows
			if !lost {
				rows, err = s.DB().QueryContext(cmd.Context(), `
WITH curr AS (
  SELECT query AS q, SUM(clicks) AS clicks, SUM(impressions) AS impressions
  FROM search_analytics_rows
  WHERE site_url = ? AND date BETWEEN ? AND ? AND query <> ''
  GROUP BY query
),
prev AS (
  SELECT query AS q FROM search_analytics_rows
  WHERE site_url = ? AND date BETWEEN ? AND ? AND query <> ''
  GROUP BY query
)
SELECT curr.q AS query,
       COALESCE(curr.clicks, 0) AS clicks,
       COALESCE(curr.impressions, 0) AS impressions
FROM curr LEFT JOIN prev ON curr.q = prev.q
WHERE prev.q IS NULL AND curr.impressions >= ?
ORDER BY curr.impressions DESC`,
					cf.site, start, end, cf.site, priorStart, priorEnd, minImpressions)
			} else {
				rows, err = s.DB().QueryContext(cmd.Context(), `
WITH curr AS (
  SELECT query AS q FROM search_analytics_rows
  WHERE site_url = ? AND date BETWEEN ? AND ? AND query <> ''
  GROUP BY query
),
prev AS (
  SELECT query AS q, SUM(clicks) AS clicks, SUM(impressions) AS impressions
  FROM search_analytics_rows
  WHERE site_url = ? AND date BETWEEN ? AND ? AND query <> ''
  GROUP BY query
)
SELECT prev.q AS query,
       COALESCE(prev.clicks, 0) AS clicks,
       COALESCE(prev.impressions, 0) AS impressions
FROM prev LEFT JOIN curr ON prev.q = curr.q
WHERE curr.q IS NULL AND prev.impressions >= ?
ORDER BY prev.impressions DESC`,
					cf.site, start, end, cf.site, priorStart, priorEnd, minImpressions)
			}
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			data, err := rowsToMaps(rows)
			if err != nil {
				return apiErr(err)
			}
			return emit(cmd, flags, map[string]any{
				"site":        cf.site,
				"window":      fmt.Sprintf("%dd", days),
				"date_range":  fmt.Sprintf("%s..%s", start, end),
				"prior_range": fmt.Sprintf("%s..%s", priorStart, priorEnd),
				"mode":        ifString(lost, "lost", "new"),
				"rows":        data,
			})
		},
	}
	cmd.Flags().StringVar(&cf.site, "site", "", "Site URL (e.g. sc-domain:example.com). Required.")
	cmd.Flags().StringVar(&cf.window, "window", "7d", "Analysis window: Nd, Nw, or Nm.")
	cmd.Flags().StringVar(&cf.db, "db", "", "SQLite path (default ~/.config/google-search-console-pp-cli/store.sqlite).")
	cmd.Flags().Float64Var(&minImpressions, "min-impressions", 50, "Filter queries below this impression count.")
	cmd.Flags().BoolVar(&lost, "lost", false, "Invert: show queries that vanished from the recent window.")
	return cmd
}

// -------- territory: country/device share shifts --------------------------

func newTerritoryCmd(flags *rootFlags) *cobra.Command {
	cf := commonFlags{}
	var by string
	var minShift float64
	cmd := &cobra.Command{
		Use:   "territory",
		Short: "Per-query change in country and device mix between recent and prior window",
		Long: strings.TrimSpace(`
Cross-time pivot on (query × dimension) where dimension is country, device,
or "country,device". Reports queries whose share split moved more than the
threshold.
`),
		Example:     "  google-search-console-pp-cli territory --site sc-domain:example.com --by country,device --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := requireSite(cf.site); err != nil {
				return err
			}
			s, err := openStoreFromFlag(cmd.Context(), cf.db)
			if err != nil {
				return err
			}
			defer s.Close()
			ok, err := requireStoreData(s, cf.site)
			if err != nil {
				return err
			}
			if !ok {
				return emitEmpty(cmd, flags, fmt.Sprintf("no data for site %s", cf.site))
			}
			days := parseWindow(cf.window, 7)
			start, end := dateRange(days)
			priorEnd := mustAddDays(start, -1)
			priorStart := mustAddDays(priorEnd, -days+1)
			groupKey, err := buildGroupKey(by)
			if err != nil {
				return usageErr(err)
			}
			query := fmt.Sprintf(`
WITH curr AS (
  SELECT query, %s AS dim, SUM(impressions) AS imp FROM search_analytics_rows
  WHERE site_url = ? AND date BETWEEN ? AND ? AND query <> ''
  GROUP BY query, dim
),
prev AS (
  SELECT query, %s AS dim, SUM(impressions) AS imp FROM search_analytics_rows
  WHERE site_url = ? AND date BETWEEN ? AND ? AND query <> ''
  GROUP BY query, dim
),
curr_tot AS (SELECT query, SUM(imp) AS t FROM curr GROUP BY query),
prev_tot AS (SELECT query, SUM(imp) AS t FROM prev GROUP BY query)
SELECT curr.query, curr.dim,
       (curr.imp * 1.0) / NULLIF(curr_tot.t, 0) AS curr_share,
       (COALESCE(prev.imp, 0) * 1.0) / NULLIF(prev_tot.t, 0) AS prev_share
FROM curr
JOIN curr_tot USING(query)
LEFT JOIN prev ON prev.query = curr.query AND prev.dim = curr.dim
LEFT JOIN prev_tot ON prev_tot.query = curr.query
ORDER BY curr.query, curr.dim`, groupKey, groupKey)
			rows, err := s.DB().QueryContext(cmd.Context(), query,
				cf.site, start, end, cf.site, priorStart, priorEnd)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type shift struct {
				Query string  `json:"query"`
				Dim   string  `json:"dim"`
				Curr  float64 `json:"curr_share"`
				Prev  float64 `json:"prev_share"`
				Delta float64 `json:"delta"`
			}
			out := []shift{}
			for rows.Next() {
				var sh shift
				var prev *float64
				if err := rows.Scan(&sh.Query, &sh.Dim, &sh.Curr, &prev); err != nil {
					return apiErr(err)
				}
				if prev != nil {
					sh.Prev = *prev
				}
				sh.Delta = sh.Curr - sh.Prev
				if math.Abs(sh.Delta) >= minShift {
					out = append(out, sh)
				}
			}
			return emit(cmd, flags, map[string]any{
				"site":       cf.site,
				"window":     fmt.Sprintf("%dd", days),
				"by":         by,
				"date_range": fmt.Sprintf("%s..%s vs %s..%s", start, end, priorStart, priorEnd),
				"min_shift":  minShift,
				"rows":       out,
			})
		},
	}
	cmd.Flags().StringVar(&cf.site, "site", "", "Site URL (e.g. sc-domain:example.com). Required.")
	cmd.Flags().StringVar(&cf.window, "window", "7d", "Analysis window: Nd, Nw, or Nm.")
	cmd.Flags().StringVar(&cf.db, "db", "", "SQLite path (default ~/.config/google-search-console-pp-cli/store.sqlite).")
	cmd.Flags().StringVar(&by, "by", "country", "Dimension(s) to pivot on: country, device, or country,device.")
	cmd.Flags().Float64Var(&minShift, "min-shift", 0.05, "Skip rows whose share delta is below this absolute value (0.05 = 5 percentage points).")
	return cmd
}

func buildGroupKey(by string) (string, error) {
	switch strings.ReplaceAll(strings.ToLower(strings.TrimSpace(by)), " ", "") {
	case "country":
		return "country", nil
	case "device":
		return "device", nil
	case "country,device", "device,country":
		return "country || ':' || device", nil
	}
	return "", fmt.Errorf("--by must be one of: country, device, country,device")
}

// -------- appearance: searchAppearance breakdown --------------------------

func newAppearanceCmd(flags *rootFlags) *cobra.Command {
	cf := commonFlags{}
	var vs string
	cmd := &cobra.Command{
		Use:   "appearance",
		Short: "Per-page or per-query searchAppearance breakdown across windows",
		Long: strings.TrimSpace(`
Reads stored searchAppearance dimension snapshots. Because GSC requires
searchAppearance to be queried alone (mutually exclusive with other
dimensions), only a CLI with persistent storage can compare windows.

Run sync with --type web first to populate the base data; appearance values
are recorded as '' when not requested. If your store has no rows with a
non-empty search_appearance, this command emits an empty envelope.
`),
		Example:     "  google-search-console-pp-cli appearance --site sc-domain:example.com --window 28d --vs prior --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := requireSite(cf.site); err != nil {
				return err
			}
			s, err := openStoreFromFlag(cmd.Context(), cf.db)
			if err != nil {
				return err
			}
			defer s.Close()
			days := parseWindow(cf.window, 28)
			start, end := dateRange(days)
			priorEnd := mustAddDays(start, -1)
			priorStart := mustAddDays(priorEnd, -days+1)
			rows, err := s.DB().QueryContext(cmd.Context(), `
WITH curr AS (
  SELECT search_appearance AS sa, SUM(clicks) AS clicks, SUM(impressions) AS impressions
  FROM search_analytics_rows
  WHERE site_url = ? AND date BETWEEN ? AND ? AND search_appearance <> ''
  GROUP BY sa
),
prev AS (
  SELECT search_appearance AS sa, SUM(clicks) AS clicks, SUM(impressions) AS impressions
  FROM search_analytics_rows
  WHERE site_url = ? AND date BETWEEN ? AND ? AND search_appearance <> ''
  GROUP BY sa
)
SELECT COALESCE(curr.sa, prev.sa) AS appearance,
       COALESCE(curr.clicks, 0) AS curr_clicks,
       COALESCE(prev.clicks, 0) AS prev_clicks,
       COALESCE(curr.impressions, 0) AS curr_impressions,
       COALESCE(prev.impressions, 0) AS prev_impressions
FROM curr LEFT JOIN prev USING(sa)
UNION
SELECT prev.sa, 0, prev.clicks, 0, prev.impressions
FROM prev LEFT JOIN curr USING(sa) WHERE curr.sa IS NULL
ORDER BY curr_impressions DESC`,
				cf.site, start, end, cf.site, priorStart, priorEnd)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			data, err := rowsToMaps(rows)
			if err != nil {
				return apiErr(err)
			}
			if len(data) == 0 {
				return emit(cmd, flags, map[string]any{
					"site": cf.site, "window": fmt.Sprintf("%dd", days),
					"date_range": fmt.Sprintf("%s..%s vs %s..%s", start, end, priorStart, priorEnd),
					"rows":       []any{},
					"note":       "No searchAppearance rows in store. Run a separate sync of the searchAppearance dimension to populate.",
				})
			}
			_ = vs // window vs prior is implicit from the query above
			return emit(cmd, flags, map[string]any{
				"site":       cf.site,
				"window":     fmt.Sprintf("%dd", days),
				"date_range": fmt.Sprintf("%s..%s vs %s..%s", start, end, priorStart, priorEnd),
				"rows":       data,
			})
		},
	}
	cmd.Flags().StringVar(&cf.site, "site", "", "Site URL (e.g. sc-domain:example.com). Required.")
	cmd.Flags().StringVar(&cf.window, "window", "28d", "Analysis window: Nd, Nw, or Nm.")
	cmd.Flags().StringVar(&cf.db, "db", "", "SQLite path (default ~/.config/google-search-console-pp-cli/store.sqlite).")
	cmd.Flags().StringVar(&vs, "vs", "prior", "Comparison baseline: 'prior' (the matching prior window) is the only supported value today.")
	return cmd
}

// -------- helpers ---------------------------------------------------------

func splitNonEmpty(s, sep string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, sep)
	out := parts[:0]
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int64:
		return float64(x), true
	case int:
		return float64(x), true
	case string:
		var f float64
		_, err := fmt.Sscanf(x, "%f", &f)
		return f, err == nil
	}
	return 0, false
}

func sortByFloatDesc(rows []map[string]any, key string) {
	// insertion sort to avoid pulling in sort.Slice indirection cost on tiny lists
	for i := 1; i < len(rows); i++ {
		j := i
		for j > 0 {
			a, _ := toFloat(rows[j-1][key])
			b, _ := toFloat(rows[j][key])
			if a >= b {
				break
			}
			rows[j-1], rows[j] = rows[j], rows[j-1]
			j--
		}
	}
}

// linearFit returns slope, intercept of simple least-squares regression.
func linearFit(xs, ys []float64) (slope, intercept float64) {
	if len(xs) < 2 {
		return 0, 0
	}
	var sumX, sumY, sumXY, sumXX float64
	n := float64(len(xs))
	for i, x := range xs {
		sumX += x
		sumY += ys[i]
		sumXY += x * ys[i]
		sumXX += x * x
	}
	denom := n*sumXX - sumX*sumX
	if denom == 0 {
		return 0, sumY / n
	}
	slope = (n*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / n
	return slope, intercept
}

func dateToEpochDays(date string) int {
	t, err := timeParse("2006-01-02", date)
	if err != nil {
		return 0
	}
	return int(t.Unix() / 86400)
}

func mustAddDays(date string, delta int) string {
	t, err := timeParse("2006-01-02", date)
	if err != nil {
		return date
	}
	return t.AddDate(0, 0, delta).Format("2006-01-02")
}

func round3(f float64) float64 {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return f
	}
	return math.Round(f*1000) / 1000
}

// sanitizeFloat replaces +Inf with a large sentinel for JSON safety.
func sanitizeFloat(f float64) any {
	if math.IsInf(f, 1) {
		return "+Inf"
	}
	if math.IsInf(f, -1) {
		return "-Inf"
	}
	if math.IsNaN(f) {
		return nil
	}
	return round3(f)
}

func directionOf(lift float64) string {
	if math.IsInf(lift, 1) || lift > 0 {
		return "rising"
	}
	if lift < 0 {
		return "falling"
	}
	return "flat"
}

func ifString(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
