package cli

import (
	"encoding/json"
	"testing"
)

func TestExtractOddsHappyPath(t *testing.T) {
	raw := json.RawMessage(`{
		"events":[{
			"id":"123","shortName":"BOS @ LAL",
			"competitions":[{
				"odds":[{
					"provider":{"name":"ESPN BET"},
					"details":"LAL -3.5","overUnder":225.5,
					"awayTeamOdds":{"moneyLine":140},
					"homeTeamOdds":{"moneyLine":-160}
				}]
			}]
		}]
	}`)
	lines := extractOdds(raw)
	if len(lines) != 1 {
		t.Fatalf("want 1 line, got %d", len(lines))
	}
	got := lines[0]
	if got.Matchup != "BOS @ LAL" || got.Spread != "LAL -3.5" || got.OverUnder != "225.5" {
		t.Errorf("unexpected line: %+v", got)
	}
	if got.AwayMoneyline != "+140" || got.HomeMoneyline != "-160" {
		t.Errorf("moneylines wrong: %+v", got)
	}
}

func TestExtractOddsEmpty(t *testing.T) {
	raw := json.RawMessage(`{"events":[]}`)
	lines := extractOdds(raw)
	if len(lines) != 0 {
		t.Errorf("expected empty slice, got %d", len(lines))
	}
}

func TestExtractOddsSkipsEventsWithoutOdds(t *testing.T) {
	raw := json.RawMessage(`{"events":[{"id":"1","shortName":"A @ B","competitions":[{"odds":[]}]}]}`)
	lines := extractOdds(raw)
	if len(lines) != 0 {
		t.Errorf("expected events without odds to be skipped, got %d lines", len(lines))
	}
}

func TestFormatMoneyline(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, ""},
		{150, "+150"},
		{-110, "-110"},
	}
	for _, c := range cases {
		got := formatMoneyline(c.in)
		if got != c.want {
			t.Errorf("formatMoneyline(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
