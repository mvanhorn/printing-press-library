// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored Phase 3 domain tables for the Vagaro marketplace. Kept in a
// separate file from the generator-owned migration slice in store.go: these
// tables are created lazily via EnsureVagaroTables (CREATE TABLE IF NOT
// EXISTS) the first time a Vagaro sync/read touches them, so the generic
// resource-store migration path is untouched.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// vagaroTablesDDL creates the domain tables. Each is idempotent.
var vagaroTablesDDL = []string{
	`CREATE TABLE IF NOT EXISTS businesses (
		slug          TEXT PRIMARY KEY,
		business_id   TEXT NOT NULL,
		name          TEXT,
		rating        REAL,
		review_count  INTEGER,
		price_range   TEXT,
		city          TEXT,
		state         TEXT,
		address       TEXT,
		phone         TEXT,
		category      TEXT,
		synced_at     TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_businesses_business_id ON businesses(business_id)`,
	`CREATE TABLE IF NOT EXISTS services (
		business_id   TEXT NOT NULL,
		service_id    TEXT NOT NULL,
		title         TEXT,
		price_text    TEXT,
		price_cents   INTEGER,
		category      TEXT,
		synced_at     TEXT NOT NULL,
		PRIMARY KEY (business_id, service_id)
	)`,
	`CREATE TABLE IF NOT EXISTS providers (
		business_id   TEXT NOT NULL,
		provider_id   TEXT NOT NULL,
		name          TEXT,
		synced_at     TEXT NOT NULL,
		PRIMARY KEY (business_id, provider_id)
	)`,
	`CREATE TABLE IF NOT EXISTS reviews (
		business_id   TEXT NOT NULL,
		review_id     TEXT NOT NULL,
		provider_id   TEXT,
		rating        REAL,
		text          TEXT,
		author        TEXT,
		date          TEXT,
		synced_at     TEXT NOT NULL,
		PRIMARY KEY (business_id, review_id)
	)`,
	// services_snapshots is append-only: every sync writes a fresh timestamped
	// snapshot of a business's menu so menu-diff can compare two syncs. Distinct
	// from the services table (which overwrites in place for current-state reads).
	`CREATE TABLE IF NOT EXISTS services_snapshots (
		business_id   TEXT NOT NULL,
		snapshot_at   TEXT NOT NULL,
		service_id    TEXT NOT NULL,
		title         TEXT,
		price_cents   INTEGER,
		PRIMARY KEY (business_id, snapshot_at, service_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_snapshots_biz_time ON services_snapshots(business_id, snapshot_at)`,
	// watch_baselines stores the last-seen next-available slot for a
	// business+service+provider so watch can report when a sooner slot opens
	// up. provider is part of the key because availability is provider-scoped;
	// an empty provider means "any provider" and keys its own baseline.
	`CREATE TABLE IF NOT EXISTS watch_baselines (
		slug            TEXT NOT NULL,
		service_id      TEXT NOT NULL,
		provider        TEXT NOT NULL DEFAULT '',
		next_available  TEXT,
		before_target   TEXT,
		recorded_at     TEXT NOT NULL,
		PRIMARY KEY (slug, service_id, provider)
	)`,
}

// EnsureVagaroTables lazily creates the domain tables. Safe to call on every
// Vagaro store operation; CREATE TABLE IF NOT EXISTS is a no-op once created.
func (s *Store) EnsureVagaroTables(ctx context.Context) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	// Migration: watch_baselines gained a provider column as part of its
	// primary key. CREATE TABLE IF NOT EXISTS below is a no-op on databases
	// created before that change, leaving the old provider-less schema in
	// place so provider-scoped reads/writes fail. Baselines are a regenerable
	// cache (the next watch run re-establishes them), so drop the stale table
	// and let the DDL loop recreate it with the current key shape.
	if err := s.migrateWatchBaselinesProvider(ctx); err != nil {
		return fmt.Errorf("migrating watch_baselines: %w", err)
	}
	for _, ddl := range vagaroTablesDDL {
		if _, err := s.db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("creating vagaro tables: %w", err)
		}
	}
	return nil
}

// migrateWatchBaselinesProvider drops a pre-provider watch_baselines table so
// EnsureVagaroTables can rebuild it with the provider-scoped primary key. It is
// idempotent: a missing table or an already-current table is left untouched.
func (s *Store) migrateWatchBaselinesProvider(ctx context.Context) error {
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='table' AND name='watch_baselines'`,
	).Scan(&name)
	if err == sql.ErrNoRows {
		return nil // table doesn't exist yet; the DDL loop will create it fresh
	}
	if err != nil {
		return err
	}
	var providerCols int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pragma_table_info('watch_baselines') WHERE name='provider'`,
	).Scan(&providerCols); err != nil {
		return err
	}
	if providerCols > 0 {
		return nil // already migrated
	}
	_, err = s.db.ExecContext(ctx, `DROP TABLE watch_baselines`)
	return err
}

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

// BusinessRecord is a row in the businesses table.
type BusinessRecord struct {
	Slug        string
	BusinessID  string
	Name        string
	Rating      float64
	ReviewCount int
	PriceRange  string
	City        string
	State       string
	Address     string
	Phone       string
	Category    string
}

// UpsertBusiness inserts or updates a business row keyed by slug.
func (s *Store) UpsertBusiness(ctx context.Context, b BusinessRecord) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO businesses
			(slug, business_id, name, rating, review_count, price_range, city, state, address, phone, category, synced_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(slug) DO UPDATE SET
			business_id=COALESCE(NULLIF(excluded.business_id, ''), businesses.business_id),
			name=COALESCE(NULLIF(excluded.name, ''), businesses.name),
			rating=COALESCE(excluded.rating, businesses.rating),
			review_count=COALESCE(excluded.review_count, businesses.review_count),
			price_range=COALESCE(NULLIF(excluded.price_range, ''), businesses.price_range),
			city=COALESCE(NULLIF(excluded.city, ''), businesses.city),
			state=COALESCE(NULLIF(excluded.state, ''), businesses.state),
			address=COALESCE(NULLIF(excluded.address, ''), businesses.address),
			phone=COALESCE(NULLIF(excluded.phone, ''), businesses.phone),
			category=COALESCE(NULLIF(excluded.category, ''), businesses.category),
			synced_at=excluded.synced_at`,
		b.Slug, b.BusinessID, nullIfEmpty(b.Name), nullIfZeroF(b.Rating), nullIfZero(b.ReviewCount),
		nullIfEmpty(b.PriceRange), nullIfEmpty(b.City), nullIfEmpty(b.State),
		nullIfEmpty(b.Address), nullIfEmpty(b.Phone), nullIfEmpty(b.Category), nowUTC(),
	)
	if err != nil {
		return fmt.Errorf("upserting business %q: %w", b.Slug, err)
	}
	return nil
}

// GetBusinessIDBySlug returns the cached businessID for a slug, or empty
// string (with a nil error) when the slug is not yet synced.
func (s *Store) GetBusinessIDBySlug(ctx context.Context, slug string) (string, error) {
	var id sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT business_id FROM businesses WHERE slug = ?`, slug).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return id.String, nil
}

// ListBusinessSlugs returns every known business slug, drained fully before
// returning so the read cursor is released promptly.
func (s *Store) ListBusinessSlugs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT slug FROM businesses ORDER BY slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var slug sql.NullString
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		if slug.Valid && slug.String != "" {
			out = append(out, slug.String)
		}
	}
	return out, rows.Err()
}

// ServiceRecord is a row in the services table.
type ServiceRecord struct {
	ServiceID  string
	Title      string
	PriceText  string
	PriceCents int
	Category   string
}

// UpsertServices replaces the service rows for a business.
func (s *Store) UpsertServices(ctx context.Context, businessID string, rows []ServiceRecord) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ts := nowUTC()
	for _, r := range rows {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO services (business_id, service_id, title, price_text, price_cents, category, synced_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(business_id, service_id) DO UPDATE SET
				title=excluded.title, price_text=excluded.price_text,
				price_cents=excluded.price_cents, category=excluded.category, synced_at=excluded.synced_at`,
			businessID, r.ServiceID, nullIfEmpty(r.Title), nullIfEmpty(r.PriceText),
			nullIfZero(r.PriceCents), nullIfEmpty(r.Category), ts,
		); err != nil {
			return fmt.Errorf("upserting service %q: %w", r.ServiceID, err)
		}
	}
	return tx.Commit()
}

// ProviderRecord is a row in the providers table.
type ProviderRecord struct {
	ProviderID string
	Name       string
}

// UpsertProviders replaces the provider rows for a business.
func (s *Store) UpsertProviders(ctx context.Context, businessID string, rows []ProviderRecord) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ts := nowUTC()
	for _, r := range rows {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO providers (business_id, provider_id, name, synced_at)
			 VALUES (?, ?, ?, ?)
			 ON CONFLICT(business_id, provider_id) DO UPDATE SET
				name=excluded.name, synced_at=excluded.synced_at`,
			businessID, r.ProviderID, nullIfEmpty(r.Name), ts,
		); err != nil {
			return fmt.Errorf("upserting provider %q: %w", r.ProviderID, err)
		}
	}
	return tx.Commit()
}

// ReviewRecord is a row in the reviews table.
type ReviewRecord struct {
	ReviewID   string
	ProviderID string
	Rating     float64
	Text       string
	Author     string
	Date       string
}

// UpsertReviews replaces the review rows for a business.
func (s *Store) UpsertReviews(ctx context.Context, businessID string, rows []ReviewRecord) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ts := nowUTC()
	for _, r := range rows {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO reviews (business_id, review_id, provider_id, rating, text, author, date, synced_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(business_id, review_id) DO UPDATE SET
				provider_id=excluded.provider_id, rating=excluded.rating, text=excluded.text,
				author=excluded.author, date=excluded.date, synced_at=excluded.synced_at`,
			businessID, r.ReviewID, nullIfEmpty(r.ProviderID), nullIfZeroF(r.Rating),
			nullIfEmpty(r.Text), nullIfEmpty(r.Author), nullIfEmpty(r.Date), ts,
		); err != nil {
			return fmt.Errorf("upserting review %q: %w", r.ReviewID, err)
		}
	}
	return tx.Commit()
}

// scanBusinessRecord reads a full businesses row with NULL-safe columns.
func scanBusinessRecord(scan func(dest ...any) error) (BusinessRecord, error) {
	var (
		b           BusinessRecord
		name        sql.NullString
		rating      sql.NullFloat64
		reviewCount sql.NullInt64
		priceRange  sql.NullString
		city        sql.NullString
		state       sql.NullString
		address     sql.NullString
		phone       sql.NullString
		category    sql.NullString
	)
	if err := scan(&b.Slug, &b.BusinessID, &name, &rating, &reviewCount, &priceRange,
		&city, &state, &address, &phone, &category); err != nil {
		return BusinessRecord{}, err
	}
	b.Name = name.String
	b.Rating = rating.Float64
	b.ReviewCount = int(reviewCount.Int64)
	b.PriceRange = priceRange.String
	b.City = city.String
	b.State = state.String
	b.Address = address.String
	b.Phone = phone.String
	b.Category = category.String
	return b, nil
}

const businessSelectCols = `slug, business_id, name, rating, review_count, price_range, city, state, address, phone, category`

// GetBusinessBySlug returns the full business row for a slug. found=false (nil
// error) when the slug has not been synced.
func (s *Store) GetBusinessBySlug(ctx context.Context, slug string) (BusinessRecord, bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+businessSelectCols+` FROM businesses WHERE slug = ?`, slug)
	b, err := scanBusinessRecord(row.Scan)
	if err == sql.ErrNoRows {
		return BusinessRecord{}, false, nil
	}
	if err != nil {
		return BusinessRecord{}, false, err
	}
	return b, true, nil
}

// ListBusinesses returns every synced business row, drained fully before
// returning so the read cursor is released promptly.
func (s *Store) ListBusinesses(ctx context.Context) ([]BusinessRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+businessSelectCols+` FROM businesses ORDER BY slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BusinessRecord
	for rows.Next() {
		b, err := scanBusinessRecord(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// InsertServiceSnapshot appends a timestamped menu snapshot for a business.
// All rows share snapshotAt so menu-diff can group by snapshot. A re-run with
// the same timestamp is idempotent (PK on business_id+snapshot_at+service_id).
func (s *Store) InsertServiceSnapshot(ctx context.Context, businessID, snapshotAt string, rows []ServiceRecord) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, r := range rows {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO services_snapshots (business_id, snapshot_at, service_id, title, price_cents)
			 VALUES (?, ?, ?, ?, ?)
			 ON CONFLICT(business_id, snapshot_at, service_id) DO UPDATE SET
				title=excluded.title, price_cents=excluded.price_cents`,
			businessID, snapshotAt, r.ServiceID, nullIfEmpty(r.Title), nullIfZero(r.PriceCents),
		); err != nil {
			return fmt.Errorf("inserting service snapshot %q: %w", r.ServiceID, err)
		}
	}
	return tx.Commit()
}

// SnapshotRow is one service entry within a menu snapshot.
type SnapshotRow struct {
	ServiceID  string
	Title      string
	PriceCents int
}

// RecentSnapshotTimes returns up to n distinct snapshot timestamps for a
// business, newest first.
func (s *Store) RecentSnapshotTimes(ctx context.Context, businessID string, n int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT snapshot_at FROM services_snapshots WHERE business_id = ?
		 ORDER BY snapshot_at DESC LIMIT ?`, businessID, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t sql.NullString
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		if t.Valid {
			out = append(out, t.String)
		}
	}
	return out, rows.Err()
}

// SnapshotServices returns the service rows for one snapshot timestamp.
func (s *Store) SnapshotServices(ctx context.Context, businessID, snapshotAt string) ([]SnapshotRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT service_id, title, price_cents FROM services_snapshots
		 WHERE business_id = ? AND snapshot_at = ? ORDER BY service_id`, businessID, snapshotAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SnapshotRow
	for rows.Next() {
		var (
			id    sql.NullString
			title sql.NullString
			cents sql.NullInt64
		)
		if err := rows.Scan(&id, &title, &cents); err != nil {
			return nil, err
		}
		out = append(out, SnapshotRow{ServiceID: id.String, Title: title.String, PriceCents: int(cents.Int64)})
	}
	return out, rows.Err()
}

// WatchBaseline is the stored next-available baseline for a business+service.
type WatchBaseline struct {
	NextAvailable string
	BeforeTarget  string
	RecordedAt    string
}

// GetWatchBaseline returns the stored baseline, found=false when none exists.
// provider scopes the key; an empty provider addresses the "any provider"
// baseline, kept distinct from any provider-specific baseline.
func (s *Store) GetWatchBaseline(ctx context.Context, slug, serviceID, provider string) (WatchBaseline, bool, error) {
	var (
		next   sql.NullString
		before sql.NullString
		rec    sql.NullString
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT next_available, before_target, recorded_at FROM watch_baselines
		 WHERE slug = ? AND service_id = ? AND provider = ?`, slug, serviceID, provider).Scan(&next, &before, &rec)
	if err == sql.ErrNoRows {
		return WatchBaseline{}, false, nil
	}
	if err != nil {
		return WatchBaseline{}, false, err
	}
	return WatchBaseline{NextAvailable: next.String, BeforeTarget: before.String, RecordedAt: rec.String}, true, nil
}

// UpsertWatchBaseline records (or refreshes) the next-available baseline.
// provider is part of the key; an empty provider keys the "any provider"
// baseline distinct from any provider-specific one.
func (s *Store) UpsertWatchBaseline(ctx context.Context, slug, serviceID, provider, nextAvailable, beforeTarget string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO watch_baselines (slug, service_id, provider, next_available, before_target, recorded_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(slug, service_id, provider) DO UPDATE SET
			next_available=excluded.next_available, before_target=excluded.before_target,
			recorded_at=excluded.recorded_at`,
		slug, serviceID, provider, nullIfEmpty(nextAvailable), nullIfEmpty(beforeTarget), nowUTC(),
	)
	if err != nil {
		return fmt.Errorf("upserting watch baseline %q/%q/%q: %w", slug, serviceID, provider, err)
	}
	return nil
}

// nullIfEmpty stores NULL for empty strings so NULL-safe reads stay meaningful.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullIfZero(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

func nullIfZeroF(f float64) any {
	if f == 0 {
		return nil
	}
	return f
}
