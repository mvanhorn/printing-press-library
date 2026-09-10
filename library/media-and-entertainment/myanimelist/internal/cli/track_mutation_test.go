package cli

import (
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/myanimelist/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/myanimelist/internal/store"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// seedLibrary adds one real title so the mutation tests can tell an existing
// row from an invented one.
func seedLibrary(t *testing.T, dbPath string) {
	t.Helper()
	if _, _, err := runRootArgs(t, "track", "add", "52991", "--title", "Fixture Show", "--status", "watching", "--db", dbPath, "--json"); err != nil {
		t.Fatalf("seeding track add: %v", err)
	}
}

func libraryEntries(t *testing.T, dbPath string) []malLibraryEntry {
	t.Helper()
	stdout, stderr, err := runRootArgs(t, "track", "list", "--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("track list: %v (stderr=%s)", err, stderr)
	}
	var entries []malLibraryEntry
	if err := json.Unmarshal([]byte(stdout), &entries); err != nil {
		t.Fatalf("decoding track list output %q: %v", stdout, err)
	}
	return entries
}

// TestTrackMutationsRejectAbsentIDs is the regression guard for blank-row
// creation: rate, note and drop (like progress) must refuse an id the user has
// not added, with the documented usage error and exit code 2.
func TestTrackMutationsRejectAbsentIDs(t *testing.T) {
	testenv.Isolate(t)

	dbPath := filepath.Join(t.TempDir(), "library.db")
	seedLibrary(t, dbPath)

	cases := []struct {
		name string
		args []string
	}{
		{"rate", []string{"track", "rate", "99999", "--score", "5", "--db", dbPath, "--json"}},
		{"note", []string{"track", "note", "99999", "--text", "hello", "--db", dbPath, "--json"}},
		{"drop", []string{"track", "drop", "99999", "--db", dbPath, "--json"}},
		{"progress", []string{"track", "progress", "99999", "--episodes", "1", "--db", dbPath, "--json"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := runRootArgs(t, tc.args...)
			if err == nil {
				t.Fatalf("%s on an unadded id: err = nil, want a usage error", tc.name)
			}
			if code := ExitCode(err); code != 2 {
				t.Fatalf("%s exit code = %d, want 2 (err = %v)", tc.name, code, err)
			}
			if !strings.Contains(err.Error(), "is not in the local library") {
				t.Fatalf("%s error = %v, want the not-in-library message", tc.name, err)
			}
			if !strings.Contains(err.Error(), "track add") {
				t.Fatalf("%s error = %v, want a hint naming `track add`", tc.name, err)
			}
		})
	}

	entries := libraryEntries(t, dbPath)
	if len(entries) != 1 || entries[0].ID != 52991 {
		t.Fatalf("library = %+v, want only the seeded 52991 row; a mutation invented a row", entries)
	}

	// The guard must not block legitimate updates to an existing row.
	if _, _, err := runRootArgs(t, "track", "rate", "52991", "--score", "9", "--db", dbPath, "--json"); err != nil {
		t.Fatalf("rating an existing title: %v", err)
	}
	if _, _, err := runRootArgs(t, "track", "note", "52991", "--text", "watch subbed", "--db", dbPath, "--json"); err != nil {
		t.Fatalf("noting an existing title: %v", err)
	}
	if _, _, err := runRootArgs(t, "track", "drop", "52991", "--db", dbPath, "--json"); err != nil {
		t.Fatalf("dropping an existing title: %v", err)
	}
	entries = libraryEntries(t, dbPath)
	if len(entries) != 1 {
		t.Fatalf("library = %+v, want exactly one row after legitimate updates", entries)
	}
	if entries[0].Score != 9 || entries[0].Notes != "watch subbed" || entries[0].Status != "dropped" {
		t.Fatalf("updated row = %+v, want score 9, the note, and dropped status", entries[0])
	}
}

// TestTrackMutationStoreFailureIsNotAUsageError pins the error split: a title
// the user never added is a usage error (exit 2), but a real store failure must
// not masquerade as bad arguments, and must not dump command usage.
func TestTrackMutationStoreFailureIsNotAUsageError(t *testing.T) {
	testenv.Isolate(t)

	dbPath := filepath.Join(t.TempDir(), "library.db")
	seedLibrary(t, dbPath)

	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if _, execErr := db.DB().Exec(`DROP TABLE mal_library`); execErr != nil {
		t.Fatalf("drop mal_library: %v", execErr)
	}
	if _, execErr := db.DB().Exec(`CREATE TABLE mal_library (wrong TEXT)`); execErr != nil {
		t.Fatalf("create wrong-schema mal_library: %v", execErr)
	}
	if closeErr := db.Close(); closeErr != nil {
		t.Fatalf("close store: %v", closeErr)
	}

	// Cobra's usage writer targets OutOrStderr, and runRootArgs sets Out to the
	// stdout buffer, so both streams have to be checked: asserting only on
	// stderr would be an assertion that can never fail.
	stdout, stderr, err := runRootArgs(t, "track", "rate", "52991", "--score", "9", "--db", dbPath, "--json")
	if err == nil {
		t.Fatal("track rate against a broken store: err = nil, want a store error")
	}
	if code := ExitCode(err); code == 2 {
		t.Fatalf("store failure reported as a usage error (exit 2): %v", err)
	}
	if strings.Contains(stdout, "Usage:") || strings.Contains(stderr, "Usage:") {
		t.Fatalf("store failure printed command usage (stdout=%q stderr=%q)", stdout, stderr)
	}
}

// TestTrackMutationAdvancesUpdatedAt pins the "last changed" timestamp: a row
// loaded for an update carries its stored updated_at, so the writer has to
// restamp it, or the mutation keeps the old time and the row never moves under
// the updated_at DESC ordering that track list and export rely on.
func TestTrackMutationAdvancesUpdatedAt(t *testing.T) {
	testenv.Isolate(t)

	dbPath := filepath.Join(t.TempDir(), "library.db")
	seedLibrary(t, dbPath)

	const backdated = "2000-01-01T00:00:00Z"
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if _, execErr := db.DB().Exec(`UPDATE mal_library SET updated_at = ? WHERE kind = 'anime' AND id = 52991`, backdated); execErr != nil {
		_ = db.Close()
		t.Fatalf("backdating updated_at: %v", execErr)
	}
	if closeErr := db.Close(); closeErr != nil {
		t.Fatalf("close store: %v", closeErr)
	}

	if _, _, err := runRootArgs(t, "track", "rate", "52991", "--score", "9", "--db", dbPath, "--json"); err != nil {
		t.Fatalf("track rate: %v", err)
	}

	entries := libraryEntries(t, dbPath)
	if len(entries) != 1 {
		t.Fatalf("library = %+v, want one row", entries)
	}
	if entries[0].UpdatedAt == backdated || entries[0].UpdatedAt == "" {
		t.Fatalf("updated_at = %q after a mutation, want a fresh timestamp", entries[0].UpdatedAt)
	}
	stamped, parseErr := time.Parse(time.RFC3339, entries[0].UpdatedAt)
	if parseErr != nil {
		t.Fatalf("updated_at %q is not RFC3339: %v", entries[0].UpdatedAt, parseErr)
	}
	if time.Since(stamped) > time.Hour {
		t.Fatalf("updated_at = %q is not recent", entries[0].UpdatedAt)
	}
}

// TestTrackRemoveReportsNotFound is the regression guard for the discarded
// delete result: a missing row must be reported as not found rather than as a
// successful removal, while a real removal still succeeds.
func TestTrackRemoveReportsNotFound(t *testing.T) {
	testenv.Isolate(t)

	dbPath := filepath.Join(t.TempDir(), "library.db")
	seedLibrary(t, dbPath)

	// Absent id: not found, exit 3, accurate JSON body.
	stdout, _, err := runRootArgs(t, "track", "remove", "4242", "--db", dbPath, "--json")
	if err == nil {
		t.Fatal("track remove of an absent id: err = nil, want a not-found error")
	}
	if code := ExitCode(err); code != 3 {
		t.Fatalf("absent-id exit code = %d, want 3 (err = %v)", code, err)
	}
	var absentBody map[string]any
	if unmarshalErr := json.Unmarshal([]byte(stdout), &absentBody); unmarshalErr != nil {
		t.Fatalf("absent-id JSON body %q: %v", stdout, unmarshalErr)
	}
	if removed, ok := absentBody["removed"].(bool); !ok || removed {
		t.Fatalf("absent-id body = %v, want removed=false", absentBody)
	}
	if found, ok := absentBody["found"].(bool); !ok || found {
		t.Fatalf("absent-id body = %v, want found=false", absentBody)
	}

	// The wrong media kind must not report a removal either.
	if _, _, err := runRootArgs(t, "track", "remove", "52991", "--kind", "manga", "--db", dbPath, "--json"); err == nil {
		t.Fatal("track remove with the wrong --kind: err = nil, want a not-found error")
	}

	// A real removal still succeeds and reports removed=true.
	stdout, _, err = runRootArgs(t, "track", "remove", "52991", "--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("removing an existing row: %v", err)
	}
	var removedBody map[string]any
	if unmarshalErr := json.Unmarshal([]byte(stdout), &removedBody); unmarshalErr != nil {
		t.Fatalf("removal JSON body %q: %v", stdout, unmarshalErr)
	}
	if removed, ok := removedBody["removed"].(bool); !ok || !removed {
		t.Fatalf("removal body = %v, want removed=true", removedBody)
	}
	if entries := libraryEntries(t, dbPath); len(entries) != 0 {
		t.Fatalf("library = %+v, want empty after removal", entries)
	}

	// Removing it again is a not-found, not a second success.
	if _, _, err := runRootArgs(t, "track", "remove", "52991", "--db", dbPath, "--json"); err == nil {
		t.Fatal("second removal: err = nil, want a not-found error")
	} else if code := ExitCode(err); code != 3 {
		t.Fatalf("second-removal exit code = %d, want 3", code)
	}
}
