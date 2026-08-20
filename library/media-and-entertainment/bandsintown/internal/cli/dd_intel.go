// Novel intelligence commands: route, gaps, lineup, snapshot, trend, sea-radar.
// All operate against the typed dd_* tables hydrated by `pull` (or `sync` if
// the framework sync is later wired to also write to dd tables).

package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/bandsintown/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/bandsintown/internal/dd"
)

// --------------------------------------------------------------------------
// route — tour-routing feasibility
// --------------------------------------------------------------------------

func newRouteCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath     string
		toLatLng   string
		onDate     string
		windowFlag string
		trackedFlg bool
		scoreFlag  bool
		limit      int
	)

	cmd := &cobra.Command{
		Use:   "route",
		Short: "Find tracked artists with shows near a target city/date",
		Long: `Joins the local dd_events table against dd_venues + dd_tracked_artists to
surface routing candidates within --window of --on near --to. Output is sorted
ascending by event datetime; use --score to add a 0-1 composite feasibility
score (0.4 gap + 0.3 distance + 0.3 tracker percentile).`,
		Example:     `  bandsintown-pp-cli route --to "Jakarta,ID" --on 2026-08-15 --window 7d --tracked --score --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			lat, lng, err := resolveCityLatLng(toLatLng)
			if err != nil {
				return usageErr(err)
			}
			target, err := time.Parse("2006-01-02", onDate)
			if err != nil {
				return usageErr(fmt.Errorf("--on must be YYYY-MM-DD: %w", err))
			}
			windowDays, err := parseWindowDays(windowFlag)
			if err != nil {
				return usageErr(err)
			}

			db, err := dd.Open(cmd.Context(), resolveDBPath(dbPath))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()

			rows, err := dd.Route(cmd.Context(), db, lat, lng, target, windowDays, trackedFlg, scoreFlag)
			if err != nil {
				return apiErr(err)
			}
			if scoreFlag {
				// Sort by score desc
				for i := 1; i < len(rows); i++ {
					for j := i; j > 0 && rows[j].Score > rows[j-1].Score; j-- {
						rows[j], rows[j-1] = rows[j-1], rows[j]
					}
				}
			}
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"to":          toLatLng,
					"on":          onDate,
					"window_days": windowDays,
					"tracked":     trackedFlg,
					"candidates":  rows,
				})
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), dd.EmptyHint("no events near "+toLatLng+" within ±"+strconv.Itoa(windowDays)+"d of "+onDate))
				return nil
			}
			headers := []string{"ARTIST", "DATE", "GAP", "CITY", "COUNTRY", "DIST_KM", "TRACKER"}
			if scoreFlag {
				headers = append(headers, "SCORE")
			}
			out := [][]string{}
			for _, r := range rows {
				row := []string{
					r.Artist, r.Datetime[:min(len(r.Datetime), 10)],
					fmt.Sprintf("±%dd", r.DaysGap),
					r.VenueCity, r.VenueCountry,
					fmt.Sprintf("%.0f", r.DistanceKM),
					strconv.Itoa(r.TrackerCount),
				}
				if scoreFlag {
					row = append(row, fmt.Sprintf("%.2f", r.Score))
				}
				out = append(out, row)
			}
			return flags.printTable(cmd, headers, out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&toLatLng, "to", "Jakarta,ID", "Target city (LAT,LNG or one of the built-in cities: Jakarta,ID Singapore,SG Bangkok,TH Manila,PH Tokyo,JP Seoul,KR KL,MY)")
	cmd.Flags().StringVar(&onDate, "on", time.Now().Format("2006-01-02"), "Target date YYYY-MM-DD")
	cmd.Flags().StringVar(&windowFlag, "window", "7d", "Window around --on (e.g. 7d, 14d, 21d)")
	cmd.Flags().BoolVar(&trackedFlg, "tracked", true, "Restrict to tracked artists")
	cmd.Flags().BoolVar(&scoreFlag, "score", false, "Compute composite feasibility score and sort by it")
	cmd.Flags().IntVar(&limit, "limit", 0, "Limit result rows (0 = unlimited)")
	return cmd
}

// Built-in city → (lat,lng). Spelled out so users can pass either
// "Jakarta,ID" or "-6.21,106.85".
var seaCities = map[string][2]float64{
	"Jakarta,ID":      {-6.2088, 106.8456},
	"Singapore,SG":    {1.3521, 103.8198},
	"Bangkok,TH":      {13.7563, 100.5018},
	"Manila,PH":       {14.5995, 120.9842},
	"Tokyo,JP":        {35.6762, 139.6503},
	"Seoul,KR":        {37.5665, 126.9780},
	"KL,MY":           {3.1390, 101.6869},
	"Kuala Lumpur,MY": {3.1390, 101.6869},
	"Ho Chi Minh,VN":  {10.8231, 106.6297},
	"Taipei,TW":       {25.0330, 121.5654},
}

func resolveCityLatLng(s string) (float64, float64, error) {
	s = strings.TrimSpace(s)
	if v, ok := seaCities[s]; ok {
		return v[0], v[1], nil
	}
	// Allow LAT,LNG numeric pair
	parts := strings.SplitN(s, ",", 2)
	if len(parts) == 2 {
		if lat, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64); err == nil {
			if lng, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
				return lat, lng, nil
			}
		}
	}
	return 0, 0, fmt.Errorf("unknown city %q — use LAT,LNG or one of: Jakarta,ID Singapore,SG Bangkok,TH Manila,PH Tokyo,JP Seoul,KR KL,MY", s)
}

func parseWindowDays(s string) (int, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		s = strings.TrimSuffix(s, "d")
	}
	if strings.HasSuffix(s, "w") {
		w, err := strconv.Atoi(strings.TrimSuffix(s, "w"))
		if err != nil {
			return 0, fmt.Errorf("bad window: %s", s)
		}
		return w * 7, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("bad window: %s", s)
	}
	return n, nil
}

// --------------------------------------------------------------------------
// gaps — empty windows in an artist's calendar
// --------------------------------------------------------------------------

func newGapsCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		minStr string
		maxStr string
		region string
	)
	cmd := &cobra.Command{
		Use:   "gaps [artist]",
		Short: "Find empty windows between consecutive shows for an artist",
		Long: `Walks an artist's synced events in date order and surfaces gaps where the
days between consecutive shows fall inside [--min, --max]. Optionally constrain
to a region; "SEA" expands to a 15-country bounding set.`,
		Example:     `  bandsintown-pp-cli gaps "Beach House" --min 5d --max 21d --in SEA --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			minDays, err := parseWindowDays(minStr)
			if err != nil {
				return usageErr(fmt.Errorf("--min: %w", err))
			}
			maxDays, err := parseWindowDays(maxStr)
			if err != nil {
				return usageErr(fmt.Errorf("--max: %w", err))
			}
			db, err := dd.Open(cmd.Context(), resolveDBPath(dbPath))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()

			rows, err := dd.Gaps(cmd.Context(), db, args[0], minDays, maxDays, region)
			if err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"artist":   args[0],
					"min_days": minDays,
					"max_days": maxDays,
					"region":   region,
					"gaps":     rows,
				})
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), dd.EmptyHint("no gaps for "+args[0]+" in ["+strconv.Itoa(minDays)+","+strconv.Itoa(maxDays)+"]d"))
				return nil
			}
			out := [][]string{}
			for _, r := range rows {
				out = append(out, []string{
					r.GapStart, r.GapEnd, fmt.Sprintf("%dd", r.GapDays),
					r.BeforeCity + "," + r.BeforeCountry,
					r.AfterCity + "," + r.AfterCountry,
				})
			}
			return flags.printTable(cmd,
				[]string{"GAP_START", "GAP_END", "DAYS", "BEFORE", "AFTER"},
				out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&minStr, "min", "5", "Minimum gap (e.g. 5, 5d, 1w)")
	cmd.Flags().StringVar(&maxStr, "max", "30", "Maximum gap (e.g. 30, 21d, 4w)")
	cmd.Flags().StringVar(&region, "in", "", "Region filter (SEA shorthand or comma-separated country names/ISO codes)")
	return cmd
}

// --------------------------------------------------------------------------
// lineup co-bill
// --------------------------------------------------------------------------

func newLineupCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lineup",
		Short: "Lineup intelligence (co-bill, festival detection)",
		Long:  "Cross-event aggregations over synced lineup arrays.",
	}
	cmd.AddCommand(newLineupCoBillCmd(flags))
	return cmd
}

func newLineupCoBillCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath    string
		since     string
		minShared int
	)
	cmd := &cobra.Command{
		Use:   "co-bill [artist]",
		Short: "Artists who frequently co-bill with the given artist",
		Long: `Aggregates the dd_lineup_members junction across every synced event of the
target artist. Returns collaborators ranked by shared appearances. Run 'pull'
with --date past or --date all to grow the historical sample.`,
		Example:     `  bandsintown-pp-cli lineup co-bill "Phoenix" --since 2024-01-01 --min-shared 2 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := dd.Open(cmd.Context(), resolveDBPath(dbPath))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()
			rows, err := dd.LineupCoBill(cmd.Context(), db, args[0], since, minShared)
			if err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"artist":      args[0],
					"since":       since,
					"min_shared":  minShared,
					"co_billings": rows,
				})
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), dd.EmptyHint("no co-bills for "+args[0]+" with min "+strconv.Itoa(minShared)+" shared events"))
				return nil
			}
			out := [][]string{}
			for _, r := range rows {
				out = append(out, []string{r.Collaborator, strconv.Itoa(r.Shared), r.FirstSeen, r.LastSeen})
			}
			return flags.printTable(cmd,
				[]string{"COLLABORATOR", "SHARED", "FIRST_SEEN", "LAST_SEEN"},
				out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&since, "since", "", "Lower-bound date (YYYY-MM-DD) on the target's events")
	cmd.Flags().IntVar(&minShared, "min-shared", 2, "Minimum shared events to surface a collaborator")
	return cmd
}

// --------------------------------------------------------------------------
// snapshot — write tracker_count + upcoming_event_count for every tracked artist
// --------------------------------------------------------------------------

func newSnapshotCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		appID  string
	)
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Snapshot tracker_count and upcoming_event_count for every tracked artist",
		Long: `Fetches each tracked artist's current tracker_count + upcoming_event_count
and writes a row to dd_artist_snapshots. The 'trend' command later reads these
rows to compute deltas over time.`,
		Example:     `  bandsintown-pp-cli snapshot --json`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := dd.Open(cmd.Context(), resolveDBPath(dbPath))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()
			tracked, err := dd.ListTracked(cmd.Context(), db)
			if err != nil {
				return apiErr(err)
			}
			if len(tracked) == 0 {
				return usageErr(fmt.Errorf("watchlist is empty: run `track add` first"))
			}
			if appID == "" {
				if cfg, _ := config.Load(flags.configPath); cfg != nil {
					appID = cfg.AppID
				}
			}
			if appID == "" {
				return authErr(fmt.Errorf("BANDSINTOWN_APP_ID not set (partner key required since 2025)"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type res struct {
				Artist       string `json:"artist"`
				TrackerCount int    `json:"tracker_count"`
				UpcomingEv   int    `json:"upcoming_event_count"`
				SnappedAt    string `json:"snapped_at"`
				Error        string `json:"error,omitempty"`
			}
			results := []res{}
			for _, t := range tracked {
				artist, err := dd.FetchArtist(c, t.Name, appID)
				if err != nil {
					results = append(results, res{Artist: t.Name, Error: err.Error()})
					continue
				}
				_ = dd.UpsertArtist(cmd.Context(), db, artist)
				ts, err := dd.SnapshotArtist(cmd.Context(), db, artist)
				if err != nil {
					results = append(results, res{Artist: t.Name, Error: err.Error()})
					continue
				}
				results = append(results, res{
					Artist: t.Name, TrackerCount: artist.TrackerCount,
					UpcomingEv: artist.UpcomingEventCount, SnappedAt: ts,
				})
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"snapshotted": len(results),
					"results":     results,
				})
			}
			for _, r := range results {
				if r.Error != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "FAIL %s: %s\n", r.Artist, r.Error)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "OK %s tracker=%d upcoming=%d\n", r.Artist, r.TrackerCount, r.UpcomingEv)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&appID, "app-id", "", "Partner application ID (falls back to BANDSINTOWN_APP_ID)")
	return cmd
}

// --------------------------------------------------------------------------
// trend — read snapshot history, surface tracker_count deltas
// --------------------------------------------------------------------------

func newTrendCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath    string
		periodStr string
		topN      int
	)
	cmd := &cobra.Command{
		Use:   "trend",
		Short: "Surface rising / falling tracker_count over the past N days",
		Long: `Reads dd_artist_snapshots and computes (latest - earliest) tracker_count
per artist over the past --period days. Sorted descending by delta; falling
artists appear at the tail. Run 'snapshot' daily to grow the time series.`,
		Example:     `  bandsintown-pp-cli trend --top 20 --period 30d --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			periodDays, err := parseWindowDays(periodStr)
			if err != nil {
				return usageErr(fmt.Errorf("--period: %w", err))
			}
			db, err := dd.Open(cmd.Context(), resolveDBPath(dbPath))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()
			rows, err := dd.Trend(cmd.Context(), db, periodDays, topN)
			if err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"period_days": periodDays,
					"top":         topN,
					"trends":      rows,
				})
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), dd.EmptyHint("need ≥2 snapshots per artist in the last "+strconv.Itoa(periodDays)+"d. Run `bandsintown-pp-cli snapshot` more than once before this."))
				return nil
			}
			out := [][]string{}
			for _, r := range rows {
				out = append(out, []string{
					r.Artist, strconv.Itoa(r.TrackerNow), strconv.Itoa(r.TrackerThen),
					fmt.Sprintf("%+d", r.Delta), fmt.Sprintf("%+.1f%%", r.PercentChange),
				})
			}
			return flags.printTable(cmd,
				[]string{"ARTIST", "NOW", "THEN", "DELTA", "%"},
				out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&periodStr, "period", "30", "Period over which to compute deltas (e.g. 30, 30d, 4w)")
	cmd.Flags().IntVar(&topN, "top", 20, "Limit rows to top N by absolute delta")
	return cmd
}

// --------------------------------------------------------------------------
// sea-radar — composed query: SEA-only upcoming shows of tracked artists
// --------------------------------------------------------------------------

func newSeaRadarCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath  string
		dateRng string
		tier    string
	)
	cmd := &cobra.Command{
		Use:   "sea-radar",
		Short: "One-shot SEA briefing: tracked-artist shows in a date window",
		Long: `Joins synced events × watchlist, filters to a 15-country Southeast Asia +
East Asia bounding set, and surfaces every show inside --date. Optional --tier
restricts to a watchlist tier (e.g. mid / emerging).`,
		Example:     `  bandsintown-pp-cli sea-radar --date 2026-08-01,2026-08-31 --tier mid --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			parts := strings.SplitN(dateRng, ",", 2)
			if len(parts) != 2 {
				return usageErr(fmt.Errorf("--date must be YYYY-MM-DD,YYYY-MM-DD"))
			}
			db, err := dd.Open(cmd.Context(), resolveDBPath(dbPath))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()
			rows, err := dd.SeaRadar(cmd.Context(), db, strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), tier)
			if err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"date":  dateRng,
					"tier":  tier,
					"shows": rows,
				})
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), dd.EmptyHint("no SEA shows for tracked artists in "+dateRng))
				return nil
			}
			out := [][]string{}
			for _, r := range rows {
				out = append(out, []string{
					r.Datetime[:min(len(r.Datetime), 10)], r.City + "," + r.Country,
					r.Artist, r.VenueName, strconv.Itoa(r.TrackerCount), r.Tier,
				})
			}
			return flags.printTable(cmd,
				[]string{"DATE", "CITY", "ARTIST", "VENUE", "TRACKER", "TIER"},
				out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&dateRng, "date", time.Now().Format("2006-01-02")+","+time.Now().AddDate(0, 1, 0).Format("2006-01-02"), "Date range YYYY-MM-DD,YYYY-MM-DD")
	cmd.Flags().StringVar(&tier, "tier", "", "Restrict to a single watchlist tier")
	return cmd
}
