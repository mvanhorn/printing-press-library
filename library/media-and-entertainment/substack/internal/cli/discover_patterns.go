// Phase 3 hand-authored novel command. Not generator-emitted.

package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/store"

	"github.com/spf13/cobra"
)

type patternRow struct {
	Pattern     string  `json:"pattern"`
	SampleCount int     `json:"sample_count"`
	AvgRestacks float64 `json:"avg_restacks"`
	AvgComments float64 `json:"avg_comments"`
	TopExample  string  `json:"top_example"`
}

type rawInspirationNote struct {
	body     string
	restacks int
	comments int
	likes    int
}

func newDiscoverPatternsCmd(flags *rootFlags) *cobra.Command {
	var niche string
	var sortBy string
	var since string

	cmd := &cobra.Command{
		Use:   "patterns",
		Short: "Mechanically extract hook patterns from inspiration Notes in a niche",
		Example: strings.Trim(`
  substack-pp-cli discover patterns --niche productivity --json
  substack-pp-cli discover patterns --niche productivity --sort comments --since 7d --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), []patternRow{}, flags)
			}
			if strings.TrimSpace(niche) == "" {
				return usageErr(fmt.Errorf("--niche is required"))
			}
			rows, err := computePatterns(flags, niche, sortBy, since)
			if err != nil {
				return err
			}
			if rows == nil {
				rows = []patternRow{}
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "no inspiration_notes found — run 'substack-pp-cli sync' to populate the local store first")
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&niche, "niche", "", "Niche tag to filter by (required)")
	cmd.Flags().StringVar(&sortBy, "sort", "restacks", "Sort field: restacks|comments|likes")
	cmd.Flags().StringVar(&since, "since", "14d", "Lookback window (e.g. 7d, 14d, 30d)")
	return cmd
}

func computePatterns(flags *rootFlags, niche, sortBy, since string) ([]patternRow, error) {
	st, err := store.Open(defaultDBPath("substack-pp-cli"))
	if err != nil {
		return []patternRow{}, nil
	}
	defer st.Close()

	days := parseSinceDays(since)
	rows, err := st.DB().Query(
		`SELECT body, restacks, comments, likes FROM inspiration_notes
		 WHERE niche LIKE ?
		 AND posted_at >= datetime('now', ?)`,
		"%"+niche+"%", fmt.Sprintf("-%d days", days))
	if err != nil {
		if isMissingTableErr(err) {
			return []patternRow{}, nil
		}
		return nil, err
	}
	defer rows.Close()
	var notes []rawInspirationNote
	for rows.Next() {
		var n rawInspirationNote
		if err := rows.Scan(&n.body, &n.restacks, &n.comments, &n.likes); err != nil {
			continue
		}
		notes = append(notes, n)
	}
	if len(notes) == 0 {
		return []patternRow{}, nil
	}
	patternBuckets := map[string][]rawInspirationNote{}
	for _, n := range notes {
		for _, p := range matchPatterns(n.body) {
			patternBuckets[p] = append(patternBuckets[p], n)
		}
	}
	out := []patternRow{}
	for p, group := range patternBuckets {
		var rs, cs int
		topIdx := 0
		topVal := -1
		for i, g := range group {
			rs += g.restacks
			cs += g.comments
			val := g.restacks
			switch sortBy {
			case "comments":
				val = g.comments
			case "likes":
				val = g.likes
			}
			if val > topVal {
				topVal = val
				topIdx = i
			}
		}
		out = append(out, patternRow{
			Pattern:     p,
			SampleCount: len(group),
			AvgRestacks: round2(float64(rs) / float64(len(group))),
			AvgComments: round2(float64(cs) / float64(len(group))),
			TopExample:  truncate(group[topIdx].body, 100),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SampleCount > out[j].SampleCount })
	return out, nil
}

func matchPatterns(body string) []string {
	var out []string
	sentences := splitSentences(body)
	if len(sentences) > 0 {
		first := strings.TrimSpace(sentences[0])
		if strings.HasSuffix(first, ":") {
			out = append(out, "colon_hook")
		}
		lower := strings.ToLower(first)
		for _, w := range []string{"who ", "what ", "why ", "when ", "where ", "how ", "which "} {
			if strings.HasPrefix(lower, w) {
				out = append(out, "question_opener")
				break
			}
		}
	}
	if len(sentences) == 3 {
		out = append(out, "three_sentence_formula")
	}
	if strings.Contains(body, " — ") {
		out = append(out, "em_dash_reframe")
	}
	wordCount := len(strings.Fields(body))
	if wordCount <= 120 && len(sentences) <= 3 {
		out = append(out, "short_punchy")
	}
	return out
}

func parseSinceDays(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 14
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err == nil {
			return n
		}
	}
	n, err := strconv.Atoi(s)
	if err == nil {
		return n
	}
	return 14
}
