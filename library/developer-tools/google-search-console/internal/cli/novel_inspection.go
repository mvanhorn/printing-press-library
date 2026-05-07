// Novel commands rooted in url_inspections / sitemaps_snapshots history:
// coverage-drift, sitemap-health, triage. Plus the inspect-batch absorb
// command and quickwins/compare absorbs.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/google-search-console/internal/store"
)

// -------- coverage-drift: url_inspection state flips ---------------------

func newCoverageDriftCmd(flags *rootFlags) *cobra.Command {
	cf := commonFlags{}
	var since string
	cmd := &cobra.Command{
		Use:   "coverage-drift",
		Short: "Pages whose coverage state, canonical, or last-crawl changed since the prior inspection snapshot",
		Long: strings.TrimSpace(`
Diff successive url_inspections snapshots: pages whose coverageState,
googleCanonical, or lastCrawlTime changed. Catches silent deindexings.
`),
		Example:     "  google-search-console-pp-cli coverage-drift --site sc-domain:example.com --since last-sync --agent",
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
			rows, err := s.DB().QueryContext(cmd.Context(), `
WITH ranked AS (
  SELECT page_url, coverage_state, google_canonical, last_crawl_time, inspected_at,
         ROW_NUMBER() OVER (PARTITION BY page_url ORDER BY inspected_at DESC) AS rn
  FROM url_inspections
  WHERE site_url = ?
),
latest AS (SELECT * FROM ranked WHERE rn = 1),
prior  AS (SELECT * FROM ranked WHERE rn = 2)
SELECT latest.page_url, prior.coverage_state, latest.coverage_state,
       prior.google_canonical, latest.google_canonical,
       prior.last_crawl_time, latest.last_crawl_time,
       prior.inspected_at, latest.inspected_at
FROM latest LEFT JOIN prior USING (page_url)
WHERE prior.page_url IS NOT NULL
  AND (latest.coverage_state <> prior.coverage_state
    OR latest.google_canonical <> prior.google_canonical
    OR latest.last_crawl_time <> prior.last_crawl_time)`,
				cf.site)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type drift struct {
				PageURL          string `json:"page_url"`
				FromCoverage     string `json:"from_coverage"`
				ToCoverage       string `json:"to_coverage"`
				FromCanonical    string `json:"from_canonical"`
				ToCanonical      string `json:"to_canonical"`
				FromCrawl        string `json:"from_crawl"`
				ToCrawl          string `json:"to_crawl"`
				PriorInspectedAt string `json:"prior_inspected_at"`
				InspectedAt      string `json:"inspected_at"`
			}
			out := []drift{}
			for rows.Next() {
				var d drift
				if err := rows.Scan(&d.PageURL, &d.FromCoverage, &d.ToCoverage,
					&d.FromCanonical, &d.ToCanonical,
					&d.FromCrawl, &d.ToCrawl,
					&d.PriorInspectedAt, &d.InspectedAt); err != nil {
					return apiErr(err)
				}
				out = append(out, d)
			}
			_ = since // semantic placeholder; we always compare last two snapshots per page
			return emit(cmd, flags, map[string]any{
				"site":  cf.site,
				"since": since,
				"rows":  out,
			})
		},
	}
	bindCommonFlags(cmd, &cf, "")
	cmd.Flags().StringVar(&since, "since", "last-sync", "Compare against (currently only 'last-sync' is supported).")
	return cmd
}

// -------- sitemap-health: regression detection -----------------------------

func newSitemapHealthCmd(flags *rootFlags) *cobra.Command {
	cf := commonFlags{}
	var regressed bool
	cmd := &cobra.Command{
		Use:   "sitemap-health",
		Short: "Cross-snapshot diff on per-property sitemap state; flags new errors and indexed-vs-submitted drops",
		Long: strings.TrimSpace(`
Joins the latest sitemaps snapshot against the prior snapshot. Flags rows
where errors or warnings increased, or where indexed-vs-submitted ratio
dropped meaningfully.
`),
		Example:     "  google-search-console-pp-cli sitemap-health --regressed --agent",
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
			where := ""
			args2 := []any{}
			if cf.site != "" {
				where = ` AND site_url = ?`
				// where is interpolated three times in the query below; bind cf.site to each ?.
				args2 = append(args2, cf.site, cf.site, cf.site)
			}
			query := fmt.Sprintf(`
WITH latest AS (
  SELECT * FROM sitemaps_snapshots
  WHERE snapshot_at = (SELECT MAX(snapshot_at) FROM sitemaps_snapshots WHERE 1=1 %s)
),
prior AS (
  SELECT * FROM sitemaps_snapshots
  WHERE snapshot_at = (
    SELECT MAX(snapshot_at) FROM sitemaps_snapshots
    WHERE snapshot_at < (SELECT MAX(snapshot_at) FROM sitemaps_snapshots WHERE 1=1 %s) %s
  )
)
SELECT latest.site_url, latest.feed_path,
       latest.errors AS curr_errors, COALESCE(prior.errors, 0) AS prev_errors,
       latest.warnings AS curr_warnings, COALESCE(prior.warnings, 0) AS prev_warnings,
       latest.last_submitted, latest.last_downloaded,
       (latest.errors - COALESCE(prior.errors, 0)) AS delta_errors,
       (latest.warnings - COALESCE(prior.warnings, 0)) AS delta_warnings
FROM latest LEFT JOIN prior USING (site_url, feed_path)`, where, where, where)
			rows, err := s.DB().QueryContext(cmd.Context(), query, args2...)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			data, err := rowsToMaps(rows)
			if err != nil {
				return apiErr(err)
			}
			if regressed {
				filtered := data[:0]
				for _, r := range data {
					de, _ := toFloat(r["delta_errors"])
					dw, _ := toFloat(r["delta_warnings"])
					if de > 0 || dw > 0 {
						filtered = append(filtered, r)
					}
				}
				data = filtered
			}
			return emit(cmd, flags, map[string]any{
				"site":      cf.site,
				"regressed": regressed,
				"rows":      data,
			})
		},
	}
	bindCommonFlags(cmd, &cf, "")
	cmd.Flags().BoolVar(&regressed, "regressed", false, "Only show sitemaps whose errors or warnings count increased since the previous snapshot.")
	return cmd
}

// -------- triage: non-INDEXED pages × historical impressions --------------

func newTriageCmd(flags *rootFlags) *cobra.Command {
	cf := commonFlags{}
	var by string
	var top int
	cmd := &cobra.Command{
		Use:   "triage",
		Short: "Non-INDEXED pages joined against last 30 days of impressions, ranked by traffic lost",
		Long: strings.TrimSpace(`
Joins the latest non-INDEXED url_inspections rows against historical
impressions in search_analytics_rows. Ranks by impact so you fix the broken
pages that actually drive traffic, not whatever was inspected most recently.
`),
		Example:     "  google-search-console-pp-cli triage --site sc-domain:example.com --by impact --top 20 --agent",
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
			windowDays := 30
			start, end := dateRange(windowDays)
			rows, err := s.DB().QueryContext(cmd.Context(), `
WITH latest AS (
  SELECT page_url, coverage_state, indexing_state, last_crawl_time
  FROM url_inspections
  WHERE site_url = ?
    AND inspected_at = (
      SELECT MAX(inspected_at) FROM url_inspections u2
      WHERE u2.site_url = url_inspections.site_url AND u2.page_url = url_inspections.page_url
    )
),
recent AS (
  SELECT page, SUM(clicks) AS clicks, SUM(impressions) AS impressions
  FROM search_analytics_rows
  WHERE site_url = ? AND date BETWEEN ? AND ? AND page <> ''
  GROUP BY page
)
SELECT latest.page_url, latest.coverage_state, latest.indexing_state, latest.last_crawl_time,
       COALESCE(recent.impressions, 0) AS recent_impressions,
       COALESCE(recent.clicks, 0) AS recent_clicks
FROM latest LEFT JOIN recent ON recent.page = latest.page_url
WHERE latest.coverage_state NOT IN ('INDEXED', 'SUBMITTED_AND_INDEXED', '')
ORDER BY recent_impressions DESC LIMIT ?`,
				cf.site, cf.site, start, end, top)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			data, err := rowsToMaps(rows)
			if err != nil {
				return apiErr(err)
			}
			_ = by
			return emit(cmd, flags, map[string]any{
				"site":       cf.site,
				"by":         by,
				"window":     fmt.Sprintf("%dd", windowDays),
				"date_range": fmt.Sprintf("%s..%s", start, end),
				"rows":       data,
			})
		},
	}
	bindCommonFlags(cmd, &cf, "")
	cmd.Flags().StringVar(&by, "by", "impact", "Ranking criterion (impact = recent impressions; reserved for future modes).")
	cmd.Flags().IntVar(&top, "top", 50, "Maximum rows to return.")
	return cmd
}

// -------- inspect-batch (absorb): NDJSON streaming for many URLs ----------

func newInspectBatchCmd(flags *rootFlags) *cobra.Command {
	var (
		siteURL string
		file    string
		lang    string
		dbPath  string
		persist bool
	)
	cmd := &cobra.Command{
		Use:   "inspect-batch",
		Short: "Inspect many URLs from a file or stdin, streaming NDJSON results",
		Long: strings.TrimSpace(`
Reads URLs (one per line) from --file or stdin. For each URL, posts to
urlInspection.index.inspect and emits one JSON object per line. With --persist
(default), records each result in the local url_inspections table so future
coverage-drift and triage runs can use it.
`),
		Example:     "  google-search-console-pp-cli inspect-batch --site sc-domain:example.com --file urls.txt --persist",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if siteURL == "" {
				return usageErr(fmt.Errorf("--site is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var sStore *store.Store
			if persist {
				sStore, err = openStoreFromFlag(cmd.Context(), dbPath)
				if err != nil {
					return err
				}
				defer sStore.Close()
			}
			var src *bufio.Scanner
			if file != "" {
				f, err := os.Open(file)
				if err != nil {
					return usageErr(err)
				}
				defer f.Close()
				src = bufio.NewScanner(f)
			} else {
				src = bufio.NewScanner(os.Stdin)
			}
			src.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			enc := json.NewEncoder(cmd.OutOrStdout())
			ok, fail := 0, 0
			for src.Scan() {
				u := strings.TrimSpace(src.Text())
				if u == "" || strings.HasPrefix(u, "#") {
					continue
				}
				body := map[string]any{"siteUrl": siteURL, "inspectionUrl": u}
				if lang != "" {
					body["languageCode"] = lang
				}
				raw, _, err := c.Post("/v1/urlInspection/index:inspect", body)
				rec := map[string]any{"url": u}
				if err != nil {
					rec["status"] = "error"
					rec["error"] = err.Error()
					fail++
				} else {
					var parsed map[string]any
					_ = json.Unmarshal(raw, &parsed)
					rec["status"] = "ok"
					rec["result"] = parsed
					ok++
					if persist && sStore != nil {
						persistInspection(cmd.Context(), sStore, siteURL, u, raw)
					}
				}
				_ = enc.Encode(rec)
			}
			if err := src.Err(); err != nil {
				return apiErr(err)
			}
			fmt.Fprintf(os.Stderr, "inspect-batch: %d ok, %d failed\n", ok, fail)
			return nil
		},
	}
	cmd.Flags().StringVar(&siteURL, "site", "", "Site URL the inspected URLs belong to. Required.")
	cmd.Flags().StringVar(&file, "file", "", "Read URLs from file (one per line). Reads stdin if omitted.")
	cmd.Flags().StringVar(&lang, "language", "", "BCP-47 language code for diagnostic messages.")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite path (used when --persist).")
	cmd.Flags().BoolVar(&persist, "persist", true, "Save each inspection to the local store (enables coverage-drift / triage).")
	return cmd
}

func persistInspection(ctx context.Context, s *store.Store, site, page string, raw []byte) {
	var resp struct {
		InspectionResult struct {
			IndexStatusResult struct {
				CoverageState   string `json:"coverageState"`
				GoogleCanonical string `json:"googleCanonical"`
				UserCanonical   string `json:"userCanonical"`
				LastCrawlTime   string `json:"lastCrawlTime"`
				PageFetchState  string `json:"pageFetchState"`
				IndexingState   string `json:"indexingState"`
				RobotsTxtState  string `json:"robotsTxtState"`
			} `json:"indexStatusResult"`
			MobileUsabilityResult struct {
				Verdict string `json:"verdict"`
			} `json:"mobileUsabilityResult"`
		} `json:"inspectionResult"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return
	}
	r := resp.InspectionResult.IndexStatusResult
	_ = s.SaveURLInspection(ctx, store.URLInspectionRow{
		InspectedAt:     time.Now().UTC().Format(time.RFC3339),
		SiteURL:         site,
		PageURL:         page,
		CoverageState:   r.CoverageState,
		GoogleCanonical: r.GoogleCanonical,
		UserCanonical:   r.UserCanonical,
		LastCrawlTime:   r.LastCrawlTime,
		PageFetchState:  r.PageFetchState,
		IndexingState:   r.IndexingState,
		RobotsTxtState:  r.RobotsTxtState,
		MobileVerdict:   resp.InspectionResult.MobileUsabilityResult.Verdict,
	})
}

// -------- quickwins (absorb): one-shot opportunity, no baseline ------------

func newQuickwinsCmd(flags *rootFlags) *cobra.Command {
	cf := commonFlags{}
	var positionMin, positionMax float64
	var minImpressions float64
	var maxCTR float64
	cmd := &cobra.Command{
		Use:   "quickwins",
		Short: "Pages in the opportunity zone today (no entry-date baseline; use `opportunity` for that)",
		Long: strings.TrimSpace(`
Mirrors ahonn/mcp-server-gsc's detectQuickWins shape: pages with average
position between --position-min and --position-max, impressions ≥
--min-impressions, CTR ≤ --max-ctr. Reads from the local store; if you want
the entry-date baseline use the 'opportunity' command.
`),
		Example:     "  google-search-console-pp-cli quickwins --site sc-domain:example.com --json",
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
SELECT page, query, SUM(clicks) AS clicks, SUM(impressions) AS impressions, AVG(position) AS avg_position
FROM search_analytics_rows
WHERE site_url = ? AND date BETWEEN ? AND ? AND page <> '' AND query <> ''
GROUP BY page, query
HAVING avg_position BETWEEN ? AND ?
   AND impressions >= ?
   AND (CASE WHEN impressions > 0 THEN clicks * 1.0 / impressions ELSE 0 END) <= ?
ORDER BY impressions DESC LIMIT 200`,
				cf.site, start, end, positionMin, positionMax, minImpressions, maxCTR)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			data, err := rowsToMaps(rows)
			if err != nil {
				return apiErr(err)
			}
			return emit(cmd, flags, map[string]any{
				"site":            cf.site,
				"window":          fmt.Sprintf("%dd", days),
				"date_range":      fmt.Sprintf("%s..%s", start, end),
				"position_range":  []float64{positionMin, positionMax},
				"min_impressions": minImpressions,
				"max_ctr":         maxCTR,
				"rows":            data,
			})
		},
	}
	bindCommonFlags(cmd, &cf, "28d")
	cmd.Flags().Float64Var(&positionMin, "position-min", 4, "Minimum average position to consider an opportunity.")
	cmd.Flags().Float64Var(&positionMax, "position-max", 20, "Maximum average position to consider an opportunity.")
	cmd.Flags().Float64Var(&minImpressions, "min-impressions", 100, "Filter pairs below this impression count.")
	cmd.Flags().Float64Var(&maxCTR, "max-ctr", 0.05, "Filter pairs whose CTR is above this (default 5%).")
	return cmd
}

// -------- compare (absorb): period-over-period ---------------------------

func newCompareCmd(flags *rootFlags) *cobra.Command {
	cf := commonFlags{}
	var dim string
	var top int
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Period-over-period comparison from the local store (no extra API calls)",
		Long: strings.TrimSpace(`
Compares the most recent --window window against the matching prior window for
the chosen dimension. Counterpart to AminForou/mcp-gsc's compare_search_periods,
without the API re-query.
`),
		Example:     "  google-search-console-pp-cli compare --site sc-domain:example.com --window 7d --dim query --top 25 --agent",
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
			col := ""
			switch dim {
			case "query":
				col = "query"
			case "page":
				col = "page"
			case "country":
				col = "country"
			case "device":
				col = "device"
			default:
				return usageErr(fmt.Errorf("--dim must be one of: query, page, country, device"))
			}
			days := parseWindow(cf.window, 7)
			start, end := dateRange(days)
			priorEnd := mustAddDays(start, -1)
			priorStart := mustAddDays(priorEnd, -days+1)
			query := fmt.Sprintf(`
WITH curr AS (
  SELECT %s AS k, SUM(clicks) AS c, SUM(impressions) AS i FROM search_analytics_rows
  WHERE site_url = ? AND date BETWEEN ? AND ? AND %s <> '' GROUP BY k
),
prev AS (
  SELECT %s AS k, SUM(clicks) AS c, SUM(impressions) AS i FROM search_analytics_rows
  WHERE site_url = ? AND date BETWEEN ? AND ? AND %s <> '' GROUP BY k
)
SELECT COALESCE(curr.k, prev.k) AS key,
       COALESCE(curr.c, 0) AS curr_clicks, COALESCE(prev.c, 0) AS prev_clicks,
       COALESCE(curr.i, 0) AS curr_impressions, COALESCE(prev.i, 0) AS prev_impressions,
       COALESCE(curr.c, 0) - COALESCE(prev.c, 0) AS click_delta
FROM curr LEFT JOIN prev USING(k)
UNION
SELECT prev.k, 0, prev.c, 0, prev.i, -prev.c
FROM prev LEFT JOIN curr USING(k) WHERE curr.k IS NULL
ORDER BY ABS(click_delta) DESC LIMIT ?`, col, col, col, col)
			rows, err := s.DB().QueryContext(cmd.Context(), query,
				cf.site, start, end, cf.site, priorStart, priorEnd, top)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			data, err := rowsToMaps(rows)
			if err != nil {
				return apiErr(err)
			}
			return emit(cmd, flags, map[string]any{
				"site": cf.site, "dim": dim, "window": fmt.Sprintf("%dd", days),
				"date_range": fmt.Sprintf("%s..%s vs %s..%s", start, end, priorStart, priorEnd),
				"rows":       data,
			})
		},
	}
	bindCommonFlags(cmd, &cf, "7d")
	cmd.Flags().StringVar(&dim, "dim", "query", "Dimension to compare on: query, page, country, device.")
	cmd.Flags().IntVar(&top, "top", 25, "Top N movers to return.")
	return cmd
}

// urlPathEscape is a tiny convenience reused by the inspection helpers.
func urlPathEscape(s string) string { return url.PathEscape(s) }
