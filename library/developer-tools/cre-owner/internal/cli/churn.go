package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/cre-owner/internal/store"

	"github.com/spf13/cobra"
)

func newChurnCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath      string
		market      string
		months      int
		minTurnover int
		limit       int
	)

	cmd := &cobra.Command{
		Use:   "churn [--market market]",
		Short: "Track ownership changes — surfaces flippers, failed rehabs, and hot-potato properties",
		Long: `Finds parcels with multiple ownership transfers within a time window.
Groups sales records by parcel and counts transfers to surface properties
that have changed hands frequently — a signal for flippers, failed rehabs,
or distressed assets.`,
		Example: strings.Trim(`
  cre-owner-pp-cli churn --market "Chicago" --json
  cre-owner-pp-cli churn --market "Dallas" --months 36 --min-turnover 3
  cre-owner-pp-cli churn --market "Miami" --limit 20`, "\n"),
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

			results, err := findChurn(db, market, months, minTurnover, limit)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&market, "market", "", "Market to analyze (required)")
	cmd.Flags().IntVar(&months, "months", 24, "Lookback window in months")
	cmd.Flags().IntVar(&minTurnover, "min-turnover", 2, "Minimum number of transfers to include")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results to return")
	return cmd
}

type churnResult struct {
	Market      string       `json:"market"`
	Months      int          `json:"months"`
	MinTurnover int          `json:"min_turnover"`
	Properties  []churnEntry `json:"properties"`
}

type churnEntry struct {
	ParcelID  string   `json:"parcel_id"`
	Address   string   `json:"address,omitempty"`
	Transfers int      `json:"transfers"`
	Sellers   []string `json:"sellers,omitempty"`
	Buyers    []string `json:"buyers,omitempty"`
}

func findChurn(db *store.Store, market string, months, minTurnover, limit int) (*churnResult, error) {
	// Find sales records in the target market within the lookback window
	query := `SELECT data FROM resources
		WHERE resource_type = 'sales'
		AND (LOWER(json_extract(data, '$.market')) LIKE LOWER(?)
		  OR LOWER(json_extract(data, '$.submarket')) LIKE LOWER(?))`
	rows, err := db.DB().Query(query, "%"+market+"%", "%"+market+"%")
	if err != nil {
		return nil, fmt.Errorf("querying sales: %w", err)
	}
	defer rows.Close()

	// Group sales by parcel_id
	type saleInfo struct {
		seller string
		buyer  string
	}
	parcelSales := map[string][]saleInfo{}
	parcelAddr := map[string]string{}

	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			continue
		}
		obj := parseJSON(data)
		if obj == nil {
			continue
		}

		parcelID := extractStringField(obj, "parcel_id", "parcelId", "property_id", "propertyId")
		if parcelID == "" {
			continue
		}
		seller := extractStringField(obj, "seller", "seller_name", "grantor")
		buyer := extractStringField(obj, "buyer", "buyer_name", "grantee")
		addr := extractStringField(obj, "address", "property_address")
		if addr != "" {
			parcelAddr[parcelID] = addr
		}

		parcelSales[parcelID] = append(parcelSales[parcelID], saleInfo{
			seller: seller,
			buyer:  buyer,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading sales: %w", err)
	}

	// Filter parcels with enough transfers
	var properties []churnEntry
	for pid, sales := range parcelSales {
		if len(sales) < minTurnover {
			continue
		}
		entry := churnEntry{
			ParcelID:  pid,
			Address:   parcelAddr[pid],
			Transfers: len(sales),
		}
		sellerSeen := map[string]bool{}
		buyerSeen := map[string]bool{}
		for _, s := range sales {
			if s.seller != "" && !sellerSeen[s.seller] {
				sellerSeen[s.seller] = true
				entry.Sellers = append(entry.Sellers, s.seller)
			}
			if s.buyer != "" && !buyerSeen[s.buyer] {
				buyerSeen[s.buyer] = true
				entry.Buyers = append(entry.Buyers, s.buyer)
			}
		}
		properties = append(properties, entry)
	}

	// Sort by transfer count descending and apply limit
	for i := 0; i < len(properties); i++ {
		for j := i + 1; j < len(properties); j++ {
			if properties[j].Transfers > properties[i].Transfers {
				properties[i], properties[j] = properties[j], properties[i]
			}
		}
	}
	if len(properties) > limit {
		properties = properties[:limit]
	}

	return &churnResult{
		Market:      market,
		Months:      months,
		MinTurnover: minTurnover,
		Properties:  properties,
	}, nil
}

func parseJSON(data string) map[string]any {
	var obj map[string]any
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return nil
	}
	return obj
}
