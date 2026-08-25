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

Seats.aero's cached search is available to Pro users with a Partner API key; set
it as SEATS_AERO_API_KEY (the same env var the standalone seats-aero skill uses).
Live search is commercial-only and is not exposed here. This command is
read-only — it never books or mutates anything.

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
			origin := strings.ToUpper(strings.TrimSpace(args[0]))
			dest := strings.ToUpper(strings.TrimSpace(args[1]))
			if origin == "" || dest == "" {
				return usageErr(fmt.Errorf("origin and destination must be non-empty IATA codes"))
			}

			c := seatsaero.NewClient("")

			params := seatsaero.SearchParams{
				OriginAirport:      origin,
				DestinationAirport: dest,
				StartDate:          startDate,
				EndDate:            endDate,
				Cabin:              cabin,
				OrderBy:            orderBy,
				OnlyDirectFlights:  onlyDirect,
				Take:               take,
			}

			// Normalize ordering aliases to the API's allowed enum, which is only
			// "" (default: by date, premium-first) or "lowest_mileage". Accept
			// "cheapest" as an alias for lowest_mileage so a user asking for the
			// cheapest award isn't sent an out-of-enum value (review finding).
			switch strings.ToLower(strings.TrimSpace(orderBy)) {
			case "", "date", "default":
				params.OrderBy = ""
			case "cheapest", "lowest", "lowest_mileage", "miles":
				params.OrderBy = "lowest_mileage"
			default:
				return usageErr(fmt.Errorf("invalid --order %q: use lowest_mileage (or cheapest) for lowest award cost, or leave empty for default date ordering", orderBy))
			}

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
				if cabin != "" {
					u += "&cabin=" + cabin
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
				if cabin != "" {
					fmt.Fprintf(cmd.OutOrStdout(), " cabin=%s", cabin)
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

			fmt.Fprintf(cmd.ErrOrStderr(), "%d award options for %s -> %s (programs merged, source: seats.aero cached)\n",
				result.Count, origin, dest)
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
	cmd.Flags().StringVar(&cabin, "cabin", "", "Cabin filter: economy, premium, business, first")
	cmd.Flags().StringVar(&orderBy, "order", "", "Order: empty (default, by departure date premium-first) or lowest_mileage")
	cmd.Flags().BoolVar(&onlyDirect, "only-direct", false, "Restrict to non-stop award availability")
	cmd.Flags().IntVar(&take, "take", 0, "Maximum results (10..1000; default 500)")
	return cmd
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
