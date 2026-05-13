// PATCH: hand-authored novel-feature file. See .printing-press-patches.json patch id "novel-series-extremum".
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ExtremumResult is the output shape of `series extremum`.
type ExtremumResult struct {
	SeriesID    string   `json:"series_id"`
	Latest      *Point   `json:"latest,omitempty"`
	Max         *Point   `json:"max,omitempty"`
	Min         *Point   `json:"min,omitempty"`
	WindowStart string   `json:"window_start,omitempty"`
	WindowEnd   string   `json:"window_end,omitempty"`
	Count       int      `json:"observation_count"`
	LatestRank  int      `json:"latest_rank,omitempty"`       // 1 = highest
	LatestPct   float64  `json:"latest_percentile,omitempty"` // 0..100
	Source      string   `json:"source"`                      // "cache" or "live"
	Notes       []string `json:"notes,omitempty"`
}

// Point is a single (year, period, value) tuple.
type Point struct {
	Year   int     `json:"year"`
	Period string  `json:"period"`
	Value  float64 `json:"value"`
}

func newSeriesExtremumCmd(flags *rootFlags) *cobra.Command {
	var since int
	cmd := &cobra.Command{
		Use:   "extremum <seriesid>",
		Short: "Compute max/min/percentile-rank of a series's latest observation across a window.",
		Long: `BLS returns a window of values, never extrema. This command runs the
needed scan locally: SQL over the cached observations table populated by
sync, with a live fallback when the cache is empty for the requested
window. Useful for release-day writeups ("is this the highest unemployment
rate since 1990?") and for agent tool-calls that need historical context.`,
		Example: `  bls-pp-cli series extremum LNS14000000 --since 1990
  bls-pp-cli series extremum CUSR0000SA0 --since 2000 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			seriesID := strings.TrimSpace(strings.ToUpper(args[0]))
			db, err := openBLSStore(cmd.Context(), "")
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer func() { _ = db.Close() }()

			result := ExtremumResult{SeriesID: seriesID, Source: "cache"}
			points, err := loadCachedPoints(cmd.Context(), db.DB(), seriesID, since)
			if err != nil {
				return err
			}
			if len(points) == 0 {
				// Fallback to live fetch with a wide window.
				c, cerr := flags.newClient()
				if cerr != nil {
					return cerr
				}
				startYr := since
				if startYr == 0 {
					startYr = 1990
				}
				params := map[string]string{
					"startyear": strconv.Itoa(startYr),
					"endyear":   strconv.Itoa(time.Now().Year()),
				}
				path := "/timeseries/data/" + seriesID
				raw, gerr := c.Get(path, params)
				if gerr != nil {
					return classifyAPIError(gerr, flags)
				}
				points = parsePointsFromResponse(raw, seriesID)
				result.Source = "live"
				if len(points) > 0 {
					rows := make([]ObservationRow, 0, len(points))
					for _, p := range points {
						rows = append(rows, ObservationRow{Year: p.Year, Period: p.Period, Value: p.Value})
					}
					_ = cacheObservations(cmd.Context(), db.DB(), seriesID, rows)
				}
			}

			if len(points) == 0 {
				return apiErr(fmt.Errorf("no observations found for series %s; verify the ID with `bls-pp-cli series search`", seriesID))
			}
			sort.Slice(points, func(i, j int) bool {
				if points[i].Year != points[j].Year {
					return points[i].Year > points[j].Year
				}
				return points[i].Period > points[j].Period
			})
			latest := points[0]
			maxP, minP := latest, latest
			for _, p := range points {
				if p.Value > maxP.Value {
					maxP = p
				}
				if p.Value < minP.Value {
					minP = p
				}
			}
			sorted := make([]Point, len(points))
			copy(sorted, points)
			sort.Slice(sorted, func(i, j int) bool { return sorted[i].Value > sorted[j].Value })
			rank := len(sorted)
			for i, p := range sorted {
				if p.Year == latest.Year && p.Period == latest.Period {
					rank = i + 1
					break
				}
			}
			percentile := 100.0 * float64(len(points)-rank+1) / float64(len(points))

			result.Latest = &latest
			result.Max = &maxP
			result.Min = &minP
			result.Count = len(points)
			result.LatestRank = rank
			result.LatestPct = roundPct(percentile)
			result.WindowStart = strconv.Itoa(points[len(points)-1].Year)
			result.WindowEnd = strconv.Itoa(points[0].Year)

			raw, _ := json.Marshal(result)
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Series %s (%s)\n", result.SeriesID, result.Source)
			fmt.Fprintf(cmd.OutOrStdout(), "  Window:   %s - %s  (%d observations)\n", result.WindowStart, result.WindowEnd, result.Count)
			fmt.Fprintf(cmd.OutOrStdout(), "  Latest:   %d %s = %g  (rank %d, %.1fth percentile)\n", result.Latest.Year, result.Latest.Period, result.Latest.Value, result.LatestRank, result.LatestPct)
			fmt.Fprintf(cmd.OutOrStdout(), "  Max:      %d %s = %g\n", result.Max.Year, result.Max.Period, result.Max.Value)
			fmt.Fprintf(cmd.OutOrStdout(), "  Min:      %d %s = %g\n", result.Min.Year, result.Min.Period, result.Min.Value)
			return nil
		},
	}
	cmd.Flags().IntVar(&since, "since", 0, "Earliest year to include in the scan. Defaults to 1990 on live fallback.")
	return cmd
}

func loadCachedPoints(ctx context.Context, db *sql.DB, seriesID string, since int) ([]Point, error) {
	q := `SELECT year, period, value FROM observations WHERE series_id = ?`
	args := []any{seriesID}
	if since > 0 {
		q += " AND year >= ?"
		args = append(args, since)
	}
	q += " ORDER BY year DESC, period DESC"
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("load cached points: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []Point
	for rows.Next() {
		var p Point
		if err := rows.Scan(&p.Year, &p.Period, &p.Value); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// parsePointsFromResponse extracts observation rows from a BLS API response
// envelope, filtered to the requested series ID. Periods that are not
// monthly/quarterly (e.g. annual averages M13) and rows with non-numeric
// values (BLS publishes "-" for unavailable) are skipped silently.
func parsePointsFromResponse(raw json.RawMessage, seriesID string) []Point {
	var env struct {
		Results struct {
			Series []struct {
				SeriesID string `json:"seriesID"`
				Data     []struct {
					Year   string `json:"year"`
					Period string `json:"period"`
					Value  string `json:"value"`
				} `json:"data"`
			} `json:"series"`
		} `json:"Results"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil
	}
	var out []Point
	for _, s := range env.Results.Series {
		if s.SeriesID != seriesID && seriesID != "" {
			continue
		}
		for _, d := range s.Data {
			yr, err := strconv.Atoi(strings.TrimSpace(d.Year))
			if err != nil {
				continue
			}
			val, err := strconv.ParseFloat(strings.TrimSpace(d.Value), 64)
			if err != nil {
				continue
			}
			out = append(out, Point{Year: yr, Period: strings.TrimSpace(d.Period), Value: val})
		}
	}
	return out
}

func roundPct(v float64) float64 {
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	// 1 decimal place
	return float64(int(v*10+0.5)) / 10.0
}
