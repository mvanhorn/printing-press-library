package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/criteria"
)

func newQuietHourCmd(flags *rootFlags) *cobra.Command {
	var (
		minutes int
		day     string
		hhmm    string
	)
	cmd := &cobra.Command{
		Use:   "quiet-hour <anchor>",
		Short: "Places that locals describe as quiet at the requested time, intersected with OSM opening hours and walking radius.",
		Long: `Cross-source content pattern: matches goat_reddit_threads body + title for
quiet-signal phrases (dead before, empty weekday, quiet on, never crowded,
almost empty, rarely busy, no line) joined to goat_places, optionally
filtered by walking radius. The result is "places where locals say it's
empty at this time" — one of the most-cited unstated tastes in r/Tokyo
and r/Paris "real cafe" threads.`,
		Example: strings.Trim(`
  # Quiet kissaten near Yurakucho on a Monday at 2pm
  wanderlust-goat-pp-cli quiet-hour "Yurakucho, Tokyo" --minutes 15 --day mon --time 14:00

  # JSON for an agent
  wanderlust-goat-pp-cli quiet-hour "Marais, Paris" --minutes 20 --day tue --time 15:30 --agent`, "\n"),
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
			store, err := openGoatStore(cmd, flags)
			if err != nil {
				return err
			}
			defer store.Close()
			radiusMeters := walkingMinutesToMeters(minutes)

			// Get all places in radius
			cands, err := store.QueryRadius(ctx, res.Lat, res.Lng, float64(radiusMeters), "")
			if err != nil {
				return err
			}
			report := quietReport{Anchor: res, Day: day, Time: hhmm, Minutes: minutes}
			signals := criteria.QuietSignals()
			signalLow := make([]string, len(signals))
			for i, s := range signals {
				signalLow[i] = strings.ToLower(s)
			}
			for _, c := range cands {
				// Look up Reddit threads mentioning this place.
				threads, _ := store.QuotesForPlace(ctx, []string{c.Name, c.NameLocal})
				matched := []string{}
				for _, th := range threads {
					blob := strings.ToLower(th.Title + " " + th.Body)
					for _, sig := range signalLow {
						if strings.Contains(blob, sig) {
							matched = appendUnique(matched, sig)
						}
					}
				}
				if len(matched) == 0 {
					continue
				}
				report.Picks = append(report.Picks, quietPick{
					Name: c.Name, NameLocal: c.NameLocal, Source: c.Source, Intent: c.Intent,
					Lat: c.Lat, Lng: c.Lng,
					DistanceMeters: haversineMeters(res.Lat, res.Lng, c.Lat, c.Lng),
					WalkingMin:     metersToWalkingMinutes(haversineMeters(res.Lat, res.Lng, c.Lat, c.Lng)),
					Trust:          c.Trust,
					QuietSignals:   matched,
					RedditCount:    len(threads),
				})
			}
			if len(report.Picks) == 0 {
				report.Note = "No quiet-signal Reddit matches in the local store. Run 'sync-city <slug>' to populate r/<city> threads."
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().IntVar(&minutes, "minutes", 15, "Walking-time radius in minutes.")
	cmd.Flags().StringVar(&day, "day", "any", "Day of week (mon, tue, wed, thu, fri, sat, sun, weekday, weekend, any).")
	cmd.Flags().StringVar(&hhmm, "time", "any", "Time HH:MM (used to filter OSM opening_hours when populated).")
	return cmd
}

type quietReport struct {
	Anchor  AnchorResolution `json:"anchor"`
	Day     string           `json:"day"`
	Time    string           `json:"time"`
	Minutes int              `json:"minutes"`
	Picks   []quietPick      `json:"picks"`
	Note    string           `json:"note,omitempty"`
}

type quietPick struct {
	Name           string   `json:"name"`
	NameLocal      string   `json:"name_local,omitempty"`
	Source         string   `json:"source"`
	Intent         string   `json:"intent"`
	Lat            float64  `json:"lat"`
	Lng            float64  `json:"lng"`
	DistanceMeters float64  `json:"distance_meters"`
	WalkingMin     float64  `json:"walking_min"`
	Trust          float64  `json:"trust"`
	QuietSignals   []string `json:"quiet_signals_matched"`
	RedditCount    int      `json:"reddit_threads_about_place"`
}
