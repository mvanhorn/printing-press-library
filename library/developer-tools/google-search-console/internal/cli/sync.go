// Sync the live Search Console API into the local SQLite store. The store
// powers every transcendence command (book, cannibalize, decay,
// coverage-drift, opportunity, momentum, new-queries, territory,
// appearance, sitemap-health, triage). Without sync, those commands have
// nothing to query.
package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/google-search-console/internal/store"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var (
		siteURL    string
		allSites   bool
		backfill   string
		searchType string
		dbPath     string
	)
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Pull search analytics, sites, and sitemaps into the local store",
		Long: strings.TrimSpace(`
Sync writes search-analytics rows (date × query × page × country × device),
plus sites and sitemaps snapshots, into the local SQLite store. The store
powers every transcendence command (book, cannibalize, decay, coverage-drift,
opportunity, momentum, new-queries, territory, appearance, sitemap-health,
triage).

GSC search analytics finalize ~3 days late, so sync writes through today-3
by default.

Examples:
  google-search-console-pp-cli sync --site sc-domain:example.com --backfill 28d
  google-search-console-pp-cli sync --all-sites --backfill 7d
`),
		Example: "  google-search-console-pp-cli sync --site sc-domain:example.com --backfill 90d",
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if siteURL == "" && !allSites {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			s, err := store.Open(cmd.Context(), dbPath)
			if err != nil {
				return configErr(err)
			}
			defer s.Close()

			window, err := parseBackfillWindow(backfill)
			if err != nil {
				return usageErr(err)
			}
			startDate := window.start.Format("2006-01-02")
			endDate := window.end.Format("2006-01-02")

			// Resolve target sites.
			targets := []string{}
			if allSites {
				sites, err := fetchSiteURLs(c)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				targets = sites
			} else {
				targets = []string{siteURL}
			}

			summary := []map[string]any{}
			started := time.Now().UTC().Format(time.RFC3339)

			// Snapshot sites once for the run.
			if siteRows, err := fetchSiteSnapshot(c); err == nil {
				_ = s.SnapshotSites(cmd.Context(), started, siteRows)
			}

			for _, t := range targets {
				rows, err := pullSearchAnalytics(c, t, startDate, endDate, searchType)
				if err != nil {
					summary = append(summary, map[string]any{"site": t, "status": "error", "error": err.Error()})
					continue
				}
				written, werr := s.UpsertAnalytics(cmd.Context(), rows)
				if werr != nil {
					summary = append(summary, map[string]any{"site": t, "status": "error", "error": werr.Error()})
					continue
				}
				_ = s.RecordSyncRun(cmd.Context(), started, time.Now().UTC().Format(time.RFC3339),
					t, "search_analytics", int64(written),
					fmt.Sprintf("%s..%s type=%s", startDate, endDate, searchType))
				// Sitemaps snapshot per site.
				if smRows, err := pullSitemaps(c, t); err == nil {
					_ = s.SnapshotSitemaps(cmd.Context(), started, smRows)
				}
				summary = append(summary, map[string]any{
					"site":       t,
					"status":     "ok",
					"date_range": fmt.Sprintf("%s..%s", startDate, endDate),
					"rows":       written,
				})
			}

			out := map[string]any{
				"started_at": started,
				"db_path":    s.Path,
				"sites":      summary,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&siteURL, "site", "", "Site URL (e.g. sc-domain:example.com or https://example.com/). Required unless --all-sites.")
	cmd.Flags().BoolVar(&allSites, "all-sites", false, "Sync every verified property in the account.")
	cmd.Flags().StringVar(&backfill, "backfill", "28d", "Window to fetch, e.g. 7d, 28d, 12w, 16m. Default 28d.")
	cmd.Flags().StringVar(&searchType, "type", "web", "Search type: web, image, video, news, discover, googleNews.")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite path (default ~/.config/google-search-console-pp-cli/store.sqlite or $GOOGLE_SEARCH_CONSOLE_DB_PATH).")
	return cmd
}

type dateWindow struct{ start, end time.Time }

// parseBackfillWindow understands "Nd", "Nw", "Nm". GSC retains ~16 months;
// we cap at 480 days. End date is today-3 (data finalizes late).
func parseBackfillWindow(spec string) (dateWindow, error) {
	if spec == "" {
		spec = "28d"
	}
	unit := spec[len(spec)-1]
	mag := spec[:len(spec)-1]
	var days int
	switch unit {
	case 'd', 'D':
		fmt.Sscanf(mag, "%d", &days)
	case 'w', 'W':
		var n int
		fmt.Sscanf(mag, "%d", &n)
		days = n * 7
	case 'm', 'M':
		var n int
		fmt.Sscanf(mag, "%d", &n)
		days = n * 30
	default:
		return dateWindow{}, fmt.Errorf("unrecognized backfill window %q (use Nd, Nw, or Nm)", spec)
	}
	if days <= 0 {
		return dateWindow{}, fmt.Errorf("backfill window must be positive, got %q", spec)
	}
	if days > 480 {
		days = 480
	}
	end := time.Now().UTC().AddDate(0, 0, -3) // GSC data finalizes ~3 days late
	start := end.AddDate(0, 0, -days+1)
	return dateWindow{start: start, end: end}, nil
}

// fetchSiteURLs returns the list of verified site URLs.
func fetchSiteURLs(c apiClient) ([]string, error) {
	data, err := c.Get("/webmasters/v3/sites", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		SiteEntry []struct {
			SiteURL string `json:"siteUrl"`
		} `json:"siteEntry"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing sites: %w", err)
	}
	out := make([]string, 0, len(resp.SiteEntry))
	for _, s := range resp.SiteEntry {
		out = append(out, s.SiteURL)
	}
	return out, nil
}

func fetchSiteSnapshot(c apiClient) ([]store.SiteRow, error) {
	data, err := c.Get("/webmasters/v3/sites", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		SiteEntry []struct {
			SiteURL         string `json:"siteUrl"`
			PermissionLevel string `json:"permissionLevel"`
		} `json:"siteEntry"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	out := make([]store.SiteRow, 0, len(resp.SiteEntry))
	for _, s := range resp.SiteEntry {
		out = append(out, store.SiteRow{SiteURL: s.SiteURL, PermissionLevel: s.PermissionLevel})
	}
	return out, nil
}

// pullSearchAnalytics fetches paginated rows for the date window with the
// canonical 5-dimension breakdown (date, query, page, country, device).
// `searchAppearance` is mutually exclusive with other dimensions in a
// single query, so it is handled by the `appearance` command on a separate
// sync path.
func pullSearchAnalytics(c apiClient, site, start, end, searchType string) ([]store.AnalyticsRow, error) {
	const pageSize = 25000
	dimensions := []string{"date", "query", "page", "country", "device"}
	all := []store.AnalyticsRow{}
	startRow := 0
	for {
		body := map[string]any{
			"startDate":  start,
			"endDate":    end,
			"dimensions": dimensions,
			"rowLimit":   pageSize,
			"startRow":   startRow,
			"searchType": searchType,
			"dataState":  "final",
		}
		path := "/webmasters/v3/sites/" + url.PathEscape(site) + "/searchAnalytics/query"
		raw, _, err := c.Post(path, body)
		if err != nil {
			return all, err
		}
		var resp struct {
			Rows []struct {
				Keys        []string `json:"keys"`
				Clicks      float64  `json:"clicks"`
				Impressions float64  `json:"impressions"`
				CTR         float64  `json:"ctr"`
				Position    float64  `json:"position"`
			} `json:"rows"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return all, fmt.Errorf("parsing analytics: %w", err)
		}
		for _, r := range resp.Rows {
			ar := store.AnalyticsRow{
				SiteURL: site, SearchType: searchType,
				Clicks: r.Clicks, Impressions: r.Impressions, CTR: r.CTR, Position: r.Position,
			}
			for i, key := range r.Keys {
				switch dimensions[i] {
				case "date":
					ar.Date = key
				case "query":
					ar.Query = key
				case "page":
					ar.Page = key
				case "country":
					ar.Country = key
				case "device":
					ar.Device = key
				}
			}
			all = append(all, ar)
		}
		if len(resp.Rows) < pageSize {
			return all, nil
		}
		startRow += pageSize
		if startRow >= 250000 {
			return all, nil // hard ceiling: 10 pages × 25k rows
		}
	}
}

func pullSitemaps(c apiClient, site string) ([]store.SitemapRow, error) {
	data, err := c.Get("/webmasters/v3/sites/"+url.PathEscape(site)+"/sitemaps", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Sitemap []struct {
			Path            string `json:"path"`
			LastSubmitted   string `json:"lastSubmitted"`
			LastDownloaded  string `json:"lastDownloaded"`
			IsPending       bool   `json:"isPending"`
			IsSitemapsIndex bool   `json:"isSitemapsIndex"`
			Errors          int64  `json:"errors,string"`
			Warnings        int64  `json:"warnings,string"`
			Contents        []struct {
				Type      string `json:"type"`
				Submitted string `json:"submitted,string"`
				Indexed   string `json:"indexed,string"`
			} `json:"contents"`
		} `json:"sitemap"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	out := make([]store.SitemapRow, 0, len(resp.Sitemap))
	for _, sm := range resp.Sitemap {
		contents := []string{}
		for _, c := range sm.Contents {
			contents = append(contents, fmt.Sprintf("%s:%s/%s", c.Type, c.Submitted, c.Indexed))
		}
		out = append(out, store.SitemapRow{
			SiteURL: site, FeedPath: sm.Path,
			LastSubmitted: sm.LastSubmitted, LastDownloaded: sm.LastDownloaded,
			IsPending: sm.IsPending, IsSitemapsIndex: sm.IsSitemapsIndex,
			Errors: sm.Errors, Warnings: sm.Warnings,
			Contents: strings.Join(contents, ";"),
		})
	}
	return out, nil
}

// apiClient narrows what we use of *client.Client so transcendence helpers
// can stub it in tests if needed later.
type apiClient interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
	Post(path string, body any) (json.RawMessage, int, error)
}
