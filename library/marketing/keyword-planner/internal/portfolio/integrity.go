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
	"sort"
	"strings"
	"time"
)

// ErrIntegrity is returned when a read-only integrity check finds persisted
// evidence drift or a broken raw-to-normalized linkage.
var ErrIntegrity = errors.New("portfolio integrity check failed")

// IntegrityIssue is an actionable, secret-free integrity finding. Details never
// contain raw response bodies; the receipt ID and hashes are sufficient to find
// the evidence in an offline portfolio view.
type IntegrityIssue struct {
	Check  string `json:"check"`
	Table  string `json:"table"`
	ID     string `json:"id"`
	Detail string `json:"detail"`
}

// IntegrityCheck summarizes one read-only check family.
type IntegrityCheck struct {
	Name       string `json:"name"`
	Passed     bool   `json:"passed"`
	IssueCount int    `json:"issue_count"`
}

// IntegrityResult is returned for both valid and invalid portfolios. Invalid
// results also return an *IntegrityError so CLI callers can produce a nonzero
// exit while retaining the structured findings.
type IntegrityResult struct {
	Snapshot      Snapshot         `json:"snapshot"`
	Valid         bool             `json:"valid"`
	Checks        []IntegrityCheck `json:"checks"`
	Issues        []IntegrityIssue `json:"issues"`
	ReceiptCount  int              `json:"receipt_count"`
	MetricCount   int              `json:"metric_count"`
	MonthlyCount  int              `json:"monthly_count"`
	CoverageCount int              `json:"coverage_count"`
	VariantCount  int              `json:"variant_count"`
	AccountEvents int              `json:"account_event_count"`
}

// IntegrityReport is a descriptive alias for IntegrityResult.
type IntegrityReport = IntegrityResult

// IntegrityError identifies a failed integrity result without embedding raw
// database values or response bodies in the error string.
type IntegrityError struct {
	SnapshotID string
	Issues     []IntegrityIssue
}

func (e *IntegrityError) Error() string {
	if e == nil {
		return ErrIntegrity.Error()
	}
	if len(e.Issues) == 0 {
		return ErrIntegrity.Error()
	}
	return fmt.Sprintf("%s: %d issue(s), first %s", ErrIntegrity, len(e.Issues), e.Issues[0].Detail)
}

func (e *IntegrityError) Unwrap() error { return ErrIntegrity }

// Integrity checks one exact ID or the latest/previous alias. It performs only
// SQLite reads and recomputes the normalized projection from each committed raw
// receipt; it never repairs or rewrites the portfolio.
func (s *Store) Integrity(ctx context.Context, alias string) (IntegrityResult, error) {
	id, err := s.ResolveSnapshot(ctx, alias)
	if err != nil {
		return IntegrityResult{}, err
	}
	snapshot, err := s.snapshotForDiff(ctx, id)
	if err != nil {
		return IntegrityResult{}, err
	}
	result := IntegrityResult{Snapshot: snapshot, Checks: []IntegrityCheck{}, Issues: []IntegrityIssue{}}
	issueCounts := make(map[string]int)
	addIssue := func(check, table, objectID, detail string) {
		result.Issues = append(result.Issues, IntegrityIssue{Check: check, Table: table, ID: objectID, Detail: detail})
		issueCounts[check]++
	}

	validateIntegritySnapshot(ctx, s, snapshot, addIssue)

	data, err := loadIntegrityData(ctx, s, id)
	if err != nil {
		return result, err
	}
	result.ReceiptCount = len(data.Receipts)
	result.MetricCount = len(data.Metrics)
	result.MonthlyCount = len(data.Monthly)
	result.CoverageCount = len(data.Coverage)
	result.VariantCount = len(data.Links)
	result.AccountEvents = len(data.Accounts)

	receipts := make(map[string]integrityReceipt, len(data.Receipts))
	for _, receipt := range data.Receipts {
		receipts[receipt.ReceiptID] = receipt
		validateIntegrityReceipt(snapshot, receipt, addIssue)
	}
	metrics := make(map[string]integrityMetric, len(data.Metrics))
	for _, metric := range data.Metrics {
		metrics[metric.ID] = metric
		validateIntegrityMetric(snapshot, receipts, metric, addIssue)
	}
	for _, monthly := range data.Monthly {
		validateIntegrityMonthly(snapshot, receipts, metrics, monthly, addIssue)
	}
	for _, link := range data.Links {
		validateIntegrityLink(snapshot, receipts, metrics, link, addIssue)
	}
	for _, coverage := range data.Coverage {
		validateIntegrityCoverage(snapshot, receipts, coverage, addIssue)
	}
	for _, account := range data.Accounts {
		validateIntegrityAccount(snapshot, receipts, account, addIssue)
	}

	monthlyByMetric := make(map[string][]integrityMonthly)
	linksByReceipt := make(map[string][]integrityLink)
	coverageByReceipt := make(map[string][]integrityCoverage)
	metricsByReceipt := make(map[string][]integrityMetric)
	for _, monthly := range data.Monthly {
		monthlyByMetric[monthly.MetricID] = append(monthlyByMetric[monthly.MetricID], monthly)
	}
	for _, link := range data.Links {
		linksByReceipt[link.ReceiptID] = append(linksByReceipt[link.ReceiptID], link)
	}
	for _, coverage := range data.Coverage {
		coverageByReceipt[coverage.ReceiptID] = append(coverageByReceipt[coverage.ReceiptID], coverage)
	}
	for _, metric := range data.Metrics {
		metricsByReceipt[metric.ReceiptID] = append(metricsByReceipt[metric.ReceiptID], metric)
	}
	for _, receipt := range data.Receipts {
		if receipt.Normalized && receipt.Endpoint != EndpointAccount {
			verifyIntegrityProjection(snapshot, receipt, metricsByReceipt[receipt.ReceiptID], monthlyByMetric, linksByReceipt[receipt.ReceiptID], coverageByReceipt[receipt.ReceiptID], addIssue)
		}
	}

	validateIntegrityCompleteness(snapshot, data, coverageByReceipt, addIssue)
	for _, name := range []string{"snapshot_context", "receipt_hashes", "receipt_context", "normalized_metrics", "normalized_monthly", "variant_links", "coverage_links", "account_metadata", "raw_projection", "completeness"} {
		result.Checks = append(result.Checks, IntegrityCheck{Name: name, Passed: issueCounts[name] == 0, IssueCount: issueCounts[name]})
	}
	result.Valid = len(result.Issues) == 0
	if !result.Valid {
		issues := append([]IntegrityIssue(nil), result.Issues...)
		return result, &IntegrityError{SnapshotID: id, Issues: issues}
	}
	return result, nil
}

// CheckIntegrity is an explicit alias for callers that prefer a verb.
func (s *Store) CheckIntegrity(ctx context.Context, alias string) (IntegrityResult, error) {
	return s.Integrity(ctx, alias)
}

type integrityReceipt struct {
	ReceiptView
	RequestBodyPresent bool
	RequestBodyJSON    []byte
	BodyJSON           []byte
	FlagsError         error
}

type integrityMetric struct {
	ID                          string
	SnapshotID                  string
	ReceiptID                   string
	Endpoint                    string
	ReturnedText                string
	SubmittedText               string
	SubmittedIndex              *int
	VariantGroup                string
	LinkageStatus               string
	AverageMonthlySearches      *int64
	AverageMonthlySearchesState string
	Competition                 string
	CompetitionIndex            *int64
	LowBidMicros                *int64
	HighBidMicros               *int64
	AverageCPCMicros            *int64
	AverageCPCState             string
	Annotations                 []byte
	RawResult                   []byte
	Status                      string
	Flags                       []string
	FlagsJSON                   []byte
	FlagsError                  error
}

type integrityMonthly struct {
	ID              int64
	SnapshotID      string
	ReceiptID       string
	MetricID        string
	ReturnedText    string
	Month           sql.NullString
	RawMonth        string
	RawMonthJSON    []byte
	RawYear         string
	RawYearJSON     []byte
	MonthlySearches *int64
	ValueState      string
	Eligibility     string
	Flags           []string
	FlagsJSON       []byte
	FlagsError      error
}

type integrityLink struct {
	ID             string
	SnapshotID     string
	ReceiptID      string
	MetricID       string
	SubmittedIndex *int
	SubmittedText  string
	ReturnedText   string
	Relationship   string
	VariantGroup   string
}

type integrityCoverage struct {
	ID                 int64
	SnapshotID         string
	ReceiptID          string
	Endpoint           string
	PageNumber         int
	BatchNumber        int
	Attempt            int
	RequestedStart     string
	RequestedEnd       string
	RequestedTerms     []string
	ReturnedTerms      []string
	ReturnedMonths     []string
	MissingMonths      []string
	UnavailableMonths  []string
	RawOnlyMonths      []string
	RequestedCount     int
	ReturnedCount      int
	ReturnedMonthCount int
	NoResult           bool
	CollectionStatus   string
	Complete           bool
	NextPageToken      string
	VendorTotalSize    *int64
	RequestedJSON      []byte
	ReturnedJSON       []byte
	ReturnedMonthsJSON []byte
	MissingJSON        []byte
	UnavailableJSON    []byte
	RawOnlyJSON        []byte
	JSONErrors         []error
}

type integrityAccount struct {
	ID           string
	SnapshotID   string
	ReceiptID    string
	CurrencyCode string
	Source       string
}

type integrityData struct {
	Receipts []integrityReceipt
	Metrics  []integrityMetric
	Monthly  []integrityMonthly
	Links    []integrityLink
	Coverage []integrityCoverage
	Accounts []integrityAccount
}

func loadIntegrityData(ctx context.Context, s *Store, snapshotID string) (integrityData, error) {
	var data integrityData
	var err error
	if data.Receipts, err = loadIntegrityReceipts(ctx, s, snapshotID); err != nil {
		return integrityData{}, err
	}
	if data.Metrics, err = loadIntegrityMetrics(ctx, s, snapshotID); err != nil {
		return integrityData{}, err
	}
	if data.Monthly, err = loadIntegrityMonthly(ctx, s, snapshotID); err != nil {
		return integrityData{}, err
	}
	if data.Links, err = loadIntegrityLinks(ctx, s, snapshotID); err != nil {
		return integrityData{}, err
	}
	if data.Coverage, err = loadIntegrityCoverage(ctx, s, snapshotID); err != nil {
		return integrityData{}, err
	}
	if data.Accounts, err = loadIntegrityAccounts(ctx, s, snapshotID); err != nil {
		return integrityData{}, err
	}
	return data, nil
}

func loadIntegrityReceipts(ctx context.Context, s *Store, snapshotID string) ([]integrityReceipt, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, snapshot_id, endpoint, page_token,
		page_number, batch_number, attempt, request_body, request_body_present,
		submitted_keywords, http_status, google_request_id, body, body_present,
		body_sha256, fetched_at, error_code, error_message, normalized
		FROM response_receipts WHERE snapshot_id = ?
		ORDER BY page_number, batch_number, attempt, rowid`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("read integrity receipts: %w", err)
	}
	defer rows.Close()
	result := make([]integrityReceipt, 0)
	for rows.Next() {
		var item integrityReceipt
		var requestBody, submittedJSON, body []byte
		var requestPresent, bodyPresent, normalized int
		var fetchedAt string
		if err := rows.Scan(&item.ReceiptID, &item.SnapshotID, &item.Endpoint, &item.PageToken,
			&item.PageNumber, &item.BatchNumber, &item.Attempt, &requestBody, &requestPresent,
			&submittedJSON, &item.HTTPStatus, &item.GoogleRequestID, &body, &bodyPresent,
			&item.BodySHA256, &fetchedAt, &item.ErrorCode, &item.ErrorMessage, &normalized); err != nil {
			return nil, fmt.Errorf("scan integrity receipt: %w", err)
		}
		item.RequestBodyJSON = cloneBytes(requestBody)
		item.RequestBodyPresent = requestPresent != 0
		item.RequestBody = json.RawMessage(cloneBytes(requestBody))
		if !item.RequestBodyPresent {
			item.RequestBody = nil
		}
		item.SubmittedKeywords, err = unmarshalStrings(submittedJSON)
		if err != nil {
			return nil, fmt.Errorf("decode integrity receipt keywords: %w", err)
		}
		item.BodyJSON = cloneBytes(body)
		item.BodyPresent = bodyPresent != 0
		item.Body = cloneBytes(body)
		if !item.BodyPresent {
			item.Body = nil
		}
		item.FetchedAt, err = time.Parse(time.RFC3339Nano, fetchedAt)
		if err != nil {
			return nil, fmt.Errorf("decode integrity receipt timestamp: %w", err)
		}
		item.Normalized = normalized != 0
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate integrity receipts: %w", err)
	}
	return result, nil
}

func loadIntegrityMetrics(ctx context.Context, s *Store, snapshotID string) ([]integrityMetric, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, snapshot_id, receipt_id, endpoint,
		returned_text, submitted_text, submitted_index, variant_group, linkage_status,
		average_monthly_searches, average_monthly_searches_state, competition,
		competition_index, low_bid_micros, high_bid_micros, average_cpc_micros,
		average_cpc_state, annotations, raw_result, status, flags
		FROM keyword_metrics WHERE snapshot_id = ? ORDER BY receipt_id, returned_text, id`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("read integrity metrics: %w", err)
	}
	defer rows.Close()
	result := make([]integrityMetric, 0)
	for rows.Next() {
		var item integrityMetric
		var submittedIndex, average, competitionIndex, lowBid, highBid, averageCPC sql.NullInt64
		var annotations, flagsJSON []byte
		if err := rows.Scan(&item.ID, &item.SnapshotID, &item.ReceiptID, &item.Endpoint,
			&item.ReturnedText, &item.SubmittedText, &submittedIndex, &item.VariantGroup,
			&item.LinkageStatus, &average, &item.AverageMonthlySearchesState,
			&item.Competition, &competitionIndex, &lowBid, &highBid, &averageCPC,
			&item.AverageCPCState, &annotations, &item.RawResult, &item.Status, &flagsJSON); err != nil {
			return nil, fmt.Errorf("scan integrity metric: %w", err)
		}
		item.SubmittedIndex = nullInt(submittedIndex)
		item.AverageMonthlySearches = nullInt64(average)
		item.CompetitionIndex = nullInt64(competitionIndex)
		item.LowBidMicros = nullInt64(lowBid)
		item.HighBidMicros = nullInt64(highBid)
		item.AverageCPCMicros = nullInt64(averageCPC)
		item.Annotations = cloneBytes(annotations)
		item.RawResult = cloneBytes(item.RawResult)
		item.FlagsJSON = cloneBytes(flagsJSON)
		item.Flags, item.FlagsError = parseIntegrityFlags(flagsJSON)
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate integrity metrics: %w", err)
	}
	return result, nil
}

func loadIntegrityMonthly(ctx context.Context, s *Store, snapshotID string) ([]integrityMonthly, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, snapshot_id, receipt_id, metric_id,
		returned_text, month, raw_month, raw_month_json, raw_year, raw_year_json,
		monthly_searches, value_state, eligibility, flags
		FROM monthly_volumes WHERE snapshot_id = ? ORDER BY metric_id, month, id`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("read integrity monthly rows: %w", err)
	}
	defer rows.Close()
	result := make([]integrityMonthly, 0)
	for rows.Next() {
		var item integrityMonthly
		var rawMonthJSON, rawYearJSON, flagsJSON []byte
		var searches sql.NullInt64
		if err := rows.Scan(&item.ID, &item.SnapshotID, &item.ReceiptID, &item.MetricID,
			&item.ReturnedText, &item.Month, &item.RawMonth, &rawMonthJSON, &item.RawYear,
			&rawYearJSON, &searches, &item.ValueState, &item.Eligibility, &flagsJSON); err != nil {
			return nil, fmt.Errorf("scan integrity monthly row: %w", err)
		}
		item.RawMonthJSON = cloneBytes(rawMonthJSON)
		item.RawYearJSON = cloneBytes(rawYearJSON)
		item.MonthlySearches = nullInt64(searches)
		item.FlagsJSON = cloneBytes(flagsJSON)
		item.Flags, item.FlagsError = parseIntegrityFlags(flagsJSON)
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate integrity monthly rows: %w", err)
	}
	return result, nil
}

func loadIntegrityLinks(ctx context.Context, s *Store, snapshotID string) ([]integrityLink, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, snapshot_id, receipt_id, metric_id,
		submitted_index, submitted_text, returned_text, relationship, variant_group
		FROM variant_links WHERE snapshot_id = ? ORDER BY receipt_id, metric_id, id`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("read integrity variant links: %w", err)
	}
	defer rows.Close()
	result := make([]integrityLink, 0)
	for rows.Next() {
		var item integrityLink
		var submittedIndex sql.NullInt64
		if err := rows.Scan(&item.ID, &item.SnapshotID, &item.ReceiptID, &item.MetricID,
			&submittedIndex, &item.SubmittedText, &item.ReturnedText, &item.Relationship,
			&item.VariantGroup); err != nil {
			return nil, fmt.Errorf("scan integrity variant link: %w", err)
		}
		item.SubmittedIndex = nullInt(submittedIndex)
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate integrity variant links: %w", err)
	}
	return result, nil
}

func loadIntegrityCoverage(ctx context.Context, s *Store, snapshotID string) ([]integrityCoverage, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, snapshot_id, receipt_id, endpoint,
		page_number, batch_number, attempt, requested_start, requested_end,
		requested_terms, returned_terms, returned_months, missing_months,
		unavailable_months, raw_only_months, requested_count, returned_count,
		returned_month_count, no_result, collection_status, complete, next_page_token,
		vendor_total_size FROM coverage WHERE snapshot_id = ?
		ORDER BY page_number, batch_number, attempt, id`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("read integrity coverage: %w", err)
	}
	defer rows.Close()
	result := make([]integrityCoverage, 0)
	for rows.Next() {
		var item integrityCoverage
		var requestedJSON, returnedJSON, returnedMonthsJSON, missingJSON, unavailableJSON, rawOnlyJSON []byte
		var noResult, complete int
		var vendorTotal sql.NullInt64
		if err := rows.Scan(&item.ID, &item.SnapshotID, &item.ReceiptID, &item.Endpoint,
			&item.PageNumber, &item.BatchNumber, &item.Attempt, &item.RequestedStart,
			&item.RequestedEnd, &requestedJSON, &returnedJSON, &returnedMonthsJSON,
			&missingJSON, &unavailableJSON, &rawOnlyJSON, &item.RequestedCount,
			&item.ReturnedCount, &item.ReturnedMonthCount, &noResult, &item.CollectionStatus,
			&complete, &item.NextPageToken, &vendorTotal); err != nil {
			return nil, fmt.Errorf("scan integrity coverage: %w", err)
		}
		item.RequestedJSON = cloneBytes(requestedJSON)
		item.ReturnedJSON = cloneBytes(returnedJSON)
		item.ReturnedMonthsJSON = cloneBytes(returnedMonthsJSON)
		item.MissingJSON = cloneBytes(missingJSON)
		item.UnavailableJSON = cloneBytes(unavailableJSON)
		item.RawOnlyJSON = cloneBytes(rawOnlyJSON)
		item.RequestedTerms, item.ReturnedTerms, item.ReturnedMonths, item.MissingMonths, item.UnavailableMonths, item.RawOnlyMonths, item.JSONErrors = parseCoverageJSON(requestedJSON, returnedJSON, returnedMonthsJSON, missingJSON, unavailableJSON, rawOnlyJSON)
		item.NoResult = noResult != 0
		item.Complete = complete != 0
		item.VendorTotalSize = nullInt64(vendorTotal)
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate integrity coverage: %w", err)
	}
	return result, nil
}

func loadIntegrityAccounts(ctx context.Context, s *Store, snapshotID string) ([]integrityAccount, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, snapshot_id, receipt_id, currency_code, source
		FROM account_metadata WHERE snapshot_id = ? ORDER BY recorded_at, id`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("read integrity account events: %w", err)
	}
	defer rows.Close()
	result := make([]integrityAccount, 0)
	for rows.Next() {
		var item integrityAccount
		if err := rows.Scan(&item.ID, &item.SnapshotID, &item.ReceiptID, &item.CurrencyCode, &item.Source); err != nil {
			return nil, fmt.Errorf("scan integrity account event: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate integrity account events: %w", err)
	}
	return result, nil
}

func validateIntegritySnapshot(ctx context.Context, s *Store, snapshot Snapshot, add func(string, string, string, string)) {
	if _, err := NormalizeEndpoint(snapshot.Endpoint); err != nil {
		add("snapshot_context", "snapshots", snapshot.ID, "unsupported endpoint")
	}
	if snapshot.CustomerID == "" {
		add("snapshot_context", "snapshots", snapshot.ID, "customer ID is empty")
	}
	if snapshot.Language == "" {
		add("snapshot_context", "snapshots", snapshot.ID, "language resource is empty")
	}
	if snapshot.Network != NetworkGoogleSearch {
		add("snapshot_context", "snapshots", snapshot.ID, "network is not GOOGLE_SEARCH")
	}
	if expectedID, expectedGeos, err := geoSetID(snapshot.GeoTargetConstants); err != nil {
		add("snapshot_context", "snapshots", snapshot.ID, "geo target set is invalid")
	} else {
		if snapshot.GeoSetID != expectedID {
			add("snapshot_context", "snapshots", snapshot.ID, "geo_set_id does not match geo targets")
		}
		if !equalStrings(snapshot.GeoTargetConstants, expectedGeos) {
			add("snapshot_context", "snapshots", snapshot.ID, "geo targets are not stored as the canonical sorted set")
		}
	}
	if _, err := ParseYearMonth(snapshot.RequestedStart); err != nil {
		add("snapshot_context", "snapshots", snapshot.ID, "requested start is not YYYY-MM")
	}
	if _, err := ParseYearMonth(snapshot.RequestedEnd); err != nil {
		add("snapshot_context", "snapshots", snapshot.ID, "requested end is not YYYY-MM")
	}
	if snapshot.RequestedStart != "" && snapshot.RequestedEnd != "" {
		start, startErr := ParseYearMonth(snapshot.RequestedStart)
		end, endErr := ParseYearMonth(snapshot.RequestedEnd)
		if startErr == nil && endErr == nil && compareMonth(start, end) > 0 {
			add("snapshot_context", "snapshots", snapshot.ID, "requested start is after requested end")
		}
	}
	if snapshot.CurrencyCode != "" {
		if !validCurrency(snapshot.CurrencyCode) {
			add("snapshot_context", "snapshots", snapshot.ID, "currency code is not three uppercase letters")
		}
		if snapshot.CurrencySource == "" {
			add("snapshot_context", "snapshots", snapshot.ID, "currency source is empty while currency is present")
		}
	} else if snapshot.CurrencySource != "" {
		add("snapshot_context", "snapshots", snapshot.ID, "currency source is present without currency")
	}
	if snapshot.RequestBody != nil && !json.Valid(snapshot.RequestBody) {
		add("snapshot_context", "snapshots", snapshot.ID, "request body is not valid JSON")
	}
	if snapshot.Metadata != nil && !json.Valid(snapshot.Metadata) {
		add("snapshot_context", "snapshots", snapshot.ID, "metadata is not valid JSON")
	}
	var requestPresent int
	var requestBody []byte
	if err := s.db.QueryRowContext(ctx, `SELECT request_body_present, request_body FROM snapshots WHERE id = ?`, snapshot.ID).Scan(&requestPresent, &requestBody); err != nil {
		add("snapshot_context", "snapshots", snapshot.ID, "request body presence cannot be read")
	} else {
		if (requestPresent != 0) != (requestBody != nil) {
			add("snapshot_context", "snapshots", snapshot.ID, "request body presence flag disagrees with stored bytes")
		}
		if requestPresent != 0 && snapshot.RequestBodyHash != bytesHash(requestBody) {
			add("receipt_hashes", "snapshots", snapshot.ID, "request body hash does not match stored request bytes")
		}
		if requestPresent == 0 && snapshot.RequestBodyHash != "" {
			add("receipt_hashes", "snapshots", snapshot.ID, "request body hash is present while request body is absent")
		}
	}
	if snapshot.Complete && snapshot.Status != StatusComplete {
		add("completeness", "snapshots", snapshot.ID, "complete flag disagrees with status")
	}
	if snapshot.Status == StatusComplete && !snapshot.Complete {
		add("completeness", "snapshots", snapshot.ID, "complete status has false complete flag")
	}
	if snapshot.Finalized && snapshot.Status == StatusInProgress {
		add("completeness", "snapshots", snapshot.ID, "finalized snapshot remains in progress")
	}
	if !snapshot.Finalized && snapshot.FinishedAt != nil {
		add("completeness", "snapshots", snapshot.ID, "unfinalized snapshot has finished_at")
	}
}

func validateIntegrityReceipt(snapshot Snapshot, receipt integrityReceipt, add func(string, string, string, string)) {
	if receipt.SnapshotID != snapshot.ID {
		add("receipt_context", "response_receipts", receipt.ReceiptID, "receipt points to another snapshot")
	}
	endpoint, err := NormalizeEndpoint(receipt.Endpoint)
	if err != nil {
		add("receipt_context", "response_receipts", receipt.ReceiptID, "receipt endpoint is unsupported")
	} else if endpoint != EndpointAccount && endpoint != snapshot.Endpoint {
		add("receipt_context", "response_receipts", receipt.ReceiptID, "Planner receipt endpoint differs from snapshot endpoint")
	}
	if receipt.PageNumber < 0 || receipt.BatchNumber < 0 || receipt.Attempt < 0 {
		add("receipt_context", "response_receipts", receipt.ReceiptID, "negative page, batch, or attempt identity")
	}
	if receipt.BodySHA256 != bytesHash(receipt.BodyJSON) {
		add("receipt_hashes", "response_receipts", receipt.ReceiptID, "body hash does not match stored raw bytes")
	}
	if receipt.BodyPresent != (receipt.BodyJSON != nil) {
		add("receipt_hashes", "response_receipts", receipt.ReceiptID, "body presence flag disagrees with stored bytes")
	}
	if receipt.RequestBodyPresent != (receipt.RequestBodyJSON != nil) {
		add("receipt_context", "response_receipts", receipt.ReceiptID, "request body presence flag disagrees with stored bytes")
	}
	if receipt.RequestBodyPresent && !json.Valid(receipt.RequestBodyJSON) {
		add("receipt_context", "response_receipts", receipt.ReceiptID, "request body is not valid JSON")
	}
	if receipt.Normalized && (receipt.HTTPStatus < 200 || receipt.HTTPStatus >= 300 || !receipt.BodyPresent || len(receipt.BodyJSON) == 0) {
		add("normalized_metrics", "response_receipts", receipt.ReceiptID, "normalized receipt is not a nonempty successful response")
	}
}

func validateIntegrityMetric(snapshot Snapshot, receipts map[string]integrityReceipt, metric integrityMetric, add func(string, string, string, string)) {
	if metric.SnapshotID != snapshot.ID {
		add("normalized_metrics", "keyword_metrics", metric.ID, "metric points to another snapshot")
	}
	receipt, ok := receipts[metric.ReceiptID]
	if !ok {
		add("normalized_metrics", "keyword_metrics", metric.ID, "metric receipt does not exist")
	} else {
		if receipt.Endpoint != metric.Endpoint {
			add("normalized_metrics", "keyword_metrics", metric.ID, "metric endpoint differs from receipt endpoint")
		}
		if receipt.Endpoint == EndpointAccount {
			add("normalized_metrics", "keyword_metrics", metric.ID, "account receipt has normalized metric")
		}
	}
	if metric.Endpoint != snapshot.Endpoint {
		add("normalized_metrics", "keyword_metrics", metric.ID, "metric endpoint differs from snapshot endpoint")
	}
	if metric.FlagsError != nil {
		add("normalized_metrics", "keyword_metrics", metric.ID, "metric flags are not a JSON string array")
	}
	if len(metric.RawResult) == 0 || !json.Valid(metric.RawResult) {
		add("normalized_metrics", "keyword_metrics", metric.ID, "raw metric result is not valid JSON")
	}
}

func validateIntegrityMonthly(snapshot Snapshot, receipts map[string]integrityReceipt, metrics map[string]integrityMetric, monthly integrityMonthly, add func(string, string, string, string)) {
	if monthly.SnapshotID != snapshot.ID {
		add("normalized_monthly", "monthly_volumes", fmt.Sprint(monthly.ID), "monthly row points to another snapshot")
	}
	metric, ok := metrics[monthly.MetricID]
	if !ok {
		add("normalized_monthly", "monthly_volumes", fmt.Sprint(monthly.ID), "monthly row metric does not exist")
	} else {
		if metric.SnapshotID != monthly.SnapshotID || metric.ReceiptID != monthly.ReceiptID {
			add("normalized_monthly", "monthly_volumes", fmt.Sprint(monthly.ID), "monthly row metric and receipt linkage disagrees")
		}
		if metric.ReturnedText != monthly.ReturnedText {
			add("normalized_monthly", "monthly_volumes", fmt.Sprint(monthly.ID), "monthly returned text differs from metric")
		}
	}
	if _, ok := receipts[monthly.ReceiptID]; !ok {
		add("normalized_monthly", "monthly_volumes", fmt.Sprint(monthly.ID), "monthly row receipt does not exist")
	}
	if monthly.FlagsError != nil {
		add("normalized_monthly", "monthly_volumes", fmt.Sprint(monthly.ID), "monthly flags are not a JSON string array")
	}
	switch monthly.ValueState {
	case "value":
		if monthly.MonthlySearches == nil {
			add("normalized_monthly", "monthly_volumes", fmt.Sprint(monthly.ID), "value state has NULL monthly searches")
		}
	case "null", "absent", "invalid":
		if monthly.MonthlySearches != nil {
			add("normalized_monthly", "monthly_volumes", fmt.Sprint(monthly.ID), "non-value state has a numeric monthly search value")
		}
	default:
		add("normalized_monthly", "monthly_volumes", fmt.Sprint(monthly.ID), "unknown monthly value state")
	}
	if monthly.MonthlySearches != nil && *monthly.MonthlySearches < 0 && !hasIntegrityFlag(monthly.Flags, "negative_monthly_searches") {
		add("normalized_monthly", "monthly_volumes", fmt.Sprint(monthly.ID), "negative monthly value lacks its exclusion flag")
	}
	if monthly.Eligibility != EligibilityEligible && monthly.Eligibility != EligibilityRawOnly {
		add("normalized_monthly", "monthly_volumes", fmt.Sprint(monthly.ID), "unknown monthly eligibility")
	}
	if monthly.Eligibility == EligibilityEligible {
		if !monthly.Month.Valid {
			add("normalized_monthly", "monthly_volumes", fmt.Sprint(monthly.ID), "eligible monthly row has no ISO month")
		} else if _, err := ParseYearMonth(strings.TrimSuffix(monthly.Month.String, "-01")); err != nil {
			add("normalized_monthly", "monthly_volumes", fmt.Sprint(monthly.ID), "eligible monthly row has invalid ISO month")
		}
	}
}

func validateIntegrityLink(snapshot Snapshot, receipts map[string]integrityReceipt, metrics map[string]integrityMetric, link integrityLink, add func(string, string, string, string)) {
	if link.SnapshotID != snapshot.ID {
		add("variant_links", "variant_links", link.ID, "variant link points to another snapshot")
	}
	metric, ok := metrics[link.MetricID]
	if !ok {
		add("variant_links", "variant_links", link.ID, "variant link metric does not exist")
	} else if metric.SnapshotID != link.SnapshotID || metric.ReceiptID != link.ReceiptID {
		add("variant_links", "variant_links", link.ID, "variant link metric and receipt linkage disagrees")
	}
	if _, ok := receipts[link.ReceiptID]; !ok {
		add("variant_links", "variant_links", link.ID, "variant link receipt does not exist")
	}
	if link.Relationship == "" {
		add("variant_links", "variant_links", link.ID, "variant link relationship is empty")
	}
}

func validateIntegrityCoverage(snapshot Snapshot, receipts map[string]integrityReceipt, coverage integrityCoverage, add func(string, string, string, string)) {
	if coverage.SnapshotID != snapshot.ID {
		add("coverage_links", "coverage", fmt.Sprint(coverage.ID), "coverage points to another snapshot")
	}
	receipt, ok := receipts[coverage.ReceiptID]
	if !ok {
		add("coverage_links", "coverage", fmt.Sprint(coverage.ID), "coverage receipt does not exist")
	} else {
		if coverage.Endpoint != receipt.Endpoint {
			add("coverage_links", "coverage", fmt.Sprint(coverage.ID), "coverage endpoint differs from receipt endpoint")
		}
		if coverage.PageNumber != receipt.PageNumber || coverage.BatchNumber != receipt.BatchNumber || coverage.Attempt != receipt.Attempt {
			add("coverage_links", "coverage", fmt.Sprint(coverage.ID), "coverage page, batch, or attempt differs from receipt")
		}
	}
	if coverage.RequestedStart != snapshot.RequestedStart || coverage.RequestedEnd != snapshot.RequestedEnd {
		add("coverage_links", "coverage", fmt.Sprint(coverage.ID), "coverage requested range differs from snapshot")
	}
	for _, err := range coverage.JSONErrors {
		if err != nil {
			add("coverage_links", "coverage", fmt.Sprint(coverage.ID), "coverage term or month field is not a JSON string array")
			break
		}
	}
	if coverage.RequestedCount < 0 || coverage.ReturnedCount < 0 || coverage.ReturnedMonthCount < 0 {
		add("coverage_links", "coverage", fmt.Sprint(coverage.ID), "coverage count is negative")
	}
	if coverage.CollectionStatus == "" {
		add("coverage_links", "coverage", fmt.Sprint(coverage.ID), "coverage status is empty")
	}
	if coverage.Complete && strings.HasPrefix(coverage.CollectionStatus, "failed:") {
		add("coverage_links", "coverage", fmt.Sprint(coverage.ID), "failed coverage is marked complete")
	}
}

func validateIntegrityAccount(snapshot Snapshot, receipts map[string]integrityReceipt, account integrityAccount, add func(string, string, string, string)) {
	if account.SnapshotID != snapshot.ID {
		add("account_metadata", "account_metadata", account.ID, "account event points to another snapshot")
	}
	receipt, ok := receipts[account.ReceiptID]
	if !ok {
		add("account_metadata", "account_metadata", account.ID, "account event receipt does not exist")
	} else if receipt.Endpoint != EndpointAccount {
		add("account_metadata", "account_metadata", account.ID, "account event receipt is not an account endpoint")
	}
	if !validCurrency(account.CurrencyCode) {
		add("account_metadata", "account_metadata", account.ID, "account event currency is not three uppercase letters")
	}
	if strings.TrimSpace(account.Source) == "" {
		add("account_metadata", "account_metadata", account.ID, "account event source is empty")
	}
}

func validateIntegrityCompleteness(snapshot Snapshot, data integrityData, coverageByReceipt map[string][]integrityCoverage, add func(string, string, string, string)) {
	if !snapshot.Complete {
		return
	}
	plannerReceipts := 0
	for _, receipt := range data.Receipts {
		if receipt.Endpoint == EndpointIdeas || receipt.Endpoint == EndpointHistorical {
			plannerReceipts++
			if receipt.Normalized && len(coverageByReceipt[receipt.ReceiptID]) == 0 {
				add("completeness", "response_receipts", receipt.ReceiptID, "normalized Planner receipt has no coverage row")
			}
			if !receipt.Normalized {
				add("completeness", "response_receipts", receipt.ReceiptID, "complete snapshot contains an unnormalized Planner receipt")
			}
		}
	}
	if plannerReceipts == 0 {
		add("completeness", "snapshots", snapshot.ID, "complete snapshot has no Planner receipts")
	}
	for _, missing := range snapshot.MissingBatches {
		if strings.TrimSpace(missing) != "" {
			add("completeness", "snapshots", snapshot.ID, "complete snapshot lists missing batches")
			break
		}
	}
}

func verifyIntegrityProjection(snapshot Snapshot, receipt integrityReceipt, actualMetrics []integrityMetric, monthlyByMetric map[string][]integrityMonthly, actualLinks []integrityLink, actualCoverage []integrityCoverage, add func(string, string, string, string)) {
	var envelope responseEnvelope
	decoder := json.NewDecoder(bytes.NewReader(receipt.BodyJSON))
	if err := decoder.Decode(&envelope); err != nil {
		add("raw_projection", "response_receipts", receipt.ReceiptID, "normalized receipt body cannot be decoded")
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		add("raw_projection", "response_receipts", receipt.ReceiptID, "normalized receipt body contains trailing JSON")
		return
	} else if !errors.Is(err, io.EOF) {
		add("raw_projection", "response_receipts", receipt.ReceiptID, "normalized receipt body has malformed trailing JSON")
		return
	}
	page, err := buildNormalizationPage(receipt.Endpoint, receipt.SubmittedKeywords, snapshot, receipt.ReceiptID, envelope)
	if err != nil {
		add("raw_projection", "response_receipts", receipt.ReceiptID, "raw receipt cannot reproduce normalized projection")
		return
	}
	expectedMetrics := append([]metricRecord(nil), page.Metrics...)
	sort.SliceStable(expectedMetrics, func(i, j int) bool { return metricSortKey(expectedMetrics[i]) < metricSortKey(expectedMetrics[j]) })
	actual := append([]integrityMetric(nil), actualMetrics...)
	sort.SliceStable(actual, func(i, j int) bool { return metricSortKeyIntegrity(actual[i]) < metricSortKeyIntegrity(actual[j]) })
	if len(expectedMetrics) != len(actual) {
		add("raw_projection", "keyword_metrics", receipt.ReceiptID, fmt.Sprintf("metric count differs from raw response: got %d want %d", len(actual), len(expectedMetrics)))
	}
	metricCount := len(expectedMetrics)
	if len(actual) < metricCount {
		metricCount = len(actual)
	}
	for index := 0; index < metricCount; index++ {
		expected := expectedMetrics[index]
		got := actual[index]
		if !equalMetricProjection(expected, got) {
			add("raw_projection", "keyword_metrics", got.ID, "stored metric projection differs from raw response")
		}
		expectedMonthly := make([]monthlyRecord, 0, len(expected.Monthly))
		for _, monthly := range expected.Monthly {
			if monthly.Eligibility == EligibilityEligible {
				expectedMonthly = append(expectedMonthly, monthly)
			}
		}
		gotMonthly := append([]integrityMonthly(nil), monthlyByMetric[got.ID]...)
		sort.SliceStable(expectedMonthly, func(i, j int) bool { return monthlySortKey(expectedMonthly[i]) < monthlySortKey(expectedMonthly[j]) })
		sort.SliceStable(gotMonthly, func(i, j int) bool {
			return monthlySortKeyIntegrity(gotMonthly[i]) < monthlySortKeyIntegrity(gotMonthly[j])
		})
		if len(expectedMonthly) != len(gotMonthly) {
			add("raw_projection", "monthly_volumes", got.ID, fmt.Sprintf("monthly row count differs from raw response: got %d want %d", len(gotMonthly), len(expectedMonthly)))
		}
		monthCount := len(expectedMonthly)
		if len(gotMonthly) < monthCount {
			monthCount = len(gotMonthly)
		}
		for monthIndex := 0; monthIndex < monthCount; monthIndex++ {
			if !equalMonthlyProjection(expectedMonthly[monthIndex], gotMonthly[monthIndex]) {
				add("raw_projection", "monthly_volumes", fmt.Sprint(gotMonthly[monthIndex].ID), "stored monthly projection differs from raw response")
			}
		}
	}
	if !equalLinkProjection(page.Links, actualLinks) {
		add("raw_projection", "variant_links", receipt.ReceiptID, "stored variant linkage differs from raw response")
	}
	if len(actualCoverage) != 1 {
		add("raw_projection", "coverage", receipt.ReceiptID, fmt.Sprintf("normalized receipt has %d coverage rows; want one", len(actualCoverage)))
	} else if !equalCoverageProjection(page, actualCoverage[0], receipt) {
		add("raw_projection", "coverage", fmt.Sprint(actualCoverage[0].ID), "stored coverage differs from raw response")
	}
}

func metricSortKey(value metricRecord) string {
	return strings.Join([]string{value.ReturnedText, value.SubmittedText, nullableIndexText(value.SubmittedIndex), value.VariantGroup, string(value.RawResult)}, "\x00")
}

func metricSortKeyIntegrity(value integrityMetric) string {
	return strings.Join([]string{value.ReturnedText, value.SubmittedText, nullableIndexText(value.SubmittedIndex), value.VariantGroup, string(value.RawResult)}, "\x00")
}

func monthlySortKey(value monthlyRecord) string {
	return strings.Join([]string{value.Month, value.RawYear, value.RawMonth, string(value.RawMonthJSON), string(value.RawYearJSON)}, "\x00")
}

func monthlySortKeyIntegrity(value integrityMonthly) string {
	month := ""
	if value.Month.Valid {
		month = value.Month.String
	}
	return strings.Join([]string{month, value.RawYear, value.RawMonth, string(value.RawMonthJSON), string(value.RawYearJSON)}, "\x00")
}

func equalMetricProjection(expected metricRecord, got integrityMetric) bool {
	return expected.ReturnedText == got.ReturnedText && expected.SubmittedText == got.SubmittedText && equalIntPtr(expected.SubmittedIndex, got.SubmittedIndex) && expected.VariantGroup == got.VariantGroup && expected.LinkageStatus == got.LinkageStatus && equalInt64Ptr(expected.AverageMonthlySearches, got.AverageMonthlySearches) && expected.AverageMonthlyState == got.AverageMonthlySearchesState && expected.Competition == got.Competition && equalInt64Ptr(expected.CompetitionIndex, got.CompetitionIndex) && equalInt64Ptr(expected.LowBidMicros, got.LowBidMicros) && equalInt64Ptr(expected.HighBidMicros, got.HighBidMicros) && equalInt64Ptr(expected.AverageCPCMicros, got.AverageCPCMicros) && expected.AverageCPCState == got.AverageCPCState && bytes.Equal(expected.Annotations, got.Annotations) && bytes.Equal(expected.RawResult, got.RawResult) && expected.Status == got.Status && equalStrings(expected.Flags, got.Flags)
}

func equalMonthlyProjection(expected monthlyRecord, got integrityMonthly) bool {
	month := ""
	if got.Month.Valid {
		month = got.Month.String
	}
	return month == expected.Month && got.RawMonth == expected.RawMonth && bytes.Equal(got.RawMonthJSON, expected.RawMonthJSON) && got.RawYear == expected.RawYear && bytes.Equal(got.RawYearJSON, expected.RawYearJSON) && equalInt64Ptr(got.MonthlySearches, expected.MonthlySearches) && got.ValueState == expected.ValueState && got.Eligibility == expected.Eligibility && equalStrings(got.Flags, expected.Flags)
}

func equalLinkProjection(expected []linkageRecord, got []integrityLink) bool {
	type linkKey struct {
		index        string
		submitted    string
		returned     string
		relationship string
		variant      string
	}
	left := make([]linkKey, 0, len(expected))
	for _, link := range expected {
		left = append(left, linkKey{nullableIndexText(link.SubmittedIndex), link.SubmittedText, link.ReturnedText, link.Relationship, link.VariantGroup})
	}
	right := make([]linkKey, 0, len(got))
	for _, link := range got {
		right = append(right, linkKey{nullableIndexText(link.SubmittedIndex), link.SubmittedText, link.ReturnedText, link.Relationship, link.VariantGroup})
	}
	sort.Slice(left, func(i, j int) bool { return fmt.Sprint(left[i]) < fmt.Sprint(left[j]) })
	sort.Slice(right, func(i, j int) bool { return fmt.Sprint(right[i]) < fmt.Sprint(right[j]) })
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalCoverageProjection(page normalizationPage, got integrityCoverage, receipt integrityReceipt) bool {
	return got.SnapshotID == receipt.SnapshotID && got.ReceiptID == receipt.ReceiptID && got.Endpoint == receipt.Endpoint && got.PageNumber == receipt.PageNumber && got.BatchNumber == receipt.BatchNumber && got.Attempt == receipt.Attempt && equalStrings(got.RequestedTerms, receipt.SubmittedKeywords) && equalStrings(got.ReturnedTerms, page.ReturnedTerms) && equalStrings(got.ReturnedMonths, page.ReturnedMonths) && equalStrings(got.MissingMonths, page.MissingMonths) && equalStrings(got.UnavailableMonths, page.UnavailableMonths) && equalStrings(got.RawOnlyMonths, page.RawOnlyMonths) && got.RequestedCount == len(receipt.SubmittedKeywords) && got.ReturnedCount == len(page.Metrics) && got.ReturnedMonthCount == len(page.ReturnedMonths) && got.NoResult == page.NoResult && got.CollectionStatus == page.Status && got.Complete && got.NextPageToken == page.NextPageToken && equalInt64Ptr(got.VendorTotalSize, page.VendorTotalSize)
}

func parseIntegrityFlags(raw []byte) ([]string, error) {
	values, err := unmarshalStrings(raw)
	if err != nil {
		return nil, err
	}
	return sortedUnique(values), nil
}

func parseCoverageJSON(values ...[]byte) ([]string, []string, []string, []string, []string, []string, []error) {
	parsed := make([][]string, len(values))
	errorsFound := make([]error, len(values))
	for index, value := range values {
		parsed[index], errorsFound[index] = unmarshalStrings(value)
	}
	return parsed[0], parsed[1], parsed[2], parsed[3], parsed[4], parsed[5], errorsFound
}

func nullInt(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	result := int(value.Int64)
	return &result
}

func nullInt64(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

func equalIntPtr(left, right *int) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func hasIntegrityFlag(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func validCurrency(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, r := range value {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}
