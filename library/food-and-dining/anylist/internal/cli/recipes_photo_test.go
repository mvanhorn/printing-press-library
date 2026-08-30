// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
)

func TestRecipePhotoHelpersPreserveRecipeFieldsAndAvoidDuplicates(t *testing.T) {
	t.Parallel()

	original := &pb.PBRecipe{
		Identifier: "recipe-1",
		Name:       "Pancakes",
		Note:       "Keep the batter lumpy",
		PhotoIds:   []string{"photo-old"},
	}
	updated := appendRecipePhoto(original, "photo-new")
	if got := original.GetPhotoIds(); len(got) != 1 || got[0] != "photo-old" {
		t.Fatalf("appendRecipePhoto mutated original photos: %#v", got)
	}
	if got := updated.GetPhotoIds(); len(got) != 2 || got[1] != "photo-new" {
		t.Fatalf("updated photos = %#v, want old and new", got)
	}
	if updated.GetNote() != original.GetNote() || updated.GetIdentifier() != original.GetIdentifier() {
		t.Fatal("appendRecipePhoto did not preserve non-photo fields")
	}
	if got := appendRecipePhoto(updated, "photo-new").GetPhotoIds(); len(got) != 2 {
		t.Fatalf("duplicate photo was appended: %#v", got)
	}

	withoutOld, removed := removeRecipePhoto(updated, "photo-old")
	if !removed || len(withoutOld.GetPhotoIds()) != 1 || withoutOld.GetPhotoIds()[0] != "photo-new" {
		t.Fatalf("removeRecipePhoto(old) = %#v, removed=%v", withoutOld.GetPhotoIds(), removed)
	}
	allCleared, removed := removeRecipePhoto(withoutOld, "")
	if !removed || len(allCleared.GetPhotoIds()) != 0 {
		t.Fatalf("removeRecipePhoto(all) = %#v, removed=%v", allCleared.GetPhotoIds(), removed)
	}
	if _, removed := removeRecipePhoto(allCleared, "missing"); removed {
		t.Fatal("removeRecipePhoto reported removal for a missing photo")
	}
}

func TestRecipesPhotoAttachPreviewIsOfflineAndStructured(t *testing.T) {
	t.Parallel()

	photoPath := filepath.Join(t.TempDir(), "recipe.jpg")
	if err := os.WriteFile(photoPath, []byte{0xff, 0xd8, 0xff, 0xe0, 'J', 'F', 'I', 'F'}, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	flags := &rootFlags{asJSON: true}
	cmd := newRecipesPhotoAttachCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--name", "Cedar Market Recipe", "--file", photoPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preview returned error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("preview output is not JSON: %v\n%s", err, out.String())
	}
	if result["dry_run"] != true || result["apply"] != false {
		t.Fatalf("preview result = %#v, want dry_run=true and apply=false", result)
	}
	if result["content_type"] != "image/jpeg" {
		t.Fatalf("content_type = %#v, want image/jpeg", result["content_type"])
	}
}

func TestRecipesPhotoClearPreviewRequiresNoNetwork(t *testing.T) {
	t.Parallel()

	flags := &rootFlags{asJSON: true}
	cmd := newRecipesPhotoClearCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--name", "Cedar Market Recipe", "--photo-id", "photo-old"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preview returned error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("preview output is not JSON: %v\n%s", err, out.String())
	}
	if result["dry_run"] != true || result["clear_all"] != false || result["photo_id"] != "photo-old" {
		t.Fatalf("preview result = %#v, want targeted dry run", result)
	}
}

func TestRecipesPhotoCommandIsRegistered(t *testing.T) {
	t.Parallel()

	cmd := newRecipesCmd(&rootFlags{})
	photo, _, err := cmd.Find([]string{"photo", "attach"})
	if err != nil {
		t.Fatalf("Find photo attach returned error: %v", err)
	}
	if photo == nil || photo.Name() != "attach" {
		t.Fatalf("photo command = %#v, want attach", photo)
	}
}

func TestRecipesPhotoAttachRequiresFile(t *testing.T) {
	t.Parallel()

	cmd := newRecipesPhotoAttachCmd(&rootFlags{})
	cmd.SetArgs([]string{"--name", "Pancakes"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), `required flag "file" not set`) {
		t.Fatalf("error = %v, want required file error", err)
	}
}
