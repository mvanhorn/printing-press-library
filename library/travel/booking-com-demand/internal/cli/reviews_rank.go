package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/booking-com-demand/internal/store"

	"github.com/spf13/cobra"
)

func newReviewsRankCmd(flags *rootFlags) *cobra.Command {
	var destination string
	var minCleanliness float64
	var minLocation float64
	var minStaff float64
	var maxPrice float64
	var sortField string
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "rank",
		Short: "Rank properties by review sub-categories with price filtering",
		Long: `Rank joins review_scores, accommodations, and availability data from the local
store to produce a ranked list of properties by user-specified criteria.

Filter by minimum review sub-scores (cleanliness, location, staff) and
maximum price, then sort by any score field or price.

Requires synced data: run 'booking-com-demand-pp-cli sync --full' first.`,
		Example: strings.Trim(`
  booking-com-demand-pp-cli reviews rank --destination Paris
  booking-com-demand-pp-cli reviews rank --min-cleanliness 8 --min-staff 8
  booking-com-demand-pp-cli reviews rank --max-price 200 --sort cleanliness
  booking-com-demand-pp-cli reviews rank --destination London --sort location --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = store.DefaultPath()
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w\nhint: run 'booking-com-demand-pp-cli sync --full' first", err)
			}
			defer s.Close()

			// Query accommodations with their review scores
			query := `
				SELECT a.id,
					json_extract(a.data, '$.name') as name,
					rs.data as scores_data,
					json_extract(av.data, '$.price') as price,
					json_extract(a.data, '$.city') as city
				FROM accommodations a
				LEFT JOIN review_scores rs ON a.id = rs.id
				LEFT JOIN availability av ON a.id = av.id
			`
			conditions := []string{}
			if destination != "" {
				conditions = append(conditions, fmt.Sprintf(
					"(json_extract(a.data, '$.city') LIKE '%%%s%%' OR json_extract(a.data, '$.city_name') LIKE '%%%s%%' OR json_extract(a.data, '$.destination') LIKE '%%%s%%')",
					destination, destination, destination,
				))
			}
			if len(conditions) > 0 {
				query += " WHERE " + strings.Join(conditions, " AND ")
			}

			rows, err := s.DB().Query(query)
			if err != nil {
				return fmt.Errorf("query failed: %w", err)
			}
			defer rows.Close()

			type rankedProperty struct {
				ID           string  `json:"id"`
				Name         string  `json:"name"`
				City         string  `json:"city,omitempty"`
				Cleanliness  float64 `json:"cleanliness"`
				Location     float64 `json:"location"`
				Staff        float64 `json:"staff"`
				Overall      float64 `json:"overall"`
				Price        float64 `json:"price,omitempty"`
				CompositeStr string  `json:"-"`
			}

			var properties []rankedProperty
			for rows.Next() {
				var id string
				var name, scoresData, priceStr, city *string
				if err := rows.Scan(&id, &name, &scoresData, &priceStr, &city); err != nil {
					continue
				}

				prop := rankedProperty{ID: id}
				if name != nil {
					prop.Name = *name
				}
				if city != nil {
					prop.City = *city
				}

				// Parse review scores
				if scoresData != nil {
					var scores map[string]any
					if json.Unmarshal([]byte(*scoresData), &scores) == nil {
						prop.Cleanliness = floatFromAny(scores["cleanliness"])
						prop.Location = floatFromAny(scores["location"])
						prop.Staff = floatFromAny(scores["staff"])
						prop.Overall = floatFromAny(scores["review_score"])
						if prop.Overall == 0 {
							prop.Overall = floatFromAny(scores["overall"])
						}
					}
				}

				// Parse price
				if priceStr != nil {
					fmt.Sscanf(*priceStr, "%f", &prop.Price)
				}

				// Apply filters
				if minCleanliness > 0 && prop.Cleanliness < minCleanliness {
					continue
				}
				if minLocation > 0 && prop.Location < minLocation {
					continue
				}
				if minStaff > 0 && prop.Staff < minStaff {
					continue
				}
				if maxPrice > 0 && prop.Price > maxPrice {
					continue
				}

				properties = append(properties, prop)
			}

			if len(properties) == 0 {
				return writeNoop(flags, "no_results", "no properties match the specified criteria")
			}

			// Sort
			sort.Slice(properties, func(i, j int) bool {
				switch sortField {
				case "cleanliness":
					return properties[i].Cleanliness > properties[j].Cleanliness
				case "location":
					return properties[i].Location > properties[j].Location
				case "staff":
					return properties[i].Staff > properties[j].Staff
				case "price":
					return properties[i].Price < properties[j].Price
				default:
					return properties[i].Overall > properties[j].Overall
				}
			})

			if limit > 0 && len(properties) > limit {
				properties = properties[:limit]
			}

			return flags.printJSON(cmd, properties)
		},
	}

	cmd.Flags().StringVar(&destination, "destination", "", "Filter by destination city name")
	cmd.Flags().Float64Var(&minCleanliness, "min-cleanliness", 0, "Minimum cleanliness score (0-10)")
	cmd.Flags().Float64Var(&minLocation, "min-location", 0, "Minimum location score (0-10)")
	cmd.Flags().Float64Var(&minStaff, "min-staff", 0, "Minimum staff score (0-10)")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Maximum price per night")
	cmd.Flags().StringVar(&sortField, "sort", "overall", "Sort field: overall, cleanliness, location, staff, price")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to SQLite store")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum results to return")

	return cmd
}

func floatFromAny(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	default:
		return 0
	}
}
