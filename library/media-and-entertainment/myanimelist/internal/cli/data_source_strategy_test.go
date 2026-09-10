package cli

import (
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/myanimelist/internal/cliutil/testenv"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestDeclaredDataSourceStrategyEnforced walks the whole command tree and
// proves the rejection matrix holds for every command that declares a strategy,
// not just the ones with a hand-written check. This is the guard the review
// asked for: a novel command is covered by its pp:data-source annotation.
func TestDeclaredDataSourceStrategyEnforced(t *testing.T) {
	t.Parallel()

	root := RootCmd()

	var live, local, auto []string
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		strategy := commandDataSourceAnnotation(cmd)
		switch strategy {
		case "live":
			if err := validateDeclaredDataSource(&rootFlags{dataSource: "local"}, cmd); err == nil {
				t.Errorf("%s declares pp:data-source=live but accepted --data-source local", cmd.CommandPath())
			} else if !strings.Contains(err.Error(), "no local data source") {
				t.Errorf("%s rejection message = %v, want %q", cmd.CommandPath(), err, "no local data source")
			}
			if err := validateDeclaredDataSource(&rootFlags{dataSource: "live"}, cmd); err != nil {
				t.Errorf("%s declares live but rejected --data-source live: %v", cmd.CommandPath(), err)
			}
			live = append(live, cmd.CommandPath())
		case "local":
			if err := validateDeclaredDataSource(&rootFlags{dataSource: "live"}, cmd); err == nil {
				t.Errorf("%s declares pp:data-source=local but accepted --data-source live", cmd.CommandPath())
			} else if !strings.Contains(err.Error(), "no live equivalent") {
				t.Errorf("%s rejection message = %v, want %q", cmd.CommandPath(), err, "no live equivalent")
			}
			if err := validateDeclaredDataSource(&rootFlags{dataSource: "local"}, cmd); err != nil {
				t.Errorf("%s declares local but rejected --data-source local: %v", cmd.CommandPath(), err)
			}
			local = append(local, cmd.CommandPath())
		case "auto":
			for _, requested := range []string{"live", "local", "auto"} {
				if err := validateDeclaredDataSource(&rootFlags{dataSource: requested}, cmd); err != nil {
					t.Errorf("%s declares auto but rejected --data-source %s: %v", cmd.CommandPath(), requested, err)
				}
			}
			auto = append(auto, cmd.CommandPath())
		case "":
			// Framework and generated commands carry no declaration; the
			// endpoint resolvers enforce their own strategy.
			if err := validateDeclaredDataSource(&rootFlags{dataSource: "local"}, cmd); err != nil {
				t.Errorf("%s has no pp:data-source annotation but rejected --data-source local: %v", cmd.CommandPath(), err)
			}
		default:
			t.Errorf("%s declares unknown pp:data-source strategy %q", cmd.CommandPath(), strategy)
		}
		for _, sub := range cmd.Commands() {
			walk(sub)
		}
	}
	walk(root)

	if len(live) < 5 || len(local) < 8 || len(auto) < 2 {
		t.Fatalf("annotation walk too small to be meaningful: live=%v local=%v auto=%v", live, local, auto)
	}
}

// TestDataSourceRejectionEndToEnd runs the exact examples from the review
// through the real command tree. None of these reach RunE, so no network access
// is possible.
func TestDataSourceRejectionEndToEnd(t *testing.T) {
	testenv.Isolate(t)

	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			"live command rejects local",
			[]string{"anime", "consistency", "52991", "--data-source", "local", "--json"},
			"no local data source",
		},
		{
			"another live command rejects local",
			[]string{"adaptation", "52991", "--data-source", "local", "--json"},
			"no local data source",
		},
		{
			"suggest rejects local (no local candidate pool)",
			[]string{"suggest", "--data-source", "local", "--json"},
			"no local data source",
		},
		{
			"local command rejects live",
			[]string{"next", "--data-source", "live", "--json"},
			"no live equivalent",
		},
		{
			"local mutation rejects live",
			[]string{"track", "list", "--data-source", "live", "--json"},
			"no live equivalent",
		},
		{
			"local export rejects live",
			[]string{"export", "--data-source", "live", "--format", "json", "--json"},
			"no live equivalent",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := runRootArgs(t, tc.args...)
			if err == nil {
				t.Fatalf("%v: err = nil, want a rejection", tc.args)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%v: err = %v, want it to contain %q", tc.args, err, tc.want)
			}
		})
	}

	// An auto-declared command accepts both explicit sources: drift reports from
	// local snapshots but --record reads the live page, so neither request may
	// be refused.
	t.Setenv("MYANIMELIST_BASE_URL", "http://127.0.0.1:1/")
	for _, requested := range []string{"local", "live", "auto"} {
		if _, stderr, err := runRootArgs(t, "drift", "--db", filepath.Join(t.TempDir(), "empty.db"), "--data-source", requested, "--json"); err != nil {
			t.Fatalf("drift --data-source %s: %v (stderr=%s)", requested, err, stderr)
		}
	}
}

// TestWeekLocalOnlyDoesNotFetchLive covers the review's first example: an auto
// command handed --data-source local must stay offline. The unroutable base URL
// turns any network attempt into a failure. Two entries are seeded so the
// collision pass is exercised: an offline slot has no local time and must not
// be reported as colliding with the other unscheduled entries.
func TestWeekLocalOnlyDoesNotFetchLive(t *testing.T) {
	testenv.Isolate(t)

	dbPath := filepath.Join(t.TempDir(), "library.db")
	seedLibrary(t, dbPath)
	for _, extra := range []string{
		"track add 5114 --title Second Fixture --status watching --db " + dbPath + " --json",
	} {
		if _, _, err := runRootArgs(t, strings.Fields(extra)...); err != nil {
			t.Fatalf("seeding %q: %v", extra, err)
		}
	}
	t.Setenv("MYANIMELIST_BASE_URL", "http://127.0.0.1:1/")

	stdout, stderr, err := runRootArgs(t, "week", "--db", dbPath, "--data-source", "local", "--json")
	if err != nil {
		t.Fatalf("week --data-source local: %v (stderr=%s)", err, stderr)
	}
	if strings.Contains(stderr, "could not read") {
		t.Fatalf("week --data-source local attempted a live read: %s", stderr)
	}
	if !strings.Contains(stdout, `"unscheduled"`) {
		t.Fatalf("week --data-source local output = %s, want an unscheduled slot", stdout)
	}
	if !strings.Contains(stdout, "--data-source local") {
		t.Fatalf("week --data-source local output = %s, want the offline caveat", stdout)
	}
	if strings.Contains(stdout, `"collision": true`) {
		t.Fatalf("week --data-source local reported a collision for unscheduled slots: %s", stdout)
	}

	var view weekView
	if err := json.Unmarshal([]byte(stdout), &view); err != nil {
		t.Fatalf("decoding week output %q: %v", stdout, err)
	}
	if view.Entries != 2 {
		t.Fatalf("entries = %d, want 2", view.Entries)
	}
	for _, slot := range view.Slots {
		if slot.LocalTime != "" {
			t.Fatalf("local-only slot %d has a schedule time %q; it must stay offline", slot.ID, slot.LocalTime)
		}
		if slot.Collision {
			t.Fatalf("local-only slot %d is flagged as a collision with no schedule to collide with", slot.ID)
		}
	}
}

// TestLocalDeclaredCommandsDeclareTheSourceTheyActuallyUse guards the failure
// mode an annotation-only check cannot see: a command annotated "local" that
// still makes live requests. The unroutable base URL turns any live read into a
// failed request, so each case asserts the command either stays offline or
// declares itself live and refuses the local request outright.
func TestLocalDeclaredCommandsDeclareTheSourceTheyActuallyUse(t *testing.T) {
	testenv.Isolate(t)

	dbPath := filepath.Join(t.TempDir(), "library.db")
	seedLibrary(t, dbPath)
	t.Setenv("MYANIMELIST_BASE_URL", "http://127.0.0.1:1/")

	// franchise gap enumerates live related-entry pages; it is declared live, so
	// --data-source local is refused instead of silently hitting the network.
	if _, _, err := runRootArgs(t, "franchise", "gap", "--kind", "anime", "--db", dbPath, "--data-source", "local", "--json"); err == nil {
		t.Fatal("franchise gap --data-source local: err = nil, want the no-local-data-source refusal")
	} else if !strings.Contains(err.Error(), "no local data source") {
		t.Fatalf("franchise gap --data-source local: err = %v, want %q", err, "no local data source")
	}

	// track add writes a local row but enriches it from the live page, so it is
	// declared auto and must skip the fetch under --data-source local.
	stdout, stderr, err := runRootArgs(t, "track", "add", "100", "--db", dbPath, "--data-source", "local", "--json")
	if err != nil {
		t.Fatalf("track add --data-source local: %v (stderr=%s)", err, stderr)
	}
	if strings.Contains(stderr, "could not fetch title details") {
		t.Fatalf("track add --data-source local attempted a live read: %s", stderr)
	}
	if !strings.Contains(stderr, "--data-source local") {
		t.Fatalf("track add --data-source local stderr = %q, want the store-the-id-only notice", stderr)
	}
	if !strings.Contains(stdout, `"id": 100`) {
		t.Fatalf("track add --data-source local output = %s, want the stored row", stdout)
	}

	// drift reports from local snapshots but --record reads the live page, so
	// the one combination that cannot work offline is refused.
	_, _, err = runRootArgs(t, "drift", "52991", "--record", "--db", dbPath, "--data-source", "local", "--json")
	if err == nil {
		t.Fatal("drift --record --data-source local: err = nil, want the live-page refusal")
	}
	if !strings.Contains(err.Error(), "--record needs the live title page") {
		t.Fatalf("drift --record --data-source local: err = %v, want the live-page refusal", err)
	}

	// The non-record drift path is local-only and must keep working offline.
	if _, stderr, err := runRootArgs(t, "drift", "--db", dbPath, "--data-source", "local", "--json"); err != nil {
		t.Fatalf("drift --data-source local: %v (stderr=%s)", err, stderr)
	}
}
