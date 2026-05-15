package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/booking-com-demand/internal/store"

	"github.com/spf13/cobra"
)

func newAccommodationsCompareCmd(flags *rootFlags) *cobra.Command {
	var ids string
	var showFields string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Side-by-side comparison of multiple properties",
		Long: `Compare joins accommodation details, review scores, and availability from the
local store for the specified property IDs and presents them side-by-side.

Useful for evaluating competing accommodations on facilities, pricing, and
review scores.

Requires synced data: run 'booking-com-demand-pp-cli sync --full' first.`,
		Example: strings.Trim(`
  booking-com-demand-pp-cli accommodations compare --ids 12345,67890
  booking-com-demand-pp-cli accommodations compare --ids 12345,67890,11111 --show name,price,review_score
  booking-com-demand-pp-cli accommodations compare --ids 12345,67890 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if ids == "" {
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

			idList := strings.Split(ids, ",")
			for i := range idList {
				idList[i] = strings.TrimSpace(idList[i])
			}

			var showFieldSet map[string]bool
			if showFields != "" {
				showFieldSet = map[string]bool{}
				for _, f := range strings.Split(showFields, ",") {
					showFieldSet[strings.TrimSpace(f)] = true
				}
			}

			type comparedProperty struct {
				ID           string         `json:"id"`
				Name         string         `json:"name,omitempty"`
				Details      map[string]any `json:"details,omitempty"`
				ReviewScores map[string]any `json:"review_scores,omitempty"`
				Availability map[string]any `json:"availability,omitempty"`
			}

			var results []comparedProperty
			for _, id := range idList {
				prop := comparedProperty{ID: id}

				// Accommodation details
				row := s.DB().QueryRow("SELECT data FROM accommodations WHERE id = ?", id)
				var data string
				if row.Scan(&data) == nil {
					var obj map[string]any
					if json.Unmarshal([]byte(data), &obj) == nil {
						if showFieldSet != nil {
							filtered := map[string]any{}
							for k, v := range obj {
								if showFieldSet[k] {
									filtered[k] = v
								}
							}
							prop.Details = filtered
						} else {
							prop.Details = obj
						}
						if n, ok := obj["name"].(string); ok {
							prop.Name = n
						}
					}
				}

				// Review scores
				row = s.DB().QueryRow("SELECT data FROM review_scores WHERE id = ?", id)
				if row.Scan(&data) == nil {
					var obj map[string]any
					if json.Unmarshal([]byte(data), &obj) == nil {
						prop.ReviewScores = obj
					}
				}

				// Availability
				row = s.DB().QueryRow("SELECT data FROM availability WHERE id = ?", id)
				if row.Scan(&data) == nil {
					var obj map[string]any
					if json.Unmarshal([]byte(data), &obj) == nil {
						prop.Availability = obj
					}
				}

				if prop.Details == nil && prop.ReviewScores == nil && prop.Availability == nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: no data found for property %s\n", id)
					continue
				}

				results = append(results, prop)
			}

			if len(results) == 0 {
				return writeNoop(flags, "no_data", "no data found for any of the specified property IDs")
			}

			return flags.printJSON(cmd, results)
		},
	}

	cmd.Flags().StringVar(&ids, "ids", "", "Comma-separated property IDs to compare (required)")
	cmd.Flags().StringVar(&showFields, "show", "", "Comma-separated fields to include in comparison (default: all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to SQLite store")

	return cmd
}
