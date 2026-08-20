package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/roadside-america/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/roadside-america/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/roadside-america/internal/roadside"
	"github.com/spf13/cobra"
)

// classifyFetchErr maps transport/HTTP failures to the CLI's typed exit codes.
// 429 -> rate-limit (7), 404 -> not-found (3), other 4xx/5xx and transport
// errors -> api error (5). Never returns empty results on a throttle.
func classifyFetchErr(err error) error {
	if err == nil {
		return nil
	}
	var rl *cliutil.RateLimitError
	if errors.As(err, &rl) {
		return rateLimitErr(err)
	}
	var ae *client.APIError
	if errors.As(err, &ae) {
		switch {
		case ae.StatusCode == 429:
			return rateLimitErr(err)
		case ae.StatusCode == 404:
			return notFoundErr(err)
		}
	}
	return apiErr(err)
}

func newNearCmd(flags *rootFlags) *cobra.Command {
	var radius int
	var limit int
	cmd := &cobra.Command{
		Use:   "near <place-or-lat,lng>",
		Short: "Find offbeat attractions near a place or coordinates",
		Long: strings.Trim(`
Find offbeat roadside attractions near a place name or coordinates.

Place names are geocoded with the keyless OpenStreetMap (Nominatim) service and
cached locally; coordinates (lat,lng) skip geocoding entirely. Results are
distance-sorted and filtered to --radius miles, using RoadsideAmerica.com's own
"X mi. away" distances. Data is community-sourced and cached on read.`, "\n"),
		Example: strings.Trim(`
  roadside-america-pp-cli near "Austin, TX" --radius 20
  roadside-america-pp-cli near 30.27,-97.74 --radius 25 --json
  roadside-america-pp-cli near "Cawker City, KS" --agent --select name,city,distance,source_url`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search for attractions near the given location")
				return nil
			}
			if cliutil.IsVerifyEnv() {
				// Hermetic: skip live geocoding (Nominatim) under verify.
				return emitAttractions(cmd, flags, attractionListView{Query: map[string]any{"verify": true}})
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a place name or lat,lng is required"))
			}
			if flags.dataSource == "local" {
				return usageErr(fmt.Errorf("near requires live data (coordinates are not cached); use 'state <ST>' for offline browsing"))
			}
			if radius <= 0 {
				radius = 25
			}
			if limit <= 0 {
				limit = 25
			}
			ctx := cmd.Context()
			s, err := openRoadsideStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()

			input := strings.Join(args, " ")
			lat, lng, label, err := resolveLocation(ctx, s, input, flags.timeout)
			if err != nil {
				if errors.Is(err, roadside.ErrPlaceNotFound) {
					return notFoundErr(fmt.Errorf("could not geocode %q; pass coordinates as lat,lng instead", input))
				}
				return classifyFetchErr(err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			atts, err := fetchNearbyAttractions(ctx, c, lat, lng, roadside.MilesToDelta(float64(radius)))
			if err != nil {
				return classifyFetchErr(err)
			}
			cacheAttractions(s, atts)

			out := make([]roadside.Attraction, 0, len(atts))
			for _, a := range atts {
				// DistanceMi == 0 means the site's distance label was unparseable;
				// keep it — nearbyAttractions.php already geo-bounds results to the
				// requested delta (~radius), so an unlabeled entry is still nearby.
				if a.DistanceMi > 0 && a.DistanceMi > float64(radius) {
					continue
				}
				out = append(out, a)
				if len(out) >= limit {
					break
				}
			}

			view := attractionListView{
				Query:       map[string]any{"place": label, "lat": lat, "lng": lng, "radius_mi": radius},
				Attractions: out,
			}
			if len(out) == 0 {
				view.Note = fmt.Sprintf("No attractions found within %d mi of %s. Try a larger --radius.", radius, label)
			}
			return emitAttractions(cmd, flags, view)
		},
	}
	cmd.Flags().IntVar(&radius, "radius", 25, "Search radius in miles")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum attractions to return")
	return cmd
}
