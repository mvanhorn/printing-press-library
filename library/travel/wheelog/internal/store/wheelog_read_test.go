// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/wheelog"
)

func TestWheelogSnapshotRejectsCommittedWALThenReadsNewestPair(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "cache.db")
	writer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = writer.InitWheelog(ctx); err != nil {
		t.Fatal(err)
	}
	zero, two := 0, 2
	spot := wheelog.Spot{ID: 166345, Name: "Initial recorded name", Category: "toilet", ObservedAt: time.Now().UTC().Format(time.RFC3339), Questions: []wheelog.Question{{ID: 102, Positive: &two, Negative: &zero, State: "reported_affirmative"}}, Gaps: []string{}}
	if err = writer.ObserveWheelog(ctx, spot); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	writer, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	spot.Name = "Newest recorded name"
	spot.Questions[0].Positive = &zero
	spot.Questions[0].State = "unreported"
	if err = writer.ObserveWheelog(ctx, spot); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path + "-wal")
	if err != nil || info.Size() == 0 {
		t.Fatalf("missing committed WAL: %v", err)
	}
	items, err := ReadWheelogSnapshot(ctx, path)
	var visibility *WheelogSnapshotError
	if len(items) != 0 || !errors.As(err, &visibility) {
		t.Fatalf("active writer silently exposed old facts: %v %v", items, err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	// URI-reserved characters in a selected saved path must remain path bytes.
	newPath := filepath.Join(filepath.Dir(path), "saved ?#.db")
	if err = os.Rename(path, newPath); err != nil {
		t.Fatal(err)
	}
	items, err = ReadWheelogSnapshot(ctx, newPath)
	if err != nil || len(items) != 1 || items[0].Latest.Name != "Newest recorded name" || *items[0].Latest.Questions[0].Positive != 0 || items[0].Previous == nil || *items[0].Previous.Questions[0].Positive != 2 {
		t.Fatalf("newest committed pair lost after writer close: %+v (%v)", items, err)
	}
}

func TestWheelogSnapshotMissingOrUninitializedDoesNotCreateState(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "absent-directory", "saved.db")
	items, err := ReadWheelogSnapshot(ctx, path)
	if err != nil || items == nil || len(items) != 0 {
		t.Fatalf("missing shortlist: %v (%v)", items, err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatal("saved read created a directory")
	}
	path = filepath.Join(t.TempDir(), "old-framework.db")
	writer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	items, err = ReadWheelogSnapshot(ctx, path)
	if err != nil || len(items) != 0 {
		t.Fatalf("uninitialized shortlist: %v (%v)", items, err)
	}
	reader, err := OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var count int
	if err = reader.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='wheelog_shortlist'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("saved read created its table: %d (%v)", count, err)
	}
}

func TestWheelogSnapshotAliasesCannotHideCommittedWAL(t *testing.T) {
	for _, kind := range []string{"symlink", "hard-link"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "main.db")
			writer, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			if err = writer.InitWheelog(ctx); err != nil {
				t.Fatal(err)
			}
			zero, two := 0, 2
			spot := wheelog.Spot{ID: 166345, Name: "Synthetic restroom", Category: "toilet", Questions: []wheelog.Question{{ID: 102, Positive: &two, Negative: &zero}}}
			if err = writer.ObserveWheelog(ctx, spot); err != nil {
				t.Fatal(err)
			}
			if err = writer.Close(); err != nil {
				t.Fatal(err)
			}
			alias := filepath.Join(filepath.Dir(path), "alias.db")
			if kind == "symlink" {
				err = os.Symlink(path, alias)
			} else {
				err = os.Link(path, alias)
			}
			if err != nil {
				t.Skipf("alias creation unavailable: %v", err)
			}
			if kind == "hard-link" {
				info, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				singleLink, err := wheelogFileHasSingleLink(path, info)
				if err != nil || singleLink {
					t.Fatalf("hard-link metadata unavailable: single_link=%v err=%v", singleLink, err)
				}
			}
			writer, err = Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Close()
			spot.Questions[0].Positive = &zero
			if err = writer.ObserveWheelog(ctx, spot); err != nil {
				t.Fatal(err)
			}
			items, err := ReadWheelogSnapshot(ctx, alias)
			var visibility *WheelogSnapshotError
			if len(items) != 0 || !errors.As(err, &visibility) {
				t.Fatalf("%s exposed stale positive counts: %v (%v)", kind, items, err)
			}
			if err = writer.Close(); err != nil {
				t.Fatal(err)
			}
			if kind == "hard-link" {
				if _, err = ReadWheelogSnapshot(ctx, alias); !errors.As(err, &visibility) {
					t.Fatalf("hard-link safety lost after close: %v", err)
				}
			} else {
				items, err = ReadWheelogSnapshot(ctx, alias)
				if err != nil || len(items) != 1 || *items[0].Latest.Questions[0].Positive != 0 {
					t.Fatalf("canonical symlink lost latest counts: %v (%v)", items, err)
				}
			}
		})
	}
}

func TestWheelogReadGuardDetectsSelectedAliasRetargeting(t *testing.T) {
	dir := t.TempDir()
	first, second, alias := filepath.Join(dir, "first.db"), filepath.Join(dir, "second.db"), filepath.Join(dir, "alias.db")
	for _, p := range []string{first, second} {
		if err := os.WriteFile(p, []byte("saved image"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(first, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	guard, err := wheelogReadState(first, wheelogFileInfo)
	if err != nil {
		t.Fatal(err)
	}
	guard.selection = alias
	if err = os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(second, alias); err != nil {
		t.Fatal(err)
	}
	var visibility *WheelogSnapshotError
	if err = guard.check(wheelogFileInfo); !errors.As(err, &visibility) {
		t.Fatalf("selected alias retargeting hidden: %v", err)
	}
}

func TestWheelogSnapshotSQLCannotReadSubstitutedRows(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path, replacement, backup := filepath.Join(dir, "selected.db"), filepath.Join(dir, "replacement.db"), filepath.Join(dir, "original.db")
	wheelogWriteFixture(t, path)
	spot := wheelogWriteFixture(t, replacement)
	db, err := Open(replacement)
	if err != nil {
		t.Fatal(err)
	}
	zero := 0
	spot.Questions[0].Positive = &zero
	if err = db.ObserveWheelog(ctx, spot); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	items, err := readWheelogSnapshot(ctx, path, func(ctx context.Context, reader *Store) ([]WheelogObservation, error) {
		if err := os.Rename(path, backup); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(replacement, path); err != nil {
			t.Fatal(err)
		}
		rows, queryErr := wheelogSnapshotRows(ctx, reader)
		if err := os.Rename(path, replacement); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(backup, path); err != nil {
			t.Fatal(err)
		}
		return rows, queryErr
	})
	if err != nil || len(items) != 1 || *items[0].Latest.Questions[0].Positive != 2 {
		t.Fatalf("SQL consumed substituted cache rows: %v (%v)", items, err)
	}
}

func TestWheelogPrivateSnapshotCancellationAndCleanup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selected.db")
	wheelogWriteFixture(t, path)
	private := t.TempDir()
	t.Setenv("TMPDIR", private)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadWheelogSnapshot(ctx, path); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled snapshot read succeeded: %v", err)
	}
	if _, err := ReadWheelogSnapshot(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(private)
	if err != nil || len(entries) != 0 {
		t.Fatalf("private snapshot leaked: %v (%v)", entries, err)
	}
}

func TestWheelogPrivateSnapshotEscapesTemporaryRootURI(t *testing.T) {
	dir := t.TempDir()
	selected := filepath.Join(dir, "selected.db")
	wheelogWriteFixture(t, selected)
	wrong := filepath.Join(dir, "temp")
	spot := wheelogWriteFixture(t, wrong)
	db, err := Open(wrong)
	if err != nil {
		t.Fatal(err)
	}
	zero := 0
	spot.Questions[0].Positive = &zero
	if err = db.ObserveWheelog(context.Background(), spot); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	temporary := filepath.Join(dir, "temp?literal#root")
	if err = os.Mkdir(temporary, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", temporary)
	items, err := ReadWheelogSnapshot(context.Background(), selected)
	if err != nil || len(items) != 1 || *items[0].Latest.Questions[0].Positive != 2 {
		t.Fatalf("private snapshot consumed truncated temporary-root cache: %v (%v)", items, err)
	}
	entries, err := os.ReadDir(temporary)
	if err != nil || len(entries) != 0 {
		t.Fatalf("URI temporary snapshot leaked: %v (%v)", entries, err)
	}
	items, err = ReadWheelogSnapshot(context.Background(), wrong)
	if err != nil || len(items) != 1 || *items[0].Latest.Questions[0].Positive != 0 {
		t.Fatalf("private snapshot mutated unselected temp cache: %v (%v)", items, err)
	}
}

func TestWheelogReadGuardRejectsReplacementJournalAndCheckpointRace(t *testing.T) {
	for _, change := range []string{"replacement", "wal", "journal", "checkpoint-race"} {
		t.Run(change, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "saved.db")
			if err := os.WriteFile(path, []byte("initial bytes"), 0600); err != nil {
				t.Fatal(err)
			}
			guard, err := wheelogReadState(path, wheelogFileInfo)
			if err != nil {
				t.Fatal(err)
			}
			stat := wheelogFileInfo
			switch change {
			case "replacement":
				if err = os.WriteFile(path+".new", []byte("initial bytes"), 0600); err != nil {
					t.Fatal(err)
				}
				if err = os.Chtimes(path+".new", guard.main.ModTime(), guard.main.ModTime()); err != nil {
					t.Fatal(err)
				}
				err = os.Rename(path+".new", path)
			case "wal", "journal":
				err = os.WriteFile(path+"-"+change, []byte("active state"), 0600)
			case "checkpoint-race":
				changed := false
				stat = func(p string) (os.FileInfo, error) {
					if p == path+"-wal" && !changed {
						changed = true
						if err := os.WriteFile(path, []byte("new checkpointed bytes"), 0600); err != nil {
							return nil, err
						}
					}
					return wheelogFileInfo(p)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			var visibility *WheelogSnapshotError
			if err = guard.check(stat); !errors.As(err, &visibility) {
				t.Fatalf("%s hidden: %v", change, err)
			}
		})
	}
}
