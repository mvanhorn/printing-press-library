package zohotools

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestHashFile_DeterministicForSameContent(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.pdf")
	b := filepath.Join(dir, "b.pdf")
	payload := []byte("fake receipt content for hash test")
	if err := os.WriteFile(a, payload, 0o600); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(b, payload, 0o600); err != nil {
		t.Fatalf("write b: %v", err)
	}

	ha, err := HashFile(a)
	if err != nil {
		t.Fatalf("hash a: %v", err)
	}
	hb, err := HashFile(b)
	if err != nil {
		t.Fatalf("hash b: %v", err)
	}
	if ha != hb {
		t.Errorf("identical content must hash equal, got %s vs %s", ha, hb)
	}

	sum := sha256.Sum256(payload)
	want := hex.EncodeToString(sum[:])
	if ha != want {
		t.Errorf("HashFile(a) = %s, want %s", ha, want)
	}
}

func TestHashFile_DifferentContentDiffers(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.pdf")
	b := filepath.Join(dir, "b.pdf")
	if err := os.WriteFile(a, []byte("one"), 0o600); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(b, []byte("two"), 0o600); err != nil {
		t.Fatalf("write b: %v", err)
	}
	ha, _ := HashFile(a)
	hb, _ := HashFile(b)
	if ha == hb {
		t.Errorf("different content must hash differently, got %s == %s", ha, hb)
	}
}

func TestHashFile_MissingFile(t *testing.T) {
	if _, err := HashFile("/nonexistent/path/that/does/not/exist"); err == nil {
		t.Errorf("expected error for missing file")
	}
}
