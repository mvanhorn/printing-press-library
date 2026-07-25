// Copyright 2026 Olivier and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: transfer-risk.
//
// Joins live per-leg delay from /v1/connections against official_transfer_time
// from the open iRail stations dataset. The API returns itineraries as though
// delays never threaten a transfer; this command says whether they still hold.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/irail/internal/irailref"
)

// Verdicts, worst first.
const (
	transferBroken  = "broken"
	transferTight   = "tight"
	transferOK      = "ok"
	transferUnknown = "unknown"
)

type transferLeg struct {
	Station          string `json:"station"`
	ArriveVehicle    string `json:"arrive_vehicle,omitempty"`
	DepartVehicle    string `json:"depart_vehicle,omitempty"`
	ScheduledGapSec  int    `json:"scheduled_gap_seconds"`
	ActualGapSec     int    `json:"actual_gap_seconds"`
	RequiredSec      int    `json:"required_seconds"`
	HasRequired      bool   `json:"required_known"`
	ArrivalDelaySec  int    `json:"arrival_delay_seconds"`
	DepartDelaySec   int    `json:"departure_delay_seconds"`
	ArrivalPlatform  string `json:"arrival_platform,omitempty"`
	DeparturePlatorm string `json:"departure_platform,omitempty"`
	Verdict          string `json:"verdict"`
	Explanation      string `json:"explanation"`
}

type transferConnection struct {
	ID           string        `json:"id"`
	DepartsAt    string        `json:"departs_at"`
	ArrivesAt    string        `json:"arrives_at"`
	DurationSec  int           `json:"duration_seconds"`
	Transfers    int           `json:"transfers"`
	Verdict      string        `json:"verdict"`
	DepartDelayS int           `json:"departure_delay_seconds"`
	Legs         []transferLeg `json:"legs"`
}

type transferRiskView struct {
	From        string               `json:"from"`
	To          string               `json:"to"`
	CheckedAt   string               `json:"checked_at"`
	Rolled      bool                 `json:"rolled_to_tomorrow,omitempty"`
	Connections []transferConnection `json:"connections"`
	Note        string               `json:"note,omitempty"`
}

func newNovelTransferRiskCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagTo string
	var flagDate string
	var flagTime string
	var flagLimit int
	var flagTightFactor float64

	cmd := &cobra.Command{
		Use:   "transfer-risk",
		Short: "Check whether a journey's transfers survive today's delays",
		Long: "Joins live per-leg delay against each station's official minimum transfer time\n" +
			"from the open iRail stations dataset, then flags transfers that no longer hold.\n\n" +
			"Use this command when a journey has a transfer and delays are in play. Do NOT\n" +
			"use it to find routes; use 'route' for planning.\n\n" +
			"Verdicts: broken (the connection is already lost), tight (less headroom than\n" +
			"the station's official minimum plus margin), ok, or unknown when the dataset\n" +
			"carries no official transfer time for that station.",
		Example: `  irail-pp-cli transfer-risk --from Oostende --to Hasselt
  irail-pp-cli transfer-risk --from Bruges --to Leuven --agent
  irail-pp-cli transfer-risk --from Oostende --to Hasselt --date tomorrow --time 08:00`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would check transfer risk for the requested journey")
				return nil
			}
			if flagFrom == "" || flagTo == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from and --to are both required"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			from := resolveStationName(flagFrom)
			to := resolveStationName(flagTo)

			date, hhmm, rolled, err := resolveWhen(flagDate, flagTime, nowInBelgium())
			if err != nil {
				return usageErr(err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{"from": from, "to": to, "lang": "en"}
			if date != "" {
				params["date"] = date
			}
			if hhmm != "" {
				params["time"] = hhmm
			}
			env, err := irailFetch(ctx, c, "/v1/connections?format=json", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			view := transferRiskView{
				From:        from,
				To:          to,
				CheckedAt:   nowInBelgium().Format(time.RFC3339),
				Rolled:      rolled,
				Connections: make([]transferConnection, 0),
			}

			for _, raw := range sliceAt(env, "connection") {
				conn, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				view.Connections = append(view.Connections, analyseConnection(conn, flagTightFactor))
				if flagLimit > 0 && len(view.Connections) >= flagLimit {
					break
				}
			}

			switch {
			case len(view.Connections) == 0:
				view.Note = "iRail returned no connections for this journey; check the station names or try a nearer date"
			case !anyWithTransfers(view.Connections):
				view.Note = "every option is direct, so there is no transfer to miss"
			}
			if rolled {
				view.Note = joinNotes(view.Note,
					"the requested time had already passed today, so this is tomorrow's journey")
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Connections) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			for _, conn := range view.Connections {
				fmt.Fprintf(cmd.OutOrStdout(), "%s -> %s  (%s, %d transfer(s))  %s\n",
					clockOf(conn.DepartsAt), clockOf(conn.ArrivesAt),
					humanDuration(conn.DurationSec), conn.Transfers, conn.Verdict)
				for _, leg := range conn.Legs {
					fmt.Fprintf(cmd.OutOrStdout(), "    %-24s %-8s %s\n", leg.Station, leg.Verdict, leg.Explanation)
				}
			}
			if view.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", view.Note)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagFrom, "from", "", "Origin station (name, telegraph code or id)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Destination station (name, telegraph code or id)")
	cmd.Flags().StringVar(&flagDate, "date", "", "Date: tomorrow, monday, 2026-07-25, +2d or ddmmyy")
	cmd.Flags().StringVar(&flagTime, "time", "", "Time: 08:12, 0812, now or +30m")
	cmd.Flags().IntVar(&flagLimit, "limit", 6, "Maximum connections to analyse")
	cmd.Flags().Float64Var(&flagTightFactor, "tight-factor", 1.5,
		"Multiple of the official transfer time below which a transfer counts as tight")
	return cmd
}

// analyseConnection evaluates every transfer in one itinerary.
func analyseConnection(conn map[string]any, tightFactor float64) transferConnection {
	dep := mapAt(conn, "departure")
	arr := mapAt(conn, "arrival")

	out := transferConnection{
		ID:          irailString(conn["id"]),
		DurationSec: irailInt(conn["duration"]),
		Verdict:     transferOK,
		Legs:        make([]transferLeg, 0),
	}
	if dep != nil {
		out.DepartsAt = unixToLocal(dep["time"])
		out.DepartDelayS = irailInt(dep["delay"])
	}
	if arr != nil {
		out.ArrivesAt = unixToLocal(arr["time"])
	}

	vias := sliceAt(conn, "vias", "via")
	out.Transfers = len(vias)

	worst := transferOK
	for _, raw := range vias {
		via, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		leg := analyseVia(via, tightFactor)
		out.Legs = append(out.Legs, leg)
		worst = worseVerdict(worst, leg.Verdict)
	}
	out.Verdict = worst
	return out
}

// analyseVia computes headroom at one transfer station.
func analyseVia(via map[string]any, tightFactor float64) transferLeg {
	station := irailString(via["station"])
	arrival := mapAt(via, "arrival")
	departure := mapAt(via, "departure")

	leg := transferLeg{
		Station:       station,
		ArriveVehicle: irailString(via["vehicle"]),
		Verdict:       transferUnknown,
	}

	var arrTime, depTime int64
	if arrival != nil {
		arrTime = irailInt64(arrival["time"])
		leg.ArrivalDelaySec = irailInt(arrival["delay"])
		leg.ArrivalPlatform = irailString(arrival["platform"])
	}
	if departure != nil {
		depTime = irailInt64(departure["time"])
		leg.DepartDelaySec = irailInt(departure["delay"])
		leg.DeparturePlatorm = irailString(departure["platform"])
		leg.DepartVehicle = irailString(departure["vehicle"])
	}
	if arrTime == 0 || depTime == 0 {
		leg.Explanation = "iRail did not report both arrival and departure times for this transfer"
		return leg
	}

	leg.ScheduledGapSec = int(depTime - arrTime)
	// Delay on the inbound train eats the gap; delay on the outbound train
	// gives it back, because the connecting train leaves later too.
	leg.ActualGapSec = leg.ScheduledGapSec - leg.ArrivalDelaySec + leg.DepartDelaySec

	required, known := irailref.TransferSecondsFor(station)
	leg.RequiredSec = required
	leg.HasRequired = known

	switch {
	case leg.ActualGapSec < 0:
		leg.Verdict = transferBroken
		leg.Explanation = fmt.Sprintf(
			"the connecting train leaves %s before you arrive", humanDuration(-leg.ActualGapSec))
	case !known:
		leg.Verdict = transferUnknown
		leg.Explanation = fmt.Sprintf(
			"%s left after delays, but no official minimum transfer time is published for this station",
			humanDuration(leg.ActualGapSec))
	case leg.ActualGapSec < required:
		leg.Verdict = transferBroken
		leg.Explanation = fmt.Sprintf(
			"only %s to change, below the official %s minimum here",
			humanDuration(leg.ActualGapSec), humanDuration(required))
	case float64(leg.ActualGapSec) < float64(required)*tightFactor:
		leg.Verdict = transferTight
		leg.Explanation = fmt.Sprintf(
			"%s to change against an official %s minimum; little room if the delay grows",
			humanDuration(leg.ActualGapSec), humanDuration(required))
	default:
		leg.Verdict = transferOK
		leg.Explanation = fmt.Sprintf(
			"%s to change, comfortably above the official %s minimum",
			humanDuration(leg.ActualGapSec), humanDuration(required))
	}
	return leg
}

// worseVerdict returns the more severe of two verdicts.
func worseVerdict(a, b string) string {
	rank := map[string]int{transferOK: 0, transferUnknown: 1, transferTight: 2, transferBroken: 3}
	if rank[b] > rank[a] {
		return b
	}
	return a
}

func anyWithTransfers(conns []transferConnection) bool {
	for _, c := range conns {
		if c.Transfers > 0 {
			return true
		}
	}
	return false
}

func joinNotes(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "; " + b
}
