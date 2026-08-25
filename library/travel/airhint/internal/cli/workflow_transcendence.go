// Copyright 2026 jvm and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

// predictSweepResult holds the prediction for a single date in a sweep.
type predictSweepResult struct {
	Date       string  `json:"date"`
	Price      float64 `json:"price"`
	Suggestion string  `json:"suggestion"`
	Confidence int     `json:"confidence"`
	MaxSaving  string  `json:"max_saving"`
	PriceMin   float64 `json:"price_min"`
	PriceMax   float64 `json:"price_max"`
	Error      string  `json:"error,omitempty"`
}

type predictResponse struct {
	Suggestion     string    `json:"suggestion"`
	Confidence     int       `json:"confidence"`
	Recommendation []string  `json:"recommendation"`
	Hints          []string  `json:"hints"`
	PriceRange     []float64 `json:"price_range"`
	MaxSaving      string    `json:"max_saving"`
}

type searchInitResponse struct {
	SearchID string `json:"search_id"`
}

type flightResult struct {
	TotalPrice      float64          `json:"total_price"`
	Currency        string           `json:"currency"`
	CabinClass      string           `json:"cabin_class"`
	OutboundFlights []outboundFlight `json:"outbound_flights"`
}

type outboundFlight struct {
	Airline string `json:"airline"`
}

// newWorkflowPredictSweepCmd sweeps predictions across a date range for a route.
// pp:data-source live
func newWorkflowPredictSweepCmd(flags *rootFlags) *cobra.Command {
	var (
		airline     string
		currency    string
		cabinClass  string
		passengers  int
		concurrency int
		fromDate    string
		toDate      string
		days        int
	)

	cmd := &cobra.Command{
		Use:   "predict-sweep <origin> <destination>",
		Short: "Predict buy/wait across a date range for a route",
		Long: `Sweep predictions across multiple departure dates for a route.
Fetches the current price for each date from the AirHint search API,
then runs the prediction model and returns a ranked list of best dates to buy.`,
		Example: `  # Sweep Aug 1-31 STN→DUB on Ryanair
  airhint-pp-cli workflow predict-sweep STN DUB --from 2026-08-01 --to 2026-08-31 --airline FR

  # Check next 14 days for cheapest prediction
  airhint-pp-cli workflow predict-sweep BCN MAD --days 14 --airline VY --json`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			origin := strings.ToUpper(args[0])
			destination := strings.ToUpper(args[1])

			// Resolve date range
			var dates []string
			if days > 0 {
				start := time.Now().AddDate(0, 0, 1)
				for i := 0; i < days; i++ {
					dates = append(dates, start.AddDate(0, 0, i).Format("2006-01-02"))
				}
			} else {
				startT, err := time.Parse("2006-01-02", fromDate)
				if err != nil {
					return fmt.Errorf("invalid --from date %q: %w", fromDate, err)
				}
				endT, err := time.Parse("2006-01-02", toDate)
				if err != nil {
					return fmt.Errorf("invalid --to date %q: %w", toDate, err)
				}
				if endT.Before(startT) {
					return fmt.Errorf("--to must be after --from")
				}
				for d := startT; !d.After(endT); d = d.AddDate(0, 0, 1) {
					dates = append(dates, d.Format("2006-01-02"))
				}
			}
			if len(dates) == 0 {
				return fmt.Errorf("no dates in range; specify --from/--to or --days")
			}
			if len(dates) > 90 {
				return fmt.Errorf("date range too large (%d days, max 90); narrow --from/--to or use --days", len(dates))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type work struct {
				date string
			}
			type result struct {
				r predictSweepResult
			}

			workCh := make(chan work, len(dates))
			resCh := make(chan result, len(dates))

			for _, d := range dates {
				workCh <- work{date: d}
			}
			close(workCh)

			if concurrency < 1 {
				concurrency = 3
			}
			if concurrency > 5 {
				concurrency = 5
			}
			var wg sync.WaitGroup
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for w := range workCh {
						r := predictSweepResult{Date: w.date}

						// Step 1: search for flights on that date (bypass cache so each sweep gets a fresh search_id)
						searchParams := map[string]string{
							"currency": currency,
							"adults":   fmt.Sprintf("%d", passengers),
							"direct":   "true",
						}
						if airline != "" {
							searchParams["airline"] = strings.ToLower(airlineCodeToName(airline))
						}
						searchPath := fmt.Sprintf("/search/%s/%s/%s", origin, destination, w.date)
						initRaw, searchErr := c.GetNoCache(cmd.Context(), searchPath, searchParams)
						if searchErr != nil {
							r.Error = searchErr.Error()
							resCh <- result{r: r}
							continue
						}

						var initResp searchInitResponse
						if jsonErr := json.Unmarshal(initRaw, &initResp); jsonErr != nil || initResp.SearchID == "" {
							r.Error = "could not parse search_id from search response"
							resCh <- result{r: r}
							continue
						}

						// Step 2: poll for results with retries (server processes async)
						pollPath := fmt.Sprintf("/search/%s", initResp.SearchID)
						var flights []flightResult
						for attempt := 0; attempt < 3; attempt++ {
							if attempt > 0 {
								time.Sleep(800 * time.Millisecond)
							}
							flightsRaw, pollErr := c.GetNoCache(cmd.Context(), pollPath, nil)
							if pollErr != nil {
								r.Error = pollErr.Error()
								break
							}
							if jsonErr := json.Unmarshal(flightsRaw, &flights); jsonErr == nil && len(flights) > 0 {
								break
							}
						}
						if len(flights) == 0 {
							if r.Error == "" {
								r.Error = "no flights found for this date"
							}
							resCh <- result{r: r}
							continue
						}

						// Use the cheapest flight
						cheapest := flights[0]
						for _, f := range flights[1:] {
							if f.TotalPrice < cheapest.TotalPrice {
								cheapest = f
							}
						}
						r.Price = cheapest.TotalPrice

						// Determine airline code for predict
						airlineCode := airline
						if airlineCode == "" && len(cheapest.OutboundFlights) > 0 {
							airlineCode = cheapest.OutboundFlights[0].Airline
						}
						if airlineCode == "" {
							r.Error = "could not determine airline code"
							resCh <- result{r: r}
							continue
						}

						// Step 3: predict
						predictPath := fmt.Sprintf("/predict/%s/%s/%s/%s",
							strings.ToUpper(airlineCode), origin, destination, w.date)
						predictParams := map[string]string{
							"price":         fmt.Sprintf("%.2f", cheapest.TotalPrice),
							"outboundPrice": fmt.Sprintf("%.2f", cheapest.TotalPrice),
							"currency":      currency,
							"cabin_class":   cabinClass,
							"passengers":    fmt.Sprintf("%d", passengers),
						}
						predRaw, predErr := c.Get(cmd.Context(), predictPath, predictParams)
						if predErr != nil {
							r.Error = predErr.Error()
							resCh <- result{r: r}
							continue
						}

						var pred predictResponse
						if jsonErr := json.Unmarshal(predRaw, &pred); jsonErr != nil {
							r.Error = "could not parse prediction response"
							resCh <- result{r: r}
							continue
						}
						r.Suggestion = pred.Suggestion
						r.Confidence = pred.Confidence
						r.MaxSaving = pred.MaxSaving
						if len(pred.PriceRange) >= 2 {
							r.PriceMin = pred.PriceRange[0]
							r.PriceMax = pred.PriceRange[1]
						}
						resCh <- result{r: r}
					}
				}()
			}
			wg.Wait()
			close(resCh)

			var allResults []predictSweepResult
			for res := range resCh {
				allResults = append(allResults, res.r)
			}

			// Sort by date
			sort.Slice(allResults, func(i, j int) bool {
				return allResults[i].Date < allResults[j].Date
			})

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"origin":      origin,
					"destination": destination,
					"airline":     airline,
					"currency":    currency,
					"results":     allResults,
				})
			}

			// Human-readable table
			fmt.Fprintf(cmd.OutOrStdout(), "Prediction sweep: %s → %s\n\n", origin, destination)
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-8s %-6s %-10s %s\n", "Date", "Price", "Action", "Confidence", "Max Saving")
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", strings.Repeat("-", 55))
			for _, r := range allResults {
				if r.Error != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-8s %-6s %-10s %s\n", r.Date, "-", "?", "-", r.Error)
					continue
				}
				action := r.Suggestion
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-8.2f %-6s %-10d %s\n",
					r.Date, r.Price, action, r.Confidence, r.MaxSaving)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&fromDate, "from", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&toDate, "to", "", "End date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&days, "days", 0, "Sweep next N days starting tomorrow (alternative to --from/--to)")
	cmd.Flags().StringVar(&airline, "airline", "", "IATA airline code to filter (e.g. FR, U2, W6)")
	cmd.Flags().StringVar(&currency, "currency", "EUR", "Currency code")
	cmd.Flags().StringVar(&cabinClass, "cabin-class", "Y", "Cabin class (Y=Economy)")
	cmd.Flags().IntVar(&passengers, "passengers", 1, "Number of passengers")
	cmd.Flags().IntVar(&concurrency, "concurrency", 3, "Parallel requests (max 5)")

	return cmd
}

// newWorkflowCompareRoutesCmd compares predictions across multiple routes.
// pp:data-source live
func newWorkflowCompareRoutesCmd(flags *rootFlags) *cobra.Command {
	var (
		airline    string
		currency   string
		cabinClass string
		passengers int
		dateStr    string
	)

	cmd := &cobra.Command{
		Use:   "compare-routes <date> <origin1:destination1> [origin2:destination2 ...]",
		Short: "Compare buy/wait predictions across multiple routes on the same date",
		Long: `Fetch predictions for multiple origin-destination pairs on the same departure date.
Useful for comparing trip options and finding the best moment to book across routes.`,
		Example: `  # Compare London departures on the same date
  airhint-pp-cli workflow compare-routes 2026-08-16 STN:DUB LGW:BCN LHR:CDG --airline FR

  # Compare with JSON output for scripting
  airhint-pp-cli workflow compare-routes 2026-08-16 MAD:LIS MAD:BCN --json`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dateStr = args[0]
			if _, err := time.Parse("2006-01-02", dateStr); err != nil {
				return fmt.Errorf("invalid date %q (expected YYYY-MM-DD): %w", dateStr, err)
			}

			type routePair struct {
				origin, destination string
			}
			var routes []routePair
			for _, pair := range args[1:] {
				parts := strings.SplitN(pair, ":", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid route %q: expected ORIGIN:DESTINATION (e.g. STN:DUB)", pair)
				}
				routes = append(routes, routePair{
					origin:      strings.ToUpper(parts[0]),
					destination: strings.ToUpper(parts[1]),
				})
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type compareResult struct {
				Origin      string  `json:"origin"`
				Destination string  `json:"destination"`
				Price       float64 `json:"price"`
				Suggestion  string  `json:"suggestion"`
				Confidence  int     `json:"confidence"`
				MaxSaving   string  `json:"max_saving"`
				Error       string  `json:"error,omitempty"`
			}

			results := make([]compareResult, len(routes))
			var wg sync.WaitGroup
			const maxCompareConc = 5
			sem := make(chan struct{}, maxCompareConc)
			for i, route := range routes {
				wg.Add(1)
				go func(idx int, r routePair) {
					sem <- struct{}{}
					defer func() { <-sem; wg.Done() }()
					res := compareResult{Origin: r.origin, Destination: r.destination}

					// Search (bypass cache to always get a fresh search_id)
					searchParams := map[string]string{
						"currency": currency,
						"adults":   fmt.Sprintf("%d", passengers),
						"direct":   "true",
					}
					if airline != "" {
						searchParams["airline"] = strings.ToLower(airlineCodeToName(airline))
					}
					searchPath := fmt.Sprintf("/search/%s/%s/%s", r.origin, r.destination, dateStr)
					initRaw, searchErr := c.GetNoCache(cmd.Context(), searchPath, searchParams)
					if searchErr != nil {
						res.Error = searchErr.Error()
						results[idx] = res
						return
					}

					var initResp searchInitResponse
					if jsonErr := json.Unmarshal(initRaw, &initResp); jsonErr != nil || initResp.SearchID == "" {
						res.Error = "could not parse search_id"
						results[idx] = res
						return
					}

					pollPath := fmt.Sprintf("/search/%s", initResp.SearchID)
					var flights []flightResult
					for attempt := 0; attempt < 3; attempt++ {
						if attempt > 0 {
							time.Sleep(800 * time.Millisecond)
						}
						flightsRaw, pollErr := c.GetNoCache(cmd.Context(), pollPath, nil)
						if pollErr != nil {
							res.Error = pollErr.Error()
							break
						}
						if jsonErr := json.Unmarshal(flightsRaw, &flights); jsonErr == nil && len(flights) > 0 {
							break
						}
					}
					if len(flights) == 0 {
						if res.Error == "" {
							res.Error = "no flights found"
						}
						results[idx] = res
						return
					}

					cheapest := flights[0]
					for _, f := range flights[1:] {
						if f.TotalPrice < cheapest.TotalPrice {
							cheapest = f
						}
					}
					res.Price = cheapest.TotalPrice

					airlineCode := airline
					if airlineCode == "" && len(cheapest.OutboundFlights) > 0 {
						airlineCode = cheapest.OutboundFlights[0].Airline
					}
					if airlineCode == "" {
						res.Error = "could not determine airline code"
						results[idx] = res
						return
					}

					predictPath := fmt.Sprintf("/predict/%s/%s/%s/%s",
						strings.ToUpper(airlineCode), r.origin, r.destination, dateStr)
					predictParams := map[string]string{
						"price":         fmt.Sprintf("%.2f", cheapest.TotalPrice),
						"outboundPrice": fmt.Sprintf("%.2f", cheapest.TotalPrice),
						"currency":      currency,
						"cabin_class":   cabinClass,
						"passengers":    fmt.Sprintf("%d", passengers),
					}
					predRaw, predErr := c.Get(cmd.Context(), predictPath, predictParams)
					if predErr != nil {
						res.Error = predErr.Error()
						results[idx] = res
						return
					}

					var pred predictResponse
					if jsonErr := json.Unmarshal(predRaw, &pred); jsonErr != nil {
						res.Error = "could not parse prediction"
						results[idx] = res
						return
					}
					res.Suggestion = pred.Suggestion
					res.Confidence = pred.Confidence
					res.MaxSaving = pred.MaxSaving
					results[idx] = res
				}(i, route)
			}
			wg.Wait()

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"date":   dateStr,
					"routes": results,
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Route comparison for %s\n\n", dateStr)
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-8s %-6s %-10s %s\n", "Route", "Price", "Action", "Confidence", "Max Saving")
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", strings.Repeat("-", 55))
			for _, r := range results {
				route := r.Origin + "→" + r.Destination
				if r.Error != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-8s %-6s %-10s %s\n", route, "-", "?", "-", r.Error)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-8.2f %-6s %-10d %s\n",
					route, r.Price, r.Suggestion, r.Confidence, r.MaxSaving)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&airline, "airline", "", "IATA airline code filter (e.g. FR, U2)")
	cmd.Flags().StringVar(&currency, "currency", "EUR", "Currency code")
	cmd.Flags().StringVar(&cabinClass, "cabin-class", "Y", "Cabin class (Y=Economy)")
	cmd.Flags().IntVar(&passengers, "passengers", 1, "Number of passengers")

	return cmd
}

// newWorkflowCheapestWindowCmd finds the cheapest date window in a month.
// pp:data-source live
func newWorkflowCheapestWindowCmd(flags *rootFlags) *cobra.Command {
	var (
		airline    string
		currency   string
		maxPrice   int
		directOnly bool
	)

	cmd := &cobra.Command{
		Use:   "cheapest-window <origin> <destination> <month>",
		Short: "Find the cheapest flight date in a given month for a route",
		Long: `Queries the cheapest-deal-month endpoint to find the lowest-priced date
in a given month for a route, then fetches the full prediction for that date.`,
		Example: `  # Find cheapest August date STN→DUB
  airhint-pp-cli workflow cheapest-window STN DUB 8 --airline FR

  # Check September with price cap
  airhint-pp-cli workflow cheapest-window LHR BCN 9 --max-price 100 --json`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			origin := strings.ToUpper(args[0])
			destination := strings.ToUpper(args[1])
			month := args[2]

			// Validate month
			var monthNum int
			if _, err := fmt.Sscanf(month, "%d", &monthNum); err != nil || monthNum < 1 || monthNum > 12 {
				return fmt.Errorf("invalid month %q: must be 1-12", month)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Get cheapest deal for the month
			cheapestPath := fmt.Sprintf("/cheapest-deal-month/one-way/%s/%s/%d", origin, destination, monthNum)
			cheapestParams := map[string]string{
				"currency":   currency,
				"directOnly": fmt.Sprintf("%v", directOnly),
				"maxPrice":   fmt.Sprintf("%d", maxPrice),
			}
			cheapestRaw, err := c.Get(cmd.Context(), cheapestPath, cheapestParams)
			if err != nil {
				return fmt.Errorf("fetching cheapest deal: %w", err)
			}

			var cheapestResp struct {
				Cheapest string `json:"cheapest"`
			}
			if jsonErr := json.Unmarshal(cheapestRaw, &cheapestResp); jsonErr != nil {
				return fmt.Errorf("parsing cheapest response: %w", jsonErr)
			}

			// If airline is known, also get cheapest airline deal
			type airlineDealResp struct {
				DepartureDate string  `json:"departure_date"`
				Price         float64 `json:"price"`
				Currency      string  `json:"currency"`
			}
			var airlineDeal *airlineDealResp

			if airline != "" {
				// Use current year + month to build a reference date
				now := time.Now()
				refDate := fmt.Sprintf("%04d-%02d-01", now.Year(), monthNum)
				if monthNum < int(now.Month()) {
					refDate = fmt.Sprintf("%04d-%02d-01", now.Year()+1, monthNum)
				}

				airlinePath := fmt.Sprintf("/cheapest-airline-deal-month/%s/%s/%s/%s/%s",
					strings.ToUpper(airline), origin, destination, refDate, currency)
				dealRaw, dealErr := c.Get(cmd.Context(), airlinePath, nil)
				if dealErr == nil {
					var deal airlineDealResp
					if json.Unmarshal(dealRaw, &deal) == nil && deal.DepartureDate != "" {
						airlineDeal = &deal
					}
				}
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"origin":            origin,
					"destination":       destination,
					"month":             monthNum,
					"cheapest_in_month": cheapestResp.Cheapest,
					"airline_deal":      airlineDeal,
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Cheapest window: %s → %s (month %d)\n\n", origin, destination, monthNum)
			fmt.Fprintf(cmd.OutOrStdout(), "Cheapest fare in month: %s\n", cheapestResp.Cheapest)
			if airlineDeal != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Cheapest %s date: %s at %.2f %s\n",
					strings.ToUpper(airline), airlineDeal.DepartureDate, airlineDeal.Price, airlineDeal.Currency)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&airline, "airline", "", "IATA airline code (e.g. FR, U2)")
	cmd.Flags().StringVar(&currency, "currency", "EUR", "Currency code")
	cmd.Flags().IntVar(&maxPrice, "max-price", 1000, "Maximum price filter")
	cmd.Flags().BoolVar(&directOnly, "direct-only", true, "Direct flights only")

	return cmd
}

// airlineCodeToName maps IATA airline codes to the lowercase names AirHint uses in the search filter.
func airlineCodeToName(code string) string {
	names := map[string]string{
		"FR": "ryanair",
		"U2": "easyjet",
		"W6": "wizz",
		"VY": "vueling",
		"IB": "iberia",
		"BA": "british-airways",
		"LH": "lufthansa",
		"AF": "air-france",
		"KL": "klm",
		"AZ": "alitalia",
	}
	if name, ok := names[strings.ToUpper(code)]; ok {
		return name
	}
	return strings.ToLower(code)
}
