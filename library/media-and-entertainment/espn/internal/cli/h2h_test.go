package cli

import "testing"

func TestAtoiSafe(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"0", 0},
		{"42", 42},
		{"", 0},
		{"abc", 0},
		{"12x", 12},
	}
	for _, c := range cases {
		got := atoiSafe(c.in)
		if got != c.want {
			t.Errorf("atoiSafe(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestH2hCmdRequiresTwoTeams(t *testing.T) {
	cmd := newH2hCmd(&rootFlags{})
	cmd.SetArgs([]string{"KC"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing second team")
	}
	cliErr, ok := err.(*cliError)
	if !ok {
		t.Fatalf("expected *cliError, got %T", err)
	}
	if cliErr.code != 2 {
		t.Errorf("expected exit code 2, got %d", cliErr.code)
	}
}

func TestH2hCmdRequiresSportLeague(t *testing.T) {
	cmd := newH2hCmd(&rootFlags{})
	cmd.SetArgs([]string{"KC", "BUF"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --sport/--league")
	}
}
