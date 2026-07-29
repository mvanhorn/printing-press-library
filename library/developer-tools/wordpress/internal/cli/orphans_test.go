// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelOrphansHelpWires smoke-tests that the orphans command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelOrphansHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"orphans", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("orphans --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "orphans"} {
		if !strings.Contains(help, want) {
			t.Fatalf("orphans --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestMediaReferencedByContent(t *testing.T) {
	tests := []struct {
		name      string
		mediaURL  string
		content   []string
		wantFound bool
	}{
		{
			name:      "resized media matches original reference",
			mediaURL:  "https://example.test/wp-content/uploads/2026/07/image-300x200.jpg",
			content:   []string{`<img src="https://example.test/wp-content/uploads/2026/07/image.jpg">`},
			wantFound: true,
		},
		{
			name:      "original media matches resized CDN reference",
			mediaURL:  "http://example.test/wp-content/uploads/2026/07/image.jpg",
			content:   []string{`<img src="https://cdn.example.test/wp-content/uploads/2026/07/image-1024x768.jpg">`},
			wantFound: true,
		},
		{
			name:      "CDN path prefix is ignored before uploads path",
			mediaURL:  "https://example.test/wp-content/uploads/2026/07/image.jpg",
			content:   []string{`<img src="https://cdn.example.test/cache/site/wp-content/uploads/2026/07/image-640x480.jpg">`},
			wantFound: true,
		},
		{
			name:      "same basename in another directory does not match",
			mediaURL:  "https://example.test/wp-content/uploads/2026/07/image.jpg",
			content:   []string{`<img src="https://example.test/wp-content/uploads/2025/07/image.jpg">`},
			wantFound: false,
		},
		{
			name:      "different file does not match",
			mediaURL:  "https://example.test/wp-content/uploads/2026/07/image.jpg",
			content:   []string{`<img src="https://example.test/wp-content/uploads/2026/07/other.jpg">`},
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mediaReferencedByContent(tt.mediaURL, tt.content); got != tt.wantFound {
				t.Fatalf("mediaReferencedByContent() = %v, want %v", got, tt.wantFound)
			}
		})
	}
}

func TestOrphansLocalReadsAreDrainFirstAndNullSafe(t *testing.T) {
	ctx, db := openLocalCommandTestStore(t)
	insertLocalCommandResource(t, ctx, db, "media", "30", `{
		"id": 30,
		"source_url": "https://example.test/wp-content/uploads/image.jpg"
	}`)
	insertLocalCommandResource(t, ctx, db, "posts", "31", `{"id": 31}`)

	mediaRows, err := loadOrphanMedia(ctx, db)
	if err != nil {
		t.Fatalf("loadOrphanMedia() error = %v", err)
	}
	if len(mediaRows) != 1 || mediaRows[0].Filesize.Valid {
		t.Fatalf("loadOrphanMedia() rows = %+v, want one row with NULL filesize", mediaRows)
	}
	contentRows, err := loadOrphanContent(ctx, db)
	if err != nil {
		t.Fatalf("loadOrphanContent() after drained media error = %v", err)
	}
	if len(contentRows) != 1 || contentRows[0].FeaturedMedia.Valid || contentRows[0].Content != "" {
		t.Fatalf("loadOrphanContent() rows = %+v, want NULL-safe empty fields", contentRows)
	}
}
