// Phase 3 hand-authored novel command. Not generator-emitted.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/store"

	"github.com/spf13/cobra"
)

type voiceMetrics struct {
	AvgSentenceLengthWords float64 `json:"avg_sentence_length_words"`
	EmDashRate             float64 `json:"em_dash_rate"`
	ColonHookRate          float64 `json:"colon_hook_rate"`
	QuestionOpenerRate     float64 `json:"question_opener_rate"`
	BodyLengthP50Words     int     `json:"body_length_p50_words"`
	BodyLengthP90Words     int     `json:"body_length_p90_words"`
	VocabularyUniqueness   float64 `json:"vocabulary_uniqueness"`
	SampleSize             int     `json:"sample_size"`
}

type voiceDiffEntry struct {
	Metric string  `json:"metric"`
	Self   float64 `json:"self"`
	Other  float64 `json:"other"`
	Delta  float64 `json:"delta"`
}

func newVoiceFingerprintCmd(flags *rootFlags) *cobra.Command {
	var handle string
	var diff string

	cmd := &cobra.Command{
		Use:   "fingerprint",
		Short: "Mechanical voice fingerprint — sentence length, em-dash rate, hook patterns, vocab uniqueness",
		Example: strings.Trim(`
  substack-pp-cli voice fingerprint --json
  substack-pp-cli voice fingerprint --handle alice --json
  substack-pp-cli voice fingerprint --handle alice --diff bob --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), voiceMetrics{}, flags)
			}
			selfBodies, err := loadNoteBodiesForHandle(flags, handle)
			if err != nil {
				return err
			}
			selfMetrics := computeVoiceMetrics(selfBodies)
			if diff == "" {
				if selfMetrics.SampleSize == 0 {
					fmt.Fprintln(cmd.ErrOrStderr(), "no notes found for handle — run 'substack-pp-cli sync' first")
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{}, flags)
				}
				return printJSONFiltered(cmd.OutOrStdout(), selfMetrics, flags)
			}
			otherBodies, err := loadNoteBodiesForHandle(flags, diff)
			if err != nil {
				return err
			}
			other := computeVoiceMetrics(otherBodies)
			// When both fingerprints are empty, every metric is 0 and every
			// delta is 0 — output is indistinguishable from a perfect match.
			// Surface a stderr hint so users see "no signal" instead of "ties".
			if selfMetrics.SampleSize == 0 && other.SampleSize == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no notes found for %q or %q — run 'substack-pp-cli sync' first or check the handles\n", handle, diff)
			} else if selfMetrics.SampleSize == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no notes found for %q (self) — deltas show only %q's metrics\n", handle, diff)
			} else if other.SampleSize == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no notes found for %q (--diff) — deltas show only %q's metrics\n", diff, handle)
			}
			return printJSONFiltered(cmd.OutOrStdout(), buildDiff(selfMetrics, other), flags)
		},
	}
	cmd.Flags().StringVar(&handle, "handle", "", "Handle to fingerprint (default: self)")
	cmd.Flags().StringVar(&diff, "diff", "", "Compare against another handle's fingerprint")
	return cmd
}

func loadNoteBodiesForHandle(flags *rootFlags, handle string) ([]string, error) {
	st, err := store.Open(defaultDBPath("substack-pp-cli"))
	if err != nil {
		return nil, nil
	}
	defer st.Close()
	rows, err := st.DB().Query(`SELECT data FROM resources WHERE resource_type = 'notes'`)
	if err != nil {
		if isMissingTableErr(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	var bodies []string
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(data), &raw); err != nil {
			continue
		}
		if handle != "" {
			h := stringField(raw, "handle", "author_handle", "user_handle")
			// Filter strictly by handle: rows without a handle field are
			// excluded from a handle-scoped fingerprint to avoid bleed
			// from unattributed notes.
			if h != handle {
				continue
			}
		}
		body := stringField(raw, "body", "body_text", "text")
		if body == "" {
			continue
		}
		bodies = append(bodies, body)
	}
	return bodies, nil
}

func computeVoiceMetrics(bodies []string) voiceMetrics {
	if len(bodies) == 0 {
		return voiceMetrics{}
	}
	totalSent := 0
	totalSentWords := 0
	totalChars := 0
	totalEmDashes := 0
	colonHooks := 0
	qOpeners := 0
	totalWords := 0
	uniqueWords := map[string]bool{}
	wordCounts := []int{}
	for _, body := range bodies {
		totalChars += len([]byte(body))
		totalEmDashes += strings.Count(body, "—")
		sentences := splitSentences(body)
		totalSent += len(sentences)
		if len(sentences) > 0 {
			first := strings.TrimSpace(sentences[0])
			if strings.HasSuffix(first, ":") {
				colonHooks++
			}
			lower := strings.ToLower(first)
			for _, w := range []string{"who ", "what ", "why ", "when ", "where ", "how ", "which "} {
				if strings.HasPrefix(lower, w) {
					qOpeners++
					break
				}
			}
		}
		wc := 0
		for _, s := range sentences {
			words := strings.Fields(s)
			totalSentWords += len(words)
			wc += len(words)
			for _, w := range words {
				lw := strings.ToLower(strings.Trim(w, ".,!?:;'\""))
				if lw != "" {
					uniqueWords[lw] = true
					totalWords++
				}
			}
		}
		wordCounts = append(wordCounts, wc)
	}
	sort.Ints(wordCounts)

	avgSent := 0.0
	if totalSent > 0 {
		avgSent = float64(totalSentWords) / float64(totalSent)
	}
	emDashRate := 0.0
	if totalChars > 0 {
		emDashRate = float64(totalEmDashes) / float64(totalChars) * 1000
	}
	colonRate := float64(colonHooks) / float64(len(bodies))
	qRate := float64(qOpeners) / float64(len(bodies))
	uniqRatio := 0.0
	if totalWords > 0 {
		uniqRatio = float64(len(uniqueWords)) / float64(totalWords)
	}

	p50 := percentile(wordCounts, 50)
	p90 := percentile(wordCounts, 90)

	return voiceMetrics{
		AvgSentenceLengthWords: round2(avgSent),
		EmDashRate:             round2(emDashRate),
		ColonHookRate:          round2(colonRate),
		QuestionOpenerRate:     round2(qRate),
		BodyLengthP50Words:     p50,
		BodyLengthP90Words:     p90,
		VocabularyUniqueness:   round2(uniqRatio),
		SampleSize:             len(bodies),
	}
}

func splitSentences(text string) []string {
	// Naive split on .!? followed by space or end. Good enough for short Notes.
	var out []string
	start := 0
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '.' || c == '!' || c == '?' {
			out = append(out, strings.TrimSpace(text[start:i+1]))
			start = i + 1
		}
	}
	if start < len(text) {
		tail := strings.TrimSpace(text[start:])
		if tail != "" {
			out = append(out, tail)
		}
	}
	return out
}

func percentile(sorted []int, p int) int {
	if len(sorted) == 0 {
		return 0
	}
	idx := (p * (len(sorted) - 1)) / 100
	return sorted[idx]
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}

func buildDiff(self, other voiceMetrics) []voiceDiffEntry {
	pairs := []struct {
		name        string
		self, other float64
	}{
		{"avg_sentence_length_words", self.AvgSentenceLengthWords, other.AvgSentenceLengthWords},
		{"em_dash_rate", self.EmDashRate, other.EmDashRate},
		{"colon_hook_rate", self.ColonHookRate, other.ColonHookRate},
		{"question_opener_rate", self.QuestionOpenerRate, other.QuestionOpenerRate},
		{"body_length_p50_words", float64(self.BodyLengthP50Words), float64(other.BodyLengthP50Words)},
		{"body_length_p90_words", float64(self.BodyLengthP90Words), float64(other.BodyLengthP90Words)},
		{"vocabulary_uniqueness", self.VocabularyUniqueness, other.VocabularyUniqueness},
	}
	out := make([]voiceDiffEntry, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, voiceDiffEntry{Metric: p.name, Self: p.self, Other: p.other, Delta: round2(p.self - p.other)})
	}
	return out
}
