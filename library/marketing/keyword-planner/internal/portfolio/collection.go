// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package portfolio

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// CollectionSummary is the bounded offline result used by the live collection
// command. Counts come from the portfolio tables and are independent of the
// rendered row limit. Raw response bodies are intentionally absent; callers
// that need the exact evidence should use ShowSnapshot explicitly.
type CollectionSummary struct {
	Snapshot           Snapshot       `json:"snapshot"`
	Rows               []Row          `json:"rows"`
	Coverage           []CoverageView `json:"coverage"`
	ReceiptCount       int            `json:"receipt_count"`
	StoredKeywordCount int            `json:"stored_keyword_count"`
	StoredMonthlyCount int            `json:"stored_monthly_count"`
	RenderedRowCount   int            `json:"rendered_row_count"`
	RenderLimit        int            `json:"render_limit"`
	RowsTruncated      bool           `json:"rows_truncated"`
}

// CollectionSummary resolves snapshotID (including latest/previous), loads
// only the requested normalized rows, and calculates stored counts with SQL
// COUNT queries. It never loads response bodies or an unlimited row set merely
// to calculate the counts. A zero limit preserves the Rows contract and means
// all normalized rows; callers that need a bounded response should pass a
// positive limit.
func (s *Store) CollectionSummary(ctx context.Context, snapshotID string, limit int) (CollectionSummary, error) {
	if limit < 0 {
		return CollectionSummary{}, fmt.Errorf("%w: collection row limit cannot be negative", ErrInvalidInput)
	}
	id, err := s.ResolveSnapshot(ctx, snapshotID)
	if err != nil {
		return CollectionSummary{}, err
	}

	var summary CollectionSummary
	row := s.db.QueryRowContext(ctx, `SELECT `+snapshotColumns+` FROM snapshots WHERE id = ?`, id)
	summary.Snapshot, err = scanSnapshot(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CollectionSummary{}, ErrNotFound
		}
		return CollectionSummary{}, fmt.Errorf("read collection snapshot: %w", err)
	}
	var currencyCode, currencySource string
	err = s.db.QueryRowContext(ctx, `
		SELECT currency_code, source
		FROM account_metadata
		WHERE snapshot_id = ?
		ORDER BY recorded_at DESC, id DESC
		LIMIT 1`, id).Scan(&currencyCode, &currencySource)
	if err == nil {
		// Project the latest append-only account event into the returned view;
		// this read path performs no metadata write or replacement.
		summary.Snapshot.CurrencyCode = currencyCode
		summary.Snapshot.CurrencySource = currencySource
	} else if !errors.Is(err, sql.ErrNoRows) {
		return CollectionSummary{}, fmt.Errorf("read collection currency metadata: %w", err)
	}

	summary.Rows, err = s.Rows(ctx, QueryOptions{
		SnapshotID:        id,
		IncludeIncomplete: true,
		Limit:             limit,
	})
	if err != nil {
		return CollectionSummary{}, err
	}
	summary.Coverage, err = s.listCoverage(ctx, id)
	if err != nil {
		return CollectionSummary{}, err
	}
	if summary.Rows == nil {
		summary.Rows = []Row{}
	}
	if summary.Coverage == nil {
		summary.Coverage = []CoverageView{}
	}

	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM response_receipts WHERE snapshot_id = ?`, id).
		Scan(&summary.ReceiptCount); err != nil {
		return CollectionSummary{}, fmt.Errorf("count collection receipts: %w", err)
	}
	// Count metric groups independently from monthly rows. A valid returned
	// keyword can have no monthly series, and it still belongs in the stored
	// keyword count (the live account currently exhibits this case).
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM keyword_metrics WHERE snapshot_id = ?`, id).
		Scan(&summary.StoredKeywordCount); err != nil {
		return CollectionSummary{}, fmt.Errorf("count collection keyword metrics: %w", err)
	}
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM monthly_volumes WHERE snapshot_id = ? AND eligibility = ?`, id, EligibilityEligible).
		Scan(&summary.StoredMonthlyCount); err != nil {
		return CollectionSummary{}, fmt.Errorf("count collection normalized rows: %w", err)
	}

	summary.RenderedRowCount = len(summary.Rows)
	summary.RenderLimit = limit
	summary.RowsTruncated = limit > 0 && summary.StoredMonthlyCount > summary.RenderedRowCount
	return summary, nil
}
