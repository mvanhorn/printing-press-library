// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// PATCH(amend-2026-05-19: award-search support) — added by /printing-press-amend.
// Thin alias over `flights search` that defaults --award=true so an agent can
// say "alaska-airlines-pp-cli flights award-search ..." without having to
// remember the --award flag on the cash command. Same underlying SvelteKit
// __data.json endpoint, same Cookie auth, same response shape.
//
// Live sniff date: 2026-05-19. Endpoint discovery + ShoppingMethod toggle
// captured via claude-in-chrome MCP against alaskaair.com (user logged in).
// See .printing-press-patches.json entry "award-search-toggle".

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newFlightsAwardSearchCmd(flags *rootFlags) *cobra.Command {
	var flagO string
	var flagD string
	var flagOD string
	var flagDD string
	var flagA string
	var flagC string
	var flagL string
	var flagRT string
	var flagCabin string
	var flagLocale string

	cmd := &cobra.Command{
		Use:         "award-search",
		Short:       "Search award (miles + cash) fares between two airports. Defaults --award=true; same endpoint as 'flights search' with ShoppingMethod=onlineaward.",
		Example:     "  alaska-airlines-pp-cli flights award-search --origin SFO --destination HND --depart 2026-08-15 --return 2026-08-22 --json",
		Annotations: map[string]string{"pp:endpoint": "flights.award_search", "pp:method": "GET", "pp:path": "/search/results/__data.json", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("origin") && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "origin")
			}
			if !cmd.Flags().Changed("destination") && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "destination")
			}
			if !cmd.Flags().Changed("depart") && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "depart")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/search/results/__data.json"
			params := buildAwardSearchParams(awardSearchInput{
				Origin:      flagO,
				Destination: flagD,
				Depart:      flagOD,
				Return:      flagDD,
				Adults:      flagA,
				Children:    flagC,
				LapInfants:  flagL,
				RoundTrip:   flagRT,
				Cabin:       flagCabin,
				Locale:      flagLocale,
			})

			data, prov, err := resolveRead(cmd.Context(), c, flags, "flights", false, path, params, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			{
				var countItems []json.RawMessage
				_ = json.Unmarshal(data, &countItems)
				printProvenance(cmd, len(countItems), prov)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						return err
					}
					if len(items) >= 25 {
						fmt.Fprintf(os.Stderr, "\nShowing %d results. To narrow: add --limit, --json --select, or filter flags.\n", len(items))
					}
					return nil
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&flagO, "origin", "", "Origin IATA code (e.g. SFO)")
	cmd.Flags().StringVar(&flagD, "destination", "", "Destination IATA code (e.g. HND)")
	cmd.Flags().StringVar(&flagOD, "depart", "", "Outbound date YYYY-MM-DD")
	cmd.Flags().StringVar(&flagDD, "return", "", "Return date YYYY-MM-DD (omit for one-way)")
	cmd.Flags().StringVar(&flagA, "adults", "1", "Adult passenger count")
	cmd.Flags().StringVar(&flagC, "children", "0", "Child passenger count")
	cmd.Flags().StringVar(&flagL, "lap-infants", "0", "Lap-infant count")
	cmd.Flags().StringVar(&flagRT, "round-trip", "true", "true for round-trip, false for one-way")
	cmd.Flags().StringVar(&flagCabin, "cabin", "", "Preferred cabin (economy, premium, business, first); empty for any")
	cmd.Flags().StringVar(&flagLocale, "locale", "en-us", "Locale string")

	return cmd
}

// awardSearchInput carries the fields that buildAwardSearchParams turns into
// the SvelteKit query-string. Kept as a struct so the same builder is reusable
// from flights_award_cheapest.go's parallel fan-out.
type awardSearchInput struct {
	Origin      string
	Destination string
	Depart      string
	Return      string
	Adults      string
	Children    string
	LapInfants  string
	RoundTrip   string
	Cabin       string
	Locale      string
}

// buildAwardSearchParams composes the query-param map for an award search.
// ShoppingMethod=onlineaward + UPG=none + OT/DT=Anytime are the minimum
// delta captured from the live alaskaair.com browser sniff. The cabin
// param maps to the existing site's SpecFare slot (best-effort; the site
// accepts unset to mean "any").
func buildAwardSearchParams(in awardSearchInput) map[string]string {
	p := map[string]string{
		"ShoppingMethod": "onlineaward",
		"UPG":            "none",
		"OT":             "Anytime",
		"DT":             "Anytime",
	}
	if in.Origin != "" {
		p["O"] = in.Origin
	}
	if in.Destination != "" {
		p["D"] = in.Destination
	}
	if in.Depart != "" {
		p["OD"] = in.Depart
	}
	if in.Return != "" {
		p["DD"] = in.Return
	}
	if in.Adults != "" {
		p["A"] = in.Adults
	}
	if in.Children != "" {
		p["C"] = in.Children
	}
	if in.LapInfants != "" {
		p["L"] = in.LapInfants
	}
	if in.RoundTrip != "" {
		p["RT"] = in.RoundTrip
	}
	if in.Cabin != "" {
		p["SpecFare"] = in.Cabin
	}
	if in.Locale != "" {
		p["locale"] = in.Locale
	}
	return p
}
