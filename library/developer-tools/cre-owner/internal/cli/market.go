package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/cre-owner/internal/store"

	"github.com/spf13/cobra"
)

func newMarketCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		market string
	)

	cmd := &cobra.Command{
		Use:   "market [--market market-name]",
		Short: "Market-level aggregation — listing volume, median values, ownership concentration",
		Long: `Aggregates local store data by market to show market health metrics including
total parcels, listing volume, recent sales, median assessed values,
tax delinquency rate, top owners by building count, and ownership concentration.`,
		Example: strings.Trim(`
  cre-owner-pp-cli market --market "Chicago" --json
  cre-owner-pp-cli market --market "Dallas" --select total_parcels,median_sale_price
  cre-owner-pp-cli market --market "Miami" --compact`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if market == "" {
				return fmt.Errorf("--market is required")
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cre-owner-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			result, err := aggregateMarket(db, market)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&market, "market", "", "Market name to analyze (required)")
	return cmd
}

type marketResult struct {
	Market                 string          `json:"market"`
	TotalParcels           int             `json:"total_parcels"`
	TotalListings          int             `json:"total_listings"`
	TotalSales12Mo         int             `json:"total_sales_12mo"`
	MedianAssessedValue    float64         `json:"median_assessed_value"`
	MedianSalePrice        float64         `json:"median_sale_price"`
	DelinquencyRate        float64         `json:"delinquency_rate"`
	TopOwners              []topOwnerEntry `json:"top_owners"`
	OwnershipConcentration float64         `json:"ownership_concentration"`
}

type topOwnerEntry struct {
	Owner string `json:"owner"`
	Count int    `json:"count"`
}

func aggregateMarket(db *store.Store, market string) (*marketResult, error) {
	result := &marketResult{Market: market}
	marketFilter := "%" + market + "%"

	// Count parcels in market
	var parcelCount int
	err := db.DB().QueryRow(
		`SELECT COUNT(*) FROM resources WHERE resource_type = 'parcels'
		 AND (LOWER(json_extract(data, '$.market')) LIKE LOWER(?)
		   OR LOWER(json_extract(data, '$.submarket')) LIKE LOWER(?))`,
		marketFilter, marketFilter,
	).Scan(&parcelCount)
	if err != nil {
		parcelCount = 0
	}
	result.TotalParcels = parcelCount

	// Count listings in market
	var listingCount int
	err = db.DB().QueryRow(
		`SELECT COUNT(*) FROM resources WHERE resource_type = 'listings'
		 AND (LOWER(json_extract(data, '$.market')) LIKE LOWER(?)
		   OR LOWER(json_extract(data, '$.submarket')) LIKE LOWER(?))`,
		marketFilter, marketFilter,
	).Scan(&listingCount)
	if err != nil {
		listingCount = 0
	}
	result.TotalListings = listingCount

	// Count sales in market
	var salesCount int
	err = db.DB().QueryRow(
		`SELECT COUNT(*) FROM resources WHERE resource_type = 'sales'
		 AND (LOWER(json_extract(data, '$.market')) LIKE LOWER(?)
		   OR LOWER(json_extract(data, '$.submarket')) LIKE LOWER(?))`,
		marketFilter, marketFilter,
	).Scan(&salesCount)
	if err != nil {
		salesCount = 0
	}
	result.TotalSales12Mo = salesCount

	// Median assessed value from tax records
	result.MedianAssessedValue = computeMedian(db,
		`SELECT json_extract(data, '$.assessed_value') FROM resources
		 WHERE resource_type = 'tax_records'
		 AND json_extract(data, '$.assessed_value') IS NOT NULL
		 AND (LOWER(json_extract(data, '$.market')) LIKE LOWER(?)
		   OR LOWER(json_extract(data, '$.submarket')) LIKE LOWER(?))`,
		marketFilter, marketFilter,
	)

	// Median sale price from sales
	result.MedianSalePrice = computeMedian(db,
		`SELECT json_extract(data, '$.sale_price') FROM resources
		 WHERE resource_type = 'sales'
		 AND json_extract(data, '$.sale_price') IS NOT NULL
		 AND (LOWER(json_extract(data, '$.market')) LIKE LOWER(?)
		   OR LOWER(json_extract(data, '$.submarket')) LIKE LOWER(?))`,
		marketFilter, marketFilter,
	)

	// Delinquency rate: count tax-delinquent parcels / total parcels
	if parcelCount > 0 {
		var delinquentCount int
		err = db.DB().QueryRow(
			`SELECT COUNT(*) FROM resources WHERE resource_type = 'tax_records'
			 AND (json_extract(data, '$.delinquent') = true
			   OR json_extract(data, '$.delinquent') = 1
			   OR LOWER(json_extract(data, '$.status')) = 'delinquent')
			 AND (LOWER(json_extract(data, '$.market')) LIKE LOWER(?)
			   OR LOWER(json_extract(data, '$.submarket')) LIKE LOWER(?))`,
			marketFilter, marketFilter,
		).Scan(&delinquentCount)
		if err == nil && parcelCount > 0 {
			result.DelinquencyRate = math.Round(float64(delinquentCount)/float64(parcelCount)*10000) / 100
		}
	}

	// Top owners by building count
	ownerCounts := map[string]int{}
	rows, err := db.DB().Query(
		`SELECT data FROM resources WHERE resource_type = 'parcels'
		 AND (LOWER(json_extract(data, '$.market')) LIKE LOWER(?)
		   OR LOWER(json_extract(data, '$.submarket')) LIKE LOWER(?))`,
		marketFilter, marketFilter,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var data string
			if rows.Scan(&data) != nil {
				continue
			}
			obj := parseJSON(data)
			if obj == nil {
				continue
			}
			owner := extractStringField(obj, "owner", "owner_name", "ownerName")
			if owner != "" {
				ownerCounts[owner]++
			}
		}
	}

	// Sort owners by count
	type ownerCount struct {
		name  string
		count int
	}
	var sorted []ownerCount
	for name, count := range ownerCounts {
		sorted = append(sorted, ownerCount{name, count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	topN := 10
	if len(sorted) < topN {
		topN = len(sorted)
	}
	for _, oc := range sorted[:topN] {
		result.TopOwners = append(result.TopOwners, topOwnerEntry{
			Owner: oc.name,
			Count: oc.count,
		})
	}

	// Ownership concentration: % of parcels held by top 10 owners
	if parcelCount > 0 && len(sorted) > 0 {
		topCount := 0
		for _, oc := range sorted[:topN] {
			topCount += oc.count
		}
		result.OwnershipConcentration = math.Round(float64(topCount)/float64(parcelCount)*10000) / 100
	}

	return result, nil
}

func computeMedian(db *store.Store, query string, args ...any) float64 {
	rows, err := db.DB().Query(query, args...)
	if err != nil {
		return 0
	}
	defer rows.Close()

	var values []float64
	for rows.Next() {
		var val *float64
		if rows.Scan(&val) != nil || val == nil {
			continue
		}
		values = append(values, *val)
	}
	if len(values) == 0 {
		return 0
	}
	sort.Float64s(values)
	mid := len(values) / 2
	if len(values)%2 == 0 {
		return math.Round((values[mid-1]+values[mid])/2*100) / 100
	}
	return math.Round(values[mid]*100) / 100
}
