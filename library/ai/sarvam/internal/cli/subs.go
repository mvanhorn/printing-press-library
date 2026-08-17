// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// sttTimestampCue is one subtitle cue derived from a timestamped transcription.
type sttTimestampCue struct {
	Start float64  `json:"start_seconds"`
	End   float64  `json:"end_seconds"`
	Text  string   `json:"text"`
	Words []string `json:"words,omitempty"`
}

func formatSRTTime(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second))
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	ms := int(d.Milliseconds()) % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

func formatVTTTime(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second))
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	ms := int(d.Milliseconds()) % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}

func renderSRT(cues []sttTimestampCue) string {
	var b strings.Builder
	for i, cue := range cues {
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString("\n")
		b.WriteString(formatSRTTime(cue.Start))
		b.WriteString(" --> ")
		b.WriteString(formatSRTTime(cue.End))
		b.WriteString("\n")
		b.WriteString(cue.Text)
		b.WriteString("\n\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func renderVTT(cues []sttTimestampCue) string {
	var b strings.Builder
	b.WriteString("WEBVTT\n\n")
	for _, cue := range cues {
		b.WriteString(formatVTTTime(cue.Start))
		b.WriteString(" --> ")
		b.WriteString(formatVTTTime(cue.End))
		b.WriteString("\n")
		b.WriteString(cue.Text)
		b.WriteString("\n\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func newNovelSubsCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagFormat string
	var flagOutput string

	cmd := &cobra.Command{
		Use:         "subs",
		Short:       "Emit .srt/.vtt subtitles from timestamped transcriptions in local history",
		Example:     "  sarvam-pp-cli subs --from last --format srt --output subtitles.srt",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "subs")
			}
			if flagFormat == "" {
				flagFormat = "srt"
			}
			if flagFormat != "srt" && flagFormat != "vtt" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--format must be srt or vtt"))
			}

			dbPath := defaultDBPath("sarvam-pp-cli")
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: sarvam-pp-cli sync --resources speech-to-text --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]sttTimestampCue, 0), flags)
				}
				return nil
			}
			db, err := openStoreForRead(cmd.Context(), "sarvam-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if db == nil {
				return apiErr(fmt.Errorf("no local transcription history. Run 'sarvam-pp-cli sync --resources speech-to-text' first"))
			}
			defer db.Close()

			if !hintIfUnsynced(cmd, db, "speech-to-text") {
				hintIfStale(cmd, db, "speech-to-text", flags.maxAge)
			}

			// Load the requested transcription (by id, or the latest).
			var raw json.RawMessage
			if flagFrom == "" || flagFrom == "last" {
				items, err := db.List("speech-to-text", 1)
				if err != nil {
					return fmt.Errorf("querying local store: %w", err)
				}
				if len(items) == 0 {
					return notFoundErr(fmt.Errorf("no timestamped transcriptions in local history"))
				}
				raw = items[0]
			} else {
				var err error
				raw, err = db.Get("speech-to-text", flagFrom)
				if err != nil {
					return notFoundErr(fmt.Errorf("transcription %q not found in local history", flagFrom))
				}
			}

			var stt struct {
				Timestamps *struct {
					Words           []string  `json:"words"`
					StartTimeSeconds []float64 `json:"start_time_seconds"`
					EndTimeSeconds   []float64 `json:"end_time_seconds"`
				} `json:"timestamps"`
			}
			if err := json.Unmarshal(raw, &stt); err != nil {
				return apiErr(fmt.Errorf("parsing transcription: %w", err))
			}
			if stt.Timestamps == nil || len(stt.Timestamps.Words) == 0 {
				return notFoundErr(fmt.Errorf("transcription has no timestamps; transcribe with --with-timestamps"))
			}
			n := len(stt.Timestamps.Words)
			cues := make([]sttTimestampCue, 0, n)
			for i := 0; i < n; i++ {
				start, end := 0.0, 0.0
				if i < len(stt.Timestamps.StartTimeSeconds) {
					start = stt.Timestamps.StartTimeSeconds[i]
				}
				if i < len(stt.Timestamps.EndTimeSeconds) {
					end = stt.Timestamps.EndTimeSeconds[i]
				}
				cues = append(cues, sttTimestampCue{
					Start: start,
					End:   end,
					Text:  stt.Timestamps.Words[i],
				})
			}

			var rendered string
			if flagFormat == "vtt" {
				rendered = renderVTT(cues)
			} else {
				rendered = renderSRT(cues)
			}

			if flagOutput != "" && flagOutput != "-" {
				// #nosec G306 -- user-facing subtitle file the caller explicitly requested; 0644 keeps it readable by media tools.
				if err := os.WriteFile(flagOutput, []byte(rendered), 0o644); err != nil {
					return fmt.Errorf("writing %s: %w", flagOutput, err)
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"format":  flagFormat,
					"cues":    cues,
					"output":  flagOutput,
					"content": rendered,
				}, flags)
			}
			if flagOutput != "" && flagOutput != "-" {
				fmt.Fprintf(cmd.OutOrStdout(), "wrote %d cues to %s\n", len(cues), flagOutput)
			} else {
				fmt.Fprint(cmd.OutOrStdout(), rendered)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "last", "Transcription id to use, or 'last' for the most recent")
	cmd.Flags().StringVar(&flagFormat, "format", "srt", "Subtitle format: srt or vtt")
	cmd.Flags().StringVar(&flagOutput, "output", "-", "Output file path, or '-' for stdout")
	return cmd
}
