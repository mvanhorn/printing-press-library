// Hand-authored coverage for curate --sort velocity. Not generated.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/lancet"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/store"
)

func TestCurateVelocityLivePathErrors(t *testing.T) {
	for _, ds := range []string{"live", "auto"} {
		t.Run(ds, func(t *testing.T) {
			origLocal, origLive := curateLocalFn, curateLiveFn
			t.Cleanup(func() { curateLocalFn, curateLiveFn = origLocal, origLive })
			liveCalls := 0
			curateLocalFn = func(ctx context.Context, path, topic, issn, sortBy string, openAccess bool, limit int) ([]lancet.WorkRow, bool, error) {
				return nil, true, nil
			}
			curateLiveFn = func(ctx context.Context, flags *rootFlags, topic, issn, sortBy string, openAccess bool, limit int) ([]lancet.WorkRow, error) {
				liveCalls++
				return curateLiveRows, nil
			}
			cmd := newNovelCurateCmd(&rootFlags{dataSource: ds, asJSON: true})
			var out, errb bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&errb)
			cmd.SetArgs([]string{"--topic", "ai", "--sort", "velocity", "--db", "unused.db"})
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), "local store") {
				t.Fatalf("err = %v, want a 'needs the local store' error", err)
			}
			if liveCalls != 0 || strings.Contains(out.String(), "Live paper") {
				t.Fatalf("live was used silently: calls=%d stdout=%q", liveCalls, out.String())
			}
		})
	}
}

func TestCurateAcceptsVelocitySortAndPassesItToTheStore(t *testing.T) {
	origLocal, origLive := curateLocalFn, curateLiveFn
	t.Cleanup(func() { curateLocalFn, curateLiveFn = origLocal, origLive })
	gotSort := ""
	curateLocalFn = func(ctx context.Context, path, topic, issn, sortBy string, openAccess bool, limit int) ([]lancet.WorkRow, bool, error) {
		gotSort = sortBy
		return curateLocalRows, true, nil
	}
	cmd := newNovelCurateCmd(&rootFlags{dataSource: "local", asJSON: true})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs([]string{"--topic", "ai", "--sort", "velocity", "--db", "unused.db"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("curate --sort velocity: %v", err)
	}
	if gotSort != "velocity" || !strings.Contains(out.String(), "Local paper") {
		t.Fatalf("sort passed = %q, stdout = %q", gotSort, out.String())
	}
}

func TestCurateUnknownSortListsVelocity(t *testing.T) {
	cmd := newNovelCurateCmd(&rootFlags{dataSource: "local", asJSON: true})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs([]string{"--topic", "ai", "--sort", "bogus", "--db", "unused.db"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "'citations', 'date', 'per-year' or 'velocity'") {
		t.Fatalf("err = %v, want the sort list to include velocity", err)
	}
}

type pageFetcher struct{ payload json.RawMessage }

func (p pageFetcher) Get(context.Context, string, map[string]string) (json.RawMessage, error) {
	return p.payload, nil
}

// An old mirror (works stored, no yearly rows) must make the real command fail
// with a hint naming refresh, not print rows with an empty velocity.
func TestCurateVelocityOldMirrorFailsWithRefreshHint(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "old.db")
	ctx := context.Background()
	st, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	st.DB().SetMaxOpenConns(1)
	page := `{"meta":{"next_cursor":""},"results":[{"id":"https://openalex.org/W1","title":"Old mirror work","publication_year":2015,"cited_by_count":100,"open_access":{"is_oa":false},"primary_topic":{"display_name":"Rate"}}]}`
	if _, err := lancet.Refresh(ctx, pageFetcher{json.RawMessage(page)}, st.DB(), []lancet.Journal{{Slug: "lancet", ISSN: "0140-6736", Display: "The Lancet"}}, 0, 0, 1, nil); err != nil {
		t.Fatal(err)
	}
	st.Close()

	cmd := newNovelCurateCmd(&rootFlags{dataSource: "local", asJSON: true})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs([]string{"--topic", "rate", "--sort", "velocity", "--db", dbPath})
	err = cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "refresh") {
		t.Fatalf("err = %v, stdout = %q; want a non-zero exit naming refresh", err, out.String())
	}
	if strings.Contains(out.String(), "Old mirror work") {
		t.Fatalf("rows were printed despite the missing yearly counts: %q", out.String())
	}
}
