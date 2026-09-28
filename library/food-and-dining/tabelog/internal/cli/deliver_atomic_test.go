package cli

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestWriteDownloadUnderLeavesPredictableTempSymlinkUntouched(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "downloads")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(root, "sentinel")
	if err := os.WriteFile(sentinel, []byte("keep this unrelated file"), 0600); err != nil {
		t.Fatal(err)
	}
	oldTemp := filepath.Join(dir, "trip.json.tmp")
	if err := os.Symlink(sentinel, oldTemp); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	body := []byte("complete intended output")
	if err := writeDownloadUnder(dir, "trip.json", body); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(sentinel)
	if err != nil || string(got) != "keep this unrelated file" {
		t.Fatalf("predictable temporary symlink clobbered the sentinel: %q, %v", got, err)
	}
	got, err = os.ReadFile(filepath.Join(dir, "trip.json"))
	if err != nil || !bytes.Equal(got, body) {
		t.Fatalf("intended output is incomplete: %q, %v", got, err)
	}
	info, err := os.Lstat(oldTemp)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("unused predictable symlink was replaced: %v", err)
	}
}

func TestWriteDownloadUnderConcurrentOutputsRemainComplete(t *testing.T) {
	dir := t.TempDir()
	const writers = 16
	bodies := make([][]byte, writers)
	valid := map[[sha256.Size]byte]bool{}
	for i := range bodies {
		bodies[i] = append([]byte(fmt.Sprintf("writer %02d\n", i)), bytes.Repeat([]byte{byte('A' + i)}, 256<<10)...)
		valid[sha256.Sum256(bodies[i])] = true
	}
	if err := writeDownloadUnder(dir, "trip.json", bodies[0]); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	start, done := make(chan struct{}), make(chan struct{})
	errors := make(chan error, writers)
	for _, body := range bodies {
		wg.Add(1)
		go func(body []byte) {
			defer wg.Done()
			<-start
			errors <- writeDownloadUnder(dir, "trip.json", body)
		}(body)
	}
	go func() { wg.Wait(); close(done) }()
	close(start)
	var readErr error
reading:
	for {
		select {
		case <-done:
			break reading
		default:
			body, err := os.ReadFile(filepath.Join(dir, "trip.json"))
			if err != nil || !valid[sha256.Sum256(body)] {
				readErr = fmt.Errorf("reader observed incomplete output: bytes=%d error=%v", len(body), err)
				break reading
			}
		}
	}
	<-done
	close(errors)
	for err := range errors {
		if err != nil {
			t.Errorf("concurrent delivery failed: %v", err)
		}
	}
	if readErr != nil {
		t.Error(readErr)
	}
	body, err := os.ReadFile(filepath.Join(dir, "trip.json"))
	if err != nil || !valid[sha256.Sum256(body)] {
		t.Fatalf("final output is incomplete: bytes=%d error=%v", len(body), err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "trip.json" {
		t.Fatalf("delivery leaked temporary files: %+v, %v", entries, err)
	}
}
