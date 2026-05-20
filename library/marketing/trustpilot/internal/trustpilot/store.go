package trustpilot

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// EnsureSchema creates the Trustpilot-specific tables and FTS5 indexes on top
// of the generic Store. Safe to call on every CLI invocation; uses
// CREATE TABLE IF NOT EXISTS so re-runs are no-ops.
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS tp_companies (
			domain TEXT PRIMARY KEY,
			display_name TEXT NOT NULL,
			trust_score REAL,
			stars REAL,
			number_of_reviews INTEGER,
			website_url TEXT,
			is_claimed INTEGER,
			is_collecting_reviews INTEGER,
			ai_summary TEXT,
			ai_summary_model TEXT,
			rating_histogram TEXT,
			categories_json TEXT,
			similar_json TEXT,
			topic_summaries_json TEXT,
			updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS tp_reviews (
			id TEXT PRIMARY KEY,
			domain TEXT NOT NULL,
			rating INTEGER NOT NULL,
			title TEXT,
			text TEXT,
			language TEXT,
			likes INTEGER,
			source TEXT,
			is_verified INTEGER,
			is_filtered INTEGER,
			is_pending INTEGER,
			experienced_date TEXT,
			published_date TEXT,
			updated_date TEXT,
			consumer_id TEXT,
			consumer_name TEXT,
			consumer_country TEXT,
			consumer_review_count INTEGER,
			reply_message TEXT,
			reply_published_date TEXT,
			raw_json TEXT,
			inserted_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tp_reviews_domain ON tp_reviews(domain)`,
		`CREATE INDEX IF NOT EXISTS idx_tp_reviews_domain_published ON tp_reviews(domain, published_date)`,
		`CREATE INDEX IF NOT EXISTS idx_tp_reviews_domain_rating ON tp_reviews(domain, rating)`,
		`CREATE INDEX IF NOT EXISTS idx_tp_reviews_country ON tp_reviews(consumer_country)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS tp_reviews_fts USING fts5(
			title, text, reply_message,
			content='tp_reviews', content_rowid='rowid', tokenize='porter unicode61'
		)`,
		`CREATE TABLE IF NOT EXISTS tp_sync_cursors (
			domain TEXT PRIMARY KEY,
			last_page INTEGER NOT NULL,
			total_pages INTEGER,
			total_count INTEGER,
			last_synced_at TEXT NOT NULL,
			last_buildid TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS tp_session (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			aws_waf_token TEXT,
			cookie_jar TEXT,
			reviews_build_id TEXT,
			search_build_id TEXT,
			harvested_at TEXT,
			user_agent TEXT
		)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("ensure schema (%s...): %w", truncate(s, 80), err)
		}
	}
	return nil
}

// UpsertCompany persists a BusinessUnit snapshot for the given canonical
// domain. Idempotent on `domain`.
func UpsertCompany(ctx context.Context, db *sql.DB, bu BusinessUnit) error {
	cat, _ := json.Marshal(bu.Categories)
	sim, _ := json.Marshal(bu.SimilarBusinessUnits)
	topics, _ := json.Marshal(bu.TopicAISummaries)
	hist, _ := json.Marshal(bu.RatingHistogram)
	_, err := db.ExecContext(ctx, `INSERT INTO tp_companies (
			domain, display_name, trust_score, stars, number_of_reviews,
			website_url, is_claimed, is_collecting_reviews,
			ai_summary, ai_summary_model, rating_histogram,
			categories_json, similar_json, topic_summaries_json, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(domain) DO UPDATE SET
			display_name=excluded.display_name,
			trust_score=excluded.trust_score,
			stars=excluded.stars,
			number_of_reviews=excluded.number_of_reviews,
			website_url=excluded.website_url,
			is_claimed=excluded.is_claimed,
			is_collecting_reviews=excluded.is_collecting_reviews,
			ai_summary=excluded.ai_summary,
			ai_summary_model=excluded.ai_summary_model,
			rating_histogram=excluded.rating_histogram,
			categories_json=excluded.categories_json,
			similar_json=excluded.similar_json,
			topic_summaries_json=excluded.topic_summaries_json,
			updated_at=excluded.updated_at`,
		bu.IdentifyingName, bu.DisplayName, bu.TrustScore, bu.Stars, bu.NumberOfReviews,
		bu.WebsiteURL, boolToInt(bu.IsClaimed), boolToInt(bu.IsCollectingReviews),
		bu.AISummary, bu.AISummaryModelVersion, string(hist),
		string(cat), string(sim), string(topics), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("upsert tp_company %s: %w", bu.IdentifyingName, err)
	}
	return nil
}

// UpsertReviews persists a batch of reviews and keeps the FTS5 index in sync.
// Returns inserted, updated counts.
func UpsertReviews(ctx context.Context, db *sql.DB, reviews []Review) (inserted, updated int, err error) {
	if len(reviews) == 0 {
		return 0, 0, nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, r := range reviews {
		raw, _ := json.Marshal(r)
		// SQLite returns RowsAffected=1 for INSERT and 1 for UPDATE under the
		// ON CONFLICT path used here; we can't tell apart from RowsAffected
		// alone, so use a Probe SELECT to bucket the counters.
		var existing int
		row := tx.QueryRowContext(ctx, `SELECT 1 FROM tp_reviews WHERE id = ?`, r.ID)
		_ = row.Scan(&existing)
		isUpdate := existing == 1

		_, err := tx.ExecContext(ctx, `INSERT INTO tp_reviews (
			id, domain, rating, title, text, language, likes, source,
			is_verified, is_filtered, is_pending,
			experienced_date, published_date, updated_date,
			consumer_id, consumer_name, consumer_country, consumer_review_count,
			reply_message, reply_published_date, raw_json, inserted_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			rating=excluded.rating, title=excluded.title, text=excluded.text,
			language=excluded.language, likes=excluded.likes, source=excluded.source,
			is_verified=excluded.is_verified, is_filtered=excluded.is_filtered, is_pending=excluded.is_pending,
			experienced_date=excluded.experienced_date, published_date=excluded.published_date, updated_date=excluded.updated_date,
			consumer_name=excluded.consumer_name, consumer_country=excluded.consumer_country, consumer_review_count=excluded.consumer_review_count,
			reply_message=excluded.reply_message, reply_published_date=excluded.reply_published_date,
			raw_json=excluded.raw_json`,
			r.ID, r.Domain, r.Rating, r.Title, r.Text, r.Language, r.Likes, r.Source,
			boolToInt(r.IsVerified), boolToInt(r.Filtered), boolToInt(r.IsPending),
			fmtTime(r.ExperiencedDate), fmtTime(r.PublishedDate), fmtTime(r.UpdatedDate),
			r.ConsumerID, r.ConsumerName, r.ConsumerCountry, r.ConsumerNumberOfReviews,
			r.ReplyMessage, fmtTime(r.ReplyPublishedDate), string(raw), now)
		if err != nil {
			return inserted, updated, fmt.Errorf("upsert tp_review %s: %w", r.ID, err)
		}
		// Refresh FTS5: delete-then-insert; using rowid is unreliable across
		// inserts vs updates for SQLite's auto-rowid, so we rebuild for this row.
		_, _ = tx.ExecContext(ctx, `INSERT INTO tp_reviews_fts(tp_reviews_fts, rowid, title, text, reply_message)
			SELECT 'delete', rowid, title, text, reply_message FROM tp_reviews WHERE id = ?`, r.ID)
		_, _ = tx.ExecContext(ctx, `INSERT INTO tp_reviews_fts(rowid, title, text, reply_message)
			SELECT rowid, title, text, reply_message FROM tp_reviews WHERE id = ?`, r.ID)
		if isUpdate {
			updated++
		} else {
			inserted++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit tp_reviews: %w", err)
	}
	return inserted, updated, nil
}

// SaveCursor records the last successfully synced page for a domain so
// subsequent syncs can resume cheaply.
func SaveCursor(ctx context.Context, db *sql.DB, domain string, page, totalPages, totalCount int, buildID string) error {
	// PATCH: Refuse cursor states produced when Trustpilot's filter cap returns totalPages=0.
	if page > 0 && totalPages == 0 {
		return errors.New("refusing to persist degenerate cursor (lastPage > 0 but totalPages == 0); likely Trustpilot filter cap hit")
	}
	_, err := db.ExecContext(ctx, `INSERT INTO tp_sync_cursors (domain, last_page, total_pages, total_count, last_synced_at, last_buildid)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(domain) DO UPDATE SET
			last_page=excluded.last_page,
			total_pages=excluded.total_pages,
			total_count=excluded.total_count,
			last_synced_at=excluded.last_synced_at,
			last_buildid=excluded.last_buildid`,
		domain, page, totalPages, totalCount, time.Now().UTC().Format(time.RFC3339), buildID)
	return err
}

// GetCursor returns the persisted cursor for a domain, or zero values if absent.
func GetCursor(ctx context.Context, db *sql.DB, domain string) (lastPage int, lastSyncedAt time.Time, err error) {
	row := db.QueryRowContext(ctx, `SELECT last_page, last_synced_at FROM tp_sync_cursors WHERE domain = ?`, domain)
	var ts string
	if err := row.Scan(&lastPage, &ts); err != nil {
		if err == sql.ErrNoRows {
			return 0, time.Time{}, nil
		}
		return 0, time.Time{}, err
	}
	t, _ := time.Parse(time.RFC3339, ts)
	return lastPage, t, nil
}

// SaveSession persists the harvested cookie+buildId so subsequent CLI
// invocations can replay without launching Chrome again until expiry.
func SaveSession(ctx context.Context, db *sql.DB, s Session) error {
	_, err := db.ExecContext(ctx, `INSERT INTO tp_session (id, aws_waf_token, cookie_jar, reviews_build_id, search_build_id, harvested_at, user_agent)
		VALUES (1, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			aws_waf_token=excluded.aws_waf_token,
			cookie_jar=excluded.cookie_jar,
			reviews_build_id=excluded.reviews_build_id,
			search_build_id=excluded.search_build_id,
			harvested_at=excluded.harvested_at,
			user_agent=excluded.user_agent`,
		s.AWSWAFToken, s.CookieJar, s.ReviewsBuildID, s.SearchBuildID,
		s.HarvestedAt.UTC().Format(time.RFC3339), s.UserAgent)
	return err
}

// LoadSession returns the persisted session if one exists.
func LoadSession(ctx context.Context, db *sql.DB) (Session, error) {
	var s Session
	var ts string
	row := db.QueryRowContext(ctx, `SELECT aws_waf_token, cookie_jar, reviews_build_id, search_build_id, harvested_at, user_agent FROM tp_session WHERE id = 1`)
	if err := row.Scan(&s.AWSWAFToken, &s.CookieJar, &s.ReviewsBuildID, &s.SearchBuildID, &ts, &s.UserAgent); err != nil {
		if err == sql.ErrNoRows {
			return Session{}, nil
		}
		return Session{}, err
	}
	s.HarvestedAt, _ = time.Parse(time.RFC3339, ts)
	return s, nil
}

// QueryFilters narrows a Reviews query.
type QueryFilters struct {
	Domain   string
	Stars    []int     // 1..5; empty = all
	MinDate  time.Time // zero = unbounded
	MaxDate  time.Time // zero = unbounded
	Language string    // empty = all
	Limit    int       // 0 = unbounded
	OrderBy  string    // "published_date DESC" by default
}

// QueryReviews runs a typed query over tp_reviews.
func QueryReviews(ctx context.Context, db *sql.DB, f QueryFilters) ([]Review, error) {
	var where []string
	var args []any
	if f.Domain != "" {
		where = append(where, "domain = ?")
		args = append(args, f.Domain)
	}
	if len(f.Stars) > 0 {
		ph := make([]string, len(f.Stars))
		for i, s := range f.Stars {
			ph[i] = "?"
			args = append(args, s)
		}
		where = append(where, "rating IN ("+strings.Join(ph, ",")+")")
	}
	if !f.MinDate.IsZero() {
		where = append(where, "published_date >= ?")
		args = append(args, f.MinDate.UTC().Format(time.RFC3339))
	}
	if !f.MaxDate.IsZero() {
		where = append(where, "published_date < ?")
		args = append(args, f.MaxDate.UTC().Format(time.RFC3339))
	}
	if f.Language != "" {
		where = append(where, "language = ?")
		args = append(args, f.Language)
	}
	q := "SELECT raw_json FROM tp_reviews"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	if f.OrderBy == "" {
		q += " ORDER BY published_date DESC"
	} else {
		q += " ORDER BY " + f.OrderBy
	}
	if f.Limit > 0 {
		q += " LIMIT " + strconv.Itoa(f.Limit)
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Review
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var r Review
		if err := json.Unmarshal([]byte(raw), &r); err == nil {
			out = append(out, r)
		}
	}
	return out, rows.Err()
}

// FullTextSearchReviews wraps QueryReviews with an FTS5 match.
func FullTextSearchReviews(ctx context.Context, db *sql.DB, query string, f QueryFilters) ([]Review, error) {
	args := []any{query}
	clauses := []string{"r.rowid IN (SELECT rowid FROM tp_reviews_fts WHERE tp_reviews_fts MATCH ?)"}
	if f.Domain != "" {
		clauses = append(clauses, "r.domain = ?")
		args = append(args, f.Domain)
	}
	if len(f.Stars) > 0 {
		ph := make([]string, len(f.Stars))
		for i, s := range f.Stars {
			ph[i] = "?"
			args = append(args, s)
		}
		clauses = append(clauses, "r.rating IN ("+strings.Join(ph, ",")+")")
	}
	if !f.MinDate.IsZero() {
		clauses = append(clauses, "r.published_date >= ?")
		args = append(args, f.MinDate.UTC().Format(time.RFC3339))
	}
	if f.Language != "" {
		clauses = append(clauses, "r.language = ?")
		args = append(args, f.Language)
	}
	q := "SELECT r.raw_json FROM tp_reviews r WHERE " + strings.Join(clauses, " AND ") + " ORDER BY r.published_date DESC"
	if f.Limit > 0 {
		q += " LIMIT " + strconv.Itoa(f.Limit)
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Review
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var r Review
		if err := json.Unmarshal([]byte(raw), &r); err == nil {
			out = append(out, r)
		}
	}
	return out, rows.Err()
}

// LoadCompany returns the cached BusinessUnit for a domain.
func LoadCompany(ctx context.Context, db *sql.DB, domain string) (BusinessUnit, error) {
	row := db.QueryRowContext(ctx, `SELECT display_name, trust_score, stars, number_of_reviews, website_url,
		is_claimed, is_collecting_reviews, ai_summary, ai_summary_model,
		rating_histogram, categories_json, similar_json, topic_summaries_json FROM tp_companies WHERE domain = ?`, domain)
	var bu BusinessUnit
	bu.IdentifyingName = domain
	var isClaimed, isCollecting int
	var histJSON, catJSON, simJSON, topicJSON string
	if err := row.Scan(&bu.DisplayName, &bu.TrustScore, &bu.Stars, &bu.NumberOfReviews, &bu.WebsiteURL,
		&isClaimed, &isCollecting, &bu.AISummary, &bu.AISummaryModelVersion,
		&histJSON, &catJSON, &simJSON, &topicJSON); err != nil {
		return BusinessUnit{}, err
	}
	bu.IsClaimed = isClaimed == 1
	bu.IsCollectingReviews = isCollecting == 1
	_ = json.Unmarshal([]byte(histJSON), &bu.RatingHistogram)
	_ = json.Unmarshal([]byte(catJSON), &bu.Categories)
	_ = json.Unmarshal([]byte(simJSON), &bu.SimilarBusinessUnits)
	_ = json.Unmarshal([]byte(topicJSON), &bu.TopicAISummaries)
	return bu, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
