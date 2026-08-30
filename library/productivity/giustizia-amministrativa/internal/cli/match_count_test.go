package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/giustizia-amministrativa/internal/gaclient"
)

func TestSearchTerms(t *testing.T) {
	opts := gaclient.SearchOptions{Testo: "appalti pubblici il"}
	terms := searchTerms(opts)
	// "il" is < 3 runes, dropped
	if len(terms) != 2 || terms[0] != "appalti" || terms[1] != "pubblici" {
		t.Errorf("terms = %v, want [appalti pubblici]", terms)
	}
}

func TestSearchTermsPhrase(t *testing.T) {
	opts := gaclient.SearchOptions{Phrase: "accesso civico generalizzato"}
	terms := searchTerms(opts)
	if len(terms) != 1 || terms[0] != "accesso civico generalizzato" {
		t.Errorf("phrase term = %v, want [accesso civico generalizzato]", terms)
	}
}

func TestCountMatches(t *testing.T) {
	text := "L'appalto e' affidato. L'APPALTO non e' in variante. Sugli appalti pubblici."
	terms := []string{"appalti", "pubblici"}
	count := countMatches(text, terms)
	// "appalti" appears 2x (lowercased: appalto, appalti — wait, "appalto" != "appalti")
	// Actually: lowercase = "l'appalto e' affidato. l'appalto non e' in variante. sugli appalti pubblici."
	// "appalti" appears once ("appalti pubblici")
	// "pubblici" appears once
	// Total = 2
	// But "appalto" != "appalti" so those don't count
	if count != 2 {
		t.Errorf("count = %d, want 2 (appalti=1, pubblici=1)", count)
	}
}

func TestCountMatchesEmptyTerms(t *testing.T) {
	if count := countMatches("some text", nil); count != 0 {
		t.Errorf("empty terms: count = %d, want 0", count)
	}
}

func TestCountMatchesCaseInsensitive(t *testing.T) {
	text := "APPALTO appalto Appalto"
	count := countMatches(text, []string{"appalto"})
	if count != 3 {
		t.Errorf("case-insensitive count = %d, want 3", count)
	}
}
