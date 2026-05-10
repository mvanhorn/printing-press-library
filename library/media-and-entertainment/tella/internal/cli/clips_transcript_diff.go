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
			cutSet := map[string]int{}
			for _, w := range cutWords {
				cutSet[strings.ToLower(w)]++
			}
			type removed struct {
				Word     string `json:"word"`
				Position int    `json:"position"`
				Context  string `json:"context"`
			}
			out := []removed{}
			for i, w := range uncutWords {
				key := strings.ToLower(w)
				if cutSet[key] > 0 {
					cutSet[key]--
					continue
				}
				ctx := contextWindow(uncutWords, i, 3)
				out = append(out, removed{Word: w, Position: i, Context: ctx})
			}
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
