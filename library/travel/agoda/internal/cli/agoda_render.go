// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// printAgodaJSON emits the agent envelope with meta.source set to the payload's
// real provenance. printJSONFiltered defaults meta.source to "local", which on
// a freshly fetched Agoda search tells an agent the prices came from cache when
// they are live off the wire. Pass "live" for network results, "computed" for
// values derived from stored history, "local" for pure cache reads.
func printAgodaJSON(w io.Writer, v any, flags *rootFlags, source string) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if flags != nil && flags.csv {
		// Every Agoda result type is an object wrapping a .results array plus
		// query echo fields. The shared CSV writer only renders a top-level
		// array, so the envelope fell through it and printed raw JSON. Hand it
		// the row array; the echo fields are not tabular anyway.
		if rows, ok := agodaCSVRows(raw); ok {
			raw = rows
		}
	}
	return printOutputWithFlagsMeta(w, json.RawMessage(raw), flags, map[string]any{"source": source})
}

// agodaCSVRows extracts the .results array from a result envelope so --csv has
// rows to render. Returns false when the payload has no array under "results",
// leaving the caller's original bytes untouched.
func agodaCSVRows(raw json.RawMessage) (json.RawMessage, bool) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, false
	}
	rows, ok := envelope["results"]
	if !ok {
		return nil, false
	}
	var probe []json.RawMessage
	if err := json.Unmarshal(rows, &probe); err != nil {
		return nil, false
	}
	return rows, true
}

// bindSearchFlags registers the destination-and-dates flag set once so every
// search-shaped command exposes identical names and defaults.
func bindSearchFlags(cmd *cobra.Command, sf *searchFlags) {
	cmd.Flags().IntVar(&sf.cityID, "city-id", 0,
		"Agoda numeric city id; skips destination lookup (see 'destinations')")
	cmd.Flags().StringVar(&sf.checkin, "checkin", "",
		"Check-in date as YYYY-MM-DD (default: 30 days from today)")
	cmd.Flags().IntVar(&sf.nights, "nights", defaultNights, "Number of nights to stay")
	cmd.Flags().IntVar(&sf.rooms, "rooms", defaultRooms, "Number of rooms")
	cmd.Flags().IntVar(&sf.adults, "adults", defaultAdults, "Number of adult guests")
	cmd.Flags().IntVar(&sf.children, "children", 0, "Number of child guests")
	cmd.Flags().StringVar(&sf.currency, "currency", defaultCurrency,
		"ISO currency code for all prices, e.g. USD, EUR, JPY, SGD")
	cmd.Flags().IntVar(&sf.limit, "limit", defaultLimit, "Maximum properties to return")
	cmd.Flags().StringVar(&sf.sort, "sort", "ranking",
		"Result order: ranking, price-asc, price-desc, distance, or true-price (sorts locally by all-in cost)")
}

// renderPropertyTable prints the human view. The advertised and all-in prices sit
// next to each other on purpose: the gap is the point of the tool.
func renderPropertyTable(cmd *cobra.Command, res agodaSearchResult) error {
	out := cmd.OutOrStdout()
	if len(res.Results) == 0 {
		if res.Note != "" {
			fmt.Fprintf(out, "No priced hotels found for %s on %s (%d nights).\n%s\n",
				res.Destination, res.CheckIn, res.Nights, res.Note)
			return nil
		}
		fmt.Fprintf(out, "No priced hotels found for %s on %s (%d nights).\n",
			res.Destination, res.CheckIn, res.Nights)
		return nil
	}
	fmt.Fprintf(out, "%s (city %d) - check-in %s, %d night(s), prices in %s\n\n",
		res.Destination, res.CityID, res.CheckIn, res.Nights, res.Currency)

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROPERTY\tADVERTISED\tALL-IN\tHIDDEN\tSCORE")
	for _, p := range res.Results {
		name := p.Name
		if len(name) > 42 {
			name = name[:39] + "..."
		}
		score := "-"
		if p.ReviewScore > 0 {
			score = fmt.Sprintf("%.1f", p.ReviewScore)
		}
		fmt.Fprintf(w, "%s\t%.2f\t%.2f\t%+.1f%%\t%s\n",
			name, p.PriceAdvertised, p.PriceAllIn, p.HiddenPct, score)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(out, "\n%d of %d properties shown. ALL-IN is what you will actually pay.\n",
		res.ReturnedProperties, res.ScannedProperties)
	if res.Note != "" {
		fmt.Fprintf(out, "%s\n", res.Note)
	}
	return nil
}
