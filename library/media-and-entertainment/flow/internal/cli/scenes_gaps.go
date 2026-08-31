// Copyright 2026 github-actionsbot and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/flow/internal/types"

	"github.com/spf13/cobra"
)

// projectContentsPayload mirrors the nested shape observed live inside
// flow.projectInitialData's projectContents field (traffic-analysis.json,
// project-contents cluster). Only CHARACTER entities were ever observed in
// the sampled project -- no SCENE entityType was confirmed, so this command
// reports that honestly instead of assuming a scene entity exists.
type projectContentsPayload struct {
	Media []struct {
		WorkflowId    string `json:"workflowId"`
		MediaMetadata struct {
			MediaTitle string `json:"mediaTitle"`
			Prompt     string `json:"prompt"`
			Status     string `json:"status"`
		} `json:"mediaMetadata"`
	} `json:"media"`
	Entities []struct {
		EntityInfo struct {
			EntityType string `json:"entityType"`
		} `json:"entityInfo"`
		DisplayName     string   `json:"displayName"`
		ImageReferences []string `json:"imageReferences"`
	} `json:"entities"`
}

type scenesGapsResult struct {
	Project                 string         `json:"project"`
	Characters              int            `json:"characters"`
	CharactersMissingImage  []string       `json:"characters_missing_image"`
	MediaByStatus           map[string]int `json:"media_by_status"`
	SceneEntityTypeObserved bool           `json:"scene_entity_type_observed"`
	Note                    string         `json:"note,omitempty"`
}

func newNovelScenesGapsCmd(flags *rootFlags) *cobra.Command {
	var flagProject string

	cmd := &cobra.Command{
		Use:   "gaps",
		Short: "Find scenes with no staged image and characters that were never assigned to a scene, before you submit a batch.",
		Long: "Reports characters with no reference image and a status breakdown of media in a project, drawn from the " +
			"same flow.projectInitialData payload that backs Flow's Images/Characters/Scenes tabs.\n\n" +
			"Only a CHARACTER entity type has ever been observed live in this payload -- no distinct SCENE entity was " +
			"confirmed, so this command cannot verify scene-level gaps the way the Scenes tab implies; it reports " +
			"character and media gaps instead and says so explicitly when a SCENE entity type is absent.",
		Example: "  flow-pp-cli scenes gaps --project a1b2c3d4-e5f6-47a8-9b0c-1d2e3f4a5b6c",
		// pp:typed-exit-codes: same placeholder-project-ID caveat as
		// promoted_project.go -- a real authenticated run legitimately gets
		// a typed 5 (API error) rejecting the well-formed but non-real ID.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagProject == "" && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if flagProject == "" {
				return usageErr(fmt.Errorf("--project is required; run %q for usage", cmd.CommandPath()+" --help"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, fmt.Sprintf("check asset gaps for project %s", flagProject))
			}

			contents, err := fetchProjectContents(cmd, flags, flagProject)
			if err != nil {
				return err
			}
			result, err := computeSceneGaps(flagProject, contents)
			if err != nil {
				return apiErr(err)
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "%d character(s), %d missing a reference image\n", result.Characters, len(result.CharactersMissingImage))
				for _, name := range result.CharactersMissingImage {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", name)
				}
				for status, count := range result.MediaByStatus {
					fmt.Fprintf(cmd.OutOrStdout(), "media[%s]: %d\n", status, count)
				}
				if result.Note != "" {
					fmt.Fprintln(cmd.OutOrStdout(), "note:", result.Note)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagProject, "project", "", "Flow project ID to check (required)")
	return cmd
}

func computeSceneGaps(projectID string, contents types.ProjectContents) (scenesGapsResult, error) {
	var payload projectContentsPayload
	if len(contents.ProjectContents) > 0 {
		if err := json.Unmarshal(contents.ProjectContents, &payload); err != nil {
			return scenesGapsResult{}, fmt.Errorf("parsing nested project contents: %w", err)
		}
	}

	result := scenesGapsResult{Project: projectID, MediaByStatus: map[string]int{}}
	for _, e := range payload.Entities {
		if e.EntityInfo.EntityType == "SCENE" {
			result.SceneEntityTypeObserved = true
		}
		if e.EntityInfo.EntityType != "CHARACTER" {
			continue
		}
		result.Characters++
		if len(e.ImageReferences) == 0 && e.DisplayName != "" {
			result.CharactersMissingImage = append(result.CharactersMissingImage, e.DisplayName)
		}
	}
	for _, m := range payload.Media {
		status := m.MediaMetadata.Status
		if status == "" {
			status = "unknown"
		}
		result.MediaByStatus[status]++
	}
	if !result.SceneEntityTypeObserved {
		result.Note = "no SCENE entity type is present in this project's data -- Flow's Scenes tab appears to be a client-side view over media/characters rather than a distinct API entity, so scene-level gaps could not be checked"
	}
	return result, nil
}

// characterDisplayNames extracts CHARACTER entity display names from a
// project's contents, for drive import --tag-scene's filename matching.
func characterDisplayNames(contents types.ProjectContents) ([]string, error) {
	var payload projectContentsPayload
	if len(contents.ProjectContents) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(contents.ProjectContents, &payload); err != nil {
		return nil, fmt.Errorf("parsing nested project contents: %w", err)
	}
	var names []string
	for _, e := range payload.Entities {
		if e.EntityInfo.EntityType == "CHARACTER" && e.DisplayName != "" {
			names = append(names, e.DisplayName)
		}
	}
	return names, nil
}
