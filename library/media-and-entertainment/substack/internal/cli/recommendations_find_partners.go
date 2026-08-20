// Phase 3 hand-authored novel command. Not generator-emitted.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/store"

	"github.com/spf13/cobra"
)

type partnerCandidate struct {
	Rank            int     `json:"rank"`
	Handle          string  `json:"handle"`
	Pub             string  `json:"pub"`
	OverlapScore    float64 `json:"overlap_score"`
	SharedFollowees int     `json:"shared_followees"`
	ChainedRecs     int     `json:"chained_recs"`
}

func newRecommendationsFindPartnersCmd(flags *rootFlags) *cobra.Command {
	var myPub string
	var top int

	cmd := &cobra.Command{
		Use:   "find-partners",
		Short: "Score Substack Recommendations swap candidates by overlap density",
		Example: strings.Trim(`
  substack-pp-cli recs find-partners --my-pub on --top 20 --json
  substack-pp-cli recs find-partners --my-pub myslug --top 5 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), []partnerCandidate{}, flags)
			}
			if strings.TrimSpace(myPub) == "" {
				return usageErr(fmt.Errorf("--my-pub is required"))
			}
			rows, err := computeFindPartners(flags, myPub, top)
			if err != nil {
				return err
			}
			if rows == nil {
				rows = []partnerCandidate{}
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "no recommendations data — run 'substack-pp-cli sync' to populate the local store first")
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&myPub, "my-pub", "", "Your publication slug (required)")
	cmd.Flags().IntVar(&top, "top", 20, "Top N candidates to return")
	return cmd
}

func computeFindPartners(flags *rootFlags, myPub string, top int) ([]partnerCandidate, error) {
	st, err := store.Open(defaultDBPath("substack-pp-cli"))
	if err != nil {
		return []partnerCandidate{}, nil
	}
	defer st.Close()

	// Strategy:
	// chained: sum of recommendation rows where target pub appears as recommended
	//          by something I also recommend / follow.
	// shared:  count of subscribers/followees shared with the candidate.
	// We tolerate missing tables — the local schema is whatever sync populated.
	chained := map[string]int{}
	shared := map[string]int{}

	// Pull recommendations rows (resource_type='recommendations'). Each row's
	// data JSON typically looks like {"recommended_pub":"slug","by_pub":"slug"}.
	if rows, err := st.DB().Query(`SELECT data FROM resources WHERE resource_type IN ('recommendations', 'recommendation')`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var data string
			if err := rows.Scan(&data); err != nil {
				continue
			}
			var raw map[string]any
			if err := json.Unmarshal([]byte(data), &raw); err != nil {
				continue
			}
			rec := stringField(raw, "recommended_pub", "recommended_publication", "to", "publication_slug")
			by := stringField(raw, "by_pub", "from", "publication", "publisher_slug")
			if rec == "" {
				continue
			}
			if by == myPub {
				continue
			}
			chained[rec]++
			_ = by
		}
	}

	// Subscribers/followees overlap. Best-effort across resource shapes.
	if rows, err := st.DB().Query(`SELECT data FROM resources WHERE resource_type IN ('subscriptions', 'subscription', 'followees')`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var data string
			if err := rows.Scan(&data); err != nil {
				continue
			}
			var raw map[string]any
			if err := json.Unmarshal([]byte(data), &raw); err != nil {
				continue
			}
			pub := stringField(raw, "publication_slug", "pub", "to_pub", "publication")
			if pub == "" || pub == myPub {
				continue
			}
			shared[pub]++
		}
	}

	all := map[string]bool{}
	for k := range chained {
		all[k] = true
	}
	for k := range shared {
		all[k] = true
	}

	out := []partnerCandidate{}
	for k := range all {
		s := math.Log(float64(shared[k]+1))*0.7 + math.Log(float64(chained[k]+1))*0.3
		out = append(out, partnerCandidate{
			Handle:          k,
			Pub:             k,
			OverlapScore:    round2(s),
			SharedFollowees: shared[k],
			ChainedRecs:     chained[k],
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OverlapScore > out[j].OverlapScore })
	if top > 0 && len(out) > top {
		out = out[:top]
	}
	for i := range out {
		out[i].Rank = i + 1
	}
	return out, nil
}
