// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package portfolio

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// VariantsOptions filters a saved lineage report by a literal, case-insensitive
// term. The full submitted request remains in the report so a filtered result
// cannot be mistaken for a newly scoped API request.
type VariantsOptions struct {
	Keyword string `json:"keyword"`
}

// SubmittedTerm preserves the original request order. Ideas terms are shown
// for context but do not receive a fabricated per-seed relationship.
type SubmittedTerm struct {
	Index int    `json:"index"`
	Text  string `json:"text"`
	Kind  string `json:"kind"`
}

// VariantLink is one persisted submitted-to-returned relationship. A nil
// SubmittedIndex is intentional for combined Ideas links and ambiguous
// historical close-variant values.
type VariantLink struct {
	ID             string    `json:"id"`
	SnapshotID     string    `json:"snapshot_id"`
	ReceiptID      string    `json:"receipt_id"`
	MetricID       string    `json:"metric_id"`
	SubmittedIndex *int      `json:"submitted_index"`
	SubmittedText  string    `json:"submitted_text"`
	ReturnedText   string    `json:"returned_text"`
	Relationship   string    `json:"relationship"`
	VariantGroup   string    `json:"variant_group"`
	CreatedAt      time.Time `json:"created_at"`
}

// VariantMetric groups a returned metric with its lineage and monthly rows.
// Rows is one entry per metric/month and links are kept separate, so a close
// variant never multiplies a demand observation.
type VariantMetric struct {
	MetricID       string        `json:"metric_id"`
	SnapshotID     string        `json:"snapshot_id"`
	ReceiptID      string        `json:"receipt_id"`
	Endpoint       string        `json:"endpoint"`
	ReturnedText   string        `json:"returned_text"`
	CanonicalText  string        `json:"canonical_text"`
	SubmittedText  string        `json:"submitted_text"`
	SubmittedIndex *int          `json:"submitted_index"`
	VariantGroup   string        `json:"variant_group"`
	LinkageStatus  string        `json:"linkage_status"`
	Flags          []string      `json:"flags"`
	Rows           []Row         `json:"rows"`
	Links          []VariantLink `json:"links"`
}

// VariantsReport exposes the full submitted order and persisted relationships
// for one snapshot. Ideas explicitly report combined-request attribution as
// unknown; no per-seed allocation is inferred from a combined response.
type VariantsReport struct {
	Snapshot          Snapshot        `json:"snapshot"`
	Submitted         []SubmittedTerm `json:"submitted"`
	SubmittedSeeds    []string        `json:"submitted_seeds"`
	SubmittedKeywords []string        `json:"submitted_keywords"`
	Metrics           []VariantMetric `json:"metrics"`
	Links             []VariantLink   `json:"links"`
	CanonicalTerms    []string        `json:"canonical_terms"`
	MetricCount       int             `json:"metric_count"`
	MonthlyRowCount   int             `json:"monthly_row_count"`
	LinkCount         int             `json:"link_count"`
	ExactCount        int             `json:"exact_count"`
	NormalizedCount   int             `json:"normalized_count"`
	CloseVariantCount int             `json:"close_variant_count"`
	UnmatchedCount    int             `json:"unmatched_count"`
	UnknownCount      int             `json:"unknown_count"`
	CombinedRequest   bool            `json:"combined_request"`
	AttributionStatus string          `json:"attribution_status"`
	Warnings          []string        `json:"warnings"`
}

type variantMetricRecord struct {
	MetricID       string
	SnapshotID     string
	ReceiptID      string
	Endpoint       string
	ReturnedText   string
	SubmittedText  string
	SubmittedIndex *int
	VariantGroup   string
	LinkageStatus  string
	Flags          []string
}

// VariantsReport reads persisted lineage and normalized rows only. It returns
// unmatched and ambiguous relationships exactly as stored and never expands
// monthly rows for each linked variant.
func (s *Store) VariantsReport(ctx context.Context, snapshotID string, opts VariantsOptions) (VariantsReport, error) {
	snapshot, err := s.loadReportSnapshot(ctx, snapshotID)
	if err != nil {
		return VariantsReport{}, err
	}
	metrics, err := s.loadVariantMetrics(ctx, snapshot.ID)
	if err != nil {
		return VariantsReport{}, err
	}
	linksByMetric, err := s.loadVariantLinks(ctx, snapshot.ID)
	if err != nil {
		return VariantsReport{}, err
	}
	rows, err := s.Rows(ctx, QueryOptions{SnapshotID: snapshot.ID, IncludeIncomplete: true})
	if err != nil {
		return VariantsReport{}, err
	}
	rowsByMetric := make(map[string][]Row)
	for _, row := range rows {
		rowsByMetric[row.MetricID] = append(rowsByMetric[row.MetricID], row)
	}

	keyword := strings.TrimSpace(opts.Keyword)

	result := VariantsReport{
		Snapshot:          snapshot,
		Submitted:         submittedTermViews(snapshot),
		SubmittedSeeds:    append([]string{}, snapshot.SubmittedSeeds...),
		SubmittedKeywords: append([]string{}, snapshot.SubmittedKeywords...),
		Metrics:           []VariantMetric{},
		Links:             []VariantLink{},
		CanonicalTerms:    []string{},
		Warnings:          []string{},
		CombinedRequest:   snapshot.Endpoint == EndpointIdeas,
		AttributionStatus: "submitted_order_linkage",
	}
	if result.CombinedRequest {
		result.AttributionStatus = "combined_request_no_per_seed_attribution"
	}

	for _, metric := range metrics {
		selected := keyword == "" || variantMetricMatches(metric, linksByMetric[metric.MetricID], keyword)
		if !selected {
			continue
		}
		metricLinks := append([]VariantLink{}, linksByMetric[metric.MetricID]...)
		metricRows := append([]Row{}, rowsByMetric[metric.MetricID]...)
		if metricRows == nil {
			metricRows = []Row{}
		}
		if metricLinks == nil {
			metricLinks = []VariantLink{}
		}
		result.Metrics = append(result.Metrics, VariantMetric{
			MetricID:       metric.MetricID,
			SnapshotID:     metric.SnapshotID,
			ReceiptID:      metric.ReceiptID,
			Endpoint:       metric.Endpoint,
			ReturnedText:   metric.ReturnedText,
			CanonicalText:  metric.ReturnedText,
			SubmittedText:  metric.SubmittedText,
			SubmittedIndex: metric.SubmittedIndex,
			VariantGroup:   metric.VariantGroup,
			LinkageStatus:  metric.LinkageStatus,
			Flags:          append([]string{}, metric.Flags...),
			Rows:           metricRows,
			Links:          metricLinks,
		})
		result.CanonicalTerms = appendUniqueOrdered(result.CanonicalTerms, metric.ReturnedText)
		result.MonthlyRowCount += len(metricRows)
		result.Links = append(result.Links, metricLinks...)
	}

	for _, link := range result.Links {
		switch link.Relationship {
		case "EXACT":
			result.ExactCount++
		case "NORMALIZED":
			result.NormalizedCount++
		case "CLOSE_VARIANT":
			result.CloseVariantCount++
		case "UNMATCHED":
			result.UnmatchedCount++
		case "UNKNOWN":
			result.UnknownCount++
		}
	}
	result.MetricCount = len(result.Metrics)
	result.LinkCount = len(result.Links)
	if !snapshot.Complete {
		result.Warnings = append(result.Warnings, "incomplete_collection")
	}
	if keyword != "" && len(result.Metrics) == 0 {
		result.Warnings = append(result.Warnings, "no_variant_match")
	}
	result.Warnings = sortedUnique(result.Warnings)
	return result, nil
}

func submittedTermViews(snapshot Snapshot) []SubmittedTerm {
	terms := reportSubmittedTerms(snapshot)
	kind := "keyword"
	if snapshot.Endpoint == EndpointIdeas {
		kind = "seed"
	}
	result := make([]SubmittedTerm, 0, len(terms))
	for index, term := range terms {
		result = append(result, SubmittedTerm{Index: index, Text: term, Kind: kind})
	}
	return result
}

func (s *Store) loadVariantMetrics(ctx context.Context, snapshotID string) ([]variantMetricRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, snapshot_id, receipt_id, endpoint, returned_text,
		       submitted_text, submitted_index, variant_group, linkage_status, flags
		FROM keyword_metrics
		WHERE snapshot_id = ?
		ORDER BY returned_text, id`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("list variant metrics: %w", err)
	}
	defer rows.Close()
	result := make([]variantMetricRecord, 0)
	for rows.Next() {
		var item variantMetricRecord
		var submittedIndex sql.NullInt64
		var flagsJSON []byte
		if err := rows.Scan(&item.MetricID, &item.SnapshotID, &item.ReceiptID, &item.Endpoint,
			&item.ReturnedText, &item.SubmittedText, &submittedIndex, &item.VariantGroup,
			&item.LinkageStatus, &flagsJSON); err != nil {
			return nil, fmt.Errorf("scan variant metric: %w", err)
		}
		if submittedIndex.Valid {
			value := int(submittedIndex.Int64)
			item.SubmittedIndex = &value
		}
		item.Flags, err = parseFlags(flagsJSON)
		if err != nil {
			return nil, fmt.Errorf("decode variant metric flags: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read variant metrics: %w", err)
	}
	return result, nil
}

func (s *Store) loadVariantLinks(ctx context.Context, snapshotID string) (map[string][]VariantLink, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, snapshot_id, receipt_id, metric_id, submitted_index,
		       submitted_text, returned_text, relationship, variant_group, created_at
		FROM variant_links
		WHERE snapshot_id = ?
		ORDER BY metric_id, CASE WHEN submitted_index IS NULL THEN 1 ELSE 0 END,
		         submitted_index, created_at, id`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("list variant links: %w", err)
	}
	defer rows.Close()
	byMetric := make(map[string][]VariantLink)
	for rows.Next() {
		var item VariantLink
		var submittedIndex sql.NullInt64
		var createdAt string
		if err := rows.Scan(&item.ID, &item.SnapshotID, &item.ReceiptID, &item.MetricID,
			&submittedIndex, &item.SubmittedText, &item.ReturnedText, &item.Relationship,
			&item.VariantGroup, &createdAt); err != nil {
			return nil, fmt.Errorf("scan variant link: %w", err)
		}
		if submittedIndex.Valid {
			value := int(submittedIndex.Int64)
			item.SubmittedIndex = &value
		}
		var err error
		item.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("decode variant link timestamp: %w", err)
		}
		byMetric[item.MetricID] = append(byMetric[item.MetricID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read variant links: %w", err)
	}
	return byMetric, nil
}

func variantMetricMatches(metric variantMetricRecord, links []VariantLink, keyword string) bool {
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
