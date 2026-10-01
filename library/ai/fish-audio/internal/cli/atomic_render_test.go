package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
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

func TestWriteAudioFileShortWritePreservesExistingOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "render.mp3")
	if err := os.WriteFile(path, []byte("existing-render"), 0o600); err != nil {
		t.Fatalf("seed output: %v", err)
	}
	_, err := writeAudioFileWith(path, []byte("replacement-audio"), func(file *os.File, data []byte) (int, error) {
		return file.Write(data[:4])
	})
	if err == nil {
		t.Fatal("short write unexpectedly succeeded")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "existing-render" {
		t.Fatalf("output after short write = %q, err=%v", got, err)
	}
	temps, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".render.mp3.tmp-*"))
	if err != nil || len(temps) != 0 {
		t.Fatalf("temporary files after short write = %v, err=%v", temps, err)
	}
}

func TestWriteAudioFileFollowsSymlinkAndPreservesMode(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.mp3")
	link := filepath.Join(dir, "render.mp3")
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	if err := os.Chmod(target, 0o644); err != nil {
		t.Fatalf("set target permissions: %v", err)
	}
	if err := os.Symlink("target.mp3", link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink creation unavailable: %v", err)
		}
		t.Fatalf("create symlink: %v", err)
	}
	if _, err := writeAudioFile(link, []byte("new-audio")); err != nil {
		t.Fatalf("write via symlink: %v", err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("output link was replaced: info=%v err=%v", info, err)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "new-audio" {
		t.Fatalf("target output = %q, err=%v", got, err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(target)
		if err != nil || info.Mode().Perm() != 0o644 {
			t.Fatalf("target permissions = %v, err=%v, want 0644", info, err)
		}
	}
}

func TestRenameAudioFileWithRetryWindowsSharingViolation(t *testing.T) {
	dir := t.TempDir()
	tmp := filepath.Join(dir, "temporary")
	path := filepath.Join(dir, "render.mp3")
	if err := os.WriteFile(tmp, []byte("new-audio"), 0o600); err != nil {
		t.Fatalf("seed temporary output: %v", err)
	}
	attempts := 0
	err := renameAudioFileWithRetryFunc(func(source, destination string) error {
		attempts++
		if attempts == 1 {
			return syscall.Errno(32)
		}
		return os.Rename(source, destination)
	}, tmp, path, "windows")
	if err != nil || attempts != 2 {
		t.Fatalf("rename error=%v attempts=%d, want two attempts and success", err, attempts)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "new-audio" {
		t.Fatalf("published output = %q, err=%v", got, err)
	}
}
