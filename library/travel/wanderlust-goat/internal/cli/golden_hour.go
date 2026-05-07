package cli

import (
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sun"
)

func newGoldenHourCmd(flags *rootFlags) *cobra.Command {
	var (
		dateStr  string
		minutes  int
		zoneName string
	)
	cmd := &cobra.Command{
		Use:   "golden-hour <anchor>",
		Short: "Compute sunrise/sunset/blue-hour locally (pure Go, no API) and pair with viewpoints photographers know about within walking distance.",
		Long: `Pure-Go SunCalc-style sun-position math (no external API) plus a
walking-radius viewpoint search. Returns:
- sunrise / sunset / civil dawn / civil dusk
- evening blue hour and golden hour windows (and morning equivalents)
- viewpoints from the local store ranked by elevation tag and Reddit
  accessibility hits.

Use --zone to display in a specific local time zone (e.g. Asia/Tokyo).`,
		Example: strings.Trim(`
  # Tonight's blue-hour windows + Tokyo viewpoints
  wanderlust-goat-pp-cli golden-hour "Tokyo Tower" --date 2026-06-15 --zone Asia/Tokyo

  # Lat,lng + minutes-radius
  wanderlust-goat-pp-cli golden-hour 48.8584,2.2945 --date 2026-06-21 --minutes 30 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			anchor := strings.Join(args, " ")
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			res, err := resolveAnchor(ctx, anchor)
			if err != nil {
				return err
			}
			date, err := parseDate(dateStr)
			if err != nil {
				return err
			}
			zone, err := time.LoadLocation(zoneName)
			if err != nil {
				zone = time.UTC
			}
			times := sun.Compute(date, res.Lat, res.Lng)

			store, err := openGoatStore(cmd, flags)
			if err != nil {
				return err
			}
			defer store.Close()
			radiusMeters := walkingMinutesToMeters(minutes)
			viewpoints, _ := store.QueryRadius(ctx, res.Lat, res.Lng, float64(radiusMeters), "viewpoint")
			// Score by trust + reddit accessibility hint via existing column.
			sort.Slice(viewpoints, func(i, j int) bool { return viewpoints[i].Trust > viewpoints[j].Trust })
			if len(viewpoints) > 5 {
				viewpoints = viewpoints[:5]
			}
			out := goldenReport{
				Anchor: res, Date: date.Format("2006-01-02"), Zone: zone.String(),
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
			for _, vp := range viewpoints {
				out.Viewpoints = append(out.Viewpoints, viewpoint{
					Name: vp.Name, Source: vp.Source, Lat: vp.Lat, Lng: vp.Lng,
					DistanceMeters: haversineMeters(res.Lat, res.Lng, vp.Lat, vp.Lng),
					WalkingMin:     metersToWalkingMinutes(haversineMeters(res.Lat, res.Lng, vp.Lat, vp.Lng)),
					Trust:          vp.Trust,
					WhySpecial:     vp.WhySpecial,
				})
			}
			if len(out.Viewpoints) == 0 {
				out.Note = "No local viewpoints in the store. Run 'sync-city <slug>' to populate, then re-run."
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dateStr, "date", time.Now().Format("2006-01-02"), "Date YYYY-MM-DD (default: today).")
	cmd.Flags().IntVar(&minutes, "minutes", 20, "Walking-time radius in minutes for viewpoint search.")
	cmd.Flags().StringVar(&zoneName, "zone", "UTC", "IANA zone for displayed times (e.g. Asia/Tokyo, Europe/Paris). Defaults to UTC; pass the destination zone for human-readable local times.")
	return cmd
}

func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Now().UTC(), nil
	}
	return time.Parse("2006-01-02", s)
}

type goldenReport struct {
	Anchor            AnchorResolution `json:"anchor"`
	Date              string           `json:"date"`
	Zone              string           `json:"zone"`
	Sunrise           string           `json:"sunrise"`
	Sunset            string           `json:"sunset"`
	BlueHourMorning   window           `json:"blue_hour_morning"`
	BlueHourEvening   window           `json:"blue_hour_evening"`
	GoldenHourMorning window           `json:"golden_hour_morning"`
	GoldenHourEvening window           `json:"golden_hour_evening"`
	Viewpoints        []viewpoint      `json:"viewpoints"`
	Note              string           `json:"note,omitempty"`
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
	WalkingMin     float64 `json:"walking_min"`
	Trust          float64 `json:"trust"`
	WhySpecial     string  `json:"why_special,omitempty"`
}
