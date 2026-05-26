package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggparse"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggstore"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/store"

	"github.com/spf13/cobra"
)

// newDiggSyncCmd replaces the generated sync command. The generator
// emits a sync that walks the spec's REST resources, but Digg's data
// only flows through HTML scrape (/ai) and one JSON endpoint
// (/api/trending/status). This implementation does what the data shape
// actually requires: fetch the HTML, decode the embedded RSC stream,
// extract clusters and authors, persist them, and pull the trending
// status events on the side.
var validTopics = map[string]bool{
	"ai": true, "technology": true, "science": true, "world": true,
	"politics": true, "business": true, "sports": true, "entertainment": true, "news": true,
}

func newDiggSyncCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var withDetails bool
	var skipEvents bool
	// PATCH(digg-enhancements): multi-topic sync. Loops fetch+parse+persist
	// once per topic; defaults to ["ai"] to preserve previous behavior.
	var topics []string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync the /ai feed and /api/trending/status events into the local store",
		Long: `Pull the current Digg AI feed and the trending pipeline event stream into a local
SQLite database. The /ai page is fetched once; the embedded RSC stream is decoded
and every cluster, author, and snapshot is persisted. The /api/trending/status
endpoint is then read for pipeline events. Replacement archaeology runs at the
end to record clusters that were present in the previous sync but are absent now.

Sync is read-only against Digg. It never mutates anything upstream.`,
		Example: `  # Sync the current /ai feed and trending events
  digg-pp-cli sync

  # Skip events (only feed)
  digg-pp-cli sync --no-events

  # Also fetch each cluster's detail page (slower; populates fuller fields)
  digg-pp-cli sync --with-details`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if dbPath == "" {
				dbPath = defaultDBPath("digg-pp-cli")
			}

			// Validate topics
			for _, t := range topics {
				if !validTopics[t] {
					valid := []string{"ai", "technology", "science", "world", "politics", "business", "sports", "entertainment", "news"}
					return fmt.Errorf("unknown topic %q; valid topics: %s", t, strings.Join(valid, ", "))
				}
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would sync topics: %s\n", strings.Join(topics, ", "))
				return nil
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

			totalClusters := 0
			totalEmbeddedEvents := 0
			observed := make(map[string]bool)

			for _, topic := range topics {
				topicURL := "https://di.gg/" + topic

				// 1. Fetch topic HTML
				fmt.Fprintf(out, "fetching /%s ...\n", topic)
				html, err := fetchURL(ctx, topicURL)
				if err != nil {
					return fmt.Errorf("fetching /%s: %w", topic, err)
				}
				clusters, embeddedEvents, _, err := diggparse.ParseHomeFeed(html)
				if err != nil {
					return fmt.Errorf("parsing /%s: %w", topic, err)
				}
				fmt.Fprintf(out, "parsed %d clusters from /%s (%d KB)\n", len(clusters), topic, len(html)/1024)

				// 2. Persist
				for _, c := range clusters {
					observed[c.ClusterID] = true
					if err := diggstore.UpsertCluster(db, c, now); err != nil {
						return err
					}
				}

				// Persist embedded events too (cluster_detected, fast_climb seen in stream)
				for _, e := range embeddedEvents {
					if err := diggstore.UpsertEvent(db, e, now); err != nil {
						return err
					}
				}

				totalClusters += len(clusters)
				totalEmbeddedEvents += len(embeddedEvents)

				// 5. Optionally fetch detail pages for clusters
				if withDetails {
					fetched := 0
					for _, c := range clusters {
						if c.ClusterURLID == "" {
							continue
						}
						detailURL := "https://di.gg/" + topic + "/" + c.ClusterURLID
						body, err := fetchURL(ctx, detailURL)
						if err != nil {
							fmt.Fprintf(out, "  detail %s: %v\n", c.ClusterURLID, err)
							continue
						}
						more, _, _, err := diggparse.ParseHomeFeed(body)
						if err != nil || len(more) == 0 {
							continue
						}
						for _, mc := range more {
							if mc.ClusterID == c.ClusterID {
								_ = diggstore.UpsertCluster(db, mc, now)
								break
							}
						}
						fetched++
						time.Sleep(500 * time.Millisecond)
					}
					fmt.Fprintf(out, "fetched %d detail pages for /%s\n", fetched, topic)
				}
			}

			// 3. Replacements (scoped to the synced topics only).
			// PATCH(digg-enhancements): the non-scoped RecordReplacements
			// scans every cluster in the 2-hour window, which means a topic-
			// subset sync would falsely mark other topics' clusters as fell-
			// out-of-feed. The ForTopics variant scopes the query to topics
			// we actually fetched this run.
			if err := diggstore.RecordReplacementsForTopics(db, observed, now, topics); err != nil {
				return err
			}

			// 4. Trending status events
			if !skipEvents {
				fmt.Fprintln(out, "fetching /api/trending/status ...")
				body, err := fetchURL(ctx, "https://di.gg/api/trending/status")
				if err != nil {
					return fmt.Errorf("fetching trending status: %w", err)
				}
				ts, err := diggparse.ParseTrendingStatus(body)
				if err != nil {
					return err
				}
				for _, e := range ts.Events {
					if err := diggstore.UpsertEvent(db, e, now); err != nil {
						return err
					}
				}
				fmt.Fprintf(out, "stored %d events; storiesToday=%d clustersToday=%d\n",
					len(ts.Events), ts.StoriesToday, ts.ClustersToday)
			}

			summary := map[string]any{
				"event":           "sync_summary",
				"topics":          topics,
				"clusters_synced": totalClusters,
				"events_synced":   totalEmbeddedEvents,
				"with_details":    withDetails,
				"skip_events":     skipEvents,
				"db_path":         dbPath,
				"computed_at":     now.Format(time.RFC3339Nano),
			}
			if flags.asJSON {
				return printJSONFiltered(out, summary, flags)
			}
			fmt.Fprintf(out, "synced %d clusters into %s\n", totalClusters, dbPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/digg-pp-cli/data.db)")
	cmd.Flags().BoolVar(&withDetails, "with-details", false, "Also fetch each cluster's detail page (slower; richer fields)")
	cmd.Flags().BoolVar(&skipEvents, "no-events", false, "Skip /api/trending/status events fetch")
	cmd.Flags().StringSliceVar(&topics, "topics", []string{"ai"}, "Comma-separated topic slugs to sync (valid: ai, technology, science, world, politics, business, sports, entertainment, news)")

	cmd.Annotations = map[string]string{}
	return cmd
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
