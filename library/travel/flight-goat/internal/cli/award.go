// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(library): Seats.aero award (mileage) availability source. Not generated
// by Printing Press — seats.aero has no OpenAPI spec imported into flight-goat;
// this is a hand-written backend beside Google Flights, Kayak, and FlySoar. See
// internal/seatsaero/seatsaero.go and the patch record in .printing-press-patches/.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/flight-goat/internal/seatsaero"

	"github.com/spf13/cobra"
)

// newAwardCmd wires the `award` command: a Seats.aero cached award-availability
// search for a route, returning mileage-program redemption options across cabin
// classes. Unlike `flights`/`soar` (cash fares, no auth), award requires a
// Seats.aero Partner API key via SEATS_AERO_API_KEY and returns miles, not
// dollars — so it's a separate command, not a flag on `flights`. Read-only.
func newAwardCmd(flags *rootFlags) *cobra.Command {
	var startDate, endDate, cabin, orderBy string
	var onlyDirect bool
	var take int

	cmd := &cobra.Command{
		Use:         "award <origin> <destination> [--from YYYY-MM-DD --to YYYY-MM-DD]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Search Seats.aero award (mileage) availability for a route (requires SEATS_AERO_API_KEY)",
		Long: `award searches Seats.aero's cached award-travel availability between two airports,
returning mileage-program redemption options across economy/premium/business/first
cabins. This is miles + taxes pricing, not cash fares: each row shows the mileage
program (e.g. united, aeroplan), the departure date, which cabins are available,
and the mileage cost in the program's currency.

Seats.aero's cached search is available to eligible Pro users with a Partner
API key; set it as SEATS_AERO_API_KEY (the same env var the standalone
seats-aero skill uses). Commercial API use requires Seats.aero's written
permission. Live search is not exposed here. This command is read-only;
it never books or mutates anything.

By default results are ordered by departure date (premium cabins first); pass
--order lowest_mileage to rank by cheapest award cost instead.`,
		Example: `  # Award availability SFO -> HND around 2026-10-01, cheapest-first
  flight-goat-pp-cli award SFO HND --from 2026-10-01 --to 2026-10-31 --order lowest_mileage

  # Business-class only, nonstop, this week
  flight-goat-pp-cli award JFK LHR --from 2026-09-20 --to 2026-09-27 --cabin business --only-direct

  # Economy, JSON for agents
  flight-goat-pp-cli award SFO NRT --cabin economy --json`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			origin, err := normalizeAirportList(args[0], "origin")
			if err != nil {
				return usageErr(err)
			}
			dest, err := normalizeAirportList(args[1], "destination")
			if err != nil {
				return usageErr(err)
			}
			var start, end time.Time
			if startDate != "" {
				start, err = time.Parse("2006-01-02", startDate)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --from %q: use YYYY-MM-DD", startDate))
				}
				startDate = start.Format("2006-01-02")
			}
			if endDate != "" {
				end, err = time.Parse("2006-01-02", endDate)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --to %q: use YYYY-MM-DD", endDate))
				}
				endDate = end.Format("2006-01-02")
			}
			if !start.IsZero() && !end.IsZero() && end.Before(start) {
				return usageErr(fmt.Errorf("invalid date window: --to must be on or after --from"))
			}
			cabin, err = normalizeCabinList(cabin)
			if err != nil {
				return usageErr(err)
			}
			// Profile overlays do not mark the Cobra flag changed. A stored zero
			// is still an explicit limit and must not become the API default.
			takeConfigured := take != 0 || cmd.Flags().Changed("take")
			if !takeConfigured && flags.profileName != "" {
				profile, err := GetProfile(flags.profileName)
				if err != nil {
					return err
				}
				if profile != nil {
					_, takeConfigured = profile.Values["take"]
				}
			}
			if takeConfigured && (take < 10 || take > 1000) {
				return usageErr(fmt.Errorf("invalid --take %d: use 10..1000, or omit it for the default", take))
			}

			// Normalize ordering aliases to the API's allowed enum, which is only
			// "" (default: by date, premium-first) or "lowest_mileage". Accept
			// "cheapest" as an alias for lowest_mileage so a user asking for the
			// cheapest award isn't sent an out-of-enum value (review finding).
			normalizedOrder := ""
			switch strings.ToLower(strings.TrimSpace(orderBy)) {
			case "", "date", "default":
			case "cheapest", "lowest", "lowest_mileage", "miles":
				normalizedOrder = "lowest_mileage"
			default:
				return usageErr(fmt.Errorf("invalid --order %q: use lowest_mileage (or cheapest) for lowest award cost, or leave empty for default date ordering", orderBy))
			}

			params := seatsaero.SearchParams{
				OriginAirport:      origin,
				DestinationAirport: dest,
				StartDate:          startDate,
				EndDate:            endDate,
				OrderBy:            normalizedOrder,
				OnlyDirectFlights:  onlyDirect,
				Take:               take,
			}
			if strings.Contains(cabin, ",") {
				params.Cabins = cabin
			} else {
				params.Cabin = cabin
			}
			c := seatsaero.NewClient("")

			if flags.dryRun {
				// Build the URL to echo (mirrors soar's dry-run contract). Do
				// not leak the API key. Dry-run works without a key.
				u := c.BaseURL + "/search?origin_airport=" + origin + "&destination_airport=" + dest
				if startDate != "" {
					u += "&start_date=" + startDate
				}
				if endDate != "" {
					u += "&end_date=" + endDate
				}
				if params.Cabin != "" {
					u += "&cabin=" + params.Cabin
				}
				if params.Cabins != "" {
					u += "&cabins=" + params.Cabins
				}
				if params.OrderBy != "" {
					u += "&order_by=" + params.OrderBy
				}
				if onlyDirect {
					u += "&only_direct_flights=true"
				}
				if take > 0 {
					u += fmt.Sprintf("&take=%d", take)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "seatsaero.Search(%s -> %s)", params.OriginAirport, params.DestinationAirport)
				if startDate != "" {
					fmt.Fprintf(cmd.OutOrStdout(), " %s..%s", startDate, endDate)
				}
				if params.Cabins != "" {
					fmt.Fprintf(cmd.OutOrStdout(), " cabins=%s", params.Cabins)
				} else if params.Cabin != "" {
					fmt.Fprintf(cmd.OutOrStdout(), " cabin=%s", params.Cabin)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\nurl: %s\n(dry run - no request sent)\n", u)
				return nil
			}

			if !c.HasAPIKey() {
				return fmt.Errorf("seats.aero partner API key not found\nhint: export SEATS_AERO_API_KEY=\"your-seats-aero-pro-key\"\n      (Pro users can generate one at seats.aero settings; cached search is Pro-eligible)")
			}

			ctx := cmd.Context()
			if cmd.Flags().Changed("timeout") && flags.timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, flags.timeout)
				defer cancel()
			}

			result, err := c.Search(ctx, params)
			if err != nil {
				return err
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				result.APIKeyUsed = true // provenance: a live (Seats.aero cached) award search ran
				bts, _ := json.MarshalIndent(result, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(bts))
				return nil
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "%d award options for %s -> %s (programs merged, cached)\n",
				result.Count, origin, dest)
			fmt.Fprintf(cmd.OutOrStdout(), "Source: %s (Seats.aero cached availability)\n", result.SourceURL)
			if result.HasMore {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: more results available (use --json or increase --take)\n")
			}

			if result.Count > 0 {
				tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "DATE	PROGRAM	CABINS	ECON	PREM	BIZ	FIRST	NONSTOP")
				limit := 20
				for i, e := range result.Data {
					if i >= limit {
						fmt.Fprintf(cmd.ErrOrStderr(), "... and %d more (use --json for full list)\n", len(result.Data)-limit)
						break
					}
					fmt.Fprintf(tw, "%s	%s	%s	%s	%s	%s	%s	%s\n",
						e.Date, e.Route.Source, cabinsLabel(e), milesOrDash(e.YMileage), milesOrDash(e.WMileage), milesOrDash(e.JMileage), milesOrDash(e.FMileage), yesNo(e.AnyDirect()))
				}
				tw.Flush()
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&startDate, "from", "", "Earliest departure date (YYYY-MM-DD, inclusive)")
	cmd.Flags().StringVar(&endDate, "to", "", "Latest departure date (YYYY-MM-DD, inclusive)")
	cmd.Flags().StringVar(&cabin, "cabin", "", "Cabin filter: economy, premium, business, first (comma-separated for multiple)")
	cmd.Flags().StringVar(&orderBy, "order", "", "Order: empty (default, by departure date premium-first) or lowest_mileage")
	cmd.Flags().BoolVar(&onlyDirect, "only-direct", false, "Restrict to non-stop award availability")
	cmd.Flags().IntVar(&take, "take", 0, "Maximum results (10..1000; default 500)")
	return cmd
}

func normalizeAirportList(value, label string) (string, error) {
	tokens := strings.Split(value, ",")
	normalized := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.ToUpper(strings.TrimSpace(token))
		if len(token) != 3 {
			return "", fmt.Errorf("invalid %s %q: use a three-letter IATA code or comma-separated list", label, value)
		}
		for _, char := range token {
			if char < 'A' || char > 'Z' {
				return "", fmt.Errorf("invalid %s %q: use a three-letter IATA code or comma-separated list", label, value)
			}
		}
		normalized = append(normalized, token)
	}
	return strings.Join(normalized, ","), nil
}

func normalizeCabinList(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	tokens := strings.Split(value, ",")
	normalized := make([]string, 0, len(tokens))
	seen := make(map[string]bool, len(tokens))
	for _, token := range tokens {
		token = strings.ToLower(strings.TrimSpace(token))
		switch token {
		case "economy", "premium", "business", "first":
		default:
			return "", fmt.Errorf("invalid --cabin %q: use economy, premium, business, or first (comma-separated for multiple)", value)
		}
		if !seen[token] {
			normalized = append(normalized, token)
			seen[token] = true
		}
	}
	return strings.Join(normalized, ","), nil
}

func cabinsLabel(e seatsaero.AvailabilityEntry) string {
	var c []string
	if e.YAvailable {
		c = append(c, "econ")
	}
	if e.WAvailable {
		c = append(c, "prem")
	}
	if e.JAvailable {
		c = append(c, "biz")
	}
	if e.FAvailable {
		c = append(c, "first")
	}
	if len(c) == 0 {
		return "-"
	}
	return strings.Join(c, ",")
}

func milesOrDash(s string) string {
	if s == "" || s == "0" || strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func yesNo(b bool) string {
	if b {
		return "Y"
	}
	return "N"
}
