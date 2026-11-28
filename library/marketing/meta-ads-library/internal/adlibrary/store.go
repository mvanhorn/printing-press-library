// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package adlibrary

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/store"
)

const adLibrarySchemaSQL = `
CREATE TABLE IF NOT EXISTS meta_ads (
	ad_archive_id TEXT PRIMARY KEY,
	collation_id TEXT,
	collation_count INTEGER NOT NULL DEFAULT 1,
	page_id TEXT NOT NULL,
	page_name TEXT NOT NULL,
	is_active INTEGER NOT NULL DEFAULT 1,
	start_date TEXT,
	end_date TEXT,
	country TEXT,
	ad_type TEXT,
	publisher_platforms_json TEXT,
	display_format TEXT,
	body_text TEXT,
	link_title TEXT,
	link_caption TEXT,
	link_description TEXT,
	cta_text TEXT,
	cta_type TEXT,
	landing_url TEXT,
	landing_domain TEXT,
	byline TEXT,
	currency TEXT,
	spend_lower INTEGER,
	spend_upper INTEGER,
	impressions_lower INTEGER,
	impressions_upper INTEGER,
	raw_json TEXT NOT NULL,
	collected_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_meta_ads_page_id ON meta_ads(page_id);
CREATE INDEX IF NOT EXISTS idx_meta_ads_page_name ON meta_ads(LOWER(page_name));
CREATE INDEX IF NOT EXISTS idx_meta_ads_landing_domain ON meta_ads(landing_domain);
CREATE INDEX IF NOT EXISTS idx_meta_ads_active_start ON meta_ads(is_active, start_date);

CREATE TABLE IF NOT EXISTS meta_advertisers (
	page_id TEXT PRIMARY KEY,
	page_name TEXT NOT NULL,
	category TEXT,
	likes INTEGER DEFAULT 0,
	verification TEXT,
	ig_username TEXT,
	ig_followers INTEGER DEFAULT 0,
	country TEXT,
	raw_json TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS meta_snapshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	page_id TEXT NOT NULL,
	page_name TEXT,
	country TEXT,
	query_string TEXT,
	captured_at TEXT NOT NULL,
	ad_count INTEGER NOT NULL DEFAULT 0,
	is_complete INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_meta_snapshots_page ON meta_snapshots(page_id, country, captured_at DESC);

CREATE TABLE IF NOT EXISTS meta_snapshot_ads (
	snapshot_id INTEGER NOT NULL,
	ad_archive_id TEXT NOT NULL,
	collation_count INTEGER NOT NULL DEFAULT 1,
	is_active INTEGER NOT NULL DEFAULT 1,
	PRIMARY KEY (snapshot_id, ad_archive_id)
);

CREATE TABLE IF NOT EXISTS meta_ad_details (
	ad_archive_id TEXT PRIMARY KEY,
	page_id TEXT NOT NULL,
	page_name TEXT,
	country TEXT,
	byline TEXT,
	eu_total_reach INTEGER DEFAULT 0,
	uk_total_reach INTEGER DEFAULT 0,
	br_total_reach INTEGER DEFAULT 0,
	target_gender TEXT,
	is_violating_eu_siep INTEGER DEFAULT 0,
	ai_disclosure_eligible INTEGER DEFAULT 0,
	raw_json TEXT NOT NULL,
	fetched_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_meta_ad_details_page ON meta_ad_details(page_id);
`

// EnsureSchema creates the Meta Ads Library tables and indexes if absent,
// and applies idempotent additive migrations on existing local databases.
func EnsureSchema(ctx context.Context, s *store.Store) error {
	if s == nil {
		return fmt.Errorf("store is nil")
	}
	if _, err := s.DB().ExecContext(ctx, adLibrarySchemaSQL); err != nil {
		return err
	}
	hasCol, err := columnExists(ctx, s, "meta_snapshots", "is_complete")
	if err != nil {
		return err
	}
	if !hasCol {
		if _, err := s.DB().ExecContext(ctx, `ALTER TABLE meta_snapshots ADD COLUMN is_complete INTEGER NOT NULL DEFAULT 1`); err != nil {
			return err
		}
	}
	return nil
}

func columnExists(ctx context.Context, s *store.Store, table, col string) (bool, error) {
	rows, err := s.DB().QueryContext(ctx, `SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return false, err
	}
	found := false
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return false, err
		}
		if strings.EqualFold(name, col) {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return false, err
	}
	_ = rows.Close()
	return found, nil
}

func tableExists(ctx context.Context, s *store.Store, table string) (bool, error) {
	if s == nil {
		return false, fmt.Errorf("store is nil")
	}
	var name string
	err := s.DB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name = ? LIMIT 1`, table).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return name == table, nil
}

type mirrorRow struct {
	id  string
	raw json.RawMessage
}

const utcTimestampLayout = "2006-01-02T15:04:05.000000Z"

func parseStoredTimestamp(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return ts.UTC(), true
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts.UTC(), true
	}
	return time.Time{}, false
}

func ftsRowIDForResource(resourceType, id string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(resourceType))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(id))
	return int64(h.Sum64() & 0x7FFFFFFFFFFFFFFF)
}

// adSearchableText mirrors store.searchableResourceContent by recursively
// extracting all searchable string values from the serialized Ad JSON
// (including publisher_platform, page_categories, and all carousel cards) so
// local FTS5 search results remain identical to s.Upsert.
func adSearchableText(_ Ad, raw []byte) string {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return string(raw)
	}
	var parts []string
	collectAdSearchableStrings(&parts, "", value)
	if len(parts) == 0 {
		return string(raw)
	}
	return strings.Join(parts, " ")
}

func collectAdSearchableStrings(parts *[]string, key string, value any) {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, childKey := range keys {
			collectAdSearchableStrings(parts, childKey, v[childKey])
		}
	case []any:
		for _, child := range v {
			collectAdSearchableStrings(parts, key, child)
		}
	case string:
		s := strings.TrimSpace(v)
		if len(s) < 2 {
			return
		}
		lower := strings.ToLower(s)
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
			return
		}
		*parts = append(*parts, s)
	}
}

func upsertFrameworkResourceTx(ctx context.Context, conn *sql.Conn, resourceType, id string, raw []byte, searchable, ts string) error {
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO resources (id, resource_type, data, synced_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(resource_type, id) DO UPDATE SET
			data = excluded.data,
			synced_at = excluded.synced_at,
			updated_at = excluded.updated_at
	`, id, resourceType, string(raw), ts, ts); err != nil {
		return err
	}
	ftsID := ftsRowIDForResource(resourceType, id)
	_, _ = conn.ExecContext(ctx, `DELETE FROM resources_fts WHERE rowid = ?`, ftsID)
	_, _ = conn.ExecContext(ctx, `
		INSERT INTO resources_fts (rowid, id, resource_type, content)
		VALUES (?, ?, ?, ?)
	`, ftsID, id, resourceType, searchable)
	return nil
}

// SaveAdsBatch persists normalized Ad rows into meta_ads AND mirrors them into
// the framework resources/FTS5 store inside a single BEGIN IMMEDIATE write
// transaction so overlapping searches and reconciliations can never diverge
// between meta_ads and resources.
func SaveAdsBatch(ctx context.Context, s *store.Store, ads []Ad) error {
	if len(ads) == 0 {
		return nil
	}
	if err := EnsureSchema(ctx, s); err != nil {
		return err
	}

	nowStr := time.Now().UTC().Format(utcTimestampLayout)
	conn, err := s.DB().Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire meta_ads conn: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin meta_ads tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()

	stmt, err := conn.PrepareContext(ctx, `
		INSERT INTO meta_ads (
			ad_archive_id, collation_id, collation_count, page_id, page_name,
			is_active, start_date, end_date, country, ad_type,
			publisher_platforms_json, display_format, body_text, link_title,
			link_caption, link_description, cta_text, cta_type,
			landing_url, landing_domain, byline, currency,
			spend_lower, spend_upper, impressions_lower, impressions_upper,
			raw_json, collected_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(ad_archive_id) DO UPDATE SET
			collation_id=excluded.collation_id,
			collation_count=excluded.collation_count,
			page_id=excluded.page_id,
			page_name=excluded.page_name,
			is_active=excluded.is_active,
			start_date=COALESCE(NULLIF(excluded.start_date, ''), meta_ads.start_date),
			end_date=excluded.end_date,
			country=COALESCE(NULLIF(excluded.country, ''), meta_ads.country),
			ad_type=COALESCE(NULLIF(excluded.ad_type, ''), meta_ads.ad_type),
			publisher_platforms_json=excluded.publisher_platforms_json,
			display_format=excluded.display_format,
			body_text=excluded.body_text,
			link_title=excluded.link_title,
			link_caption=excluded.link_caption,
			link_description=excluded.link_description,
			cta_text=excluded.cta_text,
			cta_type=excluded.cta_type,
			landing_url=excluded.landing_url,
			landing_domain=excluded.landing_domain,
			byline=COALESCE(NULLIF(excluded.byline, ''), meta_ads.byline),
			currency=COALESCE(NULLIF(excluded.currency, ''), meta_ads.currency),
			spend_lower=COALESCE(excluded.spend_lower, meta_ads.spend_lower),
			spend_upper=COALESCE(excluded.spend_upper, meta_ads.spend_upper),
			impressions_lower=COALESCE(excluded.impressions_lower, meta_ads.impressions_lower),
			impressions_upper=COALESCE(excluded.impressions_upper, meta_ads.impressions_upper),
			raw_json=excluded.raw_json,
			collected_at=excluded.collected_at
	`)
	if err != nil {
		return err
	}

	written := 0
	for _, ad := range ads {
		if ad.AdArchiveID == "" {
			continue
		}
		if ad.CollectedAt == "" {
			ad.CollectedAt = nowStr
		}
		// Never let an older observation overwrite a strictly newer one.
		var existingCollected string
		if err := conn.QueryRowContext(ctx, `SELECT COALESCE(collected_at, '') FROM meta_ads WHERE ad_archive_id = ?`, ad.AdArchiveID).Scan(&existingCollected); err == nil {
			if existingTime, ok := parseStoredTimestamp(existingCollected); ok {
				if incomingTime, ok2 := parseStoredTimestamp(ad.CollectedAt); ok2 && existingTime.After(incomingTime) {
					continue
				}
			}
		}

		platJSON, _ := json.Marshal(ad.PublisherPlatform)
		rawJSON, _ := json.Marshal(ad)

		activeInt := 0
		if ad.IsActive {
			activeInt = 1
		}
		if _, err := stmt.ExecContext(ctx,
			ad.AdArchiveID, ad.CollationID, ad.CollationCount, ad.PageID, ad.PageName,
			activeInt, ad.StartDate, ad.EndDate, ad.Country, ad.AdType,
			string(platJSON), ad.DisplayFormat, ad.BodyText, ad.LinkTitle,
			ad.LinkCaption, ad.LinkDescription, ad.CTAText, ad.CTAType,
			ad.LandingURL, ad.LandingDomain, ad.Byline, ad.Currency,
			nullableInt64(ad.SpendLower), nullableInt64(ad.SpendUpper),
			nullableInt64(ad.ImpressionsLower), nullableInt64(ad.ImpressionsUpper),
			string(rawJSON), ad.CollectedAt,
		); err != nil {
			_ = stmt.Close()
			return fmt.Errorf("upsert meta_ad %s: %w", ad.AdArchiveID, err)
		}
		if err := upsertFrameworkResourceTx(ctx, conn, "ads", ad.AdArchiveID, rawJSON, adSearchableText(ad, rawJSON), ad.CollectedAt); err != nil {
			_ = stmt.Close()
			return fmt.Errorf("mirror ad %s into framework store: %w", ad.AdArchiveID, err)
		}
		written++
	}
	_ = stmt.Close()
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("commit meta_ads tx: %w", err)
	}
	committed = true

	totalCount, countErr := s.Count("ads")
	if countErr != nil {
		totalCount = written
	}
	if err := s.SaveSyncState("ads", "", totalCount); err != nil {
		return fmt.Errorf("save sync state for ads: %w", err)
	}
	return nil
}

// SaveAdvertisers persists Advertiser records into meta_advertisers and the
// framework store.
func SaveAdvertisers(ctx context.Context, s *store.Store, advertisers []Advertiser) error {
	if len(advertisers) == 0 {
		return nil
	}
	if err := EnsureSchema(ctx, s); err != nil {
		return err
	}
	nowStr := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO meta_advertisers (
			page_id, page_name, category, likes, verification, ig_username, ig_followers, country, raw_json, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(page_id) DO UPDATE SET
			page_name=COALESCE(NULLIF(excluded.page_name, ''), meta_advertisers.page_name),
			category=COALESCE(NULLIF(excluded.category, ''), meta_advertisers.category),
			likes=CASE WHEN excluded.likes > 0 THEN excluded.likes ELSE meta_advertisers.likes END,
			verification=COALESCE(NULLIF(excluded.verification, ''), meta_advertisers.verification),
			ig_username=COALESCE(NULLIF(excluded.ig_username, ''), meta_advertisers.ig_username),
			ig_followers=CASE WHEN excluded.ig_followers > 0 THEN excluded.ig_followers ELSE meta_advertisers.ig_followers END,
			country=COALESCE(NULLIF(excluded.country, ''), meta_advertisers.country),
			raw_json=excluded.raw_json,
			updated_at=excluded.updated_at
	`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	toMirror := make([]mirrorRow, 0, len(advertisers))
	for _, adv := range advertisers {
		if adv.PageID == "" {
			continue
		}
		rawJSON, _ := json.Marshal(adv)
		toMirror = append(toMirror, mirrorRow{id: adv.PageID, raw: rawJSON})
		if _, err := stmt.ExecContext(ctx,
			adv.PageID, adv.PageName, adv.Category, adv.Likes, adv.Verification,
			adv.IGUsername, adv.IGFollowers, adv.Country, string(rawJSON), nowStr,
		); err != nil {
			_ = stmt.Close()
			_ = tx.Rollback()
			return err
		}
	}
	_ = stmt.Close()
	if err := tx.Commit(); err != nil {
		return err
	}
	for _, item := range toMirror {
		if err := s.Upsert("advertisers", item.id, item.raw); err != nil {
			return err
		}
	}
	return nil
}

// SaveAdDetails persists AdDetails into meta_ad_details and the framework store.
func SaveAdDetails(ctx context.Context, s *store.Store, det AdDetails) error {
	if det.AdArchiveID == "" {
		return nil
	}
	if err := EnsureSchema(ctx, s); err != nil {
		return err
	}
	if det.FetchedAt == "" {
		det.FetchedAt = time.Now().UTC().Format(time.RFC3339)
	}
	rawJSON, _ := json.Marshal(det)
	violInt := 0
	if det.IsViolatingEUSIEP {
		violInt = 1
	}
	aiInt := 0
	if det.AIDisclosureElig {
		aiInt = 1
	}
	_, err := s.DB().ExecContext(ctx, `
		INSERT INTO meta_ad_details (
			ad_archive_id, page_id, page_name, country, byline,
			eu_total_reach, uk_total_reach, br_total_reach, target_gender,
			is_violating_eu_siep, ai_disclosure_eligible, raw_json, fetched_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(ad_archive_id) DO UPDATE SET
			page_id=excluded.page_id,
			page_name=COALESCE(NULLIF(excluded.page_name, ''), meta_ad_details.page_name),
			country=excluded.country,
			byline=COALESCE(NULLIF(excluded.byline, ''), meta_ad_details.byline),
			eu_total_reach=excluded.eu_total_reach,
			uk_total_reach=excluded.uk_total_reach,
			br_total_reach=excluded.br_total_reach,
			target_gender=excluded.target_gender,
			is_violating_eu_siep=excluded.is_violating_eu_siep,
			ai_disclosure_eligible=excluded.ai_disclosure_eligible,
			raw_json=excluded.raw_json,
			fetched_at=excluded.fetched_at
	`, det.AdArchiveID, det.PageID, det.PageName, det.Country, det.Byline,
		det.EUTotalReach, det.UKTotalReach, det.BRTotalReach, det.TargetGender,
		violInt, aiInt, string(rawJSON), det.FetchedAt)
	if err != nil {
		return err
	}
	return s.Upsert("ad_details", det.AdArchiveID, rawJSON)
}

// RecordSnapshot writes a point-in-time snapshot of an advertiser's ad set for
// temporal diffing. Pass isComplete=true only when the upstream scan fetched
// every available active ad without hitting --limit or --max-pages truncation.
func RecordSnapshot(ctx context.Context, s *store.Store, pageID, pageName, country, queryStr string, ads []Ad, isComplete bool, capturedAt time.Time) (int64, error) {
	if strings.TrimSpace(pageID) == "" {
		return 0, nil
	}
	if err := EnsureSchema(ctx, s); err != nil {
		return 0, err
	}
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	}
	normCountry := normalizeCountry(country)
	completeInt := 0
	if isComplete {
		completeInt = 1
	}
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO meta_snapshots (page_id, page_name, country, query_string, captured_at, ad_count, is_complete)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, pageID, pageName, normCountry, queryStr, capturedAt.UTC().Format(time.RFC3339Nano), len(ads), completeInt)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	snapID, err := res.LastInsertId()
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO meta_snapshot_ads (snapshot_id, ad_archive_id, collation_count, is_active)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	for _, ad := range ads {
		if ad.AdArchiveID == "" {
			continue
		}
		act := 0
		if ad.IsActive {
			act = 1
		}
		if _, err := stmt.ExecContext(ctx, snapID, ad.AdArchiveID, ad.CollationCount, act); err != nil {
			_ = stmt.Close()
			_ = tx.Rollback()
			return 0, err
		}
	}
	_ = stmt.Close()
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return snapID, nil
}

// ReconcilePageActiveAds marks previously active ads for (pageID, country) that
// are absent from a complete fresh active page scan as inactive across both
// meta_ads and the framework resources/FTS5 store atomically inside a single
// BEGIN IMMEDIATE write transaction.
//
// Concurrency safety: the candidate read, conditional meta_ads UPDATE, and
// framework resources/FTS5 mirror all execute inside one BEGIN IMMEDIATE write
// transaction so no concurrent SaveAdsBatch can interleave between meta_ads and
// resources, and rows refreshed after scanTime are skipped.
func ReconcilePageActiveAds(ctx context.Context, s *store.Store, pageID, country string, activeAds []Ad, scanTime time.Time) (int, error) {
	pageID = strings.TrimSpace(pageID)
	if pageID == "" {
		return 0, nil
	}
	if err := EnsureSchema(ctx, s); err != nil {
		return 0, err
	}
	if scanTime.IsZero() {
		scanTime = time.Now().UTC()
	} else {
		scanTime = scanTime.UTC()
	}
	normCountry := normalizeCountry(country)
	activeSet := make(map[string]bool, len(activeAds))
	for _, ad := range activeAds {
		if ad.AdArchiveID != "" {
			activeSet[ad.AdArchiveID] = true
		}
	}

	conn, err := s.DB().Conn(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()

	var rows *sql.Rows
	if normCountry == "ALL" {
		rows, err = conn.QueryContext(ctx, `
			SELECT ad_archive_id, raw_json, COALESCE(collected_at, '')
			FROM meta_ads
			WHERE page_id = ? AND is_active = 1
		`, pageID)
	} else {
		rows, err = conn.QueryContext(ctx, `
			SELECT ad_archive_id, raw_json, COALESCE(collected_at, '')
			FROM meta_ads
			WHERE page_id = ? AND is_active = 1 AND UPPER(COALESCE(country, '')) = UPPER(?)
		`, pageID, normCountry)
	}
	if err != nil {
		return 0, err
	}
	type staleRow struct {
		id          string
		raw         string
		collectedAt string
	}
	var stale []staleRow
	for rows.Next() {
		var sr staleRow
		if err := rows.Scan(&sr.id, &sr.raw, &sr.collectedAt); err != nil {
			_ = rows.Close()
			return 0, err
		}
		if activeSet[sr.id] {
			continue
		}
		if rowTime, ok := parseStoredTimestamp(sr.collectedAt); ok && rowTime.After(scanTime) {
			// Observed active by another command after this scan started; do not overwrite.
			continue
		}
		stale = append(stale, sr)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	_ = rows.Close()

	if len(stale) == 0 {
		if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
			return 0, err
		}
		committed = true
		return 0, nil
	}

	scanStr := scanTime.Format(utcTimestampLayout)
	var stmt *sql.Stmt
	if normCountry == "ALL" {
		stmt, err = conn.PrepareContext(ctx, `
			UPDATE meta_ads
			SET is_active = 0,
			    end_date = COALESCE(NULLIF(end_date, ''), ?),
			    raw_json = ?,
			    collected_at = ?
			WHERE ad_archive_id = ?
			  AND page_id = ?
			  AND is_active = 1
			  AND COALESCE(collected_at, '') = ?
			  AND raw_json = ?
		`)
	} else {
		stmt, err = conn.PrepareContext(ctx, `
			UPDATE meta_ads
			SET is_active = 0,
			    end_date = COALESCE(NULLIF(end_date, ''), ?),
			    raw_json = ?,
			    collected_at = ?
			WHERE ad_archive_id = ?
			  AND page_id = ?
			  AND is_active = 1
			  AND UPPER(COALESCE(country, '')) = UPPER(?)
			  AND COALESCE(collected_at, '') = ?
			  AND raw_json = ?
		`)
	}
	if err != nil {
		return 0, err
	}

	updated := 0
	for _, sr := range stale {
		var ad Ad
		updatedRaw := []byte(sr.raw)
		if err := json.Unmarshal([]byte(sr.raw), &ad); err == nil {
			ad.IsActive = false
			if ad.EndDate == "" {
				ad.EndDate = scanStr
			}
			ad.DaysRunning = ComputeDaysRunning(ad.StartDate, ad.EndDate, false, scanTime)
			ad.CollectedAt = scanStr
			if marshaled, mErr := json.Marshal(ad); mErr == nil {
				updatedRaw = marshaled
			}
		}
		var res sql.Result
		if normCountry == "ALL" {
			res, err = stmt.ExecContext(ctx, scanStr, string(updatedRaw), scanStr, sr.id, pageID, sr.collectedAt, sr.raw)
		} else {
			res, err = stmt.ExecContext(ctx, scanStr, string(updatedRaw), scanStr, sr.id, pageID, normCountry, sr.collectedAt, sr.raw)
		}
		if err != nil {
			_ = stmt.Close()
			return 0, err
		}
		affected, affErr := res.RowsAffected()
		if affErr == nil && affected > 0 {
			if err := upsertFrameworkResourceTx(ctx, conn, "ads", sr.id, updatedRaw, adSearchableText(ad, updatedRaw), scanStr); err != nil {
				_ = stmt.Close()
				return 0, fmt.Errorf("mirror inactive ad %s into framework store: %w", sr.id, err)
			}
			updated++
		}
	}
	_ = stmt.Close()
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return 0, err
	}
	committed = true
	return updated, nil
}

// ResolvePageID maps either a numeric Facebook Page ID or a case-insensitive
// brand/page name against local SQLite tables (safe on read-only handles).
func ResolvePageID(ctx context.Context, s *store.Store, pageOrName string) (pageID string, pageName string, found bool, err error) {
	target := strings.TrimSpace(pageOrName)
	if target == "" {
		return "", "", false, nil
	}
	hasAds, err := tableExists(ctx, s, "meta_ads")
	if err != nil {
		return target, "", false, err
	}
	var pid, pname string
	if hasAds {
		// Exact page_id match in meta_ads first.
		err = s.DB().QueryRowContext(ctx, `SELECT page_id, page_name FROM meta_ads WHERE page_id = ? LIMIT 1`, target).Scan(&pid, &pname)
		if err == nil {
			return pid, pname, true, nil
		}
		if err != sql.ErrNoRows {
			return target, "", false, err
		}
		// Exact or substring case-insensitive name match in meta_ads.
		err = s.DB().QueryRowContext(ctx, `SELECT page_id, page_name FROM meta_ads WHERE LOWER(page_name) = LOWER(?) LIMIT 1`, target).Scan(&pid, &pname)
		if err == nil {
			return pid, pname, true, nil
		}
		if err != sql.ErrNoRows {
			return target, "", false, err
		}
		err = s.DB().QueryRowContext(ctx, `SELECT page_id, page_name FROM meta_ads WHERE LOWER(page_name) LIKE LOWER(?) LIMIT 1`, "%"+target+"%").Scan(&pid, &pname)
		if err == nil {
			return pid, pname, true, nil
		}
		if err != sql.ErrNoRows {
			return target, "", false, err
		}
	}
	// Fallback to meta_advertisers.
	hasAdvs, err := tableExists(ctx, s, "meta_advertisers")
	if err != nil {
		return target, "", false, err
	}
	if !hasAdvs {
		return target, "", false, nil
	}
	err = s.DB().QueryRowContext(ctx, `SELECT page_id, page_name FROM meta_advertisers WHERE page_id = ? OR LOWER(page_name) LIKE LOWER(?) LIMIT 1`, target, "%"+target+"%").Scan(&pid, &pname)
	if err == nil {
		return pid, pname, true, nil
	}
	if err == sql.ErrNoRows {
		return target, "", false, nil
	}
	return target, "", false, err
}

// LoadAds reads normalized Ad records from meta_ads (optionally scoped by
// page_id or case-insensitive page_name). Uses drain-first pattern cleanly and
// is safe on read-only handles when meta_ads does not exist yet.
func LoadAds(ctx context.Context, s *store.Store, pageFilter string, activeOnly bool) ([]Ad, error) {
	hasAds, err := tableExists(ctx, s, "meta_ads")
	if err != nil {
		return nil, err
	}
	if !hasAds {
		return make([]Ad, 0), nil
	}
	query := `SELECT raw_json FROM meta_ads WHERE 1=1`
	var args []any
	if pf := strings.TrimSpace(pageFilter); pf != "" {
		pid, _, found, err := ResolvePageID(ctx, s, pf)
		if err != nil {
			return nil, err
		}
		if found {
			query += ` AND page_id = ?`
			args = append(args, pid)
		} else {
			query += ` AND (page_id = ? OR LOWER(page_name) LIKE LOWER(?))`
			args = append(args, pf, "%"+pf+"%")
		}
	}
	if activeOnly {
		query += ` AND is_active = 1`
	}
	query += ` ORDER BY collected_at DESC`

	rows, err := s.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	var rawList []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			_ = rows.Close()
			return nil, err
		}
		rawList = append(rawList, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()

	now := time.Now().UTC()
	out := make([]Ad, 0, len(rawList))
	for _, r := range rawList {
		var ad Ad
		if err := json.Unmarshal([]byte(r), &ad); err == nil {
			ad.DaysRunning = ComputeDaysRunning(ad.StartDate, ad.EndDate, ad.IsActive, now)
			out = append(out, ad)
		}
	}
	return out, nil
}

// --- Analytics Result Types & Computations ---

// WinnerRow represents one ranked evergreen/high-collation creative.
type WinnerRow struct {
	Rank              int      `json:"rank"`
	AdArchiveID       string   `json:"ad_archive_id"`
	PageID            string   `json:"page_id"`
	PageName          string   `json:"page_name"`
	DaysRunning       int      `json:"days_running"`
	CollationCount    int      `json:"collation_count"`
	PlatformCount     int      `json:"platform_count"`
	PublisherPlatform []string `json:"publisher_platform"`
	WinnerScore       float64  `json:"winner_score"`
	DisplayFormat     string   `json:"display_format"`
	StartDate         string   `json:"start_date"`
	CTAText           string   `json:"cta_text,omitempty"`
	LandingDomain     string   `json:"landing_domain,omitempty"`
	OpeningHook       string   `json:"opening_hook,omitempty"`
	AdLibraryURL      string   `json:"ad_library_url,omitempty"`
}

// ComputeWinners ranks active local ads by longevity, collation scale, and
// platform breadth.
func ComputeWinners(ctx context.Context, s *store.Store, pageFilter string, minDays, minCollation, limit int) ([]WinnerRow, error) {
	ads, err := LoadAds(ctx, s, pageFilter, true)
	if err != nil {
		return nil, err
	}
	rows := make([]WinnerRow, 0, len(ads))
	for _, ad := range ads {
		if minDays > 0 && ad.DaysRunning < minDays {
			continue
		}
		if minCollation > 0 && ad.CollationCount < minCollation {
			continue
		}
		platCount := len(ad.PublisherPlatform)
		if platCount == 0 {
			platCount = 1
		}
		// Composite mechanical score: days running weighted by collation scale & platform breadth.
		score := float64(ad.DaysRunning) + float64(ad.CollationCount)*10.0 + float64(platCount)*2.0
		hook, _ := ExtractOpeningHook(ad.BodyText)
		rows = append(rows, WinnerRow{
			AdArchiveID:       ad.AdArchiveID,
			PageID:            ad.PageID,
			PageName:          ad.PageName,
			DaysRunning:       ad.DaysRunning,
			CollationCount:    ad.CollationCount,
			PlatformCount:     platCount,
			PublisherPlatform: ad.PublisherPlatform,
			WinnerScore:       score,
			DisplayFormat:     ad.DisplayFormat,
			StartDate:         ad.StartDate,
			CTAText:           ad.CTAText,
			LandingDomain:     ad.LandingDomain,
			OpeningHook:       hook,
			AdLibraryURL:      ad.AdLibraryURL,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].WinnerScore != rows[j].WinnerScore {
			return rows[i].WinnerScore > rows[j].WinnerScore
		}
		if rows[i].DaysRunning != rows[j].DaysRunning {
			return rows[i].DaysRunning > rows[j].DaysRunning
		}
		return rows[i].AdArchiveID < rows[j].AdArchiveID
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	for i := range rows {
		rows[i].Rank = i + 1
	}
	return rows, nil
}

// DiffScaledAd records an ad whose collation count changed between snapshots.
type DiffScaledAd struct {
	AdArchiveID   string `json:"ad_archive_id"`
	PageName      string `json:"page_name,omitempty"`
	PreviousCount int    `json:"previous_collation_count"`
	CurrentCount  int    `json:"current_collation_count"`
	Delta         int    `json:"delta"`
	OpeningHook   string `json:"opening_hook,omitempty"`
	AdLibraryURL  string `json:"ad_library_url,omitempty"`
}

// DiffAdBrief summarizes a launched or stopped ad in a snapshot diff.
type DiffAdBrief struct {
	AdArchiveID    string `json:"ad_archive_id"`
	PageName       string `json:"page_name,omitempty"`
	CollationCount int    `json:"collation_count"`
	DisplayFormat  string `json:"display_format,omitempty"`
	CTAText        string `json:"cta_text,omitempty"`
	LandingDomain  string `json:"landing_domain,omitempty"`
	OpeningHook    string `json:"opening_hook,omitempty"`
	StartDate      string `json:"start_date,omitempty"`
	AdLibraryURL   string `json:"ad_library_url,omitempty"`
}

// DiffReport compares the two most recent same-country snapshots for a Facebook Page.
type DiffReport struct {
	PageID           string         `json:"page_id"`
	PageName         string         `json:"page_name,omitempty"`
	Country          string         `json:"country,omitempty"`
	PreviousSnapshot string         `json:"previous_snapshot_at,omitempty"`
	CurrentSnapshot  string         `json:"current_snapshot_at,omitempty"`
	PreviousComplete bool           `json:"previous_complete"`
	CurrentComplete  bool           `json:"current_complete"`
	PartialCapture   bool           `json:"partial_capture,omitempty"`
	SnapshotCount    int            `json:"snapshot_count"`
	PreviousActive   int            `json:"previous_active_count"`
	CurrentActive    int            `json:"current_active_count"`
	NetChange        int            `json:"net_change"`
	Launched         []DiffAdBrief  `json:"launched"`
	Stopped          []DiffAdBrief  `json:"stopped"`
	Scaled           []DiffScaledAd `json:"scaled"`
	Note             string         `json:"note,omitempty"`
}

// ComputeDiff compares the two latest same-country snapshots for a page in
// local SQLite. When country is empty, it scopes to the country of the page's
// most recent snapshot so captures from different countries are never mixed.
// Returns honest empty slices (no fabricated drift) when fewer than 2 matching
// snapshots exist or when either compared snapshot was a partial capture.
func ComputeDiff(ctx context.Context, s *store.Store, pageFilter, country string) (DiffReport, error) {
	pid, pname, _, err := ResolvePageID(ctx, s, pageFilter)
	if err != nil {
		return DiffReport{}, err
	}
	targetCountry := strings.ToUpper(strings.TrimSpace(country))
	report := DiffReport{
		PageID:   pid,
		PageName: pname,
		Country:  targetCountry,
		Launched: make([]DiffAdBrief, 0),
		Stopped:  make([]DiffAdBrief, 0),
		Scaled:   make([]DiffScaledAd, 0),
	}

	hasSnaps, err := tableExists(ctx, s, "meta_snapshots")
	if err != nil {
		return report, err
	}
	if !hasSnaps {
		report.Note = "no snapshots recorded yet for this page; run 'meta-ads-library-pp-cli ads search --page-id " + pid + "' or pass --refresh"
		return report, nil
	}

	if targetCountry == "" {
		var latestCountry string
		err := s.DB().QueryRowContext(ctx, `
			SELECT COALESCE(country, '')
			FROM meta_snapshots
			WHERE page_id = ?
			ORDER BY captured_at DESC, id DESC
			LIMIT 1
		`, pid).Scan(&latestCountry)
		if err != nil && err != sql.ErrNoRows {
			return report, err
		}
		targetCountry = strings.ToUpper(strings.TrimSpace(latestCountry))
		report.Country = targetCountry
	}

	hasIsComplete, err := columnExists(ctx, s, "meta_snapshots", "is_complete")
	if err != nil {
		return report, err
	}

	type snapMeta struct {
		id         int64
		pageName   string
		country    string
		capturedAt string
		isComplete bool
	}

	var rows *sql.Rows
	if hasIsComplete {
		rows, err = s.DB().QueryContext(ctx, `
			SELECT id, COALESCE(page_name, ''), COALESCE(country, ''), captured_at, COALESCE(is_complete, 1)
			FROM meta_snapshots
			WHERE page_id = ? AND UPPER(COALESCE(country, '')) = ?
			ORDER BY captured_at DESC, id DESC
			LIMIT 2
		`, pid, targetCountry)
	} else {
		rows, err = s.DB().QueryContext(ctx, `
			SELECT id, COALESCE(page_name, ''), COALESCE(country, ''), captured_at, 1
			FROM meta_snapshots
			WHERE page_id = ? AND UPPER(COALESCE(country, '')) = ?
			ORDER BY captured_at DESC, id DESC
			LIMIT 2
		`, pid, targetCountry)
	}
	if err != nil {
		return report, err
	}
	var snaps []snapMeta
	for rows.Next() {
		var sm snapMeta
		var completeInt int
		if err := rows.Scan(&sm.id, &sm.pageName, &sm.country, &sm.capturedAt, &completeInt); err != nil {
			_ = rows.Close()
			return report, err
		}
		sm.isComplete = completeInt != 0
		snaps = append(snaps, sm)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return report, err
	}
	_ = rows.Close()

	report.SnapshotCount = len(snaps)
	if len(snaps) == 0 {
		if targetCountry != "" {
			report.Note = "no snapshots recorded yet for this page in country " + targetCountry + "; run 'meta-ads-library-pp-cli ads search --page-id " + pid + " --country " + targetCountry + "' or pass --refresh --country " + targetCountry
		} else {
			report.Note = "no snapshots recorded yet for this page; run 'meta-ads-library-pp-cli ads search --page-id " + pid + "' or pass --refresh"
		}
		return report, nil
	}
	if report.PageName == "" {
		report.PageName = snaps[0].pageName
	}
	if report.Country == "" {
		report.Country = strings.ToUpper(strings.TrimSpace(snaps[0].country))
	}
	report.CurrentSnapshot = snaps[0].capturedAt
	report.CurrentComplete = snaps[0].isComplete
	if len(snaps) < 2 {
		report.PartialCapture = !report.CurrentComplete
		if report.PartialCapture {
			report.Note = "only 1 partial snapshot recorded so far (truncated by --limit or --max-pages); re-run with higher --limit/--max-pages to establish a complete baseline"
		} else {
			report.Note = "only 1 snapshot recorded so far; run again later with --refresh to detect launched, stopped, and scaled ads"
		}
		curAds, err := loadSnapshotAdMap(ctx, s, snaps[0].id)
		if err != nil {
			return report, err
		}
		report.CurrentActive = len(curAds)
		return report, nil
	}

	report.PreviousSnapshot = snaps[1].capturedAt
	report.PreviousComplete = snaps[1].isComplete
	report.PartialCapture = !report.CurrentComplete || !report.PreviousComplete

	curMap, err := loadSnapshotAdMap(ctx, s, snaps[0].id)
	if err != nil {
		return report, err
	}
	prevMap, err := loadSnapshotAdMap(ctx, s, snaps[1].id)
	if err != nil {
		return report, err
	}

	report.PreviousActive = len(prevMap)
	report.CurrentActive = len(curMap)
	if !report.PartialCapture {
		report.NetChange = len(curMap) - len(prevMap)
	}

	// Load ad details map for enrichment (drain-first).
	adsList, err := LoadAds(ctx, s, pid, false)
	if err != nil {
		return report, err
	}
	adByID := make(map[string]Ad, len(adsList))
	for _, ad := range adsList {
		adByID[ad.AdArchiveID] = ad
	}

	toBrief := func(id string, coll int) DiffAdBrief {
		if ad, ok := adByID[id]; ok {
			hook, _ := ExtractOpeningHook(ad.BodyText)
			return DiffAdBrief{
				AdArchiveID:    id,
				PageName:       ad.PageName,
				CollationCount: coll,
				DisplayFormat:  ad.DisplayFormat,
				CTAText:        ad.CTAText,
				LandingDomain:  ad.LandingDomain,
				OpeningHook:    hook,
				StartDate:      ad.StartDate,
				AdLibraryURL:   ad.AdLibraryURL,
			}
		}
		return DiffAdBrief{AdArchiveID: id, CollationCount: coll}
	}

	for id, curColl := range curMap {
		prevColl, existed := prevMap[id]
		if !existed {
			if !report.PartialCapture {
				report.Launched = append(report.Launched, toBrief(id, curColl))
			}
		} else if curColl != prevColl {
			hook := ""
			urlStr := ""
			pn := report.PageName
			if ad, ok := adByID[id]; ok {
				hook, _ = ExtractOpeningHook(ad.BodyText)
				urlStr = ad.AdLibraryURL
				pn = ad.PageName
			}
			report.Scaled = append(report.Scaled, DiffScaledAd{
				AdArchiveID:   id,
				PageName:      pn,
				PreviousCount: prevColl,
				CurrentCount:  curColl,
				Delta:         curColl - prevColl,
				OpeningHook:   hook,
				AdLibraryURL:  urlStr,
			})
		}
	}
	if !report.PartialCapture {
		for id, prevColl := range prevMap {
			if _, still := curMap[id]; !still {
				report.Stopped = append(report.Stopped, toBrief(id, prevColl))
			}
		}
	} else {
		report.Note = "at least one compared snapshot was truncated by --limit or --max-pages (partial capture); launched and stopped lists are omitted to avoid false positives. Re-run with higher --limit/--max-pages to capture complete snapshots."
	}

	sort.Slice(report.Launched, func(i, j int) bool { return report.Launched[i].AdArchiveID < report.Launched[j].AdArchiveID })
	sort.Slice(report.Stopped, func(i, j int) bool { return report.Stopped[i].AdArchiveID < report.Stopped[j].AdArchiveID })
	sort.Slice(report.Scaled, func(i, j int) bool { return report.Scaled[i].Delta > report.Scaled[j].Delta })

	return report, nil
}

func loadSnapshotAdMap(ctx context.Context, s *store.Store, snapID int64) (map[string]int, error) {
	rows, err := s.DB().QueryContext(ctx, `SELECT ad_archive_id, collation_count FROM meta_snapshot_ads WHERE snapshot_id = ?`, snapID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]int)
	for rows.Next() {
		var id string
		var coll int
		if err := rows.Scan(&id, &coll); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out[id] = coll
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()
	return out, nil
}

// CompetitorBenchmark holds side-by-side metrics for one advertiser in compare.
type CompetitorBenchmark struct {
	Input             string         `json:"input"`
	PageID            string         `json:"page_id"`
	PageName          string         `json:"page_name"`
	ActiveAds         int            `json:"active_ads"`
	TotalCollations   int            `json:"total_collations"`
	HighCollationPct  float64        `json:"high_collation_pct"`
	MedianDaysRunning int            `json:"median_days_running"`
	MaxDaysRunning    int            `json:"max_days_running"`
	LaunchedLast7d    int            `json:"launched_last_7d"`
	LaunchedLast30d   int            `json:"launched_last_30d"`
	UniqueDomains     int            `json:"unique_landing_domains"`
	FormatMix         map[string]int `json:"format_mix"`
	PlatformMix       map[string]int `json:"platform_mix"`
	TopCTAs           []string       `json:"top_ctas"`
}

// ComputeCompare benchmarks 2..N advertisers side by side from local SQLite.
func ComputeCompare(ctx context.Context, s *store.Store, targets []string) ([]CompetitorBenchmark, error) {
	out := make([]CompetitorBenchmark, 0, len(targets))
	now := time.Now().UTC()
	for _, tgt := range targets {
		pid, pname, _, err := ResolvePageID(ctx, s, tgt)
		if err != nil {
			return nil, err
		}
		ads, err := LoadAds(ctx, s, pid, true)
		if err != nil {
			return nil, err
		}
		bench := CompetitorBenchmark{
			Input:       tgt,
			PageID:      pid,
			PageName:    pname,
			FormatMix:   make(map[string]int),
			PlatformMix: make(map[string]int),
			TopCTAs:     make([]string, 0),
		}
		if len(ads) == 0 {
			out = append(out, bench)
			continue
		}
		if bench.PageName == "" {
			bench.PageName = ads[0].PageName
		}
		bench.ActiveAds = len(ads)

		ages := make([]int, 0, len(ads))
		domains := make(map[string]bool)
		ctaCounts := make(map[string]int)
		highColl := 0
		for _, ad := range ads {
			bench.TotalCollations += ad.CollationCount
			if ad.CollationCount >= 3 {
				highColl++
			}
			ages = append(ages, ad.DaysRunning)
			if ad.DaysRunning > bench.MaxDaysRunning {
				bench.MaxDaysRunning = ad.DaysRunning
			}
			if st, ok := parseFlexibleTime(ad.StartDate); ok {
				age := now.Sub(st)
				if age <= 7*24*time.Hour {
					bench.LaunchedLast7d++
				}
				if age <= 30*24*time.Hour {
					bench.LaunchedLast30d++
				}
			}
			if ad.LandingDomain != "" {
				domains[ad.LandingDomain] = true
			}
			fmtKey := ad.DisplayFormat
			if fmtKey == "" {
				fmtKey = "UNKNOWN"
			}
			bench.FormatMix[fmtKey]++
			for _, p := range ad.PublisherPlatform {
				bench.PlatformMix[p]++
			}
			if ad.CTAText != "" {
				ctaCounts[ad.CTAText]++
			}
		}
		bench.UniqueDomains = len(domains)
		bench.HighCollationPct = round1(float64(highColl) * 100.0 / float64(len(ads)))
		bench.MedianDaysRunning = medianInt(ages)
		bench.TopCTAs = topKeysByCount(ctaCounts, 3)
		out = append(out, bench)
	}
	return out, nil
}

// HookRow represents a deduplicated opening copy hook ranked by scale and longevity.
type HookRow struct {
	Rank            int      `json:"rank"`
	OpeningHook     string   `json:"opening_hook"`
	AdCount         int      `json:"ad_count"`
	TotalCollations int      `json:"total_collations"`
	MaxDaysRunning  int      `json:"max_days_running"`
	TopHeadline     string   `json:"top_headline,omitempty"`
	TopCTA          string   `json:"top_cta,omitempty"`
	Pages           []string `json:"pages"`
	SampleAdID      string   `json:"sample_ad_id"`
}

// ComputeHooks mechanically extracts, deduplicates, and ranks opening copy
// hooks weighted by collation scale and longevity.
func ComputeHooks(ctx context.Context, s *store.Store, pageFilter string, limit int) ([]HookRow, error) {
	ads, err := LoadAds(ctx, s, pageFilter, true)
	if err != nil {
		return nil, err
	}
	type acc struct {
		display    string
		adCount    int
		collations int
		maxDays    int
		headlines  map[string]int
		ctas       map[string]int
		pages      map[string]bool
		sampleID   string
	}
	groups := make(map[string]*acc)
	for _, ad := range ads {
		display, key := ExtractOpeningHook(ad.BodyText)
		if key == "" {
			continue
		}
		g, ok := groups[key]
		if !ok {
			g = &acc{
				display:   display,
				headlines: make(map[string]int),
				ctas:      make(map[string]int),
				pages:     make(map[string]bool),
				sampleID:  ad.AdArchiveID,
			}
			groups[key] = g
		}
		g.adCount++
		g.collations += ad.CollationCount
		if ad.DaysRunning > g.maxDays {
			g.maxDays = ad.DaysRunning
		}
		if ad.LinkTitle != "" {
			g.headlines[ad.LinkTitle]++
		}
		if ad.CTAText != "" {
			g.ctas[ad.CTAText]++
		}
		if ad.PageName != "" {
			g.pages[ad.PageName] = true
		}
	}

	rows := make([]HookRow, 0, len(groups))
	for _, g := range groups {
		pages := make([]string, 0, len(g.pages))
		for p := range g.pages {
			pages = append(pages, p)
		}
		sort.Strings(pages)
		rows = append(rows, HookRow{
			OpeningHook:     g.display,
			AdCount:         g.adCount,
			TotalCollations: g.collations,
			MaxDaysRunning:  g.maxDays,
			TopHeadline:     firstOrEmpty(topKeysByCount(g.headlines, 1)),
			TopCTA:          firstOrEmpty(topKeysByCount(g.ctas, 1)),
			Pages:           pages,
			SampleAdID:      g.sampleID,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TotalCollations != rows[j].TotalCollations {
			return rows[i].TotalCollations > rows[j].TotalCollations
		}
		if rows[i].MaxDaysRunning != rows[j].MaxDaysRunning {
			return rows[i].MaxDaysRunning > rows[j].MaxDaysRunning
		}
		return rows[i].OpeningHook < rows[j].OpeningHook
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	for i := range rows {
		rows[i].Rank = i + 1
	}
	return rows, nil
}

// FunnelRow aggregates active ads targeting the same unwrapped landing domain + path.
type FunnelRow struct {
	Rank            int                 `json:"rank"`
	LandingDomain   string              `json:"landing_domain"`
	Path            string              `json:"path"`
	ActiveAds       int                 `json:"active_ads"`
	TotalCollations int                 `json:"total_collations"`
	MaxDaysRunning  int                 `json:"max_days_running"`
	TopCTAs         []string            `json:"top_ctas"`
	UTMValues       map[string][]string `json:"utm_values,omitempty"`
	SampleURL       string              `json:"sample_url"`
	SampleAdID      string              `json:"sample_ad_id"`
}

// ComputeFunnels groups active ads (including carousel card destinations) by
// unwrapped landing domain, normalized path, and UTM keys.
func ComputeFunnels(ctx context.Context, s *store.Store, pageFilter string, limit int) ([]FunnelRow, error) {
	ads, err := LoadAds(ctx, s, pageFilter, true)
	if err != nil {
		return nil, err
	}
	type acc struct {
		domain     string
		path       string
		adIDs      map[string]bool
		collations int
		maxDays    int
		ctas       map[string]int
		utms       map[string]map[string]bool
		sampleURL  string
		sampleAdID string
	}
	groups := make(map[string]*acc)
	addLink := func(ad Ad, rawURL string) {
		clean := UnwrapRedirectURL(rawURL)
		dom := ExtractDomain(clean)
		if dom == "" {
			return
		}
		path := NormalizeURLPath(clean)
		key := dom + "|" + path
		g, ok := groups[key]
		if !ok {
			g = &acc{
				domain:     dom,
				path:       path,
				adIDs:      make(map[string]bool),
				ctas:       make(map[string]int),
				utms:       make(map[string]map[string]bool),
				sampleURL:  clean,
				sampleAdID: ad.AdArchiveID,
			}
			groups[key] = g
		}
		if !g.adIDs[ad.AdArchiveID] {
			g.adIDs[ad.AdArchiveID] = true
			g.collations += ad.CollationCount
		}
		if ad.DaysRunning > g.maxDays {
			g.maxDays = ad.DaysRunning
		}
		if ad.CTAText != "" {
			g.ctas[ad.CTAText]++
		}
		for k, v := range ExtractUTMParams(clean) {
			if g.utms[k] == nil {
				g.utms[k] = make(map[string]bool)
			}
			g.utms[k][v] = true
		}
	}

	for _, ad := range ads {
		if ad.LandingURL != "" {
			addLink(ad, ad.LandingURL)
		}
		for _, card := range ad.Cards {
			if card.LandingURL != "" {
				addLink(ad, card.LandingURL)
			} else if card.LinkURL != "" {
				addLink(ad, card.LinkURL)
			}
		}
	}

	rows := make([]FunnelRow, 0, len(groups))
	for _, g := range groups {
		utmOut := make(map[string][]string)
		for k, set := range g.utms {
			vals := make([]string, 0, len(set))
			for v := range set {
				vals = append(vals, v)
			}
			sort.Strings(vals)
			utmOut[k] = vals
		}
		rows = append(rows, FunnelRow{
			LandingDomain:   g.domain,
			Path:            g.path,
			ActiveAds:       len(g.adIDs),
			TotalCollations: g.collations,
			MaxDaysRunning:  g.maxDays,
			TopCTAs:         topKeysByCount(g.ctas, 3),
			UTMValues:       utmOut,
			SampleURL:       g.sampleURL,
			SampleAdID:      g.sampleAdID,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TotalCollations != rows[j].TotalCollations {
			return rows[i].TotalCollations > rows[j].TotalCollations
		}
		if rows[i].ActiveAds != rows[j].ActiveAds {
			return rows[i].ActiveAds > rows[j].ActiveAds
		}
		return rows[i].LandingDomain+rows[i].Path < rows[j].LandingDomain+rows[j].Path
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	for i := range rows {
		rows[i].Rank = i + 1
	}
	return rows, nil
}

// TransparencyReport rolls up EU/UK/BR reach, beneficiary/payer disclosures,
// spend brackets, demographic reach shares, and violations for an advertiser
// or political byline.
type TransparencyReport struct {
	PageID              string             `json:"page_id,omitempty"`
	PageName            string             `json:"page_name,omitempty"`
	BylineFilter        string             `json:"byline_filter,omitempty"`
	AdsAnalyzed         int                `json:"ads_analyzed"`
	DetailsAnalyzed     int                `json:"details_analyzed"`
	EUTotalReachSum     int64              `json:"eu_total_reach_sum"`
	UKTotalReachSum     int64              `json:"uk_total_reach_sum"`
	BRTotalReachSum     int64              `json:"br_total_reach_sum"`
	SpendLowerSum       int64              `json:"spend_lower_sum"`
	SpendUpperSum       int64              `json:"spend_upper_sum"`
	ImpressionsLowerSum int64              `json:"impressions_lower_sum"`
	ImpressionsUpperSum int64              `json:"impressions_upper_sum"`
	Currencies          []string           `json:"currencies"`
	Bylines             []string           `json:"bylines"`
	BeneficiaryPayers   []BeneficiaryPayer `json:"beneficiary_payers"`
	GenderReach         map[string]int64   `json:"gender_reach"`
	AgeReach            map[string]int64   `json:"age_reach"`
	ViolatingAds        int                `json:"violating_ads"`
	AIDisclosureAds     int                `json:"ai_disclosure_ads"`
	ViolationTypes      []string           `json:"violation_types"`
}

// ComputeTransparency aggregates meta_ad_details + meta_ads for an advertiser
// or political byline. Safe on read-only handles when meta_* tables do not exist yet.
func ComputeTransparency(ctx context.Context, s *store.Store, pageFilter, bylineFilter string) (TransparencyReport, error) {
	rep := TransparencyReport{
		BylineFilter:      strings.TrimSpace(bylineFilter),
		Currencies:        make([]string, 0),
		Bylines:           make([]string, 0),
		BeneficiaryPayers: make([]BeneficiaryPayer, 0),
		GenderReach:       make(map[string]int64),
		AgeReach:          make(map[string]int64),
		ViolationTypes:    make([]string, 0),
	}

	var pid string
	if strings.TrimSpace(pageFilter) != "" {
		resolvedID, pname, _, err := ResolvePageID(ctx, s, pageFilter)
		if err != nil {
			return rep, err
		}
		pid = resolvedID
		rep.PageID = resolvedID
		rep.PageName = pname
	}

	// Drain meta_ad_details rows first when table exists.
	var rawDetails []string
	hasDetails, err := tableExists(ctx, s, "meta_ad_details")
	if err != nil {
		return rep, err
	}
	if hasDetails {
		q := `SELECT raw_json FROM meta_ad_details WHERE 1=1`
		var args []any
		if pid != "" {
			q += ` AND page_id = ?`
			args = append(args, pid)
		}
		if rep.BylineFilter != "" {
			q += ` AND LOWER(byline) LIKE LOWER(?)`
			args = append(args, "%"+rep.BylineFilter+"%")
		}
		rows, err := s.DB().QueryContext(ctx, q, args...)
		if err != nil {
			return rep, err
		}
		for rows.Next() {
			var r string
			if err := rows.Scan(&r); err != nil {
				_ = rows.Close()
				return rep, err
			}
			rawDetails = append(rawDetails, r)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return rep, err
		}
		_ = rows.Close()
	}

	// Also read meta_ads rows (for spend/impressions/bylines from ads search / official archive).
	ads, err := LoadAds(ctx, s, pid, false)
	if err != nil {
		return rep, err
	}

	currencies := make(map[string]bool)
	bylines := make(map[string]bool)
	bpSeen := make(map[string]bool)
	violSeen := make(map[string]bool)
	spendCounted := make(map[string]bool)
	impCounted := make(map[string]bool)

	for _, ad := range ads {
		if rep.BylineFilter != "" && !strings.Contains(strings.ToLower(ad.Byline), strings.ToLower(rep.BylineFilter)) {
			continue
		}
		rep.AdsAnalyzed++
		if rep.PageName == "" && ad.PageName != "" {
			rep.PageName = ad.PageName
		}
		if ad.Currency != "" {
			currencies[ad.Currency] = true
		}
		if ad.Byline != "" {
			bylines[ad.Byline] = true
		}
		if ad.SpendLower != nil || ad.SpendUpper != nil {
			spendCounted[ad.AdArchiveID] = true
			if ad.SpendLower != nil {
				rep.SpendLowerSum += *ad.SpendLower
			}
			if ad.SpendUpper != nil {
				rep.SpendUpperSum += *ad.SpendUpper
			}
		}
		if ad.ImpressionsLower != nil || ad.ImpressionsUpper != nil {
			impCounted[ad.AdArchiveID] = true
			if ad.ImpressionsLower != nil {
				rep.ImpressionsLowerSum += *ad.ImpressionsLower
			}
			if ad.ImpressionsUpper != nil {
				rep.ImpressionsUpperSum += *ad.ImpressionsUpper
			}
		}
	}

	for _, r := range rawDetails {
		var det AdDetails
		if err := json.Unmarshal([]byte(r), &det); err != nil {
			continue
		}
		rep.DetailsAnalyzed++
		if rep.PageName == "" && det.PageName != "" {
			rep.PageName = det.PageName
		}
		rep.EUTotalReachSum += det.EUTotalReach
		rep.UKTotalReachSum += det.UKTotalReach
		rep.BRTotalReachSum += det.BRTotalReach
		if det.Currency != "" {
			currencies[det.Currency] = true
		}
		if det.Byline != "" {
			bylines[det.Byline] = true
		}
		if !spendCounted[det.AdArchiveID] && (det.SpendLower != nil || det.SpendUpper != nil) {
			spendCounted[det.AdArchiveID] = true
			if det.SpendLower != nil {
				rep.SpendLowerSum += *det.SpendLower
			}
			if det.SpendUpper != nil {
				rep.SpendUpperSum += *det.SpendUpper
			}
		}
		if !impCounted[det.AdArchiveID] && (det.ImpressionsLower != nil || det.ImpressionsUpper != nil) {
			impCounted[det.AdArchiveID] = true
			if det.ImpressionsLower != nil {
				rep.ImpressionsLowerSum += *det.ImpressionsLower
			}
			if det.ImpressionsUpper != nil {
				rep.ImpressionsUpperSum += *det.ImpressionsUpper
			}
		}
		if det.IsViolatingEUSIEP {
			rep.ViolatingAds++
		}
		if det.AIDisclosureElig {
			rep.AIDisclosureAds++
		}
		for _, vt := range det.ViolationTypes {
			if vt != "" && !violSeen[vt] {
				violSeen[vt] = true
				rep.ViolationTypes = append(rep.ViolationTypes, vt)
			}
		}
		for _, bp := range det.BeneficiaryPayers {
			key := bp.Beneficiary + "|" + bp.Payer
			if !bpSeen[key] {
				bpSeen[key] = true
				rep.BeneficiaryPayers = append(rep.BeneficiaryPayers, bp)
			}
		}
		for _, d := range det.Demographics {
			if d.Gender != "" {
				rep.GenderReach[d.Gender] += d.Reach
			}
			if d.AgeRange != "" {
				rep.AgeReach[d.AgeRange] += d.Reach
			}
		}
	}

	for c := range currencies {
		rep.Currencies = append(rep.Currencies, c)
	}
	for b := range bylines {
		rep.Bylines = append(rep.Bylines, b)
	}
	sort.Strings(rep.Currencies)
	sort.Strings(rep.Bylines)
	sort.Strings(rep.ViolationTypes)
	return rep, nil
}

// SOVRow ranks one advertiser's share of voice for a category keyword.
type SOVRow struct {
	Rank              int     `json:"rank"`
	PageID            string  `json:"page_id"`
	PageName          string  `json:"page_name"`
	ActiveAds         int     `json:"active_ads"`
	TotalCollations   int     `json:"total_collations"`
	AdSharePct        float64 `json:"ad_share_pct"`
	CollationSharePct float64 `json:"collation_share_pct"`
	DominantFormat    string  `json:"dominant_format"`
	MedianDaysRunning int     `json:"median_days_running"`
	TopLandingDomain  string  `json:"top_landing_domain,omitempty"`
}

// ComputeSOV ranks competing advertisers matching a keyword across local SQLite
// ads (body, headline, caption, description, page name, or landing domain).
func ComputeSOV(ctx context.Context, s *store.Store, keyword string, limit int) ([]SOVRow, error) {
	ads, err := LoadAds(ctx, s, "", true)
	if err != nil {
		return nil, err
	}
	return ComputeSOVFromAds(ads, keyword, limit), nil
}

// ComputeSOVFromAds ranks competing advertisers from an in-memory slice of ads
// (used by both ComputeSOV and no-local-data live runs).
func ComputeSOVFromAds(ads []Ad, keyword string, limit int) []SOVRow {
	kw := strings.ToLower(strings.TrimSpace(keyword))
	terms := strings.Fields(kw)

	type acc struct {
		pageID     string
		pageName   string
		ads        int
		collations int
		ages       []int
		formats    map[string]int
		domains    map[string]int
	}
	byPage := make(map[string]*acc)
	totalAds := 0
	totalColl := 0

	for _, ad := range ads {
		if len(terms) > 0 {
			haystack := strings.ToLower(strings.Join([]string{
				ad.PageName, ad.BodyText, ad.LinkTitle, ad.LinkCaption, ad.LinkDescription, ad.CTAText, ad.LandingDomain,
			}, " "))
			matched := true
			for _, t := range terms {
				if !strings.Contains(haystack, t) {
					matched = false
					break
				}
			}
			if !matched {
				continue
			}
		}
		key := ad.PageID
		if key == "" {
			key = ad.PageName
		}
		g, ok := byPage[key]
		if !ok {
			g = &acc{
				pageID:   ad.PageID,
				pageName: ad.PageName,
				formats:  make(map[string]int),
				domains:  make(map[string]int),
			}
			byPage[key] = g
		}
		g.ads++
		g.collations += ad.CollationCount
		g.ages = append(g.ages, ad.DaysRunning)
		if ad.DisplayFormat != "" {
			g.formats[ad.DisplayFormat]++
		}
		if ad.LandingDomain != "" {
			g.domains[ad.LandingDomain]++
		}
		totalAds++
		totalColl += ad.CollationCount
	}

	rows := make([]SOVRow, 0, len(byPage))
	for _, g := range byPage {
		adShare := 0.0
		if totalAds > 0 {
			adShare = round1(float64(g.ads) * 100.0 / float64(totalAds))
		}
		collShare := 0.0
		if totalColl > 0 {
			collShare = round1(float64(g.collations) * 100.0 / float64(totalColl))
		}
		rows = append(rows, SOVRow{
			PageID:            g.pageID,
			PageName:          g.pageName,
			ActiveAds:         g.ads,
			TotalCollations:   g.collations,
			AdSharePct:        adShare,
			CollationSharePct: collShare,
			DominantFormat:    firstOrEmpty(topKeysByCount(g.formats, 1)),
			MedianDaysRunning: medianInt(g.ages),
			TopLandingDomain:  firstOrEmpty(topKeysByCount(g.domains, 1)),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TotalCollations != rows[j].TotalCollations {
			return rows[i].TotalCollations > rows[j].TotalCollations
		}
		if rows[i].ActiveAds != rows[j].ActiveAds {
			return rows[i].ActiveAds > rows[j].ActiveAds
		}
		return rows[i].PageName < rows[j].PageName
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	for i := range rows {
		rows[i].Rank = i + 1
	}
	return rows
}

func nullableInt64(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func medianInt(vals []int) int {
	if len(vals) == 0 {
		return 0
	}
	cp := make([]int, len(vals))
	copy(cp, vals)
	sort.Ints(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 1 {
		return cp[mid]
	}
	return (cp[mid-1] + cp[mid]) / 2
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10.0
}

func topKeysByCount(counts map[string]int, n int) []string {
	type kv struct {
		k string
		v int
	}
	list := make([]kv, 0, len(counts))
	for k, v := range counts {
		list = append(list, kv{k: k, v: v})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].v != list[j].v {
			return list[i].v > list[j].v
		}
		return list[i].k < list[j].k
	})
	if n > 0 && len(list) > n {
		list = list[:n]
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		out = append(out, item.k)
	}
	return out
}
