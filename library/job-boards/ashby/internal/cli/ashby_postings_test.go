package cli

import (
	"context"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/job-boards/ashby/internal/store"
)

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
