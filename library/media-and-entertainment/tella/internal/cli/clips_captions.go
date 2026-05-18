// Copyright 2026 gregce. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newClipsCaptionsCmd renders a cut transcript as SRT or VTT.
func newClipsCaptionsCmd(flags *rootFlags) *cobra.Command {
	var videoID string
	var format string
	cmd := &cobra.Command{
		Use:         "captions <clip-id>",
		Short:       "Render a clip's cut transcript as SRT or VTT",
		Example:     "  tella-pp-cli clips captions clp_abc --video vid_xyz --format srt",
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
			// Validate --format up front so a typo doesn't waste an API
			// round-trip on a transcript that can't be rendered.
			format = strings.ToLower(format)
			if format != "srt" && format != "vtt" {
				return usageErr(fmt.Errorf("unsupported --format %q: must be srt or vtt", format))
			}
			clipID := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(fmt.Sprintf("/v1/videos/%s/clips/%s/transcript/cut", videoID, clipID), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			cues := extractCues(data)
			if len(cues) == 0 {
				return apiErr(fmt.Errorf("no timed cues found in transcript"))
			}
			out := cmd.OutOrStdout()
			switch format {
			case "vtt":
				fmt.Fprintln(out, "WEBVTT")
				fmt.Fprintln(out)
				for _, cue := range cues {
					fmt.Fprintf(out, "%s --> %s\n", formatTimestamp(cue.StartMS, true), formatTimestamp(cue.EndMS, true))
					fmt.Fprintln(out, cue.Text)
					fmt.Fprintln(out)
				}
			case "srt":
				for i, cue := range cues {
					fmt.Fprintln(out, i+1)
					fmt.Fprintf(out, "%s --> %s\n", formatTimestamp(cue.StartMS, false), formatTimestamp(cue.EndMS, false))
					fmt.Fprintln(out, cue.Text)
					fmt.Fprintln(out)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&videoID, "video", "", "Video ID the clip belongs to")
	cmd.Flags().StringVar(&format, "format", "srt", "Output format: srt or vtt")
	return cmd
}

type captionCue struct {
	StartMS int
	EndMS   int
	Text    string
}

// extractCues turns a Tella transcript response into a list of caption cues.
// Tries `segments`, then groups `words` into cues of ~7 words each so we have
// readable subtitle blocks.
func extractCues(data json.RawMessage) []captionCue {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil
	}
	if segs, ok := obj["segments"].([]any); ok && len(segs) > 0 {
		var out []captionCue
		for _, s := range segs {
			sm, ok := s.(map[string]any)
			if !ok {
				continue
			}
			start := intField(sm, "startTimeMs", "start", "startMs", "begin")
			end := intField(sm, "endTimeMs", "end", "endMs", "stop")
			text, _ := sm["text"].(string)
			if text != "" && end > start {
				out = append(out, captionCue{StartMS: start, EndMS: end, Text: text})
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	// Fall back to grouping words.
	if words, ok := obj["words"].([]any); ok && len(words) > 0 {
		var out []captionCue
		const groupSize = 7
		batch := make([]map[string]any, 0, groupSize)
		flush := func() {
			if len(batch) == 0 {
				return
			}
			start := intField(batch[0], "startTimeMs", "start", "startMs", "begin")
			end := intField(batch[len(batch)-1], "endTimeMs", "end", "endMs", "stop")
			parts := make([]string, 0, len(batch))
			for _, w := range batch {
				for _, k := range []string{"text", "word", "value"} {
					if s, ok := w[k].(string); ok && s != "" {
						parts = append(parts, s)
						break
					}
				}
			}
			text := strings.Join(parts, " ")
			if text != "" && end > start {
				out = append(out, captionCue{StartMS: start, EndMS: end, Text: text})
			}
			batch = batch[:0]
		}
		for _, w := range words {
			wm, ok := w.(map[string]any)
			if !ok {
				continue
			}
			batch = append(batch, wm)
			if len(batch) >= groupSize {
				flush()
			}
		}
		flush()
		return out
	}
	return nil
}

// formatTimestamp renders ms as either SRT (HH:MM:SS,mmm) or VTT (HH:MM:SS.mmm).
func formatTimestamp(ms int, vtt bool) string {
	if ms < 0 {
		ms = 0
	}
	h := ms / 3_600_000
	ms -= h * 3_600_000
	m := ms / 60_000
	ms -= m * 60_000
	s := ms / 1000
	ms -= s * 1000
	sep := ","
	if vtt {
		sep = "."
	}
	return fmt.Sprintf("%02d:%02d:%02d%s%03d", h, m, s, sep, ms)
}
