// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// App Store snapshot layer. Every table here is an append-only time series
// (or an idempotent upsert keyed by Apple's ids) that the live commands write
// and the history/transcendence commands read. Commands never write SQL:
// they call the typed accessors below. Timestamps are RFC3339 UTC with second
// precision, so they sort lexically.
package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
)

// appStoreMigrations is applied by migrateExtras inside the migration lock.
// Every statement is idempotent.
var appStoreMigrations = []string{
	`CREATE TABLE IF NOT EXISTS captures (
		id INTEGER PRIMARY KEY,
		captured_at TEXT NOT NULL,
		url TEXT NOT NULL,
		host TEXT,
		status INTEGER,
		outcome TEXT,
		entry_count INTEGER,
		cache_control TEXT,
		x_cache TEXT,
		oldest TEXT,
		newest TEXT,
		parser_version TEXT,
		error TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_captures_url ON captures(url, captured_at)`,
	`CREATE INDEX IF NOT EXISTS idx_captures_at ON captures(captured_at)`,
	`CREATE TABLE IF NOT EXISTS app_snapshots (
		id INTEGER PRIMARY KEY,
		captured_at TEXT NOT NULL,
		storefront TEXT NOT NULL,
		app_id TEXT NOT NULL,
		bundle_id TEXT,
		content_hash TEXT,
		title TEXT,
		subtitle TEXT,
		version TEXT,
		price REAL,
		currency TEXT,
		formatted_price TEXT,
		average_rating REAL,
		rating_count INTEGER,
		editors_choice INTEGER,
		has_iap INTEGER,
		screenshots_hash TEXT,
		privacy_hash TEXT,
		iap_hash TEXT,
		events_hash TEXT,
		chart_position TEXT,
		fields_json TEXT,
		source TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE INDEX IF NOT EXISTS idx_app_snapshots_app ON app_snapshots(app_id, storefront, captured_at)`,
	// age_band is "" for the overall chart of a genre and the Kids age band
	// id (0, 10000, 10001, 10002) for a Kids chart (genre 36 + ageBandId), so
	// a Kids capture never joins the overall genre-36 series. Stores created
	// before the column existed get it from appStoreColumns.
	`CREATE TABLE IF NOT EXISTS chart_snapshots (
		id INTEGER PRIMARY KEY,
		captured_at TEXT NOT NULL,
		storefront TEXT NOT NULL,
		platform TEXT NOT NULL,
		genre_id TEXT NOT NULL,
		age_band TEXT NOT NULL DEFAULT '',
		chart TEXT NOT NULL,
		source TEXT NOT NULL,
		row_limit INTEGER,
		rank INTEGER NOT NULL,
		app_id TEXT NOT NULL,
		title TEXT,
		developer_name TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_chart_snapshots_app ON chart_snapshots(app_id)`,
	`CREATE TABLE IF NOT EXISTS reviews (
		review_id TEXT NOT NULL,
		storefront TEXT NOT NULL,
		app_id TEXT NOT NULL,
		rating INTEGER,
		version TEXT,
		title TEXT,
		body TEXT,
		author TEXT,
		reviewed_at TEXT,
		vote_sum INTEGER,
		vote_count INTEGER,
		sorts_seen TEXT,
		source TEXT,
		developer_response TEXT,
		first_seen_at TEXT,
		last_seen_at TEXT,
		PRIMARY KEY (review_id, storefront)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_reviews_app ON reviews(app_id, storefront, reviewed_at)`,
	// reviews_fts holds its own copy of title/body keyed by a rowid derived
	// from (review_id, storefront), so it never depends on the reviews table's
	// implicit rowid (which VACUUM may renumber). UpsertReviews keeps it in
	// sync inside the same transaction.
	`CREATE VIRTUAL TABLE IF NOT EXISTS reviews_fts USING fts5(
		title, body, review_id UNINDEXED, storefront UNINDEXED, app_id UNINDEXED,
		tokenize='trigram'
	)`,
	`CREATE TABLE IF NOT EXISTS ratings_snapshots (
		id INTEGER PRIMARY KEY,
		captured_at TEXT NOT NULL,
		storefront TEXT NOT NULL,
		app_id TEXT NOT NULL,
		average_rating REAL,
		rating_count INTEGER,
		histogram TEXT,
		version TEXT,
		source TEXT,
		data_as_of TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ratings_snapshots_app ON ratings_snapshots(app_id, storefront, captured_at)`,
	`CREATE TABLE IF NOT EXISTS version_entries (
		app_id TEXT NOT NULL,
		version TEXT NOT NULL,
		released_at TEXT,
		notes TEXT,
		first_seen_at TEXT,
		PRIMARY KEY (app_id, version)
	)`,
	`CREATE TABLE IF NOT EXISTS privacy_snapshots (
		id INTEGER PRIMARY KEY,
		captured_at TEXT NOT NULL,
		storefront TEXT NOT NULL,
		app_id TEXT NOT NULL,
		content_hash TEXT,
		privacy_json TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_privacy_snapshots_app ON privacy_snapshots(app_id, storefront, captured_at)`,
	`CREATE TABLE IF NOT EXISTS iap_prices (
		id INTEGER PRIMARY KEY,
		captured_at TEXT NOT NULL,
		storefront TEXT NOT NULL,
		app_id TEXT NOT NULL,
		position INTEGER,
		name TEXT,
		price TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_iap_prices_app ON iap_prices(app_id, storefront, captured_at)`,
	`CREATE TABLE IF NOT EXISTS app_events (
		app_id TEXT NOT NULL,
		event_id TEXT NOT NULL,
		storefront TEXT NOT NULL,
		title TEXT,
		kind TEXT,
		start_date TEXT,
		end_date TEXT,
		badge TEXT,
		first_seen_at TEXT,
		last_seen_at TEXT,
		PRIMARY KEY (app_id, event_id, storefront)
	)`,
	`CREATE TABLE IF NOT EXISTS featured_snapshots (
		id INTEGER PRIMARY KEY,
		captured_at TEXT NOT NULL,
		storefront TEXT NOT NULL,
		platform TEXT,
		story_id TEXT,
		story_title TEXT,
		app_id TEXT,
		position INTEGER
	)`,
	`CREATE INDEX IF NOT EXISTS idx_featured_snapshots_key ON featured_snapshots(storefront, platform, captured_at)`,
	`CREATE INDEX IF NOT EXISTS idx_featured_snapshots_app ON featured_snapshots(app_id)`,
	`CREATE TABLE IF NOT EXISTS keyword_ranks (
		id INTEGER PRIMARY KEY,
		captured_at TEXT NOT NULL,
		storefront TEXT NOT NULL,
		platform TEXT,
		term TEXT NOT NULL,
		source TEXT,
		app_id TEXT,
		rank INTEGER,
		result_count INTEGER,
		top_ids TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_keyword_ranks_key ON keyword_ranks(term, storefront, platform, source, captured_at)`,
	`CREATE INDEX IF NOT EXISTS idx_keyword_ranks_app ON keyword_ranks(app_id)`,
}

// appStoreColumns are columns added after a table was first shipped;
// migrateExtras adds each one (idempotently) to stores created without it.
var appStoreColumns = []struct{ table, column, decl string }{
	{table: "chart_snapshots", column: "age_band", decl: "TEXT NOT NULL DEFAULT ''"},
	// app_snapshots.source (lookup | page+lookup) names the surface a
	// listing snapshot came from; with captured_at it identifies one
	// observation, so a re-recorded cached response is not stored twice.
	{table: "app_snapshots", column: "source", decl: "TEXT NOT NULL DEFAULT ''"},
	// ratings_snapshots.data_as_of is when Apple produced the counts
	// (appstore.FetchMeta.DataAsOf; '' when unknown): a Lookup copy can be
	// up to 24 h older than its capture time.
	{table: "ratings_snapshots", column: "data_as_of", decl: "TEXT NOT NULL DEFAULT ''"},
}

// appStorePostColumnMigrations run after appStoreColumns. Every statement is
// idempotent.
var appStorePostColumnMigrations = []string{
	// Rows written under the interim composite genre key "36-kids-<band>"
	// (slice 2b, before the age_band column) move to genre 36 + their band.
	`UPDATE chart_snapshots
		SET age_band = substr(genre_id, instr(genre_id, '-kids-') + 6),
			genre_id = substr(genre_id, 1, instr(genre_id, '-kids-') - 1)
		WHERE age_band = '' AND instr(genre_id, '-kids-') > 1`,
	// The series index includes age_band; the pre-age_band index goes.
	`DROP INDEX IF EXISTS idx_chart_snapshots_key`,
	`CREATE INDEX IF NOT EXISTS idx_chart_snapshots_series ON chart_snapshots(storefront, platform, genre_id, age_band, chart, captured_at)`,
	// Snapshots written before app_snapshots.source existed carry their
	// source in fields_json only.
	`UPDATE app_snapshots SET source = COALESCE(json_extract(fields_json, '$.source'), '')
		WHERE source = '' AND COALESCE(fields_json, '') != '' AND json_valid(fields_json)`,
	`CREATE INDEX IF NOT EXISTS idx_app_snapshots_capture ON app_snapshots(app_id, storefront, source, captured_at)`,
	// AppIDForBundle looks bundle ids up case-insensitively.
	`CREATE INDEX IF NOT EXISTS idx_app_snapshots_bundle ON app_snapshots(bundle_id COLLATE NOCASE)`,
}

// Sources recorded on snapshot rows.
const (
	SourceLookup     = "lookup"      // iTunes Lookup API only
	SourcePage       = "page"        // apps.apple.com product page
	SourcePageLookup = "page+lookup" // product page merged with Lookup
	SourceWebChart   = "web"         // apps.apple.com chart page (200 ranks)
	SourceRSS        = "rss"         // legacy iTunes RSS (100 rows)
	SourceSearchAPI  = "itunes"      // iTunes Search API
	SourceWebSearch  = "web"         // opt-in web search page
	SourceAPI        = "api"         // opt-in apps.apple.com/api
)

// --- helpers ---------------------------------------------------------------
//
// Observation time: every snapshot must be stamped with when Apple served the
// data, not when the command ran. The client's results carry it (FetchedAt on
// LookupResult, SearchResponse, ChartFeed, ChartsPage, ReviewsFeed,
// ProductPage, TodayPage ...), and a result served from the local cache keeps
// its original FetchedAt (FromCache is then true), so a re-run inside the
// cache window re-observes nothing new. The writers fall back to the time
// carried by their input and only then to the wall clock, which is correct
// only for data that was just fetched from the network.
//
// Idempotency: an observation is identified by what was observed and when
// (app, storefront, source, captured_at for listing and ratings snapshots;
// storefront, platform, captured_at for a Today capture; term, storefront,
// platform, source, target, captured_at for a keyword capture; the chart key
// and second for a chart). Recording the same observation again (a response
// served from the cache keeps its FetchedAt) writes nothing, and change-point
// tables (privacy, IAP) and first/last-seen rows (events, reviews) never move
// backwards when an older observation is applied after a newer one.

func tsOrNow(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return appstore.FormatTime(t)
}

// firstTime returns the first non-zero time.
func firstTime(ts ...time.Time) time.Time {
	for _, t := range ts {
		if !t.IsZero() {
			return t
		}
	}
	return time.Time{}
}

func parseTS(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

func hashJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:12])
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// withWriteTx runs fn in one write transaction under the store write lock.
// fn must not call other accessors that open their own transaction.
func withWriteTx(ctx context.Context, st *Store, fn func(tx *sql.Tx) error) error {
	if st == nil {
		return errors.New("store is not open")
	}
	st.lockForWrite()
	defer st.unlockAfterWrite()
	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// mergeSet merges comma-separated set values, keeping them sorted.
func mergeSet(existing string, add ...string) string {
	set := map[string]bool{}
	for _, p := range strings.Split(existing, ",") {
		if p = strings.TrimSpace(p); p != "" {
			set[p] = true
		}
	}
	for _, p := range add {
		if p = strings.TrimSpace(p); p != "" {
			set[p] = true
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

// --- capture log -----------------------------------------------------------

// CaptureRow is one stored capture-log row.
type CaptureRow struct {
	ID            int64     `json:"id"`
	CapturedAt    time.Time `json:"capturedAt"`
	URL           string    `json:"url"`
	Host          string    `json:"host"`
	Status        int       `json:"status"`
	Outcome       string    `json:"outcome"`
	EntryCount    int       `json:"entryCount"`
	CacheControl  string    `json:"cacheControl,omitempty"`
	XCache        string    `json:"xCache,omitempty"`
	Oldest        string    `json:"oldest,omitempty"`
	Newest        string    `json:"newest,omitempty"`
	ParserVersion string    `json:"parserVersion,omitempty"`
	Error         string    `json:"error,omitempty"`
}

// RecordCapture appends one capture-log row.
func RecordCapture(ctx context.Context, st *Store, c appstore.Capture) error {
	return withWriteTx(ctx, st, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO captures
			(captured_at, url, host, status, outcome, entry_count, cache_control, x_cache, oldest, newest, parser_version, error)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
			tsOrNow(c.CapturedAt), c.URL, c.Host, c.Status, c.Outcome, c.EntryCount,
			c.CacheControl, c.XCache, c.Oldest, c.Newest, c.ParserVersion, c.Error)
		return err
	})
}

type captureSink struct{ st *Store }

func (s captureSink) RecordCapture(ctx context.Context, c appstore.Capture) error {
	return RecordCapture(ctx, s.st, c)
}

// NewCaptureSink adapts a store to the appstore client's capture log.
func NewCaptureSink(st *Store) appstore.CaptureSink {
	if st == nil {
		return nil
	}
	return captureSink{st: st}
}

// CaptureFilter narrows Captures. Zero values match everything.
type CaptureFilter struct {
	URLContains string
	Host        string
	Outcome     string
	Since       time.Time
	Limit       int // keep the newest N (returned oldest first)
}

// Captures returns capture-log rows, oldest first.
func Captures(ctx context.Context, st *Store, f CaptureFilter) ([]CaptureRow, error) {
	q := `SELECT id, captured_at, url, COALESCE(host,''), COALESCE(status,0), COALESCE(outcome,''),
		COALESCE(entry_count,0), COALESCE(cache_control,''), COALESCE(x_cache,''), COALESCE(oldest,''),
		COALESCE(newest,''), COALESCE(parser_version,''), COALESCE(error,'') FROM captures WHERE 1=1`
	args := []any{}
	if f.URLContains != "" {
		q += ` AND instr(url, ?) > 0`
		args = append(args, f.URLContains)
	}
	if f.Host != "" {
		q += ` AND host = ?`
		args = append(args, f.Host)
	}
	if f.Outcome != "" {
		q += ` AND outcome = ?`
		args = append(args, f.Outcome)
	}
	if !f.Since.IsZero() {
		q += ` AND captured_at >= ?`
		args = append(args, appstore.FormatTime(f.Since))
	}
	q += ` ORDER BY id DESC`
	if f.Limit > 0 {
		q += ` LIMIT ?`
		args = append(args, f.Limit)
	}
	rows, err := st.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query captures: %w", err)
	}
	out := make([]CaptureRow, 0)
	for rows.Next() {
		var r CaptureRow
		var at string
		if err := rows.Scan(&r.ID, &at, &r.URL, &r.Host, &r.Status, &r.Outcome, &r.EntryCount,
			&r.CacheControl, &r.XCache, &r.Oldest, &r.Newest, &r.ParserVersion, &r.Error); err != nil {
			_ = rows.Close()
			return nil, err
		}
		r.CapturedAt = parseTS(at)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// --- app snapshots ---------------------------------------------------------

// AppSnapshotFields is the fields_json payload of an app snapshot: the
// listing fields the watch/changes commands diff. Source says which surfaces
// contributed ("lookup" or "page+lookup"); compare snapshots of the same
// source, because a lookup-only snapshot has no subtitle, privacy, IAP or
// event data.
type AppSnapshotFields struct {
	Source            string             `json:"source"`
	AppID             string             `json:"appId"`
	BundleID          string             `json:"bundleId,omitempty"`
	Title             string             `json:"title"`
	Subtitle          string             `json:"subtitle,omitempty"`
	Version           string             `json:"version,omitempty"`
	VersionReleasedAt string             `json:"versionReleasedAt,omitempty"`
	ReleaseNotes      string             `json:"releaseNotes,omitempty"`
	Price             float64            `json:"price"`
	Currency          string             `json:"currency,omitempty"`
	FormattedPrice    string             `json:"formattedPrice,omitempty"`
	AverageRating     float64            `json:"averageRating"`
	RatingCount       int64              `json:"ratingCount"`
	EditorsChoice     bool               `json:"editorsChoice"`
	HasIAP            bool               `json:"hasInAppPurchases"`
	SellerName        string             `json:"sellerName,omitempty"`
	DeveloperID       string             `json:"developerId,omitempty"`
	DeveloperName     string             `json:"developerName,omitempty"`
	PrimaryGenreID    string             `json:"primaryGenreId,omitempty"`
	PrimaryGenre      string             `json:"primaryGenre,omitempty"`
	ContentRating     string             `json:"contentRating,omitempty"`
	MinimumOSVersion  string             `json:"minimumOsVersion,omitempty"`
	FileSizeBytes     string             `json:"fileSizeBytes,omitempty"`
	Languages         []string           `json:"languages,omitempty"`
	DescriptionHash   string             `json:"descriptionHash,omitempty"`
	ScreenshotCount   int                `json:"screenshotCount"`
	InAppPurchases    []appstore.IAPItem `json:"inAppPurchases,omitempty"`
	PrivacyTypes      []string           `json:"privacyTypes,omitempty"`
	EventIDs          []string           `json:"eventIds,omitempty"`
	ChartPosition     string             `json:"chartPosition,omitempty"`
	CanonicalURL      string             `json:"url,omitempty"`
	// LookupFetchedAt is when Apple served the Lookup fields of a
	// page+lookup snapshot (the snapshot itself is stamped with the page's
	// fetch time; the two can differ by up to the cache window).
	LookupFetchedAt string `json:"lookupFetchedAt,omitempty"`
	// LookupDataAsOf is when Apple produced the Lookup fields (their fetch
	// time minus the Lookup CDN copy's age, up to 24 h); empty when unknown.
	LookupDataAsOf string `json:"lookupDataAsOf,omitempty"`
	// VersionSource is "page" when a page+lookup snapshot's version, release
	// date and notes come from the product page's newest version entry,
	// because the page's data was newer than an older Lookup copy naming
	// another version (appstore.CurrentVersion); LookupVersion then keeps the
	// Lookup copy's version.
	VersionSource string `json:"versionSource,omitempty"`
	LookupVersion string `json:"lookupVersion,omitempty"`
}

// AppSnapshot is one stored listing snapshot.
type AppSnapshot struct {
	ID              int64             `json:"id"`
	CapturedAt      time.Time         `json:"capturedAt"`
	Storefront      string            `json:"storefront"`
	AppID           string            `json:"appId"`
	BundleID        string            `json:"bundleId,omitempty"`
	ContentHash     string            `json:"contentHash"`
	Title           string            `json:"title"`
	Subtitle        string            `json:"subtitle,omitempty"`
	Version         string            `json:"version,omitempty"`
	Price           float64           `json:"price"`
	Currency        string            `json:"currency,omitempty"`
	FormattedPrice  string            `json:"formattedPrice,omitempty"`
	AverageRating   float64           `json:"averageRating"`
	RatingCount     int64             `json:"ratingCount"`
	EditorsChoice   bool              `json:"editorsChoice"`
	HasIAP          bool              `json:"hasInAppPurchases"`
	ScreenshotsHash string            `json:"screenshotsHash,omitempty"`
	PrivacyHash     string            `json:"privacyHash,omitempty"`
	IAPHash         string            `json:"iapHash,omitempty"`
	EventsHash      string            `json:"eventsHash,omitempty"`
	ChartPosition   string            `json:"chartPosition,omitempty"`
	Source          string            `json:"source"`
	Fields          AppSnapshotFields `json:"fields"`
}

// descriptionHash is the stored stand-in for a listing description: the first
// 8 bytes of its SHA-256, hex; empty for an empty description.
func descriptionHash(desc string) string {
	if desc == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(desc))
	return hex.EncodeToString(sum[:8])
}

func lookupFields(lk appstore.LookupResult) AppSnapshotFields {
	f := AppSnapshotFields{
		Source:            SourceLookup,
		AppID:             lk.AppID(),
		BundleID:          lk.BundleID,
		Title:             lk.TrackName,
		Version:           lk.Version,
		VersionReleasedAt: lk.CurrentVersionReleaseDate,
		ReleaseNotes:      lk.ReleaseNotes,
		Price:             lk.Price,
		Currency:          lk.Currency,
		FormattedPrice:    lk.FormattedPrice,
		AverageRating:     lk.AverageUserRating,
		RatingCount:       lk.UserRatingCount,
		SellerName:        lk.SellerName,
		DeveloperName:     lk.ArtistName,
		ContentRating:     lk.TrackContentRating,
		MinimumOSVersion:  lk.MinimumOSVersion,
		FileSizeBytes:     lk.FileSizeBytes,
		Languages:         lk.LanguageCodesISO2A,
		ScreenshotCount:   len(lk.ScreenshotURLs) + len(lk.IPadScreenshotURLs) + len(lk.AppleTVScreenshotURLs),
		PrimaryGenre:      lk.PrimaryGenreName,
		CanonicalURL:      lk.CanonicalURL(),
		DescriptionHash:   descriptionHash(lk.Description),
	}
	if lk.ArtistID != 0 {
		f.DeveloperID = fmt.Sprint(lk.ArtistID)
	}
	if lk.PrimaryGenreID != 0 {
		f.PrimaryGenreID = fmt.Sprint(lk.PrimaryGenreID)
	}
	if !lk.DataAsOf.IsZero() {
		f.LookupDataAsOf = appstore.FormatTime(lk.DataAsOf)
	}
	return f
}

type snapshotHashes struct {
	screenshots, privacy, iap, events string
}

// contentHash covers the listing fields a change feed cares about and leaves
// out volatile ones (rating average/count, chart position).
func contentHash(f AppSnapshotFields, h snapshotHashes) string {
	return hashJSON([]any{
		f.Source, f.Title, f.Subtitle, f.Version, f.ReleaseNotes, f.Price, f.Currency,
		f.DescriptionHash, f.EditorsChoice, f.SellerName, f.PrimaryGenreID, f.ContentRating,
		f.MinimumOSVersion, h.screenshots, h.privacy, h.iap, h.events,
	})
}

// insertAppSnapshot appends one listing snapshot unless this observation
// (app, storefront, source, captured_at) is already stored; it reports
// whether a row was written.
func insertAppSnapshot(ctx context.Context, tx *sql.Tx, at, storefront string, f AppSnapshotFields, h snapshotHashes) (bool, error) {
	var stored int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM app_snapshots
		WHERE app_id = ? AND storefront = ? AND source = ? AND captured_at = ?`, f.AppID, storefront, f.Source, at).Scan(&stored); err != nil {
		return false, fmt.Errorf("read app snapshot: %w", err)
	}
	if stored > 0 {
		return false, nil
	}
	fieldsJSON, err := json.Marshal(f)
	if err != nil {
		return false, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO app_snapshots
		(captured_at, storefront, app_id, bundle_id, content_hash, title, subtitle, version, price, currency,
		 formatted_price, average_rating, rating_count, editors_choice, has_iap, screenshots_hash, privacy_hash,
		 iap_hash, events_hash, chart_position, fields_json, source)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		at, storefront, f.AppID, f.BundleID, contentHash(f, h), f.Title, f.Subtitle, f.Version, f.Price, f.Currency,
		f.FormattedPrice, f.AverageRating, f.RatingCount, boolInt(f.EditorsChoice), boolInt(f.HasIAP),
		h.screenshots, h.privacy, h.iap, h.events, f.ChartPosition, string(fieldsJSON), f.Source)
	return err == nil, err
}

// RecordLookupSnapshot appends a lookup-only app snapshot (source "lookup").
// at is the observation time; zero means lk.FetchedAt (when Apple served the
// result, kept by the cache), and only then now. Recording the same
// observation again (same app, storefront and time) writes nothing.
func RecordLookupSnapshot(ctx context.Context, st *Store, storefront string, lk appstore.LookupResult, at time.Time) error {
	if lk.TrackID == 0 {
		return fmt.Errorf("lookup result has no trackId")
	}
	f := lookupFields(lk)
	h := snapshotHashes{screenshots: hashJSON([]any{lk.ScreenshotURLs, lk.IPadScreenshotURLs, lk.AppleTVScreenshotURLs})}
	stamp := tsOrNow(firstTime(at, lk.FetchedAt))
	return withWriteTx(ctx, st, func(tx *sql.Tx) error {
		_, err := insertAppSnapshot(ctx, tx, stamp, storefront, f, h)
		return err
	})
}

// RecordProductPage writes everything one product page (plus its Lookup row)
// teaches the store, in one transaction: an app snapshot (only when lk has a
// trackId), version entries (insert-or-ignore), a privacy snapshot (only
// when its hash changed), the IAP price list (only when it changed), in-app
// events (upsert) and a ratings snapshot from the page histogram (source
// "page"). The capture time is page.FetchedAt, else lk.FetchedAt, else now;
// the snapshot's fields record lk.FetchedAt as lookupFetchedAt. The
// snapshot's version is appstore.CurrentVersion (the page's newest entry when
// the page's data is newer than a Lookup copy naming another version), and
// the page ratings row pairs the page's totals with the page's own newest
// version. Recording
// the same page observation again (a cached page keeps its FetchedAt) adds
// no snapshot, ratings, privacy, IAP or event row.
func RecordProductPage(ctx context.Context, st *Store, storefront string, page appstore.ProductPage, lk appstore.LookupResult) error {
	at := tsOrNow(firstTime(page.FetchedAt, lk.FetchedAt))
	appID := page.AdamID
	if appID == "" {
		appID = lk.AppID()
	}
	if appID == "" {
		return fmt.Errorf("product page has no app id")
	}
	shots := make([]string, 0)
	for _, m := range page.Media {
		shots = append(shots, m.Screenshots...)
	}
	h := snapshotHashes{
		screenshots: hashJSON(shots),
		privacy:     privacyHash(page.Privacy),
		iap:         hashJSON(page.InAppPurchases),
		events:      eventsHash(page.Events),
	}
	return withWriteTx(ctx, st, func(tx *sql.Tx) error {
		if lk.TrackID != 0 {
			f := lookupFields(lk)
			f.Source = SourcePageLookup
			f.LookupFetchedAt = appstore.FormatTime(lk.FetchedAt)
			// A Lookup CDN copy can be hours older than the page fetched
			// with it: the page's newer version wins (not the stale copy's).
			if cv := appstore.CurrentVersion(lk, &page); cv.FromPage {
				f.Version, f.VersionReleasedAt, f.ReleaseNotes = cv.Version, cv.ReleasedAt, cv.Notes
				f.VersionSource, f.LookupVersion = "page", cv.LookupVersion
			}
			// The full listing shows the page's description, so the snapshot
			// hashes that text: a Lookup CDN copy can keep serving the old
			// description for hours after an edit the page already shows.
			if page.Description != "" {
				f.DescriptionHash = descriptionHash(page.Description)
			}
			f.Subtitle = page.Subtitle
			f.EditorsChoice = page.EditorsChoice
			f.HasIAP = page.HasInAppPurchases || len(page.InAppPurchases) > 0
			f.InAppPurchases = page.InAppPurchases
			f.ScreenshotCount = len(shots)
			f.CanonicalURL = firstNonEmptyStr(page.CanonicalURL, f.CanonicalURL)
			if page.DeveloperID != "" {
				f.DeveloperID = page.DeveloperID
			}
			if page.ContentRating != "" {
				f.ContentRating = page.ContentRating
			}
			for _, t := range page.Privacy.Types {
				f.PrivacyTypes = append(f.PrivacyTypes, t.Identifier)
			}
			for _, e := range page.Events {
				f.EventIDs = append(f.EventIDs, e.EventID)
			}
			if page.ChartPosition != nil {
				f.ChartPosition = page.ChartPosition.String()
			}
			if _, err := insertAppSnapshot(ctx, tx, at, storefront, f, h); err != nil {
				return fmt.Errorf("app snapshot: %w", err)
			}
		}
		if err := recordVersionsTx(ctx, tx, appID, page.Versions, at); err != nil {
			return err
		}
		if _, err := recordPrivacyTx(ctx, tx, storefront, appID, page.Privacy, at); err != nil {
			return err
		}
		if _, err := recordIAPsTx(ctx, tx, storefront, appID, page.InAppPurchases, page.IAPNoneShown, at); err != nil {
			return err
		}
		if err := recordEventsTx(ctx, tx, storefront, appID, page.Events, at); err != nil {
			return err
		}
		return insertPageRatingsTx(ctx, tx, storefront, appID, page, lk, at)
	})
}

// RecordPageRatings writes only the ratings snapshot of a product page
// (source "page"): the counts do not depend on the page language, so a
// capture in a named language (--lang) keeps them while its translated
// listing text stays out of the history. Idempotent like RecordProductPage.
func RecordPageRatings(ctx context.Context, st *Store, storefront string, page appstore.ProductPage, lk appstore.LookupResult) error {
	appID := firstNonEmptyStr(page.AdamID, lk.AppID())
	if appID == "" {
		return fmt.Errorf("product page has no app id")
	}
	at := tsOrNow(firstTime(page.FetchedAt, lk.FetchedAt))
	return withWriteTx(ctx, st, func(tx *sql.Tx) error {
		return insertPageRatingsTx(ctx, tx, storefront, appID, page, lk, at)
	})
}

// insertPageRatingsTx appends the page's ratings snapshot when the page shows
// ratings.
func insertPageRatingsTx(ctx context.Context, tx *sql.Tx, storefront, appID string, page appstore.ProductPage, lk appstore.LookupResult, at string) error {
	if page.TotalRatings <= 0 && len(page.Histogram) == 0 {
		return nil
	}
	// The page's totals pair with the page's own newest version (the same
	// observation), never with a Lookup copy that may be hours older.
	version := lk.Version
	if len(page.Versions) > 0 && page.Versions[0].Version != "" {
		version = page.Versions[0].Version
	}
	_, err := insertRatingsTx(ctx, tx, RatingsSnapshot{
		Storefront: storefront, AppID: appID, AverageRating: page.RatingAverage,
		RatingCount: page.TotalRatings, Histogram: page.Histogram, Version: version, Source: SourcePage,
		DataAsOf: page.DataAsOf,
	}, at)
	return err
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func privacyHash(p appstore.Privacy) string {
	if len(p.Types) == 0 {
		return ""
	}
	return hashJSON(p.Types)
}

func eventsHash(events []appstore.AppEvent) string {
	if len(events) == 0 {
		return ""
	}
	keys := make([]string, 0, len(events))
	for _, e := range events {
		keys = append(keys, e.EventID+"|"+e.Title+"|"+e.StartDate+"|"+e.EndDate)
	}
	sort.Strings(keys)
	return hashJSON(keys)
}

// AppSnapshots returns an app's snapshots oldest first. storefront "" means
// every storefront; since zero means all time.
func AppSnapshots(ctx context.Context, st *Store, appID, storefront string, since time.Time) ([]AppSnapshot, error) {
	q := `SELECT id, captured_at, storefront, app_id, COALESCE(bundle_id,''), COALESCE(content_hash,''),
		COALESCE(title,''), COALESCE(subtitle,''), COALESCE(version,''), COALESCE(price,0), COALESCE(currency,''),
		COALESCE(formatted_price,''), COALESCE(average_rating,0), COALESCE(rating_count,0),
		COALESCE(editors_choice,0), COALESCE(has_iap,0), COALESCE(screenshots_hash,''), COALESCE(privacy_hash,''),
		COALESCE(iap_hash,''), COALESCE(events_hash,''), COALESCE(chart_position,''), COALESCE(fields_json,'')
		FROM app_snapshots WHERE app_id = ?`
	args := []any{appID}
	if storefront != "" {
		q += ` AND storefront = ?`
		args = append(args, storefront)
	}
	if !since.IsZero() {
		q += ` AND captured_at >= ?`
		args = append(args, appstore.FormatTime(since))
	}
	q += ` ORDER BY captured_at, id`
	return scanAppSnapshots(ctx, st, q, args...)
}

func scanAppSnapshots(ctx context.Context, st *Store, q string, args ...any) ([]AppSnapshot, error) {
	rows, err := st.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query app snapshots: %w", err)
	}
	out := make([]AppSnapshot, 0)
	for rows.Next() {
		var s AppSnapshot
		var at, fields string
		var ec, iap int
		if err := rows.Scan(&s.ID, &at, &s.Storefront, &s.AppID, &s.BundleID, &s.ContentHash, &s.Title, &s.Subtitle,
			&s.Version, &s.Price, &s.Currency, &s.FormattedPrice, &s.AverageRating, &s.RatingCount, &ec, &iap,
			&s.ScreenshotsHash, &s.PrivacyHash, &s.IAPHash, &s.EventsHash, &s.ChartPosition, &fields); err != nil {
			_ = rows.Close()
			return nil, err
		}
		s.CapturedAt = parseTS(at)
		s.EditorsChoice = ec != 0
		s.HasIAP = iap != 0
		if fields != "" {
			_ = json.Unmarshal([]byte(fields), &s.Fields)
		}
		s.Source = s.Fields.Source
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return out, nil
}

// TrackedApp summarizes the snapshots stored for one app in one storefront.
type TrackedApp struct {
	AppID      string    `json:"appId"`
	Storefront string    `json:"storefront"`
	Title      string    `json:"title"`
	Snapshots  int       `json:"snapshots"`
	First      time.Time `json:"firstCapturedAt"`
	Last       time.Time `json:"lastCapturedAt"`
}

// TrackedApps lists every (app, storefront) with app snapshots captured at or
// after since (zero = all time), ordered by app id then storefront.
func TrackedApps(ctx context.Context, st *Store, since time.Time) ([]TrackedApp, error) {
	q := `SELECT app_id, storefront, COUNT(*), MIN(captured_at), MAX(captured_at),
		(SELECT COALESCE(s2.title,'') FROM app_snapshots s2 WHERE s2.app_id = s.app_id AND s2.storefront = s.storefront
		 ORDER BY s2.captured_at DESC, s2.id DESC LIMIT 1)
		FROM app_snapshots s WHERE captured_at >= ? GROUP BY app_id, storefront ORDER BY app_id, storefront`
	sinceStr := ""
	if !since.IsZero() {
		sinceStr = appstore.FormatTime(since)
	}
	rows, err := st.db.QueryContext(ctx, q, sinceStr)
	if err != nil {
		return nil, fmt.Errorf("query tracked apps: %w", err)
	}
	out := make([]TrackedApp, 0)
	for rows.Next() {
		var t TrackedApp
		var first, last string
		if err := rows.Scan(&t.AppID, &t.Storefront, &t.Snapshots, &first, &last, &t.Title); err != nil {
			_ = rows.Close()
			return nil, err
		}
		t.First, t.Last = parseTS(first), parseTS(last)
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// NewestAppObservation returns the newest time the store observed any of
// the apps (every app when appIDs is empty) in any of the storefronts (all
// when empty): listing and ratings snapshots, chart rows, reviews (last
// seen), IAP lists, privacy labels and in-app events. Zero means the store
// holds nothing for them. Local commands compare it with --max-age.
func NewestAppObservation(ctx context.Context, st *Store, appIDs, storefronts []string) (time.Time, error) {
	type source struct{ table, column string }
	sources := []source{
		{"app_snapshots", "captured_at"}, {"ratings_snapshots", "captured_at"}, {"chart_snapshots", "captured_at"},
		{"reviews", "last_seen_at"}, {"iap_prices", "captured_at"}, {"privacy_snapshots", "captured_at"}, {"app_events", "last_seen_at"},
	}
	in := func(col string, vals []string) (string, []any) {
		if len(vals) == 0 {
			return "", nil
		}
		args := make([]any, 0, len(vals))
		for _, v := range vals {
			args = append(args, v)
		}
		return ` AND ` + col + ` IN (` + strings.TrimSuffix(strings.Repeat("?,", len(vals)), ",") + `)`, args
	}
	appWhere, appArgs := in("app_id", appIDs)
	sfWhere, sfArgs := in("storefront", storefronts)
	newest := ""
	for _, s := range sources {
		var v sql.NullString
		args := append(append([]any{}, appArgs...), sfArgs...)
		if err := st.db.QueryRowContext(ctx, `SELECT MAX(`+s.column+`) FROM `+s.table+` WHERE 1=1`+appWhere+sfWhere, args...).Scan(&v); err != nil {
			return time.Time{}, fmt.Errorf("newest %s observation: %w", s.table, err)
		}
		if v.Valid && v.String > newest {
			newest = v.String
		}
	}
	return parseTS(newest), nil
}

// AppIDForBundle returns the numeric App Store id the store holds for a
// bundle id (from listing snapshots; the most recent one wins, compared
// case-insensitively), or "" when the store has never seen it. Local history
// commands use it to accept bundle ids without a live lookup.
func AppIDForBundle(ctx context.Context, st *Store, bundleID string) (string, error) {
	bundleID = strings.TrimSpace(bundleID)
	if bundleID == "" {
		return "", nil
	}
	var id string
	err := st.db.QueryRowContext(ctx, `SELECT app_id FROM app_snapshots WHERE bundle_id = ? COLLATE NOCASE
		ORDER BY captured_at DESC, id DESC LIMIT 1`, bundleID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("look up bundle id: %w", err)
	}
	return id, nil
}

// AppsWithHistory lists every app id with any stored listing history (a
// listing snapshot, an IAP price list, a privacy label, an in-app event or a
// version entry) observed at or after since (zero = all time), sorted. It
// covers apps captured only through commands that write no listing
// snapshot, which TrackedApps (listing snapshots only) misses.
func AppsWithHistory(ctx context.Context, st *Store, since time.Time) ([]string, error) {
	s := ""
	if !since.IsZero() {
		s = appstore.FormatTime(since)
	}
	rows, err := st.db.QueryContext(ctx, `
		SELECT app_id FROM app_snapshots WHERE captured_at >= ?
		UNION SELECT app_id FROM iap_prices WHERE captured_at >= ?
		UNION SELECT app_id FROM privacy_snapshots WHERE captured_at >= ?
		UNION SELECT app_id FROM app_events WHERE COALESCE(last_seen_at, '') >= ?
		UNION SELECT app_id FROM version_entries WHERE COALESCE(first_seen_at, '') >= ?
		ORDER BY app_id`, s, s, s, s, s)
	if err != nil {
		return nil, fmt.Errorf("query apps with history: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if id != "" {
			out = append(out, id)
		}
	}
	return out, rows.Err()
}

// SearchApps finds the latest snapshot of each (app, storefront) whose title
// or subtitle contains every query word, case-insensitively in any script
// (matched in Go: SQLite's lower() folds ASCII only). A lookup-only snapshot
// carries no subtitle, so when the latest snapshot is lookup-only the subtitle
// searched (and returned) is the latest page capture's, even when that page
// showed none (a removed subtitle stays removed). storefront "" searches all
// storefronts.
func SearchApps(ctx context.Context, st *Store, query, storefront string, limit int) ([]AppSnapshot, error) {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return make([]AppSnapshot, 0), nil
	}
	q := `SELECT id, captured_at, storefront, app_id, COALESCE(bundle_id,''), COALESCE(content_hash,''),
		COALESCE(title,''), COALESCE(subtitle,''), COALESCE(version,''), COALESCE(price,0), COALESCE(currency,''),
		COALESCE(formatted_price,''), COALESCE(average_rating,0), COALESCE(rating_count,0),
		COALESCE(editors_choice,0), COALESCE(has_iap,0), COALESCE(screenshots_hash,''), COALESCE(privacy_hash,''),
		COALESCE(iap_hash,''), COALESCE(events_hash,''), COALESCE(chart_position,''), COALESCE(fields_json,'')
		FROM app_snapshots s WHERE s.id = (SELECT s2.id FROM app_snapshots s2 WHERE s2.app_id = s.app_id
		AND s2.storefront = s.storefront ORDER BY s2.captured_at DESC, s2.id DESC LIMIT 1)`
	args := []any{}
	subQ := `SELECT id, app_id, storefront, COALESCE(subtitle,'') FROM app_snapshots WHERE source = ?`
	subArgs := []any{SourcePageLookup}
	if storefront != "" {
		q += ` AND s.storefront = ?`
		args = append(args, storefront)
		subQ += ` AND storefront = ?`
		subArgs = append(subArgs, storefront)
	}
	q += ` ORDER BY s.rating_count DESC, s.app_id`
	latest, err := scanAppSnapshots(ctx, st, q, args...)
	if err != nil {
		return nil, err
	}
	rows, err := st.db.QueryContext(ctx, subQ+` ORDER BY captured_at DESC, id DESC`, subArgs...)
	if err != nil {
		return nil, fmt.Errorf("query subtitles: %w", err)
	}
	defer rows.Close()
	type pageSubtitle struct {
		id       int64
		subtitle string
	}
	pages := map[string]pageSubtitle{} // latest page capture per app|storefront
	for rows.Next() {
		var p pageSubtitle
		var app, sf string
		if err := rows.Scan(&p.id, &app, &sf, &p.subtitle); err != nil {
			return nil, err
		}
		if _, ok := pages[app+"|"+sf]; !ok {
			pages[app+"|"+sf] = p
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]AppSnapshot, 0)
	for _, s := range latest {
		// A latest page capture keeps its own subtitle (empty when removed);
		// a lookup-only latest snapshot borrows the latest page capture's.
		if p, ok := pages[s.AppID+"|"+s.Storefront]; ok && p.id != s.ID {
			s.Subtitle = p.subtitle
		}
		text := strings.ToLower(s.Title + "\n" + s.Subtitle)
		match := true
		for _, w := range words {
			if !strings.Contains(text, w) {
				match = false
				break
			}
		}
		if match {
			out = append(out, s)
			if limit > 0 && len(out) == limit {
				break
			}
		}
	}
	return out, nil
}

// --- versions, privacy, IAP, events -----------------------------------------

// StoredVersion is one version-history entry.
type StoredVersion struct {
	AppID       string    `json:"appId"`
	Version     string    `json:"version"`
	ReleasedAt  string    `json:"releasedAt"`
	Notes       string    `json:"notes,omitempty"`
	FirstSeenAt time.Time `json:"firstSeenAt"`
}

func recordVersionsTx(ctx context.Context, tx *sql.Tx, appID string, versions []appstore.VersionEntry, at string) error {
	for _, v := range versions {
		if v.Version == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO version_entries (app_id, version, released_at, notes, first_seen_at)
			VALUES (?,?,?,?,?) ON CONFLICT(app_id, version) DO NOTHING`, appID, v.Version, v.ReleasedAt, v.Notes, at); err != nil {
			return fmt.Errorf("version entry: %w", err)
		}
	}
	return nil
}

// VersionEntries returns an app's stored versions, newest release first.
func VersionEntries(ctx context.Context, st *Store, appID string) ([]StoredVersion, error) {
	rows, err := st.db.QueryContext(ctx, `SELECT app_id, version, COALESCE(released_at,''), COALESCE(notes,''),
		COALESCE(first_seen_at,'') FROM version_entries WHERE app_id = ? ORDER BY released_at DESC, version DESC`, appID)
	if err != nil {
		return nil, fmt.Errorf("query versions: %w", err)
	}
	out := make([]StoredVersion, 0)
	for rows.Next() {
		var v StoredVersion
		var seen string
		if err := rows.Scan(&v.AppID, &v.Version, &v.ReleasedAt, &v.Notes, &seen); err != nil {
			_ = rows.Close()
			return nil, err
		}
		v.FirstSeenAt = parseTS(seen)
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// emptyPrivacyHash is the content hash of an explicitly empty label: the
// page showed its App Privacy section with no label in it.
var emptyPrivacyHash = hashJSON([]appstore.PrivacyType{})

// recordPrivacyTx stores a label change point. A label with types is
// compared with the label in force; an empty label is recorded only when the
// page positively showed none (p.NoneShown), so a label that disappears is a
// change while a page the parser could not read records nothing.
func recordPrivacyTx(ctx context.Context, tx *sql.Tx, storefront, appID string, p appstore.Privacy, at string) (bool, error) {
	hash := privacyHash(p)
	if hash == "" {
		if !p.NoneShown {
			return false, nil
		}
		hash = emptyPrivacyHash
	}
	// Compare with the label in force at this observation (the latest stored
	// at or before it), so re-recording an earlier cached page after a newer
	// one adds no duplicate change point.
	var latest string
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(content_hash,'') FROM privacy_snapshots WHERE app_id = ? AND storefront = ?
		AND captured_at <= ? ORDER BY captured_at DESC, id DESC LIMIT 1`, appID, storefront, at).Scan(&latest)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("latest privacy snapshot: %w", err)
	}
	if latest == hash {
		return false, nil
	}
	body, err := json.Marshal(p)
	if err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO privacy_snapshots (captured_at, storefront, app_id, content_hash, privacy_json)
		VALUES (?,?,?,?,?)`, at, storefront, appID, hash, string(body)); err != nil {
		return false, fmt.Errorf("privacy snapshot: %w", err)
	}
	return true, nil
}

// PrivacySnapshot is one stored privacy-label version.
type PrivacySnapshot struct {
	CapturedAt  time.Time        `json:"capturedAt"`
	Storefront  string           `json:"storefront"`
	ContentHash string           `json:"contentHash"`
	Privacy     appstore.Privacy `json:"privacy"`
}

// PrivacyHistory returns stored privacy-label versions, oldest first.
// storefront "" means every storefront.
func PrivacyHistory(ctx context.Context, st *Store, appID, storefront string) ([]PrivacySnapshot, error) {
	q := `SELECT captured_at, storefront, COALESCE(content_hash,''), COALESCE(privacy_json,'') FROM privacy_snapshots WHERE app_id = ?`
	args := []any{appID}
	if storefront != "" {
		q += ` AND storefront = ?`
		args = append(args, storefront)
	}
	q += ` ORDER BY captured_at, id`
	rows, err := st.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query privacy history: %w", err)
	}
	out := make([]PrivacySnapshot, 0)
	for rows.Next() {
		var p PrivacySnapshot
		var at, body string
		if err := rows.Scan(&at, &p.Storefront, &p.ContentHash, &body); err != nil {
			_ = rows.Close()
			return nil, err
		}
		p.CapturedAt = parseTS(at)
		_ = json.Unmarshal([]byte(body), &p.Privacy)
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// latestIAPHashTx hashes the IAP list in force at asOf: the latest list
// stored at or before it.
func latestIAPHashTx(ctx context.Context, tx *sql.Tx, storefront, appID, asOf string) (string, error) {
	var at string
	err := tx.QueryRowContext(ctx, `SELECT captured_at FROM iap_prices WHERE app_id = ? AND storefront = ? AND captured_at <= ?
		ORDER BY captured_at DESC LIMIT 1`, appID, storefront, asOf).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	rows, err := tx.QueryContext(ctx, `SELECT COALESCE(position,0), COALESCE(name,''), COALESCE(price,'') FROM iap_prices
		WHERE app_id = ? AND storefront = ? AND captured_at = ? ORDER BY position, id`, appID, storefront, at)
	if err != nil {
		return "", err
	}
	items := make([]appstore.IAPItem, 0)
	for rows.Next() {
		var it appstore.IAPItem
		if err := rows.Scan(&it.Position, &it.Name, &it.Price); err != nil {
			_ = rows.Close()
			return "", err
		}
		if !isIAPEmptyMarker(it) {
			items = append(items, it)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return "", err
	}
	if err := rows.Close(); err != nil {
		return "", err
	}
	return hashJSON(items), nil
}

// An explicitly empty IAP list (the page positively showed no in-app
// purchases) is stored as one marker row: position 0 with no name or price.
// Real rows are numbered from 1.
func isIAPEmptyMarker(it appstore.IAPItem) bool {
	return it.Position == 0 && it.Name == "" && it.Price == ""
}

// recordIAPsTx stores an IAP list change point. A non-empty list is compared
// with the list in force; an empty list is recorded (as the marker row) only
// when the page positively showed none (noneShown), so a list that
// disappears is a change while a page the parser could not read records
// nothing.
func recordIAPsTx(ctx context.Context, tx *sql.Tx, storefront, appID string, items []appstore.IAPItem, noneShown bool, at string) (bool, error) {
	if len(items) == 0 && !noneShown {
		return false, nil
	}
	if items == nil {
		items = []appstore.IAPItem{}
	}
	latest, err := latestIAPHashTx(ctx, tx, storefront, appID, at)
	if err != nil {
		return false, fmt.Errorf("latest IAP prices: %w", err)
	}
	if latest == hashJSON(items) {
		return false, nil
	}
	if len(items) == 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO iap_prices (captured_at, storefront, app_id, position, name, price)
			VALUES (?,?,?,0,NULL,NULL)`, at, storefront, appID); err != nil {
			return false, fmt.Errorf("empty IAP list: %w", err)
		}
		return true, nil
	}
	for _, it := range items {
		if _, err := tx.ExecContext(ctx, `INSERT INTO iap_prices (captured_at, storefront, app_id, position, name, price)
			VALUES (?,?,?,?,?,?)`, at, storefront, appID, it.Position, it.Name, it.Price); err != nil {
			return false, fmt.Errorf("IAP price: %w", err)
		}
	}
	return true, nil
}

// IAPSnapshot is one stored in-app purchase price list.
type IAPSnapshot struct {
	CapturedAt time.Time          `json:"capturedAt"`
	Storefront string             `json:"storefront"`
	Items      []appstore.IAPItem `json:"items"`
}

// IAPHistory returns stored IAP price lists (one per change), oldest first.
// A list the page positively showed as empty is a snapshot with no items.
// storefront "" means every storefront.
func IAPHistory(ctx context.Context, st *Store, appID, storefront string) ([]IAPSnapshot, error) {
	q := `SELECT captured_at, storefront, COALESCE(position,0), COALESCE(name,''), COALESCE(price,'') FROM iap_prices WHERE app_id = ?`
	args := []any{appID}
	if storefront != "" {
		q += ` AND storefront = ?`
		args = append(args, storefront)
	}
	q += ` ORDER BY captured_at, storefront, position, id`
	rows, err := st.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query IAP history: %w", err)
	}
	out := make([]IAPSnapshot, 0)
	for rows.Next() {
		var at, sf string
		var it appstore.IAPItem
		if err := rows.Scan(&at, &sf, &it.Position, &it.Name, &it.Price); err != nil {
			_ = rows.Close()
			return nil, err
		}
		t := parseTS(at)
		if n := len(out); n == 0 || !out[n-1].CapturedAt.Equal(t) || out[n-1].Storefront != sf {
			out = append(out, IAPSnapshot{CapturedAt: t, Storefront: sf, Items: make([]appstore.IAPItem, 0)})
		}
		if !isIAPEmptyMarker(it) {
			out[len(out)-1].Items = append(out[len(out)-1].Items, it)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

func recordEventsTx(ctx context.Context, tx *sql.Tx, storefront, appID string, events []appstore.AppEvent, at string) error {
	for _, e := range events {
		if e.EventID == "" {
			continue
		}
		// first_seen_at only moves earlier and last_seen_at only later; the
		// event's fields come from the newest sighting, so applying an older
		// (cached) observation after a newer one changes nothing.
		if _, err := tx.ExecContext(ctx, `INSERT INTO app_events
			(app_id, event_id, storefront, title, kind, start_date, end_date, badge, first_seen_at, last_seen_at)
			VALUES (?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(app_id, event_id, storefront) DO UPDATE SET
				title = CASE WHEN excluded.last_seen_at >= COALESCE(app_events.last_seen_at, '') THEN excluded.title ELSE app_events.title END,
				kind = CASE WHEN excluded.last_seen_at >= COALESCE(app_events.last_seen_at, '') THEN excluded.kind ELSE app_events.kind END,
				start_date = CASE WHEN excluded.last_seen_at >= COALESCE(app_events.last_seen_at, '') THEN excluded.start_date ELSE app_events.start_date END,
				end_date = CASE WHEN excluded.last_seen_at >= COALESCE(app_events.last_seen_at, '') THEN excluded.end_date ELSE app_events.end_date END,
				badge = CASE WHEN excluded.last_seen_at >= COALESCE(app_events.last_seen_at, '') THEN excluded.badge ELSE app_events.badge END,
				first_seen_at = MIN(COALESCE(NULLIF(app_events.first_seen_at, ''), excluded.first_seen_at), excluded.first_seen_at),
				last_seen_at = MAX(COALESCE(app_events.last_seen_at, ''), excluded.last_seen_at)`,
			appID, e.EventID, storefront, e.Title, e.Kind, e.StartDate, e.EndDate, e.Badge, at, at); err != nil {
			return fmt.Errorf("app event: %w", err)
		}
	}
	return nil
}

// StoredEvent is one stored in-app event.
type StoredEvent struct {
	AppID       string    `json:"appId"`
	EventID     string    `json:"eventId"`
	Storefront  string    `json:"storefront"`
	Title       string    `json:"title"`
	Kind        string    `json:"kind,omitempty"`
	StartDate   string    `json:"startDate,omitempty"`
	EndDate     string    `json:"endDate,omitempty"`
	Badge       string    `json:"badge,omitempty"`
	FirstSeenAt time.Time `json:"firstSeenAt"`
	LastSeenAt  time.Time `json:"lastSeenAt"`
}

// EventsHistory returns stored events ordered by first sighting. storefront
// "" means every storefront.
func EventsHistory(ctx context.Context, st *Store, appID, storefront string) ([]StoredEvent, error) {
	q := `SELECT app_id, event_id, storefront, COALESCE(title,''), COALESCE(kind,''), COALESCE(start_date,''),
		COALESCE(end_date,''), COALESCE(badge,''), COALESCE(first_seen_at,''), COALESCE(last_seen_at,'')
		FROM app_events WHERE app_id = ?`
	args := []any{appID}
	if storefront != "" {
		q += ` AND storefront = ?`
		args = append(args, storefront)
	}
	q += ` ORDER BY first_seen_at, start_date, event_id`
	rows, err := st.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	out := make([]StoredEvent, 0)
	for rows.Next() {
		var e StoredEvent
		var first, last string
		if err := rows.Scan(&e.AppID, &e.EventID, &e.Storefront, &e.Title, &e.Kind, &e.StartDate, &e.EndDate, &e.Badge, &first, &last); err != nil {
			_ = rows.Close()
			return nil, err
		}
		e.FirstSeenAt, e.LastSeenAt = parseTS(first), parseTS(last)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// --- ratings ---------------------------------------------------------------

// RatingsSnapshot is one per-storefront rating capture. Histogram is 5-star
// first. Source is "lookup" (exact userRatingCount from the Lookup API) or
// "page" (the product page's productRatings); keep one source per series,
// the two are cached separately and differ slightly.
type RatingsSnapshot struct {
	CapturedAt    time.Time `json:"capturedAt"`
	Storefront    string    `json:"storefront"`
	AppID         string    `json:"appId"`
	AverageRating float64   `json:"averageRating"`
	RatingCount   int64     `json:"ratingCount"`
	Histogram     []int64   `json:"histogram,omitempty"`
	Version       string    `json:"version,omitempty"`
	Source        string    `json:"source"`
	// DataAsOf is when Apple produced the counts (zero when unknown): a
	// Lookup copy can be hours older than CapturedAt. Time-series readers
	// order and bucket by it when known.
	DataAsOf time.Time `json:"dataAsOf,omitzero"`
}

// ObservedAt is when the counts date from: DataAsOf when known, else
// CapturedAt.
func (r RatingsSnapshot) ObservedAt() time.Time {
	if !r.DataAsOf.IsZero() {
		return r.DataAsOf
	}
	return r.CapturedAt
}

// insertRatingsTx appends one ratings snapshot unless this observation
// (app, storefront, source, captured_at) is already stored; it reports
// whether a row was written.
func insertRatingsTx(ctx context.Context, tx *sql.Tx, r RatingsSnapshot, at string) (bool, error) {
	hist := ""
	if len(r.Histogram) > 0 {
		b, _ := json.Marshal(r.Histogram)
		hist = string(b)
	}
	source := r.Source
	if source == "" {
		source = SourceLookup
	}
	var stored int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM ratings_snapshots
		WHERE app_id = ? AND storefront = ? AND source = ? AND captured_at = ?`, r.AppID, r.Storefront, source, at).Scan(&stored); err != nil {
		return false, fmt.Errorf("read ratings snapshot: %w", err)
	}
	if stored > 0 {
		return false, nil
	}
	dataAsOf := ""
	if !r.DataAsOf.IsZero() {
		dataAsOf = appstore.FormatTime(r.DataAsOf)
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO ratings_snapshots
		(captured_at, storefront, app_id, average_rating, rating_count, histogram, version, source, data_as_of)
		VALUES (?,?,?,?,?,?,?,?,?)`, at, r.Storefront, r.AppID, r.AverageRating, r.RatingCount, hist, r.Version, source, dataAsOf)
	if err != nil {
		return false, fmt.Errorf("ratings snapshot: %w", err)
	}
	return true, nil
}

// RecordRatings appends ratings snapshots in one transaction. CapturedAt is
// the observation time: pass the Lookup result's (or page's) FetchedAt, which
// a cached response keeps. A zero CapturedAt means now, which is right only
// for data fetched from the network in this run. A snapshot whose
// observation (app, storefront, source, time) is already stored is skipped.
func RecordRatings(ctx context.Context, st *Store, snaps ...RatingsSnapshot) error {
	_, err := RecordRatingsN(ctx, st, snaps...)
	return err
}

// RecordRatingsN is RecordRatings that reports how many snapshots it wrote
// (observations already stored are not counted).
func RecordRatingsN(ctx context.Context, st *Store, snaps ...RatingsSnapshot) (int, error) {
	if len(snaps) == 0 {
		return 0, nil
	}
	now := tsOrNow(time.Time{})
	written := 0
	err := withWriteTx(ctx, st, func(tx *sql.Tx) error {
		for _, r := range snaps {
			if r.AppID == "" || r.Storefront == "" {
				return fmt.Errorf("ratings snapshot needs app id and storefront")
			}
			at := now
			if !r.CapturedAt.IsZero() {
				at = appstore.FormatTime(r.CapturedAt)
			}
			ok, err := insertRatingsTx(ctx, tx, r, at)
			if err != nil {
				return err
			}
			if ok {
				written++
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return written, nil
}

// RatingsSeries returns ratings snapshots oldest first. storefront and
// source "" match everything.
func RatingsSeries(ctx context.Context, st *Store, appID, storefront, source string) ([]RatingsSnapshot, error) {
	q := `SELECT captured_at, storefront, app_id, COALESCE(average_rating,0), COALESCE(rating_count,0),
		COALESCE(histogram,''), COALESCE(version,''), COALESCE(source,''), COALESCE(data_as_of,'') FROM ratings_snapshots WHERE app_id = ?`
	args := []any{appID}
	if storefront != "" {
		q += ` AND storefront = ?`
		args = append(args, storefront)
	}
	if source != "" {
		q += ` AND source = ?`
		args = append(args, source)
	}
	q += ` ORDER BY captured_at, id`
	rows, err := st.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query ratings: %w", err)
	}
	out := make([]RatingsSnapshot, 0)
	for rows.Next() {
		var r RatingsSnapshot
		var at, hist, asOf string
		if err := rows.Scan(&at, &r.Storefront, &r.AppID, &r.AverageRating, &r.RatingCount, &hist, &r.Version, &r.Source, &asOf); err != nil {
			_ = rows.Close()
			return nil, err
		}
		r.CapturedAt = parseTS(at)
		r.DataAsOf = parseTS(asOf)
		if hist != "" {
			_ = json.Unmarshal([]byte(hist), &r.Histogram)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// --- charts ----------------------------------------------------------------

// ChartKey identifies one chart series. Source "" matches any source.
// AgeBand "" is the overall chart of the genre; a Kids chart (genre 36 with
// an ageBandId) is its own series with AgeBand set to the band id ("0",
// "10000", "10001", "10002"). The interim composite genre key
// "36-kids-<band>" is accepted and read as GenreID 36 + that band.
type ChartKey struct {
	Storefront string `json:"storefront"`
	Platform   string `json:"platform"`
	GenreID    string `json:"genreId"`
	AgeBand    string `json:"ageBand,omitempty"`
	Chart      string `json:"chart"`
	Source     string `json:"source,omitempty"`
}

// kidsGenreKeySep joins a genre and a Kids age band in the interim composite
// genre key ("36-kids-10001") that predates the age_band column.
const kidsGenreKeySep = "-kids-"

// normalizeChartGenre splits the composite genre key "36-kids-<band>" into
// genre 36 and the band when no separate band is given; otherwise it returns
// its inputs (trimmed).
func normalizeChartGenre(genreID, ageBand string) (string, string) {
	genreID, ageBand = strings.TrimSpace(genreID), strings.TrimSpace(ageBand)
	if ageBand == "" {
		if g, band, ok := strings.Cut(genreID, kidsGenreKeySep); ok && g != "" && band != "" {
			return g, band
		}
	}
	return genreID, ageBand
}

// AnyAgeBand as ChartQuery.AgeBand matches overall and Kids rows alike.
const AnyAgeBand = "*"

// ChartRow is one ranked row of a chart snapshot.
type ChartRow struct {
	Rank          int    `json:"rank"`
	AppID         string `json:"appId"`
	Title         string `json:"title,omitempty"`
	DeveloperName string `json:"developerName,omitempty"`
}

// ChartSnapshot is one capture of one chart. RowLimit is how many ranks the
// capture covered (200 from the web page, 100 from RSS); only snapshots with
// the same source and row limit are comparable. AgeBand is the Kids age band
// id of a Kids chart (ChartsPageQuery.AgeBandID) and "" for every other
// chart. CapturedAt must be the observation time (the page or feed
// FetchedAt, which a cached response keeps); zero means now.
type ChartSnapshot struct {
	CapturedAt time.Time  `json:"capturedAt"`
	Storefront string     `json:"storefront"`
	Platform   string     `json:"platform"`
	GenreID    string     `json:"genreId"`
	AgeBand    string     `json:"ageBand,omitempty"`
	Chart      string     `json:"chart"`
	Source     string     `json:"source"`
	RowLimit   int        `json:"rowLimit"`
	Rows       []ChartRow `json:"rows"`
}

// Key returns the snapshot's chart key (with its source and age band).
func (s ChartSnapshot) Key() ChartKey {
	genre, band := normalizeChartGenre(s.GenreID, s.AgeBand)
	return ChartKey{Storefront: s.Storefront, Platform: s.Platform, GenreID: genre, AgeBand: band, Chart: s.Chart, Source: s.Source}
}

// RecordChart stores one chart snapshot. Rows of an identical earlier write
// (same key, age band, source and second) are replaced, so a repeated run
// within one second cannot double the snapshot; a Kids chart and the overall
// chart written in the same second are different keys and both kept.
func RecordChart(ctx context.Context, st *Store, snap ChartSnapshot) error {
	genre, band := normalizeChartGenre(snap.GenreID, snap.AgeBand)
	if snap.Storefront == "" || snap.Platform == "" || genre == "" || snap.Chart == "" || snap.Source == "" {
		return fmt.Errorf("chart snapshot needs storefront, platform, genre, chart and source")
	}
	if band != "" && !appstore.IsAppID(band) {
		return fmt.Errorf("chart snapshot age band %q is not a numeric ageBandId", band)
	}
	at := tsOrNow(snap.CapturedAt)
	limit := snap.RowLimit
	if limit <= 0 {
		limit = len(snap.Rows)
	}
	return withWriteTx(ctx, st, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM chart_snapshots WHERE storefront = ? AND platform = ? AND genre_id = ?
			AND age_band = ? AND chart = ? AND source = ? AND captured_at = ?`, snap.Storefront, snap.Platform, genre, band, snap.Chart, snap.Source, at); err != nil {
			return err
		}
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO chart_snapshots
			(captured_at, storefront, platform, genre_id, age_band, chart, source, row_limit, rank, app_id, title, developer_name)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer func() { _ = stmt.Close() }()
		for _, r := range snap.Rows {
			if r.AppID == "" || r.Rank <= 0 {
				continue
			}
			if _, err := stmt.ExecContext(ctx, at, snap.Storefront, snap.Platform, genre, band, snap.Chart, snap.Source,
				limit, r.Rank, r.AppID, r.Title, r.DeveloperName); err != nil {
				return fmt.Errorf("chart row: %w", err)
			}
		}
		return nil
	})
}

// chartKeyWhere matches one series: the age band always takes part, so a
// key without AgeBand selects the overall chart and never a Kids chart.
func chartKeyWhere(k ChartKey) (string, []any) {
	genre, band := normalizeChartGenre(k.GenreID, k.AgeBand)
	where := ` storefront = ? AND platform = ? AND genre_id = ? AND age_band = ? AND chart = ?`
	args := []any{k.Storefront, k.Platform, genre, band, k.Chart}
	if k.Source != "" {
		where += ` AND source = ?`
		args = append(args, k.Source)
	}
	return where, args
}

// ChartCapture describes one stored capture of a chart key.
type ChartCapture struct {
	CapturedAt time.Time `json:"capturedAt"`
	Source     string    `json:"source"`
	RowLimit   int       `json:"rowLimit"`
	Rows       int       `json:"rows"`
}

// ChartCaptures lists the captures of a chart key, oldest first.
func ChartCaptures(ctx context.Context, st *Store, k ChartKey) ([]ChartCapture, error) {
	where, args := chartKeyWhere(k)
	rows, err := st.db.QueryContext(ctx, `SELECT captured_at, source, COALESCE(MAX(row_limit),0), COUNT(*) FROM chart_snapshots
		WHERE`+where+` GROUP BY captured_at, source ORDER BY captured_at, source`, args...) // #nosec G202 -- chartKeyWhere emits only constant column conditions with ? placeholders; every value is bound.
	if err != nil {
		return nil, fmt.Errorf("query chart captures: %w", err)
	}
	out := make([]ChartCapture, 0)
	for rows.Next() {
		var c ChartCapture
		var at string
		if err := rows.Scan(&at, &c.Source, &c.RowLimit, &c.Rows); err != nil {
			_ = rows.Close()
			return nil, err
		}
		c.CapturedAt = parseTS(at)
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// ChartSnapshotAt loads the rows of one capture (key.Source must be set when
// two sources were captured in the same second).
func ChartSnapshotAt(ctx context.Context, st *Store, k ChartKey, at time.Time) (*ChartSnapshot, error) {
	where, args := chartKeyWhere(k)
	args = append(args, appstore.FormatTime(at))
	rows, err := st.db.QueryContext(ctx, `SELECT source, COALESCE(row_limit,0), rank, app_id, COALESCE(title,''), COALESCE(developer_name,'')
		FROM chart_snapshots WHERE`+where+` AND captured_at = ? ORDER BY rank, id`, args...) // #nosec G202 -- chartKeyWhere emits only constant column conditions with ? placeholders; every value is bound.
	if err != nil {
		return nil, fmt.Errorf("query chart snapshot: %w", err)
	}
	genre, band := normalizeChartGenre(k.GenreID, k.AgeBand)
	snap := &ChartSnapshot{CapturedAt: at.UTC().Truncate(time.Second), Storefront: k.Storefront, Platform: k.Platform, GenreID: genre, AgeBand: band, Chart: k.Chart, Source: k.Source, Rows: make([]ChartRow, 0)}
	for rows.Next() {
		var r ChartRow
		var source string
		var limit int
		if err := rows.Scan(&source, &limit, &r.Rank, &r.AppID, &r.Title, &r.DeveloperName); err != nil {
			_ = rows.Close()
			return nil, err
		}
		snap.Source, snap.RowLimit = source, limit
		snap.Rows = append(snap.Rows, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(snap.Rows) == 0 {
		return nil, nil
	}
	return snap, nil
}

// LatestTwoCharts returns the latest capture of a chart key and the most
// recent earlier capture that is comparable to it (same source and row
// limit). prev is nil when there is no comparable earlier capture; both are
// nil when the key was never captured.
func LatestTwoCharts(ctx context.Context, st *Store, k ChartKey) (prev, latest *ChartSnapshot, err error) {
	caps, err := ChartCaptures(ctx, st, k)
	if err != nil {
		return nil, nil, err
	}
	if len(caps) == 0 {
		return nil, nil, nil
	}
	last := caps[len(caps)-1]
	lk := k
	lk.Source = last.Source
	latest, err = ChartSnapshotAt(ctx, st, lk, last.CapturedAt)
	if err != nil {
		return nil, nil, err
	}
	for i := len(caps) - 2; i >= 0; i-- {
		c := caps[i]
		if c.Source == last.Source && c.RowLimit == last.RowLimit && c.CapturedAt.Before(last.CapturedAt) {
			prev, err = ChartSnapshotAt(ctx, st, lk, c.CapturedAt)
			if err != nil {
				return nil, nil, err
			}
			break
		}
	}
	return prev, latest, nil
}

// RankPoint is one app's rank in one chart capture.
type RankPoint struct {
	CapturedAt time.Time `json:"capturedAt"`
	Rank       int       `json:"rank"`
	RowLimit   int       `json:"rowLimit"`
	Source     string    `json:"source"`
}

// ChartQuery selects stored chart rows across keys. Empty fields match all,
// except AgeBand: "" selects overall charts only (so rows grouped by
// storefront, platform, genre and chart never mix a Kids chart into the
// genre-36 series), a band id selects that Kids chart, and AnyAgeBand
// selects both (read ChartRowRecord.AgeBand to tell them apart).
type ChartQuery struct {
	Storefronts []string
	Platform    string
	GenreID     string
	AgeBand     string
	Chart       string
	Source      string
	AppID       string
	Since       time.Time
	Until       time.Time
}

// ChartRowRecord is one stored chart row with its key.
type ChartRowRecord struct {
	CapturedAt    time.Time `json:"capturedAt"`
	Storefront    string    `json:"storefront"`
	Platform      string    `json:"platform"`
	GenreID       string    `json:"genreId"`
	AgeBand       string    `json:"ageBand,omitempty"`
	Chart         string    `json:"chart"`
	Source        string    `json:"source"`
	RowLimit      int       `json:"rowLimit"`
	Rank          int       `json:"rank"`
	AppID         string    `json:"appId"`
	Title         string    `json:"title,omitempty"`
	DeveloperName string    `json:"developerName,omitempty"`
}

// ChartRows returns stored chart rows matching q, ordered by storefront,
// capture time and rank.
func ChartRows(ctx context.Context, st *Store, q ChartQuery) ([]ChartRowRecord, error) {
	sqlq := `SELECT captured_at, storefront, platform, genre_id, COALESCE(age_band,''), chart, source, COALESCE(row_limit,0), rank, app_id,
		COALESCE(title,''), COALESCE(developer_name,'') FROM chart_snapshots WHERE 1=1`
	args := []any{}
	if len(q.Storefronts) > 0 {
		sqlq += ` AND storefront IN (` + strings.TrimSuffix(strings.Repeat("?,", len(q.Storefronts)), ",") + `)`
		for _, s := range q.Storefronts {
			args = append(args, s)
		}
	}
	var genre, band string
	if q.AgeBand == AnyAgeBand {
		// Every band, unless the genre itself is a composite Kids key.
		genre, band = normalizeChartGenre(q.GenreID, "")
	} else {
		genre, band = normalizeChartGenre(q.GenreID, q.AgeBand)
	}
	if q.AgeBand != AnyAgeBand || band != "" {
		sqlq += ` AND age_band = ?`
		args = append(args, band)
	}
	for _, f := range []struct{ col, val string }{{"platform", q.Platform}, {"genre_id", genre}, {"chart", q.Chart}, {"source", q.Source}, {"app_id", q.AppID}} {
		if f.val != "" {
			sqlq += ` AND ` + f.col + ` = ?` // #nosec G202 -- f.col comes from the fixed column list above; the value is bound with ?.
			args = append(args, f.val)
		}
	}
	if !q.Since.IsZero() {
		sqlq += ` AND captured_at >= ?`
		args = append(args, appstore.FormatTime(q.Since))
	}
	if !q.Until.IsZero() {
		sqlq += ` AND captured_at <= ?`
		args = append(args, appstore.FormatTime(q.Until))
	}
	sqlq += ` ORDER BY storefront, captured_at, platform, genre_id, age_band, chart, source, rank`
	rows, err := st.db.QueryContext(ctx, sqlq, args...)
	if err != nil {
		return nil, fmt.Errorf("query chart rows: %w", err)
	}
	out := make([]ChartRowRecord, 0)
	for rows.Next() {
		var r ChartRowRecord
		var at string
		if err := rows.Scan(&at, &r.Storefront, &r.Platform, &r.GenreID, &r.AgeBand, &r.Chart, &r.Source, &r.RowLimit, &r.Rank, &r.AppID, &r.Title, &r.DeveloperName); err != nil {
			_ = rows.Close()
			return nil, err
		}
		r.CapturedAt = parseTS(at)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// --- reviews ---------------------------------------------------------------

// ReviewUpsert is one batch of reviews from one feed page.
type ReviewUpsert struct {
	Storefront string
	AppID      string
	Sort       string // mostrecent | mosthelpful (rss), recent | helpful (api), seeall
	Source     string // rss | api | web
	SeenAt     time.Time
	Reviews    []appstore.Review
}

// UpsertStats counts what an upsert did.
type UpsertStats struct {
	Inserted  int `json:"inserted"`
	Updated   int `json:"updated"`
	Unchanged int `json:"unchanged"`
}

// StoredReview is one stored review.
type StoredReview struct {
	ReviewID          string    `json:"reviewId"`
	Storefront        string    `json:"storefront"`
	AppID             string    `json:"appId"`
	Rating            int       `json:"rating"`
	Version           string    `json:"version,omitempty"`
	Title             string    `json:"title"`
	Body              string    `json:"body"`
	Author            string    `json:"author"`
	ReviewedAt        string    `json:"reviewedAt"`
	VoteSum           int       `json:"voteSum"`
	VoteCount         int       `json:"voteCount"`
	SortsSeen         []string  `json:"sortsSeen"`
	Source            string    `json:"source"`
	DeveloperResponse string    `json:"developerResponse,omitempty"`
	FirstSeenAt       time.Time `json:"firstSeenAt"`
	LastSeenAt        time.Time `json:"lastSeenAt"`
}

func reviewFTSRowID(reviewID, storefront string) int64 {
	return ftsRowID("app_store_reviews", reviewID+"\x00"+storefront)
}

// UpsertReviews merges a batch: new reviews are inserted, known ones (same
// review id and storefront, from any sort) get sorts_seen/source merged,
// first/last_seen_at widened and mutable fields refreshed. SeenAt is the
// observation time (the feed's FetchedAt, kept by the cache). A batch
// observed before the stored last sighting (an older cached page applied
// after a newer one) only merges sorts and sources: it never moves
// last_seen_at back or overwrites newer text. reviews_fts is kept in sync in
// the same transaction.
func UpsertReviews(ctx context.Context, st *Store, b ReviewUpsert) (UpsertStats, error) {
	var stats UpsertStats
	if b.AppID == "" || b.Storefront == "" {
		return stats, fmt.Errorf("review upsert needs app id and storefront")
	}
	seen := tsOrNow(b.SeenAt)
	// Only the RSS feed carries helpfulness votes; a review seen through a
	// source without them (the opt-in /api, the see-all page) keeps the
	// stored counts instead of zeroing them (NULL until a feed shows them).
	withVotes := reviewSourceCarriesVotes(b.Source)
	err := withWriteTx(ctx, st, func(tx *sql.Tx) error {
		for _, r := range b.Reviews {
			if r.ID == "" {
				continue
			}
			var voteSum, voteCount any
			if withVotes {
				voteSum, voteCount = r.VoteSum, r.VoteCount
			}
			var sorts, source, title, body, response, firstSeen, lastSeen string
			err := tx.QueryRowContext(ctx, `SELECT COALESCE(sorts_seen,''), COALESCE(source,''), COALESCE(title,''),
				COALESCE(body,''), COALESCE(developer_response,''), COALESCE(first_seen_at,''), COALESCE(last_seen_at,'')
				FROM reviews WHERE review_id = ? AND storefront = ?`,
				r.ID, b.Storefront).Scan(&sorts, &source, &title, &body, &response, &firstSeen, &lastSeen)
			switch {
			case errors.Is(err, sql.ErrNoRows):
				if _, err := tx.ExecContext(ctx, `INSERT INTO reviews
					(review_id, storefront, app_id, rating, version, title, body, author, reviewed_at, vote_sum, vote_count,
					 sorts_seen, source, developer_response, first_seen_at, last_seen_at)
					VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
					r.ID, b.Storefront, b.AppID, r.Rating, r.Version, r.Title, r.Body, r.Author, r.ReviewedAt,
					voteSum, voteCount, mergeSet("", b.Sort), mergeSet("", b.Source), r.DeveloperResponse, seen, seen); err != nil {
					return fmt.Errorf("insert review %s: %w", r.ID, err)
				}
				if err := syncReviewFTS(ctx, tx, r.ID, b.Storefront, b.AppID, r.Title, r.Body); err != nil {
					return err
				}
				stats.Inserted++
			case err != nil:
				return fmt.Errorf("read review %s: %w", r.ID, err)
			default:
				newSorts := mergeSet(sorts, b.Sort)
				newSource := mergeSet(source, b.Source)
				newFirst := firstSeen
				if newFirst == "" || seen < newFirst {
					newFirst = seen
				}
				if lastSeen != "" && seen < lastSeen {
					// An older observation than the stored one: keep the
					// newer text, rating and votes; merge only the sets.
					if _, err := tx.ExecContext(ctx, `UPDATE reviews SET sorts_seen = ?, source = ?, first_seen_at = ?
						WHERE review_id = ? AND storefront = ?`, newSorts, newSource, newFirst, r.ID, b.Storefront); err != nil {
						return fmt.Errorf("update review %s: %w", r.ID, err)
					}
					if newSorts != sorts || newSource != source || newFirst != firstSeen {
						stats.Updated++
					} else {
						stats.Unchanged++
					}
					continue
				}
				newResponse := response
				if r.DeveloperResponse != "" {
					newResponse = r.DeveloperResponse
				}
				textChanged := r.Title != title || r.Body != body
				if _, err := tx.ExecContext(ctx, `UPDATE reviews SET rating = ?, version = COALESCE(NULLIF(?, ''), version),
					title = ?, body = ?, author = COALESCE(NULLIF(?, ''), author), reviewed_at = COALESCE(NULLIF(?, ''), reviewed_at),
					vote_sum = COALESCE(?, vote_sum), vote_count = COALESCE(?, vote_count),
					sorts_seen = ?, source = ?, developer_response = ?, first_seen_at = ?, last_seen_at = ?
					WHERE review_id = ? AND storefront = ?`,
					r.Rating, r.Version, r.Title, r.Body, r.Author, r.ReviewedAt, voteSum, voteCount,
					newSorts, newSource, newResponse, newFirst, seen, r.ID, b.Storefront); err != nil {
					return fmt.Errorf("update review %s: %w", r.ID, err)
				}
				if textChanged {
					if err := syncReviewFTS(ctx, tx, r.ID, b.Storefront, b.AppID, r.Title, r.Body); err != nil {
						return err
					}
				}
				if textChanged || newSorts != sorts || newSource != source || newResponse != response {
					stats.Updated++
				} else {
					stats.Unchanged++
				}
			}
		}
		return nil
	})
	return stats, err
}

// reviewSourceCarriesVotes reports whether a review source reports
// helpfulness votes: only the customer-reviews RSS feed does (im:voteSum,
// im:voteCount).
func reviewSourceCarriesVotes(source string) bool {
	return source == SourceRSS
}

func syncReviewFTS(ctx context.Context, tx *sql.Tx, reviewID, storefront, appID, title, body string) error {
	rowid := reviewFTSRowID(reviewID, storefront)
	if _, err := tx.ExecContext(ctx, `DELETE FROM reviews_fts WHERE rowid = ?`, rowid); err != nil {
		return fmt.Errorf("reviews_fts delete: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO reviews_fts (rowid, title, body, review_id, storefront, app_id)
		VALUES (?,?,?,?,?,?)`, rowid, title, body, reviewID, storefront, appID); err != nil {
		return fmt.Errorf("reviews_fts insert: %w", err)
	}
	return nil
}

// ReviewFilter narrows Reviews and SearchReviews. Zero values match all.
type ReviewFilter struct {
	AppID       string
	Storefronts []string
	Since       time.Time // reviewed_at >= Since
	Until       time.Time // reviewed_at <= Until
	MinRating   int
	MaxRating   int
	Version     string
	Limit       int
}

func (f ReviewFilter) where(alias string) (string, []any) {
	col := func(c string) string {
		if alias == "" {
			return c
		}
		return alias + "." + c
	}
	w := ""
	args := []any{}
	if f.AppID != "" {
		w += ` AND ` + col("app_id") + ` = ?`
		args = append(args, f.AppID)
	}
	if len(f.Storefronts) > 0 {
		w += ` AND ` + col("storefront") + ` IN (` + strings.TrimSuffix(strings.Repeat("?,", len(f.Storefronts)), ",") + `)`
		for _, s := range f.Storefronts {
			args = append(args, s)
		}
	}
	if !f.Since.IsZero() {
		w += ` AND ` + col("reviewed_at") + ` >= ?`
		args = append(args, appstore.FormatTime(f.Since))
	}
	if !f.Until.IsZero() {
		w += ` AND ` + col("reviewed_at") + ` <= ?`
		args = append(args, appstore.FormatTime(f.Until))
	}
	if f.MinRating > 0 {
		w += ` AND ` + col("rating") + ` >= ?`
		args = append(args, f.MinRating)
	}
	if f.MaxRating > 0 {
		w += ` AND ` + col("rating") + ` <= ?`
		args = append(args, f.MaxRating)
	}
	if f.Version != "" {
		w += ` AND ` + col("version") + ` = ?`
		args = append(args, f.Version)
	}
	return w, args
}

const reviewColumns = `r.review_id, r.storefront, r.app_id, COALESCE(r.rating,0), COALESCE(r.version,''), COALESCE(r.title,''),
	COALESCE(r.body,''), COALESCE(r.author,''), COALESCE(r.reviewed_at,''), COALESCE(r.vote_sum,0), COALESCE(r.vote_count,0),
	COALESCE(r.sorts_seen,''), COALESCE(r.source,''), COALESCE(r.developer_response,''), COALESCE(r.first_seen_at,''),
	COALESCE(r.last_seen_at,'')`

func scanReviews(rows *sql.Rows) ([]StoredReview, error) {
	out := make([]StoredReview, 0)
	for rows.Next() {
		var r StoredReview
		var sorts, first, last string
		if err := rows.Scan(&r.ReviewID, &r.Storefront, &r.AppID, &r.Rating, &r.Version, &r.Title, &r.Body, &r.Author,
			&r.ReviewedAt, &r.VoteSum, &r.VoteCount, &sorts, &r.Source, &r.DeveloperResponse, &first, &last); err != nil {
			_ = rows.Close()
			return nil, err
		}
		r.SortsSeen = make([]string, 0, 2)
		for _, s := range strings.Split(sorts, ",") {
			if s != "" {
				r.SortsSeen = append(r.SortsSeen, s)
			}
		}
		r.FirstSeenAt, r.LastSeenAt = parseTS(first), parseTS(last)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// Reviews returns stored reviews matching f, newest review first.
func Reviews(ctx context.Context, st *Store, f ReviewFilter) ([]StoredReview, error) {
	where, args := f.where("r")
	q := `SELECT ` + reviewColumns + ` FROM reviews r WHERE 1=1` + where + ` ORDER BY r.reviewed_at DESC, r.review_id DESC` // #nosec G202 -- reviewColumns is a constant and ReviewFilter.where emits only constant conditions with ? placeholders.
	if f.Limit > 0 {
		q += ` LIMIT ?`
		args = append(args, f.Limit)
	}
	rows, err := st.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query reviews: %w", err)
	}
	return scanReviews(rows)
}

// CountReviews counts stored reviews matching f.
func CountReviews(ctx context.Context, st *Store, f ReviewFilter) (int, error) {
	where, args := f.where("r")
	var n int
	err := st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reviews r WHERE 1=1`+where, args...).Scan(&n)
	return n, err
}

// SearchReviews full-text searches stored review titles and bodies
// (reviews_fts, trigram tokenizer: substrings of 3+ characters in any
// script). Words shorter than 3 characters fall back to a LIKE scan. Results
// are ordered by relevance, then newest first.
func SearchReviews(ctx context.Context, st *Store, query string, f ReviewFilter) ([]StoredReview, error) {
	tokens := ftsQueryTokenRE.FindAllString(query, -1)
	if len(tokens) == 0 {
		return make([]StoredReview, 0), nil
	}
	where, args := f.where("r")
	short := false
	for _, t := range tokens {
		if utf8.RuneCountInString(t) < ftsTrigramMinRunes {
			short = true
			break
		}
	}
	var q string
	if short {
		// Match in Go: SQLite's lower() folds ASCII only, so "ИИ" or "Über"
		// would miss their capitalized forms.
		rows, err := st.db.QueryContext(ctx, `SELECT `+reviewColumns+` FROM reviews r WHERE 1=1`+where+` ORDER BY r.reviewed_at DESC, r.review_id DESC`, args...) // #nosec G202 -- reviewColumns is a constant and ReviewFilter.where emits only constant conditions with ? placeholders.
		if err != nil {
			return nil, fmt.Errorf("search reviews: %w", err)
		}
		all, err := scanReviews(rows)
		if err != nil {
			return nil, err
		}
		out := make([]StoredReview, 0)
		for _, r := range all {
			text := strings.ToLower(r.Title + "\n" + r.Body)
			match := true
			for _, t := range tokens {
				if !strings.Contains(text, strings.ToLower(t)) {
					match = false
					break
				}
			}
			if match {
				out = append(out, r)
				if f.Limit > 0 && len(out) == f.Limit {
					break
				}
			}
		}
		return out, nil
	} else {
		q = `SELECT ` + reviewColumns + ` FROM reviews_fts f JOIN reviews r ON r.review_id = f.review_id AND r.storefront = f.storefront
			WHERE reviews_fts MATCH ?` + where + ` ORDER BY f.rank, r.reviewed_at DESC`
		args = append([]any{FTSMatchQuery(query)}, args...)
	}
	if f.Limit > 0 {
		q += ` LIMIT ?`
		args = append(args, f.Limit)
	}
	rows, err := st.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("search reviews: %w", err)
	}
	return scanReviews(rows)
}

// --- featured --------------------------------------------------------------

// FeaturedStory is one Today story in a featured capture.
type FeaturedStory struct {
	Position int      `json:"position"`
	StoryID  string   `json:"storyId"`
	Title    string   `json:"title"`
	AppIDs   []string `json:"appIds"`
}

// FeaturedSnapshot is one capture of the Today tab.
type FeaturedSnapshot struct {
	CapturedAt time.Time       `json:"capturedAt"`
	Storefront string          `json:"storefront"`
	Platform   string          `json:"platform"`
	Stories    []FeaturedStory `json:"stories"`
}

// RecordFeatured stores one Today capture: a row per (story, featured app);
// a story without apps is stored with an empty app id. CapturedAt is the
// observation time (TodayPage.FetchedAt); zero means now. A capture of the
// same storefront and platform at the same time is stored once.
func RecordFeatured(ctx context.Context, st *Store, snap FeaturedSnapshot) error {
	_, err := RecordFeaturedOnce(ctx, st, snap)
	return err
}

// RecordFeaturedOnce is RecordFeatured that reports whether it wrote the
// capture (false: this observation was already stored).
func RecordFeaturedOnce(ctx context.Context, st *Store, snap FeaturedSnapshot) (bool, error) {
	if snap.Storefront == "" {
		return false, fmt.Errorf("featured snapshot needs a storefront")
	}
	at := tsOrNow(snap.CapturedAt)
	written := false
	err := withWriteTx(ctx, st, func(tx *sql.Tx) error {
		var stored int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM featured_snapshots WHERE storefront = ? AND COALESCE(platform,'') = ? AND captured_at = ?`,
			snap.Storefront, snap.Platform, at).Scan(&stored); err != nil {
			return fmt.Errorf("read featured snapshot: %w", err)
		}
		if stored > 0 {
			return nil
		}
		written = true
		for _, s := range snap.Stories {
			ids := s.AppIDs
			if len(ids) == 0 {
				ids = []string{""}
			}
			for _, id := range ids {
				if _, err := tx.ExecContext(ctx, `INSERT INTO featured_snapshots
					(captured_at, storefront, platform, story_id, story_title, app_id, position) VALUES (?,?,?,?,?,?,?)`,
					at, snap.Storefront, snap.Platform, s.StoryID, s.Title, id, s.Position); err != nil {
					return fmt.Errorf("featured row: %w", err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return written, nil
}

// FeaturedRow is one stored (story, app) featuring.
type FeaturedRow struct {
	CapturedAt time.Time `json:"capturedAt"`
	Storefront string    `json:"storefront"`
	Platform   string    `json:"platform"`
	StoryID    string    `json:"storyId"`
	StoryTitle string    `json:"storyTitle"`
	AppID      string    `json:"appId,omitempty"`
	Position   int       `json:"position"`
}

// FeaturedSince returns featured rows captured at or after since, oldest
// capture first and story position within a capture. storefront "" means all.
func FeaturedSince(ctx context.Context, st *Store, storefront string, since time.Time) ([]FeaturedRow, error) {
	q := `SELECT captured_at, storefront, COALESCE(platform,''), COALESCE(story_id,''), COALESCE(story_title,''),
		COALESCE(app_id,''), COALESCE(position,0) FROM featured_snapshots WHERE captured_at >= ?`
	sinceStr := ""
	if !since.IsZero() {
		sinceStr = appstore.FormatTime(since)
	}
	args := []any{sinceStr}
	if storefront != "" {
		q += ` AND storefront = ?`
		args = append(args, storefront)
	}
	q += ` ORDER BY captured_at, storefront, position, id`
	rows, err := st.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query featured: %w", err)
	}
	out := make([]FeaturedRow, 0)
	for rows.Next() {
		var r FeaturedRow
		var at string
		if err := rows.Scan(&at, &r.Storefront, &r.Platform, &r.StoryID, &r.StoryTitle, &r.AppID, &r.Position); err != nil {
			_ = rows.Close()
			return nil, err
		}
		r.CapturedAt = parseTS(at)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// --- keyword ranks ---------------------------------------------------------

// KeywordRank is one keyword capture. AppID is the tracked target ("" for a
// plain store search); Rank 0 means the target was not in the scanned
// results (stored as NULL). TopIDs is the ranked result list (top-N).
type KeywordRank struct {
	CapturedAt  time.Time `json:"capturedAt"`
	Storefront  string    `json:"storefront"`
	Platform    string    `json:"platform"`
	Term        string    `json:"term"`
	Source      string    `json:"source"`
	AppID       string    `json:"appId,omitempty"`
	Rank        int       `json:"rank"`
	ResultCount int       `json:"resultCount"`
	TopIDs      []string  `json:"topIds"`
}

// RecordKeywordRank appends one keyword capture. CapturedAt is the
// observation time (SearchResponse.FetchedAt or SearchPage.FetchedAt); zero
// means now. A capture of the same term, storefront, platform, source and
// target at the same time is stored once.
func RecordKeywordRank(ctx context.Context, st *Store, k KeywordRank) error {
	_, err := RecordKeywordRankOnce(ctx, st, k)
	return err
}

// RecordKeywordRankOnce is RecordKeywordRank that reports whether it wrote
// the capture (false: this observation was already stored).
func RecordKeywordRankOnce(ctx context.Context, st *Store, k KeywordRank) (bool, error) {
	term := NormalizeKeywordTerm(k.Term)
	if term == "" || k.Storefront == "" {
		return false, fmt.Errorf("keyword rank needs a term and a storefront")
	}
	top := k.TopIDs
	if top == nil {
		top = []string{}
	}
	topJSON, _ := json.Marshal(top)
	var rank any
	if k.Rank > 0 {
		rank = k.Rank
	}
	at := tsOrNow(k.CapturedAt)
	written := false
	err := withWriteTx(ctx, st, func(tx *sql.Tx) error {
		var stored int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM keyword_ranks WHERE term = ? AND storefront = ?
			AND COALESCE(platform,'') = ? AND COALESCE(source,'') = ? AND COALESCE(app_id,'') = ? AND captured_at = ?`,
			term, k.Storefront, k.Platform, k.Source, k.AppID, at).Scan(&stored); err != nil {
			return fmt.Errorf("read keyword capture: %w", err)
		}
		if stored > 0 {
			return nil
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO keyword_ranks
			(captured_at, storefront, platform, term, source, app_id, rank, result_count, top_ids) VALUES (?,?,?,?,?,?,?,?,?)`,
			at, k.Storefront, k.Platform, term, k.Source, k.AppID, rank, k.ResultCount, string(topJSON))
		written = err == nil
		return err
	})
	if err != nil {
		return false, err
	}
	return written, nil
}

// NormalizeKeywordTerm is the one spelling a search term is stored and looked
// up under: lower case, inner whitespace collapsed to single spaces.
func NormalizeKeywordTerm(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// KeywordFilter narrows KeywordSeries. Empty fields match all; Term is
// matched after NormalizeKeywordTerm.
type KeywordFilter struct {
	Term       string
	Storefront string
	Platform   string
	Source     string
	AppID      string
	Since      time.Time
}

// KeywordSeries returns keyword captures oldest first.
func KeywordSeries(ctx context.Context, st *Store, f KeywordFilter) ([]KeywordRank, error) {
	q := `SELECT captured_at, storefront, COALESCE(platform,''), term, COALESCE(source,''), COALESCE(app_id,''),
		COALESCE(rank,0), COALESCE(result_count,0), COALESCE(top_ids,'[]') FROM keyword_ranks WHERE 1=1`
	args := []any{}
	if f.Term != "" {
		q += ` AND term = ?`
		args = append(args, NormalizeKeywordTerm(f.Term))
	}
	for _, c := range []struct{ col, val string }{{"storefront", f.Storefront}, {"platform", f.Platform}, {"source", f.Source}, {"app_id", f.AppID}} {
		if c.val != "" {
			q += ` AND ` + c.col + ` = ?` // #nosec G202 -- c.col comes from the fixed column list above; the value is bound with ?.
			args = append(args, c.val)
		}
	}
	if !f.Since.IsZero() {
		q += ` AND captured_at >= ?`
		args = append(args, appstore.FormatTime(f.Since))
	}
	q += ` ORDER BY captured_at, id`
	rows, err := st.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query keyword ranks: %w", err)
	}
	out := make([]KeywordRank, 0)
	for rows.Next() {
		var k KeywordRank
		var at, top string
		if err := rows.Scan(&at, &k.Storefront, &k.Platform, &k.Term, &k.Source, &k.AppID, &k.Rank, &k.ResultCount, &top); err != nil {
			_ = rows.Close()
			return nil, err
		}
		k.CapturedAt = parseTS(at)
		k.TopIDs = make([]string, 0)
		_ = json.Unmarshal([]byte(top), &k.TopIDs)
		out = append(out, k)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// ReviewFeedCapped reports whether a stored capture of Apple's reviews RSS
// for appID in storefront returned a full last page (page 10 with 50
// entries, in any sort): the feed's 500-review cap was reached, so older
// reviews were not fetched.
func ReviewFeedCapped(ctx context.Context, st *Store, appID, storefront string) (bool, error) {
	var n int
	err := st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM captures WHERE url LIKE ? ESCAPE '\' AND COALESCE(entry_count,0) >= 50`,
		"%/"+likeEscape(storefront)+"/rss/customerreviews/page=10/id="+likeEscape(appID)+"/%").Scan(&n)
	if err != nil {
		return false, fmt.Errorf("read review capture log: %w", err)
	}
	return n > 0, nil
}

// likeEscape escapes LIKE wildcards in s for an ESCAPE '\' pattern.
func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
