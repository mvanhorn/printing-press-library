// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/spf13/cobra"
)

func newVideosClipsUpdateTranscriptWordsCmd(flags *rootFlags) *cobra.Command {
	var wordsJSON string
	var stdin bool
	var apply bool
	cmd := &cobra.Command{
		Use:   "update-transcript-words <id> <clipId>",
		Short: "Preview or apply transcript word corrections",
		Long:  "Updates up to 100 stable word indices atomically. This changes transcript/captions text or visibility, never audio or timing.",
		Example: `  tella-pp-cli videos clips update-transcript-words vid_abc cl_xyz --words '[{"index":12,"text":"Tella"}]' --dry-run
  tella-pp-cli videos clips update-transcript-words vid_abc cl_xyz --words '[{"index":12,"hidden":true}]' --apply`,
		Args:        cobra.ExactArgs(2),
		Annotations: map[string]string{"pp:endpoint": "clips.update-transcript-words", "pp:method": "PATCH", "pp:path": "/v1/videos/{id}/clips/{clipId}/transcript/words"},
		RunE: func(cmd *cobra.Command, args []string) error {
			words, err := transcriptWordEdits(wordsJSON, stdin)
			if err != nil {
				return usageErr(err)
			}
			body := map[string]any{"words": words}
			result := map[string]any{"video_id": args[0], "clip_id": args[1], "body": body, "dry_run": true, "applied": false}
			if flags.dryRun || !apply {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			api, err := flags.newClient()
			if err != nil {
				return err
			}
			data, status, err := api.Patch(fmt.Sprintf("/v1/videos/%s/clips/%s/transcript/words", args[0], args[1]), body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			result["dry_run"] = false
			result["applied"] = true
			result["status"] = status
			result["data"] = jsonRawToAny(data)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&wordsJSON, "words", "", "JSON array of word edits with index and exactly one of text or hidden")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "Read the word-edit array or {words:[...]} from stdin")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply transcript corrections; default previews the request")
	return cmd
}

func transcriptWordEdits(raw string, fromStdin bool) ([]map[string]any, error) {
	if fromStdin {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		raw = string(data)
	}
	if raw == "" {
		return nil, fmt.Errorf("pass --words JSON or --stdin")
	}
	var words []map[string]any
	if err := json.Unmarshal([]byte(raw), &words); err != nil {
		var envelope struct {
			Words []map[string]any `json:"words"`
		}
		if envelopeErr := json.Unmarshal([]byte(raw), &envelope); envelopeErr != nil {
			return nil, fmt.Errorf("parsing word edits: %w", err)
		}
		words = envelope.Words
	}
	if len(words) == 0 || len(words) > 100 {
		return nil, fmt.Errorf("word edits must contain 1 to 100 items")
	}
	indices := map[int64]bool{}
	for i, word := range words {
		index, ok := word["index"].(float64)
		if !ok || index < 0 || index > 9007199254740991 || index != math.Trunc(index) {
			return nil, fmt.Errorf("word edit %d needs a non-negative integer index", i)
		}
		integerIndex := int64(index)
		if indices[integerIndex] {
			return nil, fmt.Errorf("word edit %d repeats index %d", i, integerIndex)
		}
		indices[integerIndex] = true
		_, hasText := word["text"]
		_, hasHidden := word["hidden"]
		if hasText == hasHidden || len(word) != 2 {
			return nil, fmt.Errorf("word edit %d must set exactly one of text or hidden", i)
		}
		if text := word["text"]; hasText {
			value, valid := text.(string)
			if !valid || value == "" {
				return nil, fmt.Errorf("word edit %d text must be a non-empty string", i)
			}
		}
		if hidden := word["hidden"]; hasHidden {
			if _, valid := hidden.(bool); !valid {
				return nil, fmt.Errorf("word edit %d hidden must be true or false", i)
			}
		}
	}
	return words, nil
}
