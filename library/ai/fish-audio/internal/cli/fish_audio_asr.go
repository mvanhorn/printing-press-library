// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: transcribe an audio file and price the call.
// pp:data-source live
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/fishaudio"
	"github.com/spf13/cobra"
)

// fishASRPath is the transcription endpoint.
const fishASRPath = "/v1/asr"

// asrSegment is one timed span of a transcript.
type asrSegment struct {
	Text  string  `json:"text"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// asrResponse is the documented shape of a successful POST /v1/asr.
type asrResponse struct {
	Text         string       `json:"text"`
	Duration     float64      `json:"duration"`
	Segments     []asrSegment `json:"segments"`
	LanguageCode string       `json:"language_code"`
	Language     string       `json:"language"`
}

// transcribeManifest is the JSON contract `asr transcribe` prints: the
// transcript plus what the call cost, which the raw endpoint does not report.
type transcribeManifest struct {
	File            string       `json:"file"`
	Text            string       `json:"text"`
	DurationSeconds float64      `json:"duration_seconds"`
	LanguageCode    string       `json:"language_code,omitempty"`
	Language        string       `json:"language,omitempty"`
	CostUSD         float64      `json:"cost_usd"`
	Segments        []asrSegment `json:"segments"`
}

// asrRetryWait is the backoff before the single 503 retry. The vendor's ASR
// endpoint returns intermittent 503s under load; one paced retry clears most
// of them without turning a transient blip into a retry storm.
const asrRetryWait = 2 * time.Second

// transcribeFile posts one audio file to /v1/asr and decodes the response. A
// 503 is retried once after a pause, because that status is a known transient
// on this endpoint rather than a real failure.
func transcribeFile(ctx context.Context, c *client.Client, path, language string, timestamps bool) (asrResponse, error) {
	audio, err := readUploadFile(path)
	if err != nil {
		return asrResponse{}, err
	}
	fields := []multipartField{
		// The API flag is inverted relative to the CLI flag: it names what to
		// ignore, not what to return. --timestamps reads the way a caller
		// thinks about it.
		{Name: "ignore_timestamps", Value: strconv.FormatBool(!timestamps)},
	}
	if strings.TrimSpace(language) != "" {
		fields = append(fields, multipartField{Name: "language", Value: language})
	}
	body, contentType, err := buildMultipart(fields, []multipartFile{{
		Name:     "audio",
		FileName: filepath.Base(path),
		Content:  audio,
	}})
	if err != nil {
		return asrResponse{}, err
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		data, _, postErr := c.PostRaw(ctx, fishASRPath, body, map[string]string{"Content-Type": contentType})
		if postErr == nil {
			var resp asrResponse
			if len(data) == 0 {
				return asrResponse{}, nil
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return asrResponse{}, fmt.Errorf("parsing the %s response: %w", fishASRPath, err)
			}
			return resp, nil
		}
		lastErr = postErr
		var upstream *client.APIError
		if !As(postErr, &upstream) || upstream.StatusCode != 503 || attempt == 1 {
			break
		}
		select {
		case <-ctx.Done():
			return asrResponse{}, ctx.Err()
		case <-time.After(asrRetryWait):
		}
	}
	return asrResponse{}, lastErr
}

// newFishAsrTranscribeCmd builds `asr transcribe`, the named form of the
// generated bare `asr` endpoint command plus the cost the vendor does not
// return and the 503 retry the endpoint needs.
func newFishAsrTranscribeCmd(flags *rootFlags) *cobra.Command {
	var (
		flagAudio      string
		flagLanguage   string
		flagTimestamps bool
	)

	cmd := &cobra.Command{
		Use:   "transcribe",
		Short: "Transcribe an audio file and report what the call cost",
		Long: `Transcribes an audio file and prints the text, the detected language, the
audio duration, and the computed cost.

--timestamps is the positive form of the API's ignore_timestamps field, which
defaults to timestamps OFF. Pass it to get per-segment start and end times.

Cost is $0.36 per hour of audio, billed on the duration rounded to the nearest
second, computed from the duration the API returns.

The endpoint returns intermittent 503s under load. This command retries once
after a short pause before reporting a failure.`,
		Example: strings.Trim(`
  fish-audio-pp-cli asr transcribe --audio interview.wav
  fish-audio-pp-cli asr transcribe --audio interview.wav --language en --timestamps --json
  fish-audio-pp-cli asr transcribe --audio voicemail.mp3 --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "false",
			"pp:happy-args":       "--audio=sample.wav;--dry-run",
			"pp:typed-exit-codes": "0,2,4,5,7",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "asr transcribe")
			}
			if strings.TrimSpace(flagAudio) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--audio is required: pass the path of the audio file to transcribe"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := transcribeFile(ctx, c, flagAudio, flagLanguage, flagTimestamps)
			if err != nil {
				return classifyRawAPIError(err)
			}
			segments := resp.Segments
			if segments == nil {
				segments = make([]asrSegment, 0)
			}
			manifest := transcribeManifest{
				File:            flagAudio,
				Text:            resp.Text,
				DurationSeconds: resp.Duration,
				LanguageCode:    resp.LanguageCode,
				Language:        resp.Language,
				CostUSD:         fishaudio.ASRCost(resp.Duration),
				Segments:        segments,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), manifest, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s\n", manifest.Text)
			fmt.Fprintf(out, "\n%.2f seconds, language %s, $%.6f\n",
				manifest.DurationSeconds, orDash(manifest.LanguageCode), manifest.CostUSD)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagAudio, "audio", "", "Path of the audio file to transcribe")
	cmd.Flags().StringVar(&flagLanguage, "language", "", "Language hint as an ISO 639-1 code; detection runs regardless")
	cmd.Flags().BoolVar(&flagTimestamps, "timestamps", false, "Return per-segment start and end times, which adds latency on short audio")
	return cmd
}

// orDash renders an empty value as a dash so a human table never shows a blank
// cell that reads as a missing column.
func orDash(v string) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	return v
}
