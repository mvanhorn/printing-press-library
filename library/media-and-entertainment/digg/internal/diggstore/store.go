// Package diggstore extends the generated SQLite store with the
// Digg-specific tables that hold ranked-list snapshots (clusters and
// their rank history), authors, the story ids search has seen, and each
// watched topic's last poll.
//
// Schema additions live here (rather than in the generated store
// package) so a regeneration of the printed CLI does not blow them
// away. EnsureSchema is idempotent and can run on every command.
package diggstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggparse"
)

// EnsureSchema creates the Digg-specific tables if they don't already
// exist. Safe to call repeatedly; uses CREATE TABLE IF NOT EXISTS.
func EnsureSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS digg_clusters (
			cluster_id TEXT PRIMARY KEY,
			cluster_url_id TEXT,
			label TEXT,
			title TEXT,
			tldr TEXT,
			url TEXT,
			permalink TEXT,
			topic TEXT,
			current_rank INTEGER,
			peak_rank INTEGER,
			previous_rank INTEGER,
			delta INTEGER,
			gravity_score REAL,
			score_components_json TEXT,
			evidence_json TEXT,
			numerator_count INTEGER,
			numerator_label TEXT,
			percent_above_average REAL,
			replacement_rationale TEXT,
			pos6h REAL, pos12h REAL, pos24h REAL, pos_last REAL,
			bookmarks INTEGER, likes INTEGER, comments INTEGER, replies INTEGER,
			quotes INTEGER, views INTEGER, view_count INTEGER, impressions INTEGER,
			retweets INTEGER, quote_tweets INTEGER,
			source_title TEXT,
			hacker_news_json TEXT,
			techmeme_json TEXT,
			external_feeds_json TEXT,
			authors_json TEXT,
			activity_at TEXT,
			computed_at TEXT,
			first_post_at TEXT,
			raw_json TEXT,
			fetched_at TEXT NOT NULL,
			last_seen_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_digg_clusters_rank ON digg_clusters(current_rank)`,
		`CREATE INDEX IF NOT EXISTS idx_digg_clusters_delta ON digg_clusters(delta DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_digg_clusters_url_id ON digg_clusters(cluster_url_id)`,
		`CREATE INDEX IF NOT EXISTS idx_digg_clusters_last_seen ON digg_clusters(last_seen_at)`,

		// Per-snapshot rank/score history. One row per (cluster, fetched_at).
		`CREATE TABLE IF NOT EXISTS digg_snapshots (
			cluster_id TEXT NOT NULL,
			fetched_at TEXT NOT NULL,
			current_rank INTEGER,
			peak_rank INTEGER,
			previous_rank INTEGER,
			delta INTEGER,
			gravity_score REAL,
			pos6h REAL, pos12h REAL, pos24h REAL,
			likes INTEGER, views INTEGER, impressions INTEGER,
			PRIMARY KEY (cluster_id, fetched_at)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_digg_snapshots_at ON digg_snapshots(fetched_at)`,

		// Authors (Digg's tracked AI-news accounts; upstream calls this the /ai/1000 roster).
		`CREATE TABLE IF NOT EXISTS digg_authors (
			username TEXT PRIMARY KEY,
			display_name TEXT,
			x_id TEXT,
			avatar_url TEXT,
			influence REAL,
			podist REAL,
			contributed_count INTEGER DEFAULT 0,
			last_seen_at TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_digg_authors_influence ON digg_authors(influence DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_digg_authors_count ON digg_authors(contributed_count DESC)`,

		// Author membership in clusters.
		`CREATE TABLE IF NOT EXISTS digg_cluster_authors (
			cluster_id TEXT NOT NULL,
			username TEXT NOT NULL,
			post_type TEXT,
			post_x_id TEXT,
			post_permalink TEXT,
			PRIMARY KEY (cluster_id, username, post_x_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_digg_cluster_authors_user ON digg_cluster_authors(username)`,

		// FTS5 over the AI 1000 author bio + display_name. Lets agents
		// answer "who works on frontier red teaming?" against the cached
		// roster (after a single `authors list` call) without a network
		// round-trip.
		`CREATE VIRTUAL TABLE IF NOT EXISTS digg_authors_fts USING fts5(
			username UNINDEXED,
			display_name,
			bio,
			category,
			tokenize='porter unicode61'
		)`,

		// digg_post_refs maps the short id in a story URL to the UUID
		// Digg's single-story tool needs, plus the story's topic. Search
		// writes it for every result so a later command given only the
		// short id can find the story.
		`CREATE TABLE IF NOT EXISTS digg_post_refs (
			short_id TEXT PRIMARY KEY,
			post_id TEXT NOT NULL,
			topic TEXT NOT NULL DEFAULT '',
			seen_at TEXT NOT NULL
		)`,

		// digg_watch_baselines holds each watched topic's last poll: one
		// row per story in it, with the story's rank in that topic. A
		// story listed in two topics has a row, and a rank, per topic.
		`CREATE TABLE IF NOT EXISTS digg_watch_baselines (
			topic TEXT NOT NULL,
			cluster_id TEXT NOT NULL,
			rank INTEGER NOT NULL,
			polled_at TEXT NOT NULL,
			PRIMARY KEY (topic, cluster_id)
		)`,
	}
	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("ensuring digg schema: %w (stmt: %s)", err, firstLine(q))
		}
	}
	// Migration: add the rich AI-1000-roster columns to digg_authors. The
	// existing schema predates the /ai/1000 ingest; we add columns
	// idempotently so older databases keep working.
	if err := ensureAuthorsRosterColumns(db); err != nil {
		return err
	}
	return nil
}

// addColumn adds one column. Commands started together on a new store
// each see the column missing and each try to add it; the ones that lose
// get "duplicate column name", which means the column is there.
func addColumn(db *sql.DB, table, name, ddl string) error {
	_, err := db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, name, ddl))
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return fmt.Errorf("adding %s.%s: %w", table, name, err)
	}
	return nil
}

// ensureAuthorsRosterColumns adds the AI-1000-roster columns to
// digg_authors if they don't already exist. SQLite has no `ADD COLUMN IF
// NOT EXISTS`; we read PRAGMA table_info and only run the ALTERs we need.
func ensureAuthorsRosterColumns(db *sql.DB) error {
	have, err := tableColumns(db, "digg_authors")
	if err != nil {
		return err
	}
	wantCols := []struct{ name, ddl string }{
		{"rank", "INTEGER"},
		{"previous_rank", "INTEGER"},
		{"rank_change", "INTEGER"},
		{"score", "REAL"},
		{"category", "TEXT"},
		{"category_rank", "INTEGER"},
		{"category_confidence", "REAL"},
		{"followers_count", "INTEGER"},
		{"followed_by_count", "INTEGER"},
		{"bio", "TEXT"},
		{"github_url", "TEXT"},
		{"vibe_distribution_json", "TEXT"},
		{"vibe_tweet_count", "INTEGER"},
		{"profile_image_url", "TEXT"},
	}
	for _, c := range wantCols {
		if have[c.name] {
			continue
		}
		if err := addColumn(db, "digg_authors", c.name, c.ddl); err != nil {
			return err
		}
	}
	// Indexes used by `authors list` ranking + filters.
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_digg_authors_rank ON digg_authors(rank)`,
		`CREATE INDEX IF NOT EXISTS idx_digg_authors_rank_change ON digg_authors(rank_change)`,
		`CREATE INDEX IF NOT EXISTS idx_digg_authors_category ON digg_authors(category, category_rank)`,
		`CREATE INDEX IF NOT EXISTS idx_digg_authors_followers ON digg_authors(followers_count DESC)`,
	}
	for _, q := range indexes {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("indexing digg_authors: %w", err)
		}
	}
	return nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i > 0 {
		return s[:i]
	}
	return s
}

// UpsertCluster writes a cluster row. The first time we see a clusterId,
// fetched_at is set to now. Every write updates last_seen_at.
func UpsertCluster(db *sql.DB, c diggparse.Cluster, fetchedAt time.Time) error {
	authorsJSON, _ := json.Marshal(c.Authors)
	scJSON := raw(c.ScoreComponents)
	evJSON := raw(c.Evidence)
	hnJSON := raw(c.HackerNews)
	tmJSON := raw(c.Techmeme)
	extJSON := raw(c.ExternalFeeds)
	rawJSON := raw(c.RawJSON)
	now := fetchedAt.UTC().Format(time.RFC3339Nano)

	_, err := db.Exec(`
		INSERT INTO digg_clusters (
			cluster_id, cluster_url_id, label, title, tldr, url, permalink, topic,
			current_rank, peak_rank, previous_rank, delta, gravity_score,
			score_components_json, evidence_json,
			numerator_count, numerator_label, percent_above_average,
			replacement_rationale,
			pos6h, pos12h, pos24h, pos_last,
			bookmarks, likes, comments, replies, quotes, views, view_count, impressions,
			retweets, quote_tweets, source_title,
			hacker_news_json, techmeme_json, external_feeds_json,
			authors_json, activity_at, computed_at, first_post_at,
			raw_json, fetched_at, last_seen_at
		) VALUES (
			?,?,?,?,?,?,?,?,
			?,?,?,?,?,
			?,?,
			?,?,?,
			?,
			?,?,?,?,
			?,?,?,?,?,?,?,?,
			?,?,?,
			?,?,?,
			?,?,?,?,
			?,?,?
		) ON CONFLICT(cluster_id) DO UPDATE SET
			cluster_url_id=COALESCE(excluded.cluster_url_id, digg_clusters.cluster_url_id),
			label=COALESCE(NULLIF(excluded.label,''), digg_clusters.label),
			title=COALESCE(NULLIF(excluded.title,''), digg_clusters.title),
			tldr=COALESCE(NULLIF(excluded.tldr,''), digg_clusters.tldr),
			url=COALESCE(NULLIF(excluded.url,''), digg_clusters.url),
			permalink=COALESCE(NULLIF(excluded.permalink,''), digg_clusters.permalink),
			topic=COALESCE(NULLIF(excluded.topic,''), digg_clusters.topic),
			current_rank=excluded.current_rank,
			peak_rank=MAX(IFNULL(digg_clusters.peak_rank, 9999), IFNULL(excluded.peak_rank, 9999)) * (CASE WHEN excluded.peak_rank IS NULL AND digg_clusters.peak_rank IS NULL THEN 0 ELSE 1 END),
			previous_rank=excluded.previous_rank,
			delta=excluded.delta,
			gravity_score=excluded.gravity_score,
			score_components_json=COALESCE(excluded.score_components_json, digg_clusters.score_components_json),
			evidence_json=COALESCE(excluded.evidence_json, digg_clusters.evidence_json),
			numerator_count=excluded.numerator_count,
			numerator_label=excluded.numerator_label,
			percent_above_average=excluded.percent_above_average,
			replacement_rationale=COALESCE(NULLIF(excluded.replacement_rationale,''), digg_clusters.replacement_rationale),
			pos6h=excluded.pos6h,
			pos12h=excluded.pos12h,
			pos24h=excluded.pos24h,
			pos_last=excluded.pos_last,
			bookmarks=excluded.bookmarks,
			likes=excluded.likes,
			comments=excluded.comments,
			replies=excluded.replies,
			quotes=excluded.quotes,
			views=excluded.views,
			view_count=excluded.view_count,
			impressions=excluded.impressions,
			retweets=excluded.retweets,
			quote_tweets=excluded.quote_tweets,
			source_title=COALESCE(NULLIF(excluded.source_title,''), digg_clusters.source_title),
			hacker_news_json=COALESCE(excluded.hacker_news_json, digg_clusters.hacker_news_json),
			techmeme_json=COALESCE(excluded.techmeme_json, digg_clusters.techmeme_json),
			external_feeds_json=COALESCE(excluded.external_feeds_json, digg_clusters.external_feeds_json),
			authors_json=COALESCE(NULLIF(excluded.authors_json,'null'), digg_clusters.authors_json),
			activity_at=COALESCE(NULLIF(excluded.activity_at,''), digg_clusters.activity_at),
			computed_at=COALESCE(NULLIF(excluded.computed_at,''), digg_clusters.computed_at),
			first_post_at=COALESCE(NULLIF(excluded.first_post_at,''), digg_clusters.first_post_at),
			raw_json=excluded.raw_json,
			last_seen_at=excluded.last_seen_at
	`,
		c.ClusterID, c.ClusterURLID, c.Label, c.Title, c.TLDR, c.URL, c.Permalink, c.Topic,
		c.CurrentRank, nullableInt(c.PeakRank), c.PreviousRank, c.Delta, c.GravityScore,
		scJSON, evJSON,
		c.NumeratorCount, c.NumeratorLabel, c.PercentAboveAverage,
		c.ReplacementRationale,
		c.Pos6h, c.Pos12h, c.Pos24h, c.PosLast,
		c.Bookmarks, c.Likes, c.Comments, c.Replies, c.Quotes, c.Views, c.ViewCount, c.Impressions,
		c.Retweets, c.QuoteTweets, c.SourceTitle,
		hnJSON, tmJSON, extJSON,
		string(authorsJSON), c.ActivityAt, c.ComputedAt, c.FirstPostAt,
		rawJSON, now, now,
	)
	if err != nil {
		return fmt.Errorf("upsert cluster %s: %w", c.ClusterID, err)
	}

	// Snapshot row.
	if _, err := db.Exec(`
		INSERT OR REPLACE INTO digg_snapshots (
			cluster_id, fetched_at, current_rank, peak_rank, previous_rank, delta,
			gravity_score, pos6h, pos12h, pos24h, likes, views, impressions
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
	`, c.ClusterID, now, c.CurrentRank, nullableInt(c.PeakRank), c.PreviousRank, c.Delta,
		c.GravityScore, c.Pos6h, c.Pos12h, c.Pos24h, c.Likes, c.Views, c.Impressions); err != nil {
		return fmt.Errorf("snapshot %s: %w", c.ClusterID, err)
	}

	// Authors and membership.
	for _, a := range c.Authors {
		if a.Username == "" {
			continue
		}
		if err := upsertAuthor(db, a, now); err != nil {
			return err
		}
		if _, err := db.Exec(`
			INSERT OR REPLACE INTO digg_cluster_authors
			(cluster_id, username, post_type, post_x_id, post_permalink)
			VALUES (?,?,?,?,?)
		`, c.ClusterID, a.Username, a.PostType, a.PostXID, a.PostPermalink); err != nil {
			return fmt.Errorf("cluster_author %s/%s: %w", c.ClusterID, a.Username, err)
		}
	}

	return nil
}

func upsertAuthor(db *sql.DB, a diggparse.ClusterAuthor, now string) error {
	_, err := db.Exec(`
		INSERT INTO digg_authors (username, display_name, x_id, avatar_url, influence, podist, contributed_count, last_seen_at)
		VALUES (?,?,?,?,?,?,1,?)
		ON CONFLICT(username) DO UPDATE SET
			display_name=COALESCE(NULLIF(excluded.display_name,''), digg_authors.display_name),
			x_id=COALESCE(NULLIF(excluded.x_id,''), digg_authors.x_id),
			avatar_url=COALESCE(NULLIF(excluded.avatar_url,''), digg_authors.avatar_url),
			influence=CASE WHEN excluded.influence > 0 THEN excluded.influence ELSE digg_authors.influence END,
			podist=CASE WHEN excluded.podist > 0 THEN excluded.podist ELSE digg_authors.podist END,
			contributed_count=digg_authors.contributed_count + 1,
			last_seen_at=excluded.last_seen_at
	`, a.Username, a.DisplayName, a.XID, a.AvatarURL, a.Influence, a.Podist, now)
	return err
}

func raw(r json.RawMessage) any {
	if len(r) == 0 {
		return nil
	}
	return string(r)
}

func nullableInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

// UpsertRoster1000 writes the parsed /ai/1000 roster into digg_authors.
// One row per username; the rich roster columns (rank, previous_rank,
// rank_change, score, category, category_rank, followers_count, bio,
// github_url, vibe_distribution_json, etc.) are upserted on every call.
//
// The existing digg_authors columns from cluster ingest (display_name,
// x_id, avatar_url, influence, podist, contributed_count) are preserved
// when the roster row already had values populated by sync. The roster
// path overwrites display_name, avatar_url (via profile_image_url), and
// x_id (via target_x_id) only when the existing values are empty —
// callers expect cluster-ingest data to be authoritative for those.
//
// Each upsert also writes a parallel row into digg_authors_fts so that
// `search "<keyword>" --data-source local` can match on bio text.
//
// All 3000+ statements (one main upsert + one FTS delete + one FTS
// insert per author) run inside a single transaction. If any statement
// fails mid-loop we ROLLBACK so digg_authors and digg_authors_fts can't
// drift out of sync (e.g. main row updated but FTS row missing or
// stale). Returns the count actually committed; on error returns 0
// and the wrapped failure.
func UpsertRoster1000(db *sql.DB, authors []diggparse.Roster1000Author, fetchedAt time.Time) (int, error) {
	if len(authors) == 0 {
		return 0, nil
	}
	now := fetchedAt.UTC().Format(time.RFC3339Nano)

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin roster transaction: %w", err)
	}
	// Deferred rollback is a no-op after a successful commit (sql.ErrTxDone).
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	written := 0
	for _, a := range authors {
		if a.Username == "" {
			continue
		}
		var prevRank, rankChange any
		if a.PreviousRank != nil {
			prevRank = *a.PreviousRank
		}
		if a.RankChange != nil {
			rankChange = *a.RankChange
		}
		var githubURL any
		if a.GithubURL != nil {
			githubURL = *a.GithubURL
		}
		var vibeJSON any
		if len(a.VibeDistribution) > 0 {
			b, jerr := json.Marshal(a.VibeDistribution)
			if jerr == nil {
				vibeJSON = string(b)
			}
		}
		_, err := tx.Exec(`
			INSERT INTO digg_authors (
				username, display_name, x_id, avatar_url,
				rank, previous_rank, rank_change, score,
				category, category_rank, category_confidence,
				followers_count, followed_by_count,
				bio, github_url,
				vibe_distribution_json, vibe_tweet_count,
				profile_image_url, last_seen_at
			) VALUES (
				?,?,?,?,
				?,?,?,?,
				?,?,?,
				?,?,
				?,?,
				?,?,
				?,?
			)
			ON CONFLICT(username) DO UPDATE SET
				display_name=COALESCE(NULLIF(digg_authors.display_name,''), excluded.display_name),
				x_id=COALESCE(NULLIF(digg_authors.x_id,''), excluded.x_id),
				avatar_url=COALESCE(NULLIF(digg_authors.avatar_url,''), excluded.avatar_url),
				rank=excluded.rank,
				previous_rank=excluded.previous_rank,
				rank_change=excluded.rank_change,
				score=excluded.score,
				category=COALESCE(NULLIF(excluded.category,''), digg_authors.category),
				category_rank=excluded.category_rank,
				category_confidence=excluded.category_confidence,
				followers_count=excluded.followers_count,
				followed_by_count=excluded.followed_by_count,
				bio=COALESCE(NULLIF(excluded.bio,''), digg_authors.bio),
				github_url=COALESCE(excluded.github_url, digg_authors.github_url),
				vibe_distribution_json=COALESCE(excluded.vibe_distribution_json, digg_authors.vibe_distribution_json),
				vibe_tweet_count=excluded.vibe_tweet_count,
				profile_image_url=COALESCE(NULLIF(excluded.profile_image_url,''), digg_authors.profile_image_url),
				last_seen_at=excluded.last_seen_at
		`,
			a.Username, a.DisplayName, a.TargetXID, a.ProfileImageURL,
			a.Rank, prevRank, rankChange, a.Score,
			a.Category, nullableInt(a.CategoryRank), a.CategoryConfidence,
			a.FollowersCount, a.FollowedByCount,
			a.Bio, githubURL,
			vibeJSON, nullableInt(a.VibeTweetCount),
			a.ProfileImageURL, now,
		)
		if err != nil {
			return 0, fmt.Errorf("upsert roster author %s: %w", a.Username, err)
		}

		// FTS row: refresh atomically.
		if _, err := tx.Exec(`DELETE FROM digg_authors_fts WHERE username = ?`, a.Username); err != nil {
			return 0, fmt.Errorf("fts delete @%s: %w", a.Username, err)
		}
		if _, err := tx.Exec(`
			INSERT INTO digg_authors_fts (username, display_name, bio, category)
			VALUES (?,?,?,?)
		`, a.Username, a.DisplayName, a.Bio, a.Category); err != nil {
			return 0, fmt.Errorf("fts insert @%s: %w", a.Username, err)
		}
		written++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit roster transaction: %w", err)
	}
	committed = true
	return written, nil
}

// PostRef identifies one Digg story three ways: the short id from its
// URL, its UUID, and the topic it is filed under.
type PostRef struct {
	ShortID string
	ID      string
	Topic   string
}

// RecordPostRefs stores refs, replacing any earlier record of the same
// short id. It takes the write lock up front so concurrent searches on
// one store queue instead of failing.
func RecordPostRefs(ctx context.Context, db *sql.DB, refs []PostRef, seenAt time.Time) error {
	if len(refs) == 0 {
		return nil
	}
	now := seenAt.UTC().Format(time.RFC3339)
	return withWriteTx(ctx, db, func(tx *sql.Conn) error {
		for _, r := range refs {
			if r.ShortID == "" || r.ID == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO digg_post_refs (short_id, post_id, topic, seen_at)
				VALUES (?, ?, ?, ?)
				ON CONFLICT(short_id) DO UPDATE SET post_id = excluded.post_id, topic = excluded.topic, seen_at = excluded.seen_at`,
				r.ShortID, r.ID, r.Topic, now); err != nil {
				return fmt.Errorf("recording story id %s: %w", r.ShortID, err)
			}
		}
		return nil
	})
}

// LookupPostRef returns the record for a story's short id, or false when
// no search has seen that story.
func LookupPostRef(db *sql.DB, shortID string) (PostRef, bool, error) {
	ref := PostRef{ShortID: shortID}
	err := db.QueryRow(`SELECT post_id, topic FROM digg_post_refs WHERE short_id = ?`, shortID).Scan(&ref.ID, &ref.Topic)
	if errors.Is(err, sql.ErrNoRows) {
		return PostRef{}, false, nil
	}
	if err != nil {
		return PostRef{}, false, err
	}
	return ref, true, nil
}

// SaveWatchBaseline replaces topic's stored poll with ranks (rank by
// cluster id), leaving other topics' polls alone. An empty poll is not
// stored, so a page that parsed to nothing keeps the last real poll. The
// swap is one write transaction: a reader sees the old poll or the new.
func SaveWatchBaseline(ctx context.Context, db *sql.DB, topic string, ranks map[string]int, polledAt time.Time) error {
	if len(ranks) == 0 {
		return nil
	}
	at := polledAt.UTC().Format(time.RFC3339Nano)
	return withWriteTx(ctx, db, func(tx *sql.Conn) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM digg_watch_baselines WHERE topic = ?`, topic); err != nil {
			return fmt.Errorf("clearing %s watch baseline: %w", topic, err)
		}
		for id, rank := range ranks {
			if _, err := tx.ExecContext(ctx, `INSERT INTO digg_watch_baselines (topic, cluster_id, rank, polled_at)
				VALUES (?, ?, ?, ?)`, topic, id, rank, at); err != nil {
				return fmt.Errorf("storing %s watch baseline: %w", topic, err)
			}
		}
		return nil
	})
}

// LoadWatchBaseline returns the ranks of topic's last stored poll by
// cluster id. The map is empty when the topic has never been polled.
func LoadWatchBaseline(ctx context.Context, db *sql.DB, topic string) (map[string]int, error) {
	rows, err := db.QueryContext(ctx, `SELECT cluster_id, rank FROM digg_watch_baselines WHERE topic = ?`, topic)
	if err != nil {
		return nil, fmt.Errorf("reading %s watch baseline: %w", topic, err)
	}
	defer rows.Close()
	ranks := map[string]int{}
	for rows.Next() {
		var id string
		var rank int
		if err := rows.Scan(&id, &rank); err != nil {
			return nil, fmt.Errorf("reading %s watch baseline: %w", topic, err)
		}
		ranks[id] = rank
	}
	return ranks, rows.Err()
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

// writeLockWait bounds how long a write waits for another
// process's write to finish before giving up.
const writeLockWait = 10 * time.Second

// withWriteTx runs fn in a BEGIN IMMEDIATE transaction on one pinned
// connection. Concurrent searches share one database. IMMEDIATE takes the
// write lock up front, so writers queue instead of failing with
// SQLITE_BUSY; BEGIN itself is retried because modernc.org/sqlite's
// busy_timeout does not reliably cover it.
func withWriteTx(ctx context.Context, db *sql.DB, fn func(*sql.Conn) error) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	deadline := time.Now().Add(writeLockWait)
	for backoff := 5 * time.Millisecond; ; backoff = min(backoff*2, 100*time.Millisecond) {
		_, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE")
		if err == nil {
			break
		}
		if !IsBusy(err) || time.Now().After(deadline) {
			return fmt.Errorf("begin write: %w", err)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("begin write: %w", ctx.Err())
		case <-time.After(backoff):
		}
	}
	// ROLLBACK runs on a fresh context so an ended ctx cannot leave the
	// pinned connection inside an open transaction.
	if err := fn(conn); err != nil {
		_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		return fmt.Errorf("commit write: %w", err)
	}
	return nil
}

// IsBusy reports whether err is SQLite refusing a write because another
// process holds the lock.
func IsBusy(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "SQLITE_BUSY") || strings.Contains(msg, "database is locked")
}
