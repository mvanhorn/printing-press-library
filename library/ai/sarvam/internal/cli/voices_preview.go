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
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// ttsSpeakerCatalog is the full bulbul:v3 speaker catalog from the Sarvam
// docs. Speakers are lowercase and case-sensitive.
var ttsSpeakerCatalog = []string{
	"anushka", "abhilash", "manisha", "vidya", "arya", "karun", "hitesh",
	"aditya", "ritu", "priya", "neha", "rahul", "pooja", "rohan", "simran",
	"kavya", "amit", "dev", "ishita", "shreya", "ratan", "varun", "manan",
	"sumit", "roopa", "kabir", "aayan", "shubh", "ashutosh", "advait",
	"anand", "tanya", "tarun", "sunny", "mani", "gokul", "vijay", "shruti",
	"suhani", "mohit", "kavitha", "rehan", "soham", "rupali",
}

func newNovelVoicesPreviewCmd(flags *rootFlags) *cobra.Command {
	var flagLang string
	var flagSample string
	var flagSpeakers string
	var flagOutput string
	var flagModel string

	cmd := &cobra.Command{
		Use:         "preview",
		Short:       "Generate one sample sentence across every TTS speaker and hear them all side by side",
		Example:     "  sarvam-pp-cli voices preview --lang hi-IN --sample 'नमस्ते, स्वागत है' --speakers shubh,ritu,priya --output ./voices",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "voices preview")
			}
			if flagSample == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--sample is required"))
			}
			if flagLang == "" {
				flagLang = "hi-IN"
			}
			if flagModel == "" {
				flagModel = "bulbul:v3"
			}
			var speakers []string
			if flagSpeakers != "" {
				for _, s := range strings.Split(flagSpeakers, ",") {
					s = strings.TrimSpace(s)
					if s != "" {
						speakers = append(speakers, s)
					}
				}
			}
			if len(speakers) == 0 {
				speakers = ttsSpeakerCatalog
			}
			if flagOutput == "" {
				flagOutput = "./voices"
			}
			if err := os.MkdirAll(flagOutput, 0o750); err != nil {
				return fmt.Errorf("creating output dir: %w", err)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type voiceResult struct {
				Speaker string `json:"speaker"`
				File    string `json:"file,omitempty"`
				Error   string `json:"error,omitempty"`
			}
			results := make([]voiceResult, 0, len(speakers))
			for _, speaker := range speakers {
				body := map[string]any{
					"text":          flagSample,
					"language_code": flagLang,
					"speaker":       speaker,
					"model":         flagModel,
				}
				data, _, err := c.PostWithParams(ctx, "/text-to-speech", nil, body)
				if err != nil {
					results = append(results, voiceResult{Speaker: speaker, Error: err.Error()})
					continue
				}
				var resp struct {
					Audios []string `json:"audios"`
				}
				if err := json.Unmarshal(data, &resp); err != nil || len(resp.Audios) == 0 {
					results = append(results, voiceResult{Speaker: speaker, Error: "empty audio response"})
					continue
				}
				audioBytes, err := base64.StdEncoding.DecodeString(resp.Audios[0])
				if err != nil {
					results = append(results, voiceResult{Speaker: speaker, Error: "invalid base64 audio"})
					continue
				}
				fname := filepath.Join(flagOutput, speaker+".wav")
				// #nosec G306 -- user-facing audio output the caller explicitly requested; 0644 allows playback by other users on shared systems.
				if err := os.WriteFile(fname, audioBytes, 0o644); err != nil {
					return fmt.Errorf("writing %s: %w", fname, err)
				}
				results = append(results, voiceResult{Speaker: speaker, File: fname})
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"language": flagLang,
					"model":    flagModel,
					"sample":   flagSample,
					"results":  results,
				}, flags); err != nil {
					return err
				}
			} else {
				for _, r := range results {
					if r.Error != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "%-12s error: %s\n", r.Speaker, r.Error)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "%-12s -> %s\n", r.Speaker, r.File)
					}
				}
			}

			// Exit non-zero when every speaker failed so scripted pipelines
			// can detect total failure from the exit status alone.
			wroteAny := false
			for _, r := range results {
				if r.Error == "" {
					wroteAny = true
					break
				}
			}
			if !wroteAny {
				return partialFailureErr(fmt.Errorf("no audio generated: all %d speaker request(s) failed", len(results)))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagLang, "lang", "hi-IN", "Language code (BCP-47) for the sample audio")
	cmd.Flags().StringVar(&flagSample, "sample", "", "Sample sentence to synthesize for each speaker")
	cmd.Flags().StringVar(&flagSpeakers, "speakers", "", "Comma-separated speaker list (default: all catalog speakers)")
	cmd.Flags().StringVar(&flagOutput, "output", "./voices", "Directory to write generated audio files")
	cmd.Flags().StringVar(&flagModel, "model", "bulbul:v3", "TTS model to use")
	return cmd
}
