// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: render a known phrase with a voice and transcribe it back.
// pp:data-source live
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/fishaudio"
	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/wer"
	"github.com/spf13/cobra"
)

// defaultVerifyPhrase is a pangram-style sentence: it exercises most English
// phonemes in one short render, so a clone that mangles a sound has somewhere
// to show it.
const defaultVerifyPhrase = "The quick brown fox jumps over the lazy dog near the riverbank at dawn."

// voiceVerifyView is the JSON contract `voice verify` prints.
type voiceVerifyView struct {
	ModelID    string  `json:"model_id"`
	Phrase     string  `json:"phrase"`
	Transcript string  `json:"transcript"`
	WER        float64 `json:"wer"`
	Verdict    string  `json:"verdict"`
	TTSModel   string  `json:"tts_model"`
	CostUSD    float64 `json:"cost_usd"`
}

func newNovelVoiceVerifyCmd(flags *rootFlags) *cobra.Command {
	var (
		flagPhrase string
		flagModel  string
		flagKeep   string
	)

	cmd := &cobra.Command{
		Use:   "verify [model_id]",
		Short: "Render a reference phrase with a cloned voice, transcribe it back, and report word-error-rate.",
		Long: `Renders a known phrase with the given voice, transcribes the result, and
scores the transcript against the phrase.

Verdict bands, on word error rate:
  pass   below 0.15
  warn   below 0.30
  fail   at or above 0.30

Run this before swapping a production voice to a new clone. A high rate means
the clone is hard to understand, not that the transcriber was wrong: casing and
punctuation are normalized away before scoring.

This costs one TTS render plus one ASR call. Use 'voice clone' to create a
voice; this command only judges one.`,
		Example: strings.Trim(`
  fish-audio-pp-cli voice verify 7f92f8afb8ec43bf81429cc1c9199cb1
  fish-audio-pp-cli voice verify 7f92f8afb8ec43bf81429cc1c9199cb1 --phrase "Your table for two is ready." --json
  fish-audio-pp-cli voice verify 7f92f8afb8ec43bf81429cc1c9199cb1 --keep ./verify.wav --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"mcp:local-write":     "true",
			"pp:happy-args":       "<model_id>=7f92f8afb8ec43bf81429cc1c9199cb1",
			"pp:typed-exit-codes": "0,2,4,5,7",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "voice verify")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("voice verify needs a voice model_id; run 'voice discover' or 'model list' to find one"))
			}
			modelID := strings.TrimSpace(args[0])
			if modelID == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("voice verify needs a non-empty voice model_id"))
			}
			phrase := strings.TrimSpace(flagPhrase)
			if phrase == "" {
				phrase = defaultVerifyPhrase
			}
			if !fishaudio.ValidReferenceID(modelID) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("model_id %q is not a valid Fish Audio reference id (1-128 chars of A-Z, a-z, 0-9, _ or -)", modelID))
			}
			ttsModel, _, err := fishaudio.ValidateModel(flagModel)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			req := fishaudio.RenderRequest{
				Text:      phrase,
				VoiceID:   modelID,
				Model:     ttsModel,
				Format:    "wav",
				Latency:   "normal",
				Normalize: true,
			}
			audio, _, err := synthesize(ctx, c, req)
			if err != nil {
				return classifyRawAPIError(err)
			}
			if len(audio) == 0 {
				return apiErr(fmt.Errorf("POST %s returned no audio for voice %s", fishTTSPath, modelID))
			}

			audioPath := strings.TrimSpace(flagKeep)
			if audioPath == "" {
				dir, tmpErr := os.MkdirTemp("", "fish-audio-verify-")
				if tmpErr != nil {
					return fmt.Errorf("creating a working directory: %w", tmpErr)
				}
				defer os.RemoveAll(dir)
				audioPath = filepath.Join(dir, "verify.wav")
			}
			if _, err := writeAudioFile(audioPath, audio); err != nil {
				return err
			}

			transcript, err := transcribeFile(ctx, c, audioPath, "", false)
			if err != nil {
				return classifyRawAPIError(err)
			}

			rate := wer.Rate(phrase, transcript.Text)
			ttsCost, _ := fishaudio.TTSCost(len([]byte(phrase)), ttsModel)
			view := voiceVerifyView{
				ModelID:    modelID,
				Phrase:     phrase,
				Transcript: transcript.Text,
				WER:        rate,
				Verdict:    wer.Verdict(rate),
				TTSModel:   ttsModel,
				CostUSD:    ttsCost + fishaudio.ASRCost(transcript.Duration),
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "voice %s: %s (word error rate %.3f)\n", view.ModelID, strings.ToUpper(view.Verdict), view.WER)
			fmt.Fprintf(out, "  phrase:     %s\n", view.Phrase)
			fmt.Fprintf(out, "  transcript: %s\n", view.Transcript)
			fmt.Fprintf(out, "  cost:       $%.6f\n", view.CostUSD)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagPhrase, "phrase", "", "Phrase to render and transcribe; defaults to a phoneme-rich English sentence")
	cmd.Flags().StringVar(&flagModel, "model", "s2.1-pro-free", "TTS model to render the phrase with (one of: s1, s2-pro, s2.1-pro, s2.1-pro-free); defaults to the free model so a fidelity check never spends API credit")
	cmd.Flags().StringVar(&flagKeep, "keep", "", "Keep the rendered audio at this path instead of discarding it")
	return cmd
}
