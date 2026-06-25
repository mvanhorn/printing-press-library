// uk-train-goat hand-authored: recent-journey replay. Transcendence feature.
//
// Re-runs the user's last N journey queries with fresh live data,
// side-by-side. Cross-entity local x live join: search_history rows
// re-fired as live OpenLDBWS calls.
package cli

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/config"
	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/openldbws"
	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/store"

	"github.com/spf13/cobra"
)

func newRecentCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "recent",
		Short:       "Replay your recent journey queries with fresh live data (transcendence)",
		Long:        "Reads the last N rows from the local search_history table and re-runs each as a live `journey` call. Useful when iterating dates or origins for one trip — every previous query stays one keystroke away.",
		Example:     "  uk-train-goat-pp-cli recent --json --limit 3",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c, err := openldbws.New(cfg.LdbwsApiToken)
			if err != nil {
				return authErr(err)
			}
			s, err := store.Open(defaultDBPath("uk-train-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()

			if limit == 0 {
				limit = 5
			}
			// search_history rows have synthetic IDs like FROM-TO-<unix>;
			// List returns most-recently-upserted first via the generic
			// resources schema's synced_at index.
			rows, err := s.List("search_history", limit)
			if err != nil {
				return err
			}

			type result struct {
				From     string         `json:"from"`
				To       string         `json:"to"`
				Date     string         `json:"date"`
				Next     map[string]any `json:"next,omitempty"`
				Err      string         `json:"error,omitempty"`
			}
			results := make([]result, len(rows))
			var wg sync.WaitGroup
			for i, raw := range rows {
				var hist map[string]string
				if err := json.Unmarshal(raw, &hist); err != nil {
					results[i] = result{Err: err.Error()}
					continue
				}
				wg.Add(1)
				go func(i int, hist map[string]string) {
					defer wg.Done()
					// pp:client-call — wraps openldbws.Departures with TerminatingAtOpt.
					board, err := c.Departures(hist["from"], hist["to"], 1, 0, 0)
					r := result{From: hist["from"], To: hist["to"], Date: hist["date"]}
					if err != nil {
						r.Err = err.Error()
					} else if board != nil && len(board.TrainServices) > 0 {
						svc := board.TrainServices[0]
						r.Next = map[string]any{
							"std":      svc.STD,
							"etd":      svc.ETD,
							"platform": ptrOrEmpty(svc.Platform),
						}
					}
					results[i] = r
				}(i, hist)
			}
			wg.Wait()

			payload := map[string]any{"queries": results}
			out, _ := json.Marshal(payload)
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Number of recent journeys to replay (default 5)")
	return cmd
}

// touch openldbws import so static analysis doesn't complain on builds
// where the type alias is unused via direct references above.
var _ openldbws.CRSCode

// touch fmt import so accidental refactors don't leave stale imports.
var _ = fmt.Sprintf
