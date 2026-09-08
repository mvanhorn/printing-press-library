// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package portfolio

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

func (s *Store) listReceipts(ctx context.Context, snapshotID string) ([]ReceiptView, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, snapshot_id, endpoint, page_token,
        page_number, batch_number, attempt, request_body, request_body_present,
        submitted_keywords, http_status, google_request_id, body, body_present,
        body_sha256, fetched_at, error_code, error_message, normalized
        FROM response_receipts WHERE snapshot_id = ?
        ORDER BY page_number, batch_number, attempt, rowid`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("list portfolio receipts: %w", err)
	}
	defer rows.Close()
	result := make([]ReceiptView, 0)
	for rows.Next() {
		var item ReceiptView
		var requestBody, submittedJSON, body []byte
		var requestPresent, bodyPresent, normalized int
		var fetchedAt string
		if err := rows.Scan(&item.ReceiptID, &item.SnapshotID, &item.Endpoint, &item.PageToken,
			&item.PageNumber, &item.BatchNumber, &item.Attempt, &requestBody, &requestPresent,
			&submittedJSON, &item.HTTPStatus, &item.GoogleRequestID, &body, &bodyPresent,
			&item.BodySHA256, &fetchedAt, &item.ErrorCode, &item.ErrorMessage, &normalized); err != nil {
			return nil, fmt.Errorf("scan portfolio receipt: %w", err)
		}
		item.RequestBody = cloneBytes(requestBody)
		if requestPresent == 0 {
			item.RequestBody = nil
		}
		item.SubmittedKeywords, err = unmarshalStrings(submittedJSON)
		if err != nil {
			return nil, fmt.Errorf("decode receipt submitted keywords: %w", err)
		}
		item.Body = cloneBytes(body)
		item.BodyPresent = bodyPresent != 0
		if !item.BodyPresent {
			item.Body = nil
		}
		item.FetchedAt, err = time.Parse(time.RFC3339Nano, fetchedAt)
		if err != nil {
			return nil, fmt.Errorf("decode receipt fetched_at: %w", err)
		}
		item.Normalized = normalized != 0
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read portfolio receipts: %w", err)
	}
	return result, nil
}

func (s *Store) listCoverage(ctx context.Context, snapshotID string) ([]CoverageView, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT snapshot_id, receipt_id, endpoint,
        page_number, batch_number, attempt, requested_start, requested_end,
        requested_terms, returned_terms, returned_months, missing_months,
        unavailable_months, raw_only_months, requested_count, returned_count,
        returned_month_count, no_result, collection_status, complete,
        next_page_token, vendor_total_size
        FROM coverage WHERE snapshot_id = ?
        ORDER BY page_number, batch_number, attempt, id`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("list portfolio coverage: %w", err)
	}
	defer rows.Close()
	result := make([]CoverageView, 0)
	for rows.Next() {
		var item CoverageView
		var requestedJSON, returnedJSON, monthsJSON, missingJSON, unavailableJSON, rawOnlyJSON []byte
		var noResult, complete int
		var vendorTotal sql.NullInt64
		if err := rows.Scan(&item.SnapshotID, &item.ReceiptID, &item.Endpoint, &item.PageNumber,
			&item.BatchNumber, &item.Attempt, &item.RequestedStart, &item.RequestedEnd,
			&requestedJSON, &returnedJSON, &monthsJSON, &missingJSON, &unavailableJSON,
			&rawOnlyJSON, &item.RequestedCount, &item.ReturnedCount, &item.ReturnedMonthCount,
			&noResult, &item.CollectionStatus, &complete, &item.NextPageToken, &vendorTotal); err != nil {
			return nil, fmt.Errorf("scan portfolio coverage: %w", err)
		}
		item.RequestedTerms, err = unmarshalStrings(requestedJSON)
		if err != nil {
			return nil, fmt.Errorf("decode requested coverage terms: %w", err)
		}
		item.ReturnedTerms, err = unmarshalStrings(returnedJSON)
		if err != nil {
			return nil, fmt.Errorf("decode returned coverage terms: %w", err)
		}
		item.ReturnedMonths, err = unmarshalStrings(monthsJSON)
		if err != nil {
			return nil, fmt.Errorf("decode returned coverage months: %w", err)
		}
		item.MissingMonths, err = unmarshalStrings(missingJSON)
		if err != nil {
			return nil, fmt.Errorf("decode missing coverage months: %w", err)
		}
		item.UnavailableMonths, err = unmarshalStrings(unavailableJSON)
		if err != nil {
			return nil, fmt.Errorf("decode unavailable coverage months: %w", err)
		}
		item.RawOnlyMonths, err = unmarshalStrings(rawOnlyJSON)
		if err != nil {
			return nil, fmt.Errorf("decode raw-only coverage months: %w", err)
		}
		item.NoResult = noResult != 0
		item.Complete = complete != 0
		if vendorTotal.Valid {
			value := vendorTotal.Int64
			item.VendorTotalSize = &value
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read portfolio coverage: %w", err)
	}
	return result, nil
}

func scanRow(scanner interface{ Scan(...any) error }) (Row, error) {
	var item Row
	var geoJSON, flagsJSON []byte
	var month sql.NullString
	var submittedIndex, monthlySearches, lowBid, highBid, averageCPC, averageMonthly, competitionIndex sql.NullInt64
	var fetchedAt string
	var complete int
	if err := scanner.Scan(&item.SnapshotID, &item.ReceiptID, &item.MetricID, &item.Endpoint,
		&item.Keyword, &item.SubmittedKeyword, &submittedIndex, &item.VariantGroup,
		&item.LinkageStatus, &item.GeoSetID, &geoJSON, &item.Language, &item.Network,
		&item.Currency, &item.CurrencySource, &month, &item.RawMonth, &item.RawYear,
		&monthlySearches, &item.ValueState, &lowBid, &highBid, &averageCPC,
		&averageMonthly, &item.Competition, &competitionIndex, &item.Status, &flagsJSON,
		&fetchedAt, &item.RequestedStart, &item.RequestedEnd, &complete); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Row{}, ErrNotFound
		}
		return Row{}, fmt.Errorf("scan portfolio row: %w", err)
	}
	var err error
	item.GeoTargets, err = unmarshalStrings(geoJSON)
	if err != nil {
		return Row{}, fmt.Errorf("decode row geo set: %w", err)
	}
	item.Flags, err = parseFlags(flagsJSON)
	if err != nil {
		return Row{}, fmt.Errorf("decode row flags: %w", err)
	}
	if submittedIndex.Valid {
		value := int(submittedIndex.Int64)
		item.SubmittedIndex = &value
	}
	item.Month = month.String
	setInt64(&item.MonthlySearches, monthlySearches)
	setInt64(&item.LowBidMicros, lowBid)
	setInt64(&item.HighBidMicros, highBid)
	setInt64(&item.AverageCPCMicros, averageCPC)
	setInt64(&item.AverageMonthlySearches, averageMonthly)
	setInt64(&item.CompetitionIndex, competitionIndex)
	item.FetchedAt, err = time.Parse(time.RFC3339Nano, fetchedAt)
	if err != nil {
		return Row{}, fmt.Errorf("decode row fetched_at: %w", err)
	}
	item.Complete = complete != 0
	return item, nil
}

func setInt64(destination **int64, value sql.NullInt64) {
	if value.Valid {
		copied := value.Int64
		*destination = &copied
	}
}

func _ensureJSONImport() {
	// Keep encoding/json referenced in this file for Go versions that inline
	// json.RawMessage methods differently; the actual package uses it in types.
	_ = json.Valid
}
