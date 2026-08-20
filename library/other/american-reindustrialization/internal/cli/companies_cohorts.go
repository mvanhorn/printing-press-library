// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/other/american-reindustrialization/internal/store"
	"github.com/spf13/cobra"
)

type cohortRow struct {
	BucketStart int    `json:"bucket_start"`
	BucketEnd   int    `json:"bucket_end"`
	Label       string `json:"label"`
	Companies   int    `json:"companies"`
	TopSectors  []kv   `json:"top_sectors"`
}

type kv struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

func newCompaniesCohortsCmd(flags *rootFlags) *cobra.Command {
	var bucket int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "cohorts",
		Short: "Bucket companies by founded_year with top sectors per cohort",
		Long: "GROUP BY bucketed founded_year over the locally synced companies. " +
			"Default bucket width is 5 years; pass --bucket to change. Top three sectors " +
			"per cohort are computed in-process.",
		Example:     "  american-reindustrialization-pp-cli companies cohorts --bucket 5 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if bucket <= 0 {
				bucket = 5
			}
			if dbPath == "" {
				dbPath = defaultDBPath("american-reindustrialization-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'american-reindustrialization-pp-cli sync' first.", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT founded_year, COALESCE(primary_sector, '')
				 FROM companies
				 WHERE founded_year IS NOT NULL AND founded_year > 1800`)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type bucketAccum struct {
				count   int
				sectors map[string]int
			}
			buckets := map[int]*bucketAccum{}
			for rows.Next() {
				var year sql.NullInt64
				var sector sql.NullString
				if err := rows.Scan(&year, &sector); err != nil {
					continue
				}
				if !year.Valid {
					continue
				}
				start := int(year.Int64/int64(bucket)) * bucket
				b := buckets[start]
				if b == nil {
					b = &bucketAccum{sectors: map[string]int{}}
					buckets[start] = b
				}
				b.count++
				if sector.String != "" {
					b.sectors[sector.String]++
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating cohorts rows: %w", err)
			}

			out := make([]cohortRow, 0, len(buckets))
			for start, b := range buckets {
				row := cohortRow{
					BucketStart: start,
					BucketEnd:   start + bucket - 1,
					Label:       fmt.Sprintf("%d-%d", start, start+bucket-1),
					Companies:   b.count,
					TopSectors:  topNSectors(b.sectors, 3),
				}
				out = append(out, row)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].BucketStart < out[j].BucketStart })
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().IntVar(&bucket, "bucket", 5, "Bucket width in years")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path override")
	return cmd
}

func topNSectors(m map[string]int, n int) []kv {
	out := make([]kv, 0, len(m))
	for k, v := range m {
		out = append(out, kv{Value: k, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Value < out[j].Value
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}
