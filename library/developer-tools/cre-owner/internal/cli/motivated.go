package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/cre-owner/internal/store"

	"github.com/spf13/cobra"
)

type motivatedResult struct {
	ParcelID        string  `json:"parcel_id"`
	Address         string  `json:"address"`
	OwnerName       string  `json:"owner_name"`
	Market          string  `json:"market"`
	Score           int     `json:"score"`
	TaxDelinquent   bool    `json:"tax_delinquent"`
	LongHold        bool    `json:"long_hold"`
	OutOfState      bool    `json:"out_of_state_llc"`
	DormantEntity   bool    `json:"dormant_entity"`
	PortfolioStress bool    `json:"portfolio_distress"`
	AssessedValue   float64 `json:"assessed_value,omitempty"`
	LastSaleDate    string  `json:"last_sale_date,omitempty"`
}

func newMotivatedCmd(flags *rootFlags) *cobra.Command {
	var dbPath, market, signal string
	var limit, minScore int

	cmd := &cobra.Command{
		Use:   "motivated [--market market]",
		Short: "Ranked deal-sourcing list with motivated-seller signals",
		Example: strings.Trim(`
  cre-owner-pp-cli motivated --market lake-county-in --json
  cre-owner-pp-cli motivated --market lake-county-in --min-score 60
  cre-owner-pp-cli motivated --market lake-county-in --signal tax-delinquent`, "\n"),
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

			sqlDB := db.DB()

			// Load parcels for the market
			parcelRows, err := sqlDB.QueryContext(cmd.Context(),
				`SELECT id, content FROM resources WHERE resource_type = ? AND json_extract(content, '$.market') = ?`,
				"parcels", market)
			if err != nil {
				return fmt.Errorf("querying parcels: %w", err)
			}
			defer parcelRows.Close()

			type parcel struct {
				ID            string
				Address       string
				OwnerName     string
				AssessedValue float64
				LastSaleDate  string
				LastSalePrice float64
				Market        string
				PropertyType  string
			}

			var parcels []parcel
			for parcelRows.Next() {
				var id string
				var content []byte
				if err := parcelRows.Scan(&id, &content); err != nil {
					return fmt.Errorf("scanning parcel: %w", err)
				}
				var p struct {
					ParcelID      string  `json:"parcel_id"`
					Address       string  `json:"address"`
					OwnerName     string  `json:"owner_name"`
					AssessedValue float64 `json:"assessed_value"`
					LastSaleDate  string  `json:"last_sale_date"`
					LastSalePrice float64 `json:"last_sale_price"`
					Market        string  `json:"market"`
					PropertyType  string  `json:"property_type"`
				}
				if err := json.Unmarshal(content, &p); err != nil {
					continue
				}
				parcels = append(parcels, parcel{
					ID:            p.ParcelID,
					Address:       p.Address,
					OwnerName:     p.OwnerName,
					AssessedValue: p.AssessedValue,
					LastSaleDate:  p.LastSaleDate,
					LastSalePrice: p.LastSalePrice,
					Market:        p.Market,
					PropertyType:  p.PropertyType,
				})
			}
			if err := parcelRows.Err(); err != nil {
				return fmt.Errorf("iterating parcels: %w", err)
			}

			// Build delinquent parcel set and per-owner delinquent count
			delinquentParcels := map[string]bool{}
			taxRows, err := sqlDB.QueryContext(cmd.Context(),
				`SELECT content FROM resources WHERE resource_type = ?`, "tax_records")
			if err != nil {
				return fmt.Errorf("querying tax records: %w", err)
			}
			defer taxRows.Close()
			for taxRows.Next() {
				var content []byte
				if err := taxRows.Scan(&content); err != nil {
					continue
				}
				var tr struct {
					ParcelID   string `json:"parcel_id"`
					Delinquent bool   `json:"delinquent"`
				}
				if err := json.Unmarshal(content, &tr); err != nil {
					continue
				}
				if tr.Delinquent {
					delinquentParcels[tr.ParcelID] = true
				}
			}

			// Build entity status map
			entityStatus := map[string]string{} // entity name -> status
			entRows, err := sqlDB.QueryContext(cmd.Context(),
				`SELECT content FROM resources WHERE resource_type = ?`, "entities")
			if err != nil {
				return fmt.Errorf("querying entities: %w", err)
			}
			defer entRows.Close()
			for entRows.Next() {
				var content []byte
				if err := entRows.Scan(&content); err != nil {
					continue
				}
				var e struct {
					Name   string `json:"name"`
					Status string `json:"status"`
				}
				if err := json.Unmarshal(content, &e); err != nil {
					continue
				}
				entityStatus[strings.ToLower(e.Name)] = strings.ToLower(e.Status)
			}

			// Count delinquent parcels per owner
			ownerDelinquentCount := map[string]int{}
			for _, p := range parcels {
				if delinquentParcels[p.ID] {
					ownerDelinquentCount[strings.ToLower(p.OwnerName)]++
				}
			}

			// Score each parcel
			now := time.Now()
			var results []motivatedResult
			for _, p := range parcels {
				var score int
				var taxDelq, longHold, outState, dormant, portfolio bool

				// Tax delinquency (weight: 30)
				if delinquentParcels[p.ID] {
					taxDelq = true
					score += 30
				}

				// Long hold >10 years (weight: 20)
				if p.LastSaleDate != "" {
					if saleDate, err := time.Parse("2006-01-02", p.LastSaleDate); err == nil {
						if now.Sub(saleDate) > 10*365*24*time.Hour {
							longHold = true
							score += 20
						}
					}
				}

				// Out-of-state LLC (weight: 15)
				ownerLower := strings.ToLower(p.OwnerName)
				if strings.Contains(ownerLower, "llc") || strings.Contains(ownerLower, "l.l.c") {
					// Check if entity jurisdiction differs from market state
					if st, ok := entityStatus[ownerLower]; ok {
						_ = st // entity exists; check jurisdiction separately
					}
					// Heuristic: if owner name contains a state abbreviation different from market
					marketState := extractState(market)
					if marketState != "" && !strings.Contains(ownerLower, strings.ToLower(marketState)) {
						outState = true
						score += 15
					}
				}

				// Dormant/dissolved entity (weight: 15)
				if status, ok := entityStatus[ownerLower]; ok {
					if status != "active" && status != "" {
						dormant = true
						score += 15
					}
				}

				// Portfolio distress: multiple delinquent parcels by same owner (weight: 20)
				if ownerDelinquentCount[ownerLower] > 1 {
					portfolio = true
					score += 20
				}

				if score < minScore {
					continue
				}

				// Filter by signal type if specified
				if signal != "" {
					switch signal {
					case "tax-delinquent":
						if !taxDelq {
							continue
						}
					case "long-hold":
						if !longHold {
							continue
						}
					case "out-of-state":
						if !outState {
							continue
						}
					case "portfolio-distress":
						if !portfolio {
							continue
						}
					}
				}

				results = append(results, motivatedResult{
					ParcelID:        p.ID,
					Address:         p.Address,
					OwnerName:       p.OwnerName,
					Market:          p.Market,
					Score:           score,
					TaxDelinquent:   taxDelq,
					LongHold:        longHold,
					OutOfState:      outState,
					DormantEntity:   dormant,
					PortfolioStress: portfolio,
					AssessedValue:   p.AssessedValue,
					LastSaleDate:    p.LastSaleDate,
				})
			}

			// Sort by score descending
			for i := 0; i < len(results); i++ {
				for j := i + 1; j < len(results); j++ {
					if results[j].Score > results[i].Score {
						results[i], results[j] = results[j], results[i]
					}
				}
			}

			// Apply limit
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&market, "market", "", "Market/county to search (e.g., lake-county-in)")
	cmd.Flags().IntVar(&minScore, "min-score", 0, "Minimum motivated-seller score (0-100)")
	cmd.Flags().StringVar(&signal, "signal", "", "Filter by signal type: tax-delinquent, long-hold, out-of-state, portfolio-distress")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	return cmd
}

// extractState extracts the state abbreviation from a market slug like "lake-county-in".
func extractState(market string) string {
	parts := strings.Split(market, "-")
	if len(parts) >= 2 {
		last := parts[len(parts)-1]
		if len(last) == 2 {
			return strings.ToUpper(last)
		}
	}
	return ""
}
