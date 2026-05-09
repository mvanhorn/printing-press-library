// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package judicial

import (
	"context"
	"database/sql"
	"sort"
)

// SentenceStats summarises sentence rows for a given filter.
type SentenceStats struct {
	Statute        string            `json:"statute"`
	Court          string            `json:"court,omitempty"`
	Year           int               `json:"year,omitempty"`
	TotalCount     int               `json:"total_count"`
	PrisonCount    int               `json:"prison_count"`
	FineCount      int               `json:"fine_count"`
	LifeCount      int               `json:"life_prison_count"`
	DetentionCount int               `json:"detention_count"`
	ProbationCount int               `json:"probation_count"`
	PrisonMin      int               `json:"prison_min_months,omitempty"`
	PrisonMedian   int               `json:"prison_median_months,omitempty"`
	PrisonMax      int               `json:"prison_max_months,omitempty"`
	FineMedianNTD  int               `json:"fine_median_ntd,omitempty"`
	Histogram      []HistogramBucket `json:"prison_histogram_months,omitempty"`
}

// HistogramBucket is one row of the prison-months histogram.
type HistogramBucket struct {
	BucketStart int `json:"bucket_start_months"`
	BucketEnd   int `json:"bucket_end_months"`
	Count       int `json:"count"`
}

// AggregateSentences joins citations × sentences × judgments to produce
// statistics for a given statute, optionally narrowed by court and year.
//
// The query intentionally uses statute-level filtering (article is omitted)
// because Taiwan court 主文 patterns cite the same statute across articles.
func AggregateSentences(ctx context.Context, db *sql.DB, statute, court string, year int) (*SentenceStats, error) {
	q := `
		SELECT s.kind, s.prison_months, s.fine_ntd, s.probation
		FROM sentences s
		JOIN citations c ON c.jid = s.jid AND c.statute = ?`
	args := []any{statute}
	if court != "" {
		q += ` WHERE SUBSTR(s.jid, 1, 3) = ?`
		args = append(args, court)
	}
	if year > 0 {
		if court == "" {
			q += ` WHERE`
		} else {
			q += ` AND`
		}
		q += ` CAST(SUBSTR(s.jid, INSTR(s.jid, ',') + 1,
		           INSTR(SUBSTR(s.jid, INSTR(s.jid, ',') + 1), ',') - 1) AS INTEGER) = ?`
		args = append(args, year)
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stats := &SentenceStats{
		Statute: statute,
		Court:   court,
		Year:    year,
	}
	var prisonMonths, fines []int
	for rows.Next() {
		var kind string
		var pm, fine, prob int
		if err := rows.Scan(&kind, &pm, &fine, &prob); err != nil {
			return nil, err
		}
		stats.TotalCount++
		switch kind {
		case "imprisonment":
			stats.PrisonCount++
			if pm > 0 {
				prisonMonths = append(prisonMonths, pm)
			}
		case "life_prison":
			stats.LifeCount++
		case "fine":
			stats.FineCount++
			if fine > 0 {
				fines = append(fines, fine)
			}
		case "detention":
			stats.DetentionCount++
		case "probation":
			stats.ProbationCount++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(prisonMonths) > 0 {
		sort.Ints(prisonMonths)
		stats.PrisonMin = prisonMonths[0]
		stats.PrisonMax = prisonMonths[len(prisonMonths)-1]
		stats.PrisonMedian = prisonMonths[len(prisonMonths)/2]
		stats.Histogram = histogram(prisonMonths)
	}
	if len(fines) > 0 {
		sort.Ints(fines)
		stats.FineMedianNTD = fines[len(fines)/2]
	}
	return stats, nil
}

// histogram bins months into 6-month buckets up to 60 months, then 12-month
// buckets up to 120 months, then a final "120+" bucket.
func histogram(months []int) []HistogramBucket {
	if len(months) == 0 {
		return nil
	}
	buckets := []HistogramBucket{
		{BucketStart: 0, BucketEnd: 6},
		{BucketStart: 6, BucketEnd: 12},
		{BucketStart: 12, BucketEnd: 24},
		{BucketStart: 24, BucketEnd: 36},
		{BucketStart: 36, BucketEnd: 60},
		{BucketStart: 60, BucketEnd: 84},
		{BucketStart: 84, BucketEnd: 120},
		{BucketStart: 120, BucketEnd: 0}, // 0 = open-ended
	}
	for _, m := range months {
		for i := range buckets {
			b := buckets[i]
			if b.BucketEnd == 0 || (m >= b.BucketStart && m < b.BucketEnd) {
				buckets[i].Count++
				break
			}
		}
	}
	return buckets
}
