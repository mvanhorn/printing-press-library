package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggparse"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggstore"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/store"

	"github.com/spf13/cobra"
)

// newDiggSyncCmd replaces the generated sync command. The generator
// emits a sync that walks the spec's REST resources, but the only Digg
// data this CLI keeps locally is rank history: each topic page's embedded
// stream carries its ranked list, and sync stores one snapshot of it per
// run. `history`, `watch`, `stats` and the `authors` commands read those
// snapshots. Search and `top` read Digg live and need no sync.

// syncTopics lists Digg's current topics in display order. The value is
// the HTML page path for the ranked-list scrape; "all" is a cross-topic
// mix whose page lives on the home route.
var syncTopics = []struct{ slug, page string }{
	{"ai", "/ai"}, {"tech", "/tech"}, {"world-business", "/world-business"}, {"sports", "/sports"},
	{"science", "/science"}, {"gaming", "/gaming"}, {"entertainment", "/entertainment"}, {"all", "/?feed=all"},
}

// syncStatePrefix keys each topic's snapshot record in sync_state.
const syncStatePrefix = "ranked-list:"

// diggSiteURL is the origin for section pages; tests point it at a mock.
var diggSiteURL = "https://digg.com"

func validTopicSlugs() []string {
	out := make([]string, len(syncTopics))
	for i, t := range syncTopics {
		out[i] = t.slug
	}
	return out
}

// validateTopics rejects any slug that is not a current Digg topic.
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

func newDiggSyncCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	// PATCH(digg-enhancements): multi-topic sync. Defaults to ["ai"] to
	// preserve previous behavior.
	var topics []string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Snapshot Digg topics' ranked lists into the local store, building rank history",
		Long: `Take one snapshot of each topic's ranked list and save it to a local SQLite
database. Each run adds a point to every listed story's rank history, which
` + "`history`" + ` shows; ` + "`watch`" + `, ` + "`stats`" + ` and the ` + "`authors`" + ` commands read the same
snapshots. Run it on a schedule to build a trajectory.

` + "`search`" + ` and ` + "`top`" + ` read Digg live and do not need a sync.

A topic that fails prints a warning and the others continue; sync exits non-zero
only when every topic failed.

Sync is read-only against Digg. It never mutates anything upstream.`,
		Example: `  # Snapshot the AI topic's ranked list
  digg-pp-cli sync

  # Snapshot several topics
  digg-pp-cli sync --topics ai,tech,science

  # Snapshot every topic
  digg-pp-cli sync --topics ai,tech,world-business,sports,science,gaming,entertainment,all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if dbPath == "" {
				dbPath = defaultDBPath("digg-pp-cli")
			}
			if err := validateTopics(topics); err != nil {
				return err
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

			// Freshness markers left by the retired story-feed walk would
			// otherwise read as stale in `doctor` forever.
			if _, err := db.ExecContext(ctx, `DELETE FROM sync_state WHERE resource_type LIKE 'feed:%'`); err != nil {
				return fmt.Errorf("clearing retired sync state: %w", err)
			}

			out := cmd.OutOrStdout()
			now := time.Now().UTC()
			// Topics run concurrently; SQLite writes are serialized here.
			var writeMu, outMu sync.Mutex

			type topicResult struct {
				Topic    string `json:"topic"`
				Clusters int    `json:"clusters_scraped"`
			}
			results, errs := cliutil.FanoutRun(ctx, topics,
				func(t string) string { return t },
				func(ctx context.Context, topic string) (topicResult, error) {
					n, err := scrapeTopic(ctx, db, &writeMu, topic, now)
					if err != nil {
						return topicResult{}, fmt.Errorf("ranked list unavailable: %w", err)
					}
					// `doctor` reports cache freshness from this record.
					writeMu.Lock()
					err = s.SaveSyncState(syncStatePrefix+topic, "", n)
					writeMu.Unlock()
					if err != nil {
						return topicResult{}, err
					}
					outMu.Lock()
					fmt.Fprintf(out, "parsed %d ranked clusters from %s\n", n, topicPage(topic))
					outMu.Unlock()
					return topicResult{Topic: topic, Clusters: n}, nil
				})
			cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)

			if len(results) == 0 {
				return fmt.Errorf("sync failed for every topic (%s)", strings.Join(topics, ", "))
			}

			perTopic := make([]topicResult, 0, len(results))
			total := 0
			for _, res := range results {
				perTopic = append(perTopic, res.Value)
				total += res.Value.Clusters
			}
			failed := make([]string, 0, len(errs))
			for _, e := range errs {
				failed = append(failed, e.Source)
			}
			if flags.asJSON {
				return printJSONFiltered(out, map[string]any{
					"event":           "sync_summary",
					"topics":          perTopic,
					"failed_topics":   failed,
					"clusters_synced": total,
					"db_path":         dbPath,
					"computed_at":     now.Format(time.RFC3339Nano),
				}, flags)
			}
			fmt.Fprintf(out, "synced %d ranked clusters into %s\n", total, dbPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/digg-pp-cli/data.db)")
	cmd.Flags().StringSliceVar(&topics, "topics", []string{"ai"}, "Comma-separated topic slugs to sync (valid: "+strings.Join(validTopicSlugs(), ", ")+")")

	cmd.Annotations = map[string]string{}
	return cmd
}

// scrapeTopic fetches and parses one topic page, saves a snapshot of its
// ranked clusters, and returns how many it saved.
func scrapeTopic(ctx context.Context, db *sql.DB, writeMu *sync.Mutex, topic string, now time.Time) (int, error) {
	html, err := fetchURL(ctx, diggSiteURL+topicPage(topic))
	if err != nil {
		return 0, err
	}
	clusters, _, _, err := diggparse.ParseHomeFeed(html)
	if err != nil {
		return 0, err
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	for _, c := range clusters {
		if err := diggstore.UpsertCluster(db, c, now); err != nil {
			return 0, err
		}
	}
	return len(clusters), nil
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
