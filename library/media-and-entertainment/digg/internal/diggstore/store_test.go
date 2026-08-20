package diggstore

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggparse"

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

func TestUpsertEvent(t *testing.T) {
	db := openTempDB(t)
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	e := diggparse.Event{
		ID:           "e-1",
		Type:         "fast_climb",
		ClusterID:    "c-1",
		Label:        "Fast Climber",
		Delta:        9,
		CurrentRank:  4,
		PreviousRank: 13,
		At:           "2026-05-09T11:00:00Z",
	}
	if err := UpsertEvent(db, e, now); err != nil {
		t.Fatal(err)
	}
	// Idempotent: second insert is a no-op.
	if err := UpsertEvent(db, e, now); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM digg_events WHERE id = ?`, e.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected exactly one event row; got %d", n)
	}
	var typ string
	var delta int
	if err := db.QueryRow(`SELECT type, COALESCE(delta,0) FROM digg_events WHERE id = ?`, e.ID).Scan(&typ, &delta); err != nil {
		t.Fatal(err)
	}
	if typ != "fast_climb" || delta != 9 {
		t.Errorf("event row mismatch: type=%q delta=%d", typ, delta)
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

func TestUpsertClusterPosts_RoundTrip(t *testing.T) {
	db := openTempDB(t)
	// Use real wall-clock now so the TTL math reflects the live
	// time.Since(fetchedAt) check the production reader uses.
	now := time.Now().UTC().Truncate(time.Second)
	body := "hmm"
	posts := []diggparse.ClusterPost{
		{
			PostXID:  "1111",
			PostType: "tweet",
			PostedAt: "2026-05-09T08:02:21+00:00",
			Author: diggparse.ClusterPostAuthor{
				Username: "tszzl", DisplayName: "roon",
				Category: "Researcher", Rank: 32,
			},
			XURL:       "https://x.com/tszzl/status/1111",
			Body:       &body,
			BodyLoaded: true,
			MediaURLs:  []string{"https://pbs.twimg.com/media/AAA.jpg"},
		},
	}
	if err := UpsertClusterPosts(db, "65idu2x5", posts, now); err != nil {
		t.Fatalf("UpsertClusterPosts: %v", err)
	}
	got, ok, fetchedAt, err := GetClusterPosts(db, "65idu2x5", time.Hour)
	if err != nil {
		t.Fatalf("GetClusterPosts: %v", err)
	}
	if !ok {
		t.Fatal("expected cache hit, got miss")
	}
	if !fetchedAt.Equal(now) {
		t.Errorf("fetched_at round-trip mismatch: got %v, want %v", fetchedAt, now)
	}
	if len(got) != 1 || got[0].PostXID != "1111" {
		t.Errorf("round-trip mismatch: got %+v", got)
	}
	if got[0].Body == nil || *got[0].Body != "hmm" {
		t.Errorf("body round-trip lost: got %v", got[0].Body)
	}
	if len(got[0].MediaURLs) != 1 {
		t.Errorf("media URL round-trip mismatch: %v", got[0].MediaURLs)
	}
}

func TestUpsertClusterPosts_TTLMiss(t *testing.T) {
	db := openTempDB(t)
	stale := time.Now().Add(-2 * time.Hour) // older than 1h TTL
	posts := []diggparse.ClusterPost{
		{
			PostXID: "1", PostType: "tweet",
			Author: diggparse.ClusterPostAuthor{Username: "alice"},
		},
	}
	if err := UpsertClusterPosts(db, "abc", posts, stale); err != nil {
		t.Fatalf("UpsertClusterPosts: %v", err)
	}
	// 1h TTL: a row from 2h ago should miss.
	got, ok, _, err := GetClusterPosts(db, "abc", time.Hour)
	if err != nil {
		t.Fatalf("GetClusterPosts: %v", err)
	}
	if ok {
		t.Errorf("expected TTL miss for stale row; got hit with %d posts", len(got))
	}
	// But a 365d TTL should hit (used by the live-fail-with-stale-cache fallback).
	_, ok2, _, err := GetClusterPosts(db, "abc", 365*24*time.Hour)
	if err != nil {
		t.Fatalf("GetClusterPosts (long TTL): %v", err)
	}
	if !ok2 {
		t.Error("long TTL should still find the cached row")
	}
}

func TestUpsertClusterPosts_OverwriteSameKey(t *testing.T) {
	db := openTempDB(t)
	// Anchor on real wall-clock so TTL math reflects production.
	t1 := time.Now().UTC().Add(-30 * time.Minute).Truncate(time.Second)
	t2 := t1.Add(20 * time.Minute)
	body1, body2 := "first", "second"
	v1 := []diggparse.ClusterPost{
		{PostXID: "1", Author: diggparse.ClusterPostAuthor{Username: "a"}, Body: &body1, MediaURLs: []string{}},
	}
	v2 := []diggparse.ClusterPost{
		{PostXID: "1", Author: diggparse.ClusterPostAuthor{Username: "a"}, Body: &body2, MediaURLs: []string{}},
		{PostXID: "2", Author: diggparse.ClusterPostAuthor{Username: "b"}, MediaURLs: []string{}},
	}
	if err := UpsertClusterPosts(db, "k", v1, t1); err != nil {
		t.Fatal(err)
	}
	if err := UpsertClusterPosts(db, "k", v2, t2); err != nil {
		t.Fatal(err)
	}
	got, ok, fetchedAt, err := GetClusterPosts(db, "k", time.Hour)
	if err != nil || !ok {
		t.Fatalf("hit failed: ok=%v err=%v", ok, err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 posts after overwrite, got %d", len(got))
	}
	if got[0].Body == nil || *got[0].Body != "second" {
		t.Errorf("overwrite did not update body; got %v", got[0].Body)
	}
	if !fetchedAt.Equal(t2) {
		t.Errorf("fetched_at not refreshed; got %v want %v", fetchedAt, t2)
	}
}

func TestGetClusterPosts_EmptyKeyOrTTLReturnsMiss(t *testing.T) {
	db := openTempDB(t)
	if _, ok, _, err := GetClusterPosts(db, "", time.Hour); ok || err != nil {
		t.Errorf("empty key should return (miss, nil); got ok=%v err=%v", ok, err)
	}
	if _, ok, _, err := GetClusterPosts(db, "x", 0); ok || err != nil {
		t.Errorf("zero TTL should return (miss, nil); got ok=%v err=%v", ok, err)
	}
}

func TestRecordReplacementsDropsClustersNotSeen(t *testing.T) {
	db := openTempDB(t)
	old := time.Date(2026, 5, 9, 11, 0, 0, 0, time.UTC)
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)

	prev := diggparse.Cluster{ClusterID: "c-old", ClusterURLID: "oldid", Label: "Old", CurrentRank: 5}
	stay := diggparse.Cluster{ClusterID: "c-stay", ClusterURLID: "stayid", Label: "Stays", CurrentRank: 6}
	if err := UpsertCluster(db, prev, old); err != nil {
		t.Fatal(err)
	}
	if err := UpsertCluster(db, stay, old); err != nil {
		t.Fatal(err)
	}

	// At "now", we observe only c-stay. c-old should be recorded as a replacement.
	if err := UpsertCluster(db, stay, now); err != nil {
		t.Fatal(err)
	}
	observed := map[string]bool{"c-stay": true}
	if err := RecordReplacements(db, observed, now); err != nil {
		t.Fatal(err)
	}
	var rationale string
	var prevRank int
	if err := db.QueryRow(`SELECT rationale, previous_rank FROM digg_replacements WHERE cluster_id = ?`, "c-old").Scan(&rationale, &prevRank); err != nil {
		t.Fatalf("replacement row not written: %v", err)
	}
	if rationale == "" {
		t.Errorf("rationale should be populated even when upstream didn't publish one")
	}
	if prevRank != 5 {
		t.Errorf("previous_rank should be 5; got %d", prevRank)
	}
}
