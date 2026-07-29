// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live
//
// `route` geocodes both endpoints through Nominatim, derives a corridor
// bounding box, and queries Overpass at call time. Same reasoning as `near`:
// the query space is every pair of places crossed with every subject type, so
// nothing meaningful can be synced in advance.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/overpass/internal/subjects"

	"github.com/spf13/cobra"
)

func newNovelRouteCmd(flags *rootFlags) *cobra.Command {
	var (
		from     string
		to       string
		country  string
		typ      string
		corridor string
		limit    int
		timeout  int
	)
	cmd := &cobra.Command{
		Use:   "route",
		Short: "Finds subjects inside a corridor between two places, for planning what to stop for",
		Long: strings.Trim(`
What is worth stopping for on the way.

Geocodes both ends, builds a bounding box around the straight line between
them, and searches inside it. Results are ordered by how far along the route
they sit, so the list reads in driving order.

One honest limitation: this is a rectangle around the straight line, not a
buffer around the actual road. On a long diagonal it sweeps in area well off
any route you would really drive, and the reported off-line distance is how to
judge that — check it before committing to a detour.
`, "\n"),
		Example: strings.Trim(`
  overpass-pp-cli route --from "Los Angeles" --to "Palm Springs" --type water_tower --corridor 12km
  overpass-pp-cli route --from "Los Angeles" --to "Salton Sea" --type ruins --corridor 20km --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if from == "" || to == "" {
				return usageErr(fmt.Errorf("give both ends of the drive: --from and --to"))
			}
			ty, err := subjects.Lookup(typ)
			if err != nil {
				return usageErr(err)
			}
			padM, err := subjects.ParseDistance(corridor)
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			start, err := geocode(ctx, from, country, flags.timeout)
			if err != nil {
				return err
			}
			end, err := geocode(ctx, to, country, flags.timeout)
			if err != nil {
				return err
			}
			if !flags.asJSON && !flags.quiet {
				fmt.Fprintf(cmd.ErrOrStderr(), "resolved %q to %s\nresolved %q to %s\n",
					from, start.Label, to, end.Label)
			}

			boxes := subjects.CorridorBBox(start.Latitude, start.Longitude,
				end.Latitude, end.Longitude, padM/1000)
			area := subjects.Area{BBoxes: boxes}
			query, err := subjects.BuildQuery(ty, area, timeout, limit)
			if err != nil {
				return usageErr(err)
			}

			subs, attempts, remark, err := runQuery(ctx, cmd, flags, query, nil)
			if err != nil {
				return err
			}

			// Order by progress along the route, and report how far each sits
			// off the straight line so an unrealistic detour is visible.
			type stop struct {
				subjects.Subject
				AlongKM   float64 `json:"along_km"`
				OffLineKM float64 `json:"off_line_km"`
			}
			total := subjects.HaversineKM(start.Latitude, start.Longitude, end.Latitude, end.Longitude)
			stops := make([]stop, 0, len(subs))
			for _, s := range subs {
				dStart := subjects.HaversineKM(start.Latitude, start.Longitude, s.Latitude, s.Longitude)
				dEnd := subjects.HaversineKM(s.Latitude, s.Longitude, end.Latitude, end.Longitude)
				// Excess path length over the direct route is a simple, honest
				// proxy for how far off the line a point sits.
				off := (dStart + dEnd - total) / 2
				if off < 0 {
					off = 0
				}
				stops = append(stops, stop{Subject: s, AlongKM: dStart, OffLineKM: off})
			}
			sort.SliceStable(stops, func(i, j int) bool { return stops[i].AlongKM < stops[j].AlongKM })

			if flags.asJSON {
				// The truncation remark rides inside the document. Printed
				// ahead of it, as prose, it would make stdout unparseable.
				payload := map[string]any{
					"type": ty.Name, "from": start, "to": end,
					"route_km": total, "corridor_m": padM, "bboxes": boxes,
					"stops": stops, "mirror_attempts": attempts,
					"caveat":  "corridor is a bounding box around the straight line, not a buffer around the road",
					"partial": remark != "",
				}
				if remark != "" {
					payload["partial_remark"] = remark
				}
				return flags.printJSONLive(cmd, payload)
			}

			out := cmd.OutOrStdout()
			if remark != "" {
				fmt.Fprintln(out, partialNote(remark))
			}
			fmt.Fprintln(out, bold(fmt.Sprintf("%d %s along the %.0f km between %s and %s",
				len(stops), pluralizeType(ty.Name, len(stops)), total, from, to)))
			if len(stops) == 0 {
				fmt.Fprintln(out, "  nothing found in the corridor; widen --corridor or try another type")
				return nil
			}
			rows := make([][]string, 0, len(stops))
			for _, s := range stops {
				rows = append(rows, []string{
					fmt.Sprintf("%.0f km", s.AlongKM), s.Name,
					fmt.Sprintf("%.0f km", s.OffLineKM),
					fmt.Sprintf("%.5f,%.5f", s.Latitude, s.Longitude), s.URL,
				})
			}
			if err := flags.printTable(cmd, []string{"ALONG", "NAME", "OFF LINE", "COORDS", "OSM"}, rows); err != nil {
				return err
			}
			fmt.Fprintln(out, "\nOFF LINE is distance from the straight line between the two points, not from the road.")
			if ty.Note != "" {
				fmt.Fprintf(out, "note: %s\n", ty.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Where the drive starts, e.g. \"Los Angeles\"")
	cmd.Flags().StringVar(&to, "to", "", "Where the drive ends, e.g. \"Palm Springs\"")
	cmd.Flags().StringVar(&country, "country", "", "Restrict geocoding to ISO-3166 alpha-2 codes, e.g. us")
	cmd.Flags().StringVar(&typ, "type", "water_tower", typeFlagHelp)
	cmd.Flags().StringVar(&corridor, "corridor", "15km", "How far either side of the line to search, e.g. 12km, 10mi")
	cmd.Flags().IntVar(&limit, "limit", 60, "Maximum results to return (0 for no limit)")
	cmd.Flags().IntVar(&timeout, "query-timeout", 40, "Overpass server-side timeout in seconds")
	return cmd
}
