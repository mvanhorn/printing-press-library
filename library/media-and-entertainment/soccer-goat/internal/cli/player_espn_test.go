package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/report"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/source/espn"
)

func TestPrintESPNBlock_Populated(t *testing.T) {
	player := &report.PlayerReport{
		ESPN: &espn.Context{
			DisplayName: "João Neves",
			Stats:       &espn.SeasonStats{Goals: 1, Assists: 0, Shots: 3, ShotsOnTarget: 1, Starts: 5},
			Splits: []espn.StatSplit{
				{DisplayName: "2026 FIFA World Cup", TeamID: "482", Stats: espn.SeasonStats{Goals: 1, Shots: 3}},
			},
			RecentGames: []espn.GameLogEntry{
				{Opponent: "Spain", AtVs: "vs", Result: "L", Score: "1-0", Goals: 0, Assists: 0},
			},
		},
	}
	var buf bytes.Buffer
	printESPNBlock(&buf, player)
	out := buf.String()
	for _, want := range []string{"ESPN season: 1G 0A", "2026 FIFA World Cup", "Last 1:", "Spain"} {
		if !strings.Contains(out, want) {
			t.Errorf("ESPN block missing %q; got:\n%s", want, out)
		}
	}
}

func TestPrintESPNBlock_AbsentIsSilent(t *testing.T) {
	var buf bytes.Buffer
	printESPNBlock(&buf, &report.PlayerReport{ESPN: nil})
	if buf.Len() != 0 {
		t.Errorf("expected no output when ESPN absent, got %q", buf.String())
	}
	// ESPN present but no stats/splits/games: still silent on the stat lines.
	buf.Reset()
	printESPNBlock(&buf, &report.PlayerReport{ESPN: &espn.Context{DisplayName: "X"}})
	if strings.Contains(buf.String(), "ESPN season") {
		t.Errorf("should not print a season line without stats, got %q", buf.String())
	}
}
