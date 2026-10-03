// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/wheelog"
)

func wheelogWriteFixture(t *testing.T, path string) wheelog.Spot {
	t.Helper()
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.InitWheelog(context.Background()); err != nil {
		t.Fatal(err)
	}
	zero, two := 0, 2
	spot := wheelog.Spot{ID: 166345, Name: "Synthetic restroom", Category: "toilet", Questions: []wheelog.Question{{ID: 102, Positive: &two, Negative: &zero}}}
	if err = db.ObserveWheelog(context.Background(), spot); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	return spot
}

func TestWheelogWriterCanonicalSymlinkPreservesConcurrentSave(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "main.db")
	spot := wheelogWriteFixture(t, path)
	canonical, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer canonical.Close()
	zero, four := 0, 4
	spot.Questions[0].Positive = &zero
	if err = canonical.ObserveWheelog(ctx, spot); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(filepath.Dir(path), "alias.db")
	if err = os.Symlink(path, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	writer, err := OpenWheelogWithContext(ctx, alias)
	if err != nil {
		t.Fatal(err)
	}
	canonicalPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	if writer.Path() != canonicalPath {
		t.Fatalf("writer did not use canonical target: %s", writer.Path())
	}
	spot.Questions[0].Positive = &four
	if err = writer.ObserveWheelog(ctx, spot); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	items, err := canonical.WheelogList(ctx)
	if err != nil || len(items) != 1 || *items[0].Latest.Questions[0].Positive != 4 {
		t.Fatalf("concurrent writer did not see new save: %v (%v)", items, err)
	}
	if err = canonical.Close(); err != nil {
		t.Fatal(err)
	}
	items, err = ReadWheelogSnapshot(ctx, alias)
	if err != nil || len(items) != 1 || *items[0].Latest.Questions[0].Positive != 4 || *items[0].Previous.Questions[0].Positive != 0 {
		t.Fatalf("writer close lost newest transition: %v (%v)", items, err)
	}
}

func TestWheelogWriterRejectsHardLinkBeforeOpeningAlternateWAL(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "main.db")
	spot := wheelogWriteFixture(t, path)
	canonical, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer canonical.Close()
	zero := 0
	spot.Questions[0].Positive = &zero
	if err = canonical.ObserveWheelog(ctx, spot); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(filepath.Dir(path), "alias.db")
	if err = os.Link(path, alias); err != nil {
		t.Skipf("hard-link unavailable: %v", err)
	}
	_, err = OpenWheelogWithContext(ctx, alias)
	var visibility *WheelogSnapshotError
	if !errors.As(err, &visibility) {
		t.Fatalf("hard-link writer opened alternate WAL: %v", err)
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if _, err := os.Stat(alias + suffix); !os.IsNotExist(err) {
			t.Fatalf("alternate writer sidecar appeared: %s (%v)", suffix, err)
		}
	}
	if err = os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err = canonical.Close(); err != nil {
		t.Fatal(err)
	}
	items, err := ReadWheelogSnapshot(ctx, path)
	if err != nil || len(items) != 1 || *items[0].Latest.Questions[0].Positive != 0 {
		t.Fatalf("rejected alias changed canonical evidence: %v (%v)", items, err)
	}
}

func TestWheelogWriterRejectsAliasChangesBeforeEachMutation(t *testing.T) {
	for _, change := range []string{"hard-link-added", "selected-symlink-retargeted"} {
		t.Run(change, func(t *testing.T) {
			ctx := context.Background()
			dir := t.TempDir()
			path := filepath.Join(dir, "main.db")
			spot := wheelogWriteFixture(t, path)
			selection := path
			if change == "selected-symlink-retargeted" {
				selection = filepath.Join(dir, "selected.db")
				if err := os.Symlink(path, selection); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			}
			writer, err := OpenWheelogWithContext(ctx, selection)
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Close()
			if change == "hard-link-added" {
				if err = os.Link(path, filepath.Join(dir, "alias.db")); err != nil {
					t.Skipf("hard-link unavailable: %v", err)
				}
			} else {
				other := filepath.Join(dir, "other.db")
				if err = os.WriteFile(other, nil, 0600); err != nil {
					t.Fatal(err)
				}
				if err = os.Remove(selection); err != nil {
					t.Fatal(err)
				}
				if err = os.Symlink(other, selection); err != nil {
					t.Fatal(err)
				}
			}
			four := 4
			spot.Questions[0].Positive = &four
			for _, mutate := range []func() error{func() error { return writer.InitWheelog(ctx) }, func() error { return writer.ObserveWheelog(ctx, spot) }, func() error { return writer.RemoveWheelog(ctx, spot.ID) }} {
				var visibility *WheelogSnapshotError
				if err = mutate(); !errors.As(err, &visibility) {
					t.Fatalf("alias change permitted mutation: %v", err)
				}
			}
			items, err := writer.WheelogList(ctx)
			if err != nil || len(items) != 1 || *items[0].Latest.Questions[0].Positive != 2 {
				t.Fatalf("failed mutation changed evidence: %v (%v)", items, err)
			}
		})
	}
}

func TestWheelogWriterRejectsURIPathsBeforeTouchingOtherCaches(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	selected, other := filepath.Join(dir, "saved?literal.db"), filepath.Join(dir, "saved")
	// Create under ordinary names, then rename; the generated writable DSN
	// cannot safely create URI-delimiter paths.
	original := filepath.Join(dir, "original.db")
	wheelogWriteFixture(t, original)
	if err := os.Rename(original, selected); err != nil {
		t.Fatal(err)
	}
	wheelogWriteFixture(t, other)
	for _, path := range []string{selected, filepath.Join(dir, "new?literal.db"), filepath.Join(dir, "new#literal.db")} {
		_, err := OpenWheelogWithContext(ctx, path)
		var visibility *WheelogSnapshotError
		if !errors.As(err, &visibility) {
			t.Fatalf("ambiguous URI path opened: %s (%v)", path, err)
		}
	}
	for _, path := range []string{selected, other} {
		items, err := ReadWheelogSnapshot(ctx, path)
		if err != nil || len(items) != 1 || *items[0].Latest.Questions[0].Positive != 2 {
			t.Fatalf("wrong cache modified: %s %v (%v)", path, items, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "new")); !os.IsNotExist(err) {
		t.Fatalf("ambiguous path created foreign cache: %v", err)
	}
}
