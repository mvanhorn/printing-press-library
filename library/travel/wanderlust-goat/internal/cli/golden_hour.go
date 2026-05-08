package cli

import (
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/dispatch"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sun"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/walking"
)

// newGoldenHourCmd is the pure-Go SunCalc + viewpoint pairing command.
// Ported from v1; works without API keys (no Google, no Anthropic).
func newGoldenHourCmd(flags *rootFlags) *cobra.Command {
	var (
		dateStr  string
		minutes  float64
		zoneName string
	)
	cmd := &cobra.Command{
		Use:   "golden-hour [anchor]",
		Short: "Sunrise/sunset/blue-hour locally + nearby viewpoints from the local store",
		Long: `Pure-Go sun-position math (no API) plus a walking-radius viewpoint search
from the local goatstore. Returns sunrise / sunset / civil dawn / civil dusk
plus blue-hour and golden-hour windows in the requested IANA zone.`,
		Example: strings.Trim(`
  wanderlust-goat-pp-cli golden-hour "Tokyo Tower" --date 2026-06-15 --zone Asia/Tokyo
  wanderlust-goat-pp-cli golden-hour 48.8584,2.2945 --date 2026-06-21 --minutes 30 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			anchor := strings.Join(args, " ")
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			res, err := dispatch.ResolveAnchor(ctx, anchor)
			if err != nil {
				return apiErr(err)
			}
			date, err := parseDate(dateStr)
			if err != nil {
				return usageErr(err)
			}
			zone, err := time.LoadLocation(zoneName)
			if err != nil {
				zone = time.UTC
			}
			times := sun.Compute(date, res.Lat, res.Lng)

			out := goldenReport{
				Anchor:  res,
				Date:    date.Format("2006-01-02"),
				Zone:    zone.String(),
				Sunrise: times.Sunrise.In(zone).Format(time.RFC3339),
				Sunset:  times.Sunset.In(zone).Format(time.RFC3339),
				BlueHourEvening: window{
					Start: times.BlueHourEve.Start.In(zone).Format(time.RFC3339),
					End:   times.BlueHourEve.End.In(zone).Format(time.RFC3339),
				},
				BlueHourMorning: window{
					Start: times.BlueHourMorn.Start.In(zone).Format(time.RFC3339),
					End:   times.BlueHourMorn.End.In(zone).Format(time.RFC3339),
				},
				GoldenHourEvening: window{
					Start: times.GoldenHourEve.Start.In(zone).Format(time.RFC3339),
					End:   times.GoldenHourEve.End.In(zone).Format(time.RFC3339),
				},
				GoldenHourMorning: window{
					Start: times.GoldenHourMorn.Start.In(zone).Format(time.RFC3339),
					End:   times.GoldenHourMorn.End.In(zone).Format(time.RFC3339),
				},
			}

			// Local-store viewpoint pairing — silently skip if the store
			// has no rows (sync-city populates it).
			store, serr := openGoatStore(cmd, flags)
			if serr == nil {
				defer store.Close()
				radius := walking.MetersFromMinutes(minutes)
				viewpoints, _ := store.QueryRadius(ctx, res.Lat, res.Lng, radius, "viewpoint")
				sort.Slice(viewpoints, func(i, j int) bool { return viewpoints[i].Trust > viewpoints[j].Trust })
				if len(viewpoints) > 5 {
					viewpoints = viewpoints[:5]
				}
				anchorLL := walking.LatLng{Lat: res.Lat, Lng: res.Lng}
				for _, vp := range viewpoints {
					vpLL := walking.LatLng{Lat: vp.Lat, Lng: vp.Lng}
					meters := walking.HaversineMeters(anchorLL, vpLL)
					out.Viewpoints = append(out.Viewpoints, viewpoint{
						Name:           vp.Name,
						Source:         vp.Source,
						Lat:            vp.Lat,
						Lng:            vp.Lng,
						DistanceMeters: meters,
						WalkingMinutes: walking.MinutesFromMeters(meters),
						Trust:          vp.Trust,
						WhySpecial:     vp.WhySpecial,
					})
				}
			}
			if len(out.Viewpoints) == 0 {
				out.Note = "No local viewpoints in the goatstore. Run `sync-city <slug> --country <CC>` to populate, then re-run."
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dateStr, "date", time.Now().Format("2006-01-02"), "Date YYYY-MM-DD (default: today)")
	cmd.Flags().Float64Var(&minutes, "minutes", 20, "Walking-time radius in minutes")
	cmd.Flags().StringVar(&zoneName, "zone", "UTC", "IANA zone for displayed times (e.g. Asia/Tokyo, Europe/Paris)")
	return cmd
}

type goldenReport struct {
	Anchor            dispatch.AnchorResolution `json:"anchor"`
	Date              string                    `json:"date"`
	Zone              string                    `json:"zone"`
	Sunrise           string                    `json:"sunrise"`
	Sunset            string                    `json:"sunset"`
	BlueHourMorning   window                    `json:"blue_hour_morning"`
	BlueHourEvening   window                    `json:"blue_hour_evening"`
	GoldenHourMorning window                    `json:"golden_hour_morning"`
	GoldenHourEvening window                    `json:"golden_hour_evening"`
	Viewpoints        []viewpoint               `json:"viewpoints"`
	Note              string                    `json:"note,omitempty"`
}

type window struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type viewpoint struct {
	Name           string  `json:"name"`
	Source         string  `json:"source"`
	Lat            float64 `json:"lat"`
	Lng            float64 `json:"lng"`
	DistanceMeters float64 `json:"distance_meters"`
	WalkingMinutes float64 `json:"walking_minutes"`
	Trust          float64 `json:"trust"`
	WhySpecial     string  `json:"why_special,omitempty"`
}
