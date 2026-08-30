// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/groq/internal/cliutil"
)

type audioBatchEntry struct {
	File   string `json:"file"`
	Action string `json:"action"`
	OK     bool   `json:"ok"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

type audioBatchReport struct {
	Action    string            `json:"action"`
	Processed int               `json:"processed"`
	Succeeded int               `json:"succeeded"`
	Failed    int               `json:"failed"`
	Paced     bool              `json:"paced"`
	Results   []audioBatchEntry `json:"results"`
}

func newNovelAudioBatchCmd(flags *rootFlags) *cobra.Command {
	var flagAction string
	var flagPace bool
	var flagModel string

	cmd := &cobra.Command{
		Use:         "batch <dir|files...>",
		Short:       "Transcribe, translate, or synthesize speech over many audio files with rate-limit-aware pacing and a results manifest.",
		Long:        "Run transcription, translation, or speech synthesis across a directory or list of files. With --pace, calls are throttled with a fixed delay to respect rate-limit budgets. Transcribe/translate accept audio files; speech reads text files and writes .wav output next to each input.",
		Example:     "  groq-pp-cli audio batch testdata/sample-audio.wav --action transcribe --model whisper-large-v3",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "testdata/sample-audio.wav;--action=transcribe;--model=whisper-large-v3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "audio batch")
			}
			if flagAction == "" {
				return usageErr(fmt.Errorf("--action is required: transcribe, translate, or speech"))
			}
			action := flagAction
			switch action {
			case "transcribe", "translate", "speech":
			default:
				return usageErr(fmt.Errorf("--action must be transcribe, translate, or speech; got %q", action))
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("at least one audio file or directory is required"))
			}
			files, err := expandAudioInputs(args, action)
			if err != nil {
				return err
			}
			if cliutil.IsDogfoodEnv() && len(files) > 1 {
				files = files[:1]
			}
			model := flagModel
			if model == "" {
				model = "whisper-large-v3"
				if action == "speech" {
					model = "playai-tts"
				}
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			report := &audioBatchReport{Action: action, Paced: flagPace, Results: make([]audioBatchEntry, 0, len(files))}
			for i, file := range files {
				entry := audioBatchEntry{File: file, Action: action}
				out, err := runAudioFile(ctx, c, action, model, file, !flags.dryRun)
				if err != nil {
					entry.Error = err.Error()
					entry.OK = false
					report.Failed++
				} else {
					entry.OK = true
					entry.Output = out
					report.Succeeded++
				}
				report.Processed++
				report.Results = append(report.Results, entry)
				if flagPace && i < len(files)-1 {
					if !sleepContext(ctx, 1*time.Second) {
						break
					}
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			for _, r := range report.Results {
				if r.OK {
					if action == "speech" {
						fmt.Fprintf(cmd.OutOrStdout(), "OK   %s -> %s\n", r.File, r.Output)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "OK   %s\n", r.File)
						if r.Output != "" {
							fmt.Fprintf(cmd.OutOrStdout(), "     %s\n", truncateString(r.Output, 200))
						}
					}
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "FAIL %s: %s\n", r.File, r.Error)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d processed, %d succeeded, %d failed\n", report.Processed, report.Succeeded, report.Failed)
			if report.Failed > 0 {
				return usageErr(fmt.Errorf("%d of %d audio files failed", report.Failed, report.Processed))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagAction, "action", "", "Action to run: transcribe, translate, or speech")
	cmd.Flags().BoolVar(&flagPace, "pace", false, "Insert a fixed ~1s delay between files to stay under rate-limit budgets")
	cmd.Flags().StringVar(&flagModel, "model", "", "Model to use (default: whisper-large-v3 for transcribe/translate, playai-tts for speech)")
	return cmd
}

func expandAudioInputs(args []string, action string) ([]string, error) {
	var files []string
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return nil, fmt.Errorf("cannot read %q: %w", arg, err)
		}
		if !info.IsDir() {
			files = append(files, arg)
			continue
		}
		entries, err := os.ReadDir(arg)
		if err != nil {
			return nil, fmt.Errorf("reading directory %q: %w", arg, err)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			ext := strings.ToLower(filepath.Ext(name))
			if action == "speech" {
				if ext == ".txt" || ext == ".md" || ext == ".text" {
					files = append(files, filepath.Join(arg, name))
				}
			} else if isAudioExt(ext) {
				files = append(files, filepath.Join(arg, name))
			}
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no matching %s files found in the given paths", audioExtLabel(action))
	}
	sort.Strings(files)
	return files, nil
}

func isAudioExt(ext string) bool {
	switch ext {
	case ".mp3", ".wav", ".m4a", ".ogg", ".flac", ".webm", ".mp4", ".mpeg", ".mpga":
		return true
	}
	return false
}

func audioExtLabel(action string) string {
	if action == "speech" {
		return "text (.txt/.md)"
	}
	return "audio (mp3/wav/m4a/ogg/flac/webm/mp4)"
}

func runAudioFile(ctx context.Context, c clientIface, action, model, file string, writeOutput bool) (string, error) {
	switch action {
	case "transcribe", "translate":
		path := "/openai/v1/audio/transcriptions"
		if action == "translate" {
			path = "/openai/v1/audio/translations"
		}
		fields := map[string]string{"model": model, "response_format": "text"}
		fileFields := map[string]string{"file": file}
		data, status, err := c.PostMultipartWithParams(ctx, path, nil, fields, fileFields)
		if err != nil {
			return "", fmt.Errorf("HTTP %d: %w", status, err)
		}
		return string(data), nil
	case "speech":
		content, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("reading text file: %w", err)
		}
		body := map[string]any{"model": model, "input": string(content), "voice": "Arista-PlayAI", "response_format": "wav"}
		data, status, err := c.PostWithParams(ctx, "/openai/v1/audio/speech", nil, body)
		if err != nil {
			return "", fmt.Errorf("HTTP %d: %w", status, err)
		}
		// The typed client wraps binary payloads (audio/wav) in a base64
		// _pp_binary envelope so the JSON contract survives; unwrap it
		// before writing the .wav file.
		audio := unwrapBinaryResponse(data)
		if writeOutput {
			outPath := strings.TrimSuffix(file, filepath.Ext(file)) + ".wav"
			if err := os.WriteFile(outPath, audio, 0o644); err != nil {
				return "", fmt.Errorf("writing audio output: %w", err)
			}
			return outPath, nil
		}
		return fmt.Sprintf("(%d bytes)", len(audio)), nil
	}
	return "", fmt.Errorf("unknown action %q", action)
}

// clientIface is the minimal client surface audio batch needs, satisfied by
// *client.Client, so the helper is testable with a fake.
type clientIface interface {
	PostMultipartWithParams(ctx context.Context, path string, params map[string]string, fields map[string]string, fileFields map[string]string) (json.RawMessage, int, error)
	PostWithParams(ctx context.Context, path string, params map[string]string, body any) (json.RawMessage, int, error)
}

// binaryEnvelope mirrors the typed client's base64 wrapper for binary
// payloads (audio/wav etc.) so callers can recover the raw bytes.
type binaryEnvelope struct {
	PPBinary    bool   `json:"_pp_binary"`
	ContentType string `json:"content_type"`
	Encoding    string `json:"encoding"`
	Bytes       int    `json:"bytes"`
	Data        string `json:"data"`
}

// unwrapBinaryResponse returns the raw payload bytes. If the response is the
// client's _pp_binary base64 envelope, it decodes and returns the embedded
// data; otherwise it returns the bytes unchanged.
func unwrapBinaryResponse(data []byte) []byte {
	var env binaryEnvelope
	if err := json.Unmarshal(data, &env); err == nil && env.PPBinary && env.Data != "" {
		if dec, derr := base64.StdEncoding.DecodeString(env.Data); derr == nil {
			return dec
		}
	}
	return data
}

// sleepContext waits for the duration or returns false early when the context
// is cancelled, so root --timeout can interrupt pacing.
func sleepContext(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
