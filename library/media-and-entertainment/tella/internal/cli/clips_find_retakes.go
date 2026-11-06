// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/client"
	"github.com/spf13/cobra"
	"os"
	"strings"
	"unicode"
)

type transcriptSentence struct {
	FirstIndex  int     `json:"firstIndex"`
	LastIndex   int     `json:"lastIndex"`
	StartTimeMs float64 `json:"startTimeMs"`
	EndTimeMs   float64 `json:"endTimeMs"`
	Text        string  `json:"text"`
}
type retakeCandidate struct {
	Range       cleanWordRange `json:"word_range"`
	StartTimeMs float64        `json:"startTimeMs"`
	EndTimeMs   float64        `json:"endTimeMs"`
	EarlierText string         `json:"earlier_text"`
	KeptText    string         `json:"kept_text"`
}
type retakePlan struct {
	Version     int               `json:"version"`
	VideoID     string            `json:"video_id"`
	ClipID      string            `json:"clip_id"`
	Fingerprint string            `json:"transcript_and_cuts_fingerprint"`
	Candidates  []retakeCandidate `json:"candidates"`
	Limits      string            `json:"limits"`
}

const retakeLimits = "Heuristic review only: adjacent sentences with at least five identical normalized prefix words, no more than 4 seconds apart. Proposes only the entire earlier fragment and preserves the last take. Cannot detect semantic mistakes or prove a repetition is unwanted."

func readSentences(ctx context.Context, c *client.Client, video, clip string) ([]transcriptSentence, error) {
	raw, err := c.Get(ctx, clipPath(video, clip)+"/transcript", map[string]string{"view": "sentences"})
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Sentences []json.RawMessage `json:"sentences"`
	}
	if err = json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	if envelope.Sentences == nil {
		return nil, fmt.Errorf("sentence response missing sentences array")
	}
	out := make([]transcriptSentence, 0, len(envelope.Sentences))
	for i, r := range envelope.Sentences {
		var fields map[string]json.RawMessage
		if err = json.Unmarshal(r, &fields); err != nil {
			return nil, err
		}
		for _, k := range []string{"firstIndex", "lastIndex", "startTimeMs", "endTimeMs", "text"} {
			if len(fields[k]) == 0 || string(fields[k]) == "null" {
				return nil, fmt.Errorf("sentence missing %s", k)
			}
		}
		var s transcriptSentence
		if err = json.Unmarshal(r, &s); err != nil {
			return nil, err
		}
		if s.FirstIndex < 0 || s.LastIndex < s.FirstIndex || s.StartTimeMs < 0 || s.EndTimeMs < s.StartTimeMs {
			return nil, fmt.Errorf("invalid sentence range")
		}
		if i > 0 && (s.FirstIndex <= out[i-1].LastIndex || s.StartTimeMs < out[i-1].EndTimeMs) {
			return nil, fmt.Errorf("sentence ranges are not ordered")
		}
		out = append(out, s)
	}
	return out, nil
}
func normalizedSentence(text string) []string {
	return strings.Fields(strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return unicode.ToLower(r)
		}
		return ' '
	}, text))
}
func proposeRetakes(video, clip string, sentences []transcriptSentence, cuts any) retakePlan {
	p := retakePlan{Version: 1, VideoID: video, ClipID: clip, Candidates: []retakeCandidate{}, Limits: retakeLimits}
	b, _ := json.Marshal(struct {
		Sentences []transcriptSentence
		Cuts      any
	}{sentences, cuts})
	sum := sha256.Sum256(b)
	p.Fingerprint = hex.EncodeToString(sum[:])
	for i := 0; i+1 < len(sentences); i++ {
		earlier, next := sentences[i], sentences[i+1]
		a, b := normalizedSentence(earlier.Text), normalizedSentence(next.Text)
		gap := next.StartTimeMs - earlier.EndTimeMs
		if len(a) < 5 || len(b) < len(a) || gap < 0 || gap > 4000 {
			continue
		}
		match := true
		for j := range a {
			if a[j] != b[j] {
				match = false
				break
			}
		}
		if match {
			p.Candidates = append(p.Candidates, retakeCandidate{Range: cleanWordRange{earlier.FirstIndex, earlier.LastIndex}, StartTimeMs: earlier.StartTimeMs, EndTimeMs: earlier.EndTimeMs, EarlierText: earlier.Text, KeptText: next.Text})
		}
	}
	return p
}
func currentRetakePlan(ctx context.Context, c *client.Client, video, clip string) (retakePlan, cutSnapshot, error) {
	s, err := readSentences(ctx, c, video, clip)
	if err != nil {
		return retakePlan{}, cutSnapshot{}, err
	}
	snap, err := captureCutSnapshot(ctx, c, video, clip)
	if err != nil {
		return retakePlan{}, snap, err
	}
	return proposeRetakes(video, clip, s, snap.Cuts), snap, nil
}
func newNovelClipsFindRetakesCmd(flags *rootFlags) *cobra.Command {
	var apply bool
	var planFile, savePlan string
	cmd := &cobra.Command{Use: "find-retakes <video> <clip>", Short: "Review conservative sentence repeats; apply only a saved, unchanged plan", Args: workflowArgs(2), Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live", "mcp:read-flags": "plan", "pp:preview-happy-path": "true", "pp:happy-args": "--json=true"}, Example: "  tella-pp-cli clips find-retakes vid_example cl_example\n  tella-pp-cli clips find-retakes vid_example cl_example --save-plan retakes.json\n  tella-pp-cli clips find-retakes vid_example cl_example --plan retakes.json --apply"}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		if apply && planFile == "" {
			return usageErr(fmt.Errorf("--apply requires a reviewed --plan file"))
		}
		if workflowOffline(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "clips find-retakes")
		}
		c, err := workflowClient(flags)
		if err != nil {
			return err
		}
		p, snapshot, err := currentRetakePlan(cmd.Context(), c, args[0], args[1])
		if err != nil {
			return err
		}
		if planFile != "" {
			b, err := os.ReadFile(planFile)
			if err != nil {
				return err
			}
			var saved retakePlan
			if err = json.Unmarshal(b, &saved); err != nil {
				return err
			}
			if err = validateReviewedRetakes(saved, p); err != nil {
				return err
			}
			saved.Limits = p.Limits
			p = saved
		}
		if savePlan != "" {
			b, err := json.MarshalIndent(p, "", "  ")
			if err != nil {
				return err
			}
			if _, err = writeLessonAsset(savePlan, b, false); err != nil {
				return err
			}
		}
		if flags.dryRun || !apply || len(p.Candidates) == 0 {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "plan": p}, flags)
		}
		path, err := persistCutSnapshot(snapshot)
		if err != nil {
			return err
		}
		ranges := make([]cleanWordRange, 0, len(p.Candidates))
		for _, candidate := range p.Candidates {
			ranges = append(ranges, candidate.Range)
		}
		plans := []cleanClipPlan{{VideoID: p.VideoID, ClipID: p.ClipID, Operations: []cleanOperation{{Op: "cut-by-transcript", WordRanges: ranges}}}}
		snapshots := []cutSnapshot{snapshot}
		result, applyErr := applyCleanPlans(cmd.Context(), c, plans, snapshots)
		applyErr = errors.Join(applyErr, updateCutSnapshotFiles(map[string]string{p.ClipID: path}, snapshots))
		if applyErr != nil {
			return workflowFailure(cmd, flags, map[string]any{"snapshot": path, "result": result, "limits": "cuts only"}, applyErr)
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"applied": true, "plan": p, "snapshot": path, "result": result}, flags)
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the explicitly reviewed saved plan (never implied by --yes)")
	cmd.Flags().StringVar(&planFile, "plan", "", "Previously reviewed saved retake plan")
	cmd.Flags().StringVar(&savePlan, "save-plan", "", "Save a plan to a new local JSON file")
	return cmd
}

func validateReviewedRetakes(saved, current retakePlan) error {
	if saved.Version != current.Version || saved.VideoID != current.VideoID || saved.ClipID != current.ClipID || saved.Fingerprint != current.Fingerprint {
		return fmt.Errorf("saved retake plan is stale or mismatched; review a fresh plan")
	}
	remaining := map[retakeCandidate]bool{}
	for _, candidate := range current.Candidates {
		remaining[candidate] = true
	}
	for _, candidate := range saved.Candidates {
		if !remaining[candidate] {
			return fmt.Errorf("saved retake plan contains an added, altered or duplicate candidate")
		}
		delete(remaining, candidate)
	}
	return nil
}
