// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// assess is a compound delay decision command. It joins AeroAPI airport delay,
// disruption, weather, route, and flight-status data with FAA NAS status and
// optional Google Flights price context.

package cli

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/flight-goat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/flight-goat/internal/gflights"

	"github.com/spf13/cobra"
)

const defaultNASStatusURL = "https://nasstatus.faa.gov/api/airport-status-information"

var metarGustKTRE = regexp.MustCompile(`\b([0-9]{3}|VRB)[0-9]{2,3}G[0-9]{2,3}KT\b`)

type assessOptions struct {
	origin            string
	destination       string
	delayedFlight     string
	date              string
	departAfter       string
	lookahead         time.Duration
	maxAlternatives   int
	maxInboundLookups int
	flightType        string
	connection        string
	currency          string
	noNAS             bool
	nasURL            string
	noPrices          bool
	includeRaw        bool
}

type airportCodes struct {
	Input string `json:"input"`
	IATA  string `json:"iata"`
	ICAO  string `json:"icao"`
	Note  string `json:"note,omitempty"`
}

type assessQuery struct {
	Origin            airportCodes `json:"origin"`
	Destination       airportCodes `json:"destination"`
	DepartureDate     string       `json:"departure_date"`
	DepartAfter       string       `json:"depart_after"`
	Lookahead         string       `json:"lookahead"`
	DelayedFlight     string       `json:"delayed_flight,omitempty"`
	MaxAlternatives   int          `json:"max_alternatives"`
	MaxInboundLookups int          `json:"max_inbound_lookups"`
	FlightType        string       `json:"flight_type,omitempty"`
	Connection        string       `json:"connection,omitempty"`
	IncludesNAS       bool         `json:"includes_nas"`
	IncludesPrices    bool         `json:"includes_prices"`
}

type assessDecision struct {
	Verdict         string   `json:"verdict"`
	Confidence      string   `json:"confidence"`
	Summary         string   `json:"summary"`
	SystemicSignals []string `json:"systemic_signals,omitempty"`
	FlightSignals   []string `json:"flight_signals,omitempty"`
	MissingEvidence []string `json:"missing_evidence,omitempty"`
	NextActions     []string `json:"next_actions,omitempty"`
}

type assessReport struct {
	GeneratedAt   string                     `json:"generated_at"`
	Query         assessQuery                `json:"query"`
	Decision      assessDecision             `json:"decision"`
	Evidence      assessEvidence             `json:"evidence"`
	DelayedFlight *assessedFlight            `json:"delayed_flight,omitempty"`
	Alternatives  []assessedFlight           `json:"alternatives,omitempty"`
	Prices        *assessPrices              `json:"prices,omitempty"`
	Sources       []assessSource             `json:"sources"`
	Raw           map[string]json.RawMessage `json:"raw,omitempty"`
}

type assessEvidence struct {
	Origin      assessAirportCondition `json:"origin"`
	Destination assessAirportCondition `json:"destination"`
	NASStatus   nasStatus              `json:"nas_status"`
}

type assessAirportCondition struct {
	Airport          string                  `json:"airport"`
	Role             string                  `json:"role"`
	AirportDelays    airportDelaySummary     `json:"airport_delays"`
	Weather          weatherSummary          `json:"weather"`
	DisruptionCounts disruptionCountsSummary `json:"disruption_counts"`
	NASEvents        []nasEvent              `json:"nas_events,omitempty"`
}

type airportDelaySummary struct {
	Available       bool     `json:"available"`
	Active          bool     `json:"active"`
	Signals         []string `json:"signals,omitempty"`
	MaxDelayMinutes int      `json:"max_delay_minutes,omitempty"`
}

type weatherSummary struct {
	Available bool     `json:"available"`
	Severe    bool     `json:"severe"`
	Signals   []string `json:"signals,omitempty"`
}

type disruptionCountsSummary struct {
	Available     bool    `json:"available"`
	Cancellations int     `json:"cancellations,omitempty"`
	Delays        int     `json:"delays,omitempty"`
	Total         int     `json:"total,omitempty"`
	DelayRate     float64 `json:"delay_rate,omitempty"`
	Signal        string  `json:"signal,omitempty"`
}

type assessedFlight struct {
	Ident                 string            `json:"ident,omitempty"`
	FAFlightID            string            `json:"fa_flight_id,omitempty"`
	Operator              string            `json:"operator,omitempty"`
	Origin                string            `json:"origin,omitempty"`
	Destination           string            `json:"destination,omitempty"`
	ScheduledOut          string            `json:"scheduled_out,omitempty"`
	EstimatedOut          string            `json:"estimated_out,omitempty"`
	ActualOut             string            `json:"actual_out,omitempty"`
	ActualOff             string            `json:"actual_off,omitempty"`
	ScheduledIn           string            `json:"scheduled_in,omitempty"`
	EstimatedIn           string            `json:"estimated_in,omitempty"`
	ActualIn              string            `json:"actual_in,omitempty"`
	Status                string            `json:"status,omitempty"`
	AircraftType          string            `json:"aircraft_type,omitempty"`
	Registration          string            `json:"registration,omitempty"`
	GateOrigin            string            `json:"gate_origin,omitempty"`
	GateDestination       string            `json:"gate_destination,omitempty"`
	InboundFAFlightID     string            `json:"inbound_fa_flight_id,omitempty"`
	DepartureDelayMinutes int               `json:"departure_delay_minutes,omitempty"`
	ArrivalDelayMinutes   int               `json:"arrival_delay_minutes,omitempty"`
	Readiness             string            `json:"readiness"`
	Risk                  string            `json:"risk"`
	Reasons               []string          `json:"reasons,omitempty"`
	Inbound               *assessedInbound  `json:"inbound,omitempty"`
	SortTime              string            `json:"-"`
	Source                map[string]string `json:"source,omitempty"`
}

type assessedInbound struct {
	Ident                 string   `json:"ident,omitempty"`
	FAFlightID            string   `json:"fa_flight_id,omitempty"`
	Origin                string   `json:"origin,omitempty"`
	Destination           string   `json:"destination,omitempty"`
	EstimatedIn           string   `json:"estimated_in,omitempty"`
	ActualIn              string   `json:"actual_in,omitempty"`
	Status                string   `json:"status,omitempty"`
	DepartureDelayMinutes int      `json:"departure_delay_minutes,omitempty"`
	ArrivalDelayMinutes   int      `json:"arrival_delay_minutes,omitempty"`
	Risk                  string   `json:"risk,omitempty"`
	Reasons               []string `json:"reasons,omitempty"`
}

type assessPrices struct {
	Source  string               `json:"source"`
	Query   gflights.SearchQuery `json:"query"`
	Count   int                  `json:"count"`
	Options []assessPriceOption  `json:"options,omitempty"`
	Error   string               `json:"error,omitempty"`
	Skipped bool                 `json:"skipped,omitempty"`
}

type assessPriceOption struct {
	Price           float64 `json:"price"`
	Currency        string  `json:"currency,omitempty"`
	Airline         string  `json:"airline,omitempty"`
	FlightNumber    string  `json:"flight_number,omitempty"`
	DepartTime      string  `json:"depart_time,omitempty"`
	DurationMinutes int     `json:"duration_minutes,omitempty"`
	Stops           int     `json:"stops"`
}

type assessSource struct {
	Name   string            `json:"name"`
	Path   string            `json:"path,omitempty"`
	Params map[string]string `json:"params,omitempty"`
	Status string            `json:"status"`
	Error  string            `json:"error,omitempty"`
}

type nasStatus struct {
	Source    string     `json:"source"`
	UpdatedAt string     `json:"updated_at,omitempty"`
	Airports  []string   `json:"airports,omitempty"`
	Events    []nasEvent `json:"events,omitempty"`
	Error     string     `json:"error,omitempty"`
	Skipped   bool       `json:"skipped,omitempty"`
}

type nasEvent struct {
	Airport  string `json:"airport"`
	Category string `json:"category"`
	Type     string `json:"type,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Average  string `json:"average,omitempty"`
	Minimum  string `json:"minimum,omitempty"`
	Maximum  string `json:"maximum,omitempty"`
	Trend    string `json:"trend,omitempty"`
	Start    string `json:"start,omitempty"`
	End      string `json:"end,omitempty"`
}

func newAssessCmd(flags *rootFlags) *cobra.Command {
	opts := assessOptions{
		lookahead:         24 * time.Hour,
		maxAlternatives:   8,
		maxInboundLookups: 4,
		flightType:        "Airline",
		nasURL:            defaultNASStatusURL,
	}

	cmd := &cobra.Command{
		Use:         "assess",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Assess route delay cause and rank viable alternatives",
		Long: `assess joins AeroAPI airport conditions, disruption counts, route options,
flight status, inbound aircraft clues, FAA NAS status, and optional Google
Flights prices into one decision report. It is built for the passenger question:
"is this delay systemic, or should I switch flights/operators?"`,
		Example: `  flight-goat-pp-cli assess --origin SFO --destination DCA --delayed-flight UA123 --agent
  flight-goat-pp-cli assess --origin KSFO --destination KJFK --depart-after 18:00 --no-prices --agent`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.origin == "" || opts.destination == "" {
				return usageErr(fmt.Errorf("--origin and --destination are required"))
			}
			if opts.maxAlternatives < 1 {
				return usageErr(fmt.Errorf("--max-alternatives must be at least 1"))
			}
			if opts.maxInboundLookups < 0 {
				return usageErr(fmt.Errorf("--max-inbound-lookups must be 0 or greater"))
			}

			now := time.Now().UTC()
			date := strings.TrimSpace(opts.date)
			if date == "" {
				date = now.Format("2006-01-02")
			}
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return usageErr(fmt.Errorf("--date must be YYYY-MM-DD: %w", err))
			}
			departAfter, err := parseAssessDepartAfter(date, opts.departAfter, now)
			if err != nil {
				return usageErr(err)
			}
			end := departAfter.Add(opts.lookahead)

			query := buildAssessQuery(opts, date, departAfter)
			if flags.dryRun {
				printAssessDryRun(cmd.ErrOrStderr(), query, opts, departAfter, end)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			report := runAssess(ctx, c, flags, opts, query, departAfter, end)
			return emitJSONOrTable(cmd.OutOrStdout(), flags, report, func(w io.Writer) {
				printAssessTable(w, report)
			})
		},
	}

	cmd.Flags().StringVar(&opts.origin, "origin", "", "Origin airport, IATA or ICAO (for US 3-letter IATA, AeroAPI uses K + code)")
	cmd.Flags().StringVar(&opts.destination, "destination", "", "Destination airport, IATA or ICAO (for US 3-letter IATA, AeroAPI uses K + code)")
	cmd.Flags().StringVar(&opts.delayedFlight, "delayed-flight", "", "Known delayed flight ident or fa_flight_id to inspect")
	cmd.Flags().StringVar(&opts.date, "date", "", "Departure date for Google Flights price context (YYYY-MM-DD; default today UTC)")
	cmd.Flags().StringVar(&opts.departAfter, "depart-after", "", "Only consider alternatives after this time (RFC3339 or HH:MM UTC; default now)")
	cmd.Flags().DurationVar(&opts.lookahead, "lookahead", opts.lookahead, "Alternative search window after --depart-after")
	cmd.Flags().IntVar(&opts.maxAlternatives, "max-alternatives", opts.maxAlternatives, "Maximum route alternatives to rank")
	cmd.Flags().IntVar(&opts.maxInboundLookups, "max-inbound-lookups", opts.maxInboundLookups, "Maximum alternatives whose inbound aircraft status should be fetched")
	cmd.Flags().StringVar(&opts.flightType, "flight-type", opts.flightType, "AeroAPI route type filter, usually Airline or General_Aviation")
	cmd.Flags().StringVar(&opts.connection, "connection", "", "AeroAPI route connection filter: nonstop or onestop")
	cmd.Flags().BoolVar(&opts.noNAS, "no-nas", false, "Skip FAA NAS Status lookup")
	cmd.Flags().StringVar(&opts.nasURL, "nas-url", opts.nasURL, "FAA NAS Status XML endpoint")
	cmd.Flags().BoolVar(&opts.noPrices, "no-prices", false, "Skip Google Flights price context")
	cmd.Flags().StringVar(&opts.currency, "currency", "", "Currency for Google Flights price context (ISO 4217, default USD)")
	cmd.Flags().BoolVar(&opts.includeRaw, "include-raw", false, "Include raw upstream AeroAPI JSON payloads in the report")
	return cmd
}

func buildAssessQuery(opts assessOptions, date string, departAfter time.Time) assessQuery {
	return assessQuery{
		Origin:            normalizeAirportCodes(opts.origin),
		Destination:       normalizeAirportCodes(opts.destination),
		DepartureDate:     date,
		DepartAfter:       departAfter.UTC().Format(time.RFC3339),
		Lookahead:         opts.lookahead.String(),
		DelayedFlight:     upperCode(opts.delayedFlight),
		MaxAlternatives:   opts.maxAlternatives,
		MaxInboundLookups: opts.maxInboundLookups,
		FlightType:        opts.flightType,
		Connection:        opts.connection,
		IncludesNAS:       !opts.noNAS,
		IncludesPrices:    !opts.noPrices,
	}
}

func runAssess(ctx context.Context, c *client.Client, flags *rootFlags, opts assessOptions, query assessQuery, departAfter, end time.Time) *assessReport {
	report := &assessReport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Query:       query,
		Sources:     []assessSource{},
	}
	if opts.includeRaw {
		report.Raw = map[string]json.RawMessage{}
	}

	origin := newAssessAirportCondition("origin", query.Origin.ICAO)
	dest := newAssessAirportCondition("destination", query.Destination.ICAO)

	originDelayRaw, src := assessGetAero(c, "aeroapi.origin_delays", fmt.Sprintf("/airports/%s/delays", query.Origin.ICAO), nil)
	report.addSource(src)
	origin.AirportDelays = summarizeAirportDelays(originDelayRaw)
	report.addRaw("origin_delays", originDelayRaw)

	destDelayRaw, src := assessGetAero(c, "aeroapi.destination_delays", fmt.Sprintf("/airports/%s/delays", query.Destination.ICAO), nil)
	report.addSource(src)
	dest.AirportDelays = summarizeAirportDelays(destDelayRaw)
	report.addRaw("destination_delays", destDelayRaw)

	originWeatherRaw, src := assessGetAero(c, "aeroapi.origin_weather", fmt.Sprintf("/airports/%s/weather/observations", query.Origin.ICAO), map[string]string{"max_pages": "1"})
	report.addSource(src)
	origin.Weather = summarizeWeather(originWeatherRaw)
	report.addRaw("origin_weather", originWeatherRaw)

	destWeatherRaw, src := assessGetAero(c, "aeroapi.destination_weather", fmt.Sprintf("/airports/%s/weather/observations", query.Destination.ICAO), map[string]string{"max_pages": "1"})
	report.addSource(src)
	dest.Weather = summarizeWeather(destWeatherRaw)
	report.addRaw("destination_weather", destWeatherRaw)

	originDisruptionRaw, src := assessGetAero(c, "aeroapi.origin_disruptions", fmt.Sprintf("/disruption_counts/origin/%s", query.Origin.ICAO), map[string]string{"time_period": "today"})
	report.addSource(src)
	origin.DisruptionCounts = summarizeDisruptions(originDisruptionRaw)
	report.addRaw("origin_disruptions", originDisruptionRaw)

	destDisruptionRaw, src := assessGetAero(c, "aeroapi.destination_disruptions", fmt.Sprintf("/disruption_counts/destination/%s", query.Destination.ICAO), map[string]string{"time_period": "today"})
	report.addSource(src)
	dest.DisruptionCounts = summarizeDisruptions(destDisruptionRaw)
	report.addRaw("destination_disruptions", destDisruptionRaw)

	if opts.noNAS {
		report.Evidence.NASStatus = nasStatus{Source: "faa-nas-status", Skipped: true}
		report.addSource(assessSource{Name: "faa.nas_status", Status: "skipped"})
	} else {
		nas, src := fetchNASStatus(ctx, opts.nasURL, query.Origin, query.Destination, flags.timeout)
		report.Evidence.NASStatus = nas
		report.addSource(src)
		origin.NASEvents = nasEventsForAirport(nas.Events, query.Origin)
		dest.NASEvents = nasEventsForAirport(nas.Events, query.Destination)
	}

	report.Evidence.Origin = origin
	report.Evidence.Destination = dest

	systemicNow := hasSystemicSignals(origin, dest)
	if query.DelayedFlight != "" {
		delayed, sources := assessFetchDelayedFlightWithInbound(c, query.DelayedFlight, query, departAfter)
		for _, source := range sources {
			report.addSource(source)
		}
		if delayed != nil {
			report.DelayedFlight = delayed
		}
	}

	alternatives, sources, raw := fetchAssessAlternatives(c, opts, query, departAfter, end, systemicNow)
	for _, source := range sources {
		report.addSource(source)
	}
	report.addRaw("route_alternatives", raw)
	report.Alternatives = alternatives

	if opts.noPrices {
		report.Prices = &assessPrices{Source: "google-flights-native", Skipped: true}
		report.addSource(assessSource{Name: "google_flights.prices", Status: "skipped"})
	} else {
		prices, source := fetchAssessPrices(ctx, opts, query)
		report.Prices = prices
		report.addSource(source)
	}

	report.Decision = buildAssessDecision(report)
	return report
}

func newAssessAirportCondition(role, airport string) assessAirportCondition {
	return assessAirportCondition{
		Airport: airport,
		Role:    role,
	}
}

func (r *assessReport) addSource(source assessSource) {
	r.Sources = append(r.Sources, source)
}

func (r *assessReport) addRaw(name string, raw json.RawMessage) {
	if r.Raw == nil || len(raw) == 0 {
		return
	}
	r.Raw[name] = raw
}

func printAssessDryRun(w io.Writer, query assessQuery, opts assessOptions, departAfter, end time.Time) {
	fmt.Fprintf(w, "GET /airports/%s/delays\n", query.Origin.ICAO)
	fmt.Fprintf(w, "GET /airports/%s/delays\n", query.Destination.ICAO)
	fmt.Fprintf(w, "GET /airports/%s/weather/observations?max_pages=1\n", query.Origin.ICAO)
	fmt.Fprintf(w, "GET /airports/%s/weather/observations?max_pages=1\n", query.Destination.ICAO)
	fmt.Fprintf(w, "GET /disruption_counts/origin/%s?time_period=today\n", query.Origin.ICAO)
	fmt.Fprintf(w, "GET /disruption_counts/destination/%s?time_period=today\n", query.Destination.ICAO)
	if query.DelayedFlight != "" {
		fmt.Fprintf(w, "GET /flights/%s?start=%s&end=%s&max_pages=2\n",
			url.PathEscape(query.DelayedFlight),
			departAfter.Add(-12*time.Hour).UTC().Format(time.RFC3339),
			departAfter.Add(36*time.Hour).UTC().Format(time.RFC3339))
	}
	fmt.Fprintf(w, "GET /airports/%s/flights/to/%s?start=%s&end=%s&max_pages=1",
		query.Origin.ICAO, query.Destination.ICAO, departAfter.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339))
	if opts.flightType != "" {
		fmt.Fprintf(w, "&type=%s", opts.flightType)
	}
	if opts.connection != "" {
		fmt.Fprintf(w, "&connection=%s", opts.connection)
	}
	fmt.Fprintln(w)
	if !opts.noNAS {
		fmt.Fprintf(w, "GET %s\n", opts.nasURL)
	}
	if !opts.noPrices {
		fmt.Fprintf(w, "gflights.Search(%s -> %s on %s", query.Origin.IATA, query.Destination.IATA, query.DepartureDate)
		if opts.currency != "" {
			fmt.Fprintf(w, " currency=%s", strings.ToUpper(strings.TrimSpace(opts.currency)))
		}
		fmt.Fprintln(w, ")")
	}
	fmt.Fprintln(w, "\n(dry run - no requests sent; would classify systemic vs carrier/aircraft-specific delay and rank alternatives)")
}

func printAssessTable(w io.Writer, report *assessReport) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "VERDICT\tCONFIDENCE\tSUMMARY")
	fmt.Fprintf(tw, "%s\t%s\t%s\n\n", report.Decision.Verdict, report.Decision.Confidence, report.Decision.Summary)
	if len(report.Decision.SystemicSignals) > 0 {
		fmt.Fprintln(tw, "SYSTEMIC SIGNALS")
		for _, signal := range report.Decision.SystemicSignals {
			fmt.Fprintf(tw, "- %s\n", signal)
		}
		fmt.Fprintln(tw)
	}
	if report.DelayedFlight != nil {
		fmt.Fprintln(tw, "DELAYED FLIGHT\tRISK\tREADINESS\tDELAY_MIN\tREASONS")
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\n\n",
			report.DelayedFlight.Ident,
			report.DelayedFlight.Risk,
			report.DelayedFlight.Readiness,
			report.DelayedFlight.DepartureDelayMinutes,
			strings.Join(report.DelayedFlight.Reasons, "; "))
	}
	if len(report.Alternatives) > 0 {
		fmt.Fprintln(tw, "ALTERNATIVE\tOPERATOR\tOUT\tRISK\tREADINESS\tDELAY_MIN\tREASONS")
		for _, alt := range report.Alternatives {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
				alt.Ident,
				alt.Operator,
				firstNonEmpty(alt.EstimatedOut, alt.ScheduledOut),
				alt.Risk,
				alt.Readiness,
				alt.DepartureDelayMinutes,
				strings.Join(alt.Reasons, "; "))
		}
	}
	tw.Flush()
}

func assessGetAero(c *client.Client, name, path string, params map[string]string) (json.RawMessage, assessSource) {
	raw, err := c.Get(path, params)
	source := assessSource{Name: name, Path: path, Params: cleanAssessParams(params), Status: "ok"}
	if err != nil {
		source.Status = "error"
		source.Error = err.Error()
		return nil, source
	}
	return raw, source
}

func cleanAssessParams(params map[string]string) map[string]string {
	if len(params) == 0 {
		return nil
	}
	clean := map[string]string{}
	for key, value := range params {
		if value != "" {
			clean[key] = value
		}
	}
	if len(clean) == 0 {
		return nil
	}
	return clean
}

func fetchAssessAlternatives(c *client.Client, opts assessOptions, query assessQuery, departAfter, end time.Time, systemicNow bool) ([]assessedFlight, []assessSource, json.RawMessage) {
	params := map[string]string{
		"start":     departAfter.UTC().Format(time.RFC3339),
		"end":       end.UTC().Format(time.RFC3339),
		"max_pages": "1",
		"type":      opts.flightType,
	}
	if opts.connection != "" {
		params["connection"] = opts.connection
	}
	path := fmt.Sprintf("/airports/%s/flights/to/%s", query.Origin.ICAO, query.Destination.ICAO)
	raw, source := assessGetAero(c, "aeroapi.route_alternatives", path, params)
	sources := []assessSource{source}
	if source.Status != "ok" {
		return nil, sources, raw
	}

	var page scheduledDeparturesPage
	if err := json.Unmarshal(raw, &page); err != nil {
		sources[0].Status = "error"
		sources[0].Error = "parse route alternatives: " + err.Error()
		return nil, sources, raw
	}

	items := filterAssessDepartures(page.items(), departAfter, opts.maxAlternatives*3)
	if len(items) == 0 {
		sources[0].Status = "empty"
		return nil, sources, raw
	}
	alternatives := make([]assessedFlight, 0, minInt(len(items), opts.maxAlternatives))
	inboundLookups := 0
	for _, item := range items {
		var inbound *assessedInbound
		if item.InboundFAID != "" && inboundLookups < opts.maxInboundLookups {
			inboundFlight, inboundSources := assessFetchFlightWithInbound(c, "alternative_inbound", item.InboundFAID, false)
			sources = append(sources, inboundSources...)
			inboundLookups++
			if inboundFlight != nil {
				inbound = inboundFromAssessedFlight(*inboundFlight)
			}
		}
		alternatives = append(alternatives, assessScheduledFlight(item, "alternative", inbound, systemicNow))
		if len(alternatives) >= opts.maxAlternatives {
			break
		}
	}

	sort.SliceStable(alternatives, func(i, j int) bool {
		ri := riskRank(alternatives[i].Risk)
		rj := riskRank(alternatives[j].Risk)
		if ri != rj {
			return ri < rj
		}
		if alternatives[i].DepartureDelayMinutes != alternatives[j].DepartureDelayMinutes {
			return alternatives[i].DepartureDelayMinutes < alternatives[j].DepartureDelayMinutes
		}
		return alternatives[i].SortTime < alternatives[j].SortTime
	})
	return alternatives, sources, raw
}

func assessFetchFlightWithInbound(c *client.Client, sourcePrefix, ident string, followInbound bool) (*assessedFlight, []assessSource) {
	flight, source := fetchOneScheduledFlight(c, sourcePrefix, ident)
	sources := []assessSource{source}
	if flight == nil {
		return nil, sources
	}
	var inbound *assessedInbound
	if followInbound && flight.InboundFAID != "" {
		inboundFlight, inboundSource := fetchOneScheduledFlight(c, sourcePrefix+".inbound", flight.InboundFAID)
		sources = append(sources, inboundSource)
		if inboundFlight != nil {
			inbound = inboundFromDeparture(*inboundFlight)
		}
	}
	assessed := assessScheduledFlight(*flight, sourcePrefix, inbound, false)
	return &assessed, sources
}

func assessFetchDelayedFlightWithInbound(c *client.Client, ident string, query assessQuery, departAfter time.Time) (*assessedFlight, []assessSource) {
	params := map[string]string{"max_pages": "2"}
	if !looksLikeFAFlightID(ident) {
		params["start"] = departAfter.Add(-12 * time.Hour).UTC().Format(time.RFC3339)
		params["end"] = departAfter.Add(36 * time.Hour).UTC().Format(time.RFC3339)
	}
	path := "/flights/" + url.PathEscape(ident)
	raw, source := assessGetAero(c, "aeroapi.delayed_flight", path, params)
	sources := []assessSource{source}
	if source.Status != "ok" {
		return nil, sources
	}

	var page scheduledDeparturesPage
	if err := json.Unmarshal(raw, &page); err != nil {
		source.Status = "error"
		source.Error = "parse delayed flight: " + err.Error()
		sources[0] = source
		return nil, sources
	}
	flight := selectAssessFlightCandidate(page.items(), query, departAfter)
	if flight == nil {
		source.Status = "empty"
		sources[0] = source
		return nil, sources
	}

	var inbound *assessedInbound
	if flight.InboundFAID != "" {
		inboundFlight, inboundSource := fetchOneScheduledFlight(c, "delayed_flight.inbound", flight.InboundFAID)
		sources = append(sources, inboundSource)
		if inboundFlight != nil {
			inbound = inboundFromDeparture(*inboundFlight)
		}
	}
	assessed := assessScheduledFlight(*flight, "delayed_flight", inbound, false)
	return &assessed, sources
}

func fetchOneScheduledFlight(c *client.Client, sourcePrefix, ident string) (*scheduledDeparture, assessSource) {
	path := "/flights/" + url.PathEscape(ident)
	raw, source := assessGetAero(c, "aeroapi."+sourcePrefix, path, map[string]string{"max_pages": "1"})
	if source.Status != "ok" {
		return nil, source
	}
	var page scheduledDeparturesPage
	if err := json.Unmarshal(raw, &page); err != nil {
		source.Status = "error"
		source.Error = "parse flight: " + err.Error()
		return nil, source
	}
	items := page.items()
	if len(items) == 0 {
		source.Status = "empty"
		return nil, source
	}
	return &items[0], source
}

func selectAssessFlightCandidate(items []scheduledDeparture, query assessQuery, target time.Time) *scheduledDeparture {
	if len(items) == 0 {
		return nil
	}
	type scored struct {
		item  scheduledDeparture
		score int
		delta time.Duration
	}
	candidates := make([]scored, 0, len(items))
	for _, item := range items {
		score := 0
		if airportRefMatches(item.Origin, query.Origin) {
			score += 4
		}
		if airportRefMatches(item.Destination, query.Destination) {
			score += 4
		}
		delta := 365 * 24 * time.Hour
		if t, ok := assessDepartureTime(item); ok {
			if t.After(target.Add(-12*time.Hour)) && t.Before(target.Add(36*time.Hour)) {
				score += 2
			}
			if t.After(target) {
				delta = t.Sub(target)
			} else {
				delta = target.Sub(t)
			}
		}
		candidates = append(candidates, scored{item: item, score: score, delta: delta})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].delta < candidates[j].delta
	})
	return &candidates[0].item
}

func airportRefMatches(ref airportRef, codes airportCodes) bool {
	wanted := map[string]bool{}
	for _, code := range []string{codes.Input, codes.IATA, codes.ICAO, stripUSICAOPrefix(codes.ICAO)} {
		if code = upperCode(code); code != "" {
			wanted[code] = true
		}
	}
	for _, code := range []string{ref.Code, ref.CodeIATA, ref.CodeICAO, stripUSICAOPrefix(ref.CodeICAO)} {
		if wanted[upperCode(code)] {
			return true
		}
	}
	return false
}

func looksLikeFAFlightID(ident string) bool {
	return strings.Count(ident, "-") >= 2
}

func fetchAssessPrices(ctx context.Context, opts assessOptions, query assessQuery) (*assessPrices, assessSource) {
	source := assessSource{Name: "google_flights.prices", Path: "gflights.Search", Status: "ok"}
	result, err := gflights.Search(ctx, gflights.SearchOptions{
		Origin:        query.Origin.IATA,
		Destination:   query.Destination.IATA,
		DepartureDate: query.DepartureDate,
		Currency:      opts.currency,
	})
	if err != nil {
		source.Status = "error"
		source.Error = err.Error()
		return &assessPrices{Source: "google-flights-native", Error: err.Error()}, source
	}
	prices := &assessPrices{
		Source: result.Source,
		Query:  result.Query,
		Count:  result.Count,
	}
	limit := minInt(5, len(result.Flights))
	for i := 0; i < limit; i++ {
		flight := result.Flights[i]
		option := assessPriceOption{
			Price:           flight.Price,
			Currency:        flight.Currency,
			DurationMinutes: flight.DurationMinutes,
			Stops:           flight.Stops,
		}
		if len(flight.Legs) > 0 {
			option.Airline = flight.Legs[0].Airline.Code
			option.FlightNumber = flight.Legs[0].FlightNumber
			option.DepartTime = flight.Legs[0].DepartureTime
		}
		prices.Options = append(prices.Options, option)
	}
	return prices, source
}

func filterAssessDepartures(items []scheduledDeparture, departAfter time.Time, max int) []scheduledDeparture {
	seen := map[string]bool{}
	filtered := make([]scheduledDeparture, 0, minInt(len(items), max))
	for _, item := range items {
		key := firstNonEmpty(item.FAFlightID, item.Ident+"|"+item.ScheduledOut+"|"+item.EstimatedOut)
		if key != "" && seen[key] {
			continue
		}
		seen[key] = true
		if t, ok := assessDepartureTime(item); ok && t.Before(departAfter.Add(-15*time.Minute)) {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		ti, iOK := assessDepartureTime(filtered[i])
		tj, jOK := assessDepartureTime(filtered[j])
		switch {
		case iOK && jOK:
			return ti.Before(tj)
		case iOK:
			return true
		case jOK:
			return false
		default:
			return filtered[i].Ident < filtered[j].Ident
		}
	})
	if len(filtered) > max {
		filtered = filtered[:max]
	}
	return filtered
}

func assessScheduledFlight(item scheduledDeparture, role string, inbound *assessedInbound, systemicNow bool) assessedFlight {
	delayMin := secondsToMinutes(item.DepartureDelay)
	arrivalDelayMin := secondsToMinutes(item.ArrivalDelay)
	readiness := assessReadiness(item, inbound)
	risk, reasons := assessFlightRisk(item, inbound, systemicNow)
	sortTime := firstNonEmpty(item.EstimatedOut, item.ScheduledOut, item.ScheduledOff)
	return assessedFlight{
		Ident:                 firstNonEmpty(item.Ident, item.IdentICAO),
		FAFlightID:            item.FAFlightID,
		Operator:              firstNonEmpty(item.Operator, item.OperatorIATA, item.OperatorICAO),
		Origin:                item.Origin.Best(),
		Destination:           item.Destination.Best(),
		ScheduledOut:          item.ScheduledOut,
		EstimatedOut:          item.EstimatedOut,
		ActualOut:             item.ActualOut,
		ActualOff:             item.ActualOff,
		ScheduledIn:           item.ScheduledIn,
		EstimatedIn:           item.EstimatedIn,
		ActualIn:              item.ActualIn,
		Status:                item.Status,
		AircraftType:          item.AircraftType,
		Registration:          item.Registration,
		GateOrigin:            item.GateOrigin,
		GateDestination:       item.GateDestination,
		InboundFAFlightID:     item.InboundFAID,
		DepartureDelayMinutes: delayMin,
		ArrivalDelayMinutes:   arrivalDelayMin,
		Readiness:             readiness,
		Risk:                  risk,
		Reasons:               reasons,
		Inbound:               inbound,
		SortTime:              sortTime,
		Source:                map[string]string{"role": role},
	}
}

func assessReadiness(item scheduledDeparture, inbound *assessedInbound) string {
	status := strings.ToLower(item.Status)
	switch {
	case item.Cancelled || strings.Contains(status, "cancel"):
		return "cancelled"
	case item.ActualOff != "":
		return "airborne"
	case item.ActualOut != "":
		return "left_gate"
	case item.Registration != "" && item.InboundFAID == "":
		return "aircraft_assigned_at_origin"
	case item.InboundFAID != "" && inbound != nil && inbound.ActualIn != "":
		return "inbound_arrived_at_origin"
	case item.InboundFAID != "":
		return "inbound_aircraft"
	case item.GateOrigin != "":
		return "gate_assigned_no_aircraft_seen"
	default:
		return "scheduled_no_aircraft_seen"
	}
}

func assessFlightRisk(item scheduledDeparture, inbound *assessedInbound, systemicNow bool) (string, []string) {
	risk := "low"
	reasons := []string{}
	status := strings.ToLower(item.Status)
	delayMin := secondsToMinutes(item.DepartureDelay)

	if item.Cancelled || strings.Contains(status, "cancel") {
		return "high", []string{"flight is cancelled"}
	}
	if delayMin >= 60 {
		risk = "high"
		reasons = append(reasons, fmt.Sprintf("departure delay %d min", delayMin))
	} else if delayMin >= 15 {
		risk = maxRisk(risk, "medium")
		reasons = append(reasons, fmt.Sprintf("departure delay %d min", delayMin))
	}
	if item.InboundFAID != "" {
		risk = maxRisk(risk, "medium")
		reasons = append(reasons, "uses inbound aircraft")
	}
	if inbound != nil {
		for _, reason := range inbound.Reasons {
			reasons = append(reasons, "inbound: "+reason)
		}
		risk = maxRisk(risk, inbound.Risk)
	}
	if systemicNow {
		risk = maxRisk(risk, "medium")
		reasons = append(reasons, "airport/system signal affects route")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "no active delay or inbound risk seen")
	}
	return risk, uniqueStrings(reasons)
}

func inboundFromDeparture(item scheduledDeparture) *assessedInbound {
	flight := assessScheduledFlight(item, "inbound", nil, false)
	return inboundFromAssessedFlight(flight)
}

func inboundFromAssessedFlight(flight assessedFlight) *assessedInbound {
	return &assessedInbound{
		Ident:                 flight.Ident,
		FAFlightID:            flight.FAFlightID,
		Origin:                flight.Origin,
		Destination:           flight.Destination,
		EstimatedIn:           flight.EstimatedIn,
		ActualIn:              flight.ActualIn,
		Status:                flight.Status,
		DepartureDelayMinutes: flight.DepartureDelayMinutes,
		ArrivalDelayMinutes:   flight.ArrivalDelayMinutes,
		Risk:                  flight.Risk,
		Reasons:               flight.Reasons,
	}
}

func buildAssessDecision(report *assessReport) assessDecision {
	systemicSignals := collectSystemicSignals(report.Evidence.Origin, report.Evidence.Destination)
	flightSignals := collectFlightSignals(report.DelayedFlight)
	missing := collectMissingEvidence(report)

	verdict := "no_systemic_delay_seen"
	confidence := "medium"
	summary := "No airport-wide or NAS delay signal is visible in the available sources."
	switch {
	case len(systemicSignals) >= 2 && len(flightSignals) > 0:
		verdict = "mixed_systemic_and_flight_specific"
		confidence = "high"
		summary = "Airport or NAS conditions are active, and the delayed flight also has aircraft/operator-specific risk."
	case len(systemicSignals) >= 2:
		verdict = "systemic_delay"
		confidence = "high"
		summary = "Multiple independent airport-wide signals point to a systemic delay environment."
	case len(systemicSignals) == 1 && len(flightSignals) > 0:
		verdict = "mixed_limited_evidence"
		confidence = "medium"
		summary = "One airport-wide signal is present, but the delayed flight also has specific risk."
	case len(systemicSignals) == 1:
		verdict = "possible_systemic_delay"
		confidence = "medium"
		summary = "One airport-wide signal is present; verify before assuming every alternative is exposed."
	case len(flightSignals) > 0:
		verdict = "carrier_or_aircraft_specific"
		confidence = "medium"
		summary = "Available airport-wide sources are not showing a systemic delay, while the delayed flight has specific risk."
	}
	if len(missing) >= 4 && len(systemicSignals) == 0 && len(flightSignals) == 0 {
		verdict = "insufficient_data"
		confidence = "low"
		summary = "Too many upstream sources failed or returned no evidence to classify the delay."
	} else if len(missing) >= 3 && confidence == "high" {
		confidence = "medium"
	} else if len(missing) >= 3 && confidence == "medium" {
		confidence = "low"
	}

	return assessDecision{
		Verdict:         verdict,
		Confidence:      confidence,
		Summary:         summary,
		SystemicSignals: systemicSignals,
		FlightSignals:   flightSignals,
		MissingEvidence: missing,
		NextActions:     assessNextActions(verdict, report),
	}
}

func collectSystemicSignals(origin, dest assessAirportCondition) []string {
	var signals []string
	for _, condition := range []assessAirportCondition{origin, dest} {
		prefix := condition.Role + " " + condition.Airport
		if condition.AirportDelays.Active {
			signal := prefix + " AeroAPI airport delay advisory"
			if len(condition.AirportDelays.Signals) > 0 {
				signal += ": " + strings.Join(condition.AirportDelays.Signals, "; ")
			}
			signals = append(signals, signal)
		}
		if condition.DisruptionCounts.Signal != "" {
			signals = append(signals, prefix+" disruption counts: "+condition.DisruptionCounts.Signal)
		}
		if condition.Weather.Severe {
			signals = append(signals, prefix+" weather: "+strings.Join(condition.Weather.Signals, "; "))
		}
		for _, event := range condition.NASEvents {
			signals = append(signals, fmt.Sprintf("%s FAA NAS %s: %s", prefix, event.Category, firstNonEmpty(event.Reason, event.Type, event.Average)))
		}
	}
	return uniqueStrings(signals)
}

func collectFlightSignals(flight *assessedFlight) []string {
	if flight == nil {
		return nil
	}
	var signals []string
	if flight.Risk == "high" || flight.Risk == "medium" {
		signals = append(signals, fmt.Sprintf("%s risk: %s", flight.Risk, strings.Join(flight.Reasons, "; ")))
	}
	if flight.Inbound != nil && flight.Inbound.Risk != "" && flight.Inbound.Risk != "low" {
		signals = append(signals, fmt.Sprintf("inbound %s risk: %s", flight.Inbound.Risk, strings.Join(flight.Inbound.Reasons, "; ")))
	}
	return uniqueStrings(signals)
}

func collectMissingEvidence(report *assessReport) []string {
	var missing []string
	sourceMissing := map[string]bool{}
	for _, source := range report.Sources {
		if source.Status == "error" {
			missing = append(missing, source.Name+": "+source.Error)
			sourceMissing[source.Name] = true
		}
		if source.Status == "empty" {
			missing = append(missing, source.Name+": no result")
			sourceMissing[source.Name] = true
		}
	}
	if !report.Evidence.Origin.AirportDelays.Available && !sourceMissing["aeroapi.origin_delays"] {
		missing = append(missing, "origin airport delay advisory unavailable or empty")
	}
	if !report.Evidence.Destination.AirportDelays.Available && !sourceMissing["aeroapi.destination_delays"] {
		missing = append(missing, "destination airport delay advisory unavailable or empty")
	}
	if !report.Evidence.Origin.DisruptionCounts.Available && !sourceMissing["aeroapi.origin_disruptions"] {
		missing = append(missing, "origin disruption counts unavailable or empty")
	}
	if !report.Evidence.Destination.DisruptionCounts.Available && !sourceMissing["aeroapi.destination_disruptions"] {
		missing = append(missing, "destination disruption counts unavailable or empty")
	}
	return uniqueStrings(missing)
}

func assessNextActions(verdict string, report *assessReport) []string {
	switch verdict {
	case "systemic_delay", "possible_systemic_delay":
		return []string{
			"Treat same-airport alternatives as exposed until NAS/AeroAPI signals clear.",
			"Prefer alternatives with assigned aircraft, low current delay, and no risky inbound dependency.",
		}
	case "mixed_systemic_and_flight_specific", "mixed_limited_evidence":
		return []string{
			"Do not read a single replacement flight as safe just because it is on another operator.",
			"Compare alternatives by aircraft readiness and departure delay, then price.",
		}
	case "carrier_or_aircraft_specific":
		return []string{
			"Prioritize same-route alternatives on other operators with no inbound delay signal.",
			"Use the delayed flight's inbound status as the main go/no-go evidence.",
		}
	default:
		if len(report.Alternatives) > 0 {
			return []string{"Use the ranked alternatives list; the lowest-risk options have the best operational evidence."}
		}
		return []string{"Re-run with --include-raw or inspect the listed failed sources before making the decision."}
	}
}

func hasSystemicSignals(origin, dest assessAirportCondition) bool {
	return len(collectSystemicSignals(origin, dest)) > 0
}

func summarizeAirportDelays(raw json.RawMessage) airportDelaySummary {
	summary := airportDelaySummary{}
	if !jsonHasMeaningfulData(raw) {
		return summary
	}
	summary.Available = true
	summary.Active = true
	var decoded any
	if json.Unmarshal(raw, &decoded) == nil {
		summary.Signals = uniqueStrings(collectStringsByKeys(decoded, 6, "reason", "type", "trend", "color", "message", "name"))
		if delay, ok := findNumberByKeys(decoded, "delay_secs", "delay_seconds", "delay"); ok {
			summary.MaxDelayMinutes = secondsToMinutes(int(delay))
		}
	}
	if len(summary.Signals) == 0 {
		summary.Signals = []string{"active delay payload returned"}
	}
	return summary
}

func summarizeWeather(raw json.RawMessage) weatherSummary {
	summary := weatherSummary{}
	if !jsonHasMeaningfulData(raw) {
		return summary
	}
	summary.Available = true
	upper := strings.ToUpper(string(raw))
	checks := map[string]string{
		" TS":        "thunderstorm marker",
		"THUNDER":    "thunderstorm marker",
		"LOW VIS":    "low visibility marker",
		"IFR":        "IFR marker",
		"LIFR":       "LIFR marker",
		"FREEZING":   "freezing precipitation marker",
		" SN":        "snow marker",
		"+RA":        "heavy rain marker",
		"WIND SHEAR": "wind shear marker",
		"WINDSHEAR":  "wind shear marker",
	}
	for needle, signal := range checks {
		if strings.Contains(upper, needle) {
			summary.Severe = true
			summary.Signals = append(summary.Signals, signal)
		}
	}
	if metarGustKTRE.MatchString(upper) {
		summary.Signals = append(summary.Signals, "gusty wind marker")
	}
	summary.Signals = uniqueStrings(summary.Signals)
	return summary
}

func summarizeDisruptions(raw json.RawMessage) disruptionCountsSummary {
	summary := disruptionCountsSummary{}
	if !jsonHasMeaningfulData(raw) {
		return summary
	}
	summary.Available = true
	var decoded any
	if json.Unmarshal(raw, &decoded) != nil {
		return summary
	}
	if v, ok := findNumberByKeys(decoded, "cancellations", "cancellation_count", "cancelled"); ok {
		summary.Cancellations = int(v)
	}
	if v, ok := findNumberByKeys(decoded, "delays", "delay_count", "delayed"); ok {
		summary.Delays = int(v)
	}
	if v, ok := findNumberByKeys(decoded, "total", "total_flights", "flights"); ok {
		summary.Total = int(v)
	}
	if summary.Total > 0 {
		summary.DelayRate = float64(summary.Delays) / float64(summary.Total)
	}
	switch {
	case summary.Cancellations > 0 && summary.Delays >= 25:
		summary.Signal = fmt.Sprintf("%d delays and %d cancellations", summary.Delays, summary.Cancellations)
	case summary.DelayRate >= 0.15 && summary.Delays >= 10:
		summary.Signal = fmt.Sprintf("%.1f%% delayed (%d of %d)", summary.DelayRate*100, summary.Delays, summary.Total)
	case summary.Delays >= 50:
		summary.Signal = fmt.Sprintf("%d delayed flights", summary.Delays)
	case summary.Cancellations > 0:
		summary.Signal = fmt.Sprintf("%d cancellations", summary.Cancellations)
	}
	return summary
}

func fetchNASStatus(ctx context.Context, endpoint string, origin, dest airportCodes, timeout time.Duration) (nasStatus, assessSource) {
	status := nasStatus{
		Source:   endpoint,
		Airports: uniqueStrings([]string{origin.IATA, origin.ICAO, dest.IATA, dest.ICAO}),
	}
	source := assessSource{Name: "faa.nas_status", Path: endpoint, Status: "ok"}
	if strings.TrimSpace(endpoint) == "" {
		status.Error = "NAS endpoint is empty"
		source.Status = "error"
		source.Error = status.Error
		return status, source
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		status.Error = err.Error()
		source.Status = "error"
		source.Error = err.Error()
		return status, source
	}
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		status.Error = err.Error()
		source.Status = "error"
		source.Error = err.Error()
		return status, source
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		status.Error = err.Error()
		source.Status = "error"
		source.Error = err.Error()
		return status, source
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		status.Error = fmt.Sprintf("HTTP %d from NAS status endpoint", resp.StatusCode)
		source.Status = "error"
		source.Error = status.Error
		return status, source
	}
	parsed, err := parseNASStatus(body, endpoint, origin, dest)
	if err != nil {
		status.Error = err.Error()
		source.Status = "error"
		source.Error = err.Error()
		return status, source
	}
	return parsed, source
}

type nasStatusXML struct {
	UpdateTime string            `xml:"Update_Time"`
	DelayTypes []nasDelayTypeXML `xml:"Delay_type"`
}

type nasDelayTypeXML struct {
	Name                      string                        `xml:"Name"`
	GroundDelayList           nasGroundDelayListXML         `xml:"Ground_Delay_List"`
	GroundStopList            nasGroundStopListXML          `xml:"Ground_Stop_List"`
	ArrivalDepartureDelayList nasArrivalDepartureDelayList  `xml:"Arrival_Departure_Delay_List"`
	AirportClosureList        nasAirportClosureListXML      `xml:"Airport_Closure_List"`
	GroundDelays              []nasGroundDelayXML           `xml:"Ground_Delay"`
	GroundStops               []nasGroundStopXML            `xml:"Ground_Stop"`
	ArrivalDepartureDelays    []nasArrivalDepartureDelayXML `xml:"Delay"`
	AirportClosures           []nasAirportClosureXML        `xml:"Airport_Closure"`
}

type nasGroundDelayListXML struct {
	Items []nasGroundDelayXML `xml:"Ground_Delay"`
}

type nasGroundStopListXML struct {
	Items []nasGroundStopXML `xml:"Ground_Stop"`
}

type nasArrivalDepartureDelayList struct {
	Items []nasArrivalDepartureDelayXML `xml:"Delay"`
}

type nasAirportClosureListXML struct {
	Items    []nasAirportClosureXML `xml:"Airport_Closure"`
	Closures []nasAirportClosureXML `xml:"Closure"`
}

type nasGroundDelayXML struct {
	Airport string `xml:"ARPT"`
	Reason  string `xml:"Reason"`
	Average string `xml:"Avg"`
	Maximum string `xml:"Max"`
}

type nasGroundStopXML struct {
	Airport string `xml:"ARPT"`
	Reason  string `xml:"Reason"`
	End     string `xml:"End_Time"`
}

type nasArrivalDepartureDelayXML struct {
	Airport           string                `xml:"ARPT"`
	Reason            string                `xml:"Reason"`
	ArrivalDepartures []nasArrivalDeparture `xml:"Arrival_Departure"`
}

type nasArrivalDeparture struct {
	Type    string `xml:"Type,attr"`
	Minimum string `xml:"Min"`
	Maximum string `xml:"Max"`
	Trend   string `xml:"Trend"`
}

type nasAirportClosureXML struct {
	Airport string `xml:"ARPT"`
	Reason  string `xml:"Reason"`
	Start   string `xml:"Start"`
	End     string `xml:"Reopen"`
}

func parseNASStatus(body []byte, source string, airports ...airportCodes) (nasStatus, error) {
	var parsed nasStatusXML
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return nasStatus{}, err
	}
	status := nasStatus{
		Source:    source,
		UpdatedAt: strings.TrimSpace(parsed.UpdateTime),
	}
	wanted := map[string]bool{}
	for _, airport := range airports {
		for _, code := range []string{airport.IATA, airport.ICAO, airport.Input, stripUSICAOPrefix(airport.ICAO)} {
			if code = upperCode(code); code != "" {
				wanted[code] = true
				status.Airports = append(status.Airports, code)
			}
		}
	}
	status.Airports = uniqueStrings(status.Airports)

	for _, delayType := range parsed.DelayTypes {
		category := strings.TrimSpace(delayType.Name)
		groundDelays := append(delayType.GroundDelayList.Items, delayType.GroundDelays...)
		for _, item := range groundDelays {
			if !wanted[upperCode(item.Airport)] {
				continue
			}
			status.Events = append(status.Events, nasEvent{
				Airport:  upperCode(item.Airport),
				Category: firstNonEmpty(category, "Ground Delay Program"),
				Reason:   strings.TrimSpace(item.Reason),
				Average:  strings.TrimSpace(item.Average),
				Maximum:  strings.TrimSpace(item.Maximum),
			})
		}

		groundStops := append(delayType.GroundStopList.Items, delayType.GroundStops...)
		for _, item := range groundStops {
			if !wanted[upperCode(item.Airport)] {
				continue
			}
			status.Events = append(status.Events, nasEvent{
				Airport:  upperCode(item.Airport),
				Category: firstNonEmpty(category, "Ground Stop"),
				Reason:   strings.TrimSpace(item.Reason),
				End:      strings.TrimSpace(item.End),
			})
		}

		arrivalDeparture := append(delayType.ArrivalDepartureDelayList.Items, delayType.ArrivalDepartureDelays...)
		for _, item := range arrivalDeparture {
			if !wanted[upperCode(item.Airport)] {
				continue
			}
			if len(item.ArrivalDepartures) == 0 {
				status.Events = append(status.Events, nasEvent{
					Airport:  upperCode(item.Airport),
					Category: firstNonEmpty(category, "Arrival/Departure Delay"),
					Reason:   strings.TrimSpace(item.Reason),
				})
				continue
			}
			for _, ad := range item.ArrivalDepartures {
				status.Events = append(status.Events, nasEvent{
					Airport:  upperCode(item.Airport),
					Category: firstNonEmpty(category, "Arrival/Departure Delay"),
					Type:     strings.TrimSpace(ad.Type),
					Reason:   strings.TrimSpace(item.Reason),
					Minimum:  strings.TrimSpace(ad.Minimum),
					Maximum:  strings.TrimSpace(ad.Maximum),
					Trend:    strings.TrimSpace(ad.Trend),
				})
			}
		}

		closures := append(delayType.AirportClosureList.Items, delayType.AirportClosureList.Closures...)
		closures = append(closures, delayType.AirportClosures...)
		for _, item := range closures {
			if !wanted[upperCode(item.Airport)] {
				continue
			}
			status.Events = append(status.Events, nasEvent{
				Airport:  upperCode(item.Airport),
				Category: firstNonEmpty(category, "Airport Closure"),
				Reason:   strings.TrimSpace(item.Reason),
				Start:    strings.TrimSpace(item.Start),
				End:      strings.TrimSpace(item.End),
			})
		}
	}
	return status, nil
}

func nasEventsForAirport(events []nasEvent, airport airportCodes) []nasEvent {
	wanted := map[string]bool{}
	for _, code := range []string{airport.IATA, airport.ICAO, stripUSICAOPrefix(airport.ICAO), airport.Input} {
		if code = upperCode(code); code != "" {
			wanted[code] = true
		}
	}
	var out []nasEvent
	for _, event := range events {
		if wanted[upperCode(event.Airport)] {
			out = append(out, event)
		}
	}
	return out
}

func normalizeAirportCodes(input string) airportCodes {
	code := upperCode(input)
	out := airportCodes{Input: code, IATA: code, ICAO: code}
	if len(code) == 4 && strings.HasPrefix(code, "K") {
		out.IATA = code[1:]
		return out
	}
	if len(code) == 3 {
		out.ICAO = "K" + code
		out.Note = "3-letter code treated as US IATA for AeroAPI by prefixing K"
		return out
	}
	return out
}

func stripUSICAOPrefix(code string) string {
	code = upperCode(code)
	if len(code) == 4 && strings.HasPrefix(code, "K") {
		return code[1:]
	}
	return code
}

func parseAssessDepartAfter(date, raw string, now time.Time) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return now.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}
	if clock, err := time.Parse("15:04", raw); err == nil {
		day, _ := time.Parse("2006-01-02", date)
		return time.Date(day.Year(), day.Month(), day.Day(), clock.Hour(), clock.Minute(), 0, 0, time.UTC), nil
	}
	return time.Time{}, fmt.Errorf("--depart-after must be RFC3339 or HH:MM UTC")
}

func assessDepartureTime(item scheduledDeparture) (time.Time, bool) {
	for _, value := range []string{item.EstimatedOut, item.ScheduledOut, item.ScheduledOff} {
		if value == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, value); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

func secondsToMinutes(seconds int) int {
	if seconds == 0 {
		return 0
	}
	return seconds / 60
}

func riskRank(risk string) int {
	switch risk {
	case "low":
		return 0
	case "medium":
		return 1
	case "high":
		return 2
	default:
		return 3
	}
}

func maxRisk(a, b string) string {
	if riskRank(b) > riskRank(a) {
		return b
	}
	return a
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func jsonHasMeaningfulData(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var decoded any
	if json.Unmarshal(raw, &decoded) != nil {
		return false
	}
	return anyHasMeaningfulData(decoded)
}

func anyHasMeaningfulData(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case map[string]any:
		for _, child := range v {
			if anyHasMeaningfulData(child) {
				return true
			}
		}
		return false
	case []any:
		for _, child := range v {
			if anyHasMeaningfulData(child) {
				return true
			}
		}
		return false
	case string:
		return strings.TrimSpace(v) != ""
	case float64:
		return v != 0
	case bool:
		return v
	default:
		return true
	}
}

func findNumberByKeys(value any, keys ...string) (float64, bool) {
	wanted := map[string]bool{}
	for _, key := range keys {
		wanted[strings.ToLower(key)] = true
	}
	return findNumberByKeySet(value, wanted)
}

func findNumberByKeySet(value any, wanted map[string]bool) (float64, bool) {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if wanted[strings.ToLower(key)] {
				if n, ok := numberFromAny(child); ok {
					return n, true
				}
			}
		}
		for _, child := range v {
			if n, ok := findNumberByKeySet(child, wanted); ok {
				return n, true
			}
		}
	case []any:
		for _, child := range v {
			if n, ok := findNumberByKeySet(child, wanted); ok {
				return n, true
			}
		}
	}
	return 0, false
}

func numberFromAny(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	default:
		return 0, false
	}
}

func collectStringsByKeys(value any, limit int, keys ...string) []string {
	wanted := map[string]bool{}
	for _, key := range keys {
		wanted[strings.ToLower(key)] = true
	}
	out := []string{}
	collectStringsByKeySet(value, wanted, limit, &out)
	return uniqueStrings(out)
}

func collectStringsByKeySet(value any, wanted map[string]bool, limit int, out *[]string) {
	if limit > 0 && len(*out) >= limit {
		return
	}
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if limit > 0 && len(*out) >= limit {
				return
			}
			if wanted[strings.ToLower(key)] {
				if s, ok := child.(string); ok && strings.TrimSpace(s) != "" {
					*out = append(*out, strings.TrimSpace(s))
					if limit > 0 && len(*out) >= limit {
						return
					}
				}
			}
		}
		for _, child := range v {
			if limit > 0 && len(*out) >= limit {
				return
			}
			collectStringsByKeySet(child, wanted, limit, out)
		}
	case []any:
		for _, child := range v {
			if limit > 0 && len(*out) >= limit {
				return
			}
			collectStringsByKeySet(child, wanted, limit, out)
		}
	}
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
