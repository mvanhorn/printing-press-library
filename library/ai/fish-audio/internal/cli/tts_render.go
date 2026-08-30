// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: render one line of text to an audio file and record it.
// pp:data-source live
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/fishaudio"
	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/store"
	"github.com/spf13/cobra"
)

// renderOptions collects the flags shared by `tts render` and `tts batch`, so
// the two commands cannot drift on what a render means.
type renderOptions struct {
	model             string
	format            string
	mp3Bitrate        int
	opusBitrate       int
	sampleRate        int
	latency           string
	speed             float64
	volume            float64
	normalizeLoudness bool
	temperature       float64
	topP              float64
	chunkLength       int
}

// registerRenderFlags declares the synthesis flags on a command.
func registerRenderFlags(cmd *cobra.Command, o *renderOptions) {
	cmd.Flags().StringVar(&o.model, "model", fishaudio.DefaultModel, "TTS model sent in the `model` header (one of: s1, s2-pro, s2.1-pro, s2.1-pro-free)")
	cmd.Flags().StringVar(&o.format, "format", "mp3", "Output audio container (one of: mp3, wav, pcm, opus)")
	cmd.Flags().IntVar(&o.mp3Bitrate, "mp3-bitrate", 0, "MP3 bitrate in kbps; applies only when --format mp3")
	cmd.Flags().IntVar(&o.opusBitrate, "opus-bitrate", 0, "Opus bitrate in bps, -1000 for automatic; applies only when --format opus")
	cmd.Flags().IntVar(&o.sampleRate, "sample-rate", 0, "Output sample rate in Hz; 0 keeps the format default")
	cmd.Flags().StringVar(&o.latency, "latency", "normal", "Latency and quality trade-off (one of: normal, balanced, low)")
	cmd.Flags().Float64Var(&o.speed, "speed", 0, "Prosody speaking-rate multiplier; 0 keeps the model default")
	cmd.Flags().Float64Var(&o.volume, "volume", 0, "Prosody volume adjustment in decibels; 0 keeps the model default")
	cmd.Flags().BoolVar(&o.normalizeLoudness, "normalize-loudness", true, "Normalize English and Chinese text for stable numbers; has no effect on s1")
	cmd.Flags().Float64Var(&o.temperature, "temperature", 0, "Expressiveness; higher is more varied, 0 keeps the model default")
	cmd.Flags().Float64Var(&o.topP, "top-p", 0, "Nucleus-sampling diversity; 0 keeps the model default")
	cmd.Flags().IntVar(&o.chunkLength, "chunk-length", 0, "Characters per synthesis chunk; 0 keeps the model default")
}

// resolveRenderOptions validates the closed-set flags and reports the
// deprecation warning, if any, for the chosen model.
func resolveRenderOptions(o *renderOptions) (model string, format string, latency string, warning string, err error) {
	model, warning, err = fishaudio.ValidateModel(o.model)
	if err != nil {
		return "", "", "", "", err
	}
	format, err = fishaudio.ValidateFormat(o.format)
	if err != nil {
		return "", "", "", "", err
	}
	latency, err = fishaudio.ValidateLatency(o.latency)
	if err != nil {
		return "", "", "", "", err
	}
	return model, format, latency, warning, nil
}

// buildRenderRequest assembles the validated render request for one text.
func buildRenderRequest(text, voice, model, format, latency string, o *renderOptions) fishaudio.RenderRequest {
	return fishaudio.RenderRequest{
		Text:        text,
		VoiceID:     voice,
		Model:       model,
		Format:      format,
		MP3Bitrate:  o.mp3Bitrate,
		OpusBitrate: o.opusBitrate,
		SampleRate:  o.sampleRate,
		Latency:     latency,
		Speed:       o.speed,
		Volume:      o.volume,
		Normalize:   o.normalizeLoudness,
		Temperature: o.temperature,
		TopP:        o.topP,
		ChunkLength: o.chunkLength,
	}
}

// manifestFromRow rebuilds the printed manifest from a stored row, used by the
// `--skip-if-rendered` hit path.
func manifestFromRow(row *store.RenderRow, skipped bool) renderManifest {
	return renderManifest{
		ID:               row.ID,
		File:             row.FilePath,
		BytesIn:          row.BytesIn,
		BytesOut:         row.BytesOut,
		SHA256:           row.FileSHA256,
		Model:            row.Model,
		Voice:            row.VoiceID,
		Format:           row.Format,
		CostUSD:          row.CostUSD,
		CostUSDPaidEquiv: row.CostUSDPaidEquiv,
		Skipped:          skipped,
	}
}

func newNovelTtsRenderCmd(flags *rootFlags) *cobra.Command {
	var (
		flagText           string
		flagTextFile       string
		flagVoice          string
		flagOut            string
		flagReferenceAudio string
		flagReferenceText  string
		flagSkipIfRendered bool
		flagPlay           bool
		flagDB             string
		opts               renderOptions
	)

	cmd := &cobra.Command{
		Use:   "render",
		Short: "Hash the request and reuse a prior identical render instead of paying for it again.",
		Long: `Renders one piece of text to an audio file, records it in the local render
log, and prints a manifest naming the file, its size, and what it cost.

Name the voice one of two ways, never both: --voice for a saved voice model, or
--reference-audio to clone a voice inline for this one render.

--skip-if-rendered hashes the text, voice, model, format, and prosody. When
that hash is already in the log and its file is still on disk, no API call is
made: the prior audio is placed at --out and the manifest comes back with
"skipped": true. You always get the file you asked for.

Bracket tags in the text control delivery ([happy], [whispering], [sad] and
other free-text cues). The vendor does not publish a closed list, so treat
them as hints, not an enum.

--format wav is streamed by the API with a zero frame count in the header,
which makes pygame and several browser decoders treat the file as empty. This
command rewrites the RIFF and data sizes before writing the file.`,
		Example: strings.Trim(`
  fish-audio-pp-cli tts render --text "Your table is ready." --voice 7f92f8afb8ec43bf81429cc1c9199cb1 --out greeting.mp3
  fish-audio-pp-cli tts render --text-file script.txt --voice 7f92f8afb8ec43bf81429cc1c9199cb1 --out line.wav --format wav --json
  fish-audio-pp-cli tts render --text "[whispering] Meet me at dawn." --voice 7f92f8afb8ec43bf81429cc1c9199cb1 --out whisper.mp3 --skip-if-rendered --agent
  fish-audio-pp-cli tts render --text "Hello there." --reference-audio sample.wav --reference-text "Sample transcript." --out zeroshot.mp3   # zero-shot: no --voice
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"mcp:local-write":     "true",
			"pp:happy-args":       "--text=Your table is ready.;--voice=7f92f8afb8ec43bf81429cc1c9199cb1;--model=s2.1-pro-free;--out=render.mp3",
			"pp:typed-exit-codes": "0,2,4,5,7",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "tts render")
			}
			// A render names its voice one of two ways: a saved voice model, or
			// inline reference audio for a zero-shot clone. Exactly one of them
			// must be present. Both together is ambiguous, and the wire body
			// would carry reference_id and references at the same time.
			hasVoice := strings.TrimSpace(flagVoice) != ""
			hasReference := strings.TrimSpace(flagReferenceAudio) != ""
			switch {
			case hasVoice && hasReference:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("pass either --voice or --reference-audio, not both: --voice renders with a saved voice model, --reference-audio clones one inline for this render"))
			case !hasVoice && !hasReference:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--voice or --reference-audio is required: pass the model_id of a saved voice, or an audio file to clone from for this render"))
			}
			if strings.TrimSpace(flagOut) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--out is required: pass the path to write the audio file to"))
			}
			text, err := readTextInput(flagText, flagTextFile)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			model, format, latency, warning, err := resolveRenderOptions(&opts)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if warning != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning)
			}

			req := buildRenderRequest(text, flagVoice, model, format, latency, &opts)
			if hasReference {
				// #nosec G304 -- flagReferenceAudio is the operator's own
				// --reference-audio value; reading it is the flag's purpose.
				audio, readErr := os.ReadFile(flagReferenceAudio)
				if readErr != nil {
					return usageErr(fmt.Errorf("reading --reference-audio %s: %w", flagReferenceAudio, readErr))
				}
				// The zero-shot path owns the voice identity: leaving a
				// reference_id alongside references would put two competing
				// voice sources in one body.
				req.VoiceID = ""
				req.ReferenceAudio = audio
				req.ReferenceText = flagReferenceText
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			dbPath := fishRenderDBPath(flagDB)
			db, err := openRenderStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			hash := req.Hash()
			if flagSkipIfRendered {
				prior, lookupErr := db.RenderRowByHash(ctx, hash)
				if lookupErr != nil {
					return lookupErr
				}
				if prior != nil && prior.FilePath != "" {
					if reused, reuseErr := reusePriorRender(prior, flagOut); reuseErr == nil && reused != nil {
						return emitRenderManifest(cmd, flags, *reused)
					} else if reuseErr != nil {
						// A prior row whose file cannot be reproduced at --out is
						// not a failure: fall through and render for real, so the
						// caller still ends up with the file they asked for.
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not reuse the prior render (%v); rendering again\n", reuseErr)
					}
				}
			}

			audio, repaired, err := synthesize(ctx, c, req)
			if err != nil {
				return classifyRawAPIError(err)
			}
			if len(audio) == 0 {
				return apiErr(fmt.Errorf("POST %s returned no audio bytes", fishTTSPath))
			}
			sum, err := writeAudioFile(flagOut, audio)
			if err != nil {
				return err
			}
			cost, paidEquiv := fishaudio.TTSCost(req.BytesIn(), model)
			id, err := db.InsertRenderRow(ctx, store.RenderRow{
				RequestHash:      hash,
				Text:             text,
				Model:            model,
				VoiceID:          renderVoiceLabel(flagVoice, hasReference, flagReferenceAudio),
				Format:           format,
				BytesIn:          int64(req.BytesIn()),
				BytesOut:         int64(len(audio)),
				CostUSD:          cost,
				CostUSDPaidEquiv: paidEquiv,
				FilePath:         flagOut,
				FileSHA256:       sum,
				Source:           "tts render",
			})
			if err != nil {
				return err
			}

			manifest := renderManifest{
				ID:                id,
				File:              flagOut,
				BytesIn:           int64(req.BytesIn()),
				BytesOut:          int64(len(audio)),
				SHA256:            sum,
				Model:             model,
				Voice:             renderVoiceLabel(flagVoice, hasReference, flagReferenceAudio),
				Format:            format,
				CostUSD:           cost,
				CostUSDPaidEquiv:  paidEquiv,
				Skipped:           false,
				WAVHeaderRepaired: repaired,
			}
			if flagPlay {
				if playErr := playAudioFile(cmd, flags, flagOut); playErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", playErr)
				}
			}
			return emitRenderManifest(cmd, flags, manifest)
		},
	}

	cmd.Flags().StringVar(&flagText, "text", "", "Text to render; pass --text-file instead to read it from a file")
	cmd.Flags().StringVar(&flagTextFile, "text-file", "", "File whose whole contents are the text to render")
	cmd.Flags().StringVar(&flagVoice, "voice", "", "Voice model_id to render with, from `voice discover` or `model list`; mutually exclusive with --reference-audio")
	cmd.Flags().StringVar(&flagOut, "out", "", "Path to write the rendered audio file to")
	cmd.Flags().StringVar(&flagReferenceAudio, "reference-audio", "", "Audio file to clone from for this render; replaces --voice and switches the request to MessagePack encoding")
	cmd.Flags().StringVar(&flagReferenceText, "reference-text", "", "Transcript of --reference-audio, which improves zero-shot fidelity")
	cmd.Flags().BoolVar(&flagSkipIfRendered, "skip-if-rendered", false, "Reuse the prior identical render instead of calling the API again")
	cmd.Flags().BoolVar(&flagPlay, "play", false, "Play the rendered file through the operating system audio handler")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	registerRenderFlags(cmd, &opts)
	return cmd
}

// emitRenderManifest prints a manifest through the shared output helpers so
// --json, --select, --compact, and --csv all behave.
func emitRenderManifest(cmd *cobra.Command, flags *rootFlags, manifest renderManifest) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), manifest, flags)
	}
	out := cmd.OutOrStdout()
	state := "rendered"
	if manifest.Skipped {
		state = "reused (skip-if-rendered hit)"
	}
	fmt.Fprintf(out, "%s: %s\n", state, manifest.File)
	fmt.Fprintf(out, "  id %d  model %s  voice %s  format %s\n", manifest.ID, manifest.Model, manifest.Voice, manifest.Format)
	fmt.Fprintf(out, "  %d bytes in, %d bytes out, $%.6f (paid equivalent $%.6f)\n",
		manifest.BytesIn, manifest.BytesOut, manifest.CostUSD, manifest.CostUSDPaidEquiv)
	if manifest.WAVHeaderRepaired {
		fmt.Fprintln(out, "  repaired the streamed WAV frame-count header")
	}
	return nil
}

// playAudioFile hands the rendered file to the operating system audio handler.
// It refuses under any Printing Press harness: a verify or dogfood matrix must
// never make sound on the operator's machine, and curtailing playback would
// make it shorter, not absent.
func playAudioFile(cmd *cobra.Command, flags *rootFlags, path string) error {
	if cliutil.IsAnyHarness() {
		return writeHarnessRefusal(cmd.ErrOrStderr(), flags, "play audio")
	}
	var bin string
	var argv []string
	switch runtime.GOOS {
	case "darwin":
		bin, argv = "afplay", []string{path}
	case "windows":
		bin, argv = "rundll32", []string{"url.dll,FileProtocolHandler", path}
	default:
		bin, argv = "xdg-open", []string{path}
	}
	resolved, err := exec.LookPath(bin)
	if err != nil {
		return fmt.Errorf("cannot play %s: %s is not on PATH", path, bin)
	}
	// #nosec G204 -- bin comes from a fixed per-OS switch and is resolved through
	// exec.LookPath; argv carries the rendered file path as a single argument with
	// no shell in between.
	if err := exec.Command(resolved, argv...).Start(); err != nil {
		return fmt.Errorf("cannot play %s: %w", path, err)
	}
	return nil
}

// renderVoiceLabel names the voice a render used, for the manifest and the log
// row. A zero-shot render has no voice model_id, so the reference file stands
// in: an unlabeled row would be indistinguishable from a render whose voice was
// never recorded.
func renderVoiceLabel(voiceID string, hasReference bool, referencePath string) string {
	if hasReference {
		return "zero-shot:" + filepath.Base(referencePath)
	}
	return voiceID
}

// reusePriorRender satisfies a --skip-if-rendered hit by making sure the audio
// is at outPath, then returning the manifest for it.
//
// Exit 0 with "skipped": true has to mean the file exists. Returning the prior
// row without producing outPath would report success for a file the caller
// cannot open, which is worse than paying for the render again.
//
// It returns (nil, nil) when the prior row's file is gone, which is the signal
// to render for real.
func reusePriorRender(prior *store.RenderRow, outPath string) (*renderManifest, error) {
	audio, err := os.ReadFile(prior.FilePath)
	if err != nil {
		return nil, nil
	}
	manifest := manifestFromRow(prior, true)
	if prior.FilePath == outPath {
		return &manifest, nil
	}
	sum, err := writeAudioFile(outPath, audio)
	if err != nil {
		return nil, err
	}
	manifest.File = outPath
	manifest.SHA256 = sum
	manifest.BytesOut = int64(len(audio))
	return &manifest, nil
}
