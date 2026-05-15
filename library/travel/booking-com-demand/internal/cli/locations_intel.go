package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/booking-com-demand/internal/store"

	"github.com/spf13/cobra"
)

func newLocationsIntelCmd(flags *rootFlags) *cobra.Command {
	var city string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "intel",
		Short: "City-level analytics from the local store",
		Long: `Intel aggregates data across cities, districts, landmarks, accommodations,
review_scores, and availability tables to produce city-level analytics.

Shows property count, average review scores, price ranges, top landmarks,
and district breakdown for a given city.

Requires synced data: run 'booking-com-demand-pp-cli sync --full' first.`,
		Example: strings.Trim(`
  booking-com-demand-pp-cli locations intel --city Paris
  booking-com-demand-pp-cli locations intel --city "New York" --json
  booking-com-demand-pp-cli locations intel --city London`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if city == "" {
				return cmd.Help()
			}
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

			type districtSummary struct {
				Name          string  `json:"name"`
				PropertyCount int     `json:"property_count,omitempty"`
				AvgScore      float64 `json:"avg_score,omitempty"`
			}

			type landmarkInfo struct {
				Name string `json:"name"`
				ID   string `json:"id,omitempty"`
			}

			type cityIntel struct {
				City           string            `json:"city"`
				PropertyCount  int               `json:"property_count"`
				AvgReviewScore float64           `json:"avg_review_score,omitempty"`
				AvgCleanliness float64           `json:"avg_cleanliness,omitempty"`
				AvgLocation    float64           `json:"avg_location,omitempty"`
				AvgStaff       float64           `json:"avg_staff,omitempty"`
				MinPrice       float64           `json:"min_price,omitempty"`
				MaxPrice       float64           `json:"max_price,omitempty"`
				AvgPrice       float64           `json:"avg_price,omitempty"`
				Districts      []districtSummary `json:"districts,omitempty"`
				TopLandmarks   []landmarkInfo    `json:"top_landmarks,omitempty"`
				CityInfo       map[string]any    `json:"city_info,omitempty"`
			}

			intel := cityIntel{City: city}

			// Find city info
			cityRows, err := s.DB().Query(
				"SELECT id, data FROM locations_cities WHERE json_extract(data, '$.name') LIKE ? OR json_extract(data, '$.city_name') LIKE ?",
				"%"+city+"%", "%"+city+"%",
			)
			if err == nil {
				defer cityRows.Close()
				for cityRows.Next() {
					var cid, cdata string
					if cityRows.Scan(&cid, &cdata) == nil {
						var obj map[string]any
						if json.Unmarshal([]byte(cdata), &obj) == nil {
							intel.CityInfo = obj
							break
						}
					}
				}
			}

			// Count accommodations in city
			accRows, err := s.DB().Query(
				"SELECT id, data FROM accommodations WHERE json_extract(data, '$.city') LIKE ? OR json_extract(data, '$.city_name') LIKE ? OR json_extract(data, '$.destination') LIKE ?",
				"%"+city+"%", "%"+city+"%", "%"+city+"%",
			)
			if err == nil {
				defer accRows.Close()
				var propIDs []string
				for accRows.Next() {
					var aid, adata string
					if accRows.Scan(&aid, &adata) == nil {
						propIDs = append(propIDs, aid)
						intel.PropertyCount++
					}
				}

				// Aggregate review scores for matched properties
				if len(propIDs) > 0 {
					var totalScore, totalClean, totalLoc, totalStaff float64
					var scoreCount int
					for _, pid := range propIDs {
						row := s.DB().QueryRow("SELECT data FROM review_scores WHERE id = ?", pid)
						var sdata string
						if row.Scan(&sdata) != nil {
							continue
						}
						var scores map[string]any
						if json.Unmarshal([]byte(sdata), &scores) != nil {
							continue
						}
						overall := floatFromAny(scores["review_score"])
						if overall == 0 {
							overall = floatFromAny(scores["overall"])
						}
						if overall > 0 {
							totalScore += overall
							scoreCount++
						}
						totalClean += floatFromAny(scores["cleanliness"])
						totalLoc += floatFromAny(scores["location"])
						totalStaff += floatFromAny(scores["staff"])
					}
					if scoreCount > 0 {
						intel.AvgReviewScore = roundTo2(totalScore / float64(scoreCount))
						intel.AvgCleanliness = roundTo2(totalClean / float64(scoreCount))
						intel.AvgLocation = roundTo2(totalLoc / float64(scoreCount))
						intel.AvgStaff = roundTo2(totalStaff / float64(scoreCount))
					}

					// Aggregate pricing
					var minP, maxP, totalP float64
					var priceCount int
					for _, pid := range propIDs {
						row := s.DB().QueryRow("SELECT data FROM availability WHERE id = ?", pid)
						var adata string
						if row.Scan(&adata) != nil {
							continue
						}
						var obj map[string]any
						if json.Unmarshal([]byte(adata), &obj) != nil {
							continue
						}
						p := extractPrice(obj)
						if p > 0 {
							if priceCount == 0 || p < minP {
								minP = p
							}
							if p > maxP {
								maxP = p
							}
							totalP += p
							priceCount++
						}
					}
					if priceCount > 0 {
						intel.MinPrice = minP
						intel.MaxPrice = maxP
						intel.AvgPrice = roundTo2(totalP / float64(priceCount))
					}
				}
			}

			// Districts
			distRows, err := s.DB().Query(
				"SELECT id, data FROM locations_districts WHERE json_extract(data, '$.city_name') LIKE ? OR json_extract(data, '$.name') LIKE ?",
				"%"+city+"%", "%"+city+"%",
			)
			if err == nil {
				defer distRows.Close()
				for distRows.Next() {
					var did, ddata string
					if distRows.Scan(&did, &ddata) == nil {
						var obj map[string]any
						if json.Unmarshal([]byte(ddata), &obj) == nil {
							name, _ := obj["name"].(string)
							if name == "" {
								name = did
							}
							intel.Districts = append(intel.Districts, districtSummary{Name: name})
						}
					}
				}
			}

			// Landmarks
			lmRows, err := s.DB().Query(
				"SELECT id, data FROM locations_landmarks WHERE json_extract(data, '$.city_name') LIKE ? OR json_extract(data, '$.name') LIKE ?",
				"%"+city+"%", "%"+city+"%",
			)
			if err == nil {
				defer lmRows.Close()
				for lmRows.Next() {
					var lid, ldata string
					if lmRows.Scan(&lid, &ldata) == nil {
						var obj map[string]any
						if json.Unmarshal([]byte(ldata), &obj) == nil {
							name, _ := obj["name"].(string)
							if name == "" {
								name = lid
							}
							intel.TopLandmarks = append(intel.TopLandmarks, landmarkInfo{Name: name, ID: lid})
						}
					}
				}
			}

			if intel.PropertyCount == 0 && intel.CityInfo == nil && len(intel.Districts) == 0 && len(intel.TopLandmarks) == 0 {
				return writeNoop(flags, "no_data", fmt.Sprintf("no data found for city %q; run 'sync --full' first", city))
			}

			return flags.printJSON(cmd, intel)
		},
	}

	cmd.Flags().StringVar(&city, "city", "", "City name to analyze (required)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to SQLite store")

	return cmd
}

func roundTo2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
