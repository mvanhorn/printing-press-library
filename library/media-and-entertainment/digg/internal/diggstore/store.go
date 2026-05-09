// Package diggstore extends the generated SQLite store with the
// Digg-AI-specific tables that hold parsed clusters, snapshots,
// authors, and events.
//
// Schema additions live here (rather than in the generated store
// package) so a regeneration of the printed CLI does not blow them
// away. EnsureSchema is idempotent and can run on every command.
package diggstore

import (
	"database/sql"
	"encoding/json"
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

		// Authors (the Digg AI 1000).
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

		// /api/trending/status events.
		`CREATE TABLE IF NOT EXISTS digg_events (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			run_id TEXT,
			cluster_id TEXT,
			label TEXT,
			username TEXT,
			post_type TEXT,
			post_x_id TEXT,
			permalink TEXT,
			delta INTEGER,
			current_rank INTEGER,
			previous_rank INTEGER,
			count INTEGER,
			total INTEGER,
			original_posts INTEGER,
			retweets INTEGER,
			quote_tweets INTEGER,
			replies INTEGER,
			links INTEGER,
			videos INTEGER,
			images INTEGER,
			embedded_count INTEGER,
			total_count INTEGER,
			at TEXT,
			created_at TEXT,
			dedupe_key TEXT,
			raw_json TEXT,
			fetched_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_digg_events_type_at ON digg_events(type, at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_digg_events_cluster ON digg_events(cluster_id)`,
		`CREATE INDEX IF NOT EXISTS idx_digg_events_at ON digg_events(at DESC)`,

		// Replacement archaeology — derived during sync.
		`CREATE TABLE IF NOT EXISTS digg_replacements (
			cluster_id TEXT NOT NULL,
			observed_at TEXT NOT NULL,
			rationale TEXT,
			previous_rank INTEGER,
			cluster_url_id TEXT,
			label TEXT,
			PRIMARY KEY (cluster_id, observed_at)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_digg_replacements_at ON digg_replacements(observed_at DESC)`,

		// FTS5 over cluster searchable text.
		`CREATE VIRTUAL TABLE IF NOT EXISTS digg_clusters_fts USING fts5(
			cluster_id UNINDEXED,
			cluster_url_id UNINDEXED,
			label,
			title,
			tldr,
			source_title,
			tokenize='porter unicode61'
		)`,

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

		// Per-cluster posts cache. One row per clusterUrlId; the parsed
		// posts are stored as a JSON-encoded blob so the schema doesn't
		// have to track the structured-post-plus-DOM-correlation shape
		// in normalized form. fetched_at is the cache freshness anchor;
		// `posts <id>` and `story <id>` honor a 1h TTL by default and
		// can bypass via --no-cache.
		`CREATE TABLE IF NOT EXISTS digg_cluster_posts (
			cluster_url_id TEXT PRIMARY KEY,
			posts_json TEXT NOT NULL,
			fetched_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_digg_cluster_posts_at ON digg_cluster_posts(fetched_at)`,
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

// ensureAuthorsRosterColumns adds the AI-1000-roster columns to
// digg_authors if they don't already exist. SQLite has no `ADD COLUMN IF
// NOT EXISTS`; we read PRAGMA table_info and only run the ALTERs we need.
func ensureAuthorsRosterColumns(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(digg_authors)`)
	if err != nil {
		return fmt.Errorf("reading digg_authors columns: %w", err)
	}
	defer rows.Close()
	have := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("scanning digg_authors columns: %w", err)
		}
		have[name] = true
	}
	if err := rows.Err(); err != nil {
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
		stmt := fmt.Sprintf(`ALTER TABLE digg_authors ADD COLUMN %s %s`, c.name, c.ddl)
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("adding column %s: %w", c.name, err)
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

	// FTS row.
	if _, err := db.Exec(`DELETE FROM digg_clusters_fts WHERE cluster_id = ?`, c.ClusterID); err != nil {
		return fmt.Errorf("fts delete: %w", err)
	}
	if _, err := db.Exec(`
		INSERT INTO digg_clusters_fts (cluster_id, cluster_url_id, label, title, tldr, source_title)
		VALUES (?,?,?,?,?,?)
	`, c.ClusterID, c.ClusterURLID, c.Label, c.Title, c.TLDR, c.SourceTitle); err != nil {
		return fmt.Errorf("fts insert: %w", err)
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

// UpsertEvent writes one event row from /api/trending/status.events[].
func UpsertEvent(db *sql.DB, e diggparse.Event, fetchedAt time.Time) error {
	if e.ID == "" {
		return nil
	}
	rawJSON := raw(e.RawJSON)
	now := fetchedAt.UTC().Format(time.RFC3339Nano)
	_, err := db.Exec(`
		INSERT INTO digg_events (
			id, type, run_id, cluster_id, label, username, post_type, post_x_id, permalink,
			delta, current_rank, previous_rank, count, total,
			original_posts, retweets, quote_tweets, replies, links, videos, images,
			embedded_count, total_count, at, created_at, dedupe_key, raw_json, fetched_at
		) VALUES (?,?,?,?,?,?,?,?,?, ?,?,?,?,?, ?,?,?,?,?,?,?, ?,?,?,?,?,?,?)
		ON CONFLICT(id) DO NOTHING
	`,
		e.ID, e.Type, e.RunID, e.ClusterID, e.Label, e.Username, e.PostType, e.PostXID, e.Permalink,
		e.Delta, e.CurrentRank, e.PreviousRank, e.Count, e.Total,
		e.OriginalPosts, e.Retweets, e.QuoteTweets, e.Replies, e.Links, e.Videos, e.Images,
		e.EmbeddedCount, e.TotalCount, e.At, e.CreatedAt, e.DedupeKey, rawJSON, now,
	)
	if err != nil {
		return fmt.Errorf("upsert event %s: %w", e.ID, err)
	}
	return nil
}

// RecordReplacements compares the cluster IDs we just observed against
// the cluster IDs we had in the local store and records a replacement
// row for any cluster present last sync but missing from the current
// snapshot. The "rationale" is best-effort — Digg only sometimes ships
// it; otherwise we record a synthetic "fell out of feed" rationale.
func RecordReplacements(db *sql.DB, observedClusterIDs map[string]bool, observedAt time.Time) error {
	now := observedAt.UTC().Format(time.RFC3339Nano)
	rows, err := db.Query(`
		SELECT cluster_id, cluster_url_id, label, current_rank, replacement_rationale, last_seen_at
		FROM digg_clusters
		WHERE last_seen_at < ?
		  AND last_seen_at >= datetime(?, '-2 hours')
	`, now, now)
	if err != nil {
		return fmt.Errorf("scanning replacements: %w", err)
	}
	defer rows.Close()

	type pending struct {
		id, urlID, label, rationale string
		rank                        int
	}
	var pendings []pending
	for rows.Next() {
		var id, urlID, label, rationale string
		var rank sql.NullInt64
		var lastSeen string
		if err := rows.Scan(&id, &urlID, &label, &rank, &rationale, &lastSeen); err != nil {
			return err
		}
		if observedClusterIDs[id] {
			continue
		}
		r := rationale
		if r == "" {
			r = "fell out of feed (no rationale published)"
		}
		pendings = append(pendings, pending{id: id, urlID: urlID, label: label, rationale: r, rank: int(rank.Int64)})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, p := range pendings {
		if _, err := db.Exec(`
			INSERT OR IGNORE INTO digg_replacements (cluster_id, observed_at, rationale, previous_rank, cluster_url_id, label)
			VALUES (?,?,?,?,?,?)
		`, p.id, now, p.rationale, p.rank, p.urlID, p.label); err != nil {
			return fmt.Errorf("record replacement %s: %w", p.id, err)
		}
	}
	return nil
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

// UpsertClusterPosts caches the parsed posts for a single cluster.
// One row per clusterUrlId; subsequent calls overwrite. The posts
// blob is JSON-marshalled directly from the slice so the structured
// shape (per-post body / media_urls / repost_context) round-trips
// untouched on the read side.
//
// fetchedAt is the cache anchor — readers compare it to time.Now()
// against a TTL (1h by default; `--no-cache` bypasses entirely).
// We store the timestamp in RFC3339Nano UTC so SQL ordering stays
// monotonic.
func UpsertClusterPosts(db *sql.DB, clusterUrlID string, posts []diggparse.ClusterPost, fetchedAt time.Time) error {
	if clusterUrlID == "" {
		return fmt.Errorf("UpsertClusterPosts: clusterUrlID required")
	}
	body, err := json.Marshal(posts)
	if err != nil {
		return fmt.Errorf("marshalling cluster posts: %w", err)
	}
	now := fetchedAt.UTC().Format(time.RFC3339Nano)
	_, err = db.Exec(`
		INSERT INTO digg_cluster_posts (cluster_url_id, posts_json, fetched_at)
		VALUES (?,?,?)
		ON CONFLICT(cluster_url_id) DO UPDATE SET
			posts_json = excluded.posts_json,
			fetched_at = excluded.fetched_at
	`, clusterUrlID, string(body), now)
	if err != nil {
		return fmt.Errorf("upsert cluster posts %s: %w", clusterUrlID, err)
	}
	return nil
}

// GetClusterPosts reads the cached posts for one clusterUrlId,
// honoring the supplied TTL. Returns:
//
//   - (posts, true, fetchedAt, nil) when a fresh row exists.
//   - (nil, false, time.Time{}, nil) when no row exists OR the row
//     has aged past the TTL. The caller refetches in either case.
//   - (nil, false, time.Time{}, err) on a SQL or JSON-decode error
//     — surfaced rather than swallowed so flaky storage doesn't
//     silently look like a cache miss.
//
// A zero-or-negative ttl disables the cache (always returns miss).
func GetClusterPosts(db *sql.DB, clusterUrlID string, ttl time.Duration) ([]diggparse.ClusterPost, bool, time.Time, error) {
	if clusterUrlID == "" || ttl <= 0 {
		return nil, false, time.Time{}, nil
	}
	var body, fetchedStr string
	row := db.QueryRow(`SELECT posts_json, fetched_at FROM digg_cluster_posts WHERE cluster_url_id = ?`, clusterUrlID)
	if err := row.Scan(&body, &fetchedStr); err != nil {
		if err == sql.ErrNoRows {
			return nil, false, time.Time{}, nil
		}
		return nil, false, time.Time{}, fmt.Errorf("read cluster posts %s: %w", clusterUrlID, err)
	}
	fetchedAt, perr := time.Parse(time.RFC3339Nano, fetchedStr)
	if perr != nil {
		// Defensive: if the timestamp is unparseable, treat as a miss
		// rather than crashing. A subsequent upsert will fix the row.
		return nil, false, time.Time{}, nil
	}
	if time.Since(fetchedAt) > ttl {
		return nil, false, fetchedAt, nil
	}
	var posts []diggparse.ClusterPost
	if err := json.Unmarshal([]byte(body), &posts); err != nil {
		return nil, false, fetchedAt, fmt.Errorf("decode cluster posts %s: %w", clusterUrlID, err)
	}
	return posts, true, fetchedAt, nil
}
