// Copyright 2026 gregce. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newClipsTranscriptDiffCmd diffs the cut transcript against the uncut one
// for a clip and returns the words that editing removed.
func newClipsTranscriptDiffCmd(flags *rootFlags) *cobra.Command {
	var videoID string
	cmd := &cobra.Command{
		Use:         "transcript-diff <clip-id>",
		Short:       "Diff cut vs uncut transcript for a clip",
		Example:     "  tella-pp-cli clips transcript-diff clp_abc --video vid_xyz --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				_ = cmd.Help()
				return usageErr(fmt.Errorf("missing required positional argument"))
			}
			if dryRunOK(flags) {
				return nil
			}
			if videoID == "" {
				return usageErr(fmt.Errorf("--video <id> is required"))
			}
			clipID := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			cutData, err := c.Get(fmt.Sprintf("/v1/videos/%s/clips/%s/transcript/cut", videoID, clipID), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			uncutData, err := c.Get(fmt.Sprintf("/v1/videos/%s/clips/%s/transcript/uncut", videoID, clipID), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			cutText, _ := extractTranscriptText(cutData)
			uncutText, _ := extractTranscriptText(uncutData)
			cutWords := tokenize(cutText)
			uncutWords := tokenize(uncutText)
			out := diffRemovedWords(uncutWords, cutWords)
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"video_id":      videoID,
				"clip_id":       clipID,
				"removed_words": out,
				"removed_count": len(out),
				"cut_length":    len(cutWords),
				"uncut_length":  len(uncutWords),
			}, flags)
		},
	}
	cmd.Flags().StringVar(&videoID, "video", "", "Video ID the clip belongs to")
	return cmd
}

// removedWord records a single word that was present in uncut but not in
// the corresponding subsequence position of cut. Exported (via its JSON
// tags) as the per-entry element of clips transcript-diff's removed_words
// output.
type removedWord struct {
	Word     string `json:"word"`
	Position int    `json:"position"`
	Context  string `json:"context"`
}

// diffRemovedWords walks uncut and cut with two pointers and returns every
// uncut token that isn't matched (case-insensitively) by the next-unconsumed
// token in cut. A cut transcript is by definition a subsequence of the
// uncut transcript — edits only remove words; they don't insert — so this
// is exact and O(n).
//
// The previous implementation built a multiset bag-count of cut words and
// decremented as it walked uncut. That produced correct removal counts but
// the wrong position field for repeated words: for uncut
// "alpha beta gamma alpha" cut to "beta gamma alpha" (with alpha removed
// at position 0), the bag-count matched the position-3 alpha first and
// reported the removal at position 3. The two-pointer walk preserves
// sequence information and reports position 0 — the actual removal
// boundary that callers (the `removed_words[].position` JSON field) rely
// on for "show me the words deleted near this timestamp" workflows.
func diffRemovedWords(uncutWords, cutWords []string) []removedWord {
	out := []removedWord{}
	j := 0
	for i, w := range uncutWords {
		if j < len(cutWords) && strings.EqualFold(w, cutWords[j]) {
			j++
			continue
		}
		out = append(out, removedWord{
			Word:     w,
			Position: i,
			Context:  contextWindow(uncutWords, i, 3),
		})
	}
	return out
}

// tokenize splits text into lowercase word tokens. Trims punctuation so
// "uh," and "uh." both compare as "uh".
func tokenize(s string) []string {
	out := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\t' || r == ',' || r == '.' || r == '!' || r == '?' || r == ';' || r == ':' || r == '"'
	})
	return out
}

func contextWindow(words []string, i, span int) string {
	start := i - span
	if start < 0 {
		start = 0
	}
	end := i + span + 1
	if end > len(words) {
		end = len(words)
	}
	return strings.Join(words[start:end], " ")
}
