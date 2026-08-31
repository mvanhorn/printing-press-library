// Copyright 2026 github-actionsbot and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// beatSheet is the schema this command expects from --beats: a target
// duration in seconds for each positional clip, aligned by index. There is
// no Flow or Scribe-defined "beat sheet" format to conform to -- this is a
// hand-authored local schema, documented here and in --help.
type beatSheet struct {
	Durations []float64 `json:"durations"`
}

func newNovelMuxCmd(flags *rootFlags) *cobra.Command {
	var flagAudio string
	var flagBeats string
	var flagOut string

	cmd := &cobra.Command{
		Use:   "mux <clip.mp4>...",
		Short: "Lay your real audio-drama mp3 back over the rendered Flow clips, in the right order and at the right offsets",
		Long: "Lay your real audio-drama mp3 back over the rendered Flow clips, in the right order and at the right offsets.\n\n" +
			"Flow has no mechanism to import or sync to an existing audio track, so this closes the loop locally with ffmpeg " +
			"(must be installed and on PATH) instead of a manual video-editor pass.\n\n" +
			"--beats, if given, points to a local JSON file: {\"durations\": [12.5, 8.0, 15.2]}, one target duration in " +
			"seconds per clip in the order given on the command line. Each clip is time-stretched (ffmpeg setpts) to fill " +
			"its slot before the clips are concatenated. Without --beats, clips are concatenated at their native durations.",
		Example: "  flow-pp-cli mux shot1.mp4 shot2.mp4 shot3.mp4 --audio episode3.mp3 --beats episode3-beats.json --out episode3-final.mp4",
		// pp:happy-args points the live dogfood matrix at tiny real ffmpeg
		// fixtures (two 1x1s black/blue clips, a 2s silent mp3, a matching
		// beats.json) instead of the Example's illustrative, non-existent
		// filenames -- this is a full fix, no account-specific data needed,
		// pure local ffmpeg processing. pp:typed-exit-codes: presence
		// unlocks a real (non-dry-run) execution attempt for this mutating
		// command instead of the matrix's default dry-run-only happy path;
		// see .printing-press-patches/2026-08-30-happy-args-dogfood-fixtures.md.
		Annotations: map[string]string{
			"mcp:read-only":       "false",
			"pp:happy-args":       "clip1=testdata/dogfood-fixtures/mux/shot1.mp4;clip2=testdata/dogfood-fixtures/mux/shot2.mp4;--audio=testdata/dogfood-fixtures/mux/audio.mp3;--beats=testdata/dogfood-fixtures/mux/beats.json;--out=testdata/dogfood-fixtures/mux/output.mp4",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("mux requires at least one video clip; run %q for usage", cmd.CommandPath()+" --help"))
			}
			if flagAudio == "" {
				return usageErr(fmt.Errorf("--audio is required"))
			}
			if flagOut == "" {
				return usageErr(fmt.Errorf("--out is required"))
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, fmt.Sprintf("mux %d clip(s) with %s -> %s", len(args), flagAudio, flagOut))
			}

			var durations []float64
			if flagBeats != "" {
				raw, err := os.ReadFile(filepath.Clean(flagBeats)) // #nosec G304 -- user-specified beats file is this flag's documented purpose.
				if err != nil {
					return usageErr(fmt.Errorf("reading beats file: %w", err))
				}
				var beats beatSheet
				if err := json.Unmarshal(raw, &beats); err != nil {
					return usageErr(fmt.Errorf("parsing beats file (expected {\"durations\": [seconds, ...]}): %w", err))
				}
				if len(beats.Durations) != len(args) {
					return usageErr(fmt.Errorf("beats file has %d duration(s), but %d clip(s) were given -- these must match 1:1 in order", len(beats.Durations), len(args)))
				}
				durations = beats.Durations
			}

			ffmpegPath, err := exec.LookPath("ffmpeg")
			if err != nil {
				return configErr(fmt.Errorf("ffmpeg not found on PATH -- install ffmpeg to use mux: %w", err))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			var filterParts []string
			var concatLabels strings.Builder
			for i := range args {
				label := fmt.Sprintf("v%d", i)
				if durations != nil {
					srcDuration, err := probeClipDuration(ctx, args[i])
					if err != nil {
						return usageErr(fmt.Errorf("probing duration of %s: %w", args[i], err))
					}
					if srcDuration <= 0 {
						return usageErr(fmt.Errorf("could not determine a positive duration for %s", args[i]))
					}
					scale := durations[i] / srcDuration
					filterParts = append(filterParts, fmt.Sprintf("[%d:v]setpts=%s*PTS[%s]", i, strconv.FormatFloat(scale, 'f', -1, 64), label))
				} else {
					filterParts = append(filterParts, fmt.Sprintf("[%d:v]setpts=PTS-STARTPTS[%s]", i, label))
				}
				concatLabels.WriteString("[" + label + "]")
			}
			filterParts = append(filterParts, fmt.Sprintf("%sconcat=n=%d:v=1:a=0[vout]", concatLabels.String(), len(args)))
			filterComplex := strings.Join(filterParts, ";")

			audioInputIndex := len(args)
			ffmpegArgs := []string{"-y"}
			for _, clip := range args {
				ffmpegArgs = append(ffmpegArgs, "-i", clip)
			}
			ffmpegArgs = append(ffmpegArgs,
				"-i", flagAudio,
				"-filter_complex", filterComplex,
				"-map", "[vout]",
				"-map", fmt.Sprintf("%d:a", audioInputIndex),
				"-shortest",
				"-c:v", "libx264",
				"-c:a", "aac",
				"-pix_fmt", "yuv420p",
				flagOut,
			)

			runCmd := exec.CommandContext(ctx, ffmpegPath, ffmpegArgs...) // #nosec G204 -- ffmpegPath resolved via exec.LookPath (not attacker-controlled); args are locally-composed clip/audio/output paths, no shell involved.
			output, err := runCmd.CombinedOutput()
			if err != nil {
				return apiErr(fmt.Errorf("ffmpeg failed: %w\n%s", err, truncate(string(output), 4000)))
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "muxed %d clip(s) with %s -> %s\n", len(args), flagAudio, flagOut)
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"clips": len(args), "audio": flagAudio, "out": flagOut}, flags)
		},
	}
	cmd.Flags().StringVar(&flagAudio, "audio", "", "Real audio-drama mp3 to lay over the concatenated clips (required)")
	cmd.Flags().StringVar(&flagBeats, "beats", "", "Local beat-sheet JSON ({\"durations\": [seconds, ...]}) to retime each clip before concatenation")
	cmd.Flags().StringVar(&flagOut, "out", "", "Output video file path (required)")
	return cmd
}

func probeClipDuration(ctx context.Context, clip string) (float64, error) {
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		return 0, fmt.Errorf("ffprobe not found on PATH (installed alongside ffmpeg): %w", err)
	}
	cmd := exec.CommandContext(ctx, ffprobePath, "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", clip) // #nosec G204 -- ffprobePath resolved via exec.LookPath (not attacker-controlled); clip is a locally-composed path argument, no shell involved.
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
}
