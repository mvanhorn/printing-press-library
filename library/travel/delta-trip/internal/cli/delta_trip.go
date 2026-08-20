package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/delta-trip/internal/delta"
	"github.com/mvanhorn/printing-press-library/library/travel/delta-trip/internal/store"
	"github.com/spf13/cobra"
)

const (
	tripCacheTTL  = 4 * time.Hour
	tripCacheType = "trips"
	cliName       = "delta-trip-pp-cli"
)

// newTripCmd creates the `trip` parent command with `show` and `flights` subcommands.
func newTripCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trip",
		Short: "Manage Delta trips by confirmation number",
		Long:  "Look up and display Delta Air Lines trip details using a confirmation number (no login required).",
	}
	cmd.AddCommand(newTripShowCmd(flags))
	cmd.AddCommand(newTripFlightsCmd(flags))
	cmd.AddCommand(newCheckinStatusCmd(flags))
	cmd.AddCommand(newTripSeatMapCmd(flags))
	cmd.AddCommand(newLayoverRiskCmd(flags))
	return cmd
}

// newTripShowCmd implements `delta-trip trip show CONF FIRST LAST`.
func newTripShowCmd(flags *rootFlags) *cobra.Command {
	var flagNoCache bool
	cmd := &cobra.Command{
		Use:     "show <confirmation> <first-name> <last-name>",
		Short:   "Show full trip details for a confirmation number",
		Example: "  delta-trip trip show ABC123 JANE SMITH",
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			conf, first, last := strings.ToUpper(args[0]), strings.ToUpper(args[1]), strings.ToUpper(args[2])
			trip, err := fetchAndCacheTrip(cmd.Context(), conf, first, last, flags, flagNoCache)
			if err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				b, _ := json.MarshalIndent(trip, "", "  ")
				wrapped, _ := wrapWithProvenance(b, DataProvenance{Source: "live", ResourceType: tripCacheType})
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			return printTripTable(cmd.OutOrStdout(), trip)
		},
	}
	cmd.Flags().BoolVar(&flagNoCache, "no-cache", false, "Bypass local cache and fetch live from delta.com")
	return cmd
}

// newTripFlightsCmd implements `delta-trip trip flights CONF FIRST LAST`.
func newTripFlightsCmd(flags *rootFlags) *cobra.Command {
	var flagNoCache bool
	var flagFlight int
	cmd := &cobra.Command{
		Use:     "flights <confirmation> <first-name> <last-name>",
		Short:   "List flights in a trip itinerary",
		Example: "  delta-trip trip flights ABC123 JANE SMITH",
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			conf, first, last := strings.ToUpper(args[0]), strings.ToUpper(args[1]), strings.ToUpper(args[2])
			trip, err := fetchAndCacheTrip(cmd.Context(), conf, first, last, flags, flagNoCache)
			if err != nil {
				return err
			}
			flights := trip.Flights
			if flagFlight > 0 && flagFlight <= len(flights) {
				flights = []*delta.Flight{flights[flagFlight-1]}
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				b, _ := json.MarshalIndent(flights, "", "  ")
				wrapped, _ := wrapWithProvenance(b, DataProvenance{Source: "live", ResourceType: "flights"})
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			return printFlightsTable(cmd.OutOrStdout(), flights)
		},
	}
	cmd.Flags().BoolVar(&flagNoCache, "no-cache", false, "Bypass local cache and fetch live from delta.com")
	cmd.Flags().IntVar(&flagFlight, "flight", 0, "Show only this flight number (1-indexed)")
	return cmd
}

// newCheckinStatusCmd implements `delta-trip trip checkin CONF FIRST LAST`.
func newCheckinStatusCmd(flags *rootFlags) *cobra.Command {
	var flagNoCache bool
	cmd := &cobra.Command{
		Use:     "checkin <confirmation> <first-name> <last-name>",
		Short:   "Check check-in window status for a trip",
		Example: "  delta-trip trip checkin ABC123 JANE SMITH",
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			conf, first, last := strings.ToUpper(args[0]), strings.ToUpper(args[1]), strings.ToUpper(args[2])
			trip, err := fetchAndCacheTrip(cmd.Context(), conf, first, last, flags, flagNoCache)
			if err != nil {
				return err
			}
			return printCheckinStatus(cmd.OutOrStdout(), trip, flags)
		},
	}
	cmd.Flags().BoolVar(&flagNoCache, "no-cache", false, "Bypass local cache and fetch live from delta.com")
	return cmd
}

// fetchAndCacheTrip looks up a trip using the local SQLite cache with TTL,
// falling back to the browser scraper when stale or not cached.
func fetchAndCacheTrip(ctx context.Context, conf, first, last string, flags *rootFlags, noCache bool) (*delta.TripResult, error) {
	cacheKey := strings.ToUpper(conf)
	dbPath := defaultDBPath(cliName)

	// Try local cache first (unless --no-cache or --data-source=live).
	useCache := !noCache && flags.dataSource != "live"
	if useCache {
		if trip, ok := loadCachedTrip(ctx, dbPath, cacheKey); ok {
			return trip, nil
		}
	}

	// --data-source=local means only read from cache; never hit the browser.
	if flags.dataSource == "local" {
		return nil, fmt.Errorf("no cached trip found for %s. Remove --data-source=local or run without it to fetch live.", conf)
	}

	// Browser scrape.
	fmt.Fprintf(os.Stderr, "Fetching trip %s from delta.com (this opens a browser window)...\n", conf)
	timeout := flags.timeout
	if timeout < 60*time.Second {
		timeout = 60 * time.Second
	}
	scrapeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	trip, err := delta.GetTrip(scrapeCtx, conf, first, last)
	if err != nil {
		return nil, fmt.Errorf("fetching trip from delta.com: %w", err)
	}

	// Cache the result synchronously to ensure it completes before exit.
	cacheTripResult(dbPath, cacheKey, trip)

	return trip, nil
}

// loadCachedTrip reads a TripResult from SQLite if it exists and is within TTL.
func loadCachedTrip(ctx context.Context, dbPath, cacheKey string) (*delta.TripResult, bool) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, false
	}
	db, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return nil, false
	}
	defer db.Close()

	raw, err := db.Get(tripCacheType, cacheKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false
		}
		return nil, false
	}

	// Check TTL via store's sync state.
	_, lastSynced, _, stateErr := db.GetSyncState(tripCacheType + "/" + cacheKey)
	if stateErr == nil && !lastSynced.IsZero() && time.Since(lastSynced) > tripCacheTTL {
		return nil, false
	}

	var trip delta.TripResult
	if err := json.Unmarshal(raw, &trip); err != nil {
		return nil, false
	}
	return &trip, true
}

// cacheTripResult stores a TripResult in the SQLite store.
func cacheTripResult(dbPath, cacheKey string, trip *delta.TripResult) {
	db, err := store.Open(dbPath)
	if err != nil {
		return
	}
	defer db.Close()

	b, err := json.Marshal(trip)
	if err != nil {
		return
	}
	_ = db.Upsert(tripCacheType, cacheKey, b)
	_ = db.SaveSyncState(tripCacheType+"/"+cacheKey, "", 1)
}

// printTripTable renders a human-friendly full trip summary.
func printTripTable(w io.Writer, trip *delta.TripResult) error {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)

	fmt.Fprintf(tw, "Confirmation:\t%s\n", trip.ConfirmationNumber)
	if trip.Destination != "" {
		fmt.Fprintf(tw, "Destination:\t%s\n", trip.Destination)
	}
	if trip.TripType != "" {
		fmt.Fprintf(tw, "Type:\t%s\n", trip.TripType)
	}
	if trip.StatusBadge != "" {
		fmt.Fprintf(tw, "Status:\t%s\n", trip.StatusBadge)
	}
	if len(trip.Alerts) > 0 {
		fmt.Fprintf(tw, "Alerts:\t%s\n", strings.Join(trip.Alerts, " | "))
	}
	tw.Flush()

	fmt.Fprintln(w)

	for _, f := range trip.Flights {
		printFlightBlock(w, f)
	}
	return nil
}

func printFlightBlock(w io.Writer, f *delta.Flight) {
	fmt.Fprintf(w, "─── Flight %s: %s ──────────────────────\n", f.FlightIndex, f.FlightNumber)
	if f.Aircraft != "" {
		fmt.Fprintf(w, "  Aircraft:   %s\n", f.Aircraft)
	}
	if f.OperatedBy != "" {
		fmt.Fprintf(w, "  Operated:   %s\n", f.OperatedBy)
	}
	if f.Status != "" {
		fmt.Fprintf(w, "  Status:     %s\n", f.Status)
	}
	if f.Duration != "" {
		fmt.Fprintf(w, "  Duration:   %s\n", f.Duration)
	}

	dep := f.Departure
	arr := f.Arrival
	fmt.Fprintf(w, "  Depart:     %s %s  %s (%s)  Terminal %s  Gate %s\n",
		dep.Date, dep.Time, dep.City, dep.Airport, orTBD(dep.Terminal), orTBD(dep.Gate))
	fmt.Fprintf(w, "  Arrive:     %s %s  %s (%s)  Terminal %s  Gate %s\n",
		arr.Date, arr.Time, arr.City, arr.Airport, orTBD(arr.Terminal), orTBD(arr.Gate))

	if len(f.Passengers) > 0 {
		tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "  PASSENGER\tSEAT\tFARE CLASS\tETICKET")
		for _, p := range f.Passengers {
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", p.Name, p.Seat, p.FareClass, p.ETicket)
		}
		tw.Flush()
	}

	if f.Layover != nil {
		risk := ""
		if f.Layover.RiskLevel == "HIGH" {
			risk = " ⚠ HIGH RISK"
		} else if f.Layover.RiskLevel == "TIGHT" {
			risk = " ⚠ TIGHT"
		}
		fmt.Fprintf(w, "  Layover:    %s in %s (%s)%s\n", f.Layover.Duration, f.Layover.City, f.Layover.Airport, risk)
	}
	fmt.Fprintln(w)
}

func printFlightsTable(w io.Writer, flights []*delta.Flight) error {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "#\tFLIGHT\tROUTE\tDEPARTS\tARRIVES\tSTATUS\tDURATION")
	for _, f := range flights {
		route := f.Departure.Airport + "→" + f.Arrival.Airport
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s %s\t%s %s\t%s\t%s\n",
			f.FlightIndex,
			f.FlightNumber,
			route,
			f.Departure.Date, f.Departure.Time,
			f.Arrival.Date, f.Arrival.Time,
			orTBD(f.Status),
			f.Duration,
		)
	}
	return tw.Flush()
}

func printCheckinStatus(w io.Writer, trip *delta.TripResult, flags *rootFlags) error {
	type checkinResult struct {
		Flight      string `json:"flight"`
		Departure   string `json:"departure"`
		OpenAt      string `json:"checkinOpensAt"`
		IsOpen      bool   `json:"isOpen"`
		OpensInSecs int64  `json:"opensInSeconds,omitempty"`
	}

	var results []checkinResult
	now := time.Now()

	for _, f := range trip.Flights {
		depStr := f.Departure.Date + " " + f.Departure.Time
		var depTime time.Time
		for _, layout := range []string{"Mon, Jan 2 3:04 PM", "Jan 2, 2006 3:04 PM", "2006-01-02 15:04"} {
			t, err := time.Parse(layout, depStr)
			if err == nil {
				depTime = t.AddDate(now.Year(), 0, 0)
				break
			}
		}

		checkinOpen := depTime.Add(-24 * time.Hour)
		isOpen := !depTime.IsZero() && now.After(checkinOpen) && now.Before(depTime)
		openAt := "unknown"
		var opensInSecs int64
		if !depTime.IsZero() {
			openAt = checkinOpen.Format("Mon, Jan 2 at 3:04 PM MST")
			if !isOpen {
				opensInSecs = int64(time.Until(checkinOpen).Seconds())
			}
		}

		results = append(results, checkinResult{
			Flight:      f.FlightNumber,
			Departure:   depStr,
			OpenAt:      openAt,
			IsOpen:      isOpen,
			OpensInSecs: opensInSecs,
		})
	}

	if flags.asJSON || !isTerminal(w) {
		b, _ := json.MarshalIndent(results, "", "  ")
		_, err := fmt.Fprintln(w, string(b))
		return err
	}

	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "FLIGHT\tDEPARTURE\tCHECK-IN OPENS\tSTATUS")
	for _, r := range results {
		status := "NOT OPEN"
		if r.IsOpen {
			status = "OPEN NOW"
		} else if r.OpensInSecs > 0 {
			hrs := r.OpensInSecs / 3600
			mins := (r.OpensInSecs % 3600) / 60
			if hrs > 0 {
				status = fmt.Sprintf("opens in %dh %dm", hrs, mins)
			} else {
				status = fmt.Sprintf("opens in %dm", mins)
			}
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.Flight, r.Departure, r.OpenAt, status)
	}
	return tw.Flush()
}

func orTBD(s string) string {
	if s == "" || strings.EqualFold(s, "tbd") {
		return "TBD"
	}
	return s
}

// newLayoverRiskCmd implements `delta-trip trip layover CONF FIRST LAST`.
// It fetches trip data and produces a focused connection risk report — each
// layover scored as OK / TIGHT / HIGH with an agent-ready recommended action.
// Unlike `trip flights` (which buries layover info in full flight detail rows),
// this command returns only the connection data, structured for agent decision-making.
func newLayoverRiskCmd(flags *rootFlags) *cobra.Command {
	var flagNoCache bool
	cmd := &cobra.Command{
		Use:   "layover [confirmation] [first-name] [last-name]",
		Short: "Score connection risk for each layover in a trip",
		Long: "Analyzes each connection in a trip itinerary and rates risk as OK, TIGHT, or HIGH " +
			"using minimum connection time (MCT) thresholds: domestic connections under 45 min are HIGH, " +
			"under 90 min are TIGHT; international connections under 90 min are HIGH, under 120 min are TIGHT. " +
			"Returns an overall risk rating and a recommended action per connection.",
		Example: strings.Trim(`
  delta-trip-pp-cli trip layover ABC123 JANE SMITH
  delta-trip-pp-cli trip layover ABC123 JANE SMITH --json
  delta-trip-pp-cli trip layover ABC123 JANE SMITH --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) != 3 {
				return fmt.Errorf("usage: trip layover <confirmation> <first-name> <last-name>")
			}
			conf, first, last := strings.ToUpper(args[0]), strings.ToUpper(args[1]), strings.ToUpper(args[2])
			trip, err := fetchAndCacheTrip(cmd.Context(), conf, first, last, flags, flagNoCache)
			if err != nil {
				return err
			}
			return printLayoverRisk(cmd.OutOrStdout(), trip, flags)
		},
	}
	cmd.Flags().BoolVar(&flagNoCache, "no-cache", false, "Bypass local cache and fetch live from delta.com")
	return cmd
}

type layoverRiskReport struct {
	Confirmation string     `json:"confirmation"`
	Connections  []connRisk `json:"connections"`
	OverallRisk  string     `json:"overallRisk"`
}

type connRisk struct {
	FromFlight    string `json:"fromFlight"`
	ToFlight      string `json:"toFlight"`
	Airport       string `json:"airport"`
	City          string `json:"city"`
	Duration      string `json:"duration"`
	Minutes       int    `json:"minutes"`
	RiskLevel     string `json:"riskLevel"`
	International bool   `json:"international"`
	Action        string `json:"action"`
}

func printLayoverRisk(w io.Writer, trip *delta.TripResult, flags *rootFlags) error {
	report := layoverRiskReport{Confirmation: trip.ConfirmationNumber}

	for i, f := range trip.Flights {
		if f.Layover == nil {
			continue
		}
		ly := f.Layover

		var action string
		switch ly.RiskLevel {
		case "HIGH":
			action = "Request gate assistance on arrival or consider rebooking if departure is delayed."
		case "TIGHT":
			action = "Move quickly on arrival; avoid checked bags if possible."
		default:
			action = "Comfortable connection — no special action needed."
		}

		toFlight := ""
		if i+1 < len(trip.Flights) {
			toFlight = trip.Flights[i+1].FlightNumber
		}

		report.Connections = append(report.Connections, connRisk{
			FromFlight:    f.FlightNumber,
			ToFlight:      toFlight,
			Airport:       ly.Airport,
			City:          ly.City,
			Duration:      ly.Duration,
			Minutes:       ly.RiskMinutes,
			RiskLevel:     ly.RiskLevel,
			International: ly.International,
			Action:        action,
		})
	}

	if len(report.Connections) == 0 {
		if flags.asJSON || !isTerminal(w) {
			report.OverallRisk = "N/A"
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}
		fmt.Fprintln(w, "No connections — this appears to be a direct flight.")
		return nil
	}

	overallRisk := "OK"
	for _, c := range report.Connections {
		if c.RiskLevel == "HIGH" {
			overallRisk = "HIGH"
			break
		} else if c.RiskLevel == "TIGHT" && overallRisk == "OK" {
			overallRisk = "TIGHT"
		}
	}
	report.OverallRisk = overallRisk

	if flags.asJSON || !isTerminal(w) {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	riskLabel := map[string]string{
		"OK":    "✓ OK",
		"TIGHT": "⚠ TIGHT",
		"HIGH":  "✖ HIGH RISK",
	}

	fmt.Fprintf(w, "Connection Risk — %s\n\n", trip.ConfirmationNumber)
	for _, c := range report.Connections {
		connType := "domestic"
		if c.International {
			connType = "international"
		}
		fmt.Fprintf(w, "  %s → %s  via %s (%s)\n", c.FromFlight, c.ToFlight, c.City, c.Airport)
		fmt.Fprintf(w, "  %s  %dm  %s connection\n", riskLabel[c.RiskLevel], c.Minutes, connType)
		fmt.Fprintf(w, "  → %s\n\n", c.Action)
	}
	fmt.Fprintf(w, "Overall: %s\n", riskLabel[overallRisk])
	return nil
}
