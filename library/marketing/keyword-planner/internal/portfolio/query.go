// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package portfolio

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ListSnapshots returns the offline snapshot inventory. The inventory includes
// complete, incomplete, failed and still-in-progress collections so an
// operator can account for every attempted run. IncludeIncomplete is retained
// for compatibility; analytical Rows applies the completion filter instead.
func (s *Store) ListSnapshots(ctx context.Context, opts QueryOptions) ([]Snapshot, error) {
	if err := validateQueryOptions(opts); err != nil {
		return nil, err
	}
	if opts.SnapshotID != "" {
		resolved, err := s.ResolveSnapshot(ctx, opts.SnapshotID)
		if err != nil {
			return []Snapshot{}, err
		}
		opts.SnapshotID = resolved
	}
	geoID, err := queryGeoID(opts)
	if err != nil {
		return nil, err
	}
	query := `SELECT ` + snapshotColumns + ` FROM snapshots s WHERE 1=1`
	args := make([]any, 0, 8)
	if opts.SnapshotID != "" {
		query += ` AND s.id = ?`
		args = append(args, opts.SnapshotID)
	}
	if opts.Endpoint != "" {
		endpoint, err := NormalizeEndpoint(opts.Endpoint)
		if err != nil {
			return nil, err
		}
		query += ` AND s.endpoint = ?`
		args = append(args, endpoint)
	}
	if opts.Language != "" {
		query += ` AND s.language = ?`
		args = append(args, opts.Language)
	}
	if geoID != "" {
		query += ` AND s.geo_set_id = ?`
		args = append(args, geoID)
	}
	if opts.StartMonth != "" {
		query += ` AND s.requested_end >= ?`
		args = append(args, opts.StartMonth)
	}
	if opts.EndMonth != "" {
		query += ` AND s.requested_start <= ?`
		args = append(args, opts.EndMonth)
	}
	if opts.Keyword != "" {
		query += ` AND (instr(LOWER(s.submitted_keywords), LOWER(?)) > 0
			OR instr(LOWER(s.submitted_seeds), LOWER(?)) > 0
			OR EXISTS (SELECT 1 FROM keyword_metrics km WHERE km.snapshot_id = s.id
				AND instr(LOWER(km.returned_text), LOWER(?)) > 0))`
		args = append(args, opts.Keyword, opts.Keyword, opts.Keyword)
	}
	query += ` ORDER BY s.fetched_at DESC, s.rowid DESC`
	if opts.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, opts.Limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list portfolio snapshots: %w", err)
	}
	defer rows.Close()
	result := make([]Snapshot, 0)
	for rows.Next() {
		snapshot, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read portfolio snapshots: %w", err)
	}
	return result, nil
}

// ResolveSnapshot resolves an exact ID or the stable latest/previous alias.
func (s *Store) ResolveSnapshot(ctx context.Context, alias string) (string, error) {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return "", fmt.Errorf("%w: snapshot alias is empty", ErrInvalidInput)
	}
	if alias != "latest" && alias != "previous" {
		var id string
		if err := s.db.QueryRowContext(ctx, `SELECT id FROM snapshots WHERE id = ?`, alias).Scan(&id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return "", ErrNotFound
			}
			return "", fmt.Errorf("resolve snapshot ID: %w", err)
		}
		return id, nil
	}
	offset := 0
	if alias == "previous" {
		offset = 1
	}
	var id string
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM snapshots
        ORDER BY fetched_at DESC, rowid DESC LIMIT 1 OFFSET ?`, offset).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("resolve %s snapshot: %w", alias, err)
	}
	return id, nil
}

// ShowSnapshot returns all offline evidence for one snapshot. Raw receipts are
// included verbatim and are not reconstructed from typed rows.
func (s *Store) ShowSnapshot(ctx context.Context, alias string) (SnapshotView, error) {
	id, err := s.ResolveSnapshot(ctx, alias)
	if err != nil {
		return SnapshotView{}, err
	}
	var view SnapshotView
	row := s.db.QueryRowContext(ctx, `SELECT `+snapshotColumns+` FROM snapshots WHERE id = ?`, id)
	view.Snapshot, err = scanSnapshot(row)
	if err != nil {
		return SnapshotView{}, err
	}
	view.Receipts, err = s.listReceipts(ctx, id)
	if err != nil {
		return SnapshotView{}, err
	}
	view.Rows, err = s.Rows(ctx, QueryOptions{SnapshotID: id, IncludeIncomplete: true})
	if err != nil {
		return SnapshotView{}, err
	}
	view.Coverage, err = s.listCoverage(ctx, id)
	if err != nil {
		return SnapshotView{}, err
	}
	if view.Receipts == nil {
		view.Receipts = []ReceiptView{}
	}
	if view.Rows == nil {
		view.Rows = []Row{}
	}
	if view.Coverage == nil {
		view.Coverage = []CoverageView{}
	}
	return view, nil
}

// Rows returns one row per returned metric group and month. It never joins
// variant_links, so submitting two close variants cannot multiply a monthly
// observation.
func (s *Store) Rows(ctx context.Context, opts QueryOptions) ([]Row, error) {
	if err := validateQueryOptions(opts); err != nil {
		return nil, err
	}
	if opts.SnapshotID != "" {
		resolved, err := s.ResolveSnapshot(ctx, opts.SnapshotID)
		if err != nil {
			return []Row{}, err
		}
		opts.SnapshotID = resolved
	}
	geoID, err := queryGeoID(opts)
	if err != nil {
		return nil, err
	}
	query := `SELECT
        mv.snapshot_id, mv.receipt_id, mv.metric_id, km.endpoint,
        km.returned_text, km.submitted_text, km.submitted_index,
        km.variant_group, km.linkage_status,
        s.geo_set_id, s.geo_target_constants, s.language, s.network,
        COALESCE((SELECT am.currency_code FROM account_metadata am
                  WHERE am.snapshot_id = s.id ORDER BY am.recorded_at DESC, am.id DESC LIMIT 1), s.currency_code),
        COALESCE((SELECT am.source FROM account_metadata am
                  WHERE am.snapshot_id = s.id ORDER BY am.recorded_at DESC, am.id DESC LIMIT 1), s.currency_source),
        mv.month, mv.raw_month, mv.raw_year, mv.monthly_searches, mv.value_state,
        km.low_bid_micros, km.high_bid_micros, km.average_cpc_micros,
        km.average_monthly_searches, km.competition, km.competition_index,
        mv.eligibility, mv.flags, s.fetched_at, s.requested_start, s.requested_end,
        s.complete
    FROM monthly_volumes mv
    JOIN keyword_metrics km ON km.id = mv.metric_id
    JOIN snapshots s ON s.id = mv.snapshot_id
    WHERE mv.eligibility = ?`
	args := []any{EligibilityEligible}
	if !opts.IncludeIncomplete {
		query += ` AND s.complete = 1`
	}
	if opts.SnapshotID != "" {
		query += ` AND s.id = ?`
		args = append(args, opts.SnapshotID)
	}
	if opts.Keyword != "" {
		query += ` AND (instr(LOWER(km.returned_text), LOWER(?)) > 0
			OR instr(LOWER(km.submitted_text), LOWER(?)) > 0
			OR EXISTS (SELECT 1 FROM variant_links vl WHERE vl.metric_id = km.id
				AND vl.relationship IN ('EXACT', 'NORMALIZED', 'CLOSE_VARIANT')
				AND (instr(LOWER(vl.submitted_text), LOWER(?)) > 0
					 OR instr(LOWER(vl.returned_text), LOWER(?)) > 0)))`
		args = append(args, opts.Keyword, opts.Keyword, opts.Keyword, opts.Keyword)
	}
	if opts.Language != "" {
		query += ` AND s.language = ?`
		args = append(args, opts.Language)
	}
	if geoID != "" {
		query += ` AND s.geo_set_id = ?`
		args = append(args, geoID)
	}
	if opts.Endpoint != "" {
		endpoint, err := NormalizeEndpoint(opts.Endpoint)
		if err != nil {
			return nil, err
		}
		query += ` AND km.endpoint = ?`
		args = append(args, endpoint)
	}
	if opts.StartMonth != "" {
		query += ` AND (mv.month IS NULL OR mv.month >= ?)`
		args = append(args, opts.StartMonth+"-01")
	}
	if opts.EndMonth != "" {
		query += ` AND (mv.month IS NULL OR mv.month <= ?)`
		args = append(args, opts.EndMonth+"-01")
	}
	if opts.AvailableOnly {
		query += ` AND mv.value_state = 'value'`
	}
	query += ` ORDER BY s.fetched_at DESC, s.rowid DESC, km.id, mv.month, mv.id`
	if opts.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, opts.Limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query portfolio rows: %w", err)
	}
	defer rows.Close()
	result := make([]Row, 0)
	for rows.Next() {
		item, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read portfolio rows: %w", err)
	}
	return result, nil
}

// Search performs an offline keyword search. A term with no local match is a
// successful empty result and never falls back to a network call.
func (s *Store) Search(ctx context.Context, query string, opts QueryOptions) ([]Row, error) {
	if strings.TrimSpace(query) == "" {
		return []Row{}, nil
	}
	opts.Keyword = query
	return s.Rows(ctx, opts)
}

// Export renders offline rows as JSON or CSV. Raw response bodies are exposed
// only by ShowSnapshot; analytical exports contain typed rows and provenance.
func (s *Store) Export(ctx context.Context, opts QueryOptions, format ExportFormat) ([]byte, error) {
	rows, err := s.Rows(ctx, opts)
	if err != nil {
		return nil, err
	}
	switch format {
	case ExportJSON:
		return json.Marshal(rows)
	case ExportCSV:
		var buffer bytes.Buffer
		writer := csv.NewWriter(&buffer)
		header := []string{"keyword", "geo", "language_code", "month", "monthly_searches", "low_bid_micros", "high_bid_micros", "competition", "fetched_at", "snapshot_id", "endpoint", "variant_group", "geo_set_id", "geo_targets_json", "language_resource", "network", "currency", "requested_start", "requested_end", "status", "complete", "flags", "value_state"}
		if err := writer.Write(header); err != nil {
			return nil, fmt.Errorf("write portfolio CSV header: %w", err)
		}
		for _, item := range rows {
			geo := verifiedGeo(item.GeoTargets)
			record := []string{item.Keyword, geo, verifiedLanguage(item.Language), item.Month,
				formatInt64(item.MonthlySearches), formatInt64(item.LowBidMicros), formatInt64(item.HighBidMicros),
				item.Competition, item.FetchedAt.UTC().Format(time.RFC3339Nano), item.SnapshotID,
				item.Endpoint, item.VariantGroup, item.GeoSetID, mustJSONText(item.GeoTargets), item.Language,
				item.Network, item.Currency,
				item.RequestedStart, item.RequestedEnd, item.Status, strconv.FormatBool(item.Complete),
				mustJSONText(item.Flags), item.ValueState}
			if err := writer.Write(record); err != nil {
				return nil, fmt.Errorf("write portfolio CSV row: %w", err)
			}
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return nil, fmt.Errorf("flush portfolio CSV: %w", err)
		}
		return buffer.Bytes(), nil
	default:
		return nil, fmt.Errorf("%w: unsupported export format %q", ErrInvalidInput, format)
	}
}

func verifiedGeo(targets []string) string {
	if len(targets) == 1 && targets[0] == "geoTargetConstants/2840" {
		return "US"
	}
	return ""
}

func verifiedLanguage(resource string) string {
	if resource == "languageConstants/1000" {
		return "en"
	}
	return ""
}

func validateQueryOptions(opts QueryOptions) error {
	if opts.Limit < 0 {
		return fmt.Errorf("%w: query limit cannot be negative", ErrInvalidInput)
	}
	if opts.StartMonth != "" {
		if _, err := ParseYearMonth(opts.StartMonth); err != nil {
			return err
		}
	}
	if opts.EndMonth != "" {
		if _, err := ParseYearMonth(opts.EndMonth); err != nil {
			return err
		}
	}
	if opts.StartMonth != "" && opts.EndMonth != "" {
		start, _ := ParseYearMonth(opts.StartMonth)
		end, _ := ParseYearMonth(opts.EndMonth)
		if compareMonth(start, end) > 0 {
			return fmt.Errorf("%w: query start is after end", ErrInvalidInput)
		}
	}
	return nil
}

func queryGeoID(opts QueryOptions) (string, error) {
	if len(opts.GeoTargets) == 0 {
		return strings.TrimSpace(opts.GeoSetID), nil
	}
	id, _, err := geoSetID(opts.GeoTargets)
	if err != nil {
		return "", err
	}
	if opts.GeoSetID != "" && opts.GeoSetID != id {
		return "", fmt.Errorf("%w: geo_set_id does not match geo targets", ErrInvalidInput)
	}
	return id, nil
}

func scanSnapshot(scanner interface{ Scan(...any) error }) (Snapshot, error) {
	var snapshot Snapshot
	var geoJSON, seedsJSON, keywordsJSON []byte
	var requestBody, metadata []byte
	var requestBodyPresent, complete, finalized int
	var finishedAt sql.NullString
	var fetchedAt string
	var warningJSON, failedJSON, missingJSON []byte
	if err := scanner.Scan(
		&snapshot.ID, &snapshot.RunID, &snapshot.Endpoint, &snapshot.APIVersion,
		&snapshot.DiscoveryRevision, &snapshot.CustomerID, &snapshot.LoginCustomerID,
		&snapshot.Language, &snapshot.Network, &snapshot.CurrencyCode, &snapshot.CurrencySource,
		&snapshot.GeoSetID, &geoJSON, &seedsJSON, &keywordsJSON, &snapshot.RequestedStart,
		&snapshot.RequestedEnd, &requestBody, &requestBodyPresent, &snapshot.RequestBodyHash,
		&snapshot.SourceVariant, &metadata, &fetchedAt, &snapshot.Status, &complete,
		&finalized, &finishedAt, &warningJSON, &failedJSON, &missingJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Snapshot{}, ErrNotFound
		}
		return Snapshot{}, fmt.Errorf("scan portfolio snapshot: %w", err)
	}
	var err error
	if snapshot.GeoTargetConstants, err = unmarshalStrings(geoJSON); err != nil {
		return Snapshot{}, fmt.Errorf("decode snapshot geo set: %w", err)
	}
	if snapshot.SubmittedSeeds, err = unmarshalStrings(seedsJSON); err != nil {
		return Snapshot{}, fmt.Errorf("decode snapshot seeds: %w", err)
	}
	if snapshot.SubmittedKeywords, err = unmarshalStrings(keywordsJSON); err != nil {
		return Snapshot{}, fmt.Errorf("decode snapshot keywords: %w", err)
	}
	snapshot.RequestBody = json.RawMessage(cloneBytes(requestBody))
	if requestBodyPresent == 0 {
		snapshot.RequestBody = nil
	}
	snapshot.Metadata = json.RawMessage(cloneBytes(metadata))
	snapshot.FetchedAt, err = time.Parse(time.RFC3339Nano, fetchedAt)
	if err != nil {
		return Snapshot{}, fmt.Errorf("decode snapshot fetched_at: %w", err)
	}
	snapshot.Complete = complete != 0
	snapshot.Finalized = finalized != 0
	if finishedAt.Valid {
		value, parseErr := time.Parse(time.RFC3339Nano, finishedAt.String)
		if parseErr != nil {
			return Snapshot{}, fmt.Errorf("decode snapshot finished_at: %w", parseErr)
		}
		snapshot.FinishedAt = &value
	}
	if snapshot.WarningFlags, err = parseFlags(warningJSON); err != nil {
		return Snapshot{}, fmt.Errorf("decode snapshot warning flags: %w", err)
	}
	if snapshot.FailedReceipts, err = unmarshalStrings(failedJSON); err != nil {
		return Snapshot{}, fmt.Errorf("decode snapshot failed receipts: %w", err)
	}
	if snapshot.MissingBatches, err = unmarshalStrings(missingJSON); err != nil {
		return Snapshot{}, fmt.Errorf("decode snapshot missing batches: %w", err)
	}
	return snapshot, nil
}

func formatInt64(value *int64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatInt(*value, 10)
}

func mustJSONText(value any) string {
	return string(mustJSON(value))
}
