package client

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A non-image file (read succeeds, content is not a raster image) must produce
// an actionable error naming the cause, not a bare MIME type — this is the
// dead-end that caused repeated publish retries in real use.
func TestUploadArticleImageRejectsNonImageWithDiagnosis(t *testing.T) {
	dir := t.TempDir()

	lfs := filepath.Join(dir, "illo.png")
	lfsData := []byte("version https://git-lfs.github.com/spec/v1\noid sha256:abc\nsize 12345\n")
	if err := os.WriteFile(lfs, lfsData, 0o600); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(dir, "note.png")
	plainData := []byte("this is just some text, not an image at all")
	if err := os.WriteFile(plain, plainData, 0o600); err != nil {
		t.Fatal(err)
	}

	c := &Client{}

	_, err := c.UploadArticleImage(context.Background(), lfs)
	if err == nil {
		t.Fatal("expected error for Git LFS pointer, got nil")
	}
	if !strings.Contains(err.Error(), "Git LFS pointer") || !strings.Contains(err.Error(), "git lfs pull") ||
		!strings.Contains(err.Error(), lfs) ||
		!strings.Contains(err.Error(), fmt.Sprintf("%d bytes", len(lfsData))) ||
		!strings.Contains(err.Error(), http.DetectContentType(lfsData)) {
		t.Fatalf("LFS error should name the cause and the fix, got: %v", err)
	}

	_, err = c.UploadArticleImage(context.Background(), plain)
	if err == nil {
		t.Fatal("expected error for text file, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "not a supported image") {
		t.Fatalf("error should explain it is not a supported image, got: %v", err)
	}
	if !strings.Contains(msg, plain) {
		t.Fatalf("error should name the resolved path, got: %v", err)
	}
	if !strings.Contains(msg, fmt.Sprintf("%d bytes", len(plainData))) ||
		!strings.Contains(msg, http.DetectContentType(plainData)) {
		t.Fatalf("error should name the byte count and detected MIME type, got: %v", err)
	}
	// Bare MIME type alone (the old message) is not actionable.
	if msg == `unsupported image type "text/plain; charset=utf-8"` {
		t.Fatalf("error regressed to the non-actionable form: %v", err)
	}
}
