package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/internal/cliutil"
)

// controversial ranks current top stories by a comment-to-point
// ratio. The Firebase API doesn't expose this; pulse and search
// return hits but neither ranks by dissent. The interesting threshold
// is stories with > some-comments-per-point — pure ranking gets
// noisy from low-engagement items, so we let the user set a floor.
//
// Implementation: fetch top N IDs, fan-out fetch each item, rank in
// memory. We pull the *current* front page rather than the synced
// store so the command works without a sync step.

type controversialRow struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Score       int     `json:"score"`
	Descendants int     `json:"descendants"`
	Ratio       float64 `json:"ratio"`
}

func newControversialCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var minComments int
	var minScore int

	cmd := &cobra.Command{
		Use:   "controversial",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Find stories with the highest comment-to-point ratio (polarizing discussions)",
		Long: `Rank locally synced stories by descendants/score.

A high ratio implies many comments relative to upvotes — usually a
contentious topic. The default floor of 25 comments and 10 points
filters out drive-by submissions; lower or raise as needed.`,
		Example: strings.Trim(`
  hackernews-pp-cli controversial --limit 10
  hackernews-pp-cli controversial --min-comments 100 --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Fetch the top N IDs.
			rawList, err := c.Get("/topstories.json", nil)
			if err != nil {
				return apiErr(err)
			}
			var ids []int
			if jerr := json.Unmarshal(rawList, &ids); jerr != nil {
				return apiErr(fmt.Errorf("parsing top stories list: %w", jerr))
			}
			if len(ids) > 100 {
				ids = ids[:100]
			}

			// Fan-out fetch each item.
			results, _ := cliutil.FanoutRun(
				context.Background(),
				toStringIDs(ids),
				func(s string) string { return s },
				func(ctx context.Context, id string) (json.RawMessage, error) {
					return c.Get("/item/"+id+".json", nil)
				},
				cliutil.WithConcurrency(8),
			)

			out := []controversialRow{}
			for _, r := range results {
				obj := map[string]any{}
				if jerr := json.Unmarshal(r.Value, &obj); jerr != nil {
					continue
				}
				score, _ := obj["score"].(float64)
				desc, _ := obj["descendants"].(float64)
				if int(score) < minScore || int(desc) < minComments {
					continue
				}
				ratio := 0.0
				if score > 0 {
					ratio = desc / score
				}
				idVal, _ := obj["id"].(float64)
				out = append(out, controversialRow{
					ID:          fmt.Sprintf("%d", int64(idVal)),
					Title:       stringOrEmpty(obj["title"]),
					URL:         stringOrEmpty(obj["url"]),
					Score:       int(score),
					Descendants: int(desc),
					Ratio:       ratio,
				})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Ratio > out[j].Ratio })
			if len(out) > limit {
				out = out[:limit]
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				j, _ := json.MarshalIndent(out, "", "  ")
				return printOutput(cmd.OutOrStdout(), j, true)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no stories match — try lowering --min-comments or run sync first")
				return nil
			}
			tableRows := make([][]string, 0, len(out))
			for _, r := range out {
				tableRows = append(tableRows, []string{
					r.ID,
					strconv.Itoa(r.Descendants),
					strconv.Itoa(r.Score),
					strconv.FormatFloat(r.Ratio, 'f', 2, 64),
					truncateAtRune(r.Title, 70),
				})
			}
			return flags.printTable(cmd, []string{"ID", "CMTS", "PTS", "RATIO", "TITLE"}, tableRows)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum results to return")
	cmd.Flags().IntVar(&minComments, "min-comments", 25, "Floor on descendants count to avoid drive-by submissions")
	cmd.Flags().IntVar(&minScore, "min-score", 10, "Floor on points to avoid divide-by-zero noise")
	return cmd
}
