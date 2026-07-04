// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Primary flight-goat commands: Google Flights search, cheapest-dates, and
// Kayak-style nonstop explore. These are the headline features and do NOT
// require any API key. FlightAware commands live elsewhere and are optional.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/flight-goat/internal/gflights"
	"github.com/mvanhorn/printing-press-library/library/travel/flight-goat/internal/kayak"

	"github.com/spf13/cobra"
)

// registerPrimaryCommands wires the three free-tier commands: search, dates,
// and explore. These are added BEFORE the AeroAPI commands in root.go so they
// appear at the top of --help, matching the user's stated priority of Google
// Flights > Kayak > FlightAware.
func registerPrimaryCommands(rootCmd *cobra.Command, flags *rootFlags) {
	rootCmd.AddCommand(newGfFlightsCmd(flags))
	rootCmd.AddCommand(newGfDatesCmd(flags))
	rootCmd.AddCommand(newKayakExploreCmd(flags))
	rootCmd.AddCommand(newKayakLonghaulCmd(flags))
}

// ----- search: Google Flights one-shot search -----

func newGfFlightsCmd(flags *rootFlags) *cobra.Command {
	// PATCH(upstream cli-printing-press#804): expose currency only on Google
	// Flights-backed price commands, not as a misleading root flag.
	var returnDate, timeWindow, cabin, stops, sortBy, currencyCode string
	var airlines []string
	var passengers int
	var excludeBasic bool
	// PATCH(upstream cli-printing-press): new filters unlocked by the
	// native Google Flights backend (see internal/gflights/flights_native.go).
	var emissions string
	var checkedBags int
	var carryOn bool
	var layoverAirports []string
	var maxLayoverMinutes int
	var limitedResults bool
	// PATCH(library): multi-city — repeatable --segment "ORIG>DEST@YYYY-MM-DD"
	// triggers cross-provider multi-city search. See multicity.go +
	// internal/kayak/multicity.go.
	var segmentStrs []string
	var provider string
	var nonstop bool

	cmd := &cobra.Command{
		Use:         "flights <origin> <destination> <date>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Search Google Flights for a specific date (free, no API key required)",
		Long: `flights is flight-goat's headline command. It queries Google Flights via
flight-goat's native Go backend (no Python dependency) and returns real prices,
durations, airlines, and leg details. No API key. No auth. Just results.`,
		Example: `  # Cheapest SEA -> LHR on June 15
  flight-goat-pp-cli flights SEA LHR 2026-06-15

  # Non-stop only, business class, JSON for agents
  flight-goat-pp-cli flights JFK CDG 2026-07-01 --stops non_stop --class business --json

  # Morning departures on British Airways or KLM
  flight-goat-pp-cli flights JFK LHR 2026-07-01 --time 6-12 --airlines BA,KL

  # Show prices in GBP
  flight-goat-pp-cli flights MAN AGP 2026-05-10 --currency GBP --sort cheapest

  # Round trip with return date
  flight-goat-pp-cli flights SEA HNL 2026-08-01 --return 2026-08-10

  # Multi-city (repeat --segment, positional args become optional)
  flight-goat-pp-cli flights --segment "SFO>NRT@2026-08-15" --segment "NRT>ICN@2026-08-28" --segment "ICN>SFO@2026-09-05"`,
		Args: func(cmd *cobra.Command, args []string) error {
			// PATCH(library): multi-city mode (>=2 --segment values) makes
			// the positional <origin> <destination> <date> optional and
			// ignored. Single-segment positional invocation remains required
			// for one-way / round-trip.
			if len(segmentStrs) >= 2 {
				return nil
			}
			return cobra.ExactArgs(3)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var origin, destination, departureDate string
			var segments []gflights.Segment
			if len(segmentStrs) >= 2 {
				parsed, perr := parseMultiCitySegments(segmentStrs)
				if perr != nil {
					return perr
				}
				segments = parsed
				// PATCH(library): multi-city has its own provider dispatch
				// below; it does NOT flow through the normal gflights.Search
				// path because Kayak is the data source.
				return runMultiCity(cmd, flags, segments, provider, passengers, cabin, nonstop, currencyCode)
			} else if len(segmentStrs) == 1 {
				return fmt.Errorf("--segment requires >= 2 values for multi-city; got 1. Use the positional <origin> <destination> <date> form for a one-way search")
			} else {
				origin = strings.ToUpper(args[0])
				destination = strings.ToUpper(args[1])
				departureDate = args[2]
			}
			opts := gflights.SearchOptions{
				Origin:         origin,
				Destination:    destination,
				DepartureDate:  departureDate,
				ReturnDate:     returnDate,
				TimeWindow:     timeWindow,
				Airlines:       airlines,
				CabinClass:     cabin,
				MaxStops:       stops,
				SortBy:         sortBy,
				Passengers:     passengers,
				ExcludeBasic:   excludeBasic,
				Currency:       currencyCode,
				Emissions:      emissions,
				LimitedResults: limitedResults,
				Segments:       segments,
			}
			if checkedBags > 0 || carryOn {
				opts.Bags = &gflights.BagsFilter{CheckedBags: checkedBags, CarryOn: carryOn}
			}
			if len(layoverAirports) > 0 || maxLayoverMinutes > 0 {
				opts.Layover = &gflights.LayoverRestrictions{Airports: layoverAirports, MaxDuration: maxLayoverMinutes}
			}
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "gflights.Search(%s -> %s on %s)", opts.Origin, opts.Destination, opts.DepartureDate)
				if opts.ReturnDate != "" {
					fmt.Fprintf(cmd.OutOrStdout(), " return=%s", opts.ReturnDate)
				}
				if opts.MaxStops != "" {
					fmt.Fprintf(cmd.OutOrStdout(), " stops=%s", strings.ToUpper(opts.MaxStops))
				}
				if len(opts.Airlines) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), " airlines=%s", strings.Join(opts.Airlines, ","))
				}
				if opts.Currency != "" {
					fmt.Fprintf(cmd.OutOrStdout(), " currency=%s", strings.ToUpper(strings.TrimSpace(opts.Currency)))
				}
				fmt.Fprintln(cmd.OutOrStdout(), "\n(dry run - no request sent)")
				return nil
			}

			ctx := context.Background()
			result, err := gflights.Search(ctx, opts)
			if err != nil {
				return err
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				bts, _ := json.MarshalIndent(result, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(bts))
				return nil
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "%d flights found for %s -> %s on %s (source: %s)\n",
				result.Count, opts.Origin, opts.Destination, opts.DepartureDate, result.Source)

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "PRICE\tDURATION\tSTOPS\tAIRLINES\tDEPART\tARRIVE")
			limit := 15
			for i, f := range result.Flights {
				if i >= limit {
					fmt.Fprintf(cmd.ErrOrStderr(), "... and %d more (use --json for full list)\n", len(result.Flights)-limit)
					break
				}
				carriers := map[string]bool{}
				for _, l := range f.Legs {
					if l.Airline.Code != "" {
						carriers[l.Airline.Code] = true
					}
				}
				carrierList := make([]string, 0, len(carriers))
				for c := range carriers {
					carrierList = append(carrierList, c)
				}
				sort.Strings(carrierList)
				depart, arrive := "", ""
				if len(f.Legs) > 0 {
					depart = trimTime(f.Legs[0].DepartureTime)
					arrive = trimTime(f.Legs[len(f.Legs)-1].ArrivalTime)
				}
				fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\t%s\n",
					formatPrice(f.Currency, f.Price), minutesToHM(f.DurationMinutes), f.Stops, strings.Join(carrierList, ","), depart, arrive)
			}
			tw.Flush()
			return nil
		},
	}
	cmd.Flags().StringVarP(&returnDate, "return", "r", "", "Return date for round-trip (YYYY-MM-DD)")
	cmd.Flags().StringVarP(&timeWindow, "time", "t", "", "Departure time window in 24h format (e.g. 6-20 for 6am-8pm)")
	cmd.Flags().StringSliceVarP(&airlines, "airlines", "a", nil, "Airline IATA codes (e.g. BA,KL,DL)")
	cmd.Flags().StringVarP(&cabin, "class", "c", "", "Cabin class: economy, premium_economy, business, first")
	cmd.Flags().StringVarP(&stops, "stops", "s", "", "Max stops: any, non_stop, one_stop, two_plus_stops")
	cmd.Flags().StringVar(&sortBy, "sort", "", "Sort by: cheapest, top_flights, best, departure_time, arrival_time, duration, emissions")
	cmd.Flags().IntVarP(&passengers, "passengers", "p", 1, "Number of passengers")
	cmd.Flags().BoolVar(&excludeBasic, "exclude-basic", false, "Exclude basic economy fares")
	cmd.Flags().StringVar(&currencyCode, "currency", "", "Currency for prices (ISO 4217, e.g. GBP, EUR, USD; default USD)")
	// PATCH(upstream cli-printing-press): new flags exposing the filters
	// unlocked by the native Google Flights backend.
	cmd.Flags().StringVar(&emissions, "emissions", "", "Emissions filter: ALL (default) or LESS for lower-emission itineraries only")
	cmd.Flags().IntVarP(&checkedBags, "bags", "b", 0, "Include N checked-bag fees in the returned price (0, 1, or 2)")
	cmd.Flags().BoolVar(&carryOn, "carry-on", false, "Include carry-on bag fee in the returned price")
	cmd.Flags().StringSliceVarP(&layoverAirports, "layover", "l", nil, "Restrict layovers to specific airports (repeatable, e.g. -l ORD -l DFW)")
	cmd.Flags().IntVar(&maxLayoverMinutes, "max-layover", 0, "Maximum layover duration in minutes (0 = no constraint)")
	cmd.Flags().BoolVar(&limitedResults, "limited", false, "Return only the ~30 Google-curated results instead of the full set")
	cmd.Flags().StringSliceVar(&segmentStrs, "segment", nil, "Multi-city: repeatable segment in 'ORIG>DEST@YYYY-MM-DD' form. Pass >=2 to trigger multi-city search; positional args become optional and ignored.")
	cmd.Flags().StringVar(&provider, "provider", "auto", "Multi-city provider: 'auto' (Kayak for prices + Google URL fallback, default), 'kayak' (prices only), or 'google' (URL only — opens authenticated multi-city search in browser).")
	cmd.Flags().BoolVar(&nonstop, "nonstop", false, "Multi-city only: restrict to nonstop flights on every leg. Equivalent to /nonstop on Kayak.")
	return cmd
}

// runMultiCity dispatches a multi-city flight search to Kayak (which works
// server-side without auth) and/or the Google Flights URL builder.
//
// provider:
//
//	auto   — Kayak prices + Google URL appended (default)
//	kayak  — Kayak only
//	google — Google URL only (no prices; user opens in browser)
func runMultiCity(cmd *cobra.Command, flags *rootFlags, segments []gflights.Segment, provider string, passengers int, cabin string, nonstop bool, currencyCode string) error {
	if flags.dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "multi-city: %d segments provider=%s\n", len(segments), provider)
		return nil
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = "auto"
	}

	// Google URL is cheap; always compute when 'auto' or 'google'.
	var googleURL string
	if provider == "auto" || provider == "google" {
		mcsegs := make([]gflights.Segment, len(segments))
		copy(mcsegs, segments)
		u, err := gflights.MultiCityBookingURL(mcsegs)
		if err != nil {
			return fmt.Errorf("multi-city: google url: %w", err)
		}
		googleURL = u
	}

	// google-only short-circuit: emit URL-only envelope, no Kayak call.
	if provider == "google" {
		return emitMultiCityResult(cmd, flags, multiCityEnvelope{
			Success:    true,
			Complete:   true, // URL builder always produces the full deeplink
			Source:     "google_flights",
			SearchType: "flights_multicity",
			Query:      multiCityQueryEcho(segments, passengers, cabin, nonstop, currencyCode),
			GoogleURL:  googleURL,
		})
	}

	// kayak | auto: call Kayak for live prices.
	kc, err := kayak.NewMultiCityClient()
	if err != nil {
		return fmt.Errorf("multi-city: %w", err)
	}
	ksegs := make([]kayak.Segment, len(segments))
	for i, s := range segments {
		ksegs[i] = kayak.Segment{Origin: s.Origin, Destination: s.Destination, DepartureDate: s.DepartureDate}
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), 90*time.Second)
	defer cancel()
	kres, err := kc.SearchMultiCity(ctx, kayak.MultiCityOptions{
		Segments:   ksegs,
		Passengers: passengers,
		Cabin:      cabin,
		Nonstop:    nonstop,
		Currency:   currencyCode,
	})
	if err != nil {
		if provider == "kayak" {
			return fmt.Errorf("multi-city: kayak: %w", err)
		}
		// auto mode: degrade gracefully to URL-only with the kayak error attached
		return emitMultiCityResult(cmd, flags, multiCityEnvelope{
			Success:    true,
			Complete:   true, // the Google deeplink is fully formed even when Kayak fails
			Source:     "google_flights",
			SearchType: "flights_multicity",
			Query:      multiCityQueryEcho(segments, passengers, cabin, nonstop, currencyCode),
			GoogleURL:  googleURL,
			Note:       fmt.Sprintf("kayak unavailable, falling back to Google URL: %v", err),
		})
	}

	env := multiCityEnvelope{
		Success:     true,
		Complete:    kres.Complete,
		Source:      "kayak",
		SearchType:  "flights_multicity",
		Query:       multiCityQueryEcho(segments, passengers, cabin, nonstop, currencyCode),
		TotalCount:  kres.TotalCount,
		Count:       kres.Count,
		Itineraries: kres.Itineraries,
		KayakURL:    kres.SearchURL,
		GoogleURL:   googleURL, // populated only when provider==auto
	}
	if !kres.Complete {
		// Kayak's incremental search ran out of poll attempts before
		// converging — surface that the itineraries are partial so an
		// agent/user doesn't read Count as the full result set.
		env.Note = fmt.Sprintf("kayak search did not finish before the poll limit; showing %d of %d itineraries (partial — re-run for more)", kres.Count, kres.TotalCount)
	}
	return emitMultiCityResult(cmd, flags, env)
}

// multiCityEnvelope is the user-facing JSON shape for multi-city responses.
// Distinct from SearchResult because the leg shape differs materially: each
// itinerary covers all N requested segments rather than a single pair.
type multiCityEnvelope struct {
	Success bool `json:"success"`
	// Complete is false when a Kayak search exhausted its poll budget before
	// finishing; Itineraries is then a partial set and Note carries a warning.
	Complete    bool                    `json:"complete"`
	Source      string                  `json:"source"`
	SearchType  string                  `json:"search_type"`
	Query       multiCityQuery          `json:"query"`
	TotalCount  int                     `json:"total_count,omitempty"`
	Count       int                     `json:"count"`
	Itineraries []kayak.MultiCityFlight `json:"itineraries,omitempty"`
	KayakURL    string                  `json:"kayak_url,omitempty"`
	GoogleURL   string                  `json:"google_url,omitempty"`
	Note        string                  `json:"note,omitempty"`
}

type multiCityQuery struct {
	Segments   []gflights.Segment `json:"segments"`
	Passengers int                `json:"passengers"`
	Cabin      string             `json:"cabin,omitempty"`
	Nonstop    bool               `json:"nonstop,omitempty"`
	Currency   string             `json:"currency,omitempty"`
}

func multiCityQueryEcho(segs []gflights.Segment, pax int, cabin string, nonstop bool, currency string) multiCityQuery {
	return multiCityQuery{Segments: segs, Passengers: pax, Cabin: cabin, Nonstop: nonstop, Currency: currency}
}

// emitMultiCityResult writes the envelope as JSON when --json/agent is set,
// otherwise a compact human-readable summary.
func emitMultiCityResult(cmd *cobra.Command, flags *rootFlags, env multiCityEnvelope) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		b, _ := json.MarshalIndent(env, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Multi-city via %s — %d itineraries (of %d total)\n",
		env.Source, env.Count, env.TotalCount)
	if env.Note != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  note: %s\n", env.Note)
	}
	for i, f := range env.Itineraries {
		if i >= 10 {
			fmt.Fprintf(cmd.OutOrStdout(), "  ... and %d more (use --json)\n", len(env.Itineraries)-10)
			break
		}
		price := f.LocalizedPrice
		if price == "" && f.Price > 0 {
			price = fmt.Sprintf("%.0f %s", f.Price, f.Currency)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %d legs  %s\n", price, len(f.Legs), summarizeLegs(f.Legs))
	}
	if env.GoogleURL != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "\n  google: %s\n", env.GoogleURL)
	}
	return nil
}

func summarizeLegs(legs []kayak.MultiCityLeg) string {
	parts := make([]string, 0, len(legs))
	for _, l := range legs {
		parts = append(parts, fmt.Sprintf("%s→%s", l.Origin, l.Destination))
	}
	return strings.Join(parts, ", ")
}

// parseMultiCitySegments parses repeated --segment values of the form
// "ORIG>DEST@YYYY-MM-DD" into gflights.Segment objects. Lenient on whitespace
// and case but strict on the date format (YYYY-MM-DD) and the >/@ separators.
func parseMultiCitySegments(in []string) ([]gflights.Segment, error) {
	out := make([]gflights.Segment, 0, len(in))
	for i, raw := range in {
		s := strings.TrimSpace(raw)
		atIdx := strings.LastIndex(s, "@")
		if atIdx <= 0 {
			return nil, fmt.Errorf("--segment %d %q: missing date suffix '@YYYY-MM-DD'", i+1, raw)
		}
		route := strings.TrimSpace(s[:atIdx])
		date := strings.TrimSpace(s[atIdx+1:])
		gtIdx := strings.Index(route, ">")
		if gtIdx <= 0 || gtIdx >= len(route)-1 {
			return nil, fmt.Errorf("--segment %d %q: route must look like ORIG>DEST", i+1, raw)
		}
		origin := strings.ToUpper(strings.TrimSpace(route[:gtIdx]))
		destination := strings.ToUpper(strings.TrimSpace(route[gtIdx+1:]))
		if len(origin) < 3 || len(destination) < 3 {
			return nil, fmt.Errorf("--segment %d %q: origin and destination must be 3-letter IATA codes", i+1, raw)
		}
		// Validate date format here so the error message blames the user's
		// input rather than a downstream Kayak/Google failure when the
		// provider is Kayak-only (which skips gflights.MultiCityBookingURL,
		// the other date validator).
		if _, derr := time.Parse("2006-01-02", date); derr != nil {
			return nil, fmt.Errorf("--segment %d %q: date must be YYYY-MM-DD (e.g. 2026-08-15), got %q", i+1, raw, date)
		}
		out = append(out, gflights.Segment{Origin: origin, Destination: destination, DepartureDate: date})
	}
	return out, nil
}

// ----- dates: cheapest-dates discovery -----

func newGfDatesCmd(flags *rootFlags) *cobra.Command {
	// PATCH(upstream cli-printing-press#804): mirror the flights currency flag
	// on the calendar-price command that uses the same Google Flights backend.
	var from, to, cabin, stops, currencyCode string
	var duration int
	var round, doSort bool
	var airlines []string
	var limit int

	cmd := &cobra.Command{
		Use:         "dates <origin> <destination>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Find the cheapest dates to fly between two airports (free, no API key required)",
		Long: `dates scans Google Flights for the cheapest days to travel a route over
a range of dates. No API key required. Uses flight-goat's native Go backend
(no Python dependency).`,
		Example: `  # Cheapest dates SEA -> LHR over the next 2 months
  flight-goat-pp-cli dates SEA LHR

  # Non-stop business class, next month only
  flight-goat-pp-cli dates JFK CDG --from 2026-07-01 --to 2026-07-31 --stops non_stop --class business

  # Cheapest dates priced in EUR
  flight-goat-pp-cli dates JFK CDG --from 2026-07-01 --to 2026-07-31 --currency EUR --sort

  # Round trip with 7-day duration
  flight-goat-pp-cli dates SEA HNL --round --duration 7 --sort`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := gflights.DatesOptions{
				Origin:      strings.ToUpper(args[0]),
				Destination: strings.ToUpper(args[1]),
				From:        from,
				To:          to,
				Duration:    duration,
				Airlines:    airlines,
				RoundTrip:   round,
				MaxStops:    stops,
				CabinClass:  cabin,
				Sort:        doSort,
				Currency:    currencyCode,
			}
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "gflights.Dates(%s -> %s from=%s to=%s", opts.Origin, opts.Destination, opts.From, opts.To)
				if opts.Currency != "" {
					fmt.Fprintf(cmd.OutOrStdout(), " currency=%s", strings.ToUpper(strings.TrimSpace(opts.Currency)))
				}
				fmt.Fprintln(cmd.OutOrStdout(), ")")
				fmt.Fprintln(cmd.OutOrStdout(), "(dry run - no request sent)")
				return nil
			}

			ctx := context.Background()
			result, err := gflights.Dates(ctx, opts)
			if err != nil {
				return err
			}

			dates := result.Dates
			if doSort || flags.asJSON {
				sort.SliceStable(dates, func(i, j int) bool { return dates[i].Price < dates[j].Price })
			}
			if limit > 0 && len(dates) > limit {
				dates = dates[:limit]
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				bts, _ := json.MarshalIndent(struct {
					Origin      string               `json:"origin"`
					Destination string               `json:"destination"`
					Count       int                  `json:"count"`
					Dates       []gflights.DatePrice `json:"dates"`
					// PATCH(amend-2026-06-11): surface the HTML-fallback note
					// (set when Google's calendar RPC is blocked) to agents.
					Note string `json:"note,omitempty"`
				}{opts.Origin, opts.Destination, len(dates), dates, result.Note}, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(bts))
				return nil
			}

			if result.Note != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: %s\n", result.Note)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "%d dates priced for %s -> %s\n", len(dates), opts.Origin, opts.Destination)
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "DATE\tPRICE")
			for _, p := range dates {
				fmt.Fprintf(tw, "%s\t%s\n", p.DepartureDate, formatPrice(p.Currency, p.Price))
			}
			tw.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "End date (YYYY-MM-DD)")
	cmd.Flags().IntVarP(&duration, "duration", "d", 0, "Trip duration in days (round-trip)")
	cmd.Flags().StringSliceVarP(&airlines, "airlines", "a", nil, "Airline IATA codes")
	cmd.Flags().BoolVarP(&round, "round", "R", false, "Search for round-trip flights")
	cmd.Flags().StringVarP(&stops, "stops", "s", "", "Max stops: any, non_stop, one_stop")
	cmd.Flags().StringVarP(&cabin, "class", "c", "", "Cabin class: economy, premium_economy, business, first")
	cmd.Flags().BoolVar(&doSort, "sort", false, "Sort by price ascending")
	cmd.Flags().IntVarP(&limit, "limit", "l", 0, "Limit to top N dates (0 = all)")
	cmd.Flags().StringVar(&currencyCode, "currency", "", "Currency for prices (ISO 4217, e.g. GBP, EUR, USD; default USD)")
	return cmd
}

// ----- shared helpers -----

func formatPrice(code string, price float64) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		code = "USD"
	}
	return fmt.Sprintf("%s %.0f", code, price)
}

func minutesToHM(m int) string {
	if m <= 0 {
		return "?"
	}
	return fmt.Sprintf("%dh%02dm", m/60, m%60)
}

func trimTime(s string) string {
	// fli returns "2026-06-15T15:40:00", keep just date + HH:MM
	if len(s) >= 16 {
		return s[:10] + " " + s[11:16]
	}
	return s
}

// ----- explore: Kayak /direct nonstop matrix -----

func newKayakExploreCmd(flags *rootFlags) *cobra.Command {
	var minFrequency int
	var country string
	var sortBy string
	var limit int

	cmd := &cobra.Command{
		Use:         "explore <airport>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Every nonstop destination from an airport (free, via Kayak /direct)",
		Long: `explore fetches Kayak's /direct/<airport> page and parses the nonstop
destinations table that Kayak server-renders into the HTML. Same data you see
on www.kayak.com/direct/SEA, but in your terminal as structured output.

No API key, no scraping (Kayak embeds the full routes array server-side), no
browser automation. Just one HTTP GET.

Data includes: destination code, city, country, distance, nonstop flight
duration, number of daily flights, and operating airlines.`,
		Example: `  # Every nonstop destination from SEA
  flight-goat-pp-cli explore SEA

  # Only destinations with 3+ daily flights
  flight-goat-pp-cli explore SEA --min-frequency 3

  # International only
  flight-goat-pp-cli explore SEA --country-not US --json

  # Longest nonstop flights first
  flight-goat-pp-cli explore SEA --sort duration`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			airport := strings.ToUpper(strings.TrimSpace(args[0]))
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "GET https://www.kayak.com/direct/%s\n(dry run - no request sent)\n", airport)
				return nil
			}
			client := kayak.New()
			routes, err := client.Direct(airport)
			if err != nil {
				return err
			}

			filtered := routes[:0]
			for _, r := range routes {
				if r.FlightsCount < minFrequency {
					continue
				}
				if country != "" && !strings.EqualFold(r.CountryCode, country) {
					continue
				}
				filtered = append(filtered, r)
			}

			switch strings.ToLower(sortBy) {
			case "duration", "dur":
				sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].Duration > filtered[j].Duration })
			case "distance", "dist":
				sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].DistanceMiles > filtered[j].DistanceMiles })
			case "frequency", "freq", "flights":
				sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].FlightsCount > filtered[j].FlightsCount })
			default:
				sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].FlightsCount > filtered[j].FlightsCount })
			}

			if limit > 0 && len(filtered) > limit {
				filtered = filtered[:limit]
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				bts, _ := json.MarshalIndent(struct {
					Origin string        `json:"origin"`
					Source string        `json:"source"`
					Count  int           `json:"count"`
					Routes []kayak.Route `json:"routes"`
				}{airport, "kayak-direct", len(filtered), filtered}, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(bts))
				return nil
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "%d nonstop destinations from %s (source: kayak-direct)\n", len(filtered), airport)
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "CODE\tCITY\tCOUNTRY\tDURATION\tDISTANCE\tFLIGHTS\tAIRLINES")
			for _, r := range filtered {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d mi\t%d\t%s\n",
					r.Code, r.DisplayLocation, r.CountryCode,
					minutesToHM(r.Duration), r.DistanceMiles, r.FlightsCount,
					strings.Join(r.AirlineCodes, ","))
			}
			tw.Flush()
			return nil
		},
	}
	cmd.Flags().IntVar(&minFrequency, "min-frequency", 0, "Only destinations with at least N flights per day")
	cmd.Flags().StringVar(&country, "country", "", "Filter to a two-letter country code (e.g. GB, JP)")
	cmd.Flags().StringVar(&sortBy, "sort", "frequency", "Sort by: frequency, duration, distance")
	cmd.Flags().IntVarP(&limit, "limit", "l", 0, "Limit to top N results (0 = all)")
	return cmd
}

// ----- longhaul: Kayak-backed longhaul nonstop discovery -----

func newKayakLonghaulCmd(flags *rootFlags) *cobra.Command {
	var minHours, maxHours float64
	var country string
	var minFrequency int
	var limit int

	cmd := &cobra.Command{
		Use:         "longhaul <airport>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Nonstop destinations from an airport filtered by minimum flight duration (free, via Kayak)",
		Long: `longhaul is the headline flight-goat command. It answers the classic
travel-hacker question: "show me every nonstop flight from my airport that's
at least N hours long, so I know where I can actually use a long-haul redemption."

Source: Kayak's /direct/<airport> page, which server-renders the full nonstop
destinations table with durations into HTML. flight-goat parses the embedded
data directly (no browser, no API key, no scraping). This is the same data
you'd see on www.kayak.com/direct/SEA in a browser.`,
		Example: `  # Every nonstop flight from SEA that's 8+ hours
  flight-goat-pp-cli longhaul SEA --min-hours 8

  # 10+ hour flights, international only, with at least 1 flight per day
  flight-goat-pp-cli longhaul SEA --min-hours 10 --country-not US --min-frequency 1

  # Medium-haul range: 5 to 8 hours
  flight-goat-pp-cli longhaul SEA --min-hours 5 --max-hours 8

  # JSON output for agents
  flight-goat-pp-cli longhaul SEA --min-hours 8 --json | jq '.routes[].code'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			airport := strings.ToUpper(strings.TrimSpace(args[0]))
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "GET https://www.kayak.com/direct/%s\nfilter: duration >= %.1fh", airport, minHours)
				if maxHours > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), ", duration <= %.1fh", maxHours)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "\n(dry run - no request sent)")
				return nil
			}
			client := kayak.New()
			routes, err := client.Direct(airport)
			if err != nil {
				return err
			}

			minMin := int(minHours * 60)
			maxMin := int(maxHours * 60)
			filtered := routes[:0]
			for _, r := range routes {
				if r.Duration < minMin {
					continue
				}
				if maxMin > 0 && r.Duration > maxMin {
					continue
				}
				if r.FlightsCount < minFrequency {
					continue
				}
				if country != "" && !strings.EqualFold(r.CountryCode, country) {
					continue
				}
				filtered = append(filtered, r)
			}
			sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].Duration > filtered[j].Duration })
			if limit > 0 && len(filtered) > limit {
				filtered = filtered[:limit]
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				bts, _ := json.MarshalIndent(struct {
					Origin   string        `json:"origin"`
					MinHours float64       `json:"min_hours"`
					Source   string        `json:"source"`
					Count    int           `json:"count"`
					Routes   []kayak.Route `json:"routes"`
				}{airport, minHours, "kayak-direct", len(filtered), filtered}, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(bts))
				return nil
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "%d nonstop destinations from %s with flights >= %.1fh (source: kayak-direct)\n",
				len(filtered), airport, minHours)
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "CODE\tCITY\tCOUNTRY\tDURATION\tDISTANCE\tFLIGHTS\tAIRLINES")
			for _, r := range filtered {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d mi\t%d\t%s\n",
					r.Code, r.DisplayLocation, r.CountryCode,
					minutesToHM(r.Duration), r.DistanceMiles, r.FlightsCount,
					strings.Join(r.AirlineCodes, ","))
			}
			tw.Flush()
			return nil
		},
	}
	cmd.Flags().Float64Var(&minHours, "min-hours", 8, "Minimum flight duration in hours")
	cmd.Flags().Float64Var(&maxHours, "max-hours", 0, "Maximum flight duration in hours (0 = unbounded)")
	cmd.Flags().StringVar(&country, "country", "", "Only include destinations in this country code (e.g. JP)")
	cmd.Flags().IntVar(&minFrequency, "min-frequency", 0, "Only destinations with at least N flights per day")
	cmd.Flags().IntVarP(&limit, "limit", "l", 0, "Limit to top N (0 = all)")
	return cmd
}

// silence unused import warnings on slim builds
var _ = io.Discard
var _ = time.Now
