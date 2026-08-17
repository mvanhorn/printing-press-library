// Hand-authored absorbed analytics over OpenAlex group_by facets:
// rank-authors, rank-institutions, rank-journals, timeline, trends.
// Not generated.
package cli

import (
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"
)

// newRankCmd builds a rank-<entity> command backed by an OpenAlex group_by.
func newRankCmd(flags *rootFlags, use, short, groupBy, unit string) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         use + " <query>",
		Short:       short,
		Example:     "  scientific-consensus-pp-cli " + use + " \"vitamin d\" --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would rank %s\n", unit)
				return nil
			}
			query, err := requireQuery(args)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rows, err := fetchGroupBy(ctx, c, "/works", query, "", groupBy)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			// group_by is already count-desc from OpenAlex; trim and drop unknowns.
			out := make([]groupRow, 0, limit)
			for _, r := range rows {
				if r.Name == "" || r.Name == "unknown" {
					continue
				}
				out = append(out, r)
				if len(out) >= limit {
					break
				}
			}
			return emit(cmd, flags, struct {
				Query string     `json:"query"`
				Unit  string     `json:"unit"`
				Rows  []groupRow `json:"rows"`
			}{query, unit, out}, func(w io.Writer) {
				fmt.Fprintf(w, "Top %s for: %s\n\n", unit, query)
				for i, r := range out {
					fmt.Fprintf(w, "  %2d. %-52s %d works\n", i+1, truncate(r.Name, 52), r.Count)
				}
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 15, "number of "+unit+" to return")
	return cmd
}

func newRankAuthorsCmd(flags *rootFlags) *cobra.Command {
	return newRankCmd(flags, "rank-authors", "Rank the most prolific authors publishing on a topic", "authorships.author.id", "authors")
}
func newRankInstitutionsCmd(flags *rootFlags) *cobra.Command {
	return newRankCmd(flags, "rank-institutions", "Rank the most prolific institutions publishing on a topic", "authorships.institutions.id", "institutions")
}
func newRankJournalsCmd(flags *rootFlags) *cobra.Command {
	return newRankCmd(flags, "rank-journals", "Rank the journals publishing most on a topic", "primary_location.source.id", "journals")
}

type timelinePoint struct {
	Year  int `json:"year"`
	Works int `json:"works"`
}

func newTimelineCmd(flags *rootFlags) *cobra.Command {
	var sinceYear int
	cmd := &cobra.Command{
		Use:         "timeline <query>",
		Short:       "Show publication history and milestone year for a topic",
		Long:        "Render the per-year publication count for a topic and flag the peak (milestone)\nyear. Use --since to limit the range.",
		Example:     "  scientific-consensus-pp-cli timeline \"mrna vaccine\" --since 2000 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would render publication timeline")
				return nil
			}
			query, err := requireQuery(args)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rows, err := fetchGroupBy(ctx, c, "/works", query, "", "publication_year")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			points := make([]timelinePoint, 0, len(rows))
			peakYear, peakCount := 0, 0
			for _, r := range rows {
				var y int
				fmt.Sscanf(r.Key, "%d", &y)
				if y == 0 || (sinceYear > 0 && y < sinceYear) {
					continue
				}
				points = append(points, timelinePoint{Year: y, Works: r.Count})
				if r.Count > peakCount {
					peakCount, peakYear = r.Count, y
				}
			}
			sort.Slice(points, func(i, j int) bool { return points[i].Year < points[j].Year })
			return emit(cmd, flags, struct {
				Query        string          `json:"query"`
				MilestoneYear int            `json:"milestone_year"`
				Points       []timelinePoint `json:"points"`
			}{query, peakYear, points}, func(w io.Writer) {
				fmt.Fprintf(w, "Publication timeline: %s  (peak %d)\n\n", query, peakYear)
				for _, p := range points {
					fmt.Fprintf(w, "  %d  %5d  %s\n", p.Year, p.Works, bar(p.Works, peakCount, 40))
				}
			})
		},
	}
	cmd.Flags().IntVar(&sinceYear, "since", 0, "only include years from this year onward")
	return cmd
}

type trendPoint struct {
	Year      int     `json:"year"`
	Works     int     `json:"works"`
	GrowthPct float64 `json:"growth_pct"`
}

func newTrendsCmd(flags *rootFlags) *cobra.Command {
	var years int
	cmd := &cobra.Command{
		Use:         "trends <query>",
		Short:       "Show year-over-year publication growth for a topic",
		Long:        "Compute per-year publication counts for the last --years years and the\nyear-over-year growth percentage, plus the overall trend direction.",
		Example:     "  scientific-consensus-pp-cli trends \"large language models\" --years 8 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute publication trends")
				return nil
			}
			query, err := requireQuery(args)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rows, err := fetchGroupBy(ctx, c, "/works", query, "", "publication_year")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			byYear := map[int]int{}
			maxYear := 0
			for _, r := range rows {
				var y int
				fmt.Sscanf(r.Key, "%d", &y)
				if y == 0 {
					continue
				}
				byYear[y] = r.Count
				if y > maxYear {
					maxYear = y
				}
			}
			if years <= 0 {
				years = 8
			}
			points := make([]trendPoint, 0, years)
			prev := 0
			for y := maxYear - years + 1; y <= maxYear; y++ {
				cnt := byYear[y]
				growth := 0.0
				if prev > 0 {
					growth = round1(float64(cnt-prev) * 100 / float64(prev))
				}
				points = append(points, trendPoint{Year: y, Works: cnt, GrowthPct: growth})
				prev = cnt
			}
			direction := "flat"
			if len(points) >= 2 {
				first, last := points[0].Works, points[len(points)-1].Works
				switch {
				case last > first*5/4:
					direction = "rising"
				case last < first*4/5:
					direction = "declining"
				}
			}
			return emit(cmd, flags, struct {
				Query     string       `json:"query"`
				Direction string       `json:"direction"`
				Points    []trendPoint `json:"points"`
			}{query, direction, points}, func(w io.Writer) {
				fmt.Fprintf(w, "Publication trend: %s  (%s)\n\n", query, direction)
				for _, p := range points {
					fmt.Fprintf(w, "  %d  %5d  %+6.1f%%\n", p.Year, p.Works, p.GrowthPct)
				}
			})
		},
	}
	cmd.Flags().IntVar(&years, "years", 8, "number of recent years to analyze")
	return cmd
}
