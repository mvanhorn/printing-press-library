package diggstore

import (
	"context"
	"database/sql"
	"fmt"
	"maps"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggparse"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/store"

	_ "modernc.org/sqlite"
)

func openTempDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := EnsureSchema(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestEnsureSchemaIsIdempotent(t *testing.T) {
	db := openTempDB(t)
	if err := EnsureSchema(db); err != nil {
		t.Errorf("second EnsureSchema call failed: %v", err)
	}
	// Confirm the cluster table is in place.
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM digg_clusters`).Scan(&n); err != nil {
		t.Fatalf("digg_clusters not queryable: %v", err)
	}
	if n != 0 {
		t.Errorf("fresh DB should have zero clusters; got %d", n)
	}
}

func TestUpsertClusterRoundTrip(t *testing.T) {
	db := openTempDB(t)
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	c := diggparse.Cluster{
		ClusterID:    "c-1",
		ClusterURLID: "abcd1234",
		Label:        "hello world",
		TLDR:         "a tldr",
		CurrentRank:  3,
		Delta:        2,
		Authors: []diggparse.ClusterAuthor{
			{Username: "alice", DisplayName: "Alice", PostType: "quote", PostXID: "x1"},
		},
	}
	if err := UpsertCluster(db, c, now); err != nil {
		t.Fatal(err)
	}
	var rank int
	var label, urlID string
	if err := db.QueryRow(`SELECT cluster_url_id, label, current_rank FROM digg_clusters WHERE cluster_id = ?`, c.ClusterID).Scan(&urlID, &label, &rank); err != nil {
		t.Fatal(err)
	}
	if urlID != "abcd1234" || label != "hello world" || rank != 3 {
		t.Errorf("cluster round-trip mismatch: urlID=%q label=%q rank=%d", urlID, label, rank)
	}
	// Author row
	var username string
	if err := db.QueryRow(`SELECT username FROM digg_authors WHERE username = ?`, "alice").Scan(&username); err != nil {
		t.Fatal(err)
	}
	if username != "alice" {
		t.Errorf("author row not written")
	}
	// Membership row
	var postType string
	if err := db.QueryRow(`SELECT post_type FROM digg_cluster_authors WHERE cluster_id = ? AND username = ?`, c.ClusterID, "alice").Scan(&postType); err != nil {
		t.Fatal(err)
	}
	if postType != "quote" {
		t.Errorf("membership row not written; got post_type=%q", postType)
	}
	// Snapshot row
	var snapRank int
	if err := db.QueryRow(`SELECT current_rank FROM digg_snapshots WHERE cluster_id = ?`, c.ClusterID).Scan(&snapRank); err != nil {
		t.Fatal(err)
	}
	if snapRank != 3 {
		t.Errorf("snapshot rank mismatch: got %d", snapRank)
	}
}

func intp(v int) *int { return &v }

func TestEnsureSchemaAddsRosterColumnsToOldDB(t *testing.T) {
	// Simulate an "old schema" database: create digg_authors with the pre-
	// roster column set, then run EnsureSchema and confirm the migration
	// added every roster column. The matching index check confirms the
	// migration helper ran end-to-end.
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "old.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE digg_authors (
		username TEXT PRIMARY KEY,
		display_name TEXT,
		x_id TEXT,
		avatar_url TEXT,
		influence REAL,
		podist REAL,
		contributed_count INTEGER DEFAULT 0,
		last_seen_at TEXT
	)`); err != nil {
		t.Fatal(err)
	}
	// Seed an existing row so we know migration preserves data.
	if _, err := db.Exec(`INSERT INTO digg_authors (username, display_name, contributed_count, last_seen_at)
		VALUES ('legacy_alice', 'Legacy Alice', 7, '2026-04-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}

	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema migration: %v", err)
	}

	// Every roster column we care about must now exist.
	rows, err := db.Query(`PRAGMA table_info(digg_authors)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	have := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		have[name] = true
	}
	for _, want := range []string{
		"rank", "previous_rank", "rank_change", "score",
		"category", "category_rank", "category_confidence",
		"followers_count", "followed_by_count",
		"bio", "github_url",
		"vibe_distribution_json", "vibe_tweet_count",
		"profile_image_url",
	} {
		if !have[want] {
			t.Errorf("expected migration to add column %q", want)
		}
	}

	// Existing row preserved.
	var dn string
	var cc int
	if err := db.QueryRow(`SELECT display_name, contributed_count FROM digg_authors WHERE username = 'legacy_alice'`).Scan(&dn, &cc); err != nil {
		t.Fatalf("legacy row dropped: %v", err)
	}
	if dn != "Legacy Alice" || cc != 7 {
		t.Errorf("legacy row mutated: dn=%q cc=%d", dn, cc)
	}

	// UpsertRoster1000 cleanly inserts after migration.
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	rosterIn := []diggparse.Roster1000Author{
		{
			Rank: 991, Username: "logangraham", DisplayName: "Logan Graham",
			Bio:      "Head of the Frontier Red Team @anthropicai",
			Category: "AI Safety", CategoryRank: 7,
			FollowersCount: 12345, FollowedByCount: 88,
			PreviousRank: nil, RankChange: nil,
			GithubURL: nil,
			VibeDistribution: map[string]float64{
				"troll": 0.1, "informing": 12.3, "teaching": 5.0,
			},
			VibeTweetCount: 200,
			Score:          1.23e-6,
		},
	}
	written, err := UpsertRoster1000(db, rosterIn, now)
	if err != nil {
		t.Fatal(err)
	}
	if written != 1 {
		t.Errorf("UpsertRoster1000 wrote %d, want 1", written)
	}

	var rank, catRank int
	var bio, category string
	if err := db.QueryRow(`SELECT rank, category_rank, bio, category FROM digg_authors WHERE username = 'logangraham'`).Scan(&rank, &catRank, &bio, &category); err != nil {
		t.Fatalf("roster row not written: %v", err)
	}
	if rank != 991 || catRank != 7 || category != "AI Safety" {
		t.Errorf("roster fields wrong: rank=%d catRank=%d category=%q", rank, catRank, category)
	}
	if !contains(bio, "Frontier Red Team") {
		t.Errorf("bio not stored: %q", bio)
	}
}

func TestUpsertRoster1000_TwiceUpdatesNoDup(t *testing.T) {
	db := openTempDB(t)
	t1 := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	first := []diggparse.Roster1000Author{
		{
			Rank: 200, Username: "alice", DisplayName: "Alice", Score: 1.0,
			PreviousRank: intp(210), RankChange: intp(10),
			Category: "Researcher",
		},
	}
	second := []diggparse.Roster1000Author{
		{
			Rank: 195, Username: "alice", DisplayName: "Alice (renamed)", Score: 1.5,
			PreviousRank: intp(200), RankChange: intp(5),
			Category: "Researcher",
		},
	}
	if _, err := UpsertRoster1000(db, first, t1); err != nil {
		t.Fatal(err)
	}
	if _, err := UpsertRoster1000(db, second, t2); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM digg_authors WHERE username = 'alice'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected exactly one row for alice; got %d", n)
	}
	var rank, change int
	var lastSeen string
	if err := db.QueryRow(`SELECT rank, rank_change, last_seen_at FROM digg_authors WHERE username = 'alice'`).Scan(&rank, &change, &lastSeen); err != nil {
		t.Fatal(err)
	}
	if rank != 195 || change != 5 {
		t.Errorf("upsert did not overwrite rank/change; got rank=%d change=%d", rank, change)
	}
	if !contains(lastSeen, "2026-05-10") {
		t.Errorf("last_seen_at should reflect t2; got %q", lastSeen)
	}
}

func TestUpsertRoster1000_RollbackOnFTSFailure(t *testing.T) {
	// Verify the whole roster upsert is atomic: if the FTS write fails
	// mid-loop, the main digg_authors table must roll back too so
	// digg_authors and digg_authors_fts can't drift out of sync.
	//
	// To force a failure, drop the digg_authors_fts table after schema
	// init. The first author's main INSERT will succeed inside the
	// transaction; the subsequent FTS DELETE will fail with "no such
	// table"; the transaction rolls back; digg_authors must be empty.
	db := openTempDB(t)
	if _, err := db.Exec(`DROP TABLE digg_authors_fts`); err != nil {
		t.Fatalf("dropping FTS table: %v", err)
	}

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	in := []diggparse.Roster1000Author{
		{Rank: 5, Username: "alice", DisplayName: "Alice", Score: 1.0, Category: "Researcher"},
		{Rank: 6, Username: "bob", DisplayName: "Bob", Score: 0.9, Category: "Founder"},
	}
	written, err := UpsertRoster1000(db, in, now)
	if err == nil {
		t.Fatal("expected UpsertRoster1000 to fail because the FTS table is gone")
	}
	if written != 0 {
		t.Errorf("on rollback written should be 0; got %d", written)
	}
	// Main table must NOT contain partial rows from before the failure.
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM digg_authors`).Scan(&n); err != nil {
		t.Fatalf("counting digg_authors: %v", err)
	}
	if n != 0 {
		t.Errorf("rollback failed: digg_authors has %d rows, want 0 (transaction should have undone the main INSERT)", n)
	}
}

func TestUpsertRoster1000_BioQueryableViaFTS5(t *testing.T) {
	db := openTempDB(t)
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	gh := "https://github.com/logangraham"
	in := []diggparse.Roster1000Author{
		{
			Rank: 991, Username: "logangraham", DisplayName: "Logan Graham",
			Bio:      "Head of the Frontier Red Team @anthropicai. Make things radically good.",
			Category: "AI Safety", CategoryRank: 7,
			FollowersCount: 12345,
			GithubURL:      &gh,
		},
		{
			Rank: 1, Username: "sama", DisplayName: "Sam Altman",
			Bio: "AI is cool i guess", Category: "Founder", CategoryRank: 1,
			FollowersCount: 4773236,
		},
	}
	if _, err := UpsertRoster1000(db, in, now); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`
		SELECT username FROM digg_authors_fts WHERE digg_authors_fts MATCH ?
	`, `"frontier red team"`)
	if err != nil {
		t.Fatalf("fts query: %v", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			t.Fatal(err)
		}
		got = append(got, u)
	}
	if len(got) != 1 || got[0] != "logangraham" {
		t.Errorf("FTS bio search returned %v, want [logangraham]", got)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		(len(haystack) > 0 && (indexOf(haystack, needle) >= 0)))
}

// indexOf is a tiny strings.Index to avoid pulling strings in.
func indexOf(s, sub string) int {
	if len(sub) == 0 {
		return 0
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestPostRefRoundTrip(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	refs := []PostRef{
		{ShortID: "ab12cd34", ID: "0190a1b2-0000-7000-8000-000000000001", Topic: "ai"},
		{ShortID: "ef56gh78", ID: "0190a1b2-0000-7000-8000-000000000002", Topic: "sports"},
	}
	if err := RecordPostRefs(ctx, db, refs, now); err != nil {
		t.Fatal(err)
	}
	got, ok, err := LookupPostRef(db, "ef56gh78")
	if err != nil || !ok {
		t.Fatalf("LookupPostRef ok=%v err=%v, want the recorded story", ok, err)
	}
	if got != refs[1] {
		t.Fatalf("LookupPostRef = %+v, want %+v", got, refs[1])
	}
	if _, ok, err := LookupPostRef(db, "nosuchid"); err != nil || ok {
		t.Fatalf("unknown short id: ok=%v err=%v, want not found and no error", ok, err)
	}
}

func TestRecordPostRefsUpdatesAMovedStory(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	if err := RecordPostRefs(ctx, db, []PostRef{{ShortID: "ab12cd34", ID: "uuid-1", Topic: "ai"}}, now); err != nil {
		t.Fatal(err)
	}
	if err := RecordPostRefs(ctx, db, []PostRef{{ShortID: "ab12cd34", ID: "uuid-1", Topic: "tech"}}, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, ok, err := LookupPostRef(db, "ab12cd34")
	if err != nil || !ok || got.Topic != "tech" {
		t.Fatalf("LookupPostRef = %+v ok=%v err=%v, want the latest topic", got, ok, err)
	}
}

// Each topic's baseline is its own last poll: a story listed in two
// topics keeps a rank per topic, and a story that left a topic's list is
// gone from that topic's baseline.
func TestWatchBaselineIsKeptPerTopic(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)

	if got, err := LoadWatchBaseline(ctx, db, "ai"); err != nil || len(got) != 0 {
		t.Fatalf("never-polled topic: %v err=%v, want empty and no error", got, err)
	}
	if err := SaveWatchBaseline(ctx, db, "ai", map[string]int{"shared": 1, "ai-only": 2}, now); err != nil {
		t.Fatal(err)
	}
	if err := SaveWatchBaseline(ctx, db, "sports", map[string]int{"shared": 7}, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := SaveWatchBaseline(ctx, db, "ai", map[string]int{"shared": 3, "ai-new": 1}, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	// A poll that found no stories leaves the last real poll in place.
	if err := SaveWatchBaseline(ctx, db, "sports", nil, now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}

	ai, err := LoadWatchBaseline(ctx, db, "ai")
	if err != nil || !maps.Equal(ai, map[string]int{"shared": 3, "ai-new": 1}) {
		t.Errorf("ai baseline = %v err=%v, want its latest poll only", ai, err)
	}
	sports, err := LoadWatchBaseline(ctx, db, "sports")
	if err != nil || !maps.Equal(sports, map[string]int{"shared": 7}) {
		t.Errorf("sports baseline = %v err=%v, want its own poll", sports, err)
	}
}

// Watchers of different topics are separate processes writing one store.
// Their writes queue; none fails and none lands in the other's baseline.
func TestWatchBaselineConcurrentTopicsShareOneStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	ctx := context.Background()
	const rounds = 25
	topics := []string{"ai", "sports", "tech"}
	// One handle per watcher, opened the way each CLI process opens it.
	dbs := map[string]*sql.DB{}
	for _, topic := range topics {
		s, err := store.OpenWithContext(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { s.Close() })
		if err := EnsureSchema(s.DB()); err != nil {
			t.Fatal(err)
		}
		dbs[topic] = s.DB()
	}
	errs := make(chan error, len(topics))
	for _, topic := range topics {
		db := dbs[topic]
		go func() {
			for i := 1; i <= rounds; i++ {
				if err := SaveWatchBaseline(ctx, db, topic, map[string]int{"shared": i, topic + "-only": i}, time.Now()); err != nil {
					errs <- fmt.Errorf("%s round %d: %w", topic, i, err)
					return
				}
			}
			errs <- nil
		}()
	}
	for range topics {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	for _, topic := range topics {
		got, err := LoadWatchBaseline(ctx, dbs[topic], topic)
		if want := map[string]int{"shared": rounds, topic + "-only": rounds}; err != nil || !maps.Equal(got, want) {
			t.Errorf("%s baseline = %v err=%v, want %v", topic, got, err, want)
		}
	}
}
