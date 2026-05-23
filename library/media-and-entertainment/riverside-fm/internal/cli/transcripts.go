// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// newTranscriptsCmd is the parent for the `transcripts` novel subcommands.
// The generator already created `transcriptions` for the absorb-side endpoint;
// `transcripts` (plural, no 'i') is the human-friendly novel surface.
func newTranscriptsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transcripts",
		Short: "Transcript transforms: convert formats and compute talktime from cached voice-activity data.",
	}
	cmd.AddCommand(newTranscriptsConvertCmd(flags))
	cmd.AddCommand(newTranscriptsTalktimeCmd(flags))
	return cmd
}

func newTranscriptsConvertCmd(flags *rootFlags) *cobra.Command {
	var format string
	var outPath string

	cmd := &cobra.Command{
		Use:         "convert <session-id>",
		Short:       "Convert a transcript to vtt | srt | txt | json | md (formats Riverside's UI doesn't natively expose).",
		Example:     "  riverside-fm-pp-cli transcripts convert bf487406-af40-4bb4-b7f9-a6b49047b55d --format vtt --out ep.vtt",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			sid := strings.TrimSpace(args[0])
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/api/v4/transcriptions/editableWithVoiceActivity/"+url.PathEscape(sid), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			out, ferr := convertTranscript(data, format)
			if ferr != nil {
				return usageErr(ferr)
			}
			if outPath != "" {
				if err := os.WriteFile(outPath, []byte(out), 0o644); err != nil {
					return err
				}
				if flags.asJSON {
					fmt.Fprintf(cmd.OutOrStdout(), `{"path":"%s","format":"%s","bytes":%d}`+"\n", outPath, format, len(out))
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "wrote %s (%d bytes, format=%s)\n", outPath, len(out), format)
				}
				return nil
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "txt", "Output format: vtt | srt | txt | json | md")
	cmd.Flags().StringVar(&outPath, "out", "", "Write to this file path (default: stdout)")
	return cmd
}

func newTranscriptsTalktimeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "talktime <session-id>",
		Short:       "Compute per-speaker talktime stats (seconds, %, longest monologue, interrupts) from voice-activity timestamps.",
		Example:     "  riverside-fm-pp-cli transcripts talktime bf487406-af40-4bb4-b7f9-a6b49047b55d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			sid := strings.TrimSpace(args[0])
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/api/v4/transcriptions/editableWithVoiceActivity/"+url.PathEscape(sid), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			stats := computeTalktime(data)
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				j, _ := json.MarshalIndent(stats, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(j))
				return nil
			}
			if len(stats.Speakers) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No voice-activity data for this session (transcript may still be processing or take was too short).")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Session length: %.1fs (%.1f min). Total speech: %.1fs across %d speakers.\n",
				stats.SessionTotalSeconds, stats.SessionTotalSeconds/60, stats.TotalSpeechSeconds, len(stats.Speakers))
			for _, sp := range stats.Speakers {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s: %.1fs (%.1f%%) longest=%.1fs interrupts=%d\n",
					sp.Name, sp.Seconds, sp.Pct, sp.LongestMonologueSec, sp.Interrupts)
			}
			return nil
		},
	}
	return cmd
}

// --- Real Riverside transcript shapes ---
//
// /api/v4/transcriptions/editableWithVoiceActivity/{sessionId} returns:
//   {
//     "success": true,
//     "data": {
//       "speakers": [
//         {
//           "id": "<archiveId>",
//           "archiveId": "<archiveId>",
//           "name": "Damien Stevens",
//           "sentences": [
//             {"index": 0, "words": [["Clearly,", 8024, 770], ["man,", 8794, 451], ...]}
//           ]
//         }
//       ],
//       "voiceActivity": {
//         "speakers": [
//           {
//             "speaker": {"id": "<archiveId>", "archiveId": "<archiveId>", "name": "Damien Stevens"},
//             "segments": [{"start": 8024, "end": 36988}, ...]
//           }
//         ]
//       }
//     }
//   }
//
// All timestamps are MILLISECONDS. Words are [text, start_ms, duration_ms] tuples.
// Segments don't carry text; sentences' words[] do.

type transcriptResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Speakers []struct {
			ID        string           `json:"id"`
			ArchiveID string           `json:"archiveId"`
			Name      string           `json:"name"`
			Sentences []transcriptSent `json:"sentences"`
		} `json:"speakers"`
		VoiceActivity struct {
			Speakers []struct {
				Speaker struct {
					ID        string `json:"id"`
					ArchiveID string `json:"archiveId"`
					Name      string `json:"name"`
				} `json:"speaker"`
				Segments []struct {
					Start int64 `json:"start"`
					End   int64 `json:"end"`
				} `json:"segments"`
			} `json:"speakers"`
		} `json:"voiceActivity"`
	} `json:"data"`
}

type transcriptSent struct {
	Index int               `json:"index"`
	Words []json.RawMessage `json:"words"`
}

// transcriptWord parses one ["text", start_ms, duration_ms] tuple.
type transcriptWord struct {
	Text       string
	StartMS    int64
	DurationMS int64
}

func parseWord(raw json.RawMessage) (transcriptWord, bool) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil || len(arr) < 3 {
		return transcriptWord{}, false
	}
	w := transcriptWord{}
	_ = json.Unmarshal(arr[0], &w.Text)
	_ = json.Unmarshal(arr[1], &w.StartMS)
	_ = json.Unmarshal(arr[2], &w.DurationMS)
	return w, true
}

type flatSeg struct {
	Speaker  string
	StartSec float64
	EndSec   float64
	Text     string
}

// sentencesToFlat joins per-sentence words[] into one paragraph per sentence,
// keyed by the first word's start_ms. Speaker name comes from the parent.
func sentencesToFlat(tr transcriptResponse) []flatSeg {
	var out []flatSeg
	for _, sp := range tr.Data.Speakers {
		for _, sent := range sp.Sentences {
			if len(sent.Words) == 0 {
				continue
			}
			var first, last transcriptWord
			var words []string
			gotFirst := false
			for _, raw := range sent.Words {
				w, ok := parseWord(raw)
				if !ok {
					continue
				}
				if !gotFirst {
					first = w
					gotFirst = true
				}
				last = w
				words = append(words, w.Text)
			}
			if !gotFirst {
				continue
			}
			endMS := last.StartMS + last.DurationMS
			out = append(out, flatSeg{
				Speaker:  sp.Name,
				StartSec: float64(first.StartMS) / 1000.0,
				EndSec:   float64(endMS) / 1000.0,
				Text:     strings.Join(words, " "),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartSec < out[j].StartSec })
	return out
}

func convertTranscript(data json.RawMessage, format string) (string, error) {
	var tr transcriptResponse
	if err := json.Unmarshal(data, &tr); err != nil {
		return "", fmt.Errorf("transcript JSON parse failed: %w", err)
	}
	switch strings.ToLower(format) {
	case "json":
		return string(data), nil
	case "txt":
		return renderTxt(tr), nil
	case "srt":
		return renderSRT(tr), nil
	case "vtt":
		return renderVTT(tr), nil
	case "md":
		return renderMD(tr), nil
	default:
		return "", fmt.Errorf("unsupported format %q (use vtt | srt | txt | json | md)", format)
	}
}

func renderTxt(tr transcriptResponse) string {
	var sb strings.Builder
	prev := ""
	for _, s := range sentencesToFlat(tr) {
		if s.Speaker != prev {
			if prev != "" {
				sb.WriteString("\n")
			}
			sb.WriteString(s.Speaker)
			sb.WriteString(":\n")
			prev = s.Speaker
		}
		sb.WriteString(fmt.Sprintf("[%s] %s\n", formatTimestampHMS(s.StartSec), s.Text))
	}
	return sb.String()
}

func renderMD(tr transcriptResponse) string {
	var sb strings.Builder
	sb.WriteString("# Transcript\n\n")
	prev := ""
	for _, s := range sentencesToFlat(tr) {
		if s.Speaker != prev {
			if prev != "" {
				sb.WriteString("\n\n")
			}
			sb.WriteString(fmt.Sprintf("**%s** _(%s)_\n\n", s.Speaker, formatTimestampHMS(s.StartSec)))
			prev = s.Speaker
		}
		sb.WriteString(s.Text)
		sb.WriteString(" ")
	}
	return sb.String() + "\n"
}

func renderSRT(tr transcriptResponse) string {
	var sb strings.Builder
	for i, s := range sentencesToFlat(tr) {
		sb.WriteString(fmt.Sprintf("%d\n", i+1))
		sb.WriteString(fmt.Sprintf("%s --> %s\n", srtTime(s.StartSec), srtTime(s.EndSec)))
		if s.Speaker != "" {
			sb.WriteString(s.Speaker + ": ")
		}
		sb.WriteString(s.Text)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

func renderVTT(tr transcriptResponse) string {
	var sb strings.Builder
	sb.WriteString("WEBVTT\n\n")
	for _, s := range sentencesToFlat(tr) {
		sb.WriteString(fmt.Sprintf("%s --> %s\n", vttTime(s.StartSec), vttTime(s.EndSec)))
		if s.Speaker != "" {
			sb.WriteString("<v " + s.Speaker + ">")
		}
		sb.WriteString(s.Text)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

func srtTime(sec float64) string {
	h := int(sec / 3600)
	m := int(sec/60) % 60
	s := int(sec) % 60
	ms := int((sec - float64(int(sec))) * 1000)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

func vttTime(sec float64) string {
	h := int(sec / 3600)
	m := int(sec/60) % 60
	s := int(sec) % 60
	ms := int((sec - float64(int(sec))) * 1000)
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}

func formatTimestampHMS(sec float64) string {
	h := int(sec / 3600)
	m := int(sec/60) % 60
	s := int(sec) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

type speakerStat struct {
	Name                string  `json:"speaker"`
	Seconds             float64 `json:"seconds"`
	Pct                 float64 `json:"percent_of_total"`
	LongestMonologueSec float64 `json:"longest_monologue_seconds"`
	Interrupts          int     `json:"interrupts"`
}

type talktimeStats struct {
	SessionTotalSeconds float64       `json:"session_total_seconds"`
	TotalSpeechSeconds  float64       `json:"total_speech_seconds"`
	Speakers            []speakerStat `json:"speakers"`
}

func computeTalktime(data json.RawMessage) talktimeStats {
	var tr transcriptResponse
	if json.Unmarshal(data, &tr) != nil {
		return talktimeStats{}
	}
	// Use voiceActivity.segments for talktime; the segments correspond to continuous speech blocks per speaker.
	var totalSpeechMS int64
	var sessionEndMS int64
	totalsByName := map[string]int64{}
	longestByName := map[string]int64{}

	type segWithSpeaker struct {
		Name  string
		Start int64
		End   int64
	}
	var allSegs []segWithSpeaker

	for _, sp := range tr.Data.VoiceActivity.Speakers {
		name := sp.Speaker.Name
		var longest int64
		var ms int64
		for _, seg := range sp.Segments {
			dur := seg.End - seg.Start
			if dur < 0 {
				dur = 0
			}
			ms += dur
			if dur > longest {
				longest = dur
			}
			if seg.End > sessionEndMS {
				sessionEndMS = seg.End
			}
			allSegs = append(allSegs, segWithSpeaker{Name: name, Start: seg.Start, End: seg.End})
		}
		totalsByName[name] += ms
		if longest > longestByName[name] {
			longestByName[name] = longest
		}
		totalSpeechMS += ms
	}

	// Interrupts = segments whose start overlaps another speaker's still-open segment.
	sort.Slice(allSegs, func(i, j int) bool { return allSegs[i].Start < allSegs[j].Start })
	interruptsByName := map[string]int{}
	for i, s := range allSegs {
		for j := i - 1; j >= 0 && j >= i-6; j-- {
			prev := allSegs[j]
			if prev.Name != s.Name && prev.End > s.Start {
				interruptsByName[s.Name]++
				break
			}
		}
	}

	out := talktimeStats{
		SessionTotalSeconds: float64(sessionEndMS) / 1000.0,
		TotalSpeechSeconds:  float64(totalSpeechMS) / 1000.0,
	}
	for name, totalMS := range totalsByName {
		pct := 0.0
		if totalSpeechMS > 0 {
			pct = float64(totalMS) / float64(totalSpeechMS) * 100
		}
		out.Speakers = append(out.Speakers, speakerStat{
			Name:                name,
			Seconds:             float64(totalMS) / 1000.0,
			Pct:                 pct,
			LongestMonologueSec: float64(longestByName[name]) / 1000.0,
			Interrupts:          interruptsByName[name],
		})
	}
	sort.Slice(out.Speakers, func(i, j int) bool { return out.Speakers[i].Seconds > out.Speakers[j].Seconds })
	return out
}
