package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/cre-owner/internal/store"

	"github.com/spf13/cobra"
)

type taxCountdownResult struct {
	ParcelID      string  `json:"parcel_id"`
	Address       string  `json:"address"`
	OwnerName     string  `json:"owner_name"`
	Market        string  `json:"market"`
	AmountOwed    float64 `json:"amount_owed"`
	DaysRemaining int     `json:"days_remaining"`
	Urgency       string  `json:"urgency"`
	AssessedValue float64 `json:"assessed_value,omitempty"`
}

func newTaxCountdownCmd(flags *rootFlags) *cobra.Command {
	var dbPath, market, within string
	var limit int

	cmd := &cobra.Command{
		Use:   "tax-countdown [--market market]",
		Short: "Properties approaching tax sale deadline, ranked by urgency",
		Example: strings.Trim(`
  cre-owner-pp-cli tax-countdown --market lake-county-in --json
  cre-owner-pp-cli tax-countdown --market lake-county-in --within 6mo
  cre-owner-pp-cli tax-countdown --market lake-county-in --within 1y --limit 20`, "\n"),
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

			// Parse --within window
			var withinDays int
			if within != "" {
				withinDays, err = parseWithin(within)
				if err != nil {
					return fmt.Errorf("invalid --within value %q: %w", within, err)
				}
			}

			sqlDB := db.DB()

			// Load delinquent tax records
			type taxRecord struct {
				TaxID          string  `json:"tax_id"`
				ParcelID       string  `json:"parcel_id"`
				AmountOwed     float64 `json:"amount_owed"`
				Delinquent     bool    `json:"delinquent"`
				DelinquentDate string  `json:"delinquent_date"`
			}

			taxRows, err := sqlDB.QueryContext(cmd.Context(),
				`SELECT content FROM resources WHERE resource_type = ?`, "tax_records")
			if err != nil {
				return fmt.Errorf("querying tax records: %w", err)
			}
			defer taxRows.Close()

			delinquentByParcel := map[string]taxRecord{}
			for taxRows.Next() {
				var content []byte
				if err := taxRows.Scan(&content); err != nil {
					continue
				}
				var tr taxRecord
				if err := json.Unmarshal(content, &tr); err != nil {
					continue
				}
				if tr.Delinquent {
					delinquentByParcel[tr.ParcelID] = tr
				}
			}

			// Load parcels for the market
			parcelRows, err := sqlDB.QueryContext(cmd.Context(),
				`SELECT content FROM resources WHERE resource_type = ? AND json_extract(content, '$.market') = ?`,
				"parcels", market)
			if err != nil {
				return fmt.Errorf("querying parcels: %w", err)
			}
			defer parcelRows.Close()

			now := time.Now()
			// Indiana redemption period: ~365 days after tax sale
			const redemptionDays = 365

			var results []taxCountdownResult
			for parcelRows.Next() {
				var content []byte
				if err := parcelRows.Scan(&content); err != nil {
					continue
				}
				var p struct {
					ParcelID      string  `json:"parcel_id"`
					Address       string  `json:"address"`
					OwnerName     string  `json:"owner_name"`
					Market        string  `json:"market"`
					AssessedValue float64 `json:"assessed_value"`
				}
				if err := json.Unmarshal(content, &p); err != nil {
					continue
				}

				tr, ok := delinquentByParcel[p.ParcelID]
				if !ok {
					continue
				}

				// Estimate days remaining until tax sale
				daysRemaining := redemptionDays // default if no delinquent date
				if tr.DelinquentDate != "" {
					if delinqDate, err := time.Parse("2006-01-02", tr.DelinquentDate); err == nil {
						elapsed := int(math.Round(now.Sub(delinqDate).Hours() / 24))
						daysRemaining = redemptionDays - elapsed
						if daysRemaining < 0 {
							daysRemaining = 0
						}
					}
				}

				// Filter by --within window
				if withinDays > 0 && daysRemaining > withinDays {
					continue
				}

				urgency := "low"
				switch {
				case daysRemaining <= 30:
					urgency = "critical"
				case daysRemaining <= 90:
					urgency = "high"
				case daysRemaining <= 180:
					urgency = "medium"
				}

				results = append(results, taxCountdownResult{
					ParcelID:      p.ParcelID,
					Address:       p.Address,
					OwnerName:     p.OwnerName,
					Market:        p.Market,
					AmountOwed:    tr.AmountOwed,
					DaysRemaining: daysRemaining,
					Urgency:       urgency,
					AssessedValue: p.AssessedValue,
				})
			}
			if err := parcelRows.Err(); err != nil {
				return fmt.Errorf("iterating parcels: %w", err)
			}

			// Sort by days remaining ascending (most urgent first)
			for i := 0; i < len(results); i++ {
				for j := i + 1; j < len(results); j++ {
					if results[j].DaysRemaining < results[i].DaysRemaining {
						results[i], results[j] = results[j], results[i]
					}
				}
			}

			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&market, "market", "", "Market/county to search (e.g., lake-county-in)")
	cmd.Flags().StringVar(&within, "within", "", "Time window filter (e.g., 6mo, 1y, 90d)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	return cmd
}

// parseWithin converts a duration string like "6mo", "1y", "90d" into days.
func parseWithin(s string) (int, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if strings.HasSuffix(s, "mo") {
		var n int
		if _, err := fmt.Sscanf(s, "%dmo", &n); err != nil {
			return 0, fmt.Errorf("expected format like 6mo")
		}
		return n * 30, nil
	}
	if strings.HasSuffix(s, "y") {
		var n int
		if _, err := fmt.Sscanf(s, "%dy", &n); err != nil {
			return 0, fmt.Errorf("expected format like 1y")
		}
		return n * 365, nil
	}
	if strings.HasSuffix(s, "d") {
		var n int
		if _, err := fmt.Sscanf(s, "%dd", &n); err != nil {
			return 0, fmt.Errorf("expected format like 90d")
		}
		return n, nil
	}
	return 0, fmt.Errorf("expected format like 6mo, 1y, or 90d")
}
