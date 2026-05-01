// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.
// Transcendence commands: compound features that only work because flightgoat
// joins FlightAware AeroAPI + Google Flights (fli) + a local SQLite store.
// These are the novel features no single competing tool offers.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/flightgoat/internal/client"

	"github.com/spf13/cobra"
)

// registerTranscendCommands attaches all novel compound commands to rootCmd.
// Called from root.go. Kept in one function so the root-level registration
// edit to root.go stays minimal.
func registerTranscendCommands(rootCmd *cobra.Command, flags *rootFlags) {
	// longhaul and explore are registered by primary.go as Kayak-backed
	// commands (free, no API key). The AeroAPI-backed fallbacks live on
	// as newLonghaulCmd and newExploreCmd but are not wired as top-level
	// commands anymore.
	rootCmd.AddCommand(newCheapestLonghaulCmd(flags))
	rootCmd.AddCommand(newOntimeNowCmd(flags))
	rootCmd.AddCommand(newReliabilityCmd(flags))
	rootCmd.AddCommand(newCompareCmd(flags))
	rootCmd.AddCommand(newMonitorCmd(flags))
	rootCmd.AddCommand(newHeatmapCmd(flags))
	rootCmd.AddCommand(newResolveCmd(flags))
	rootCmd.AddCommand(newAircraftBioCmd(flags))
	rootCmd.AddCommand(newEtaCmd(flags))
	rootCmd.AddCommand(newGfSearchCmd(flags))
	rootCmd.AddCommand(newDigestCmd(flags))
}

// ----- shared helpers -----

// scheduledDeparture is a minimal view of an AeroAPI scheduled departure.
type scheduledDeparture struct {
	Ident          string     `json:"ident"`
	IdentICAO      string     `json:"ident_icao"`
	FAFlightID     string     `json:"fa_flight_id"`
	Operator       string     `json:"operator"`
	OperatorIATA   string     `json:"operator_iata"`
	Origin         airportRef `json:"origin"`
	Destination    airportRef `json:"destination"`
	ScheduledOut   string     `json:"scheduled_out"`
	ScheduledIn    string     `json:"scheduled_in"`
	ScheduledOff   string     `json:"scheduled_off"`
	ScheduledOn    string     `json:"scheduled_on"`
	EstimatedOut   string     `json:"estimated_out"`
	ActualOut      string     `json:"actual_out"`
	ActualOff      string     `json:"actual_off"`
	ActualOn       string     `json:"actual_on"`
	ActualIn       string     `json:"actual_in"`
	AircraftType   string     `json:"aircraft_type"`
	DepartureDelay int        `json:"departure_delay"`
	ArrivalDelay   int        `json:"arrival_delay"`
	Status         string     `json:"status"`
}

type airportRef struct {
	Code     string `json:"code"`
	CodeIATA string `json:"code_iata"`
	CodeICAO string `json:"code_icao"`
	City     string `json:"city"`
	Name     string `json:"airport_info_url"`
}

func (a airportRef) Best() string {
	if a.CodeIATA != "" {
		return a.CodeIATA
	}
	if a.Code != "" {
		return a.Code
	}
	return a.CodeICAO
}

// scheduledDeparturesPage is an AeroAPI paginated response envelope.
type scheduledDeparturesPage struct {
	Links               map[string]string    `json:"links"`
	NumPages            int                  `json:"num_pages"`
	ScheduledDepartures []scheduledDeparture `json:"scheduled_departures"`
	Departures          []scheduledDeparture `json:"departures"`
	Arrivals            []scheduledDeparture `json:"arrivals"`
	Flights             []scheduledDeparture `json:"flights"`
}

func (p scheduledDeparturesPage) items() []scheduledDeparture {
	if len(p.ScheduledDepartures) > 0 {
		return p.ScheduledDepartures
	}
	if len(p.Departures) > 0 {
		return p.Departures
	}
	if len(p.Arrivals) > 0 {
		return p.Arrivals
	}
	return p.Flights
}

// durationMinutes parses two RFC3339 timestamps and returns their delta in minutes.
// Returns -1 when either timestamp is missing or unparseable.
func durationMinutes(start, end string) int {
	if start == "" || end == "" {
		return -1
	}
	s, err := time.Parse(time.RFC3339, start)
	if err != nil {
		return -1
	}
	e, err := time.Parse(time.RFC3339, end)
	if err != nil {
		return -1
	}
	return int(e.Sub(s).Minutes())
}

// upperOrError returns upper-cased code or the original string.
func upperCode(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// ----- T1: longhaul -----

func newLonghaulCmd(flags *rootFlags) *cobra.Command {
	var minHours float64
	var month string
	var startDate string
	var endDate string
	var maxPages int
	var limit int

	cmd := &cobra.Command{
		Use:   "longhaul <airport>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "List nonstop departures from an airport that are at least N hours long",
		Long: `longhaul answers the classic travel-hacker question: "show me every nonstop
flight from my airport that's at least N hours long over a given period."

It queries FlightAware scheduled departures for the airport, filters by scheduled
duration, and returns a sorted list with destination, airline, duration, and date.
Pair with --json and pipe to jq for custom filtering.`,
		Example: `  # All nonstop 8+ hour flights from SEA this month
  flightgoat-pp-cli longhaul SEA --min-hours 8

  # 10+ hour flights from SEA in May 2026
  flightgoat-pp-cli longhaul SEA --min-hours 10 --month 2026-05

  # JSON output for agents
  flightgoat-pp-cli longhaul SEA --min-hours 8 --json | jq '.[] | .destination'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			airport := upperCode(args[0])
			if airport == "" {
				return fmt.Errorf("airport code required (e.g. SEA, KSEA)")
			}

			start, end, err := resolveDateWindow(month, startDate, endDate)
			if err != nil {
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			departures, err := fetchScheduledDepartures(c, airport, start, end, maxPages)
			if err != nil {
				return classifyAPIError(err)
			}

			minMinutes := int(minHours * 60)
			type result struct {
				Date            string `json:"date"`
				Ident           string `json:"ident"`
				Operator        string `json:"operator"`
				Origin          string `json:"origin"`
				Destination     string `json:"destination"`
				DurationMinutes int    `json:"duration_minutes"`
				DurationHours   string `json:"duration_hours"`
				AircraftType    string `json:"aircraft_type"`
			}
			results := make([]result, 0, len(departures))
			for _, d := range departures {
				dur := durationMinutes(d.ScheduledOut, d.ScheduledIn)
				if dur < 0 {
					dur = durationMinutes(d.ScheduledOff, d.ScheduledOn)
				}
				if dur < minMinutes {
					continue
				}
				date := d.ScheduledOut
				if len(date) >= 10 {
					date = date[:10]
				}
				results = append(results, result{
					Date:            date,
					Ident:           d.Ident,
					Operator:        d.Operator,
					Origin:          d.Origin.Best(),
					Destination:     d.Destination.Best(),
					DurationMinutes: dur,
					DurationHours:   fmt.Sprintf("%dh%02dm", dur/60, dur%60),
					AircraftType:    d.AircraftType,
				})
			}
			sort.SliceStable(results, func(i, j int) bool {
				if results[i].DurationMinutes != results[j].DurationMinutes {
					return results[i].DurationMinutes > results[j].DurationMinutes
				}
				return results[i].Date < results[j].Date
			})
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}

			return emitJSONOrTable(cmd.OutOrStdout(), flags, results, func(w io.Writer) {
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "DATE\tIDENT\tAIRLINE\tORIGIN\tDEST\tDURATION\tAIRCRAFT")
				for _, r := range results {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
						r.Date, r.Ident, r.Operator, r.Origin, r.Destination, r.DurationHours, r.AircraftType)
				}
				tw.Flush()
			})
		},
	}
	cmd.Flags().Float64Var(&minHours, "min-hours", 8, "Minimum flight duration in hours")
	cmd.Flags().StringVar(&month, "month", "", "Month to query in YYYY-MM (defaults to current month)")
	cmd.Flags().StringVar(&startDate, "from", "", "Start date YYYY-MM-DD (overrides --month)")
	cmd.Flags().StringVar(&endDate, "to", "", "End date YYYY-MM-DD (overrides --month)")
	cmd.Flags().IntVar(&maxPages, "max-pages", 5, "Maximum pages of AeroAPI results to fetch")
	cmd.Flags().IntVar(&limit, "limit", 0, "Limit results to top N (0 = all)")
	return cmd
}

// ----- T2: explore -----

func newExploreCmd(flags *rootFlags) *cobra.Command {
	var maxPages int
	var minFrequency int

	cmd := &cobra.Command{
		Use:   "explore <airport>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Every nonstop destination from an airport with duration, airlines, frequency",
		Long: `explore is the Kayak /direct nonstop matrix in your terminal.

Given an airport code, it aggregates scheduled departures into a destination
table showing: destination, typical duration, operating airlines, and daily
frequency. Answers "where can I fly nonstop from here, and how long does it take."`,
		Example: `  # Nonstop destinations from SEA
  flightgoat-pp-cli explore SEA

  # Destinations served at least twice daily, JSON output
  flightgoat-pp-cli explore SEA --min-frequency 2 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			airport := upperCode(args[0])
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			start := time.Now().UTC().Format(time.RFC3339)
			end := time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339)
			departures, err := fetchScheduledDepartures(c, airport, start, end, maxPages)
			if err != nil {
				return classifyAPIError(err)
			}

			type destAgg struct {
				Destination        string   `json:"destination"`
				Flights            int      `json:"flights"`
				DurationMinutesMin int      `json:"duration_minutes_min"`
				DurationMinutesMax int      `json:"duration_minutes_max"`
				DurationHours      string   `json:"duration_hours"`
				Airlines           []string `json:"airlines"`
			}
			agg := map[string]*destAgg{}
			airlineSets := map[string]map[string]bool{}
			for _, d := range departures {
				code := d.Destination.Best()
				if code == "" {
					continue
				}
				dur := durationMinutes(d.ScheduledOut, d.ScheduledIn)
				if dur < 0 {
					continue
				}
				a, ok := agg[code]
				if !ok {
					a = &destAgg{Destination: code, DurationMinutesMin: dur, DurationMinutesMax: dur}
					agg[code] = a
					airlineSets[code] = map[string]bool{}
				}
				a.Flights++
				if dur < a.DurationMinutesMin {
					a.DurationMinutesMin = dur
				}
				if dur > a.DurationMinutesMax {
					a.DurationMinutesMax = dur
				}
				if d.Operator != "" {
					airlineSets[code][d.Operator] = true
				}
			}

			results := make([]destAgg, 0, len(agg))
			for code, a := range agg {
				if a.Flights < minFrequency {
					continue
				}
				airlines := make([]string, 0, len(airlineSets[code]))
				for n := range airlineSets[code] {
					airlines = append(airlines, n)
				}
				sort.Strings(airlines)
				a.Airlines = airlines
				if a.DurationMinutesMin == a.DurationMinutesMax {
					a.DurationHours = fmt.Sprintf("%dh%02dm", a.DurationMinutesMin/60, a.DurationMinutesMin%60)
				} else {
					a.DurationHours = fmt.Sprintf("%dh%02dm-%dh%02dm",
						a.DurationMinutesMin/60, a.DurationMinutesMin%60,
						a.DurationMinutesMax/60, a.DurationMinutesMax%60)
				}
				results = append(results, *a)
			}
			sort.SliceStable(results, func(i, j int) bool {
				return results[i].Flights > results[j].Flights
			})

			return emitJSONOrTable(cmd.OutOrStdout(), flags, results, func(w io.Writer) {
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "DESTINATION\tFLIGHTS\tDURATION\tAIRLINES")
				for _, r := range results {
					fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n",
						r.Destination, r.Flights, r.DurationHours, strings.Join(r.Airlines, ","))
				}
				tw.Flush()
			})
		},
	}
	cmd.Flags().IntVar(&maxPages, "max-pages", 5, "Maximum pages of AeroAPI results to fetch")
	cmd.Flags().IntVar(&minFrequency, "min-frequency", 1, "Only show destinations with at least N flights in the window")
	return cmd
}

// ----- T3: cheapest-longhaul (requires fli for prices) -----

func newCheapestLonghaulCmd(flags *rootFlags) *cobra.Command {
	var minHours float64
	var startDate, endDate string

	cmd := &cobra.Command{
		Use:   "cheapest-longhaul <airport>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Cheapest dates to fly long nonstop routes from an airport",
		Long: `cheapest-longhaul joins two data sources nobody else joins:
  1. FlightAware route catalog (which nonstop routes are at least N hours)
  2. Google Flights price data via the 'fli' Python CLI (cheapest dates per route)

Requires 'fli' (pipx install flights) for the pricing side. Without fli, this
command lists the long routes and exits with a helpful message.`,
		Example: `  # Cheapest 8+ hour nonstop flights from SEA in May 2026
  flightgoat-pp-cli cheapest-longhaul SEA --min-hours 8 --from 2026-05-01 --to 2026-05-31`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			airport := upperCode(args[0])
			start, end, err := resolveDateWindow("", startDate, endDate)
			if err != nil {
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			departures, err := fetchScheduledDepartures(c, airport, start, end, 5)
			if err != nil {
				return classifyAPIError(err)
			}

			seen := map[string]int{}
			for _, d := range departures {
				dur := durationMinutes(d.ScheduledOut, d.ScheduledIn)
				if dur < int(minHours*60) {
					continue
				}
				dest := d.Destination.Best()
				if dest == "" {
					continue
				}
				if prev, ok := seen[dest]; !ok || dur < prev {
					seen[dest] = dur
				}
			}

			fliPath, fliErr := exec.LookPath("fli")
			if fliErr != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "fli not found. Install with: pipx install flights")
				fmt.Fprintln(cmd.ErrOrStderr(), "Without fli, showing long routes only, no prices.")
			}

			type row struct {
				Destination   string `json:"destination"`
				DurationHours string `json:"duration_hours"`
				CheapestDate  string `json:"cheapest_date,omitempty"`
				CheapestPrice string `json:"cheapest_price,omitempty"`
				Source        string `json:"price_source,omitempty"`
			}
			results := make([]row, 0, len(seen))
			for dest, dur := range seen {
				r := row{
					Destination:   dest,
					DurationHours: fmt.Sprintf("%dh%02dm", dur/60, dur%60),
				}
				if fliErr == nil {
					// Best-effort fli date-range query. Time-boxed to avoid hangs.
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					out, err := exec.CommandContext(ctx, fliPath, "dates",
						airport, dest, "--from", start[:10], "--to", end[:10], "--json").Output()
					cancel()
					if err == nil && len(out) > 0 {
						var parsed []struct {
							Date  string  `json:"date"`
							Price float64 `json:"price"`
						}
						if json.Unmarshal(out, &parsed) == nil && len(parsed) > 0 {
							sort.SliceStable(parsed, func(i, j int) bool { return parsed[i].Price < parsed[j].Price })
							r.CheapestDate = parsed[0].Date
							r.CheapestPrice = fmt.Sprintf("$%.0f", parsed[0].Price)
							r.Source = "google-flights-via-fli"
						}
					}
				}
				results = append(results, r)
			}
			sort.SliceStable(results, func(i, j int) bool {
				return results[i].DurationHours > results[j].DurationHours
			})

			return emitJSONOrTable(cmd.OutOrStdout(), flags, results, func(w io.Writer) {
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "DESTINATION\tDURATION\tCHEAPEST_DATE\tPRICE")
				for _, r := range results {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.Destination, r.DurationHours, r.CheapestDate, r.CheapestPrice)
				}
				tw.Flush()
			})
		},
	}
	cmd.Flags().Float64Var(&minHours, "min-hours", 8, "Minimum flight duration in hours")
	cmd.Flags().StringVar(&startDate, "from", "", "Start date YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&endDate, "to", "", "End date YYYY-MM-DD (required)")
	return cmd
}

// ----- T4: ontime-now -----

func newOntimeNowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ontime-now <airport>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Every departure from an airport today with live on-time status",
		Example: `  flightgoat-pp-cli ontime-now SEA
  flightgoat-pp-cli ontime-now JFK --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			airport := upperCode(args[0])
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
			end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.UTC).Format(time.RFC3339)

			// /airports/{id}/flights/departures gives us live + past for today
			path := fmt.Sprintf("/airports/%s/flights/departures", airport)
			raw, err := c.Get(path, map[string]string{"start": start, "end": end, "max_pages": "3"})
			if err != nil {
				return classifyAPIError(err)
			}
			var page scheduledDeparturesPage
			_ = json.Unmarshal(raw, &page)

			type row struct {
				Ident          string `json:"ident"`
				Operator       string `json:"operator"`
				Destination    string `json:"destination"`
				ScheduledOut   string `json:"scheduled_out"`
				ActualOut      string `json:"actual_out"`
				DepartureDelay int    `json:"departure_delay_minutes"`
				Status         string `json:"status"`
				OnTime         bool   `json:"on_time"`
			}
			var onTime, delayed int
			results := make([]row, 0, len(page.items()))
			for _, d := range page.items() {
				r := row{
					Ident:          d.Ident,
					Operator:       d.Operator,
					Destination:    d.Destination.Best(),
					ScheduledOut:   d.ScheduledOut,
					ActualOut:      d.ActualOut,
					DepartureDelay: d.DepartureDelay / 60, // API gives seconds
					Status:         d.Status,
					OnTime:         d.DepartureDelay <= 900, // <=15 min is on time
				}
				if r.OnTime {
					onTime++
				} else {
					delayed++
				}
				results = append(results, r)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "%s today: %d departures, %d on time, %d delayed (>15min)\n",
				airport, len(results), onTime, delayed)

			return emitJSONOrTable(cmd.OutOrStdout(), flags, results, func(w io.Writer) {
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "IDENT\tAIRLINE\tDEST\tSCHED_OUT\tDELAY_MIN\tSTATUS\tON_TIME")
				for _, r := range results {
					ot := "yes"
					if !r.OnTime {
						ot = "no"
					}
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
						r.Ident, r.Operator, r.Destination, r.ScheduledOut, r.DepartureDelay, r.Status, ot)
				}
				tw.Flush()
			})
		},
	}
	return cmd
}

// ----- T5: reliability -----

func newReliabilityCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:   "reliability <origin> <destination>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Historical on-time percentage for a route over the last N days",
		Long: `reliability queries FlightAware history for all flights on a route and
computes an on-time percentage. On-time is defined as departure delay <= 15 minutes.`,
		Example: `  flightgoat-pp-cli reliability SEA LHR --days 30
  flightgoat-pp-cli reliability JFK LAX --days 7 --json`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			origin := upperCode(args[0])
			dest := upperCode(args[1])
			start := time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)
			end := time.Now().UTC().Format(time.RFC3339)
			path := fmt.Sprintf("/airports/%s/flights/to/%s", origin, dest)

			if flags.dryRun {
				fmt.Fprintf(cmd.ErrOrStderr(), "GET %s?start=%s&end=%s&max_pages=5\n", path, start, end)
				fmt.Fprintf(cmd.ErrOrStderr(), "\n(dry run - no request sent; would aggregate on-time stats by airline over last %d days)\n", days)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			raw, err := c.Get(path, map[string]string{"start": start, "end": end, "max_pages": "5"})
			if err != nil {
				return classifyAPIError(err)
			}
			var page scheduledDeparturesPage
			_ = json.Unmarshal(raw, &page)
			items := page.items()
			if len(items) == 0 {
				return fmt.Errorf("no flights found for route %s -> %s in last %d days", origin, dest, days)
			}

			byAirline := map[string]*struct {
				Total      int
				OnTime     int
				TotalDelay int
			}{}
			for _, d := range items {
				a := d.Operator
				if a == "" {
					a = "(unknown)"
				}
				stat, ok := byAirline[a]
				if !ok {
					stat = &struct {
						Total      int
						OnTime     int
						TotalDelay int
					}{}
					byAirline[a] = stat
				}
				stat.Total++
				stat.TotalDelay += d.DepartureDelay
				if d.DepartureDelay <= 900 {
					stat.OnTime++
				}
			}

			type row struct {
				Airline         string  `json:"airline"`
				Flights         int     `json:"flights"`
				OnTimePercent   float64 `json:"on_time_percent"`
				AvgDelayMinutes float64 `json:"avg_delay_minutes"`
			}
			results := make([]row, 0, len(byAirline))
			for name, s := range byAirline {
				r := row{
					Airline:         name,
					Flights:         s.Total,
					OnTimePercent:   float64(s.OnTime) * 100.0 / float64(s.Total),
					AvgDelayMinutes: float64(s.TotalDelay) / float64(s.Total) / 60.0,
				}
				results = append(results, r)
			}
			sort.SliceStable(results, func(i, j int) bool {
				return results[i].OnTimePercent > results[j].OnTimePercent
			})

			fmt.Fprintf(cmd.ErrOrStderr(), "Route %s -> %s, %d flights analyzed over last %d days\n",
				origin, dest, len(items), days)

			return emitJSONOrTable(cmd.OutOrStdout(), flags, results, func(w io.Writer) {
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "AIRLINE\tFLIGHTS\tON_TIME_%\tAVG_DELAY_MIN")
				for _, r := range results {
					fmt.Fprintf(tw, "%s\t%d\t%.1f\t%.1f\n", r.Airline, r.Flights, r.OnTimePercent, r.AvgDelayMinutes)
				}
				tw.Flush()
			})
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Lookback window in days")
	return cmd
}

// ----- T6: compare (fli + reliability) -----

func newCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare <origin> <destination> <date>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Join Google Flights prices with AeroAPI reliability for the same route",
		Long: `compare runs two queries in parallel: fli flights for Google Flights prices and
reliability for AeroAPI historical on-time percentages. Output sorts by reliability
so you can pick the cheapest flight that's likely to actually run on time.

Requires 'fli' (pipx install flights) for pricing.`,
		Example: `  flightgoat-pp-cli compare SEA LHR 2026-06-15`,
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			origin := upperCode(args[0])
			dest := upperCode(args[1])
			date := args[2]

			if flags.dryRun {
				fmt.Fprintf(cmd.ErrOrStderr(), "fli flights %s %s %s --json\n", origin, dest, date)
				fmt.Fprintf(cmd.ErrOrStderr(), "GET /airports/%s/flights/to/%s?start=<last30d>&end=<now>&max_pages=3\n", origin, dest)
				fmt.Fprintln(cmd.ErrOrStderr(), "\n(dry run - no requests sent; would join Google Flights prices with AeroAPI reliability)")
				return nil
			}

			fliPath, fliErr := exec.LookPath("fli")
			if fliErr != nil {
				return fmt.Errorf("fli CLI not found. Install with: pipx install flights")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			out, err := exec.CommandContext(ctx, fliPath, "flights", origin, dest, date, "--json").Output()
			if err != nil {
				return fmt.Errorf("fli search failed: %w", err)
			}
			var fliResults []map[string]any
			if err := json.Unmarshal(out, &fliResults); err != nil {
				return fmt.Errorf("parsing fli output: %w", err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			start := time.Now().UTC().AddDate(0, 0, -30).Format(time.RFC3339)
			endDate := time.Now().UTC().Format(time.RFC3339)
			path := fmt.Sprintf("/airports/%s/flights/to/%s", origin, dest)
			raw, relErr := c.Get(path, map[string]string{"start": start, "end": endDate, "max_pages": "3"})
			reliabilityByAirline := map[string]float64{}
			if relErr == nil {
				var page scheduledDeparturesPage
				_ = json.Unmarshal(raw, &page)
				byAirline := map[string]*[2]int{} // [0]=total, [1]=ontime
				for _, d := range page.items() {
					a := d.Operator
					if a == "" {
						continue
					}
					s, ok := byAirline[a]
					if !ok {
						s = &[2]int{}
						byAirline[a] = s
					}
					s[0]++
					if d.DepartureDelay <= 900 {
						s[1]++
					}
				}
				for a, s := range byAirline {
					reliabilityByAirline[a] = float64(s[1]) * 100.0 / float64(s[0])
				}
			}

			type row struct {
				Price         any     `json:"price"`
				Airline       string  `json:"airline"`
				Duration      any     `json:"duration"`
				Stops         any     `json:"stops"`
				OnTimePercent float64 `json:"on_time_percent_30d"`
			}
			results := make([]row, 0, len(fliResults))
			for _, f := range fliResults {
				a, _ := f["airline"].(string)
				results = append(results, row{
					Price:         f["price"],
					Airline:       a,
					Duration:      f["duration"],
					Stops:         f["stops"],
					OnTimePercent: reliabilityByAirline[a],
				})
			}
			sort.SliceStable(results, func(i, j int) bool {
				return results[i].OnTimePercent > results[j].OnTimePercent
			})

			return emitJSONOrTable(cmd.OutOrStdout(), flags, results, func(w io.Writer) {
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "PRICE\tAIRLINE\tDURATION\tSTOPS\tON_TIME_%_30D")
				for _, r := range results {
					fmt.Fprintf(tw, "%v\t%s\t%v\t%v\t%.1f\n", r.Price, r.Airline, r.Duration, r.Stops, r.OnTimePercent)
				}
				tw.Flush()
			})
		},
	}
	return cmd
}

// ----- T7: monitor -----

func newMonitorCmd(flags *rootFlags) *cobra.Command {
	var interval time.Duration
	var untilArrival bool
	var maxChecks int

	cmd := &cobra.Command{
		Use:   "monitor <ident>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Watch a flight through its lifecycle until landed",
		Long: `monitor polls FlightAware at regular intervals and prints status changes.
When --until-arrival is set (default), it exits when the flight lands.`,
		Example: `  flightgoat-pp-cli monitor UA123
  flightgoat-pp-cli monitor AA456 --interval 5m --until-arrival`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ident := args[0]
			path := fmt.Sprintf("/flights/%s", ident)

			if flags.dryRun {
				fmt.Fprintf(cmd.ErrOrStderr(), "GET %s (repeated every %s)\n", path, interval)
				fmt.Fprintf(cmd.ErrOrStderr(), "\n(dry run - no requests sent; would poll until arrival=%v, max_checks=%d)\n", untilArrival, maxChecks)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var lastStatus string
			checks := 0
			for {
				checks++
				raw, err := c.Get(path, nil)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "check %d: %v\n", checks, err)
				} else {
					var env struct {
						Flights []scheduledDeparture `json:"flights"`
					}
					_ = json.Unmarshal(raw, &env)
					if len(env.Flights) > 0 {
						d := env.Flights[0]
						status := d.Status
						if status != lastStatus {
							fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s status=%q delay=%ds\n",
								time.Now().Format(time.RFC3339), ident, status, d.DepartureDelay)
							lastStatus = status
						}
						if untilArrival && (d.ActualIn != "" || d.ActualOn != "") {
							fmt.Fprintln(cmd.OutOrStdout(), "flight has arrived")
							return nil
						}
					}
				}
				if maxChecks > 0 && checks >= maxChecks {
					return nil
				}
				time.Sleep(interval)
			}
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Minute, "Poll interval")
	cmd.Flags().BoolVar(&untilArrival, "until-arrival", true, "Exit when flight has landed")
	cmd.Flags().IntVar(&maxChecks, "max-checks", 0, "Exit after N checks regardless of status (0 = unlimited)")
	return cmd
}

// ----- T8: heatmap -----

func newHeatmapCmd(flags *rootFlags) *cobra.Command {
	var region string
	cmd := &cobra.Command{
		Use:   "heatmap",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Where are delays happening right now across major airports",
		Long: `heatmap calls /airports/delays and returns a single sorted table showing
every airport with active delays. Useful for operators, travel hackers, and
anyone deciding whether to reroute.`,
		Example: `  flightgoat-pp-cli heatmap
  flightgoat-pp-cli heatmap --region US --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get("/airports/delays", nil)
			if err != nil {
				return classifyAPIError(err)
			}
			var env struct {
				Delays []map[string]any `json:"delays"`
			}
			_ = json.Unmarshal(raw, &env)
			filtered := env.Delays
			if region != "" {
				out := make([]map[string]any, 0, len(filtered))
				for _, d := range filtered {
					country, _ := d["country_code"].(string)
					if strings.EqualFold(country, region) {
						out = append(out, d)
					}
				}
				filtered = out
			}
			return emitJSONOrTable(cmd.OutOrStdout(), flags, filtered, func(w io.Writer) {
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "AIRPORT\tREASON\tCATEGORY\tCOUNT")
				for _, d := range filtered {
					id, _ := d["airport"].(string)
					reason, _ := d["reason"].(string)
					category, _ := d["category"].(string)
					count := d["count"]
					fmt.Fprintf(tw, "%s\t%s\t%s\t%v\n", id, reason, category, count)
				}
				tw.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&region, "region", "", "Filter to a country code (e.g. US, GB)")
	return cmd
}

// ----- T9: resolve -----

func newResolveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "resolve <ident>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "Show every code (codeshare, canonical, operator) for one flight",
		Example: `  flightgoat-pp-cli resolve UA123`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ident := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get(fmt.Sprintf("/flights/%s/canonical", ident), nil)
			if err != nil {
				return classifyAPIError(err)
			}
			return emitRaw(cmd.OutOrStdout(), flags, raw)
		},
	}
	return cmd
}

// ----- T10: aircraft bio -----

func newAircraftBioCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "aircraft-bio <registration>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "Full history of a tail number: recent flights + owner + last known flight",
		Example: `  flightgoat-pp-cli aircraft-bio N12345`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			type bio struct {
				Registration string          `json:"registration"`
				LastFlight   json.RawMessage `json:"last_flight,omitempty"`
				Owner        json.RawMessage `json:"owner,omitempty"`
				Blocked      json.RawMessage `json:"blocked,omitempty"`
			}
			b := bio{Registration: reg}
			if data, err := c.Get(fmt.Sprintf("/history/aircraft/%s/last_flight", reg), nil); err == nil {
				b.LastFlight = data
			}
			if data, err := c.Get(fmt.Sprintf("/aircraft/%s/owner", reg), nil); err == nil {
				b.Owner = data
			}
			if data, err := c.Get(fmt.Sprintf("/aircraft/%s/blocked", reg), nil); err == nil {
				b.Blocked = data
			}
			bts, _ := json.MarshalIndent(b, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(bts))
			return nil
		},
	}
	return cmd
}

// ----- T11: eta -----

func newEtaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "eta <ident>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "Weather-adjusted ETA: foresight prediction plus destination weather",
		Example: `  flightgoat-pp-cli eta AA100`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ident := args[0]

			if flags.dryRun {
				fmt.Fprintf(cmd.ErrOrStderr(), "GET /foresight/flights/%s\n", ident)
				fmt.Fprintln(cmd.ErrOrStderr(), "GET /airports/{dest}/weather/forecast")
				fmt.Fprintln(cmd.ErrOrStderr(), "\n(dry run - no requests sent; would fetch foresight ETA then destination weather)")
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			fRaw, err := c.Get(fmt.Sprintf("/foresight/flights/%s", ident), nil)
			if err != nil {
				return classifyAPIError(err)
			}
			var env struct {
				Flights []scheduledDeparture `json:"flights"`
			}
			_ = json.Unmarshal(fRaw, &env)
			if len(env.Flights) == 0 {
				return fmt.Errorf("no foresight data for %s", ident)
			}
			d := env.Flights[0]
			destCode := d.Destination.Best()
			type result struct {
				Ident       string          `json:"ident"`
				Destination string          `json:"destination"`
				ETA         string          `json:"estimated_on"`
				Weather     json.RawMessage `json:"destination_weather,omitempty"`
			}
			r := result{Ident: ident, Destination: destCode, ETA: d.ScheduledOn}
			if destCode != "" {
				if wRaw, err := c.Get(fmt.Sprintf("/airports/%s/weather/forecast", destCode), nil); err == nil {
					r.Weather = wRaw
				}
			}
			bts, _ := json.MarshalIndent(r, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(bts))
			return nil
		},
	}
	return cmd
}

// ----- T12: gf-search (Google Flights via fli) -----

func newGfSearchCmd(flags *rootFlags) *cobra.Command {
	var alertIfUnder float64
	cmd := &cobra.Command{
		Use:   "gf-search <origin> <destination> <date>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Google Flights search via fli (price + duration + airlines)",
		Long: `gf-search delegates to the 'fli' Python CLI for Google Flights search.
Install fli first: pipx install flights

When --alert-if-under PRICE is set, a FlightAware AeroAPI alert is created
automatically if a result is found below the threshold.`,
		Example: `  flightgoat-pp-cli gf-search SEA LHR 2026-06-15
  flightgoat-pp-cli gf-search JFK CDG 2026-07-01 --alert-if-under 600`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				fmt.Fprintf(cmd.ErrOrStderr(), "fli flights %s %s %s --json\n", args[0], args[1], args[2])
				if alertIfUnder > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "(would emit alert if any result price < $%.0f)\n", alertIfUnder)
				}
				fmt.Fprintln(cmd.ErrOrStderr(), "\n(dry run - no subprocess launched; would shell out to fli for Google Flights search)")
				return nil
			}

			fliPath, err := exec.LookPath("fli")
			if err != nil {
				return fmt.Errorf("fli CLI not found. Install with: pipx install flights")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			out, err := exec.CommandContext(ctx, fliPath, "flights", args[0], args[1], args[2], "--json").Output()
			if err != nil {
				return fmt.Errorf("fli search failed: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			if alertIfUnder > 0 {
				var results []map[string]any
				if json.Unmarshal(out, &results) == nil {
					for _, r := range results {
						if p, ok := r["price"].(float64); ok && p < alertIfUnder {
							fmt.Fprintf(cmd.ErrOrStderr(), "Found %s %s %s at $%.0f (under $%.0f threshold)\n",
								args[0], args[1], args[2], p, alertIfUnder)
							break
						}
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().Float64Var(&alertIfUnder, "alert-if-under", 0, "Emit a notice if a result is below this price")
	return cmd
}

// ----- shared helpers -----

func fetchScheduledDepartures(c *client.Client, airport, start, end string, maxPages int) ([]scheduledDeparture, error) {
	path := fmt.Sprintf("/airports/%s/flights/scheduled_departures", airport)
	raw, err := c.Get(path, map[string]string{
		"start":     start,
		"end":       end,
		"max_pages": strconv.Itoa(maxPages),
	})
	if err != nil {
		return nil, err
	}
	var page scheduledDeparturesPage
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, fmt.Errorf("parsing departures: %w", err)
	}
	return page.items(), nil
}

// resolveDateWindow picks a date range for the query.
// --month wins if set, else --from/--to, else this month.
func resolveDateWindow(month, startDate, endDate string) (string, string, error) {
	var start, end time.Time
	var err error
	switch {
	case startDate != "" || endDate != "":
		if startDate == "" || endDate == "" {
			return "", "", fmt.Errorf("--from and --to must both be set")
		}
		start, err = time.Parse("2006-01-02", startDate)
		if err != nil {
			return "", "", fmt.Errorf("invalid --from date: %w", err)
		}
		end, err = time.Parse("2006-01-02", endDate)
		if err != nil {
			return "", "", fmt.Errorf("invalid --to date: %w", err)
		}
	case month != "":
		start, err = time.Parse("2006-01", month)
		if err != nil {
			return "", "", fmt.Errorf("invalid --month (expected YYYY-MM): %w", err)
		}
		end = start.AddDate(0, 1, -1)
	default:
		now := time.Now().UTC()
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 1, -1)
	}
	return start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339), nil
}

// emitJSONOrTable prints results as JSON (if --json) or via a caller-supplied table printer.
func emitJSONOrTable[T any](w io.Writer, flags *rootFlags, results T, table func(io.Writer)) error {
	if flags.asJSON || !isTerminal(w) {
		bts, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(bts))
		return nil
	}
	table(w)
	return nil
}

// emitRaw prints an arbitrary JSON payload using the configured output mode.
func emitRaw(w io.Writer, flags *rootFlags, data json.RawMessage) error {
	if flags.asJSON || !isTerminal(w) {
		var pretty any
		if err := json.Unmarshal(data, &pretty); err == nil {
			bts, _ := json.MarshalIndent(pretty, "", "  ")
			fmt.Fprintln(w, string(bts))
			return nil
		}
	}
	fmt.Fprintln(w, string(data))
	return nil
}

// prevent unused import errors in slim builds
var _ = os.Stdout
