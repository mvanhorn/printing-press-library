package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/job-boards/ashby/internal/store"
	"github.com/spf13/cobra"
)

func TestAshbyPostingsDryRunReturnsPreviewWithoutDecodingSentinel(t *testing.T) {
	for _, tc := range []struct {
		name string
		cmd  *cobra.Command
		args []string
	}{
		{name: "list", cmd: newAshbyPostingsListCmd(&rootFlags{dryRun: true, asJSON: true}), args: []string{"example"}},
		{name: "get", cmd: newAshbyPostingsGetCmd(&rootFlags{dryRun: true, asJSON: true}), args: []string{"example", "job-1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			tc.cmd.SetOut(&output)
			tc.cmd.SetErr(&output)
			tc.cmd.SetArgs(tc.args)
			if err := tc.cmd.Execute(); err != nil {
				t.Fatalf("dry-run: %v (%s)", err, output.String())
			}
			if !strings.Contains(output.String(), `"dry_run":true`) {
				t.Fatalf("missing dry-run preview: %q", output.String())
			}
		})
	}
}

func TestAshbyPostingsDryRunValidatesAndShowsRequest(t *testing.T) {
	for _, tc := range []struct {
		name string
		cmd  *cobra.Command
		args []string
		want string
	}{
		{name: "list board", cmd: newAshbyPostingsListCmd(&rootFlags{dryRun: true, asJSON: true}), args: []string{"bad/name"}, want: "invalid job board name"},
		{name: "get board", cmd: newAshbyPostingsGetCmd(&rootFlags{dryRun: true, asJSON: true}), args: []string{"bad/name", "job-1"}, want: "invalid job board name"},
		{name: "list date", cmd: newAshbyPostingsListCmd(&rootFlags{dryRun: true, asJSON: true}), args: []string{"example", "--published-since", "yesterday"}, want: "invalid --published-since"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.cmd.SetArgs(tc.args)
			if err := tc.cmd.Execute(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("dry-run error = %v, want %q", err, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name string
		cmd  *cobra.Command
		args []string
		key  string
		want any
	}{
		{name: "list", cmd: newAshbyPostingsListCmd(&rootFlags{dryRun: true, asJSON: true}), args: []string{"example", "--include-compensation", "--published-since", "2026-01-01"}, key: "filters", want: "2026-01-01"},
		{name: "get", cmd: newAshbyPostingsGetCmd(&rootFlags{dryRun: true, asJSON: true}), args: []string{"example", "job-1", "--include-compensation"}, key: "posting_id", want: "job-1"},
	} {
		t.Run(tc.name+" preview", func(t *testing.T) {
			var output bytes.Buffer
			tc.cmd.SetOut(&output)
			tc.cmd.SetArgs(tc.args)
			if err := tc.cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			var preview map[string]any
			if err := json.Unmarshal(output.Bytes(), &preview); err != nil {
				t.Fatal(err)
			}
			if preview["dry_run"] != true || preview["method"] != "GET" || preview["path"] != "/posting-api/job-board/example" {
				t.Fatalf("incomplete request preview: %#v", preview)
			}
			if query, ok := preview["query"].(map[string]any); !ok || query["includeCompensation"] != "true" {
				t.Fatalf("missing query parameter: %#v", preview)
			}
			if tc.key == "filters" {
				filters, ok := preview["filters"].(map[string]any)
				if !ok || filters["published_since"] != tc.want {
					t.Fatalf("missing local filter: %#v", preview)
				}
			} else if preview[tc.key] != tc.want {
				t.Fatalf("missing posting ID: %#v", preview)
			}
		})
	}
}

func floatPtr(v float64) *float64 { return &v }

func TestFilterAshbyJobsExcludesUnlistedAndAppliesStructuredFilters(t *testing.T) {
	jobs := []ashbyJobPosting{
		{ID: "listed", Title: "Platform Engineer", Department: "Engineering", IsListed: true, IsRemote: true, WorkplaceType: "Remote", EmploymentType: "FullTime", Compensation: &ashbyCompensation{SummaryComponents: []ashbyCompensationComponent{{Type: "Salary", CurrencyCode: "USD", MinValue: floatPtr(180000), MaxValue: floatPtr(220000)}}}},
		{ID: "unlisted", Title: "Secret Engineer", Department: "Engineering", IsListed: false, IsRemote: true},
		{ID: "onsite", Title: "Platform Engineer", Department: "Engineering", IsListed: true, IsRemote: false, WorkplaceType: "OnSite"},
	}
	got, err := filterAshbyJobs(jobs, ashbyPostingFilter{Query: "platform", Department: "engineer", Remote: true, Currency: "usd", SalaryMin: 200000})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "listed" {
		t.Fatalf("got %#v, want only listed", got)
	}
}

func TestFilterAshbyJobsRejectsInvalidDate(t *testing.T) {
	if _, err := filterAshbyJobs(nil, ashbyPostingFilter{PublishedSince: "yesterday"}); err == nil {
		t.Fatal("expected invalid date error")
	}
}

func TestFilterAshbyJobsSortsBeforeApplyingLimit(t *testing.T) {
	jobs := []ashbyJobPosting{
		{ID: "old", IsListed: true, PublishedAt: "2026-01-01T00:00:00Z"},
		{ID: "new", IsListed: true, PublishedAt: "2026-03-01T00:00:00Z"},
		{ID: "middle", IsListed: true, PublishedAt: "2026-02-01T00:00:00Z"},
	}
	got, err := filterAshbyJobs(jobs, ashbyPostingFilter{Limit: 1})
	if err != nil || len(got) != 1 || got[0].ID != "new" {
		t.Fatalf("newest job with limit 1 = %#v, %v", got, err)
	}
}

func TestFilterAshbyJobsSortsTimezonesBeforeApplyingLimit(t *testing.T) {
	jobs := []ashbyJobPosting{
		{ID: "older", IsListed: true, PublishedAt: "2026-01-01T01:00:00+02:00"},
		{ID: "newer", IsListed: true, PublishedAt: "2026-01-01T00:30:00Z"},
		{ID: "invalid", IsListed: true, PublishedAt: "later-looking-but-invalid"},
	}
	got, err := filterAshbyJobs(jobs, ashbyPostingFilter{Limit: 1})
	if err != nil || len(got) != 1 || got[0].ID != "newer" {
		t.Fatalf("newest job across timezone offsets = %#v, %v", got, err)
	}
}

func TestDecodeAshbyBoardJobsRejectsMissingSnapshot(t *testing.T) {
	for _, raw := range []string{
		`{}`, `{"jobs":null}`, `{"jobs":{"id":"wrong-shape"}}`,
		`{"jobs":[null]}`, `{"jobs":[{}]}`,
		`{"jobs":[{"id":"x"}]}`, `{"jobs":[{"id":"x","isListed":null}]}`,
		`{"jobs":[{"id":"x","isListed":"false"}]}`,
		`{"jobs":[{"id":"","isListed":false}]}`,
	} {
		if _, err := decodeAshbyBoardJobs([]byte(raw)); err == nil {
			t.Fatalf("accepted incomplete job board response %s", raw)
		}
	}
	if _, err := decodeAshbyBoardJobs([]byte(`{"jobs":[{"id":"x","isListed":false}]}`)); err != nil {
		t.Fatalf("rejected complete unlisted row: %v", err)
	}
	jobs, err := decodeAshbyBoardJobs([]byte(`{"jobs":[]}`))
	if err != nil || len(jobs) != 0 {
		t.Fatalf("complete empty snapshot = %#v, %v", jobs, err)
	}
}

func TestListedAshbyJobs(t *testing.T) {
	got := listedAshbyJobs([]ashbyJobPosting{{ID: "a", IsListed: true}, {ID: "b", IsListed: false}})
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("got %#v", got)
	}
}

func TestPersistAshbyBoardSnapshotRemovesNewlyUnlistedPosting(t *testing.T) {
	db, err := store.OpenWithContext(context.Background(), t.TempDir()+"/ashby.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	first := []ashbyJobPosting{{ID: "keep", Title: "Keep", IsListed: true}, {ID: "hide", Title: "Hide", IsListed: true}}
	if stored, removed, err := persistAshbyBoardSnapshot(db, "example", first); err != nil || stored != 2 || removed != 0 {
		t.Fatalf("first snapshot: stored=%d removed=%d err=%v", stored, removed, err)
	}
	second := []ashbyJobPosting{{ID: "keep", Title: "Keep", IsListed: true}, {ID: "hide", Title: "Hide", IsListed: false}}
	if stored, removed, err := persistAshbyBoardSnapshot(db, "example", second); err != nil || stored != 1 || removed != 1 {
		t.Fatalf("second snapshot: stored=%d removed=%d err=%v", stored, removed, err)
	}
	ids, err := db.ListIDs("postings:example")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "keep" {
		t.Fatalf("ids=%v, want [keep]", ids)
	}
}

func TestPersistAshbyBoardSnapshotRejectsMissingIDWithoutDeleting(t *testing.T) {
	db, err := store.OpenWithContext(context.Background(), t.TempDir()+"/ashby.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, _, err := persistAshbyBoardSnapshot(db, "example", []ashbyJobPosting{{ID: "keep", IsListed: true}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := persistAshbyBoardSnapshot(db, "example", []ashbyJobPosting{{IsListed: true}}); err == nil {
		t.Fatal("missing ID was accepted")
	}
	ids, err := db.ListIDs("postings:example")
	if err != nil || !reflect.DeepEqual(ids, []string{"keep"}) {
		t.Fatalf("snapshot after invalid item = %v, %v", ids, err)
	}
}

func TestPersistAshbyBoardSnapshotRollsBackOnRemovalFailure(t *testing.T) {
	db, err := store.OpenWithContext(context.Background(), t.TempDir()+"/ashby.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	initial := []ashbyJobPosting{{ID: "keep", Title: "Original", IsListed: true}, {ID: "stale", IsListed: true}}
	if _, _, err := persistAshbyBoardSnapshot(db, "example", initial); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`CREATE TRIGGER block_stale_delete BEFORE DELETE ON resources
		WHEN OLD.resource_type = 'postings:example' AND OLD.id = 'stale'
		BEGIN SELECT RAISE(ABORT, 'blocked reconcile'); END`); err != nil {
		t.Fatal(err)
	}
	replacement := []ashbyJobPosting{{ID: "keep", Title: "Updated", IsListed: true}, {ID: "new", IsListed: true}}
	if _, _, err := persistAshbyBoardSnapshot(db, "example", replacement); err == nil || !strings.Contains(err.Error(), "blocked reconcile") {
		t.Fatalf("expected failed removal, got %v", err)
	}
	ids, err := db.ListIDs("postings:example")
	if err != nil || !reflect.DeepEqual(ids, []string{"keep", "stale"}) {
		t.Fatalf("IDs after failed replacement = %v, %v", ids, err)
	}
	item, err := db.Get("postings:example", "keep")
	if err != nil || !strings.Contains(string(item), "Original") {
		t.Fatalf("old item changed during failed replacement: %s, %v", item, err)
	}
}
