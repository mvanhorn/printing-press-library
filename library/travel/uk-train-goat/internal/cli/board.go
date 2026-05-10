// uk-train-goat hand-authored: live departure board command.
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/config"
	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/openldbws"

	"github.com/spf13/cobra"
)

func newBoardCmd(flags *rootFlags) *cobra.Command {
	var (
		dest      string
		numRows   int
		offsetStr string
		windowStr string
		details   bool
	)
	cmd := &cobra.Command{
		Use:   "board <crs[,crs2,...]>",
		Short: "Live departures from a UK rail station",
		Long: `Show the next departures from one or more UK rail stations using OpenLDBWS.

Accepts a single CRS code (e.g. PAD) or a comma-separated list (e.g. PAD,KGX,EUS)
to merge departures across multiple stations into one ranked time-ordered list.`,
		Example: strings.Trim(`
  uk-train-goat-pp-cli board PAD --num 5
  uk-train-goat-pp-cli board PAD --dest RDG
  uk-train-goat-pp-cli board PAD,KGX,EUS --in 30m
  uk-train-goat-pp-cli board PAD --json --select services.std,services.platform,services.destination.name
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
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

			crsList := splitCSV(args[0])
			if numRows == 0 {
				numRows = 10
			}
			offsetMin, err := parseDurationMinutes(offsetStr)
			if err != nil {
				return usageErr(err)
			}
			windowMin, err := parseDurationMinutes(windowStr)
			if err != nil {
				return usageErr(err)
			}

			boards, err := fanoutBoards(c, crsList, dest, numRows, offsetMin, windowMin, details)
			if err != nil {
				return apiErr(err)
			}

			payload := map[string]any{
				"origins":  crsList,
				"services": flattenServices(boards),
				"messages": flattenMessages(boards),
			}
			data, _ := json.Marshal(payload)
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&dest, "dest", "", "Filter to services going to this destination CRS")
	cmd.Flags().IntVar(&numRows, "num", 0, "Maximum number of services to return (default 10, max 150)")
	cmd.Flags().StringVar(&offsetStr, "in", "", "Show departures starting after this duration (e.g. 30, 30m, 1h)")
	cmd.Flags().StringVar(&windowStr, "within", "", "Limit to departures within this duration (e.g. 30, 30m, 1h)")
	cmd.Flags().BoolVar(&details, "details", false, "Include calling-points for every service")
	return cmd
}

// splitCSV parses a comma-separated list of CRS codes; returns uppercased.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToUpper(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// fanoutBoards calls Departures (or DeparturesWithDetails) in parallel
// across multiple origin CRS codes. Returns the boards in the same
// ordering as the input. pp:client-call — wraps openldbws.Departures.
func fanoutBoards(c *openldbws.Client, crsList []string, dest string, numRows, offsetMin, windowMin int, details bool) ([]*openldbws.StationBoard, error) {
	type result struct {
		idx   int
		board *openldbws.StationBoard
		err   error
	}
	results := make(chan result, len(crsList))
	var wg sync.WaitGroup
	for i, crs := range crsList {
		wg.Add(1)
		go func(i int, crs string) {
			defer wg.Done()
			var board *openldbws.StationBoard
			var err error
			if details {
				board, err = c.DeparturesWithDetails(crs, dest, numRows, offsetMin, windowMin)
			} else {
				board, err = c.Departures(crs, dest, numRows, offsetMin, windowMin)
			}
			results <- result{idx: i, board: board, err: err}
		}(i, crs)
	}
	wg.Wait()
	close(results)

	out := make([]*openldbws.StationBoard, len(crsList))
	for r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("%s: %w", crsList[r.idx], r.err)
		}
		out[r.idx] = r.board
	}
	return out, nil
}

// flattenServices merges every TrainService from all boards into one
// time-ordered slice (by STD), tagging each with its origin CRS so the
// fan-out is visible to agents.
func flattenServices(boards []*openldbws.StationBoard) []map[string]any {
	type item struct {
		std string
		row map[string]any
	}
	var items []item
	for _, b := range boards {
		if b == nil {
			continue
		}
		for _, s := range b.TrainServices {
			row := map[string]any{
				"origin_crs":   b.CRS,
				"origin_name":  b.LocationName,
				"std":          s.STD,
				"etd":          s.ETD,
				"platform":     ptrOrEmpty(s.Platform),
				"operator":     s.Operator,
				"service_id":   s.ServiceID,
				"service_type": s.ServiceType,
			}
			if s.Destination != nil {
				row["destination"] = map[string]any{
					"crs":  s.Destination.CRS,
					"name": s.Destination.Name,
				}
			}
			if s.DelayReason != nil {
				row["delay_reason"] = *s.DelayReason
			}
			items = append(items, item{std: s.STD, row: row})
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].std < items[j].std
	})
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, it.row)
	}
	return out
}

// flattenMessages collects NRCC operator messages across boards, keyed
// by origin CRS so consumers can attribute alerts.
func flattenMessages(boards []*openldbws.StationBoard) []map[string]any {
	var out []map[string]any
	for _, b := range boards {
		if b == nil {
			continue
		}
		for _, m := range b.NRCCMessages {
			out = append(out, map[string]any{
				"origin_crs": b.CRS,
				"text":       m,
			})
		}
	}
	return out
}

func ptrOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// (touch to keep time import used by other commands sharing this file's package)
var _ = time.Now
