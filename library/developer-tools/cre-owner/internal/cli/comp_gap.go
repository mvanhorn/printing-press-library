package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/cre-owner/internal/store"
	"github.com/spf13/cobra"
)

func newCompGapCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var flagAddress string
	var flagRadius float64
	var flagMonths int

	cmd := &cobra.Command{
		Use:   "comp-gap",
		Short: "Compare assessed value against recent comparable sales to find value arbitrage",
		Long: `Given an address, find the parcel's assessed value, then find nearby sold
comparables and compute the gap between assessed value and market value.

Uses latitude/longitude from the target parcel to find sales within the
specified radius. Computes average sale price per sqft vs assessed value per
sqft to identify under- or over-valued properties.

Data comes from the local SQLite store. Run 'sync' first to populate.`,
		Example: strings.Trim(`
  cre-owner-pp-cli comp-gap --address "123 Main St"
  cre-owner-pp-cli comp-gap --address "456 Oak Ave" --radius 0.5 --months 12
  cre-owner-pp-cli comp-gap --address "789 Elm Blvd" --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("address") && !flags.dryRun {
				return fmt.Errorf("required flag \"address\" not set")
			}
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("cre-owner-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			db := s.DB()

			// Find target parcel by address (case-insensitive partial match).
			var targetData string
			err = db.QueryRow(`
				SELECT data FROM resources
				WHERE resource_type = 'parcels'
				  AND LOWER(json_extract(data, '$.address')) LIKE LOWER(?)
				LIMIT 1`,
				"%"+flagAddress+"%",
			).Scan(&targetData)
			if err != nil {
				return fmt.Errorf("parcel not found for address %q: run 'sync' first or check the address", flagAddress)
			}

			var target struct {
				ParcelID      string  `json:"parcel_id"`
				Address       string  `json:"address"`
				City          string  `json:"city"`
				State         string  `json:"state"`
				Zip           string  `json:"zip"`
				OwnerName     string  `json:"owner_name"`
				PropertyType  string  `json:"property_type"`
				Sqft          float64 `json:"sqft"`
				AssessedValue float64 `json:"assessed_value"`
				Latitude      float64 `json:"latitude"`
				Longitude     float64 `json:"longitude"`
			}
			if err := json.Unmarshal([]byte(targetData), &target); err != nil {
				return fmt.Errorf("parsing parcel data: %w", err)
			}

			if target.Latitude == 0 && target.Longitude == 0 {
				return fmt.Errorf("parcel %q has no latitude/longitude data for radius search", flagAddress)
			}

			// Approximate degree-to-mile conversion at target latitude.
			latDeg := flagRadius / 69.0
			lngDeg := flagRadius / (69.0 * math.Cos(target.Latitude*math.Pi/180))

			// Find recent sales within radius.
			salesRows, err := db.Query(`
				SELECT data FROM resources
				WHERE resource_type = 'sales'
				  AND ABS(CAST(json_extract(data, '$.latitude') AS REAL) - ?) <= ?
				  AND ABS(CAST(json_extract(data, '$.longitude') AS REAL) - ?) <= ?
				  AND json_extract(data, '$.sale_date') >= date('now', ?)`,
				target.Latitude, latDeg,
				target.Longitude, lngDeg,
				fmt.Sprintf("-%d months", flagMonths),
			)
			if err != nil {
				return fmt.Errorf("querying comparable sales: %w", err)
			}
			defer salesRows.Close()

			type compSale struct {
				Address       string  `json:"address"`
				SaleDate      string  `json:"sale_date"`
				SalePrice     float64 `json:"sale_price"`
				Sqft          float64 `json:"sqft"`
				PricePerSqft  float64 `json:"price_per_sqft"`
				DistanceMiles float64 `json:"distance_miles"`
			}

			var comps []compSale
			var totalPricePerSqft float64
			for salesRows.Next() {
				var saleData string
				if err := salesRows.Scan(&saleData); err != nil {
					continue
				}
				var sale struct {
					Address   string  `json:"address"`
					SaleDate  string  `json:"sale_date"`
					SalePrice float64 `json:"sale_price"`
					Sqft      float64 `json:"sqft"`
					Latitude  float64 `json:"latitude"`
					Longitude float64 `json:"longitude"`
				}
				if err := json.Unmarshal([]byte(saleData), &sale); err != nil {
					continue
				}
				if sale.SalePrice <= 0 || sale.Sqft <= 0 {
					continue
				}

				// Haversine approximation for distance.
				dLat := (sale.Latitude - target.Latitude) * math.Pi / 180
				dLng := (sale.Longitude - target.Longitude) * math.Pi / 180
				a := math.Sin(dLat/2)*math.Sin(dLat/2) +
					math.Cos(target.Latitude*math.Pi/180)*math.Cos(sale.Latitude*math.Pi/180)*
						math.Sin(dLng/2)*math.Sin(dLng/2)
				dist := 3959 * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

				if dist > flagRadius {
					continue
				}

				ppsf := sale.SalePrice / sale.Sqft
				comps = append(comps, compSale{
					Address:       sale.Address,
					SaleDate:      sale.SaleDate,
					SalePrice:     sale.SalePrice,
					Sqft:          sale.Sqft,
					PricePerSqft:  math.Round(ppsf*100) / 100,
					DistanceMiles: math.Round(dist*100) / 100,
				})
				totalPricePerSqft += ppsf
			}
			if err := salesRows.Err(); err != nil {
				return fmt.Errorf("reading sales rows: %w", err)
			}

			// Compute gap analysis.
			var assessedPSF, avgMarketPSF, gapPct float64
			var gapLabel string
			if target.Sqft > 0 {
				assessedPSF = math.Round(target.AssessedValue/target.Sqft*100) / 100
			}
			if len(comps) > 0 {
				avgMarketPSF = math.Round(totalPricePerSqft/float64(len(comps))*100) / 100
			}
			if assessedPSF > 0 && avgMarketPSF > 0 {
				gapPct = math.Round((avgMarketPSF-assessedPSF)/assessedPSF*10000) / 100
				if gapPct > 0 {
					gapLabel = "under_market"
				} else if gapPct < 0 {
					gapLabel = "over_market"
				} else {
					gapLabel = "at_market"
				}
			}

			result := map[string]any{
				"target_property": map[string]any{
					"parcel_id":      target.ParcelID,
					"address":        target.Address,
					"city":           target.City,
					"state":          target.State,
					"owner_name":     target.OwnerName,
					"property_type":  target.PropertyType,
					"sqft":           target.Sqft,
					"assessed_value": target.AssessedValue,
					"assessed_psf":   assessedPSF,
				},
				"comparable_sales": comps,
				"gap_analysis": map[string]any{
					"comp_count":      len(comps),
					"avg_market_psf":  avgMarketPSF,
					"assessed_psf":    assessedPSF,
					"gap_percent":     gapPct,
					"gap_direction":   gapLabel,
					"radius_miles":    flagRadius,
					"lookback_months": flagMonths,
				},
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&flagAddress, "address", "", "Target property address (required)")
	cmd.Flags().Float64Var(&flagRadius, "radius", 1.0, "Search radius in miles for comparables")
	cmd.Flags().IntVar(&flagMonths, "months", 24, "Lookback period in months for sales")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/cre-owner-pp-cli/data.db)")

	return cmd
}
