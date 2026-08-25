// internal/cli/lookup_test.go
package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/catalog"
	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/domain"
	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/provider"
)

type stubProvider struct{}

func (stubProvider) Name() string { return "mika" }
func (stubProvider) Lookup(_ context.Context, e domain.Event, bib string) (domain.Result, error) {
	return domain.Result{Provider: "mika", RaceName: e.Name, Year: e.Year, Bib: bib, Runner: "Sample Runner", NetTime: "02:45:10"}, nil
}

func TestLookupRendersResult(t *testing.T) {
	reg := provider.NewRegistry()
	reg.Register(stubProvider{})
	root := NewRoot(reg)

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"lookup", "berlin marathon", "1234", "--year", "2025"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "Sample Runner") {
		t.Fatalf("expected runner in output:\n%s", out.String())
	}
}

func TestLookupUnknownRaceErrors(t *testing.T) {
	reg := provider.NewRegistry()
	root := NewRoot(reg)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"lookup", "zzz nonexistent", "1"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for unknown race")
	}
}

type stubNameProvider struct{}

func (stubNameProvider) Name() string { return "mika" }
func (stubNameProvider) Lookup(_ context.Context, e domain.Event, bib string) (domain.Result, error) {
	return domain.Result{Provider: "mika", RaceName: e.Name, Year: e.Year, Bib: bib, Runner: "Sample Runner", NetTime: "02:45:10"}, nil
}
func (stubNameProvider) SearchByName(_ context.Context, ev domain.Event, name string) ([]domain.Result, error) {
	return []domain.Result{
		{Provider: "mika", RaceName: ev.Name, Runner: "Jane Runner", Bib: "11", NetTime: "03:00:00"},
		{Provider: "mika", RaceName: ev.Name, Runner: "John Runner", Bib: "12", NetTime: "03:10:00"},
	}, nil
}

func TestLookupByName(t *testing.T) {
	reg := provider.NewRegistry()
	reg.Register(stubNameProvider{})
	entries := []catalog.Entry{{Race: "BMW Berlin Marathon", Aliases: []string{"berlin"}, Provider: "mika", Year: 2025}}
	cmd := newLookupCmd(reg, entries)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"berlin", "--name", "Runner", "--year", "2025"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "Jane Runner") || !strings.Contains(out.String(), "John Runner") {
		t.Fatalf("expected name matches:\n%s", out.String())
	}
}

func TestLookupDeriveYearFromDate(t *testing.T) {
	reg := provider.NewRegistry()
	reg.Register(stubProvider{})
	entries := []catalog.Entry{
		{Race: "BMW Berlin Marathon", Aliases: []string{"berlin"}, Provider: "mika", EventID: "B2024", Year: 2024},
		{Race: "BMW Berlin Marathon", Aliases: []string{"berlin"}, Provider: "mika", EventID: "B2025", Year: 2025},
	}
	cmd := newLookupCmd(reg, entries)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"berlin", "1234", "--date", "2025-09-28"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "Multiple matches") {
		t.Fatalf("date did not disambiguate edition; got ambiguity:\n%s", got)
	}
	if !strings.Contains(got, "BMW Berlin Marathon 2025") {
		t.Fatalf("expected 2025 edition resolved, got:\n%s", got)
	}
}
