package gaclient

import (
	"strings"
	"testing"
)

func TestFrontMatter(t *testing.T) {
	p := Provvedimento{
		Ecli:         "ECLI:IT:TARNA:2021:6765SENT",
		Tipo:         "Sentenza",
		Sede:         "NAPOLI",
		Sezione:      "SEZIONE 6",
		Numero:       6765,
		Anno:         2021,
		Nrg:          "202102978",
		DataDeposito: "28/10/2021",
		Formato:      "html",
		URL:          "https://mdp.giustizia-amministrativa.it/visualizzah2/?schema=tar_na&nrg=202102978&nomeFile=202106765_01.html",
	}
	fm := FrontMatter(p)

	if !strings.HasPrefix(fm, "---\n") || !strings.HasSuffix(fm, "---\n") {
		t.Fatalf("front matter must be delimited by ---:\n%s", fm)
	}
	for _, want := range []string{
		`ecli: "ECLI:IT:TARNA:2021:6765SENT"`,
		`tipo: "Sentenza"`,
		`sede: "NAPOLI"`,
		`sezione: "SEZIONE 6"`,
		"numero: 6765",
		"anno: 2021",
		`nrg: "202102978"`,
		`data_deposito: "28/10/2021"`,
		`formato: "html"`,
		`url: "https://mdp.giustizia-amministrativa.it/visualizzah2/?schema=tar_na&nrg=202102978&nomeFile=202106765_01.html"`,
	} {
		if !strings.Contains(fm, want) {
			t.Errorf("front matter missing %q in:\n%s", want, fm)
		}
	}
}

// Empty fields must be omitted; ints at zero must not appear.
func TestFrontMatterOmitsEmpty(t *testing.T) {
	fm := FrontMatter(Provvedimento{Nrg: "202102978"})
	if strings.Contains(fm, "ecli:") || strings.Contains(fm, "numero:") || strings.Contains(fm, "anno:") {
		t.Errorf("empty/zero fields should be omitted:\n%s", fm)
	}
	if !strings.Contains(fm, `nrg: "202102978"`) {
		t.Errorf("present field nrg should be emitted:\n%s", fm)
	}
}

// String values containing quotes/backslashes must be escaped.
func TestYAMLQuoteEscaping(t *testing.T) {
	if got := yamlQuote(`a"b\c`); got != `"a\"b\\c"` {
		t.Errorf("yamlQuote = %s", got)
	}
}

// Control characters (CR/LF/TAB) must be escaped so a stray CR from CRLF HTML
// cannot produce invalid YAML. A real front-matter value carrying a CR must
// keep the block on a single logical line (no raw newline leaks).
func TestYAMLQuoteControlChars(t *testing.T) {
	if got := yamlQuote("a\r\nb\tc"); got != `"a\r\nb\tc"` {
		t.Errorf("yamlQuote control chars = %s", got)
	}
	fm := FrontMatter(Provvedimento{DataDeposito: "28/10/2021\r"})
	if strings.Contains(fm, "\r") {
		t.Errorf("front matter leaked a raw CR:\n%q", fm)
	}
	if !strings.Contains(fm, `data_deposito: "28/10/2021\r"`) {
		t.Errorf("CR not escaped in front matter:\n%s", fm)
	}
}
