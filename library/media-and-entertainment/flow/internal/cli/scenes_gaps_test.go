// Copyright 2026 github-actionsbot and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/flow/internal/types"
)

// TestNovelScenesGapsHelpWires smoke-tests that the scenes gaps command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelScenesGapsHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"scenes", "gaps", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("scenes gaps --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "gaps"} {
		if !strings.Contains(help, want) {
			t.Fatalf("scenes gaps --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestComputeSceneGapsFindsCharactersMissingImagesAndNoScenetype(t *testing.T) {
	raw := json.RawMessage(`{
		"media": [
			{"workflowId": "w1", "mediaMetadata": {"mediaTitle": "shot1", "status": "MEDIA_GENERATION_STATUS_SUCCEEDED"}},
			{"workflowId": "w2", "mediaMetadata": {"mediaTitle": "shot2", "status": "MEDIA_GENERATION_STATUS_FAILED"}}
		],
		"entities": [
			{"entityInfo": {"entityType": "CHARACTER"}, "displayName": "Fade", "imageReferences": ["img1"]},
			{"entityInfo": {"entityType": "CHARACTER"}, "displayName": "Safarkou", "imageReferences": []}
		]
	}`)
	contents := types.ProjectContents{ProjectId: "proj-1", ProjectContents: raw}

	result, err := computeSceneGaps("proj-1", contents)
	if err != nil {
		t.Fatalf("computeSceneGaps error = %v", err)
	}
	if result.Characters != 2 {
		t.Fatalf("expected 2 characters, got %d", result.Characters)
	}
	if len(result.CharactersMissingImage) != 1 || result.CharactersMissingImage[0] != "Safarkou" {
		t.Fatalf("expected only Safarkou missing an image, got %v", result.CharactersMissingImage)
	}
	if result.MediaByStatus["MEDIA_GENERATION_STATUS_SUCCEEDED"] != 1 || result.MediaByStatus["MEDIA_GENERATION_STATUS_FAILED"] != 1 {
		t.Fatalf("expected one succeeded and one failed media item, got %+v", result.MediaByStatus)
	}
	if result.SceneEntityTypeObserved {
		t.Fatalf("expected no SCENE entity type in this fixture")
	}
	if result.Note == "" {
		t.Fatalf("expected a note explaining the missing SCENE entity type")
	}
}

// TestComputeSceneGapsSkipsCharactersWithNoDisplayName reproduces a real
// live project (a CHARACTER entity with no displayName): before this fix,
// computeSceneGaps appended the empty string to CharactersMissingImage,
// producing a useless "" entry a user can't act on. The character should
// still count toward Characters -- only the unusable name is dropped from
// the actionable list. See
// .printing-press-patches/2026-08-30-nextauth-cookie-surface.md.
func TestComputeSceneGapsSkipsCharactersWithNoDisplayName(t *testing.T) {
	raw := json.RawMessage(`{
		"media": [],
		"entities": [
			{"entityInfo": {"entityType": "CHARACTER"}, "displayName": "", "imageReferences": []},
			{"entityInfo": {"entityType": "CHARACTER"}, "displayName": "Fade", "imageReferences": []}
		]
	}`)
	contents := types.ProjectContents{ProjectId: "proj-1", ProjectContents: raw}

	result, err := computeSceneGaps("proj-1", contents)
	if err != nil {
		t.Fatalf("computeSceneGaps error = %v", err)
	}
	if result.Characters != 2 {
		t.Fatalf("expected both characters (including the unnamed one) to count, got %d", result.Characters)
	}
	if len(result.CharactersMissingImage) != 1 || result.CharactersMissingImage[0] != "Fade" {
		t.Fatalf("expected only the named character in CharactersMissingImage, got %v", result.CharactersMissingImage)
	}
	for _, name := range result.CharactersMissingImage {
		if name == "" {
			t.Fatalf("CharactersMissingImage must never contain an empty string entry")
		}
	}
}
