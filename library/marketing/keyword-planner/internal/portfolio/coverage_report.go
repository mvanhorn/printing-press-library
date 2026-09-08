// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package portfolio

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// CoverageOptions filters a saved snapshot's coverage report. Coverage always
// includes the explicit snapshot's persisted page and batch records, including
// failed attempts and no-result batches; Keyword only narrows the metric and
// matching page entries.
type CoverageOptions struct {
	Keyword string `json:"keyword"`
}

// KeywordCoverage reports the requested and returned month state for one
// returned metric group. It is metric-specific, so a complete sibling metric
// cannot mask a short or unavailable series.
type KeywordCoverage struct {
	MetricID       string   `json:"metric_id"`
	ReceiptID      string   `json:"receipt_id"`
	Endpoint       string   `json:"endpoint"`
	PageNumber     int      `json:"page_number"`
	BatchNumber    int      `json:"batch_number"`
	Attempt        int      `json:"attempt"`
	ReturnedText   string   `json:"returned_text"`
	SubmittedText  string   `json:"submitted_text"`
	SubmittedIndex *int     `json:"submitted_index"`
	VariantGroup   string   `json:"variant_group"`
	LinkageStatus  string   `json:"linkage_status"`
	Flags          []string `json:"flags"`

	RequestedTerms     []string `json:"requested_terms"`
	RequestedMonths    []string `json:"requested_months"`
	ReturnedMonths     []string `json:"returned_months"`
	MissingMonths      []string `json:"missing_months"`
	UnavailableMonths  []string `json:"unavailable_months"`
	RawOnlyMonths      []string `json:"raw_only_months"`
	RequestedCount     int      `json:"requested_count"`
	ReturnedCount      int      `json:"returned_count"`
	ReturnedMonthCount int      `json:"returned_month_count"`
	NoResult           bool     `json:"no_result"`
	CollectionStatus   string   `json:"collection_status"`
	Complete           bool     `json:"complete"`
	ShortWindow        bool     `json:"short_window"`
}

// CoverageReport combines metric-specific coverage with the original page or
// batch coverage records. Summary month arrays are unions for inspection only;
// consumers must use Keywords for per-metric completeness decisions.
type CoverageReport struct {
	Snapshot Snapshot          `json:"snapshot"`
	Keywords []KeywordCoverage `json:"keywords"`
	Pages    []CoverageView    `json:"pages"`

	RequestedTerms    []string `json:"requested_terms"`
	ReturnedTerms     []string `json:"returned_terms"`
	RequestedMonths   []string `json:"requested_months"`
	ReturnedMonths    []string `json:"returned_months"`
	MissingMonths     []string `json:"missing_months"`
	UnavailableMonths []string `json:"unavailable_months"`
	RawOnlyMonths     []string `json:"raw_only_months"`

	RequestedCount     int      `json:"requested_count"`
	ReturnedCount      int      `json:"returned_count"`
	ReturnedMonthCount int      `json:"returned_month_count"`
	NoResultBatches    int      `json:"no_result_batches"`
	IncompletePages    int      `json:"incomplete_pages"`
	ShortWindow        bool     `json:"short_window"`
	NoResult           bool     `json:"no_result"`
	Complete           bool     `json:"complete"`
	Status             string   `json:"status"`
	Warnings           []string `json:"warnings"`
}

type coverageMetric struct {
	MetricID       string
	ReceiptID      string
	Endpoint       string
	ReturnedText   string
	SubmittedText  string
	SubmittedIndex *int
	VariantGroup   string
	LinkageStatus  string
	RawResult      []byte
	Flags          []string
	Normalized     bool
}

type coverageMonth struct {
	Month           string
	MonthlySearches sql.NullInt64
	Eligibility     string
}

// CoverageReport reads only the local portfolio. It never pads, interpolates
// or infers missing months; raw-only anomalies stay in their coverage lists and
// are absent from normalized monthly rows by construction.
func (s *Store) CoverageReport(ctx context.Context, snapshotID string, opts CoverageOptions) (CoverageReport, error) {
	snapshot, err := s.loadReportSnapshot(ctx, snapshotID)
	if err != nil {
		return CoverageReport{}, err
	}
	requestedMonths, err := reportRequestedMonths(snapshot)
	if err != nil {
		return CoverageReport{}, err
	}
	pages, err := s.listCoverage(ctx, snapshot.ID)
	if err != nil {
		return CoverageReport{}, err
	}
	if pages == nil {
		pages = []CoverageView{}
	}
	metrics, err := s.loadCoverageMetrics(ctx, snapshot.ID)
	if err != nil {
		return CoverageReport{}, err
	}
	links, err := s.loadCoverageLinkTexts(ctx, snapshot.ID)
	if err != nil {
		return CoverageReport{}, err
	}
	monthRows, err := s.loadCoverageMonths(ctx, snapshot.ID)
	if err != nil {
		return CoverageReport{}, err
	}

	pageByReceipt := make(map[string]CoverageView, len(pages))
	for _, page := range pages {
		// A normalized receipt has one authoritative coverage row. Keep the
		// first row if malformed external data created duplicates, and let the
		// raw page list expose every persisted row to the caller.
		if _, exists := pageByReceipt[page.ReceiptID]; !exists {
			pageByReceipt[page.ReceiptID] = page
		}
	}

	keyword := strings.TrimSpace(opts.Keyword)
	result := CoverageReport{
		Snapshot:        snapshot,
		Keywords:        []KeywordCoverage{},
		Pages:           []CoverageView{},
		RequestedMonths: append([]string{}, requestedMonths...),
		Status:          snapshot.Status,
		Warnings:        []string{},
	}

	selectedReceipts := make(map[string]struct{})
	for _, metric := range metrics {
		if keyword != "" && !coverageMetricMatches(metric, links[metric.MetricID], keyword) {
			continue
		}
		page, pageOK := pageByReceipt[metric.ReceiptID]
		requestedTerms := []string{}
		if pageOK {
			requestedTerms = append(requestedTerms, page.RequestedTerms...)
		}
		if len(requestedTerms) == 0 {
			requestedTerms = reportSubmittedTerms(snapshot)
		}
		returnedMonths := []string{}
		unavailableMonths := []string{}
		for _, item := range monthRows[metric.MetricID] {
			if item.Eligibility != EligibilityEligible || item.Month == "" {
				continue
			}
			returnedMonths = append(returnedMonths, item.Month)
			if !item.MonthlySearches.Valid {
				unavailableMonths = append(unavailableMonths, item.Month)
			}
		}
		returnedMonths = sortedUnique(returnedMonths)
		unavailableMonths = sortedUnique(unavailableMonths)
		rawOnlyMonths, rawErr := rawOnlyMonthsForMetric(snapshot, metric.RawResult)
		if rawErr != nil {
			return CoverageReport{}, fmt.Errorf("decode raw-only coverage for metric %s: %w", metric.MetricID, rawErr)
		}
		missingMonths := missingRequestedMonths(snapshot.RequestedStart, snapshot.RequestedEnd, returnedMonths)
		shortWindow := len(returnedMonths) < len(requestedMonths)
		item := KeywordCoverage{
			MetricID:           metric.MetricID,
			ReceiptID:          metric.ReceiptID,
			Endpoint:           metric.Endpoint,
			ReturnedText:       metric.ReturnedText,
			SubmittedText:      metric.SubmittedText,
			SubmittedIndex:     metric.SubmittedIndex,
			VariantGroup:       metric.VariantGroup,
			LinkageStatus:      metric.LinkageStatus,
			Flags:              append([]string{}, metric.Flags...),
			RequestedTerms:     append([]string{}, requestedTerms...),
			RequestedMonths:    append([]string{}, requestedMonths...),
			ReturnedMonths:     returnedMonths,
			MissingMonths:      missingMonths,
			UnavailableMonths:  unavailableMonths,
			RawOnlyMonths:      rawOnlyMonths,
			RequestedCount:     len(requestedTerms),
			ReturnedCount:      1,
			ReturnedMonthCount: len(returnedMonths),
			ShortWindow:        shortWindow,
			Complete:           metric.Normalized && pageOK && page.Complete,
			CollectionStatus:   "unknown",
		}
		if pageOK {
			item.PageNumber = page.PageNumber
			item.BatchNumber = page.BatchNumber
			item.Attempt = page.Attempt
			item.NoResult = page.NoResult
			item.CollectionStatus = page.CollectionStatus
			selectedReceipts[metric.ReceiptID] = struct{}{}
		}
		result.Keywords = append(result.Keywords, item)
	}

	for _, page := range pages {
		if keyword != "" {
			if _, selected := selectedReceipts[page.ReceiptID]; !selected && !coveragePageMatches(page, keyword) {
				continue
			}
		}
		result.Pages = append(result.Pages, page)
		result.ReturnedMonths = append(result.ReturnedMonths, page.ReturnedMonths...)
		result.MissingMonths = append(result.MissingMonths, page.MissingMonths...)
		result.UnavailableMonths = append(result.UnavailableMonths, page.UnavailableMonths...)
		result.RawOnlyMonths = append(result.RawOnlyMonths, page.RawOnlyMonths...)
		result.ReturnedTerms = append(result.ReturnedTerms, page.ReturnedTerms...)
		result.RequestedTerms = appendUniqueOrdered(result.RequestedTerms, page.RequestedTerms...)
	}
	if keyword == "" && len(result.Pages) == 0 {
		result.Pages = []CoverageView{}
	}
	result.ReturnedMonths = sortedUnique(result.ReturnedMonths)
	result.ReturnedTerms = appendUniqueOrdered(nil, result.ReturnedTerms...)
	result.MissingMonths = sortedUnique(result.MissingMonths)
	result.UnavailableMonths = sortedUnique(result.UnavailableMonths)
	result.RawOnlyMonths = sortedUnique(result.RawOnlyMonths)
	result.RequestedCount = len(result.RequestedTerms)
	result.ReturnedCount = len(result.Keywords)
	result.ReturnedMonthCount = len(result.ReturnedMonths)
	result.NoResultBatches = countNoResultGroups(result.Pages)
	result.IncompletePages = countIncompleteGroups(result.Pages)
	result.NoResult = result.NoResultBatches > 0
	result.ShortWindow = len(result.ReturnedMonths) < len(result.RequestedMonths)
	for _, keywordCoverage := range result.Keywords {
		if keywordCoverage.ShortWindow {
			result.ShortWindow = true
			break
		}
	}
	result.Complete = snapshot.Complete && result.IncompletePages == 0
	if result.ShortWindow {
		result.Warnings = append(result.Warnings, "short_window")
	}
	if result.NoResult {
		result.Warnings = append(result.Warnings, "no_result_batch")
	}
	if !result.Complete {
		result.Warnings = append(result.Warnings, "incomplete_collection")
	}
	result.Warnings = sortedUnique(result.Warnings)
	return result, nil
}

func reportRequestedMonths(snapshot Snapshot) ([]string, error) {
	from, err := ParseYearMonth(snapshot.RequestedStart)
	if err != nil {
		return nil, err
	}
	to, err := ParseYearMonth(snapshot.RequestedEnd)
	if err != nil {
		return nil, err
	}
	months := make([]string, 0, len(monthsBetween(from, to)))
	for _, month := range monthsBetween(from, to) {
		months = append(months, monthStartText(month))
	}
	return months, nil
}

func reportSubmittedTerms(snapshot Snapshot) []string {
	if snapshot.Endpoint == EndpointIdeas && len(snapshot.SubmittedSeeds) > 0 {
		return append([]string{}, snapshot.SubmittedSeeds...)
	}
	if len(snapshot.SubmittedKeywords) > 0 {
		return append([]string{}, snapshot.SubmittedKeywords...)
	}
	return append([]string{}, snapshot.SubmittedSeeds...)
}

func (s *Store) loadCoverageMetrics(ctx context.Context, snapshotID string) ([]coverageMetric, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.receipt_id, m.endpoint, m.returned_text, m.submitted_text,
		       m.submitted_index, m.variant_group, m.linkage_status, m.raw_result,
		       m.flags, r.normalized
		FROM keyword_metrics AS m
		JOIN response_receipts AS r ON r.id = m.receipt_id
		WHERE m.snapshot_id = ?
		ORDER BY m.returned_text, m.id`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("list coverage metrics: %w", err)
	}
	defer rows.Close()
	result := make([]coverageMetric, 0)
	for rows.Next() {
		var item coverageMetric
		var submittedIndex sql.NullInt64
		var rawResult, flagsJSON []byte
		var normalized int
		if err := rows.Scan(&item.MetricID, &item.ReceiptID, &item.Endpoint, &item.ReturnedText,
			&item.SubmittedText, &submittedIndex, &item.VariantGroup, &item.LinkageStatus,
			&rawResult, &flagsJSON, &normalized); err != nil {
			return nil, fmt.Errorf("scan coverage metric: %w", err)
		}
		if submittedIndex.Valid {
			value := int(submittedIndex.Int64)
			item.SubmittedIndex = &value
		}
		item.RawResult = cloneBytes(rawResult)
		item.Flags, err = parseFlags(flagsJSON)
		if err != nil {
			return nil, fmt.Errorf("decode coverage metric flags: %w", err)
		}
		item.Normalized = normalized != 0
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read coverage metrics: %w", err)
	}
	return result, nil
}

type coverageLinkText struct {
	SubmittedText string
	ReturnedText  string
}

func (s *Store) loadCoverageLinkTexts(ctx context.Context, snapshotID string) (map[string][]coverageLinkText, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT metric_id, submitted_text, returned_text
		FROM variant_links
		WHERE snapshot_id = ?
		ORDER BY metric_id, id`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("list coverage variant texts: %w", err)
	}
	defer rows.Close()
	result := make(map[string][]coverageLinkText)
	for rows.Next() {
		var metricID, submittedText, returnedText string
		if err := rows.Scan(&metricID, &submittedText, &returnedText); err != nil {
			return nil, fmt.Errorf("scan coverage variant text: %w", err)
		}
		result[metricID] = append(result[metricID], coverageLinkText{SubmittedText: submittedText, ReturnedText: returnedText})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read coverage variant texts: %w", err)
	}
	return result, nil
}

func (s *Store) loadCoverageMonths(ctx context.Context, snapshotID string) (map[string][]coverageMonth, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT metric_id, month, monthly_searches, eligibility
		FROM monthly_volumes
		WHERE snapshot_id = ?
		ORDER BY metric_id, month, id`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("list coverage months: %w", err)
	}
	defer rows.Close()
	result := make(map[string][]coverageMonth)
	for rows.Next() {
		var metricID string
		var month sql.NullString
		var item coverageMonth
		if err := rows.Scan(&metricID, &month, &item.MonthlySearches, &item.Eligibility); err != nil {
			return nil, fmt.Errorf("scan coverage month: %w", err)
		}
		if month.Valid {
			item.Month = month.String
		}
		result[metricID] = append(result[metricID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read coverage months: %w", err)
	}
	return result, nil
}

func coverageMetricMatches(metric coverageMetric, links []coverageLinkText, keyword string) bool {
	if containsFold(metric.ReturnedText, keyword) || containsFold(metric.SubmittedText, keyword) {
		return true
	}
	for _, link := range links {
		if containsFold(link.SubmittedText, keyword) || containsFold(link.ReturnedText, keyword) {
			return true
		}
	}
	return false
}

func coveragePageMatches(page CoverageView, keyword string) bool {
	for _, value := range page.RequestedTerms {
		if containsFold(value, keyword) {
			return true
		}
	}
	for _, value := range page.ReturnedTerms {
		if containsFold(value, keyword) {
			return true
		}
	}
	return false
}

func containsFold(value, needle string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(needle))
}

func rawOnlyMonthsForMetric(snapshot Snapshot, rawResult []byte) ([]string, error) {
	if len(rawResult) == 0 {
		return []string{}, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(rawResult, &object); err != nil {
		return nil, err
	}
	metricsRaw := object["keywordIdeaMetrics"]
	if len(metricsRaw) == 0 {
		metricsRaw = object["keywordMetrics"]
	}
	if len(metricsRaw) == 0 || string(metricsRaw) == "null" {
		return []string{}, nil
	}
	var metrics map[string]json.RawMessage
	if err := json.Unmarshal(metricsRaw, &metrics); err != nil {
		return nil, err
	}
	monthlyRaw := metrics["monthlySearchVolumes"]
	if len(monthlyRaw) == 0 {
		monthlyRaw = metrics["monthly_search_volumes"]
	}
	if len(monthlyRaw) == 0 || string(monthlyRaw) == "null" {
		return []string{}, nil
	}
	var volumes []json.RawMessage
	if err := json.Unmarshal(monthlyRaw, &volumes); err != nil {
		return nil, err
	}
	result := []string{}
	for _, raw := range volumes {
		monthly, err := parseMonthly(snapshot, raw)
		if err != nil {
			return nil, err
		}
		if monthly.Eligibility != EligibilityRawOnly {
			continue
		}
		label := monthly.Month
		if label == "" {
			label = rawMonthStartText(monthly.RawYear, monthly.RawMonth)
		}
		if label != "" {
			result = append(result, label)
		}
	}
	return sortedUnique(result), nil
}

func appendUniqueOrdered(values []string, additions ...string) []string {
	if values == nil {
		values = []string{}
	}
	seen := make(map[string]struct{}, len(values)+len(additions))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	for _, value := range additions {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values
}

func countNoResultGroups(pages []CoverageView) int {
	seen := map[string]struct{}{}
	for _, page := range pages {
		if !page.NoResult {
			continue
		}
		seen[coverageGroupKey(page)] = struct{}{}
	}
	return len(seen)
}

func countIncompleteGroups(pages []CoverageView) int {
	groups := make(map[string]bool)
	for _, page := range pages {
		key := coverageGroupKey(page)
		if _, exists := groups[key]; !exists {
			groups[key] = false
		}
		if page.Complete {
			groups[key] = true
		}
	}
	incomplete := 0
	for _, complete := range groups {
		if !complete {
			incomplete++
		}
	}
	return incomplete
}

func coverageGroupKey(page CoverageView) string {
	return fmt.Sprintf("%s:%d:%d", page.Endpoint, page.PageNumber, page.BatchNumber)
}
