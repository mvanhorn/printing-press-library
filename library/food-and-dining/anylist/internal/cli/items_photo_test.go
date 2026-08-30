// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestItemsPhotoAttachDryRunEmitsValidatedPreview(t *testing.T) {
	t.Parallel()

	photoPath := filepath.Join(t.TempDir(), "photo.bin")
	if err := os.WriteFile(photoPath, []byte{0xff, 0xd8, 0xff, 0xe0, 'J', 'F', 'I', 'F'}, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	flags := &rootFlags{asJSON: true, dryRun: true}
	cmd := newItemsPhotoAttachCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--list", "Groceries", "--item", "Milk", "--file", photoPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("dry-run output is not JSON: %v\n%s", err, out.String())
	}
	if got, ok := payload["dry_run"].(bool); !ok || !got {
		t.Fatalf("dry_run = %#v, want true", payload["dry_run"])
	}
	if got := payload["content_type"]; got != "image/jpeg" {
		t.Fatalf("content_type = %#v, want image/jpeg", got)
	}
	if got := payload["apply"]; got != false {
		t.Fatalf("apply = %#v, want false", got)
	}
}

func TestItemsPhotoAttachRequiresExplicitApply(t *testing.T) {
	t.Parallel()

	photoPath := filepath.Join(t.TempDir(), "photo.jpg")
	if err := os.WriteFile(photoPath, []byte{0xff, 0xd8, 0xff, 0xe0, 'J', 'F', 'I', 'F'}, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	cmd := newItemsPhotoAttachCmd(&rootFlags{})
	cmd.SetArgs([]string{"--list", "Groceries", "--item", "Milk", "--file", photoPath})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "pass --apply") {
		t.Fatalf("error = %v, want explicit apply gate", err)
	}
}

func TestItemsPhotoCommandIsRegistered(t *testing.T) {
	t.Parallel()

	cmd := newItemsCmd(&rootFlags{})
	photo, _, err := cmd.Find([]string{"photo", "attach"})
	if err != nil {
		t.Fatalf("Find photo attach returned error: %v", err)
	}
	if photo == nil || photo.Name() != "attach" {
		t.Fatalf("photo command = %#v, want attach", photo)
	}
}
