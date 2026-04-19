package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSportForLeague(t *testing.T) {
	cases := []struct {
		league string
		want   string
	}{
		{"nfl", "football"},
		{"NBA", "basketball"},
		{"mlb", "baseball"},
		{"nhl", "hockey"},
		{"mls", "soccer"},
		{"unknown", ""},
	}
	for _, c := range cases {
		got := sportForLeague(c.league)
		if got != c.want {
			t.Errorf("sportForLeague(%q) = %q, want %q", c.league, got, c.want)
		}
	}
}

func TestRenderBoxscoreEmpty(t *testing.T) {
	// Edge case: pre-game boxscore has no players or teams.
	raw := json.RawMessage(`{"players":[],"teams":[]}`)
	var sb strings.Builder
	cmd := newBoxscoreCmd(&rootFlags{})
	cmd.SetOut(&sb)
	if err := renderBoxscore(cmd, raw); err != nil {
		t.Fatalf("renderBoxscore: %v", err)
	}
	if !strings.Contains(sb.String(), "empty") {
		t.Errorf("expected hint about empty boxscore, got: %s", sb.String())
	}
}

func TestRenderBoxscorePopulated(t *testing.T) {
	// Happy path: a single team with a single statistic group renders as table.
	raw := json.RawMessage(`{
		"players":[{
			"team":{"abbreviation":"KC","displayName":"Kansas City"},
			"statistics":[{
				"name":"passing",
				"labels":["YDS","TD"],
				"athletes":[{"athlete":{"displayName":"Mahomes"},"stats":["310","2"]}]
			}]
		}],
		"teams":[]
	}`)
	var sb strings.Builder
	cmd := newBoxscoreCmd(&rootFlags{})
	cmd.SetOut(&sb)
	if err := renderBoxscore(cmd, raw); err != nil {
		t.Fatalf("renderBoxscore: %v", err)
	}
	out := sb.String()
	for _, want := range []string{"KC", "Mahomes", "passing", "YDS"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got: %s", want, out)
		}
	}
}
