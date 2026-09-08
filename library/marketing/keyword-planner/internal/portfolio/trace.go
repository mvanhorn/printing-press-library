// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package portfolio

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// TraceOptions selects one normalized monthly observation. Keyword matching
// is case-insensitive and literal; wildcard characters have no special
// meaning. IncludeRaw is opt-in because a trace normally needs the receipt
// identity, body hash and presence bit, rather than the complete response.
type TraceOptions struct {
	Keyword    string `json:"keyword"`
	Month      string `json:"month"`
	IncludeRaw bool   `json:"include_raw"`
}

// TraceReport ties every matching monthly observation to the metric, request
// snapshot and exact response receipt that produced it. Multiple matches are
// returned in full and marked ambiguous instead of being silently collapsed.
type TraceReport struct {
	Snapshot   Snapshot     `json:"snapshot"`
	Keyword    string       `json:"keyword"`
	Month      string       `json:"month"`
	Matches    []TraceMatch `json:"matches"`
	MatchCount int          `json:"match_count"`
	Ambiguous  bool         `json:"ambiguous"`
	Resolution string       `json:"resolution"`
	Warnings   []string     `json:"warnings"`
}

// TraceMatch contains the normalized row plus the stored metric and receipt
// records. Row.MonthlySearches remains a pointer, so unavailable and zero are
// distinct in the trace.
type TraceMatch struct {
	Row     Row          `json:"row"`
	Metric  TraceMetric  `json:"metric"`
	Receipt TraceReceipt `json:"receipt"`
}

// TraceMetric exposes the typed metric context and the exact normalized result
// object retained alongside the full raw response receipt.
type TraceMetric struct {
	MetricID                    string          `json:"metric_id"`
	SnapshotID                  string          `json:"snapshot_id"`
	ReceiptID                   string          `json:"receipt_id"`
	Endpoint                    string          `json:"endpoint"`
	ReturnedText                string          `json:"returned_text"`
	SubmittedText               string          `json:"submitted_text"`
	SubmittedIndex              *int            `json:"submitted_index"`
	VariantGroup                string          `json:"variant_group"`
	LinkageStatus               string          `json:"linkage_status"`
	AverageMonthlySearches      *int64          `json:"average_monthly_searches"`
	AverageMonthlySearchesState string          `json:"average_monthly_searches_state"`
	Competition                 string          `json:"competition"`
	CompetitionIndex            *int64          `json:"competition_index"`
	LowBidMicros                *int64          `json:"low_bid_micros"`
	HighBidMicros               *int64          `json:"high_bid_micros"`
	AverageCPCMicros            *int64          `json:"average_cpc_micros"`
	AverageCPCState             string          `json:"average_cpc_state"`
	Annotations                 json.RawMessage `json:"annotations"`
	RawResult                   json.RawMessage `json:"raw_result"`
	RawResultPresent            bool            `json:"raw_result_present"`
	Status                      string          `json:"status"`
	Flags                       []string        `json:"flags"`
}

// TraceReceipt is the exact response-receipt link. RawBody is populated only
// when TraceOptions.IncludeRaw is true; BodyPresent and BodySHA256 are always
// reported so absence is never mistaken for an empty successful response.
type TraceReceipt struct {
	ResponseIDs
	Endpoint           string          `json:"endpoint"`
	PageToken          string          `json:"page_token"`
	RequestBody        json.RawMessage `json:"request_body"`
	RequestBodyPresent bool            `json:"request_body_present"`
	SubmittedKeywords  []string        `json:"submitted_keywords"`
	HTTPStatus         int             `json:"http_status"`
	GoogleRequestID    string          `json:"google_request_id"`
	RawBody            []byte          `json:"raw_body_base64"`
	BodyPresent        bool            `json:"body_present"`
	FetchedAt          time.Time       `json:"fetched_at"`
	ErrorCode          string          `json:"error_code"`
	ErrorMessage       string          `json:"error_message"`
	Normalized         bool            `json:"normalized"`
}

// Trace resolves one stored monthly value entirely offline. It includes
// normalized rows from an incomplete snapshot so a partial collection remains
// traceable; the snapshot status and warnings make that state explicit.
func (s *Store) Trace(ctx context.Context, snapshotID string, opts TraceOptions) (TraceReport, error) {
	keyword := strings.TrimSpace(opts.Keyword)
	if keyword == "" {
		return TraceReport{}, fmt.Errorf("%w: trace keyword is required", ErrInvalidInput)
	}
	month, err := ParseYearMonth(opts.Month)
	if err != nil {
		return TraceReport{}, err
	}
	snapshot, err := s.loadReportSnapshot(ctx, snapshotID)
	if err != nil {
		return TraceReport{}, err
	}
	rows, err := s.Rows(ctx, QueryOptions{
		SnapshotID:        snapshot.ID,
		Keyword:           keyword,
		StartMonth:        month.String(),
		EndMonth:          month.String(),
		IncludeIncomplete: true,
	})
	if err != nil {
		return TraceReport{}, err
	}

	result := TraceReport{
		Snapshot: snapshot,
		Keyword:  keyword,
		Month:    month.String(),
		Matches:  make([]TraceMatch, 0, len(rows)),
		Warnings: []string{},
	}
	for _, row := range rows {
		metric, err := s.loadTraceMetric(ctx, snapshot.ID, row.MetricID)
		if err != nil {
			return TraceReport{}, err
		}
		receipt, err := s.loadTraceReceipt(ctx, snapshot.ID, row.ReceiptID, opts.IncludeRaw)
		if err != nil {
			return TraceReport{}, err
		}
		result.Matches = append(result.Matches, TraceMatch{Row: row, Metric: metric, Receipt: receipt})
		if row.MonthlySearches == nil {
			result.Warnings = append(result.Warnings, "monthly_value_unavailable")
		}
		if hasString(row.Flags, "ambiguous_zero_unspecified") {
			result.Warnings = append(result.Warnings, "ambiguous_zero_unspecified")
		}
	}
	result.MatchCount = len(result.Matches)
	switch result.MatchCount {
	case 0:
		result.Resolution = "none"
		result.Warnings = append(result.Warnings, "no_normalized_monthly_match")
	case 1:
		result.Resolution = "unique"
	default:
		result.Resolution = "ambiguous"
		result.Ambiguous = true
		result.Warnings = append(result.Warnings, "multiple_normalized_monthly_matches")
	}
	result.Warnings = sortedUnique(result.Warnings)
	return result, nil
}

func (s *Store) loadReportSnapshot(ctx context.Context, alias string) (Snapshot, error) {
	id, err := s.ResolveSnapshot(ctx, alias)
	if err != nil {
		return Snapshot{}, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT `+snapshotColumns+` FROM snapshots WHERE id = ?`, id)
	snapshot, err := scanSnapshot(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Snapshot{}, ErrNotFound
	}
	if err != nil {
		return Snapshot{}, fmt.Errorf("load report snapshot: %w", err)
	}
	return snapshot, nil
}

func (s *Store) loadTraceMetric(ctx context.Context, snapshotID, metricID string) (TraceMetric, error) {
	var metric TraceMetric
	var submittedIndex sql.NullInt64
	var averageMonthly, competitionIndex, lowBid, highBid, averageCPC sql.NullInt64
	var annotations, rawResult, flagsJSON []byte
	if err := s.db.QueryRowContext(ctx, `
		SELECT id, snapshot_id, receipt_id, endpoint, returned_text,
		       submitted_text, submitted_index, variant_group, linkage_status,
		       average_monthly_searches, average_monthly_searches_state,
		       competition, competition_index, low_bid_micros, high_bid_micros,
		       average_cpc_micros, average_cpc_state, annotations, raw_result,
		       status, flags
		FROM keyword_metrics WHERE id = ? AND snapshot_id = ?`, metricID, snapshotID).
		Scan(&metric.MetricID, &metric.SnapshotID, &metric.ReceiptID, &metric.Endpoint,
			&metric.ReturnedText, &metric.SubmittedText, &submittedIndex,
			&metric.VariantGroup, &metric.LinkageStatus, &averageMonthly,
			&metric.AverageMonthlySearchesState, &metric.Competition,
			&competitionIndex, &lowBid, &highBid, &averageCPC,
			&metric.AverageCPCState, &annotations, &rawResult, &metric.Status,
			&flagsJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TraceMetric{}, ErrNotFound
		}
		return TraceMetric{}, fmt.Errorf("load trace metric: %w", err)
	}
	if submittedIndex.Valid {
		value := int(submittedIndex.Int64)
		metric.SubmittedIndex = &value
	}
	setInt64(&metric.AverageMonthlySearches, averageMonthly)
	setInt64(&metric.CompetitionIndex, competitionIndex)
	setInt64(&metric.LowBidMicros, lowBid)
	setInt64(&metric.HighBidMicros, highBid)
	setInt64(&metric.AverageCPCMicros, averageCPC)
	metric.Annotations = json.RawMessage(cloneBytes(annotations))
	metric.RawResult = json.RawMessage(cloneBytes(rawResult))
	metric.RawResultPresent = rawResult != nil
	var err error
	metric.Flags, err = parseFlags(flagsJSON)
	if err != nil {
		return TraceMetric{}, fmt.Errorf("decode trace metric flags: %w", err)
	}
	return metric, nil
}

func (s *Store) loadTraceReceipt(ctx context.Context, snapshotID, receiptID string, includeRaw bool) (TraceReceipt, error) {
	bodyExpression := "NULL"
	if includeRaw {
		bodyExpression = "body"
	}
	query := `SELECT id, snapshot_id, endpoint, page_token, page_number,
		batch_number, attempt, request_body, request_body_present,
		submitted_keywords, http_status, google_request_id, body_present, ` + bodyExpression + `,
		body_sha256, fetched_at, error_code, error_message, normalized
		FROM response_receipts WHERE id = ? AND snapshot_id = ?`
	var receipt TraceReceipt
	var requestBody, submittedJSON, body []byte
	var requestPresent, bodyPresent, normalized int
	var fetchedAt string
	if err := s.db.QueryRowContext(ctx, query, receiptID, snapshotID).Scan(
		&receipt.ReceiptID, &receipt.SnapshotID, &receipt.Endpoint, &receipt.PageToken,
		&receipt.PageNumber, &receipt.BatchNumber, &receipt.Attempt,
		&requestBody, &requestPresent, &submittedJSON, &receipt.HTTPStatus,
		&receipt.GoogleRequestID, &bodyPresent, &body, &receipt.BodySHA256,
		&fetchedAt, &receipt.ErrorCode, &receipt.ErrorMessage, &normalized); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TraceReceipt{}, ErrNotFound
		}
		return TraceReceipt{}, fmt.Errorf("load trace receipt: %w", err)
	}
	receipt.RequestBody = json.RawMessage(cloneBytes(requestBody))
	receipt.RequestBodyPresent = requestPresent != 0
	if !receipt.RequestBodyPresent {
		receipt.RequestBody = nil
	}
	var err error
	receipt.SubmittedKeywords, err = unmarshalStrings(submittedJSON)
	if err != nil {
		return TraceReceipt{}, fmt.Errorf("decode trace receipt keywords: %w", err)
	}
	receipt.BodyPresent = bodyPresent != 0
	if includeRaw && receipt.BodyPresent {
		receipt.RawBody = cloneBytes(body)
	}
	receipt.FetchedAt, err = time.Parse(time.RFC3339Nano, fetchedAt)
	if err != nil {
		return TraceReceipt{}, fmt.Errorf("decode trace receipt timestamp: %w", err)
	}
	receipt.Normalized = normalized != 0
	return receipt, nil
}

func hasString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
