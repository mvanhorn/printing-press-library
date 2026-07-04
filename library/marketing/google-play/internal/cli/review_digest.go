// Hand-authored transcendence command: mechanical review aggregation.
package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-play/internal/store"
)

type termCount struct {
	Term  string `json:"term"`
	Count int    `json:"count"`
}

type reviewDigestView struct {
	AppID         string         `json:"appId"`
	Total         int            `json:"total"`
	MeanScore     float64        `json:"meanScore"`
	StarHistogram map[string]int `json:"starHistogram"`
	ByVersion     map[string]int `json:"byVersion"`
	ReplyRate     float64        `json:"replyRate"`
	TopTerms      []termCount    `json:"topTerms"`
	SinceVersion  string         `json:"sinceVersion,omitempty"`
	Note          string         `json:"note,omitempty"`
}

// pp:data-source local
func newNovelReviewDigestCmd(flags *rootFlags) *cobra.Command {
	var sinceVersion string
	cmd := &cobra.Command{
		Use:   "review-digest <appId>",
		Short: "Aggregate stored reviews into star/version histograms, reply rate, and complaint-term frequency.",
		Long: "Mechanical (no NLP) aggregation over locally-stored reviews: per-star and per-version rating histograms, " +
			"developer reply rate, and the most frequent complaint terms. Reads reviews written by the 'reviews' command; " +
			"run 'reviews <appId>' first to populate the local store.\n\n" +
			"Use this command for mechanical review stats. For raw individual reviews use 'reviews'; for a prose summary, pipe this output to an LLM.",
		Example: "  google-play-pp-cli review-digest com.dreamgames.royalkingdom --agent",
		Args:    cobra.ArbitraryArgs,
		// Reads local reviews; any appId with none stored returns a valid
		// empty-state digest, which is not a bad-input error.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would aggregate stored reviews")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("appId is required"))
			}
			db := resolveDBFlag(cmd)
			hint := fmt.Sprintf("run: google-play-pp-cli reviews %s --limit 200   (to populate the local store)", args[0])
			if !dbFileExists(db) {
				hintStderr(cmd, db, hint)
				v := digestReviews(args[0], nil, sinceVersion)
				v.Note = "no reviews stored for this app yet; " + hint
				return emit(cmd, flags, v)
			}
			s, err := openStoreFor(cmd.Context(), db)
			if err != nil {
				return apiErr(err)
			}
			defer s.Close()
			reviews, err := s.StoredReviews(cmd.Context(), args[0])
			if err != nil {
				return apiErr(err)
			}
			view := digestReviews(args[0], reviews, sinceVersion)
			if view.Total == 0 {
				view.Note = "no reviews stored for this app yet; " + hint
			}
			return emit(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&sinceVersion, "since-version", "", "Only include reviews whose recorded app version matches this value")
	cmd.Flags().String("db", "", "Local snapshot database path")
	return cmd
}

// stopWords are excluded from complaint-term frequency.
var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "you": true, "this": true, "that": true, "with": true,
	"are": true, "but": true, "not": true, "have": true, "was": true, "its": true, "it's": true,
	"game": true, "app": true, "play": true, "get": true, "can": true, "all": true, "very": true,
	"just": true, "out": true, "too": true, "now": true, "they": true, "your": true, "them": true,
	"when": true, "what": true, "from": true, "has": true, "had": true, "will": true, "would": true,
	"there": true, "their": true, "been": true, "more": true, "some": true, "than": true, "then": true,
}

func digestReviews(appID string, reviews []store.ReviewRow, sinceVersion string) reviewDigestView {
	v := reviewDigestView{
		AppID:         appID,
		StarHistogram: map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0},
		ByVersion:     map[string]int{},
		TopTerms:      []termCount{},
		SinceVersion:  sinceVersion,
	}
	var scoreSum, replyCount int
	terms := map[string]int{}
	for _, r := range reviews {
		if sinceVersion != "" && r.Version != sinceVersion {
			continue
		}
		v.Total++
		scoreSum += r.Score
		if r.Score >= 1 && r.Score <= 5 {
			v.StarHistogram[fmt.Sprintf("%d", r.Score)]++
		}
		if r.Version != "" {
			v.ByVersion[r.Version]++
		}
		if r.Reply {
			replyCount++
		}
		// complaint-term frequency from low-star reviews only (<=3)
		if r.Score <= 3 {
			for _, w := range tokenize(r.Text) {
				if len(w) >= 4 && !stopWords[w] {
					terms[w]++
				}
			}
		}
	}
	if v.Total > 0 {
		v.MeanScore = round2(float64(scoreSum) / float64(v.Total))
		v.ReplyRate = round2(float64(replyCount) / float64(v.Total))
	}
	v.TopTerms = topTerms(terms, 15)
	return v
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '\''
	})
	return fields
}

func topTerms(terms map[string]int, n int) []termCount {
	out := make([]termCount, 0, len(terms))
	for t, c := range terms {
		if c >= 2 {
			out = append(out, termCount{Term: t, Count: c})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Term < out[j].Term
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
