package diggstore

// PATCH(digg-feed-continuous-replaces-removed-search-and-trending): store
// side of the /api/feed/continuous ingest. Feed rows share digg_clusters
// with the ranked-page scrape but own a separate set of columns, record
// which Digg sections each story appeared in, and keep per-section
// freshness in sync_state so search can tell a current copy from a stale
// one.

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/client"
)

// ensureFeedSchema adds the feed-owned cluster columns, the topic
// membership table, and sync_state (normally created by the generated
// store; repeated here so this package works on a bare database).
func ensureFeedSchema(db *sql.DB) error {
	have, err := tableColumns(db, "digg_clusters")
	if err != nil {
		return err
	}
	for _, c := range []struct{ name, ddl string }{
		{"created_at", "TEXT"},
		{"last_source_post_at", "TEXT"},
		{"post_count", "INTEGER"},
		{"unique_authors", "INTEGER"},
		{"digg_score", "REAL"},
		{"feed_rank", "INTEGER"},
	} {
		if have[c.name] {
			continue
		}
		if _, err := db.Exec(fmt.Sprintf(`ALTER TABLE digg_clusters ADD COLUMN %s %s`, c.name, c.ddl)); err != nil {
			return fmt.Errorf("adding digg_clusters.%s: %w", c.name, err)
		}
	}
	stmts := []string{
		`CREATE INDEX IF NOT EXISTS idx_digg_clusters_created ON digg_clusters(created_at)`,
		// One row per (cluster, Digg section) the feed listed it under.
		`CREATE TABLE IF NOT EXISTS digg_cluster_topics (
			cluster_id TEXT NOT NULL,
			topic TEXT NOT NULL,
			feed_range TEXT,
			rank INTEGER,
			seen_at TEXT NOT NULL,
			PRIMARY KEY (cluster_id, topic)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_digg_cluster_topics_topic ON digg_cluster_topics(topic)`,
		`CREATE TABLE IF NOT EXISTS sync_state (
			resource_type TEXT PRIMARY KEY,
			last_cursor TEXT,
			last_synced_at DATETIME,
			total_count INTEGER DEFAULT 0
		)`,
	}
	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("ensuring digg feed schema: %w (stmt: %s)", err, firstLine(q))
		}
	}
	return nil
}

func tableColumns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return nil, fmt.Errorf("reading %s columns: %w", table, err)
	}
	defer rows.Close()
	have := map[string]bool{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, fmt.Errorf("scanning %s columns: %w", table, err)
		}
		have[name] = true
	}
	return have, rows.Err()
}

// UpsertFeedPage writes one feed page in a single transaction. It updates
// only the columns the feed supplies; the scrape-owned rank, delta, score
// and position columns are never touched. A cluster first seen through the
// feed gets an empty last_seen_at so it stays out of the latest-scrape set
// that `top` and `rising` read.
func UpsertFeedPage(db *sql.DB, stories []client.FeedStory, topic, feedRange string, seenAt time.Time) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := seenAt.UTC().Format(time.RFC3339Nano)
	for _, s := range stories {
		if s.ClusterID == "" {
			continue
		}
		authorsJSON, _ := json.Marshal(s.Authors)
		if _, err := tx.Exec(`
			INSERT INTO digg_clusters (
				cluster_id, cluster_url_id, title, tldr, topic,
				created_at, last_source_post_at, post_count, unique_authors, digg_score, feed_rank,
				likes, views, comments, quotes, replies, bookmarks,
				authors_json, raw_json, fetched_at, last_seen_at
			) VALUES (?,?,?,?,?, ?,?,?,?,?,?, ?,?,?,?,?,?, ?,?,?, '')
			ON CONFLICT(cluster_id) DO UPDATE SET
				cluster_url_id=COALESCE(NULLIF(excluded.cluster_url_id,''), digg_clusters.cluster_url_id),
				title=COALESCE(NULLIF(excluded.title,''), digg_clusters.title),
				tldr=COALESCE(NULLIF(excluded.tldr,''), digg_clusters.tldr),
				topic=COALESCE(digg_clusters.topic, excluded.topic),
				created_at=COALESCE(NULLIF(excluded.created_at,''), digg_clusters.created_at),
				last_source_post_at=COALESCE(NULLIF(excluded.last_source_post_at,''), digg_clusters.last_source_post_at),
				post_count=excluded.post_count,
				unique_authors=excluded.unique_authors,
				digg_score=excluded.digg_score,
				feed_rank=excluded.feed_rank,
				likes=excluded.likes,
				views=excluded.views,
				comments=excluded.comments,
				quotes=excluded.quotes,
				replies=excluded.replies,
				bookmarks=excluded.bookmarks,
				authors_json=COALESCE(digg_clusters.authors_json, excluded.authors_json),
				raw_json=COALESCE(digg_clusters.raw_json, excluded.raw_json)
		`,
			s.ClusterID, s.ClusterURLID, s.Summary.Title, s.Summary.Description, topic,
			normalizeFeedTime(s.CreatedAt), normalizeFeedTime(s.LastSourcePostAt), s.PostCount, s.UniqueAuthors, s.DiggScore, s.Rank,
			s.Likes, s.Views, s.Comments, s.Quotes, s.Replies, s.Bookmarks,
			string(authorsJSON), raw(s.Raw), now,
		); err != nil {
			return fmt.Errorf("upsert feed story %s: %w", s.ClusterID, err)
		}
		if _, err := tx.Exec(`
			INSERT INTO digg_cluster_topics (cluster_id, topic, feed_range, rank, seen_at)
			VALUES (?,?,?,?,?)
			ON CONFLICT(cluster_id, topic) DO UPDATE SET
				feed_range=excluded.feed_range, rank=excluded.rank, seen_at=excluded.seen_at
		`, s.ClusterID, topic, feedRange, s.Rank, now); err != nil {
			return fmt.Errorf("record topic %s for %s: %w", topic, s.ClusterID, err)
		}
		if _, err := tx.Exec(`DELETE FROM digg_clusters_fts WHERE cluster_id = ?`, s.ClusterID); err != nil {
			return fmt.Errorf("fts delete: %w", err)
		}
		if _, err := tx.Exec(`
			INSERT INTO digg_clusters_fts (cluster_id, cluster_url_id, label, title, tldr, source_title)
			SELECT cluster_id, cluster_url_id, label, title, tldr, source_title FROM digg_clusters WHERE cluster_id = ?
		`, s.ClusterID); err != nil {
			return fmt.Errorf("fts insert: %w", err)
		}
	}
	return tx.Commit()
}

// normalizeFeedTime rewrites the feed's offset timestamps as UTC RFC3339 so
// stored values compare correctly as strings. Unparseable input is kept.
func normalizeFeedTime(s string) string {
	if s == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return s
	}
	return t.UTC().Format(time.RFC3339)
}

func feedSyncKey(topic, feedRange string) string {
	return "feed:" + topic + ":" + feedRange
}

// MarkFeedSynced records a completed feed walk for one topic and range.
// Callers must not call it for a walk that failed or was cut short.
func MarkFeedSynced(db *sql.DB, topic, feedRange string, stories int, at time.Time) error {
	_, err := db.Exec(`
		INSERT INTO sync_state (resource_type, last_cursor, last_synced_at, total_count)
		VALUES (?, '', ?, ?)
		ON CONFLICT(resource_type) DO UPDATE SET
			last_synced_at = excluded.last_synced_at, total_count = excluded.total_count`,
		feedSyncKey(topic, feedRange), at.UTC(), stories)
	return err
}

// FeedSyncedAt returns when topic was last fully walked at feedRange.
// ok is false when it never was.
func FeedSyncedAt(db *sql.DB, topic, feedRange string) (time.Time, bool, error) {
	var at time.Time
	err := db.QueryRow(`SELECT last_synced_at FROM sync_state WHERE resource_type = ?`,
		feedSyncKey(topic, feedRange)).Scan(&at)
	if err == sql.ErrNoRows {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return at, !at.IsZero(), nil
}

// FeedMatch is one stored feed story matching a search.
type FeedMatch struct {
	ClusterID     string
	ClusterURLID  string
	Title         string
	TLDR          string
	Rank          int
	PostCount     int
	UniqueAuthors int
	CreatedAt     time.Time
}

// SearchFeedStories full-text matches query against stored stories the feed
// listed under any of topics and created at or after since. Each query word
// must appear; words are quoted so punctuation never reaches FTS5 syntax.
func SearchFeedStories(db *sql.DB, query string, topics []string, since time.Time, limit int) ([]FeedMatch, error) {
	match := ftsQuery(query)
	if match == "" || len(topics) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = -1
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(topics)), ",")
	args := []any{match, since.UTC().Format(time.RFC3339)}
	for _, t := range topics {
		args = append(args, t)
	}
	args = append(args, limit)
	rows, err := db.Query(`
		SELECT c.cluster_id, COALESCE(c.cluster_url_id,''), COALESCE(c.title, c.label, ''), COALESCE(c.tldr,''),
			COALESCE(c.feed_rank,0), COALESCE(c.post_count,0), COALESCE(c.unique_authors,0), c.created_at
		FROM digg_clusters_fts f JOIN digg_clusters c ON c.cluster_id = f.cluster_id
		WHERE digg_clusters_fts MATCH ?
			AND c.created_at IS NOT NULL AND c.created_at >= ?
			AND EXISTS (SELECT 1 FROM digg_cluster_topics t WHERE t.cluster_id = c.cluster_id AND t.topic IN (`+placeholders+`))
		ORDER BY f.rank
		LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("feed search: %w", err)
	}
	defer rows.Close()
	var out []FeedMatch
	for rows.Next() {
		var m FeedMatch
		var created string
		if err := rows.Scan(&m.ClusterID, &m.ClusterURLID, &m.Title, &m.TLDR, &m.Rank, &m.PostCount, &m.UniqueAuthors, &created); err != nil {
			return nil, err
		}
		m.CreatedAt, _ = time.Parse(time.RFC3339, created)
		out = append(out, m)
	}
	return out, rows.Err()
}

func ftsQuery(q string) string {
	var parts []string
	for _, w := range strings.Fields(q) {
		w = strings.ReplaceAll(w, `"`, "")
		if w != "" {
			parts = append(parts, `"`+w+`"`)
		}
	}
	return strings.Join(parts, " ")
}
