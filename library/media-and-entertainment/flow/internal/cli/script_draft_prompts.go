// Copyright 2026 github-actionsbot and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// defaultShotCredits mirrors the real cost of a single Veo generation
// observed live against the user's Flow account (traffic-analysis.json,
// credits_and_cost.test_generation_cost_credits). Flow does not expose a
// pre-submission cost estimate API, so this is the best available default
// until queue estimate/video watch see per-shot tier overrides.
const defaultShotCredits = 15

// radioPlayScript mirrors wryenmeek/Scribe's RadioPlayScript pydantic model
// (scribe/models/assembly_models.py), the canonical recap_script.json
// artifact per Scribe's own ADR-011 ("the canonical persisted recap-script
// artifact is a typed Pydantic-backed JSON artifact... using RadioPlayScript
// as the core content model"). Not the raw Transcript/TranscriptSegment
// model, which is an earlier-stage ASR artifact, not the recap script.
type radioPlayScript struct {
	Title    string          `json:"title"`
	Elements []scriptElement `json:"elements"`
}

// scriptElement is the union of Scribe's ScriptElement variants
// (MusicCue | SoundEffect | Narration | DialogueLine), discriminated by
// element_type, decoded loosely into one struct since Go lacks tagged
// unions. Fields not present for a given element_type are simply empty.
type scriptElement struct {
	ElementType   string `json:"element_type"`
	Speaker       string `json:"speaker,omitempty"`
	Text          string `json:"text,omitempty"`
	CharacterName string `json:"character_name,omitempty"`
	Description   string `json:"description,omitempty"`
	SceneContext  string `json:"scene_context,omitempty"`
	Filepath      string `json:"filepath,omitempty"`
}

// queueShot is one entry of the prompt queue this command emits. episode
// import, queue estimate, and video watch --batch all read this same shape.
type queueShot struct {
	Index            int    `json:"index"`
	ElementType      string `json:"element_type"`
	Speaker          string `json:"speaker,omitempty"`
	CharacterName    string `json:"character_name,omitempty"`
	Text             string `json:"text,omitempty"`
	SeedImage        string `json:"seed_image,omitempty"`
	Prompt           string `json:"prompt"`
	EstimatedCredits int    `json:"estimated_credits"`
	JobName          string `json:"job_name,omitempty"`
}

// promptQueue is the file script draft-prompts / episode import write and
// queue estimate / video watch --batch consume.
type promptQueue struct {
	GeneratedFrom string      `json:"generated_from"`
	Title         string      `json:"title,omitempty"`
	ImagesDir     string      `json:"images_dir,omitempty"`
	Shots         []queueShot `json:"shots"`
}

func newNovelScriptDraftPromptsCmd(flags *rootFlags) *cobra.Command {
	var flagImagesDir string
	var flagOut string

	cmd := &cobra.Command{
		Use:   "draft-prompts <recap-script.json>",
		Short: "Turn a Scribe recap-script JSON into a ready-to-approve queue of per-segment Flow prompts",
		Long: "Reads a Scribe recap_script.json (a RadioPlayScript: {\"title\": ..., \"elements\": [...]}, per Scribe's " +
			"own ADR-011) and drafts one Flow prompt per element -- dialogue, narration, sound effect, or music cue -- " +
			"matched where possible to a seed image from --images-dir.",
		Example: "  flow-pp-cli script draft-prompts recap_script.json --images-dir ./seed-images --out episode3-queue.json",
		// pp:happy-args points at the shipped dogfood fixtures explicitly
		// (rather than relying on the Example literal matching whatever
		// root-level files happen to exist) and writes to a dedicated
		// scratch output so it never collides with other commands' shared
		// fixtures. pp:typed-exit-codes: presence unlocks a real
		// (non-dry-run) execution attempt instead of the matrix's default
		// dry-run-only happy path for this mutating command.
		Annotations: map[string]string{
			"mcp:read-only":       "false",
			"pp:happy-args":       "recap-script=testdata/dogfood-fixtures/scribe/recap_script.json;--images-dir=testdata/dogfood-fixtures/images;--out=testdata/dogfood-fixtures/script-draft-output.json",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("draft-prompts requires a recap-script.json path; run %q for usage", cmd.CommandPath()+" --help"))
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, fmt.Sprintf("draft a prompt queue from %s", args[0]))
			}

			queue, err := draftPromptQueue(args[0], flagImagesDir)
			if err != nil {
				return usageErr(err)
			}

			if flagOut != "" {
				out, err := json.MarshalIndent(queue, "", "  ")
				if err != nil {
					return err
				}
				if err := os.WriteFile(flagOut, out, 0o600); err != nil {
					return usageErr(fmt.Errorf("writing queue file: %w", err))
				}
				unmatched := 0
				for _, s := range queue.Shots {
					if s.SeedImage == "" {
						unmatched++
					}
				}
				if wantsHumanTable(cmd.OutOrStdout(), flags) {
					fmt.Fprintf(cmd.OutOrStdout(), "drafted %d shots to %s (%d without a matched seed image)\n", len(queue.Shots), flagOut, unmatched)
					return nil
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"out": flagOut, "shots": len(queue.Shots), "unmatched_images": unmatched}, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), queue, flags)
		},
	}
	cmd.Flags().StringVar(&flagImagesDir, "images-dir", "", "Directory of seed images to match against each element's speaker/character name")
	cmd.Flags().StringVar(&flagOut, "out", "", "Write the drafted queue to this file instead of stdout")
	return cmd
}

// draftPromptQueue is the pure drafting logic shared by script draft-prompts
// and episode import. It is a plain function, not a sub-command invocation,
// because the generated novel-command scaffolds are only wired with global
// flags (--json, --agent, etc.) when attached to the real root command tree
// -- a detached *cobra.Command built via newNovelScriptDraftPromptsCmd and
// Execute()'d standalone does not inherit root's persistent flags and
// rejects them as unknown.
func draftPromptQueue(scriptPath, imagesDir string) (promptQueue, error) {
	raw, err := os.ReadFile(filepath.Clean(scriptPath)) // #nosec G304 -- user-specified script file is this command's documented purpose.
	if err != nil {
		return promptQueue{}, fmt.Errorf("reading recap script: %w", err)
	}
	var script radioPlayScript
	if err := json.Unmarshal(raw, &script); err != nil {
		return promptQueue{}, fmt.Errorf("parsing recap script as a Scribe RadioPlayScript ({\"title\":..., \"elements\":[...]}): %w", err)
	}
	if len(script.Elements) == 0 {
		return promptQueue{}, fmt.Errorf("recap script at %s has no elements", scriptPath)
	}

	var images []string
	if imagesDir != "" {
		entries, err := os.ReadDir(imagesDir)
		if err != nil {
			return promptQueue{}, fmt.Errorf("reading images dir: %w", err)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if isImageFile(e.Name()) {
				images = append(images, filepath.Join(imagesDir, e.Name()))
			}
		}
	}

	queue := promptQueue{GeneratedFrom: scriptPath, Title: script.Title, ImagesDir: imagesDir}
	for i, el := range script.Elements {
		shot := queueShot{
			Index:            i,
			ElementType:      el.ElementType,
			Speaker:          el.Speaker,
			CharacterName:    el.CharacterName,
			Text:             el.Text,
			Prompt:           draftPromptForElement(el),
			EstimatedCredits: defaultShotCredits,
		}
		if match := matchSeedImage(el, images); match != "" {
			shot.SeedImage = match
		}
		queue.Shots = append(queue.Shots, shot)
	}
	return queue, nil
}

func draftPromptForElement(el scriptElement) string {
	who := el.CharacterName
	if who == "" {
		who = el.Speaker
	}
	switch el.ElementType {
	case "sfx":
		return fmt.Sprintf("Sound effect: %s", el.Description)
	case "music":
		if el.SceneContext != "" {
			return fmt.Sprintf("Background music cue (%s): %s", el.SceneContext, el.Description)
		}
		return fmt.Sprintf("Background music cue: %s", el.Description)
	case "narration":
		return el.Text
	default: // "dialogue"
		if who == "" {
			return el.Text
		}
		return fmt.Sprintf("%s: %q", who, el.Text)
	}
}

var imageExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".heic": true, ".heif": true, ".webp": true, ".gif": true,
}

func isImageFile(name string) bool {
	return imageExtensions[strings.ToLower(filepath.Ext(name))]
}

// matchSeedImage does a best-effort filename match, not a documented Flow or
// Scribe convention -- there is no confirmed naming scheme for the user's
// separate images-folder Drive assets, so this is a mechanical substring
// match over the character name (falling back to speaker), with the
// shortest matching filename winning ties. Only dialogue/narration carry a
// speaker or character to match against; sfx/music elements have none.
func matchSeedImage(el scriptElement, images []string) string {
	who := el.CharacterName
	if who == "" {
		who = el.Speaker
	}
	needle := normalizeForMatch(who)
	if needle == "" {
		return ""
	}
	var matches []string
	for _, img := range images {
		if strings.Contains(normalizeForMatch(filepath.Base(img)), needle) {
			matches = append(matches, img)
		}
	}
	if len(matches) == 0 {
		return ""
	}
	sort.Slice(matches, func(i, j int) bool { return len(matches[i]) < len(matches[j]) })
	return matches[0]
}

func normalizeForMatch(s string) string {
	s = strings.ToLower(s)
	replacer := strings.NewReplacer("_", " ", "-", " ", ".", " ")
	return strings.Join(strings.Fields(replacer.Replace(s)), " ")
}
