// Copyright 2026 Olivier and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: disruptions route.
//
// The disturbances endpoint is a flat national feed. This resolves the stations
// a journey actually passes through and matches them against that feed, so the
// caller sees only what affects their trip.
//
// Matching is deliberately mechanical (station-name containment over folded
// text), not an LLM summarisation, so results are reproducible and testable.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/irail/internal/irailref"
)

type routeDisruption struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Type            string   `json:"type"`
	Description     string   `json:"description,omitempty"`
	Link            string   `json:"link,omitempty"`
	Timestamp       string   `json:"timestamp,omitempty"`
	MatchedStations []string `json:"matched_stations"`
}

type disruptionsRouteView struct {
	From          string            `json:"from"`
	To            string            `json:"to"`
	RouteStations []string          `json:"route_stations"`
	Scanned       int               `json:"scanned_disruptions"`
	Disruptions   []routeDisruption `json:"disruptions"`
	PlannedWorks  []routeDisruption `json:"planned_works"`
	Note          string            `json:"note,omitempty"`
}

func newNovelDisruptionsRouteCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagTo string
	var flagDate string
	var flagTime string
	var flagIncludePlanned bool

	cmd := &cobra.Command{
		Use:   "route",
		Short: "Disruptions affecting one journey, not the whole network",
		Long: "Resolves the stations a journey passes through, then filters the national\n" +
			"disruption feed to entries naming those stations.\n\n" +
			"Use this command to see whether one specific trip is affected. Do NOT use it\n" +
			"for the whole network; use 'disruptions' for that.\n\n" +
			"Real disruptions and planned engineering works are reported separately, since\n" +
			"the national feed is dominated by planned works.",
		Example: `  irail-pp-cli disruptions route --from Ghent-Sint-Pieters --to Brussels-Central
  irail-pp-cli disruptions route --from Bruges --to Leuven --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would filter the national disruption feed to this journey")
				return nil
			}
			if flagFrom == "" || flagTo == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from and --to are both required"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			from := resolveStationName(flagFrom)
			to := resolveStationName(flagTo)

			date, hhmm, _, err := resolveWhen(flagDate, flagTime, nowInBelgium())
			if err != nil {
				return usageErr(err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			connParams := map[string]string{"from": from, "to": to, "lang": "en"}
			if date != "" {
				connParams["date"] = date
			}
			if hhmm != "" {
				connParams["time"] = hhmm
			}
			connEnv, err := irailFetch(ctx, c, "/v1/connections?format=json", connParams)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			stations := routeStationNames(connEnv, from, to)

			distEnv, err := irailFetch(ctx, c, "/v1/disturbances?format=json", map[string]string{"lang": "en"})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			all := sliceAt(distEnv, "disturbance")

			view := disruptionsRouteView{
				From:          from,
				To:            to,
				RouteStations: stations,
				Scanned:       len(all),
				Disruptions:   make([]routeDisruption, 0),
				PlannedWorks:  make([]routeDisruption, 0),
			}

			for _, raw := range all {
				d, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				matched := matchDisruptionToStations(d, stations)
				if len(matched) == 0 {
					continue
				}
				entry := routeDisruption{
					ID:              irailString(d["id"]),
					Title:           irailString(d["title"]),
					Type:            irailString(d["type"]),
					Description:     irailString(d["description"]),
					Link:            irailString(d["link"]),
					Timestamp:       unixToLocal(d["timestamp"]),
					MatchedStations: matched,
				}
				if entry.Type == "planned" {
					view.PlannedWorks = append(view.PlannedWorks, entry)
					continue
				}
				view.Disruptions = append(view.Disruptions, entry)
			}

			if !flagIncludePlanned {
				view.PlannedWorks = view.PlannedWorks[:0]
			}

			switch {
			case len(stations) == 0:
				view.Note = "iRail returned no route between these stations, so no disruption could be matched"
			case len(view.Disruptions) == 0 && len(view.PlannedWorks) == 0:
				view.Note = fmt.Sprintf(
					"scanned %d national entries; none names a station on this route", view.Scanned)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Disruptions) == 0 && len(view.PlannedWorks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			for _, d := range view.Disruptions {
				fmt.Fprintf(cmd.OutOrStdout(), "[disruption] %s\n    affects: %s\n", d.Title, strings.Join(d.MatchedStations, ", "))
			}
			for _, d := range view.PlannedWorks {
				fmt.Fprintf(cmd.OutOrStdout(), "[planned]    %s\n    affects: %s\n", d.Title, strings.Join(d.MatchedStations, ", "))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d disruption(s), %d planned work(s) from %d national entries\n",
				len(view.Disruptions), len(view.PlannedWorks), view.Scanned)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagFrom, "from", "", "Origin station (name, telegraph code or id)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Destination station (name, telegraph code or id)")
	cmd.Flags().StringVar(&flagDate, "date", "", "Date: tomorrow, monday, 2026-07-25, +2d or ddmmyy")
	cmd.Flags().StringVar(&flagTime, "time", "", "Time: 08:12, 0812, now or +30m")
	cmd.Flags().BoolVar(&flagIncludePlanned, "include-planned", true,
		"Include planned engineering works as well as live disruptions")
	return cmd
}

// routeStationNames collects every station the first itinerary touches:
// origin, destination, transfer points and intermediate stops.
func routeStationNames(env map[string]any, from, to string) []string {
	// Key by resolved station identity, not by label: the API answers in the
	// requested language ("Ghent-Sint-Pieters") while the bundled dataset uses
	// its own canonical form ("Gent-Sint-Pieters"). Keying on the raw string
	// would list one physical station twice.
	seen := map[string]string{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		key := irailref.Fold(name)
		label := name
		if st, ok := irailref.Lookup(name); ok {
			key = st.URI
			label = st.Name
		}
		if _, exists := seen[key]; !exists {
			seen[key] = label
		}
	}
	add(from)
	add(to)

	for _, raw := range sliceAt(env, "connection") {
		conn, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if dep := mapAt(conn, "departure"); dep != nil {
			add(irailString(dep["station"]))
			for _, s := range sliceAt(dep, "stops", "stop") {
				if stop, ok := s.(map[string]any); ok {
					add(irailString(stop["station"]))
				}
			}
		}
		if arr := mapAt(conn, "arrival"); arr != nil {
			add(irailString(arr["station"]))
		}
		for _, v := range sliceAt(conn, "vias", "via") {
			if via, ok := v.(map[string]any); ok {
				add(irailString(via["station"]))
			}
		}
		// One itinerary is enough; later options repeat the same corridor.
		break
	}

	out := make([]string, 0, len(seen))
	for _, label := range seen {
		out = append(out, label)
	}
	sort.Strings(out)
	return out
}

// matchDisruptionToStations reports which route stations a disruption names.
//
// Station names are matched against folded title and description text. Short
// names are required to sit on a word boundary in the raw text, so "Hal" does
// not match "Halle" and "Aat" does not match inside an unrelated word.
func matchDisruptionToStations(d map[string]any, stations []string) []string {
	title := irailString(d["title"])
	desc := irailString(d["description"])
	haystack := irailref.Fold(title + " " + desc)
	rawHaystack := strings.ToLower(title + " " + desc)

	var matched []string
	for _, station := range stations {
		if stationNamedIn(station, haystack, rawHaystack) {
			matched = append(matched, station)
		}
	}
	sort.Strings(matched)
	return matched
}

// stationNamedIn tests one station, including each half of a bilingual name.
func stationNamedIn(station, folded, raw string) bool {
	candidates := []string{station}
	if strings.Contains(station, "/") {
		candidates = append(candidates, strings.Split(station, "/")...)
	}
	if st, ok := irailref.Lookup(station); ok {
		for _, alt := range []string{st.NameNL, st.NameFR, st.NameEN, st.NameDE} {
			if alt != "" {
				candidates = append(candidates, alt)
				if strings.Contains(alt, "/") {
					candidates = append(candidates, strings.Split(alt, "/")...)
				}
			}
		}
	}

	for _, cand := range candidates {
		cand = strings.TrimSpace(cand)
		if len(cand) < 3 {
			continue
		}
		foldedCand := irailref.Fold(cand)
		if foldedCand == "" {
			continue
		}
		// Short names are ambiguous once folding removes separators, so require
		// a word-boundary hit in the raw text instead.
		if len(foldedCand) <= 5 {
			if containsWord(raw, strings.ToLower(cand)) {
				return true
			}
			continue
		}
		if strings.Contains(folded, foldedCand) {
			return true
		}
	}
	return false
}

// containsWord reports whether needle appears in haystack delimited by
// non-alphanumeric characters.
func containsWord(haystack, needle string) bool {
	if needle == "" {
		return false
	}
	idx := 0
	for {
		i := strings.Index(haystack[idx:], needle)
		if i < 0 {
			return false
		}
		start := idx + i
		end := start + len(needle)
		beforeOK := start == 0 || !isAlphaNum(rune(haystack[start-1]))
		afterOK := end == len(haystack) || !isAlphaNum(rune(haystack[end]))
		if beforeOK && afterOK {
			return true
		}
		idx = start + 1
		if idx >= len(haystack) {
			return false
		}
	}
}

func isAlphaNum(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}
