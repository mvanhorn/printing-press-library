// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package portfolio

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

type storedReceipt struct {
	ReceiptView
	RequestBodyPresent bool
}

type responseEnvelope struct {
	Results       []json.RawMessage `json:"results"`
	NextPageToken string            `json:"nextPageToken"`
	TotalSize     json.RawMessage   `json:"totalSize"`
}

type metricRecord struct {
	ID                     string
	ReturnedText           string
	SubmittedText          string
	SubmittedIndex         *int
	VariantGroup           string
	LinkageStatus          string
	AverageMonthlySearches *int64
	AverageMonthlyState    string
	Competition            string
	CompetitionIndex       *int64
	LowBidMicros           *int64
	HighBidMicros          *int64
	AverageCPCMicros       *int64
	AverageCPCState        string
	Annotations            []byte
	RawResult              []byte
	Status                 string
	Flags                  []string
	Monthly                []monthlyRecord
}

type monthlyRecord struct {
	Month           string
	RawMonth        string
	RawMonthJSON    []byte
	RawYear         string
	RawYearJSON     []byte
	Year            int
	MonthNumber     int
	MonthlySearches *int64
	ValueState      string
	Eligibility     string
	Flags           []string
}

type linkageRecord struct {
	MetricID       string
	SubmittedIndex *int
	SubmittedText  string
	ReturnedText   string
	Relationship   string
	VariantGroup   string
}

type normalizationPage struct {
	Metrics           []metricRecord
	Links             []linkageRecord
	ReturnedTerms     []string
	ReturnedMonths    []string
	MissingMonths     []string
	UnavailableMonths []string
	RawOnlyMonths     []string
	NextPageToken     string
	VendorTotalSize   *int64
	Warnings          []string
	NoResult          bool
	Status            string
}

// NormalizeReceipt decodes only the exact raw body already committed by
// AppendReceipt, then atomically appends its typed page projection. Raw bytes
// remain the source of truth if decoding or normalization fails.
func (s *Store) NormalizeReceipt(ctx context.Context, receiptID string) (NormalizeResult, error) {
	if strings.TrimSpace(receiptID) == "" {
		return NormalizeResult{}, fmt.Errorf("%w: receipt ID is empty", ErrInvalidInput)
	}
	receipt, snapshot, err := s.loadReceiptSnapshot(ctx, receiptID)
	if err != nil {
		return NormalizeResult{}, err
	}
	if receipt.Endpoint == EndpointAccount {
		return NormalizeResult{SnapshotID: snapshot.ID, ReceiptID: receiptID, Warnings: []string{"account_receipt_is_raw_only"}}, fmt.Errorf("%w: account helper receipts require account metadata parsing", ErrInvalidInput)
	}
	if receipt.Normalized {
		return NormalizeResult{}, ErrAlreadyNormalized
	}
	result := NormalizeResult{SnapshotID: snapshot.ID, ReceiptID: receiptID, Warnings: []string{}}
	if snapshot.Finalized {
		return result, ErrFinalized
	}
	if receipt.HTTPStatus < 200 || receipt.HTTPStatus >= 300 || !receipt.BodyPresent || len(receipt.Body) == 0 {
		result.Warnings = []string{"receipt_not_normalizable"}
		result.Complete = false
		if err := s.recordNormalizationFailure(ctx, receipt, snapshot, result.Warnings); err != nil {
			return result, err
		}
		return result, fmt.Errorf("%w: receipt status %d or body unavailable", ErrInvalidResponse, receipt.HTTPStatus)
	}
	var envelope responseEnvelope
	decoder := json.NewDecoder(bytes.NewReader(receipt.Body))
	if err := decoder.Decode(&envelope); err != nil {
		result.Warnings = []string{"response_json_decode_failed"}
		if markErr := s.recordNormalizationFailure(ctx, receipt, snapshot, result.Warnings); markErr != nil {
			return result, markErr
		}
		return result, fmt.Errorf("%w: decode receipt %s: %v", ErrInvalidResponse, receiptID, err)
	}
	// Reject a second top-level JSON value. Unknown fields within the response
	// remain intentionally accepted and are retained by the raw receipt.
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		result.Warnings = []string{"response_has_trailing_json"}
		if markErr := s.recordNormalizationFailure(ctx, receipt, snapshot, result.Warnings); markErr != nil {
			return result, markErr
		}
		return result, fmt.Errorf("%w: response contains multiple JSON values", ErrInvalidResponse)
	} else if !errors.Is(err, io.EOF) {
		// A malformed trailing token is also a decode failure.
		if markErr := s.recordNormalizationFailure(ctx, receipt, snapshot, []string{"response_trailing_json_decode_failed"}); markErr != nil {
			return result, markErr
		}
		return result, fmt.Errorf("%w: trailing JSON decode: %v", ErrInvalidResponse, err)
	}

	page, err := buildNormalizationPage(receipt.Endpoint, receipt.SubmittedKeywords, snapshot, receipt.ReceiptID, envelope)
	if err != nil {
		result.Warnings = append(result.Warnings, "response_normalization_failed")
		if markErr := s.recordNormalizationFailure(ctx, receipt, snapshot, result.Warnings); markErr != nil {
			return result, markErr
		}
		return result, err
	}
	result.Warnings = append(result.Warnings, page.Warnings...)
	result.KeywordRows = len(page.Metrics)
	result.VariantRows = len(page.Links)
	for _, metric := range page.Metrics {
		for _, monthly := range metric.Monthly {
			if monthly.Eligibility == EligibilityEligible {
				result.MonthlyRows++
			}
		}
	}

	err = s.withWriteLock(ctx, func() error {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin normalization transaction: %w", err)
		}
		defer func() { _ = tx.Rollback() }()
		var status string
		var finalized, normalized int
		if err := tx.QueryRowContext(ctx, `SELECT s.status, s.finalized, r.normalized
            FROM snapshots s JOIN response_receipts r ON r.snapshot_id = s.id
            WHERE r.id = ?`, receiptID).Scan(&status, &finalized, &normalized); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("recheck receipt before normalization: %w", err)
		}
		if finalized != 0 {
			return ErrFinalized
		}
		if status != StatusInProgress {
			return fmt.Errorf("%w: snapshot status is %s", ErrIncompleteSnapshot, status)
		}
		if normalized != 0 {
			return ErrAlreadyNormalized
		}
		now := utcText(time.Now().UTC())
		for _, metric := range page.Metrics {
			flags, err := flagsJSON(metric.Flags)
			if err != nil {
				return fmt.Errorf("encode metric flags: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO keyword_metrics
                (id, snapshot_id, receipt_id, endpoint, returned_text, submitted_text,
                 submitted_index, variant_group, linkage_status, average_monthly_searches,
                 average_monthly_searches_state, competition, competition_index,
                 low_bid_micros, high_bid_micros, average_cpc_micros, average_cpc_state,
                 annotations, raw_result, status, flags, created_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				metric.ID, snapshot.ID, receipt.ReceiptID, receipt.Endpoint, metric.ReturnedText,
				metric.SubmittedText, nullableInt(metric.SubmittedIndex), metric.VariantGroup,
				metric.LinkageStatus, nullableInt64(metric.AverageMonthlySearches), metric.AverageMonthlyState,
				metric.Competition, nullableInt64(metric.CompetitionIndex), nullableInt64(metric.LowBidMicros),
				nullableInt64(metric.HighBidMicros), nullableInt64(metric.AverageCPCMicros), metric.AverageCPCState,
				nullableBytes(metric.Annotations), metric.RawResult, metric.Status, string(flags), now); err != nil {
				return fmt.Errorf("insert normalized metric: %w", err)
			}
			for _, monthly := range metric.Monthly {
				if monthly.Eligibility != EligibilityEligible {
					continue
				}
				monthlyFlags := append([]string{}, metric.Flags...)
				monthlyFlags = append(monthlyFlags, monthly.Flags...)
				flags, err := flagsJSON(monthlyFlags)
				if err != nil {
					return fmt.Errorf("encode monthly flags: %w", err)
				}
				var month any
				if monthly.Month != "" {
					month = monthly.Month
				}
				if _, err := tx.ExecContext(ctx, `INSERT INTO monthly_volumes
                    (snapshot_id, receipt_id, metric_id, returned_text, month,
                     raw_month, raw_month_json, raw_year, raw_year_json, monthly_searches,
                     value_state, eligibility, flags, created_at)
                    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					snapshot.ID, receipt.ReceiptID, metric.ID, metric.ReturnedText, month,
					monthly.RawMonth, nullableBytes(monthly.RawMonthJSON), monthly.RawYear,
					nullableBytes(monthly.RawYearJSON), nullableInt64(monthly.MonthlySearches),
					monthly.ValueState, monthly.Eligibility, string(flags), now); err != nil {
					return fmt.Errorf("insert normalized monthly volume: %w", err)
				}
			}
		}
		for _, link := range page.Links {
			if link.MetricID == "" {
				return fmt.Errorf("%w: variant linkage has no metric ID", ErrInvalidResponse)
			}
			linkID, err := newID()
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO variant_links
                (id, snapshot_id, receipt_id, metric_id, submitted_index, submitted_text,
                 returned_text, relationship, variant_group, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				linkID, snapshot.ID, receipt.ReceiptID, link.MetricID, nullableInt(link.SubmittedIndex),
				link.SubmittedText, link.ReturnedText, link.Relationship, link.VariantGroup, now); err != nil {
				return fmt.Errorf("insert variant linkage: %w", err)
			}
		}
		requestedJSON, _ := marshalStrings(receipt.SubmittedKeywords)
		returnedJSON, _ := marshalStrings(page.ReturnedTerms)
		monthsJSON, _ := marshalStrings(page.ReturnedMonths)
		missingJSON, _ := marshalStrings(page.MissingMonths)
		unavailableJSON, _ := marshalStrings(page.UnavailableMonths)
		rawOnlyJSON, _ := marshalStrings(page.RawOnlyMonths)
		coverageStatus := page.Status
		if coverageStatus == "" {
			coverageStatus = "complete"
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO coverage
            (snapshot_id, receipt_id, endpoint, page_number, batch_number, attempt,
             requested_start, requested_end, requested_terms, returned_terms,
             returned_months, missing_months, unavailable_months, raw_only_months,
             requested_count, returned_count, returned_month_count, no_result,
             collection_status, complete, next_page_token, vendor_total_size, created_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			snapshot.ID, receipt.ReceiptID, receipt.Endpoint, receipt.PageNumber, receipt.BatchNumber,
			receipt.Attempt, snapshot.RequestedStart, snapshot.RequestedEnd, string(requestedJSON),
			string(returnedJSON), string(monthsJSON), string(missingJSON), string(unavailableJSON), string(rawOnlyJSON),
			len(receipt.SubmittedKeywords), len(page.Metrics), len(page.ReturnedMonths), boolInt(page.NoResult),
			coverageStatus, 1, page.NextPageToken, nullableInt64(page.VendorTotalSize), now); err != nil {
			return fmt.Errorf("insert coverage: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE response_receipts SET normalized = 1, normalized_at = ? WHERE id = ?`, now, receiptID); err != nil {
			return fmt.Errorf("mark receipt normalized: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit normalized receipt: %w", err)
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	result.CoverageRows = 1
	result.Complete = true
	return result, nil
}

func (s *Store) loadReceiptSnapshot(ctx context.Context, receiptID string) (storedReceipt, Snapshot, error) {
	var receipt storedReceipt
	var requestBody, submittedJSON, body []byte
	var requestPresent, bodyPresent, normalized int
	var fetchedAt string
	err := s.db.QueryRowContext(ctx, `SELECT id, snapshot_id, endpoint, page_token,
        page_number, batch_number, attempt, request_body, request_body_present,
        submitted_keywords, http_status, google_request_id, body, body_present,
        body_sha256, fetched_at, error_code, error_message, normalized
        FROM response_receipts WHERE id = ?`, receiptID).Scan(
		&receipt.ReceiptID, &receipt.SnapshotID, &receipt.Endpoint, &receipt.PageToken,
		&receipt.PageNumber, &receipt.BatchNumber, &receipt.Attempt, &requestBody,
		&requestPresent, &submittedJSON, &receipt.HTTPStatus, &receipt.GoogleRequestID,
		&body, &bodyPresent, &receipt.BodySHA256, &fetchedAt, &receipt.ErrorCode,
		&receipt.ErrorMessage, &normalized)
	if errors.Is(err, sql.ErrNoRows) {
		return storedReceipt{}, Snapshot{}, ErrNotFound
	}
	if err != nil {
		return storedReceipt{}, Snapshot{}, fmt.Errorf("load response receipt: %w", err)
	}
	receipt.RequestBody = cloneBytes(requestBody)
	receipt.RequestBodyPresent = requestPresent != 0
	if !receipt.RequestBodyPresent {
		receipt.RequestBody = nil
	}
	receipt.SubmittedKeywords, err = unmarshalStrings(submittedJSON)
	if err != nil {
		return storedReceipt{}, Snapshot{}, fmt.Errorf("decode receipt keywords: %w", err)
	}
	receipt.Body = cloneBytes(body)
	receipt.BodyPresent = bodyPresent != 0
	if !receipt.BodyPresent {
		receipt.Body = nil
	}
	receipt.FetchedAt, err = time.Parse(time.RFC3339Nano, fetchedAt)
	if err != nil {
		return storedReceipt{}, Snapshot{}, fmt.Errorf("decode receipt timestamp: %w", err)
	}
	receipt.Normalized = normalized != 0
	row := s.db.QueryRowContext(ctx, `SELECT `+snapshotColumns+` FROM snapshots WHERE id = ?`, receipt.SnapshotID)
	snapshot, err := scanSnapshot(row)
	if err != nil {
		return storedReceipt{}, Snapshot{}, err
	}
	return receipt, snapshot, nil
}

func (s *Store) recordNormalizationFailure(ctx context.Context, receipt storedReceipt, snapshot Snapshot, warnings []string) error {
	return s.withWriteLock(ctx, func() error {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin failed normalization transaction: %w", err)
		}
		defer func() { _ = tx.Rollback() }()
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM coverage WHERE receipt_id = ?`, receipt.ReceiptID).Scan(&exists); err != nil {
			return fmt.Errorf("check failed receipt coverage: %w", err)
		}
		if exists == 0 {
			requestedJSON, _ := marshalStrings(receipt.SubmittedKeywords)
			warningJSON, _ := flagsJSON(warnings)
			if _, err := tx.ExecContext(ctx, `INSERT INTO coverage
                (snapshot_id, receipt_id, endpoint, page_number, batch_number, attempt,
                 requested_start, requested_end, requested_terms, returned_terms,
                 returned_months, missing_months, unavailable_months, raw_only_months,
                 requested_count, returned_count, returned_month_count, no_result,
                 collection_status, complete, next_page_token, vendor_total_size, created_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, '[]', '[]', '[]', '[]', '[]', ?, 0, 0, 0, ?, 0, '', NULL, ?)`,
				snapshot.ID, receipt.ReceiptID, receipt.Endpoint, receipt.PageNumber, receipt.BatchNumber,
				receipt.Attempt, snapshot.RequestedStart, snapshot.RequestedEnd, string(requestedJSON),
				len(receipt.SubmittedKeywords), "failed:"+string(warningJSON), utcText(time.Now().UTC())); err != nil {
				return fmt.Errorf("insert failed receipt coverage: %w", err)
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit failed normalization: %w", err)
		}
		return nil
	})
}

func buildNormalizationPage(endpoint string, submitted []string, snapshot Snapshot, receiptID string, envelope responseEnvelope) (normalizationPage, error) {
	page := normalizationPage{Metrics: make([]metricRecord, 0, len(envelope.Results)), Links: []linkageRecord{}, ReturnedTerms: []string{}, ReturnedMonths: []string{}, MissingMonths: []string{}, UnavailableMonths: []string{}, RawOnlyMonths: []string{}, Warnings: []string{}, Status: "complete"}
	page.NoResult = len(envelope.Results) == 0
	for _, rawResult := range envelope.Results {
		metric, links, err := parseMetric(endpoint, submitted, rawResult, snapshot, receiptID)
		if err != nil {
			return normalizationPage{}, err
		}
		page.Metrics = append(page.Metrics, metric)
		page.Links = append(page.Links, links...)
		page.ReturnedTerms = append(page.ReturnedTerms, metric.ReturnedText)
		for _, monthly := range metric.Monthly {
			label := monthly.Month
			if label == "" {
				label = rawMonthStartText(monthly.RawYear, monthly.RawMonth)
			}
			if label != "" {
				if monthly.Eligibility == EligibilityRawOnly {
					page.RawOnlyMonths = append(page.RawOnlyMonths, label)
				} else {
					page.ReturnedMonths = append(page.ReturnedMonths, label)
					if monthly.MonthlySearches == nil {
						page.UnavailableMonths = append(page.UnavailableMonths, label)
					}
				}
			}
			page.Warnings = append(page.Warnings, monthly.Flags...)
		}
		page.Warnings = append(page.Warnings, metric.Flags...)
	}
	page.ReturnedMonths = sortedUnique(page.ReturnedMonths)
	page.UnavailableMonths = sortedUnique(page.UnavailableMonths)
	page.RawOnlyMonths = sortedUnique(page.RawOnlyMonths)
	page.ReturnedTerms = append([]string{}, page.ReturnedTerms...)
	page.MissingMonths = missingRequestedMonths(snapshot.RequestedStart, snapshot.RequestedEnd, page.ReturnedMonths)
	page.NextPageToken = envelope.NextPageToken
	if envelope.TotalSize != nil {
		value, state, flag := parseNullableInt64(envelope.TotalSize, true)
		if value != nil && state == "value" {
			page.VendorTotalSize = value
		} else if flag != "" {
			page.Warnings = append(page.Warnings, "invalid_total_size")
		}
	}
	if page.NoResult {
		page.Status = "empty"
	}
	return page, nil
}

func parseMetric(endpoint string, submitted []string, rawResult json.RawMessage, snapshot Snapshot, receiptID string) (metricRecord, []linkageRecord, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(rawResult, &object); err != nil {
		return metricRecord{}, nil, fmt.Errorf("%w: result is not an object: %v", ErrInvalidResponse, err)
	}
	textValue, textPresent := rawString(object["text"])
	flags := []string{}
	if !textPresent {
		flags = append(flags, "missing_returned_text")
	}
	metricJSON := object["keywordIdeaMetrics"]
	if len(metricJSON) == 0 {
		metricJSON = object["keywordMetrics"]
	}
	var metrics map[string]json.RawMessage
	if len(metricJSON) > 0 && string(metricJSON) != "null" {
		if err := json.Unmarshal(metricJSON, &metrics); err != nil {
			return metricRecord{}, nil, fmt.Errorf("%w: decode metrics for %q: %v", ErrInvalidResponse, textValue, err)
		}
	}
	closeVariants, err := parseStringArray(field(object, "closeVariants"))
	if err != nil {
		return metricRecord{}, nil, fmt.Errorf("%w: decode close variants for %q: %v", ErrInvalidResponse, textValue, err)
	}
	if len(closeVariants) == 0 {
		closeVariants, err = parseStringArray(field(object, "close_variants"))
		if err != nil {
			return metricRecord{}, nil, fmt.Errorf("%w: decode close variants for %q: %v", ErrInvalidResponse, textValue, err)
		}
	}
	avg, avgState, avgFlag := parseNullableInt64(field(metrics, "avgMonthlySearches"), hasField(metrics, "avgMonthlySearches"))
	if avgFlag != "" {
		flags = append(flags, "invalid_avg_monthly_searches")
	}
	competition := rawStringValue(field(metrics, "competition"))
	if avg != nil && *avg == 0 && strings.EqualFold(competition, "UNSPECIFIED") {
		flags = append(flags, "ambiguous_zero_unspecified")
	}
	competitionIndex, _, competitionFlag := parseNullableInt64(field(metrics, "competitionIndex"), hasField(metrics, "competitionIndex"))
	if competitionFlag != "" {
		flags = append(flags, "invalid_competition_index")
	}
	lowBid, _, lowFlag := parseNullableInt64(field(metrics, "lowTopOfPageBidMicros"), hasField(metrics, "lowTopOfPageBidMicros"))
	if lowFlag != "" {
		flags = append(flags, "invalid_low_bid_micros")
	}
	highBid, _, highFlag := parseNullableInt64(field(metrics, "highTopOfPageBidMicros"), hasField(metrics, "highTopOfPageBidMicros"))
	if highFlag != "" {
		flags = append(flags, "invalid_high_bid_micros")
	}
	averageCPC, averageCPCState, averageCPCFlag := parseNullableInt64(field(metrics, "averageCpcMicros"), hasField(metrics, "averageCpcMicros"))
	if !hasField(metrics, "averageCpcMicros") {
		averageCPC, averageCPCState, averageCPCFlag = parseNullableInt64(field(metrics, "averageCPCMicros"), hasField(metrics, "averageCPCMicros"))
	}
	if averageCPCFlag != "" {
		flags = append(flags, "invalid_average_cpc_micros")
	}
	annotations := cloneBytes(object["keywordAnnotations"])
	if annotations == nil {
		annotations = cloneBytes(object["keyword_annotations"])
	}
	linkageStatus := "UNKNOWN"
	variantGroup := ""
	var submittedText string
	var submittedIndex *int
	links := []linkageRecord{}
	if endpoint == EndpointIdeas {
		linkageStatus = "COMBINED_REQUEST"
		variantGroup = "ideas:" + receiptID
		for _, variant := range closeVariants {
			links = append(links, linkageRecord{SubmittedText: variant, ReturnedText: textValue, Relationship: "CLOSE_VARIANT", VariantGroup: variantGroup})
		}
	} else {
		submittedText, submittedIndex, linkageStatus = representativeLink(textValue, submitted)
		variantGroup = termVariantGroup(textValue)
		matched := make(map[int]bool, len(submitted))
		for index, term := range submitted {
			indexCopy := index
			relationship := "UNKNOWN"
			if term == textValue {
				relationship = "EXACT"
				matched[index] = true
			} else if normalizedTerm(term) == normalizedTerm(textValue) && normalizedTerm(term) != "" {
				relationship = "NORMALIZED"
				matched[index] = true
			}
			if relationship == "UNKNOWN" {
				continue
			}
			links = append(links, linkageRecord{SubmittedIndex: &indexCopy, SubmittedText: term, ReturnedText: textValue, Relationship: relationship, VariantGroup: variantGroup})
		}
		for _, variant := range closeVariants {
			indices := matchingSubmittedIndices(variant, submitted)
			link := linkageRecord{SubmittedText: variant, ReturnedText: textValue, Relationship: "CLOSE_VARIANT", VariantGroup: variantGroup}
			if len(indices) == 1 {
				index := indices[0]
				indexCopy := index
				link.SubmittedIndex = &indexCopy
				link.SubmittedText = submitted[index]
				matched[index] = true
			} else if len(indices) > 1 {
				link.Relationship = "UNKNOWN"
			}
			links = append(links, link)
		}
		for index, term := range submitted {
			if matched[index] {
				continue
			}
			indexCopy := index
			links = append(links, linkageRecord{SubmittedIndex: &indexCopy, SubmittedText: term, Relationship: "UNMATCHED", VariantGroup: variantGroup})
		}
	}
	monthly := []monthlyRecord{}
	var monthlyRaw []json.RawMessage
	if raw := field(metrics, "monthlySearchVolumes"); len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &monthlyRaw); err != nil {
			return metricRecord{}, nil, fmt.Errorf("%w: monthly volumes for %q: %v", ErrInvalidResponse, textValue, err)
		}
	}
	for _, rawMonth := range monthlyRaw {
		value, err := parseMonthly(snapshot, rawMonth)
		if err != nil {
			return metricRecord{}, nil, err
		}
		monthly = append(monthly, value)
	}
	metricID, err := newID()
	if err != nil {
		return metricRecord{}, nil, err
	}
	for index := range links {
		links[index].MetricID = metricID
	}
	return metricRecord{ID: metricID, ReturnedText: textValue, SubmittedText: submittedText,
		SubmittedIndex: submittedIndex, VariantGroup: variantGroup, LinkageStatus: linkageStatus,
		AverageMonthlySearches: avg, AverageMonthlyState: avgState, Competition: competition,
		CompetitionIndex: competitionIndex, LowBidMicros: lowBid, HighBidMicros: highBid,
		AverageCPCMicros: averageCPC, AverageCPCState: averageCPCState, Annotations: annotations,
		RawResult: cloneBytes(rawResult), Status: "ok", Flags: sortedUnique(flags), Monthly: monthly}, links, nil
}

func parseMonthly(snapshot Snapshot, raw json.RawMessage) (monthlyRecord, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return monthlyRecord{}, fmt.Errorf("%w: monthly volume is not an object: %v", ErrInvalidResponse, err)
	}
	monthRaw := field(object, "month")
	yearRaw := field(object, "year")
	monthDisplay := rawDisplay(monthRaw)
	yearDisplay := rawDisplay(yearRaw)
	month, monthOK, monthFlag := parseVendorMonth(monthRaw)
	yearValue, yearState, yearFlag := parseNullableInt64(yearRaw, hasField(object, "year"))
	searches, valueState, searchFlag := parseNullableInt64(field(object, "monthlySearches"), hasField(object, "monthlySearches"))
	flags := []string{}
	if monthFlag != "" {
		flags = append(flags, monthFlag)
	}
	if yearFlag != "" || yearState != "value" {
		flags = append(flags, "invalid_or_missing_year")
	}
	if searchFlag != "" {
		flags = append(flags, "invalid_monthly_searches")
	}
	if searches != nil && *searches < 0 {
		flags = append(flags, "negative_monthly_searches")
	}
	if monthOK && yearValue != nil && (*yearValue < 1 || *yearValue > int64(^uint(0)>>1)) {
		flags = append(flags, "invalid_year")
		monthOK = false
	}
	if monthOK && yearValue != nil {
		month.Year = int(*yearValue)
	}
	eligibility := EligibilityEligible
	monthText := ""
	if !monthOK || yearValue == nil {
		eligibility = EligibilityRawOnly
	} else {
		monthText = monthStartText(month)
		requestedStart, _ := ParseYearMonth(snapshot.RequestedStart)
		requestedEnd, _ := ParseYearMonth(snapshot.RequestedEnd)
		if compareMonth(month, currentMonth(time.Now().UTC())) >= 0 {
			eligibility = EligibilityRawOnly
			flags = append(flags, "open_or_future_month")
		} else if compareMonth(month, requestedStart) < 0 || compareMonth(month, requestedEnd) > 0 {
			eligibility = EligibilityRawOnly
			flags = append(flags, "outside_requested_range")
		}
	}
	return monthlyRecord{Month: monthText, RawMonth: monthDisplay, RawMonthJSON: cloneBytes(monthRaw),
		RawYear: yearDisplay, RawYearJSON: cloneBytes(yearRaw), Year: month.Year, MonthNumber: month.Number,
		MonthlySearches: searches, ValueState: valueState, Eligibility: eligibility,
		Flags: sortedUnique(flags)}, nil
}

func parseVendorMonth(raw json.RawMessage) (Month, bool, string) {
	if len(raw) == 0 || string(raw) == "null" {
		return Month{}, false, "missing_month"
	}
	value := strings.ToUpper(strings.TrimSpace(rawStringValue(raw)))
	if value == "" {
		value = strings.TrimSpace(string(raw))
	}
	if number, err := strconv.Atoi(value); err == nil {
		if number >= 2 && number <= 13 {
			return Month{Number: number - 1}, true, ""
		}
		return Month{}, false, "invalid_month_enum"
	}
	names := map[string]int{"JANUARY": 1, "FEBRUARY": 2, "MARCH": 3, "APRIL": 4, "MAY": 5, "JUNE": 6, "JULY": 7, "AUGUST": 8, "SEPTEMBER": 9, "OCTOBER": 10, "NOVEMBER": 11, "DECEMBER": 12}
	if number, ok := names[value]; ok {
		return Month{Number: number}, true, ""
	}
	return Month{}, false, "invalid_month"
}

func parseNullableInt64(raw json.RawMessage, present bool) (*int64, string, string) {
	if !present || len(raw) == 0 {
		return nil, "absent", ""
	}
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("null")) {
		return nil, "null", ""
	}
	var text string
	if len(trimmed) > 0 && trimmed[0] == '"' {
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return nil, "invalid", "invalid_int64"
		}
	} else {
		text = string(trimmed)
	}
	if text == "" {
		return nil, "invalid", "invalid_int64"
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return nil, "invalid", "invalid_int64"
	}
	return &value, "value", ""
}

func representativeLink(returned string, submitted []string) (string, *int, string) {
	exact := []int{}
	normalized := []int{}
	for index, term := range submitted {
		if term == returned {
			exact = append(exact, index)
		}
		if normalizedTerm(term) != "" && normalizedTerm(term) == normalizedTerm(returned) {
			normalized = append(normalized, index)
		}
	}
	if len(exact) == 1 {
		index := exact[0]
		return submitted[index], &index, "EXACT"
	}
	if len(exact) > 1 {
		return "", nil, "UNKNOWN"
	}
	if len(normalized) == 1 {
		index := normalized[0]
		return submitted[index], &index, "NORMALIZED"
	}
	return "", nil, "UNKNOWN"
}

func matchingSubmittedIndices(value string, submitted []string) []int {
	normalized := normalizedTerm(value)
	if normalized == "" {
		return []int{}
	}
	indices := []int{}
	for index, term := range submitted {
		if normalizedTerm(term) == normalized {
			indices = append(indices, index)
		}
	}
	return indices
}

func normalizedTerm(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func termVariantGroup(value string) string {
	return "term:" + bytesHash([]byte(normalizedTerm(value)))
}

func missingRequestedMonths(start, end string, returned []string) []string {
	from, errFrom := ParseYearMonth(start)
	to, errTo := ParseYearMonth(end)
	if errFrom != nil || errTo != nil {
		return []string{}
	}
	seen := make(map[string]struct{}, len(returned))
	for _, value := range returned {
		if parsed, err := ParseYearMonth(strings.TrimSuffix(value, "-01")); err == nil {
			seen[monthStartText(parsed)] = struct{}{}
		} else {
			seen[value] = struct{}{}
		}
	}
	missing := []string{}
	for _, month := range monthsBetween(from, to) {
		if _, ok := seen[monthStartText(month)]; !ok {
			missing = append(missing, monthStartText(month))
		}
	}
	return missing
}

func monthStartText(month Month) string {
	if value := month.String(); value != "" {
		return value + "-01"
	}
	return ""
}

func rawMonthStartText(rawYear, rawMonth string) string {
	if rawYear == "" || rawMonth == "" {
		return ""
	}
	month, err := ParseYearMonth(strings.TrimSpace(rawYear) + "-" + strings.TrimSpace(rawMonth))
	if err != nil {
		return strings.TrimSpace(rawYear) + "-" + strings.TrimSpace(rawMonth)
	}
	return monthStartText(month)
}

func field(object map[string]json.RawMessage, key string) json.RawMessage {
	if object == nil {
		return nil
	}
	return object[key]
}

func hasField(object map[string]json.RawMessage, key string) bool {
	_, ok := object[key]
	return ok
}

func rawString(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

func rawStringValue(raw json.RawMessage) string {
	if value, ok := rawString(raw); ok {
		return value
	}
	return strings.TrimSpace(string(raw))
}

func parseStringArray(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	if values == nil {
		return []string{}, nil
	}
	return values, nil
}

func rawDisplay(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	if value, ok := rawString(raw); ok {
		return value
	}
	return strings.TrimSpace(string(raw))
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}
