package report

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/source/eafc"
)

func TestFormatEuros(t *testing.T) {
	tests := []struct {
		name  string
		value int64
		want  string
	}{
		{name: "zero", value: 0, want: "€0"},
		{name: "millions", value: 30_000_000, want: "€30.00m"},
		{name: "thousands", value: 950_000, want: "€950k"},
		{name: "fractional thousands", value: 1_500, want: "€1.5k"},
		{name: "euros", value: 750, want: "€750"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := FormatEuros(test.value); got != test.want {
				t.Fatalf("FormatEuros(%d) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestDecodeRosterInitializesEmptySlice(t *testing.T) {
	players, err := decodeRoster([]byte(`{"players":null}`))
	if err != nil {
		t.Fatalf("decodeRoster() error = %v", err)
	}
	if players == nil || len(players) != 0 {
		t.Fatalf("decodeRoster() = %#v, want non-nil empty slice", players)
	}
}

func TestEAMatchConsistent(t *testing.T) {
	cases := []struct {
		name    string
		tmName  string
		club    string
		eaFirst string
		eaLast  string
		eaTeam  string
		wantOK  bool
	}{
		{"exact club + name", "Andreas Schjelderup", "SL Benfica", "Andreas", "Schjelderup", "SL Benfica", true},
		{"club affix variation", "Andreas Schjelderup", "Benfica", "Andreas", "Schjelderup", "SL Benfica", true},
		{"nickname prefix affirms", "Rodri", "Manchester City", "Rodrigo", "Hernández", "Manchester City", true},
		{"missing tm club unverifiable rejected", "Andreas Schjelderup", "", "Andreas", "Schjelderup", "SL Benfica", false},
		{"missing ea team unverifiable rejected", "Andreas Schjelderup", "SL Benfica", "Andreas", "Schjelderup", "", false},
		{"different club rejected", "Andreas Schjelderup", "SL Benfica", "Andreas", "Schjelderup", "Manchester City", false},
		{"same-club namesake rejected", "Tomás Araújo", "SL Benfica", "António", "Silva", "SL Benfica", false},
		{"shared first name different surname rejected", "João Silva", "SL Benfica", "João", "Santos", "SL Benfica", false},
		{"shared surname affirms", "João Silva", "SL Benfica", "Pedro", "Silva", "SL Benfica", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := &PlayerReport{Name: tc.tmName, Club: tc.club}
			player := &eafc.Player{FirstName: tc.eaFirst, LastName: tc.eaLast, Team: tc.eaTeam}
			detail, ok := eaMatchConsistent(report, player)
			if ok != tc.wantOK {
				t.Fatalf("eaMatchConsistent ok = %v (detail %q), want %v", ok, detail, tc.wantOK)
			}
			if !ok && detail == "" {
				t.Fatalf("rejected match must carry a detail reason")
			}
		})
	}
}
