package diggstore

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggparse"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/store"
)

func feedStory(t *testing.T, id, urlID, title, desc string, created time.Time) client.FeedStory {
	t.Helper()
	b, _ := json.Marshal(map[string]any{
		"clusterId": id, "clusterUrlId": urlID, "rank": 4, "diggScore": 55.5,
		"createdAt": created.UTC().Format(time.RFC3339), "postCount": 12, "uniqueAuthors": 7,
		"likes": 30, "views": 900,
		"summary": map[string]string{"title": title, "description": desc},
		"authors": []map[string]string{{"username": "alice"}},
	})
	var s client.FeedStory
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestEnsureSchemaUpgradesExistingClusterStore(t *testing.T) {
	// Rebuild the shape a store from the release before the feed fix has:
	// the full cluster table without the feed columns, and no topics table.
	db := openTempDB(t)
	for _, q := range []string{
		`DROP INDEX idx_digg_clusters_created`,
		`DROP TABLE digg_cluster_topics`,
		`ALTER TABLE digg_clusters DROP COLUMN created_at`,
		`ALTER TABLE digg_clusters DROP COLUMN last_source_post_at`,
		`ALTER TABLE digg_clusters DROP COLUMN post_count`,
		`ALTER TABLE digg_clusters DROP COLUMN unique_authors`,
		`ALTER TABLE digg_clusters DROP COLUMN digg_score`,
		`ALTER TABLE digg_clusters DROP COLUMN feed_rank`,
		`INSERT INTO digg_clusters (cluster_id, label, current_rank, fetched_at, last_seen_at)
			VALUES ('legacy', 'Legacy story', 2, '2026-06-01T00:00:00Z', '2026-06-01T00:00:00Z')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema on old store: %v", err)
	}
	var label string
	if err := db.QueryRow(`SELECT label FROM digg_clusters WHERE cluster_id='legacy'`).Scan(&label); err != nil || label != "Legacy story" {
		t.Fatalf("legacy row lost: label=%q err=%v", label, err)
	}
	if _, err := db.Exec(`SELECT created_at, post_count, unique_authors, feed_rank FROM digg_clusters`); err != nil {
		t.Fatalf("feed columns missing after upgrade: %v", err)
	}
	if _, err := db.Exec(`SELECT cluster_id, topic FROM digg_cluster_topics`); err != nil {
		t.Fatalf("digg_cluster_topics missing after upgrade: %v", err)
	}
}

func TestUpsertFeedPageRecordsEachTopicOnce(t *testing.T) {
	db := openTempDB(t)
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	s := feedStory(t, "c-1", "u1", "Claude ships a thing", "desc", now.Add(-48*time.Hour))
	if err := UpsertFeedPage(db, []client.FeedStory{s}, "ai", "30d", now); err != nil {
		t.Fatal(err)
	}
	if err := UpsertFeedPage(db, []client.FeedStory{s}, "all", "30d", now); err != nil {
		t.Fatal(err)
	}
	if err := UpsertFeedPage(db, []client.FeedStory{s}, "ai", "7d", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var clusters, topics int
	db.QueryRow(`SELECT COUNT(*) FROM digg_clusters`).Scan(&clusters)
	db.QueryRow(`SELECT COUNT(*) FROM digg_cluster_topics WHERE cluster_id='c-1'`).Scan(&topics)
	if clusters != 1 || topics != 2 {
		t.Fatalf("clusters=%d topics=%d, want 1 and 2", clusters, topics)
	}
	var title string
	var posts, authors, rank int
	db.QueryRow(`SELECT title, post_count, unique_authors, feed_rank FROM digg_clusters WHERE cluster_id='c-1'`).Scan(&title, &posts, &authors, &rank)
	if title != "Claude ships a thing" || posts != 12 || authors != 7 || rank != 4 {
		t.Fatalf("feed fields: title=%q posts=%d authors=%d rank=%d", title, posts, authors, rank)
	}
}

func TestUpsertFeedPageKeepsScrapeOwnedColumns(t *testing.T) {
	db := openTempDB(t)
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	c := diggparse.Cluster{ClusterID: "c-1", ClusterURLID: "u1", Label: "scraped", CurrentRank: 3, Delta: 5, GravityScore: 9.5, Pos6h: 0.7}
	if err := UpsertCluster(db, c, now); err != nil {
		t.Fatal(err)
	}
	s := feedStory(t, "c-1", "u1", "Feed title", "Feed desc", now.Add(-time.Hour))
	if err := UpsertFeedPage(db, []client.FeedStory{s}, "ai", "30d", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	var rank, delta int
	var gravity, pos6h float64
	var lastSeen string
	db.QueryRow(`SELECT current_rank, delta, gravity_score, pos6h, last_seen_at FROM digg_clusters WHERE cluster_id='c-1'`).
		Scan(&rank, &delta, &gravity, &pos6h, &lastSeen)
	if rank != 3 || delta != 5 || gravity != 9.5 || pos6h != 0.7 {
		t.Fatalf("scrape columns clobbered: rank=%d delta=%d gravity=%v pos6h=%v", rank, delta, gravity, pos6h)
	}
	if lastSeen != now.Format(time.RFC3339Nano) {
		t.Fatalf("last_seen_at = %q, want the scrape's %q (top/rising key off it)", lastSeen, now.Format(time.RFC3339Nano))
	}
}

func TestFeedOnlyRowsStayOutOfLatestScrape(t *testing.T) {
	db := openTempDB(t)
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	if err := UpsertCluster(db, diggparse.Cluster{ClusterID: "scraped", CurrentRank: 1}, now); err != nil {
		t.Fatal(err)
	}
	if err := UpsertFeedPage(db, []client.FeedStory{feedStory(t, "feed-only", "f1", "t", "d", now)}, "ai", "30d", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM digg_clusters WHERE last_seen_at = (SELECT MAX(last_seen_at) FROM digg_clusters)`).Scan(&n)
	if n != 1 {
		t.Fatalf("latest-scrape set has %d rows, want only the scraped cluster", n)
	}
}

func TestSearchFeedStoriesFiltersByTopicAndWindow(t *testing.T) {
	db := openTempDB(t)
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	stories := map[string]struct {
		topic   string
		title   string
		created time.Time
	}{
		"ai-new":     {"ai", "Claude Code gets plugins", now.Add(-3 * 24 * time.Hour)},
		"sports-new": {"sports", "Claude the goalkeeper saves a penalty", now.Add(-2 * 24 * time.Hour)},
		"ai-old":     {"ai", "Claude launch retrospective", now.Add(-40 * 24 * time.Hour)},
		"ai-other":   {"ai", "Gemini update", now.Add(-time.Hour)},
	}
	for id, s := range stories {
		if err := UpsertFeedPage(db, []client.FeedStory{feedStory(t, id, id+"-u", s.title, "", s.created)}, s.topic, "30d", now); err != nil {
			t.Fatal(err)
		}
	}
	got, err := SearchFeedStories(db, "claude", []string{"ai"}, now.Add(-30*24*time.Hour), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ClusterID != "ai-new" {
		t.Fatalf("got %+v, want only ai-new", got)
	}
	if got[0].Title != "Claude Code gets plugins" || got[0].PostCount != 12 || got[0].UniqueAuthors != 7 || got[0].Rank != 4 {
		t.Fatalf("match fields wrong: %+v", got[0])
	}
	if !got[0].CreatedAt.Equal(now.Add(-3 * 24 * time.Hour)) {
		t.Fatalf("CreatedAt = %v", got[0].CreatedAt)
	}
}

func TestSearchFeedStoriesMatchesDescriptionAndPunctuation(t *testing.T) {
	db := openTempDB(t)
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	if err := UpsertFeedPage(db, []client.FeedStory{feedStory(t, "c", "u", "New model", "OpenAI ships gpt-5 to everyone", now)}, "ai", "30d", now); err != nil {
		t.Fatal(err)
	}
	got, err := SearchFeedStories(db, `gpt-5 "openai`, []string{"ai"}, now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("punctuated query errored: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d matches, want 1 from the description", len(got))
	}
}

func TestFeedFreshness(t *testing.T) {
	db := openTempDB(t)
	if _, ok, err := FeedSyncedAt(db, "ai", "30d"); err != nil || ok {
		t.Fatalf("never-synced topic: ok=%v err=%v, want stale", ok, err)
	}
	at := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	if err := MarkFeedSynced(db, "ai", "30d", 400, at); err != nil {
		t.Fatal(err)
	}
	got, ok, err := FeedSyncedAt(db, "ai", "30d")
	if err != nil || !ok || !got.Equal(at) {
		t.Fatalf("FeedSyncedAt = %v ok=%v err=%v, want %v", got, ok, err, at)
	}
	if _, ok, _ := FeedSyncedAt(db, "ai", "7d"); ok {
		t.Fatal("a 30d sync must not mark the 7d key fresh")
	}
}

func TestSearchFeedStoriesRankIsPerTopic(t *testing.T) {
	db := openTempDB(t)
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	s := feedStory(t, "c-1", "u1", "Claude ships a thing", "", now.Add(-time.Hour))
	s.Rank = 2
	if err := UpsertFeedPage(db, []client.FeedStory{s}, "ai", "30d", now); err != nil {
		t.Fatal(err)
	}
	s.Rank = 40
	if err := UpsertFeedPage(db, []client.FeedStory{s}, "all", "30d", now); err != nil {
		t.Fatal(err)
	}
	for topic, want := range map[string]int{"ai": 2, "all": 40} {
		got, err := SearchFeedStories(db, "claude", []string{topic}, now.Add(-24*time.Hour), 10)
		if err != nil || len(got) != 1 || got[0].Rank != want {
			t.Fatalf("topic %s: got %+v err=%v, want rank %d", topic, got, err, want)
		}
	}
}

func TestUpsertFeedPageReindexesChangedTitle(t *testing.T) {
	db := openTempDB(t)
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	for _, title := range []string{"Old headline", "Old headline", "Fresh headline"} {
		s := feedStory(t, "c-1", "u1", title, "", now.Add(-time.Hour))
		if err := UpsertFeedPage(db, []client.FeedStory{s}, "ai", "30d", now); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM digg_clusters_fts WHERE cluster_id = 'c-1'`).Scan(&n)
	if n != 1 {
		t.Fatalf("fts rows for c-1 = %d, want exactly 1", n)
	}
	if got, _ := SearchFeedStories(db, "old", []string{"ai"}, now.Add(-24*time.Hour), 10); len(got) != 0 {
		t.Fatalf("stale title still matches: %+v", got)
	}
	if got, _ := SearchFeedStories(db, "fresh", []string{"ai"}, now.Add(-24*time.Hour), 10); len(got) != 1 {
		t.Fatalf("new title not indexed: %+v", got)
	}
}

func TestScrapeWithBlankTextKeepsFeedStoryIndexed(t *testing.T) {
	db := openTempDB(t)
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	s := feedStory(t, "c-1", "u1", "Claude ships", "A summary", now.Add(-time.Hour))
	if err := UpsertFeedPage(db, []client.FeedStory{s}, "ai", "30d", now); err != nil {
		t.Fatal(err)
	}
	// A scrape that carries no headline or summary keeps the saved text.
	if err := UpsertCluster(db, diggparse.Cluster{ClusterID: "c-1", ClusterURLID: "u1"}, now); err != nil {
		t.Fatal(err)
	}
	if err := UpsertFeedPage(db, []client.FeedStory{s}, "ai", "30d", now); err != nil {
		t.Fatal(err)
	}
	if got, _ := SearchFeedStories(db, "claude", []string{"ai"}, now.Add(-24*time.Hour), 10); len(got) != 1 {
		t.Fatalf("saved headline stopped matching after a blank scrape: %+v", got)
	}
}

func TestSearchFeedStoriesIgnoresSourceTitleAndLabel(t *testing.T) {
	db := openTempDB(t)
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	s := feedStory(t, "c-1", "u1", "Claude ships", "A summary", now.Add(-time.Hour))
	if err := UpsertFeedPage(db, []client.FeedStory{s}, "ai", "30d", now); err != nil {
		t.Fatal(err)
	}
	if err := UpsertCluster(db, diggparse.Cluster{ClusterID: "c-1", ClusterURLID: "u1", Label: "Zebra label", SourceTitle: "Walrus source"}, now); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"walrus", "zebra"} {
		if got, _ := SearchFeedStories(db, q, []string{"ai"}, now.Add(-24*time.Hour), 10); len(got) != 0 {
			t.Fatalf("%q only appears in label/source_title but matched: %+v", q, got)
		}
	}
	if got, _ := SearchFeedStories(db, "summary", []string{"ai"}, now.Add(-24*time.Hour), 10); len(got) != 1 {
		t.Fatalf("summary word should still match: %+v", got)
	}
}

// Concurrent searches each open their own handle on one database. The
// page write reads before it writes, so a deferred transaction would lose
// its write upgrade to another writer with SQLITE_BUSY.
func TestUpsertFeedPageConcurrentWriters(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	const writers = 4
	dbs := make([]*sql.DB, writers)
	for i := range dbs {
		s, err := store.Open(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { s.Close() })
		if err := EnsureSchema(s.DB()); err != nil {
			t.Fatal(err)
		}
		dbs[i] = s.DB()
	}
	now := time.Now()
	const pages = 20
	work := make([][][]client.FeedStory, writers)
	for w := range work {
		for p := 0; p < pages; p++ {
			var page []client.FeedStory
			for i := 0; i < 20; i++ {
				id := fmt.Sprintf("c%d", (p*20+i+w*7)%60)
				page = append(page, feedStory(t, id, id+"-u", fmt.Sprintf("Story %s by writer %d", id, w), "", now))
			}
			work[w] = append(work[w], page)
		}
	}
	var wg sync.WaitGroup
	errs := make(chan error, writers*pages)
	for w, db := range dbs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, page := range work[w] {
				if err := UpsertFeedPage(db, page, "ai", "30d", now); err != nil {
					errs <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent page write failed: %v", err)
	}
}
