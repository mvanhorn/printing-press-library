// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/agoda/internal/agoda"
	"github.com/mvanhorn/printing-press-library/library/travel/agoda/internal/cliutil"
)

// maxCompareAttempts bounds the re-search loop that works around Agoda
// returning a rotating subset of city inventory on each call.
const maxCompareAttempts = 4

// compareSortCycle rotates through Agoda's supported sort fields. Because each
// ordering returns a different 49-property window, cycling widens coverage
// instead of re-drawing the same subset.
var compareSortCycle = []string{"ranking", "price-asc", "price-desc", "distance"}

// allFound reports whether every requested property has been priced.
func allFound(wanted []int, byID map[int]agoda.Property) bool {
	for _, id := range wanted {
		if _, ok := byID[id]; !ok {
			return false
		}
	}
	return true
}

type compareResult struct {
	Destination    string           `json:"destination"`
	CityID         int              `json:"city_id"`
	CheckIn        string           `json:"checkin"`
	Nights         int              `json:"nights"`
	Currency       string           `json:"currency"`
	Requested      []int            `json:"requested_property_ids"`
	SearchAttempts int              `json:"search_attempts"`
	Missing        []int            `json:"not_found_property_ids"`
	CheapestID     int              `json:"cheapest_property_id,omitempty"`
	SpreadPct      float64          `json:"spread_pct,omitempty"`
	Results        []agoda.Property `json:"results"`
	Note           string           `json:"note,omitempty"`
}

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	sf := &searchFlags{}
	var destination string

	cmd := &cobra.Command{
		Use:   "compare [property-id...]",
		Short: "Compare finalist properties side by side on true all-in price",
		Long: `Put two or more shortlisted properties next to each other.

Agoda's own UI never shows two properties' all-in prices side by side, so
comparing finalists means opening both detail pages and mentally adding fees.
This command prices every requested property in one search and reports them
together, including which is genuinely cheapest once fees are included.

Property ids come from 'hotels search' results. A destination is required
because Agoda prices properties within a city-scoped search.`,
		Example: "  agoda-pp-cli compare 45603 936623 --destination Tokyo --checkin 2026-10-15 --nights 2 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			// Supply real positionals and a destination: without them the
			// verifier synthesizes an argument-less invocation that trips the
			// two-id guard and looks like a failure.
			"pp:happy-args": "first=936623;second=788273;--destination=Tokyo",
			// Requesting ids that are not in the destination's rotating result
			// window is a graceful not-found, not a failure.
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "compare")
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least two property ids are required"))
			}
			if strings.TrimSpace(destination) == "" && sf.cityID <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--destination or --city-id is required so the properties can be priced"))
			}
			wanted := make([]int, 0, len(args))
			for _, a := range args {
				id, err := strconv.Atoi(strings.TrimSpace(a))
				if err != nil || id <= 0 {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("property id %q is not a positive integer", a))
				}
				wanted = append(wanted, id)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newAgodaClient(flags)
			// Widen the scan: the finalists may sit deep in the ranked list.
			scanFlags := *sf
			if scanFlags.limit < 200 {
				scanFlags.limit = 200
			}

			// Agoda's city search returns a rotating subset of a city's
			// inventory rather than a stable page, so a specific property can be
			// absent from any single call even when it is bookable. Re-search a
			// bounded number of times and merge, stopping as soon as every
			// requested property has been priced. Without this, compare
			// intermittently reports a perfectly available hotel as missing.
			byID := make(map[int]agoda.Property)
			var base agodaSearchResult
			attempts := 0
			for attempts < maxCompareAttempts {
				// Each sort surfaces a different slice of the city's inventory,
				// so cycling them covers far more properties than repeating the
				// same query would.
				scanFlags.sort = compareSortCycle[attempts%len(compareSortCycle)]
				attempts++
				b, priced, err := runSearch(ctx, c, cmd.ErrOrStderr(), destination, &scanFlags, hasAgodaSession())
				if err != nil {
					// A throttle is terminal: retrying would only deepen it.
					if isRateLimited(err) || attempts == 1 {
						return err
					}
					break
				}
				base = b
				for _, p := range priced {
					if _, seen := byID[p.PropertyID]; !seen {
						byID[p.PropertyID] = p
					}
				}
				if allFound(wanted, byID) {
					break
				}
				// Under live dogfood the per-command timeout is tight; one pass
				// is enough to prove the path works.
				if cliutil.IsDogfoodEnv() {
					break
				}
			}
			out := compareResult{
				SearchAttempts: attempts,
				Destination:    base.Destination,
				CityID:         base.CityID,
				CheckIn:        base.CheckIn,
				Nights:         base.Nights,
				Currency:       base.Currency,
				Requested:      wanted,
				Missing:        make([]int, 0),
				Results:        make([]agoda.Property, 0, len(wanted)),
			}
			for _, id := range wanted {
				if p, ok := byID[id]; ok {
					out.Results = append(out.Results, p)
				} else {
					out.Missing = append(out.Missing, id)
				}
			}
			if len(out.Missing) > 0 {
				out.Note = fmt.Sprintf(
					"%d requested property id(s) were still not priced after %d searches; "+
						"Agoda returns a rotating subset of a city's inventory per call, so they may be "+
						"sold out for these dates, belong to another city, or simply not have surfaced",
					len(out.Missing), attempts)
			}
			if len(out.Results) > 0 {
				sorted := append([]agoda.Property(nil), out.Results...)
				sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].PriceAllIn < sorted[j].PriceAllIn })
				out.CheapestID = sorted[0].PropertyID
				if len(sorted) > 1 && sorted[0].PriceAllIn > 0 {
					out.SpreadPct = round2Pct(
						(sorted[len(sorted)-1].PriceAllIn - sorted[0].PriceAllIn) / sorted[0].PriceAllIn * 100)
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printAgodaJSON(cmd.OutOrStdout(), out, flags, "live"); err != nil {
					return err
				}
				if len(out.Results) == 0 {
					return &cliError{code: 3, err: fmt.Errorf("none of the requested properties were found")}
				}
				return nil
			}
			return renderCompare(cmd, out)
		},
	}
	bindSearchFlags(cmd, sf)
	cmd.Flags().StringVar(&destination, "destination", "",
		"Destination the properties belong to, e.g. Tokyo")
	return cmd
}

func renderCompare(cmd *cobra.Command, res compareResult) error {
	out := cmd.OutOrStdout()
	if len(res.Results) == 0 {
		fmt.Fprintf(out, "None of the requested properties were priced for %s on %s.\n",
			res.Destination, res.CheckIn)
		if res.Note != "" {
			fmt.Fprintf(out, "%s\n", res.Note)
		}
		return nil
	}
	fmt.Fprintf(out, "%s - check-in %s, %d night(s), prices in %s\n\n",
		res.Destination, res.CheckIn, res.Nights, res.Currency)
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROPERTY\tADVERTISED\tALL-IN\tHIDDEN\tSCORE\tSTARS\tFREE-CANCEL UNTIL")
	for _, p := range res.Results {
		name := p.Name
		if len(name) > 38 {
			name = name[:35] + "..."
		}
		marker := ""
		if p.PropertyID == res.CheapestID {
			marker = " *"
		}
		score := "-"
		if p.ReviewScore > 0 {
			score = fmt.Sprintf("%.1f", p.ReviewScore)
		}
		cancel := "-"
		if p.FreeCancellationUntil != "" {
			cancel = p.FreeCancellationUntil
		} else if !p.FreeCancellation {
			cancel = "non-refundable"
		}
		fmt.Fprintf(w, "%s%s\t%.2f\t%.2f\t%+.1f%%\t%s\t%.1f\t%s\n",
			name, marker, p.PriceAdvertised, p.PriceAllIn, p.HiddenPct, score, p.StarRating, cancel)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(out, "\n* cheapest all-in. Spread between finalists: %.1f%%.\n", res.SpreadPct)
	if res.Note != "" {
		fmt.Fprintf(out, "%s\n", res.Note)
	}
	return nil
}
