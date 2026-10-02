package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteDownloadUnderIgnoresPredictableTempSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows")
	}
	dir := t.TempDir()
	victim := filepath.Join(t.TempDir(), "unrelated.txt")
	if err := os.WriteFile(victim, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(dir, "report.txt.tmp")); err != nil {
		t.Fatal(err)
	}
	if err := writeDownloadUnder(dir, "report.txt", []byte("delivered")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(victim)
	if err != nil || string(got) != "unchanged" {
		t.Fatalf("predictable temporary symlink modified another file: %q, %v", got, err)
	}
	got, err = os.ReadFile(filepath.Join(dir, "report.txt"))
	if err != nil || string(got) != "delivered" {
		t.Fatalf("delivery contents = %q, %v", got, err)
	}
}

func TestConcurrentDownloadsReplaceOneCompletePrivateFile(t *testing.T) {
	dir := t.TempDir()
	errors := make(chan error, 24)
	want := make(map[string]bool)
	for i := 0; i < cap(errors); i++ {
		body := bytes.Repeat([]byte(fmt.Sprintf("%02d", i)), 4096)
		want[string(body)] = true
		go func() { errors <- writeDownloadUnder(dir, "report.txt", body) }()
	}
	for i := 0; i < cap(errors); i++ {
		if err := <-errors; err != nil {
			t.Errorf("concurrent delivery: %v", err)
		}
	}
	got, err := os.ReadFile(filepath.Join(dir, "report.txt"))
	if err != nil || !want[string(got)] {
		t.Fatalf("final file is not one complete delivery: %d bytes, %v", len(got), err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "report.txt" {
		t.Fatalf("temporary deliveries were not cleaned up: %v, %v", entries, err)
	}
}

func TestWriteDownloadUnderAcceptsLongValidDestinationName(t *testing.T) {
	dir := t.TempDir()
	name := strings.Repeat("r", 250)
	if err := writeDownloadUnder(dir, name, []byte("delivered")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil || string(got) != "delivered" {
		t.Fatalf("long destination contents = %q, %v", got, err)
	}
}
