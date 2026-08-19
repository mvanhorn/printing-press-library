// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command: split a blended star rating into per-topic scores.
//
// The reviews API returns a ratings[] array per review with separate topics
// (overall, restaurant_food, rider, ...). The website collapses all of it into
// one star value, so "the food is bad" and "the delivery is bad" look identical.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/foodpanda/internal/cliutil"
)

type fpReviewsEnvelope struct {
	Data []struct {
		UUID         string `json:"uuid"`
		CreatedAt    string `json:"createdAt"`
		Text         string `json:"text"`
		ReviewerName string `json:"reviewerName"`
		IsAnonymous  bool   `json:"isAnonymous"`
		LikeCount    int    `json:"likeCount"`
		Ratings      []struct {
			Topic string  `json:"topic"`
			Score float64 `json:"score"`
		} `json:"ratings"`
		Replies []json.RawMessage `json:"replies"`
	} `json:"data"`
}

// fpTopicOverall is the blended rating foodpanda attaches to every review
// alongside the real per-topic scores. It is an aggregate of the others, not a
// topic in its own right, so best/worst ranking excludes it.
const fpTopicOverall = "overall"

type fpTopicStat struct {
	Topic    string  `json:"topic"`
	Average  float64 `json:"average"`
	Count    int     `json:"count"`
	OneStar  int     `json:"one_star"`
	FiveStar int     `json:"five_star"`
	LowShare float64 `json:"low_score_percentage"`
}

type fpDigestView struct {
	VendorCode    string        `json:"vendor_code"`
	ReviewsRead   int           `json:"reviews_read"`
	WithText      int           `json:"reviews_with_text"`
	VendorReplies int           `json:"reviews_with_vendor_reply"`
	Topics        []fpTopicStat `json:"topics"`
	WorstTopic    string        `json:"worst_topic,omitempty"`
	BestTopic     string        `json:"best_topic,omitempty"`
	RecentLow     []string      `json:"recent_low_score_comments,omitempty"`
	Note          string        `json:"note,omitempty"`
}

func newNovelDigestCmd(flags *rootFlags) *cobra.Command {
	var (
		flagVendorCode string
		pages          int
		perPage        int
		country        string
		showLow        int
		lowBelow       float64
	)

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Split a restaurant's blended star rating into per-topic scores so food quality and delivery quality are separated.",
		Long: "Break a restaurant's rating into per-topic averages.\n\n" +
			"foodpanda shows one blended star value; the reviews API actually carries separate\n" +
			"topic scores (overall, restaurant_food, rider and others).\n" +
			"Use this to tell 'the food is bad' apart from 'the delivery is bad'.",
		Example: "  foodpanda-pp-cli digest --vendor-code pk2v --agent",
		// foodpanda returns HTTP 200 with an empty list for unknown vendor codes,
		// so a bad code is indistinguishable from a vendor with no reviews. Inventing
		// a local "not found" heuristic would be an API-specific guess, so the
		// error-path probe is opted out rather than faked.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--vendor-code=pk2v;--pages=1", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "digest")
			}
			code := strings.TrimSpace(flagVendorCode)
			if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
				code = strings.TrimSpace(args[0])
			}
			if code == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a vendor code is required (positional or --vendor-code), e.g. pk2v"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if cliutil.IsDogfoodEnv() && pages > 1 {
				pages = 1
			}

			type agg struct {
				sum          float64
				n, ones, fvs int
				low          int
			}
			topics := map[string]*agg{}
			view := fpDigestView{VendorCode: code}
			lows := make([]string, 0, showLow)

			base := fmt.Sprintf("%s/%s", fpReviewsHost(country), code)
			for page := 0; page < pages; page++ {
				raw, err := c.Get(ctx, base, map[string]string{
					"limit":  strconv.Itoa(perPage),
					"offset": strconv.Itoa(page * perPage),
				})
				if err != nil {
					return fmt.Errorf("fetching reviews page %d: %w", page+1, err)
				}
				var env fpReviewsEnvelope
				if err := json.Unmarshal(raw, &env); err != nil {
					return fmt.Errorf("parsing reviews page %d: %w", page+1, err)
				}
				if len(env.Data) == 0 {
					break
				}
				for _, r := range env.Data {
					view.ReviewsRead++
					txt := cliutil.CleanText(r.Text)
					if txt != "" {
						view.WithText++
					}
					if len(r.Replies) > 0 {
						view.VendorReplies++
					}
					worst := 0.0
					seen := false
					for _, rt := range r.Ratings {
						a := topics[rt.Topic]
						if a == nil {
							a = &agg{}
							topics[rt.Topic] = a
						}
						a.sum += rt.Score
						a.n++
						switch {
						case rt.Score <= 1:
							a.ones++
						case rt.Score >= 5:
							a.fvs++
						}
						if rt.Score < lowBelow {
							a.low++
						}
						if !seen || rt.Score < worst {
							worst, seen = rt.Score, true
						}
					}
					if seen && worst < lowBelow && txt != "" && len(lows) < showLow {
						lows = append(lows, fmt.Sprintf("[%.0f] %s", worst, truncate(txt, 90)))
					}
				}
				if len(env.Data) < perPage {
					break
				}
			}

			if view.ReviewsRead == 0 {
				view.Note = fmt.Sprintf("no reviews returned for %s; the vendor may be new or the code may belong to another market", code)
				view.Topics = make([]fpTopicStat, 0)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}

			stats := make([]fpTopicStat, 0, len(topics))
			for name, a := range topics {
				if a.n == 0 {
					continue
				}
				stats = append(stats, fpTopicStat{
					Topic: name, Average: fpRound2(a.sum / float64(a.n)), Count: a.n,
					OneStar: a.ones, FiveStar: a.fvs,
					LowShare: fpRound2(float64(a.low) * 100 / float64(a.n)),
				})
			}
			sort.SliceStable(stats, func(i, j int) bool { return stats[i].Average < stats[j].Average })
			view.Topics = stats
			view.RecentLow = lows
			// best/worst rank real topics only. "overall" is the blended score
			// every review already carries — the exact number this command
			// exists to decompose — so letting it compete with its own
			// components makes worst_topic report "overall" and tells the user
			// nothing about whether the food or the rider was the problem.
			// It stays in Topics; it just cannot win or lose the ranking.
			ranked := make([]fpTopicStat, 0, len(stats))
			for _, s := range stats {
				if s.Topic == fpTopicOverall {
					continue
				}
				ranked = append(ranked, s)
			}
			if len(ranked) > 0 {
				view.WorstTopic = ranked[0].Topic
				view.BestTopic = ranked[len(ranked)-1].Topic
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s — %d reviews read (%d with text, %d answered by the vendor)\n\n",
				code, view.ReviewsRead, view.WithText, view.VendorReplies)
			out := make([][]string, 0, len(stats))
			for _, s := range stats {
				out = append(out, []string{
					s.Topic, fmt.Sprintf("%.2f", s.Average), fmt.Sprintf("%d", s.Count),
					fmt.Sprintf("%d", s.OneStar), fmt.Sprintf("%d", s.FiveStar),
					fmt.Sprintf("%.0f%%", s.LowShare),
				})
			}
			if err := flags.printTable(cmd,
				[]string{"TOPIC", "AVG", "N", "1-STAR", "5-STAR", "LOW %"}, out); err != nil {
				return err
			}
			if len(lows) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nRecent low-score comments:")
				for _, l := range lows {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", l)
				}
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&pages, "pages", 3, "Review pages to read")
	cmd.Flags().IntVar(&perPage, "per-page", 30, "Reviews per page")
	cmd.Flags().StringVar(&country, "country", "pk", "Market code: pk, bd, sg, my, hk, th")
	cmd.Flags().IntVar(&showLow, "show-low", 5, "Low-score comments to surface")
	cmd.Flags().Float64Var(&lowBelow, "low-below", 3, "Scores strictly below this count as low")
	cmd.Flags().StringVar(&flagVendorCode, "vendor-code", "", "Vendor code (alternative to the positional argument)")
	return cmd
}
