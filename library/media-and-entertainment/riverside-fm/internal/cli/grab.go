// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// newGrabCmd returns the priority-fallback grab command:
// transcript first, then audio tracks, then HLS video for a given session ID.
// Exits with a code naming the tier reached: 0 transcript, 11 audio, 12 video, 13 nothing.
func newGrabCmd(flags *rootFlags) *cobra.Command {
	var outDir string
	var format string

	cmd := &cobra.Command{
		Use:   "grab <session-id>",
		Short: "Priority-fallback download: transcript first, then audio tracks, then HLS video.",
		Long: `Tries Riverside endpoints in priority order for one take session ID:
  1. transcript  (/api/v4/transcriptions/editableWithVoiceActivity/{sessionId})
  2. audio       (/api/v4/take/{sessionId}/assets — extracts per-participant audio track filenames)
  3. video       (/api/v4/vod/{sessionId}/{participantHandle} — HLS manifest per participant)

Writes the first tier that exists to disk (transcript text or JSON, asset metadata, manifest body)
and exits with code 0 on success. Exit code 13 means none of the three tiers returned usable data.`,
		Example: `  riverside-fm-pp-cli grab bf487406-af40-4bb4-b7f9-a6b49047b55d
  riverside-fm-pp-cli grab <session-id> --out ./downloads --json`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,4,5,13"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			sessionID := strings.TrimSpace(args[0])
			if sessionID == "" {
				return usageErr(fmt.Errorf("session-id is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if outDir == "" {
				outDir = "."
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return err
			}

			// Tier 1: transcript
			tPath := "/api/v4/transcriptions/editableWithVoiceActivity/" + url.PathEscape(sessionID)
			tData, tErr := c.Get(tPath, nil)
			if tErr == nil && hasTranscriptContent(tData) {
				result := map[string]any{"tier": "transcript", "session_id": sessionID}
				if format == "json" || flags.asJSON {
					return writeGrabResult(cmd, flags, outDir, sessionID, "transcript.json", tData, result)
				}
				txt := transcriptToPlainText(tData)
				return writeGrabResult(cmd, flags, outDir, sessionID, "transcript.txt", []byte(txt), result)
			}

			// Tier 2: audio (via take assets)
			aPath := "/api/v4/take/" + url.PathEscape(sessionID) + "/assets"
			aData, aErr := c.Get(aPath, nil)
			if aErr == nil && hasAudioTracks(aData) {
				result := map[string]any{"tier": "audio", "session_id": sessionID}
				return writeGrabResult(cmd, flags, outDir, sessionID, "take-assets.json", aData, result)
			}

			// Tier 3: video — need a participant handle; pull from take assets if we have it
			if aErr == nil {
				handles := extractParticipantHandles(aData)
				if len(handles) > 0 {
					vPath := "/api/v4/vod/" + url.PathEscape(sessionID) + "/" + url.PathEscape(handles[0])
					vData, vErr := c.Get(vPath, nil)
					if vErr == nil && len(vData) > 0 {
						result := map[string]any{
							"tier":        "video",
							"session_id":  sessionID,
							"participant": handles[0],
							"manifest":    "hls m3u8",
						}
						return writeGrabResult(cmd, flags, outDir, sessionID, handles[0]+"-manifest.m3u8", vData, result)
					}
				}
			}

			// All tiers failed
			if flags.asJSON {
				fmt.Fprintln(cmd.OutOrStdout(), `{"tier":"none","session_id":"`+sessionID+`","error":"no transcript, audio, or video available for this session"}`)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "No transcript, audio assets, or video manifest available for session "+sessionID)
			}
			return &cliError{code: 13, err: fmt.Errorf("nothing available for session %s", sessionID)}
		},
	}

	cmd.Flags().StringVar(&outDir, "out", ".", "Output directory for downloaded files")
	cmd.Flags().StringVar(&format, "format", "auto", "Transcript format: auto (txt for human, json for --json), txt, json")
	return cmd
}

func hasTranscriptContent(data json.RawMessage) bool {
	var tr transcriptResponse
	if json.Unmarshal(data, &tr) != nil {
		return false
	}
	if !tr.Success {
		return false
	}
	// Real transcript: at least one speaker has at least one sentence with words.
	for _, sp := range tr.Data.Speakers {
		for _, sent := range sp.Sentences {
			if len(sent.Words) > 0 {
				return true
			}
		}
	}
	// Fall back to voiceActivity segments — older shape might lack sentences but have voice activity.
	for _, sp := range tr.Data.VoiceActivity.Speakers {
		if len(sp.Segments) > 0 {
			return true
		}
	}
	return false
}

func hasAudioTracks(data json.RawMessage) bool {
	var probe struct {
		Take struct {
			Tracks []struct {
				Type     string `json:"type"`
				Status   string `json:"status"`
				Filename string `json:"filename"`
			} `json:"tracks"`
		} `json:"take"`
	}
	if json.Unmarshal(data, &probe) != nil {
		return false
	}
	for _, t := range probe.Take.Tracks {
		if t.Status == "done" && t.Filename != "" {
			return true
		}
	}
	return false
}

func extractParticipantHandles(data json.RawMessage) []string {
	var probe struct {
		Take struct {
			Tracks []struct {
				ArchiveID string `json:"archiveId"`
			} `json:"tracks"`
		} `json:"take"`
	}
	if json.Unmarshal(data, &probe) != nil {
		return nil
	}
	seen := map[string]bool{}
	out := []string{}
	for _, t := range probe.Take.Tracks {
		if t.ArchiveID != "" && !seen[t.ArchiveID] {
			seen[t.ArchiveID] = true
			out = append(out, t.ArchiveID)
		}
	}
	return out
}

func writeGrabResult(cmd *cobra.Command, flags *rootFlags, outDir, sessionID, filename string, body []byte, result map[string]any) error {
	path := filepath.Join(outDir, sessionID+"-"+filename)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	result["path"] = path
	result["bytes"] = len(body)

	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		j, _ := json.MarshalIndent(result, "", "  ")
		_, _ = io.WriteString(cmd.OutOrStdout(), string(j)+"\n")
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "tier=%s session=%s wrote=%s (%d bytes)\n", result["tier"], sessionID, path, len(body))
	}
	return nil
}

// transcriptToPlainText converts the editableWithVoiceActivity JSON to plain text
// with one paragraph per speaker turn. Reuses sentencesToFlat from transcripts.go.
func transcriptToPlainText(data json.RawMessage) string {
	var tr transcriptResponse
	if json.Unmarshal(data, &tr) != nil {
		return string(data)
	}
	segs := sentencesToFlat(tr)
	if len(segs) == 0 {
		return string(data)
	}
	var sb strings.Builder
	prev := ""
	for _, s := range segs {
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

// Keep http package import for future asset-stream support.
var _ = http.StatusOK
