package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/phgql"

	"github.com/spf13/cobra"
)

func newPostsBenchmarkCmd(flags *rootFlags) *cobra.Command {
	var (
		topic string
		hour  int
		count int
	)
	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "Show percentile vote curves at hour-N for top-10 / top-50 launches in a topic",
		Long: strings.Trim(`
Helps a founder set realistic vote targets for hour-N of launch day. Pulls
recent featured launches in the topic, then reports the median, top-10 percentile
and top-50 percentile of vote counts among launches old enough to have crossed
hour-N.

Note: this is "current votes for launches that are at least N hours old" — a
true hour-by-hour benchmark would require historical hourly snapshots from
your local store.
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli posts benchmark --topic artificial-intelligence --hour 6
  producthunt-pp-cli posts benchmark --topic developer-tools --hour 12 --count 50 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if topic == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if hour < 1 {
				return fmt.Errorf("--hour must be >= 1")
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := phgql.New(cfg)
			// Pull recent launches in this topic; we'll then filter to those at least `hour` hours old.
			vars := map[string]any{"first": count, "topic": topic, "order": "VOTES"}
			var resp phgql.PostsResponse
			if _, err := c.Query(cmd.Context(), phgql.PostsQuery, vars, &resp); err != nil {
				return err
			}
			votes := make([]int, 0, len(resp.Posts.Edges))
			for _, e := range resp.Posts.Edges {
				ageHours := time.Since(e.Node.CreatedAt).Hours()
				if int(ageHours) >= hour {
					votes = append(votes, e.Node.VotesCount)
				}
			}
			sort.Ints(votes)
			out := benchmarkOut{
				Topic:    topic,
				Hour:     hour,
				Sampled:  len(votes),
				Median:   percentile(votes, 50),
				P75:      percentile(votes, 75),
				P90:      percentile(votes, 90),
				MinVotes: firstOr(votes, 0),
				MaxVotes: lastOr(votes, 0),
				Note:     fmt.Sprintf("Reported as: current vote counts for launches in topic %q that are at least %dh old. For true per-hour curves you need historical snapshots; schedule `posts trajectory` hourly to build them.", topic, hour),
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&topic, "topic", "", "Topic slug (required)")
	cmd.Flags().IntVar(&hour, "hour", 6, "Hour of launch day (e.g. 6 = 6 hours after launch). Filters to launches at least N hours old.")
	cmd.Flags().IntVar(&count, "count", 20, "Number of recent launches to sample (max 20 per page)")
	return cmd
}

type benchmarkOut struct {
	Topic    string `json:"topic"`
	Hour     int    `json:"hour"`
	Sampled  int    `json:"sampled"`
	Median   int    `json:"median_votes"`
	P75      int    `json:"p75_votes"`
	P90      int    `json:"p90_votes"`
	MinVotes int    `json:"min_votes"`
	MaxVotes int    `json:"max_votes"`
	Note     string `json:"note"`
}

func percentile(sorted []int, p int) int {
	if len(sorted) == 0 {
		return 0
	}
	idx := (len(sorted) - 1) * p / 100
	return sorted[idx]
}

func firstOr(s []int, def int) int {
	if len(s) == 0 {
		return def
	}
	return s[0]
}

func lastOr(s []int, def int) int {
	if len(s) == 0 {
		return def
	}
	return s[len(s)-1]
}
