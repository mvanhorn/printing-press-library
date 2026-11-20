package cobratree

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFMirrorConcurrency(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "child.sh")
	release := filepath.Join(dir, "release")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho started > \"$1\"\nwhile [ ! -e \"$2\" ]; do sleep 0.01; done\n"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 4)
	// Always release and reap only our own children, including on assertion failures.
	defer func() {
		os.WriteFile(release, nil, 0600)
		for i := 0; i < 4; i++ {
			if err := <-done; err != nil {
				t.Errorf("child: %v", err)
			}
		}
	}()
	for i := 0; i < 4; i++ {
		go func(i int) {
			_, err := RunCLICommand(ctx, script, []string{filepath.Join(dir, fmt.Sprint(i)), release})
			done <- err
		}(i)
	}
	count := func() int {
		n := 0
		for i := 0; i < 4; i++ {
			if _, err := os.Stat(filepath.Join(dir, fmt.Sprint(i))); err == nil {
				n++
			}
		}
		return n
	}
	deadline := time.Now().Add(time.Second)
	for count() < 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if count() != 3 {
		t.Fatalf("running children=%d want 3", count())
	}
	time.Sleep(50 * time.Millisecond)
	if count() != 3 {
		t.Fatalf("fourth shellout started before a slot was released: %d", count())
	}
}
