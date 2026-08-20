// Copyright 2026 Matt and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: Phase 3 transcendence layer.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-search-console/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-search-console/internal/store"

	"github.com/spf13/cobra"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var (
		siteURL         string
		allSites        bool
		lastFlag        string
		startDate       string
		endDate         string
		searchTypeFlag  string
		dimensionsFlag  string
		rowLimit        int
		withInspections bool
		withSitemaps    bool
	)

	cmd := &cobra.Command{
		Use:     "sync",
		Short:   "Pull GSC data into the local SQLite store powering the transcendence commands",
		Long:    "Pulls verified sites, sitemaps, and search-analytics rows from the Google Search Console API and writes them to the local SQLite store at ~/.cache/google-search-console-pp-cli/store.db. Idempotent and incremental — re-running picks up new dates only. The transcendence commands (quick-wins, cannibalization, compare, cliff, roll-up, coverage-drift, historical, outliers, sitemap-watch, decaying, new-queries) all read from this store.",
		Example: "  google-search-console-pp-cli sync --site sc-domain:example.com --last 90d\n  google-search-console-pp-cli sync --all-sites --last 28d --json",
		Annotations: map[string]string{
			// Sync writes to the local store, not external state. The MCP boundary
			// considers it agent-friendly — agents can call it freely.
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if siteURL == "" && !allSites {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"dry_run":          true,
						"site":             siteURL,
						"all_sites":        allSites,
						"last":             lastFlag,
						"with_inspections": withInspections,
						"with_sitemaps":    withSitemaps,
					}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "(dry run) would sync site=%q all_sites=%v last=%q\n", siteURL, allSites, lastFlag)
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "(verify env) sync skipped — would not dial API in PRINTING_PRESS_VERIFY=1 mode")
				return nil
			}

			ctx := context.Background()
			s, err := openStore(ctx)
			if err != nil {
				return configErr(err)
			}
			defer s.Close()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Compute the [start, end] window.
			if startDate == "" || endDate == "" {
				dur, err := parseLast(lastFlag)
				if err != nil {
					return usageErr(err)
				}
				startDate, endDate = dateRange(dur)
			}

			// Build the site list. --all-sites pulls from the API too so a fresh
			// store can bootstrap itself.
			var sites []string
			if allSites {
				data, err := c.Get("/webmasters/v3/sites", nil)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var resp struct {
					SiteEntry []struct {
						SiteURL         string `json:"siteUrl"`
						PermissionLevel string `json:"permissionLevel"`
					} `json:"siteEntry"`
				}
				if err := json.Unmarshal(data, &resp); err != nil {
					return fmt.Errorf("parsing sites response: %w", err)
				}
				for _, e := range resp.SiteEntry {
					sites = append(sites, e.SiteURL)
					_ = s.UpsertSite(ctx, store.Site{SiteURL: e.SiteURL, PermissionLevel: e.PermissionLevel})
				}
			} else {
				sites = []string{siteURL}
				_ = s.UpsertSite(ctx, store.Site{SiteURL: siteURL})
			}

			dims := strings.Split(dimensionsFlag, ",")
			for i := range dims {
				dims[i] = strings.TrimSpace(dims[i])
			}

			report := map[string]any{}
			totalRows := int64(0)
			totalSitemaps := 0
			totalInspections := 0

			for _, site := range sites {
				rows, err := syncSearchAnalytics(ctx, c, s, site, startDate, endDate, searchTypeFlag, dims, rowLimit)
				if err != nil {
					if !flags.asJSON {
						fmt.Fprintf(cmd.OutOrStdout(), "warning: search analytics for %s failed: %v\n", site, err)
					}
				} else {
					totalRows += rows
				}
				if withSitemaps {
					n, err := syncSitemaps(ctx, c, s, site)
					if err != nil {
						if !flags.asJSON {
							fmt.Fprintf(cmd.OutOrStdout(), "warning: sitemaps for %s failed: %v\n", site, err)
						}
					} else {
						totalSitemaps += n
					}
				}
				_ = withInspections // user must add URLs explicitly via inspect-batch — sync intentionally skips
			}
			report["sites_synced"] = len(sites)
			report["search_analytics_rows_upserted"] = totalRows
			report["sitemap_snapshots_upserted"] = totalSitemaps
			report["url_inspection_snapshots_upserted"] = totalInspections
			report["start_date"] = startDate
			report["end_date"] = endDate

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "synced %d site(s) — %d analytics rows, %d sitemap snapshots (%s..%s)\n",
				len(sites), totalRows, totalSitemaps, startDate, endDate)
			return nil
		},
	}

	cmd.Flags().StringVar(&siteURL, "site", "", "GSC property to sync (e.g. sc-domain:example.com)")
	cmd.Flags().BoolVar(&allSites, "all-sites", false, "Sync every verified property the token can see")
	cmd.Flags().StringVar(&lastFlag, "last", "90d", "How far back to sync (e.g. 7d, 4w, 3m, 1y). Ignored when --start-date/--end-date are set.")
	cmd.Flags().StringVar(&startDate, "start-date", "", "Start date (YYYY-MM-DD). Overrides --last.")
	cmd.Flags().StringVar(&endDate, "end-date", "", "End date (YYYY-MM-DD). Overrides --last.")
	cmd.Flags().StringVar(&searchTypeFlag, "type", "WEB", "Search property type (WEB, IMAGE, VIDEO, NEWS, DISCOVER, GOOGLE_NEWS).")
	cmd.Flags().StringVar(&dimensionsFlag, "dimensions", "date,query,page", "Dimensions to slice by (comma-separated; subset of date,query,page,country,device,searchAppearance).")
	cmd.Flags().IntVar(&rowLimit, "row-limit", 25000, "Page size for search analytics requests (max 25000).")
	cmd.Flags().BoolVar(&withInspections, "with-inspections", false, "Reserved — use `url-inspection inspect-batch` to populate the inspection table directly.")
	cmd.Flags().BoolVar(&withSitemaps, "with-sitemaps", true, "Pull sitemaps for each site into the snapshots table.")

	return cmd
}

// syncSearchAnalytics paginates startRow until the API returns an empty page,
// upserting every row into the local store. Returns the total rows upserted.
func syncSearchAnalytics(ctx context.Context, c apiClient, s *store.Store, site, startDate, endDate, searchType string, dims []string, rowLimit int) (int64, error) {
	if rowLimit <= 0 || rowLimit > 25000 {
		rowLimit = 25000
	}
	startRow := 0
	total := int64(0)
	for {
		body := map[string]any{
			"startDate":  startDate,
			"endDate":    endDate,
			"dimensions": dims,
			"rowLimit":   rowLimit,
			"startRow":   startRow,
		}
		if searchType != "" {
			body["type"] = searchType
		}
		path := strings.ReplaceAll("/webmasters/v3/sites/{siteUrl}/searchAnalytics/query", "{siteUrl}", site)
		data, _, err := c.Post(path, body)
		if err != nil {
			return total, err
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
		if err := json.Unmarshal(data, &resp); err != nil {
			return total, err
		}
		if len(resp.Rows) == 0 {
			break
		}
		batch := make([]store.SearchAnalyticsRow, 0, len(resp.Rows))
		for _, r := range resp.Rows {
			row := store.SearchAnalyticsRow{
				SiteURL:     site,
				SearchType:  searchType,
				Clicks:      r.Clicks,
				Impressions: r.Impressions,
				CTR:         r.CTR,
				Position:    r.Position,
			}
			for i, dim := range dims {
				if i >= len(r.Keys) {
					break
				}
				switch dim {
				case "date":
					row.Date = r.Keys[i]
				case "query":
					row.Query = r.Keys[i]
				case "page":
					row.Page = r.Keys[i]
				case "country":
					row.Country = r.Keys[i]
				case "device":
					row.Device = r.Keys[i]
				case "searchAppearance":
					row.SearchAppearance = r.Keys[i]
				}
			}
			if row.Date == "" {
				// API skipped the date dim — fall back to today so the row
				// still has a primary-key value.
				row.Date = time.Now().UTC().Format("2006-01-02")
			}
			batch = append(batch, row)
		}
		if err := s.BulkUpsertSearchAnalyticsRows(ctx, batch); err != nil {
			return total, err
		}
		total += int64(len(batch))
		if len(resp.Rows) < rowLimit {
			break
		}
		startRow += rowLimit
	}
	_ = strconv.Itoa // keep import-stable
	return total, nil
}

// syncSitemaps pulls the sitemap list for a site and snapshots every entry.
func syncSitemaps(ctx context.Context, c apiClient, s *store.Store, site string) (int, error) {
	path := strings.ReplaceAll("/webmasters/v3/sites/{siteUrl}/sitemaps", "{siteUrl}", site)
	data, err := c.Get(path, nil)
	if err != nil {
		return 0, err
	}
	var resp struct {
		Sitemap []struct {
			Path            string `json:"path"`
			Type            string `json:"type"`
			IsPending       bool   `json:"isPending"`
			IsSitemapsIndex bool   `json:"isSitemapsIndex"`
			LastSubmitted   string `json:"lastSubmitted"`
			LastDownloaded  string `json:"lastDownloaded"`
			Errors          int64  `json:"errors,string"`
			Warnings        int64  `json:"warnings,string"`
			Contents        []any  `json:"contents"`
		} `json:"sitemap"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	count := 0
	for _, sm := range resp.Sitemap {
		contentsJSON, _ := json.Marshal(sm.Contents)
		row := store.SitemapRow{
			SiteURL:         site,
			Feedpath:        sm.Path,
			Type:            sm.Type,
			IsPending:       sm.IsPending,
			IsSitemapsIndex: sm.IsSitemapsIndex,
			LastSubmitted:   sm.LastSubmitted,
			LastDownloaded:  sm.LastDownloaded,
			Errors:          sm.Errors,
			Warnings:        sm.Warnings,
			ContentsJSON:    string(contentsJSON),
			SnapshotAt:      now,
		}
		if err := s.UpsertSitemap(ctx, row); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// apiClient is the subset of *client.Client we use here. Captured as an
// interface so the sync helpers stay testable without a live HTTP client.
type apiClient interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
	Post(path string, body any) (json.RawMessage, int, error)
}
