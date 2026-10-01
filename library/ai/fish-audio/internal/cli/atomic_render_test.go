package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAudioFileFailurePreservesExistingOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "render.mp3")
	old := []byte("existing-valid-render")
	if err := os.WriteFile(path, old, 0o600); err != nil {
		t.Fatalf("seed output: %v", err)
	}

	_, err := writeAudioFileWith(path, []byte("replacement-audio"), func(file *os.File, data []byte) (int, error) {
		if _, writeErr := file.Write(data[:4]); writeErr != nil {
			return 0, writeErr
		}
		return 4, errors.New("injected write failure")
	})
	if err == nil {
		t.Fatal("writeAudioFileWith unexpectedly succeeded")
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read preserved output: %v", readErr)
	}
	if string(got) != string(old) {
		t.Fatalf("existing output changed to %q, want %q", got, old)
	}
	temps, globErr := filepath.Glob(filepath.Join(filepath.Dir(path), ".render.mp3.tmp-*"))
	if globErr != nil {
		t.Fatalf("glob temp outputs: %v", globErr)
	}
	if len(temps) != 0 {
		t.Fatalf("temporary outputs leaked: %v", temps)
	}

	replacement := []byte("replacement-audio")
	digest, err := writeAudioFile(path, replacement)
	if err != nil {
		t.Fatalf("write replacement: %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != string(replacement) {
		t.Fatalf("replacement output = %q, err = %v", got, err)
	}
	want := sha256.Sum256(replacement)
	if digest != hex.EncodeToString(want[:]) {
		t.Fatalf("digest = %s, want %x", digest, want)
	}
}
