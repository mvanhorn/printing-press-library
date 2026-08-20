// uk-train-goat hand-authored: A->B journey planning via chained OpenLDBWS calls.
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/config"
	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/openldbws"
	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/store"

	"github.com/spf13/cobra"
)

func newJourneyCmd(flags *rootFlags) *cobra.Command {
	var (
		date      string
		offsetStr string
		windowStr string
		numRows   int
		rank      bool
	)
	cmd := &cobra.Command{
		Use:   "journey <from-crs> <to-crs>",
		Short: "Plan a journey A->B via OpenLDBWS",
		Long: `Plan a UK rail journey from one CRS code to another. Chained OpenLDBWS calls
filter departures from the origin to the destination. With --rank, each candidate
service is enriched with live status (extra round trips per service) so that the
on-time-but-later option ranks above the earlier-but-late one.`,
		Example: strings.Trim(`
  uk-train-goat-pp-cli journey RDG PAD
  uk-train-goat-pp-cli journey RDG PAD --in 30m --within 60m
  uk-train-goat-pp-cli journey RDG PAD --rank --num 3
  uk-train-goat-pp-cli journey RDG PAD --json --select journeys.std,journeys.etd,journeys.platform
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("journey requires <from-crs> <to-crs>; got %d args", len(args)))
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c, err := openldbws.New(cfg.LdbwsApiToken)
			if err != nil {
				return authErr(err)
			}

			from := strings.ToUpper(strings.TrimSpace(args[0]))
			to := strings.ToUpper(strings.TrimSpace(args[1]))
			if numRows == 0 {
				numRows = 5
			}
			// Cap --rank fan-out per advisor guidance (each ranked service costs
			// one extra GetServiceDetails round trip).
			if rank && numRows > 10 {
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

			// pp:client-call — wraps openldbws.Departures with TerminatingAtOpt(to).
			board, err := c.Departures(from, to, numRows, offsetMin, windowMin)
			if err != nil {
				return apiErr(err)
			}

			journeys := serializeJourneys(board)
			if rank {
				journeys = enrichWithStatus(c, journeys)
				sort.SliceStable(journeys, func(i, j int) bool {
					return journeyScore(journeys[i]) < journeyScore(journeys[j])
				})
			}

			// Persist to search_history (cross-entity local query feeds `recent`).
			recordSearchHistory(flags, from, to, date)

			payload := map[string]any{
				"from":     from,
				"to":       to,
				"date":     date,
				"ranked":   rank,
				"journeys": journeys,
			}
			data, _ := json.Marshal(payload)
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "Target date (YYYY-MM-DD); defaults to today")
	cmd.Flags().StringVar(&offsetStr, "in", "", "Show departures starting after this duration (e.g. 30, 30m, 1h)")
	cmd.Flags().StringVar(&windowStr, "within", "", "Limit to departures within this duration (e.g. 30, 30m, 1h)")
	cmd.Flags().IntVar(&numRows, "num", 0, "Maximum number of journey options (default 5, capped at 10 with --rank)")
	cmd.Flags().BoolVar(&rank, "rank", false, "Rank options by combined scheduled-time + current delay + platform-known")
	return cmd
}

func serializeJourneys(board *openldbws.StationBoard) []map[string]any {
	if board == nil {
		return nil
	}
	out := make([]map[string]any, 0, len(board.TrainServices))
	for _, s := range board.TrainServices {
		row := map[string]any{
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
		out = append(out, row)
	}
	return out
}

// enrichWithStatus calls GetServiceDetails for each candidate journey
// and folds the live data into the row. pp:client-call — wraps
// openldbws.Service per row.
func enrichWithStatus(c *openldbws.Client, journeys []map[string]any) []map[string]any {
	for _, j := range journeys {
		sid, _ := j["service_id"].(string)
		if sid == "" {
			continue
		}
		details, err := c.Service(sid)
		if err != nil || details == nil {
			errMsg := "service details unavailable"
			if err != nil {
				errMsg = err.Error()
			}
			j["live_status"] = map[string]any{"error": errMsg}
			continue
		}
		j["live_status"] = map[string]any{
			"platform_known": details.Platform != nil && *details.Platform != "",
			"etd_live":       ptrOrEmpty(details.ETD),
		}
	}
	return journeys
}

// journeyScore is a simple combined score: lower is better. STD is the
// scheduled departure time; on-time-but-later is preferred when delay
// pushes an earlier service past it.
func journeyScore(j map[string]any) string {
	std, _ := j["std"].(string)
	etd, _ := j["etd"].(string)
	if etd == "On time" || etd == "" {
		return std
	}
	// "HH:MM" format means the etd value is the live expected time
	if len(etd) == 5 && etd[2] == ':' {
		return etd
	}
	// "Cancelled", "Delayed", anything else: push to bottom.
	return "99:99"
}

// recordSearchHistory writes the journey query into the local store so
// the `recent` command can replay it. Best-effort — failures are silent
// (a transient store write failure should not block the journey response).
func recordSearchHistory(flags *rootFlags, from, to, date string) {
	dbPath := defaultDBPath("uk-train-goat-pp-cli")
	s, err := store.Open(dbPath)
	if err != nil {
		return
	}
	defer s.Close()
	row := map[string]any{
		"from":       from,
		"to":         to,
		"date":       date,
		"queried_at": time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.Marshal(row)
	id := fmt.Sprintf("%s-%s-%d", from, to, time.Now().UnixNano())
	_ = s.Upsert("search_history", id, data)
}
