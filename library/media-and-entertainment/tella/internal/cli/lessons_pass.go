// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/client"
	"github.com/spf13/cobra"
	"net/url"
	"strings"
	"unicode/utf8"
)

type lessonChapter struct {
	Title            string `json:"title"`
	Description      string `json:"description"`
	TimestampSeconds int    `json:"timestampSeconds"`
}
type lessonReview struct {
	Retakes       []retakePlan    `json:"retake_plans"`
	Chapters      []lessonChapter `json:"chapters"`
	FirstSpokenMs int             `json:"first_spoken_ms"`
}

func draftLessonReview(ctx context.Context, c *client.Client, video string, t lessonTimeline) (lessonReview, error) {
	r := lessonReview{Retakes: []retakePlan{}, Chapters: []lessonChapter{}}
	foundSpeech := false
	lastChapter := -60
	for _, clip := range t.Clips {
		sentences, err := readSentences(ctx, c, video, clip.ID)
		if err != nil {
			return r, err
		}
		snap, err := captureCutSnapshot(ctx, c, video, clip.ID)
		if err != nil {
			return r, err
		}
		r.Retakes = append(r.Retakes, proposeRetakes(video, clip.ID, sentences, snap.Cuts))
		for _, sentence := range sentences {
			if sentence.StartTimeMs >= clip.DurationMs || sentence.EndTimeMs > clip.DurationMs+1 {
				return r, fmt.Errorf("sentence exceeds playback clip duration")
			}
			if strings.TrimSpace(sentence.Text) == "" {
				continue
			}
			start := int(clip.TimelineStartMs + sentence.StartTimeMs)
			if !foundSpeech {
				r.FirstSpokenMs = start
				foundSpeech = true
			}
			second := start / 1000
			if len(r.Chapters) >= 100 || second-lastChapter < 60 {
				continue
			}
			title := strings.Join(strings.Fields(sentence.Text), " ")
			title = string([]rune(title)[:min(255, utf8.RuneCountInString(title))])
			r.Chapters = append(r.Chapters, lessonChapter{Title: title, Description: "Draft from spoken transcript.", TimestampSeconds: second})
			lastChapter = second
		}
	}
	return r, nil
}
func preflightLessonSources(ctx context.Context, c *client.Client, r lessonRecipe) error {
	ids := []string{}
	for _, m := range []*lessonMedia{r.Intro, r.Outro} {
		if m != nil {
			ids = append(ids, m.SourceID)
		}
	}
	if r.BackgroundMusic != nil && r.BackgroundMusic.SourceID != "" {
		ids = append(ids, r.BackgroundMusic.SourceID)
	}
	for _, id := range uniqueStrings(ids) {
		if _, err := c.Get(ctx, "/v1/sources/"+url.PathEscape(id), nil); err != nil {
			return fmt.Errorf("checking recipe source %s: %w", id, err)
		}
	}
	return nil
}
func insertLessonMedia(ctx context.Context, c *client.Client, video string, m *lessonMedia, first bool) error {
	if m == nil {
		return nil
	}
	raw, _, err := attachExistingSource(ctx, c, video, m.SourceID, m.Name)
	if err != nil {
		return err
	}
	var response struct {
		Clip struct {
			ID string `json:"id"`
		} `json:"clip"`
	}
	if err = json.Unmarshal(raw, &response); err != nil {
		return err
	}
	if response.Clip.ID == "" {
		return fmt.Errorf("inserted media response missing clip id; inspect timeline before retrying")
	}
	if first {
		_, _, err = c.Post(ctx, clipPath(video, response.Clip.ID)+"/reorder", map[string]any{"order": 0})
	}
	return err
}
func applyLessonStyle(ctx context.Context, c *client.Client, video string, r lessonRecipe) error {
	base := videoPath(video)
	if r.SubtitlePreset != "" {
		if _, _, err := c.Put(ctx, base+"/subtitle-preset", map[string]any{"presetId": r.SubtitlePreset}); err != nil {
			return err
		}
	}
	if r.Captions != nil {
		if patch := r.Captions.patch(); len(patch) > 0 {
			if _, _, err := c.Patch(ctx, base, patch); err != nil {
				return err
			}
		}
	}
	if m := r.BackgroundMusic; m != nil {
		body := map[string]any{"volume": *m.Volume}
		if m.PresetID != "" {
			body["presetId"] = m.PresetID
		} else {
			body["sourceId"] = m.SourceID
		}
		if _, _, err := c.Put(ctx, base+"/background-music", body); err != nil {
			return err
		}
	}
	return nil
}
func newNovelLessonsPassCmd(flags *rootFlags) *cobra.Command {
	var recipeFile string
	var apply bool
	cmd := &cobra.Command{Use: "pass <video>", Short: "Plan a lesson recipe; --apply edits and creates a preview, stopping before export", Args: workflowArgs(1), Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live", "mcp:read-flags": "recipe", "pp:preview-happy-path": "true", "pp:happy-args": "--json=true"}, Example: "  tella-pp-cli lessons pass vid_example\n  tella-pp-cli lessons pass vid_example --recipe lesson-recipe.example.yaml\n  tella-pp-cli lessons pass vid_example --recipe lesson-recipe.example.yaml --apply"}
	cmd.RunE = func(cmd *cobra.Command, args []string) (err error) {
		if len(args) == 0 {
			return cmd.Help()
		}
		recipe, err := readLessonRecipe(recipeFile)
		if err != nil {
			return usageErr(err)
		}
		if workflowOffline(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "lessons pass")
		}
		c, err := workflowClient(flags)
		if err != nil {
			return err
		}
		p, err := planAutomatics(cmd.Context(), c, args[0], recipe.automatics())
		if err != nil {
			return err
		}
		if len(p.Plans) == 0 {
			return fmt.Errorf("lesson has no clips")
		}
		if err = preflightLessonSources(cmd.Context(), c, recipe); err != nil {
			return err
		}
		t, _, err := readLessonTimeline(cmd.Context(), c, args[0])
		if err != nil {
			return err
		}
		if recipe.Thumbnail.Rule == "fixed" && float64(*recipe.Thumbnail.InpointMs) >= t.Video.DurationMs {
			return usageErr(fmt.Errorf("thumbnail inpointMs must be inside the video"))
		}
		// Preflight transcript access as well as recipe syntax before the first write.
		review, err := draftLessonReview(cmd.Context(), c, args[0], t)
		if err != nil {
			return err
		}
		if flags.dryRun || !apply {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "recipe": recipe, "automatics": p, "review": review, "note": "Retakes are review proposals only. Chapters and first-spoken thumbnail are redrafted from fresh playback times after edits.", "recovery_limits": lessonRecoveryLimit}, flags)
		}
		recovery, err := saveAutoRecovery(p)
		if err != nil {
			return err
		}
		defer func() {
			if err != nil {
				workflowFailure(cmd, flags, recovery, err)
			}
		}()
		if err = applyAutomatics(cmd.Context(), c, p, recovery); err != nil {
			return err
		}
		if err = insertLessonMedia(cmd.Context(), c, args[0], recipe.Intro, true); err != nil {
			return err
		}
		if err = insertLessonMedia(cmd.Context(), c, args[0], recipe.Outro, false); err != nil {
			return err
		}
		// All timing edits precede this read: sentence times are clip playback times.
		t, _, err = readLessonTimeline(cmd.Context(), c, args[0])
		if err != nil {
			return err
		}
		review, err = draftLessonReview(cmd.Context(), c, args[0], t)
		if err != nil {
			return err
		}
		if err = applyLessonStyle(cmd.Context(), c, args[0], recipe); err != nil {
			return err
		}
		if _, _, err = c.Put(cmd.Context(), videoPath(args[0])+"/chapters", map[string]any{"chapters": review.Chapters}); err != nil {
			return err
		}
		inpoint := review.FirstSpokenMs
		if recipe.Thumbnail.Rule == "fixed" {
			inpoint = *recipe.Thumbnail.InpointMs
		}
		if float64(inpoint) >= t.Video.DurationMs {
			return fmt.Errorf("thumbnail frame outside final playback duration")
		}
		if _, _, err = c.Put(cmd.Context(), videoPath(args[0])+"/thumbnail", map[string]any{"inpointMs": inpoint}); err != nil {
			return err
		}
		preview, err := createLessonPreview(cmd.Context(), c, args[0])
		if err != nil {
			return err
		}
		result := map[string]any{"applied": true, "recovery": recovery, "review": review, "retakes_applied": false, "preview": preview, "next": "Review preview, then invoke lessons ship separately."}
		if flags.asJSON || flags.selectFields != "" || flags.compact {
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		}
		if err = printJSONFiltered(cmd.OutOrStdout(), map[string]any{"review": review, "recovery": recovery}, flags); err != nil {
			return err
		}
		return printPreview(cmd, flags, preview)
	}
	cmd.Flags().StringVar(&recipeFile, "recipe", "", "Strict YAML recipe (see lesson-recipe.example.yaml)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply edits; otherwise plan only. --dry-run always wins")
	return cmd
}
