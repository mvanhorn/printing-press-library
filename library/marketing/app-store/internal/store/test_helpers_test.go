// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"time"
)

// Test-only helpers: production code writes through RecordProductPage,
// RecordKeywordRankOnce and the other once-per-observation writers.

// RecordVersions inserts version entries that are not stored yet
// (append-only: the 25-entry window Apple shows grows into a full history).
func RecordVersions(ctx context.Context, st *Store, appID string, versions []appstore.VersionEntry, seenAt time.Time) error {
	return withWriteTx(ctx, st, func(tx *sql.Tx) error {
		return recordVersionsTx(ctx, tx, appID, versions, tsOrNow(seenAt))
	})
}

// RecordPrivacy stores a privacy label snapshot when it differs from the
// latest stored one for (app, storefront). It reports whether a row was written.
func RecordPrivacy(ctx context.Context, st *Store, storefront, appID string, p appstore.Privacy, at time.Time) (bool, error) {
	var changed bool
	err := withWriteTx(ctx, st, func(tx *sql.Tx) error {
		var err error
		changed, err = recordPrivacyTx(ctx, tx, storefront, appID, p, tsOrNow(at))
		return err
	})
	return changed, err
}

// RecordIAPs stores the in-app purchase price list when it differs from the
// latest stored list for (app, storefront). It reports whether rows were
// written. An empty list records nothing (the caller cannot tell an app
// without in-app purchases from a page that hid them; RecordProductPage
// records an empty list when the page positively showed none).
func RecordIAPs(ctx context.Context, st *Store, storefront, appID string, items []appstore.IAPItem, at time.Time) (bool, error) {
	var changed bool
	err := withWriteTx(ctx, st, func(tx *sql.Tx) error {
		var err error
		changed, err = recordIAPsTx(ctx, tx, storefront, appID, items, false, tsOrNow(at))
		return err
	})
	return changed, err
}

// RecordEvents upserts in-app events (first_seen_at kept, last_seen_at bumped).
func RecordEvents(ctx context.Context, st *Store, storefront, appID string, events []appstore.AppEvent, at time.Time) error {
	return withWriteTx(ctx, st, func(tx *sql.Tx) error {
		return recordEventsTx(ctx, tx, storefront, appID, events, tsOrNow(at))
	})
}

// ChartSeries returns an app's rank in a chart key over time, oldest first
// (captures where the app was absent are not included; use ChartCaptures to
// see every capture).
func ChartSeries(ctx context.Context, st *Store, k ChartKey, appID string) ([]RankPoint, error) {
	where, args := chartKeyWhere(k)
	args = append(args, appID)
	rows, err := st.db.QueryContext(ctx, `SELECT captured_at, rank, COALESCE(row_limit,0), source FROM chart_snapshots
		WHERE`+where+` AND app_id = ? ORDER BY captured_at, source`, args...)
	if err != nil {
		return nil, fmt.Errorf("query chart series: %w", err)
	}
	out := make([]RankPoint, 0)
	for rows.Next() {
		var p RankPoint
		var at string
		if err := rows.Scan(&at, &p.Rank, &p.RowLimit, &p.Source); err != nil {
			_ = rows.Close()
			return nil, err
		}
		p.CapturedAt = parseTS(at)
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}
