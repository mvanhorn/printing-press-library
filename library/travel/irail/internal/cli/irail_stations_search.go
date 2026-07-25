// Copyright 2026 Olivier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored command: stations search.
//
// Resolves a station across Dutch, French, German and English names plus
// historic telegraph codes and TAF/TAP codes. clirail accepts telegraph codes
// and commandtrein does fuzzy names; this does both, offline, and can also
// reverse a code back to a name.
//
// pp:novel-static-reference

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/irail/internal/irailref"
)

type stationHit struct {
	Name          string  `json:"name"`
	ID            string  `json:"id,omitempty"`
	TelegraphCode string  `json:"telegraph_code,omitempty"`
	TafTapCode    string  `json:"taf_tap_code,omitempty"`
	Country       string  `json:"country,omitempty"`
	NameNL        string  `json:"name_nl,omitempty"`
	NameFR        string  `json:"name_fr,omitempty"`
	NameDE        string  `json:"name_de,omitempty"`
	NameEN        string  `json:"name_en,omitempty"`
	Latitude      float64 `json:"latitude,omitempty"`
	Longitude     float64 `json:"longitude,omitempty"`
	AvgStopTimes  float64 `json:"avg_stop_times,omitempty"`
	TransferSec   int     `json:"official_transfer_seconds,omitempty"`
}

type stationSearchView struct {
	Query   string       `json:"query"`
	Exact   *stationHit  `json:"exact_match,omitempty"`
	Results []stationHit `json:"results"`
	Note    string       `json:"note,omitempty"`
}

func newIrailStationsSearchCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var flagBelgianOnly bool

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Find a station by name, alias or telegraph code, offline",
		Long: "Searches every station across Dutch, French, German and English names plus\n" +
			"historic telegraph codes (FBMZ, FGSP) and TAF/TAP codes.\n\n" +
			"Runs entirely against the bundled dataset, so it costs no API request and\n" +
			"works offline. An exact alias or code match is reported separately from\n" +
			"partial matches.",
		Example: `  irail-pp-cli stations search gent
  irail-pp-cli stations search FBMZ
  irail-pp-cli stations search brussel --agent --select results.name,results.telegraph_code`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search the bundled station dataset")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query is required, e.g. 'stations search gent'"))
			}
			query := args[0]

			view := stationSearchView{Query: query, Results: make([]stationHit, 0)}
			if st, ok := irailref.Lookup(query); ok {
				hit := stationHitFrom(st)
				view.Exact = &hit
			}
			for _, st := range irailref.Search(query, flagLimit) {
				if flagBelgianOnly && st.Country != "be" {
					continue
				}
				view.Results = append(view.Results, stationHitFrom(st))
			}
			if len(view.Results) == 0 && view.Exact == nil {
				view.Note = fmt.Sprintf("no station matches %q; try a shorter prefix", query)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			if view.Exact != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "exact: %s  [%s]\n\n", view.Exact.Name, view.Exact.TelegraphCode)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-38s %-8s %-8s %s\n", "STATION", "CODE", "COUNTRY", "STOPS/DAY")
			for _, r := range view.Results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-38s %-8s %-8s %9.0f\n",
					truncate(r.Name, 38), r.TelegraphCode, r.Country, r.AvgStopTimes)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&flagLimit, "limit", 15, "Maximum results to return")
	cmd.Flags().BoolVar(&flagBelgianOnly, "belgian-only", false, "Exclude cross-border stations")
	return cmd
}

func stationHitFrom(st *irailref.Station) stationHit {
	h := stationHit{
		Name:          st.Name,
		ID:            st.ID,
		TelegraphCode: st.Telegraph,
		TafTapCode:    st.TafTap,
		Country:       st.Country,
		NameNL:        st.NameNL,
		NameFR:        st.NameFR,
		NameDE:        st.NameDE,
		NameEN:        st.NameEN,
		Latitude:      st.Latitude,
		Longitude:     st.Longitude,
		AvgStopTimes:  st.AvgStopTimes,
	}
	if st.HasTransfer {
		h.TransferSec = st.TransferSeconds
	}
	return h
}
