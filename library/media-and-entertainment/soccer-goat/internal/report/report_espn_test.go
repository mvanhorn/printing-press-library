package report

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/source/espn"
)

// stubESPN is a concurrency-safe ESPNResolver that records how many lookups it
// served and returns canned enrichment.
type stubESPN struct {
	mu        sync.Mutex
	calls     int
	failMatch bool // when true, Lookup returns ok=false
	enrichErr bool // when true, Enrich returns an error (resolve still succeeds)
}

func (s *stubESPN) Lookup(_ context.Context, name string) (espn.Context, bool, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	if s.failMatch {
		return espn.Context{}, false, nil
	}
	return espn.Context{DisplayName: name, AthleteID: "1", Team: "Club"}, true, nil
}

func (s *stubESPN) Enrich(_ context.Context, _ string) (*espn.Enrichment, error) {
	if s.enrichErr {
		return nil, fmt.Errorf("boom")
	}
	return &espn.Enrichment{Stats: &espn.SeasonStats{Goals: 3, Assists: 2}}, nil
}

func makeSquad(n int) []PlayerReport {
	players := make([]PlayerReport, n)
	for i := range players {
		players[i] = PlayerReport{
			Name:        fmt.Sprintf("Player %02d", i),
			MarketValue: int64((n - i) * 1_000_000), // Player 00 is most valuable
			Sources:     map[string]SourceStatus{},
		}
	}
	return players
}

func TestEnrichTeamESPN_BudgetAndRanking(t *testing.T) {
	players := makeSquad(15)
	stub := &stubESPN{}
	agg := &Aggregator{ESPN: stub}

	okCount, attempted := agg.enrichTeamESPN(context.Background(), players)

	// R5: the ESPN client is called at most budget times, never once per player.
	if stub.calls > espnTeamBudget {
		t.Fatalf("ESPN called %d times, must be <= budget %d", stub.calls, espnTeamBudget)
	}
	if attempted != stub.calls {
		t.Fatalf("attempted %d != actual calls %d", attempted, stub.calls)
	}
	if okCount != attempted {
		t.Fatalf("okCount %d != attempted %d (all stub lookups succeed)", okCount, attempted)
	}

	// Every enriched (ok) player must be worth at least as much as every skipped
	// player — the budget takes the top by market value.
	var maxSkipped, minEnriched int64 = 0, 1 << 62
	enriched, skipped := 0, 0
	for i := range players {
		st := players[i].Sources["espn"]
		switch {
		case st.OK:
			enriched++
			if players[i].MarketValue < minEnriched {
				minEnriched = players[i].MarketValue
			}
			if players[i].ESPN == nil || players[i].ESPN.Stats == nil || players[i].ESPN.Stats.Goals != 3 {
				t.Errorf("enriched player %s missing attached stats", players[i].Name)
			}
		case st.Detail == "skipped: espn enrichment limit":
			skipped++
			if players[i].MarketValue > maxSkipped {
				maxSkipped = players[i].MarketValue
			}
		default:
			t.Errorf("player %s has unexpected espn status %+v", players[i].Name, st)
		}
	}
	if enriched+skipped != len(players) {
		t.Fatalf("covered %d players, want %d", enriched+skipped, len(players))
	}
	if skipped > 0 && maxSkipped >= minEnriched {
		t.Errorf("a skipped player (value %d) outranks an enriched one (value %d)", maxSkipped, minEnriched)
	}
}

func TestEnrichTeamESPN_SmallSquadNoSkips(t *testing.T) {
	players := makeSquad(3)
	stub := &stubESPN{}
	agg := &Aggregator{ESPN: stub}
	okCount, attempted := agg.enrichTeamESPN(context.Background(), players)
	if attempted != 3 || okCount != 3 {
		t.Fatalf("small squad: okCount=%d attempted=%d, want 3/3", okCount, attempted)
	}
	for i := range players {
		if !players[i].Sources["espn"].OK {
			t.Errorf("player %s not enriched in an under-budget squad", players[i].Name)
		}
	}
}

func TestResolveESPNPlayer_EnrichFailureKeepsResolve(t *testing.T) {
	// R6: a failed Enrich must not demote a successful resolve.
	player := &PlayerReport{Name: "Someone", Sources: map[string]SourceStatus{}}
	agg := &Aggregator{ESPN: &stubESPN{enrichErr: true}}
	if !agg.resolveESPNPlayer(context.Background(), player) {
		t.Fatal("resolve should succeed even when enrich fails")
	}
	if player.ESPN == nil || !player.Sources["espn"].OK {
		t.Fatal("context and OK status must survive an enrich failure")
	}
	if player.ESPN.Stats != nil {
		t.Error("stats should be absent after enrich failure")
	}
}

func TestResolveESPNPlayer_NoMatch(t *testing.T) {
	player := &PlayerReport{Name: "Nobody", Sources: map[string]SourceStatus{}}
	agg := &Aggregator{ESPN: &stubESPN{failMatch: true}}
	if agg.resolveESPNPlayer(context.Background(), player) {
		t.Fatal("resolve should report failure on no match")
	}
	if player.Sources["espn"].OK || player.ESPN != nil {
		t.Error("no-match player must not be marked ok or carry context")
	}
}
