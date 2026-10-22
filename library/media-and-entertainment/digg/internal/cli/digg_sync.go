package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggparse"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggstore"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/store"

	"github.com/spf13/cobra"
)

// newDiggSyncCmd replaces the generated sync command. The generator
// emits a sync that walks the spec's REST resources, but Digg's data
// flows through two surfaces: each section's HTML page, whose embedded
// RSC stream carries the ranked list (rank, delta, scores, sentiment,
// cross-references), and the paginated /api/feed/continuous JSON feed,
// which reaches back far enough to back search.
//
// PATCH(digg-feed-continuous-replaces-removed-search-and-trending): Digg
// renamed its sections and removed /api/trending/status in October 2026.
// Sync now walks the feed per section, tolerates per-section failures,
// and no longer fetches pipeline events.

// syncTopics lists Digg's current sections in display order. The value is
// the HTML page path for the ranked-list scrape; "all" is a cross-section
// mix whose page lives on the home route.
var syncTopics = []struct{ slug, page string }{
	{"ai", "/ai"}, {"tech", "/tech"}, {"world-business", "/world-business"}, {"sports", "/sports"},
	{"science", "/science"}, {"gaming", "/gaming"}, {"entertainment", "/entertainment"}, {"all", "/?feed=all"},
}

// feedRanges are the range values /api/feed/continuous accepts.
var feedRanges = []string{"24h", "7d", "30d", "all"}

// defaultFeedRange is the range sync walks and search assumes without --since.
const defaultFeedRange = "30d"

// diggSiteURL is the origin for section pages; tests point it at a mock.
var diggSiteURL = "https://digg.com"

// feedURLOverride, when set, replaces the feed endpoint. Tests only.
var feedURLOverride string

func validTopicSlugs() []string {
	out := make([]string, len(syncTopics))
	for i, t := range syncTopics {
		out[i] = t.slug
	}
	return out
}

// validateTopics rejects any slug that is not a current Digg section.
func validateTopics(topics []string) error {
	valid := validTopicSlugs()
	for _, t := range topics {
		if !slices.Contains(valid, t) {
			return usageErr(fmt.Errorf("unknown topic %q; valid topics: %s", t, strings.Join(valid, ", ")))
		}
	}
	return nil
}

func topicPage(slug string) string {
	for _, t := range syncTopics {
		if t.slug == slug {
			return t.page
		}
	}
	return "/" + slug
}

// walkFeed walks one topic's feed, calling fn per page.
func walkFeed(ctx context.Context, c *client.Client, p client.FeedParams, maxPages int, fn func(*client.FeedPage) error) (client.FeedWalkResult, error) {
	if feedURLOverride != "" {
		return c.WalkFeedFrom(ctx, feedURLOverride, p, maxPages, fn)
	}
	return c.WalkFeed(ctx, p, maxPages, fn)
}

// topicSyncResult is one topic's outcome. scraped reports whether the
// ranked-list scrape succeeded, which gates replacement detection.
type topicSyncResult struct {
	Topic    string `json:"topic"`
	Clusters int    `json:"clusters_scraped"`
	Pages    int    `json:"feed_pages"`
	Stories  int    `json:"feed_stories"`
	Capped   bool   `json:"capped_by_max_pages"`
	scraped  bool
	observed []string
}

func newDiggSyncCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var withDetails bool
	// PATCH(digg-enhancements): multi-topic sync. Defaults to ["ai"] to
	// preserve previous behavior.
	var topics []string
	var feedRange string
	var maxPages int

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync Digg sections' ranked lists and story feeds into the local store",
		Long: `Pull Digg sections into a local SQLite database. For each topic, sync reads the
section page's ranked list (rank, delta, scores, sentiment, cross-references) and
then walks the section's story feed newest-ranked first, page by page, so search
can reach back across the whole --range window. Stories are saved as each page
arrives, so an interrupted sync keeps its progress.

A topic that fails prints a warning and the others continue; sync exits non-zero
only when every topic failed. Replacement archaeology runs at the end for topics
whose ranked list was read.

Sync is read-only against Digg. It never mutates anything upstream.`,
		Example: `  # Sync the AI section (ranked list plus 30 days of stories)
  digg-pp-cli sync

  # Sync several sections
  digg-pp-cli sync --topics ai,tech,science

  # Quick, shallow refresh of every section (search still tops these up live)
  digg-pp-cli sync --topics ai,tech,world-business,sports,science,gaming,entertainment,all --max-pages 1

  # Also fetch each cluster's detail page (slower; populates fuller fields)
  digg-pp-cli sync --with-details`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if dbPath == "" {
				dbPath = defaultDBPath("digg-pp-cli")
			}
			if err := validateTopics(topics); err != nil {
				return err
			}
			if !slices.Contains(feedRanges, feedRange) {
				return usageErr(fmt.Errorf("invalid --range %q; valid ranges: %s", feedRange, strings.Join(feedRanges, ", ")))
			}
			if maxPages < 1 {
				return usageErr(fmt.Errorf("--max-pages must be at least 1"))
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would sync topics: %s\n", strings.Join(topics, ", "))
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			s, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer s.Close()

			db := s.DB()
			if err := diggstore.EnsureSchema(db); err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			now := time.Now().UTC()
			// Topics run concurrently; SQLite writes are serialized here.
			var writeMu sync.Mutex
			var outMu sync.Mutex
			logf := func(format string, a ...any) {
				outMu.Lock()
				defer outMu.Unlock()
				fmt.Fprintf(out, format, a...)
			}

			results, errs := cliutil.FanoutRun(ctx, topics,
				func(t string) string { return t },
				func(ctx context.Context, topic string) (topicSyncResult, error) {
					r := topicSyncResult{Topic: topic}

					// 1. Ranked list from the section page.
					clusters, err := scrapeTopic(ctx, db, &writeMu, topic, now)
					if err != nil {
						outMu.Lock()
						fmt.Fprintf(cmd.ErrOrStderr(), "warn: %s: ranked list unavailable: %v\n", topic, err)
						outMu.Unlock()
					} else {
						r.scraped = true
						r.Clusters = len(clusters)
						for _, cl := range clusters {
							r.observed = append(r.observed, cl.ClusterID)
						}
						logf("parsed %d ranked clusters from %s\n", len(clusters), topicPage(topic))
						if withDetails {
							fetchDetails(ctx, db, &writeMu, topic, clusters, now, logf)
						}
					}

					// 2. Story feed, saved page by page.
					p := client.FeedParams{Topic: topic, View: "top", Range: feedRange}
					var oldest time.Time
					res, werr := walkFeed(ctx, c, p, maxPages, func(page *client.FeedPage) error {
						for _, s := range page.Posts {
							if t, err := time.Parse(time.RFC3339Nano, s.CreatedAt); err == nil && (oldest.IsZero() || t.Before(oldest)) {
								oldest = t
							}
						}
						writeMu.Lock()
						defer writeMu.Unlock()
						return diggstore.UpsertFeedPage(db, page.Posts, topic, feedRange, now)
					})
					r.Pages, r.Stories, r.Capped = res.Pages, res.Stories, res.Capped
					if werr != nil {
						return r, fmt.Errorf("feed walk stopped after %d pages (%d stories kept): %w", res.Pages, res.Stories, werr)
					}
					covered := res.Complete || cappedWalkCovers(feedRange, oldest, now)
					if covered {
						writeMu.Lock()
						err = diggstore.MarkFeedSynced(db, topic, feedRange, res.Stories, now)
						writeMu.Unlock()
						if err != nil {
							return r, err
						}
					}
					note := ""
					switch {
					case res.Capped && covered:
						note = fmt.Sprintf(" (stopped at --max-pages %d; reached far enough back to count as synced)", maxPages)
					case res.Capped:
						note = fmt.Sprintf(" (stopped at --max-pages %d before reaching the start of the %s range; search will still treat %s as stale)", maxPages, feedRange, topic)
					}
					logf("stored %d stories from %d feed pages for %s%s\n", res.Stories, res.Pages, topic, note)
					return r, nil
				})
			cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)

			// Replacements, scoped to topics whose ranked list was read this
			// run: a topic whose scrape failed would otherwise look emptied.
			// PATCH(digg-enhancements): the non-scoped RecordReplacements
			// would mark other topics' clusters as fell-out-of-feed.
			observed := map[string]bool{}
			var scrapedTopics []string
			var perTopic []topicSyncResult
			totalClusters, totalStories := 0, 0
			for _, res := range results {
				perTopic = append(perTopic, res.Value)
				totalClusters += res.Value.Clusters
				totalStories += res.Value.Stories
				if res.Value.scraped {
					scrapedTopics = append(scrapedTopics, res.Value.Topic)
					for _, id := range res.Value.observed {
						observed[id] = true
					}
				}
			}
			if len(scrapedTopics) > 0 {
				if err := diggstore.RecordReplacementsForTopics(db, observed, now, scrapedTopics); err != nil {
					return err
				}
			}

			if len(results) == 0 {
				return fmt.Errorf("sync failed for every topic (%s)", strings.Join(topics, ", "))
			}

			failed := make([]string, 0, len(errs))
			for _, e := range errs {
				failed = append(failed, e.Source)
			}
			summary := map[string]any{
				"event":           "sync_summary",
				"topics":          perTopic,
				"failed_topics":   failed,
				"clusters_synced": totalClusters,
				"stories_synced":  totalStories,
				"range":           feedRange,
				"with_details":    withDetails,
				"db_path":         dbPath,
				"computed_at":     now.Format(time.RFC3339Nano),
			}
			if flags.asJSON {
				return printJSONFiltered(out, summary, flags)
			}
			fmt.Fprintf(out, "synced %d ranked clusters and %d feed stories into %s\n", totalClusters, totalStories, dbPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/digg-pp-cli/data.db)")
	cmd.Flags().BoolVar(&withDetails, "with-details", false, "Also fetch each cluster's detail page (slower; richer fields)")
	cmd.Flags().StringSliceVar(&topics, "topics", []string{"ai"}, "Comma-separated topic slugs to sync (valid: "+strings.Join(validTopicSlugs(), ", ")+")")
	cmd.Flags().StringVar(&feedRange, "range", defaultFeedRange, "How far back the story feed reaches: 24h, 7d, 30d, or all")
	cmd.Flags().IntVar(&maxPages, "max-pages", 120, "Maximum feed pages (20 stories each) to fetch per topic")

	cmd.Annotations = map[string]string{}
	return cmd
}

// cappedCoverage is how much of a range a walk cut short by --max-pages
// must reach back through to count as a sync of that range.
const cappedCoverage = 0.9

// cappedWalkCovers reports whether a walk that stopped at the page cap,
// whose oldest story was created at oldest, reached far enough back to
// stand in for a complete walk of feedRange. The unbounded "all" range
// never does.
func cappedWalkCovers(feedRange string, oldest, now time.Time) bool {
	window, ok := map[string]time.Duration{"24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour, "30d": 30 * 24 * time.Hour}[feedRange]
	if !ok || oldest.IsZero() {
		return false
	}
	return !oldest.After(now.Add(-time.Duration(float64(window) * cappedCoverage)))
}

// scrapeTopic fetches and parses one section page and saves its ranked
// clusters and embedded events.
func scrapeTopic(ctx context.Context, db *sql.DB, writeMu *sync.Mutex, topic string, now time.Time) ([]diggparse.Cluster, error) {
	html, err := fetchURL(ctx, diggSiteURL+topicPage(topic))
	if err != nil {
		return nil, err
	}
	clusters, events, _, err := diggparse.ParseHomeFeed(html)
	if err != nil {
		return nil, err
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	if err := persistScrape(db, clusters, events, now); err != nil {
		return nil, err
	}
	return clusters, nil
}

// persistScrape saves one section page's ranked clusters and embedded events.
func persistScrape(db *sql.DB, clusters []diggparse.Cluster, events []diggparse.Event, now time.Time) error {
	for _, c := range clusters {
		if err := diggstore.UpsertCluster(db, c, now); err != nil {
			return err
		}
	}
	for _, e := range events {
		if err := diggstore.UpsertEvent(db, e, now); err != nil {
			return err
		}
	}
	return nil
}

// fetchDetails re-reads each cluster's detail page for fuller fields.
func fetchDetails(ctx context.Context, db *sql.DB, writeMu *sync.Mutex, topic string, clusters []diggparse.Cluster, now time.Time, logf func(string, ...any)) {
	fetched := 0
	for _, c := range clusters {
		if c.ClusterURLID == "" {
			continue
		}
		body, err := fetchURL(ctx, diggSiteURL+"/"+topic+"/"+c.ClusterURLID)
		if err != nil {
			logf("  detail %s: %v\n", c.ClusterURLID, err)
			continue
		}
		more, _, _, err := diggparse.ParseHomeFeed(body)
		if err != nil || len(more) == 0 {
			continue
		}
		for _, mc := range more {
			if mc.ClusterID == c.ClusterID {
				writeMu.Lock()
				_ = diggstore.UpsertCluster(db, mc, now)
				writeMu.Unlock()
				break
			}
		}
		fetched++
		time.Sleep(500 * time.Millisecond)
	}
	logf("fetched %d detail pages for /%s\n", fetched, topic)
}

// fetchURL is a tiny stdlib HTTP client used by the digg sync and live
// commands. Identifies itself with a clear User-Agent so Digg ops can
// rate-limit it cleanly. 30s timeout; respects ctx cancellation.
func fetchURL(ctx context.Context, url string) ([]byte, error) {
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", diggUserAgent())
	req.Header.Set("Accept", "text/html,application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func diggUserAgent() string {
	return "digg-pp-cli/0.1.0 (+https://github.com/mvanhorn/printing-press-library)"
}

// asJSONString takes any nullable JSON value and returns either a parsed
// any or nil — used by command output paths that pass raw_json straight
// through.
func asJSONString(s string) any {
	if s == "" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err == nil {
		return v
	}
	return s
}

// joinNonEmpty filters out empty strings then joins.
func joinNonEmpty(xs []string, sep string) string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if x != "" {
			out = append(out, x)
		}
	}
	return strings.Join(out, sep)
}
