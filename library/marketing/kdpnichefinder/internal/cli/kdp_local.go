// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/marketing/kdpnichefinder/internal/kdpsource"
	"github.com/mvanhorn/printing-press-library/library/marketing/kdpnichefinder/internal/store"
)

// nicheRow is a fully-scanned niches row used by the local analytics commands.
type nicheRow struct {
	ID        string
	Title     string
	AmazonURL string
	Price     float64
	PriceStr  string
	Publisher string
	Sales     int
	Revenue   float64
	Bucket    string
	ASIN      string
}

// resolveKDPDBPath returns the db path honoring an explicit --db flag.
func resolveKDPDBPath(dbFlag string) string {
	if dbFlag != "" {
		return dbFlag
	}
	return defaultDBPath("kdpnichefinder-pp-cli")
}

// openKDPLocal opens the local mirror for reading and ensures the KDP schema
// exists. It returns (nil, nil, true) when the mirror file is missing, after
// emitting the standard missing-mirror guidance and (for machine output) an
// empty `[]` to stdout. Callers should `return nil` when missing is true.
func openKDPLocal(ctx context.Context, flags *rootFlags, dbFlag string, w interface{ Write([]byte) (int, error) }) (*store.Store, string, bool, error) {
	dbPath := resolveKDPDBPath(dbFlag)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "no local mirror at %s\nrun: kdpnichefinder-pp-cli refresh\n", dbPath)
		if flags.asJSON || flags.agent {
			_, _ = w.Write([]byte("[]\n"))
		}
		return nil, dbPath, true, nil
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, dbPath, false, err
	}
	if err := db.EnsureKDPSchema(ctx); err != nil {
		_ = db.Close()
		return nil, dbPath, false, err
	}
	return db, dbPath, false, nil
}

// loadNiches reads all niches rows (optionally filtered to a single bucket),
// NULL-safe, computing the parsed price and ASIN per row.
func loadNiches(ctx context.Context, db *store.Store, bucketFilter string) ([]nicheRow, error) {
	query := `SELECT id, title, amazon_url, price, publisher, estimated_monthly_sales, estimated_monthly_revenue, bucket FROM niches`
	args := []any{}
	if bucketFilter != "" {
		query += ` WHERE bucket = ?`
		args = append(args, bucketFilter)
	}
	rows, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]nicheRow, 0)
	for rows.Next() {
		var (
			id        sql.NullString
			title     sql.NullString
			amazonURL sql.NullString
			price     sql.NullString
			publisher sql.NullString
			sales     sql.NullInt64
			revenue   sql.NullFloat64
			bucket    sql.NullString
		)
		if err := rows.Scan(&id, &title, &amazonURL, &price, &publisher, &sales, &revenue, &bucket); err != nil {
			return nil, err
		}
		r := nicheRow{
			ID:        id.String,
			Title:     title.String,
			AmazonURL: amazonURL.String,
			PriceStr:  price.String,
			Publisher: publisher.String,
			Sales:     int(sales.Int64),
			Revenue:   revenue.Float64,
			Bucket:    bucket.String,
			ASIN:      kdpsource.ASIN(amazonURL.String),
		}
		r.Price = parsePrice(price.String)
		out = append(out, r)
	}
	return out, rows.Err()
}

// parsePrice converts a numeric price string to a float, 0 on failure.
func parsePrice(s string) float64 {
	var f float64
	if s == "" {
		return 0
	}
	if _, err := fmt.Sscanf(s, "%f", &f); err != nil {
		return 0
	}
	return f
}

// validateBucket returns a usage error when a non-empty --type is not one of
// the known niche buckets. Without this, a typo (e.g. "evergeen") flows into a
// `WHERE bucket = ?` predicate and silently returns an empty result with exit
// 0, which an agent reads as "no data" rather than "bad input".
func validateBucket(flagType string) error {
	if flagType == "" {
		return nil
	}
	for _, b := range kdpsource.Buckets {
		if b == flagType {
			return nil
		}
	}
	return usageErr(fmt.Errorf("unknown --type %q (valid: %v)", flagType, kdpsource.Buckets))
}
