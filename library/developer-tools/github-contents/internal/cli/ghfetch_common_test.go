// Copyright 2026 Rick van de Laar and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-contents/internal/ghfetch"
)

// TestDeadlineForTransfer covers the download-phase context-selection
// rule: the DEFAULT --timeout must not cap bulk downloads (a 1.92 GB
// fetch died at exactly the 60s default before this rule existed), while
// an EXPLICIT --timeout is honored for the whole run.
func TestDeadlineForTransfer(t *testing.T) {
	t.Parallel()

	t.Run("default timeout does not bound the download phase", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := deadlineForTransfer(context.Background(), time.Minute, false)
		defer cancel()
		if _, ok := ctx.Deadline(); ok {
			t.Fatal("download phase must have no deadline when --timeout was not explicitly set")
		}
	})

	t.Run("explicit timeout bounds the download phase", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := deadlineForTransfer(context.Background(), time.Minute, true)
		defer cancel()
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("explicitly-set --timeout must bound the download phase")
		}
	})

	t.Run("explicit zero timeout means no deadline", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := deadlineForTransfer(context.Background(), 0, true)
		defer cancel()
		if _, ok := ctx.Deadline(); ok {
			t.Fatal("explicit zero --timeout must not add a deadline")
		}
	})

	t.Run("cancel propagates from parent", func(t *testing.T) {
		t.Parallel()
		parent, parentCancel := context.WithCancel(context.Background())
		ctx, cancel := deadlineForTransfer(parent, time.Minute, false)
		defer cancel()
		parentCancel()
		select {
		case <-ctx.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("parent cancellation did not propagate to the download-phase context")
		}
	})
}

// TestListLocalFilesSkipsPartialTemps proves that both temp-name shapes —
// the legacy "X.partial" suffix AND ghfetch.StreamToFile's randomized
// "X.partial-NNNN" — are excluded from local listings, so a crash-orphaned
// temp never surfaces as an "extra" file in verify/sync-dir.
func TestListLocalFilesSkipsPartialTemps(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFixture := func(rel, content string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("fixture mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatalf("fixture write: %v", err)
		}
	}

	writeFixture("real.pdf", "content")
	writeFixture("sub/nested.txt", "content")
	writeFixture("legacy.bin.partial", "in-flight legacy temp")
	// A genuine StreamToFile orphan: create it exactly the way StreamToFile
	// would, by letting os.CreateTemp pick the randomized suffix.
	orphan, err := os.CreateTemp(dir, "crashed.bin.partial-*")
	if err != nil {
		t.Fatalf("fixture temp: %v", err)
	}
	if _, err := orphan.WriteString("orphaned mid-download"); err != nil {
		t.Fatalf("fixture temp write: %v", err)
	}
	_ = orphan.Close()
	if !strings.Contains(filepath.Base(orphan.Name()), ".partial-") {
		t.Fatalf("fixture temp %q does not carry the StreamToFile temp shape", orphan.Name())
	}

	got, err := listLocalFiles(dir)
	if err != nil {
		t.Fatalf("listLocalFiles: %v", err)
	}
	want := []string{"real.pdf", "sub/nested.txt"}
	if len(got) != len(want) {
		t.Fatalf("listLocalFiles = %v, want %v (temps must be skipped)", got, want)
	}
	for _, w := range want {
		found := false
		for _, g := range got {
			if g == w {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("listLocalFiles = %v, missing %q", got, w)
		}
	}
}

// TestListLocalFilesOrphanFromRealStreamToFileFailure goes one step
// further: it produces the orphan via an actual failed StreamToFile call
// (size mismatch) with removal-after-failure suppressed being impossible,
// so instead it validates the inverse — a FAILED StreamToFile leaves no
// temp behind at all.
func TestListLocalFilesOrphanFromRealStreamToFileFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if _, err := ghfetch.StreamToFile(strings.NewReader("abc"), filepath.Join(dir, "x.bin"), 999); err == nil {
		t.Fatal("StreamToFile size mismatch: want error")
	}
	got, err := listLocalFiles(dir)
	if err != nil {
		t.Fatalf("listLocalFiles: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("listLocalFiles = %v, want empty (failed stream must clean up)", got)
	}
}
