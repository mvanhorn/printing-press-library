// PATCH(crawl-stats: cobra command tree for the GSC Crawl Stats UI surface).
// The Crawl Stats report is NOT in the public Search Console v1 API; this
// command tree calls the private SearchConsoleAggReportUi.batchexecute
// endpoint reverse-engineered via chrome-MCP. See
// internal/client/crawlstats.go for the HTTP client and the discovery report
// at manuscripts/google-search-console/amend-2026-05-21T1402/.
//
// Auth: cookie-jar, NOT the OAuth bearer token. Two paths:
//   1. `auth login --chrome`  (prints manual capture instructions for v0.2)
//   2. `GSC_COOKIE_JAR=<path>` env var pointing at a Netscape cookie jar
//
// All subcommands honor --json (root), --dry-run (root), and --no-persist.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-search-console/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-search-console/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-search-console/internal/store"
	"github.com/spf13/cobra"
)

// fileTypeCodes maps the human label expected on --file-type to the integer
// code Google uses internally. Pulled directly from the discovery report.
var fileTypeCodes = map[string]int{
	"html": 1, "image": 2, "video": 3, "js": 4, "javascript": 4,
	"css": 5, "pdf": 6, "other": 7, "unknown": 9,
}

// responseCodes maps --response-code values to integer codes (1-7, 9-13,
// 15-20 per the discovery report; 8 and 14 are not valid).
var responseCodes = map[string]int{
	"ok":                 1,
	"not-found":          2,
	"unauthorized":       3,
	"other-4xx":          4,
	"dns-error":          5,
	"fetch-error":        6,
	"robots-blocked":     7,
	"server-error":       9,
	"dns-unresponsive":   10,
	"robots-unavailable": 11,
	"page-unreachable":   12,
	"page-timeout":       13,
	"redirect-error":     15,
	"other-fetch-error":  16,
	"moved-permanently":  17,
	"moved-temporarily":  18,
	"moved-other":        19,
	"not-modified":       20,
}

// googlebotCodes maps --googlebot values to integer codes (1-9).
var googlebotCodes = map[string]int{
	"smartphone":    1,
	"desktop":       2,
	"image":         3,
	"video":         4,
	"page-resource": 5,
	"other":         6,
	"adsbot":        7,
	"storebot":      8,
	"amp":           9,
}

// purposeCodes maps --purpose values to integer codes (1-2).
var purposeCodes = map[string]int{
	"discovery": 1,
	"refresh":   2,
}

func newCrawlStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "crawl-stats",
		Short: "Pull Crawl Stats reports (URL samples, time-series, totals) from GSC",
		Long: `Pull Crawl Stats reports from Google Search Console.

The Crawl Stats UI (Settings > Crawl stats) exposes data that is NOT in the
public Search Console v1 API: per-URL sample lists, time-series, and
aggregate totals broken down by file type, response code, Googlebot type,
and purpose.

This command tree authenticates against Google's private internal RPC
endpoint using a cookie-jar export from your logged-in Chrome session.
The standard ` + "`auth login`" + ` OAuth flow does NOT authenticate against this
surface — you need to point the CLI at a cookie jar via GSC_COOKIE_JAR or
the cookies field in config.toml.

Run ` + "`google-search-console-pp-cli auth login --chrome`" + ` for instructions on
exporting cookies from Chrome.

Each poll caps at ~1000 sample URLs (Google's limit, no pagination). The
` + "`crawl-stats union`" + ` subcommand reads all polled samples from the local
SQLite store to give you the unioned URL set across every poll.`,
		Example: `  # Top-level list (overview — no drill-down filter)
  google-search-console-pp-cli crawl-stats list sc-domain:example.com --json

  # Drill-down by file type
  google-search-console-pp-cli crawl-stats by-type sc-domain:example.com --file-type css --top 500

  # Drill-down by response code
  google-search-console-pp-cli crawl-stats by-response sc-domain:example.com --response-code not-found --json

  # Drill-down by Googlebot crawler type
  google-search-console-pp-cli crawl-stats by-googlebot sc-domain:example.com --googlebot smartphone

  # Drill-down by purpose
  google-search-console-pp-cli crawl-stats by-purpose sc-domain:example.com --purpose discovery

  # Read the unioned sample URL set across all polls (no network call)
  google-search-console-pp-cli crawl-stats union sc-domain:example.com --file-type html`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}

	cmd.AddCommand(newCrawlStatsListCmd(flags))
	cmd.AddCommand(newCrawlStatsByTypeCmd(flags))
	cmd.AddCommand(newCrawlStatsByResponseCmd(flags))
	cmd.AddCommand(newCrawlStatsByGooglebotCmd(flags))
	cmd.AddCommand(newCrawlStatsByPurposeCmd(flags))
	cmd.AddCommand(newCrawlStatsUnionCmd(flags))
	return cmd
}

// crawlStatsCommonFlags carries the flag set every subcommand reuses.
type crawlStatsCommonFlags struct {
	top        int
	noPersist  bool
	xsrfToken  string
	buildLabel string
	sessionID  string
	cookieJar  string
	requestSeq int
}

func attachCommonCrawlStatsFlags(c *cobra.Command, f *crawlStatsCommonFlags) {
	c.Flags().IntVar(&f.top, "top", 100, "Max samples to return (Google caps at ~1000)")
	c.Flags().BoolVar(&f.noPersist, "no-persist", false, "Skip writing this poll to the local SQLite store")
	c.Flags().StringVar(&f.xsrfToken, "xsrf-token", "", "XSRF token (the `at=` form field from GSC HTML); falls back to GSC_XSRF_TOKEN env")
	c.Flags().StringVar(&f.buildLabel, "build-label", "", "Override the bl= GSC web build label (defaults to a recent pinned value)")
	c.Flags().StringVar(&f.sessionID, "session-id", "", "Optional f.sid= query parameter")
	c.Flags().StringVar(&f.cookieJar, "cookie-jar", "", "Path to Netscape cookie jar (overrides config + GSC_COOKIE_JAR env)")
	c.Flags().IntVar(&f.requestSeq, "request-seq", 1, "Monotonic _reqid= counter (auto-increments across multiple calls in one process)")
}

func newCrawlStatsListCmd(flags *rootFlags) *cobra.Command {
	common := &crawlStatsCommonFlags{}
	cmd := &cobra.Command{
		Use:         "list <property>",
		Short:       "Pull the Crawl Stats overview (no drill-down filter)",
		Long:        "Pull totals + time-series + URL samples for the overview view at GSC Settings > Crawl stats. No drill-down dimension applied.",
		Example:     "  google-search-console-pp-cli crawl-stats list sc-domain:example.com --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				return c.Help()
			}
			return runCrawlStats(c, flags, common, args[0], client.DimensionNone, 0, "")
		},
	}
	attachCommonCrawlStatsFlags(cmd, common)
	return cmd
}

func newCrawlStatsByTypeCmd(flags *rootFlags) *cobra.Command {
	common := &crawlStatsCommonFlags{}
	var fileType string
	cmd := &cobra.Command{
		Use:         "by-type <property>",
		Short:       "Pull Crawl Stats drill-down filtered by file type",
		Long:        "Drill-down by file type. Valid values: html, image, video, js (alias javascript), css, pdf, other, unknown.",
		Example:     "  google-search-console-pp-cli crawl-stats by-type sc-domain:example.com --file-type css --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				return c.Help()
			}
			code, err := lookupCode(fileTypeCodes, fileType, "--file-type", fileTypeKeys())
			if err != nil {
				return usageErr(err)
			}
			return runCrawlStats(c, flags, common, args[0], client.DimensionFileType, code, fileType)
		},
	}
	cmd.Flags().StringVar(&fileType, "file-type", "", "File type filter (required)")
	_ = cmd.MarkFlagRequired("file-type")
	attachCommonCrawlStatsFlags(cmd, common)
	return cmd
}

func newCrawlStatsByResponseCmd(flags *rootFlags) *cobra.Command {
	common := &crawlStatsCommonFlags{}
	var responseCode string
	cmd := &cobra.Command{
		Use:         "by-response <property>",
		Short:       "Pull Crawl Stats drill-down filtered by response code",
		Long:        "Drill-down by Googlebot response code. Valid values: ok, not-found, unauthorized, other-4xx, dns-error, fetch-error, robots-blocked, server-error, dns-unresponsive, robots-unavailable, page-unreachable, page-timeout, redirect-error, other-fetch-error, moved-permanently, moved-temporarily, moved-other, not-modified.",
		Example:     "  google-search-console-pp-cli crawl-stats by-response sc-domain:example.com --response-code not-found",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				return c.Help()
			}
			code, err := lookupCode(responseCodes, responseCode, "--response-code", responseKeys())
			if err != nil {
				return usageErr(err)
			}
			return runCrawlStats(c, flags, common, args[0], client.DimensionResponseCode, code, responseCode)
		},
	}
	cmd.Flags().StringVar(&responseCode, "response-code", "", "Response code filter (required)")
	_ = cmd.MarkFlagRequired("response-code")
	attachCommonCrawlStatsFlags(cmd, common)
	return cmd
}

func newCrawlStatsByGooglebotCmd(flags *rootFlags) *cobra.Command {
	common := &crawlStatsCommonFlags{}
	var googlebot string
	cmd := &cobra.Command{
		Use:         "by-googlebot <property>",
		Short:       "Pull Crawl Stats drill-down filtered by Googlebot crawler type",
		Long:        "Drill-down by Googlebot crawler type. Valid values: smartphone, desktop, image, video, page-resource, other, adsbot, storebot, amp.",
		Example:     "  google-search-console-pp-cli crawl-stats by-googlebot sc-domain:example.com --googlebot smartphone",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				return c.Help()
			}
			code, err := lookupCode(googlebotCodes, googlebot, "--googlebot", googlebotKeys())
			if err != nil {
				return usageErr(err)
			}
			return runCrawlStats(c, flags, common, args[0], client.DimensionGooglebotType, code, googlebot)
		},
	}
	cmd.Flags().StringVar(&googlebot, "googlebot", "", "Googlebot crawler type (required)")
	_ = cmd.MarkFlagRequired("googlebot")
	attachCommonCrawlStatsFlags(cmd, common)
	return cmd
}

func newCrawlStatsByPurposeCmd(flags *rootFlags) *cobra.Command {
	common := &crawlStatsCommonFlags{}
	var purpose string
	cmd := &cobra.Command{
		Use:         "by-purpose <property>",
		Short:       "Pull Crawl Stats drill-down filtered by purpose",
		Long:        "Drill-down by Googlebot purpose. Valid values: discovery, refresh.",
		Example:     "  google-search-console-pp-cli crawl-stats by-purpose sc-domain:example.com --purpose discovery",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				return c.Help()
			}
			code, err := lookupCode(purposeCodes, purpose, "--purpose", purposeKeys())
			if err != nil {
				return usageErr(err)
			}
			return runCrawlStats(c, flags, common, args[0], client.DimensionPurpose, code, purpose)
		},
	}
	cmd.Flags().StringVar(&purpose, "purpose", "", "Purpose filter (required)")
	_ = cmd.MarkFlagRequired("purpose")
	attachCommonCrawlStatsFlags(cmd, common)
	return cmd
}

func newCrawlStatsUnionCmd(flags *rootFlags) *cobra.Command {
	var (
		fileType   string
		googlebot  string
		responseCd string
		limit      int
	)
	cmd := &cobra.Command{
		Use:   "union <property>",
		Short: "Read the unioned sample-URL set across all polls (offline; no network)",
		Long: `Returns the deduplicated union of every URL ever polled for this property,
read from the local SQLite store. Optionally narrow with the same filter
flags as the live drill-down commands.

Each live poll caps at ~1000 URL samples; unioning across multiple polls is
the only way to build a corpus larger than any single snapshot.`,
		Example:     "  google-search-console-pp-cli crawl-stats union sc-domain:example.com --file-type html",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				return c.Help()
			}
			site := args[0]
			ctx := context.Background()
			s, err := openStore(ctx)
			if err != nil {
				return configErr(err)
			}
			defer s.Close()

			var responseInt int
			if responseCd != "" {
				v, err := lookupCode(responseCodes, responseCd, "--response-code", responseKeys())
				if err != nil {
					return usageErr(err)
				}
				responseInt = v
			}
			fileTypeCanon := canonFileType(fileType)
			googlebotCanon := canonGooglebot(googlebot)

			rows, err := s.QueryCrawlStatsSamplesUnion(ctx, site, fileTypeCanon, googlebotCanon, responseInt, limit)
			if err != nil {
				return apiErr(err)
			}
			if len(rows) == 0 {
				count, _ := s.CountCrawlStatsSamples(ctx, site)
				if count == 0 {
					return fmt.Errorf("local store has no crawl-stats polls for %q; run `crawl-stats list %s` (or a drill-down) at least once first", site, site)
				}
				return fmt.Errorf("no rows match the requested filters; %d total samples for %q", count, site)
			}

			out := []map[string]any{}
			for _, r := range rows {
				row := map[string]any{
					"url":            r.SampleURL,
					"file_type":      r.FileType,
					"response_code":  r.ResponseCode,
					"googlebot_type": r.GooglebotType,
					"size_bytes":     r.SizeBytes,
					"response_ms":    r.ResponseMs,
					"last_poll_at":   r.PollAt.Format(time.RFC3339),
				}
				if !r.FetchedAt.IsZero() {
					row["fetched_at"] = r.FetchedAt.Format(time.RFC3339)
				}
				out = append(out, row)
			}
			return printJSONFiltered(c.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&fileType, "file-type", "", "Filter union by file type (e.g. html, css)")
	cmd.Flags().StringVar(&googlebot, "googlebot", "", "Filter union by Googlebot crawler type")
	cmd.Flags().StringVar(&responseCd, "response-code", "", "Filter union by response code")
	cmd.Flags().IntVar(&limit, "top", 0, "Max rows to return (0 = no limit)")
	return cmd
}

// runCrawlStats is the shared execution path for the five live drill-down
// subcommands. dimLabel is the human-readable filter value (e.g. "css") used
// for SQLite persistence and JSON output; pass "" when dim is DimensionNone.
func runCrawlStats(cobraCmd *cobra.Command, flags *rootFlags, common *crawlStatsCommonFlags, property string, dim client.Dimension, filterCode int, dimLabel string) error {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return configErr(err)
	}
	jarPath := common.cookieJar
	if jarPath == "" {
		jarPath = cfg.CrawlStatsCookieJar
	}
	if jarPath == "" {
		return authErr(fmt.Errorf("crawl-stats requires a Google session cookie jar; set GSC_COOKIE_JAR=<path>, pass --cookie-jar <path>, or run `google-search-console-pp-cli auth login --chrome` for export instructions"))
	}
	jar, err := client.LoadNetscapeCookieJar(jarPath)
	if err != nil {
		return authErr(err)
	}
	if missing := jar.MissingCookies(); len(missing) > 0 {
		return authErr(fmt.Errorf("cookie jar %s is missing %d Google session cookie(s): %s", jarPath, len(missing), strings.Join(missing, ", ")))
	}

	csClient, err := client.NewCrawlStatsClient(jar, flags.timeout)
	if err != nil {
		return authErr(err)
	}
	csClient.DryRun = flags.dryRun

	xsrf := common.xsrfToken
	if xsrf == "" {
		xsrf = os.Getenv("GSC_XSRF_TOKEN")
	}
	if xsrf == "" && !flags.dryRun {
		return authErr(fmt.Errorf("crawl-stats requires an XSRF token (the `at=` form field from the GSC HTML); pass --xsrf-token <value> or set GSC_XSRF_TOKEN"))
	}
	if xsrf == "" {
		xsrf = "DRY-RUN-PLACEHOLDER"
	}

	ctx := context.Background()
	req := client.CrawlStatsRequest{
		Property:   property,
		Dimension:  dim,
		FilterCode: filterCode,
		XSRFToken:  xsrf,
		BuildLabel: common.buildLabel,
		SessionID:  common.sessionID,
		RequestSeq: common.requestSeq,
	}

	resp, err := csClient.Fetch(ctx, req)
	if err != nil {
		return apiErr(err)
	}

	// Apply --top to the in-memory sample list before printing / persisting.
	if common.top > 0 && len(resp.Samples) > common.top {
		resp.Samples = resp.Samples[:common.top]
	}

	// Best-effort persistence. --no-persist or --dry-run suppresses.
	if !common.noPersist && !flags.dryRun {
		if err := persistCrawlStatsPoll(ctx, resp, dim, filterCode, dimLabel); err != nil {
			fmt.Fprintf(os.Stderr, "warning: persisting crawl-stats poll to local store failed: %v\n", err)
		}
	}

	return emitCrawlStats(cobraCmd, flags, resp, dimLabel)
}

func emitCrawlStats(c *cobra.Command, flags *rootFlags, resp *client.CrawlStatsResponse, dimLabel string) error {
	if flags.asJSON {
		out := map[string]any{
			"property":         resp.Property,
			"filter_dimension": string(resp.Dimension),
			"filter_value":     dimLabel,
			"filter_code":      resp.FilterCode,
			"captured_at":      resp.CapturedAt.Format(time.RFC3339),
			"totals":           resp.Totals,
			"time_series":      resp.TimeSeries,
			"samples":          resp.Samples,
		}
		if len(resp.RawResponses) > 0 {
			out["raw"] = resp.RawResponses
		}
		return printJSONFiltered(c.OutOrStdout(), out, flags)
	}

	// Human-friendly table output.
	w := c.OutOrStdout()
	fmt.Fprintf(w, "Property: %s\n", resp.Property)
	if dimLabel != "" {
		fmt.Fprintf(w, "Filter:   %s = %s (code %d)\n", resp.Dimension, dimLabel, resp.FilterCode)
	}
	fmt.Fprintf(w, "Captured: %s\n", resp.CapturedAt.Format(time.RFC3339))
	if resp.Totals != nil {
		fmt.Fprintf(w, "\nTotals:\n  crawl_requests=%d  download_size=%d  avg_response_ms=%d\n",
			resp.Totals.CrawlRequests, resp.Totals.DownloadSizeBytes, resp.Totals.AvgResponseMs)
	}
	if len(resp.TimeSeries) > 0 {
		fmt.Fprintf(w, "\nTime series (%d points):\n", len(resp.TimeSeries))
		for _, p := range resp.TimeSeries {
			fmt.Fprintf(w, "  %s  %d\n", p.Date, p.CrawlRequests)
		}
	}
	if len(resp.Samples) > 0 {
		fmt.Fprintf(w, "\nSamples (%d URLs):\n", len(resp.Samples))
		for _, s := range resp.Samples {
			fmt.Fprintln(w, "  "+s.URL)
		}
	}
	if len(resp.RawResponses) > 0 {
		fmt.Fprintf(w, "\n(raw response shards available via --json)\n")
	}
	return nil
}

func persistCrawlStatsPoll(ctx context.Context, resp *client.CrawlStatsResponse, dim client.Dimension, filterCode int, dimLabel string) error {
	if resp == nil {
		return nil
	}
	s, err := openStore(ctx)
	if err != nil {
		return err
	}
	defer s.Close()

	pollAt := resp.CapturedAt
	if pollAt.IsZero() {
		pollAt = time.Now().UTC()
	}

	// Samples — encode dim into the row so subsequent --file-type/--googlebot
	// filters on `union` work without re-derivation.
	sampleRows := make([]store.CrawlStatsSampleRow, 0, len(resp.Samples))
	for _, sample := range resp.Samples {
		row := store.CrawlStatsSampleRow{
			SiteURL:    resp.Property,
			SampleURL:  sample.URL,
			FetchedAt:  sample.FetchedAt,
			SizeBytes:  sample.SizeBytes,
			ResponseMs: sample.ResponseMs,
			PollAt:     pollAt,
		}
		switch dim {
		case client.DimensionFileType:
			row.FileType = dimLabel
		case client.DimensionResponseCode:
			row.ResponseCode = filterCode
		case client.DimensionGooglebotType:
			row.GooglebotType = dimLabel
		}
		if sample.ResponseCode > 0 {
			row.ResponseCode = sample.ResponseCode
		}
		// Best-effort raw payload preservation.
		if b, err := json.Marshal(sample); err == nil {
			row.RawJSON = string(b)
		}
		sampleRows = append(sampleRows, row)
	}
	if err := s.UpsertCrawlStatsSamples(ctx, sampleRows); err != nil {
		return err
	}

	if resp.Totals != nil {
		if err := s.UpsertCrawlStatsTotals(ctx, store.CrawlStatsTotalsRow{
			SiteURL:           resp.Property,
			FilterDim:         string(dim),
			FilterCode:        filterCode,
			PollAt:            pollAt,
			CrawlRequests:     resp.Totals.CrawlRequests,
			DownloadSizeBytes: resp.Totals.DownloadSizeBytes,
			AvgResponseMs:     resp.Totals.AvgResponseMs,
		}); err != nil {
			return err
		}
	}

	if len(resp.TimeSeries) > 0 {
		tsRows := make([]store.CrawlStatsTimeSeriesRow, 0, len(resp.TimeSeries))
		for _, p := range resp.TimeSeries {
			tsRows = append(tsRows, store.CrawlStatsTimeSeriesRow{
				SiteURL:       resp.Property,
				FilterDim:     string(dim),
				FilterCode:    filterCode,
				Date:          p.Date,
				CrawlRequests: p.CrawlRequests,
				PollAt:        pollAt,
			})
		}
		if err := s.UpsertCrawlStatsTimeSeries(ctx, tsRows); err != nil {
			return err
		}
	}
	return nil
}

// Helpers — enum lookup with friendly error messaging.

func lookupCode(table map[string]int, val, flag string, valid []string) (int, error) {
	v := strings.ToLower(strings.TrimSpace(val))
	if v == "" {
		return 0, fmt.Errorf("%s is required", flag)
	}
	if code, ok := table[v]; ok {
		return code, nil
	}
	return 0, fmt.Errorf("unknown %s %q; valid values: %s", flag, val, strings.Join(valid, ", "))
}

// fileTypeKeys returns the canonical list of valid --file-type values for
// error messages. Skips the `javascript` alias.
func fileTypeKeys() []string {
	return []string{"html", "image", "video", "js", "css", "pdf", "other", "unknown"}
}

func responseKeys() []string {
	return []string{
		"ok", "not-found", "unauthorized", "other-4xx", "dns-error",
		"fetch-error", "robots-blocked", "server-error", "dns-unresponsive",
		"robots-unavailable", "page-unreachable", "page-timeout",
		"redirect-error", "other-fetch-error", "moved-permanently",
		"moved-temporarily", "moved-other", "not-modified",
	}
}

func googlebotKeys() []string {
	return []string{
		"smartphone", "desktop", "image", "video", "page-resource",
		"other", "adsbot", "storebot", "amp",
	}
}

func purposeKeys() []string {
	return []string{"discovery", "refresh"}
}

// canonFileType normalizes a free-form input ("HTML", "Css") to the canonical
// lowercase form used as the SQLite column value. Returns the alias-resolved
// canonical name (e.g. "javascript" -> "js").
func canonFileType(s string) string {
	v := strings.ToLower(strings.TrimSpace(s))
	if v == "" {
		return ""
	}
	if v == "javascript" {
		return "js"
	}
	if _, ok := fileTypeCodes[v]; ok {
		return v
	}
	return v
}

func canonGooglebot(s string) string {
	v := strings.ToLower(strings.TrimSpace(s))
	return v
}
