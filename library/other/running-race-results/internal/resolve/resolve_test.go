// internal/resolve/resolve_test.go
package resolve

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/catalog"
)

func seed() []catalog.Entry {
	return []catalog.Entry{
		{Race: "BMW Berlin Marathon", Aliases: []string{"berlin"}, Provider: "mika", EventID: "BERLIN", Year: 2025},
		{Race: "TCS New York City Marathon", Aliases: []string{"nyc"}, Provider: "nyrr", Year: 2024},
	}
}

func TestResolveStrongMatch(t *testing.T) {
	got, err := Resolve(seed(), "berlin marathon", 0)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got[0].Event.Provider != "mika" {
		t.Fatalf("top candidate %q, want mika", got[0].Event.Provider)
	}
}

func TestResolveYearFilter(t *testing.T) {
	got, err := Resolve(seed(), "berlin", 2025)
	if err != nil || got[0].Event.Year != 2025 {
		t.Fatalf("year filter failed: %+v err=%v", got, err)
	}
}

func TestResolveNoMatch(t *testing.T) {
	if _, err := Resolve(seed(), "zzz nonexistent race", 0); err != ErrNoMatch {
		t.Fatalf("expected ErrNoMatch, got %v", err)
	}
}
