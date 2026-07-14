package setfile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/mt5-backtester/internal/config"
	"github.com/mvanhorn/printing-press-library/library/other/mt5-backtester/internal/setfile"
)

func TestParseAndTesterValue(t *testing.T) {
	src := `; comment, skipped
StopLoss=50||50||1||200||N
TakeProfit=100||100||1||500||Y
MagicNumber=99001
UseFilter=true
`
	sf, err := setfile.ParseReader(strings.NewReader(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := len(sf.Params); got != 4 {
		t.Fatalf("want 4 params, got %d", got)
	}

	sl, _ := sf.Get("StopLoss")
	if sl.TesterValue() != "50||50||1||200||N" {
		t.Errorf("StopLoss TesterValue=%q", sl.TesterValue())
	}
	tp, _ := sf.Get("TakeProfit")
	if !tp.OptOn || tp.TesterValue() != "100||100||1||500||Y" {
		t.Errorf("TakeProfit unexpected: %+v / %q", tp, tp.TesterValue())
	}
	mn, _ := sf.Get("MagicNumber")
	if mn.HasRange || mn.TesterValue() != "99001" {
		t.Errorf("MagicNumber unexpected: %+v / %q", mn, mn.TesterValue())
	}
	uf, _ := sf.Get("UseFilter")
	if uf.TesterValue() != "true" || !setfile.IsBool(uf.Value) {
		t.Errorf("UseFilter unexpected: %+v", uf)
	}
}

func TestApplyOverridesPreservesRange(t *testing.T) {
	sf, _ := setfile.ParseReader(strings.NewReader("StopLoss=50||50||1||200||N\n"))
	sf.ApplyOverrides(map[string]string{"StopLoss": "75"})
	p, _ := sf.Get("StopLoss")
	if p.Value != "75" {
		t.Errorf("Value = %q", p.Value)
	}
	if !p.HasRange || p.OptStart != "50" || p.OptStop != "200" {
		t.Errorf("range lost: %+v", p)
	}
	if p.TesterValue() != "75||50||1||200||N" {
		t.Errorf("TesterValue=%q", p.TesterValue())
	}
}

func TestINIFromSetFile(t *testing.T) {
	dir := t.TempDir()
	setPath := filepath.Join(dir, "MyEA.set")
	if err := os.WriteFile(setPath, []byte(
		"StopLoss=50||50||1||200||N\nTakeProfit=100||100||1||500||Y\nMagicNumber=99001\n"), 0644); err != nil {
		t.Fatal(err)
	}

	p := &config.BacktestParams{
		Expert: "Experts\\MyEA.ex5", Symbol: "EURUSD", Period: "H1",
		FromDate: "2024.01.01", ToDate: "2024.06.01",
		Model: 1, Deposit: 10000, Currency: "USD", Leverage: 100,
		ShutdownMode: 1, Report: "smoke",
		SetFile: setPath,
		Inputs:  map[string]string{"MagicNumber": "99002"}, // override
	}
	iniPath, err := config.WriteINI(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(iniPath)
	ini := string(data)

	for _, want := range []string{
		"[TesterInputs]",
		"StopLoss=50||50||1||200||N",
		"TakeProfit=100||100||1||500||Y",
		"MagicNumber=99002", // overridden, no range (original had none)
	} {
		if !strings.Contains(ini, want) {
			t.Errorf("ini missing %q\n---\n%s", want, ini)
		}
	}
}

func TestINIFromSimpleInputs(t *testing.T) {
	dir := t.TempDir()
	p := &config.BacktestParams{
		Expert: "Experts\\X.ex5", Symbol: "EURUSD", Period: "H1",
		FromDate: "2024.01.01", ToDate: "2024.06.01",
		Model: 1, Deposit: 10000, Currency: "USD", Leverage: 100,
		ShutdownMode: 1, Report: "simple",
		Inputs: map[string]string{"A": "1", "B": "2"},
	}
	iniPath, err := config.WriteINI(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(iniPath)
	ini := string(data)
	if !strings.Contains(ini, "A=1") || !strings.Contains(ini, "B=2") {
		t.Errorf("simple inputs missing:\n%s", ini)
	}
	if strings.Contains(ini, "||") {
		t.Errorf("unexpected || in simple-inputs ini:\n%s", ini)
	}
}
