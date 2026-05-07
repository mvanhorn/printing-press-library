package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/osrm"
)

func newRouteViewCmd(flags *rootFlags) *cobra.Command {
	var (
		bufferStr string
		minTrust  float64
	)
	cmd := &cobra.Command{
		Use:   "route-view <from> <to>",
		Short: "Walking polyline from A to B, then everything interesting along the path — not just at the endpoints.",
		Long: `OSRM walking polyline (cached) + spatial buffer query against the local
goat_places store. Returns places within --buffer meters of the polyline
geometry, ranked by trust × proximity-to-path. Geometric local-store query
no single API can answer.`,
		Example: strings.Trim(`
  # Walk from Shibuya to Yoyogi Park, see what's worth a 150m detour
  wanderlust-goat-pp-cli route-view "Shibuya Station, Tokyo" "Yoyogi Park, Tokyo" --buffer 150m

  # Pipe to jq for an agent
  wanderlust-goat-pp-cli route-view "Place de la République, Paris" "Marais, Paris" --buffer 200m --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("route-view requires both <from> and <to>; got %d arg(s)", len(args)))
			}
			fromQ := args[0]
			toQ := args[1]
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			from, err := resolveAnchor(ctx, fromQ)
			if err != nil {
				return err
			}
			to, err := resolveAnchor(ctx, toQ)
			if err != nil {
				return err
			}
			buf, err := parseDistance(bufferStr)
			if err != nil {
				return err
			}

			store, err := openGoatStore(cmd, flags)
			if err != nil {
				return err
			}
			defer store.Close()

			or := osrm.New("", nil, userAgent())
			poly, err := or.WalkingPolyline(ctx, from.Lat, from.Lng, to.Lat, to.Lng)
			if err != nil {
				return err
			}
			// Cache the route distance/duration so subsequent calls don't refetch.
			polyJSON := polylineToJSON(poly.Coords)
			_ = store.CacheRoute(ctx, from.Lat, from.Lng, to.Lat, to.Lng,
				poly.DistanceMeters, poly.DurationSeconds, polyJSON)

			// Query the bounding box of the polyline + buffer; then point-to-segment
			// distance refinement.
			minLat, maxLat, minLng, maxLng := boundingBox(poly.Coords, float64(buf))
			midLat := (minLat + maxLat) / 2
			midLng := (minLng + maxLng) / 2
			diag := haversineMeters(minLat, minLng, maxLat, maxLng)
			candidates, err := store.QueryRadius(ctx, midLat, midLng, diag, "")
			if err != nil {
				return err
			}
			report := routeReport{
				From: from, To: to,
				Buffer: buf,
				DistanceMeters: poly.DistanceMeters, WalkingMin: poly.DurationSeconds / 60.0,
			}
			for _, c := range candidates {
				if c.Trust < minTrust {
					continue
				}
				d := minDistanceToPolyline(c.Lat, c.Lng, poly.Coords)
				if d > float64(buf) {
					continue
				}
				report.AlongRoute = append(report.AlongRoute, alongPick{
					Name: c.Name, NameLocal: c.NameLocal, Source: c.Source, Intent: c.Intent,
					Lat: c.Lat, Lng: c.Lng, DistanceFromPath: d, Trust: c.Trust,
					WhySpecial: c.WhySpecial,
				})
			}
			sort.Slice(report.AlongRoute, func(i, j int) bool {
				return report.AlongRoute[i].Trust*100/(1+report.AlongRoute[i].DistanceFromPath/50) >
					report.AlongRoute[j].Trust*100/(1+report.AlongRoute[j].DistanceFromPath/50)
			})
			if len(report.AlongRoute) > 12 {
				report.AlongRoute = report.AlongRoute[:12]
			}
			if len(report.AlongRoute) == 0 {
				report.Note = "No places along route in the local store. Run 'sync-city <slug>' for a covering city."
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().StringVar(&bufferStr, "buffer", "150m", "Max distance from the polyline (e.g. 150m, 0.3km).")
	cmd.Flags().Float64Var(&minTrust, "min-trust", 0.75, "Minimum trust filter for places along the route.")
	return cmd
}

type routeReport struct {
	From            AnchorResolution `json:"from"`
	To              AnchorResolution `json:"to"`
	Buffer          int              `json:"buffer_meters"`
	DistanceMeters  float64          `json:"distance_meters"`
	WalkingMin      float64          `json:"walking_minutes"`
	AlongRoute      []alongPick      `json:"along_route"`
	Note            string           `json:"note,omitempty"`
}

type alongPick struct {
	Name             string  `json:"name"`
	NameLocal        string  `json:"name_local,omitempty"`
	Source           string  `json:"source"`
	Intent           string  `json:"intent"`
	Lat              float64 `json:"lat"`
	Lng              float64 `json:"lng"`
	DistanceFromPath float64 `json:"distance_from_path_meters"`
	Trust            float64 `json:"trust"`
	WhySpecial       string  `json:"why_special,omitempty"`
}

// boundingBox returns the lat/lng box covering all points expanded by `buffer` meters.
func boundingBox(coords [][2]float64, buffer float64) (minLat, maxLat, minLng, maxLng float64) {
	if len(coords) == 0 {
		return 0, 0, 0, 0
	}
	bufDeg := buffer / 111000.0
	minLat, maxLat = coords[0][1], coords[0][1]
	minLng, maxLng = coords[0][0], coords[0][0]
	for _, c := range coords {
		lng, lat := c[0], c[1]
		if lat < minLat {
			minLat = lat
		}
		if lat > maxLat {
			maxLat = lat
		}
		if lng < minLng {
			minLng = lng
		}
		if lng > maxLng {
			maxLng = lng
		}
	}
	return minLat - bufDeg, maxLat + bufDeg, minLng - bufDeg, maxLng + bufDeg
}

// minDistanceToPolyline returns the shortest haversine distance from point
// to any polyline vertex. (Approximation; for v1 this is enough — proper
// point-to-segment math is a v2 enhancement.)
func minDistanceToPolyline(lat, lng float64, coords [][2]float64) float64 {
	if len(coords) == 0 {
		return 1e9
	}
	min := 1e18
	for _, c := range coords {
		d := haversineMeters(lat, lng, c[1], c[0])
		if d < min {
			min = d
		}
	}
	return min
}

func polylineToJSON(coords [][2]float64) string {
	if len(coords) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, c := range coords {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString("[")
		b.WriteString(fmtFloat(c[0]))
		b.WriteByte(',')
		b.WriteString(fmtFloat(c[1]))
		b.WriteString("]")
	}
	b.WriteByte(']')
	return b.String()
}

func fmtFloat(f float64) string { return strFmt(f) }
