// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live

package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelPronCheckCmd(flags *rootFlags) *cobra.Command {
	var flagLang string
	var flagDict string
	var flagModel string

	cmd := &cobra.Command{
		Use:         "pron-check [term]",
		Short:       "Verify a term's TTS pronunciation via a speech round-trip (TTS then STT)",
		Example:     "  sarvam-pp-cli pron-check SarvamPay --lang hi-IN",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "term=SarvamPay;--lang=hi-IN", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "pron-check")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("missing required positional argument: the term to check"))
			}
			term := args[0]
			if flagLang == "" {
				flagLang = "hi-IN"
			}
			if flagModel == "" {
				flagModel = "bulbul:v3"
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Step 1: synthesize the term with TTS (optionally with dict_id).
			ttsBody := map[string]any{
				"text":          term,
				"language_code": flagLang,
				"model":         flagModel,
				"output_audio_codec": "wav",
			}
			if flagDict != "" {
				ttsBody["dict_id"] = flagDict
			}
			data, _, err := c.PostWithParams(ctx, "/text-to-speech", nil, ttsBody)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var ttsResp struct {
				Audios []string `json:"audios"`
			}
			if err := json.Unmarshal(data, &ttsResp); err != nil || len(ttsResp.Audios) == 0 {
				return apiErr(fmt.Errorf("TTS returned no audio for %q", term))
			}
			audioBytes, err := base64.StdEncoding.DecodeString(ttsResp.Audios[0])
			if err != nil {
				return apiErr(fmt.Errorf("decoding TTS audio: %w", err))
			}

			// Step 2: transcribe the generated audio back with STT.
			tmpFile, err := os.CreateTemp("", "pron-check-*.wav")
			if err != nil {
				return fmt.Errorf("creating temp audio: %w", err)
			}
			tmpPath := tmpFile.Name()
			defer os.Remove(tmpPath)
			if _, err := tmpFile.Write(audioBytes); err != nil {
				_ = tmpFile.Close()
				return fmt.Errorf("writing temp audio: %w", err)
			}
			if err := tmpFile.Close(); err != nil {
				return fmt.Errorf("closing temp audio: %w", err)
			}
			sttData, _, err := c.PostMultipart(ctx, "/speech-to-text", map[string]string{
				"language_code": flagLang,
				"model":         "saaras:v3",
			}, map[string]string{"file": tmpPath})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var sttResp struct {
				Transcript string `json:"transcript"`
			}
			if err := json.Unmarshal(sttData, &sttResp); err != nil {
				return apiErr(fmt.Errorf("parsing STT response: %w", err))
			}

			// Step 3: normalized comparison.
			norm := func(s string) string {
				s = strings.ToLower(strings.TrimSpace(s))
				s = strings.NewReplacer(
					"।", "", ".", "", ",", "", "?", "", "!", "",
					"  ", " ", "\t", " ",
				).Replace(s)
				return strings.Join(strings.Fields(s), " ")
			}
			spoken := norm(sttResp.Transcript)
			expected := norm(term)
			matched := spoken == expected || strings.Contains(spoken, expected) || strings.Contains(expected, spoken)

			result := map[string]any{
				"term":             term,
				"language":         flagLang,
				"dict_id":          flagDict,
				"tts_transcript":   sttResp.Transcript,
				"pronunciation_ok": matched,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if matched {
				fmt.Fprintf(cmd.OutOrStdout(), "OK: %q was spoken as %q\n", term, sttResp.Transcript)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "MISMATCH: %q was spoken as %q\n", term, sttResp.Transcript)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagLang, "lang", "hi-IN", "Language code (BCP-47) for the round-trip")
	cmd.Flags().StringVar(&flagDict, "dict", "", "Pronunciation dictionary ID to apply during synthesis")
	cmd.Flags().StringVar(&flagModel, "model", "bulbul:v3", "TTS model to use")
	return cmd
}
