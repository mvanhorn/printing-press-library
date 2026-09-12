package cli

import (
	"strings"
	"testing"
)

// L'avviso sulla punteggiatura descrive una riscrittura che avviene dentro
// BuildQuery. Gli archivi /bd/ BuildQuery non lo attraversano: il valore parte
// intatto nella POST del form e torna righe. Dirlo li' sarebbe un avviso che
// afferma il falso.
func TestPunteggiaturaHintMutoSugliArchiviBD(t *testing.T) {
	params := map[string]string{"testo": "dell'ambiente"}
	if h := punteggiaturaHint("resoconti", params); h != "" {
		t.Errorf("atteso muto su /bd/, ottenuto: %s", h)
	}
	if h := punteggiaturaHint("sommari", params); h != "" {
		t.Errorf("atteso muto su /bd/, ottenuto: %s", h)
	}
	h := punteggiaturaHint("ddl", params)
	if !strings.Contains(h, "dell ambiente") {
		t.Errorf("su un archivio ISIS l'avviso deve dire cosa e' partito: %q", h)
	}
}

// Su un campo identificativo parte un OR di ventisei grafie. L'avviso ne
// dichiara il NUMERO: l'espressione e' lunga quasi mille caratteri e riversarla
// nel terminale non la fa leggere a nessuno.
func TestPunteggiaturaHintSuIdentificativoDichiaraQuanteGrafie(t *testing.T) {
	h := punteggiaturaHint("biblioteca", map[string]string{"isbn": "978-88-7524-166-7"})
	if !strings.Contains(h, "26 grafie") {
		t.Errorf("l'avviso deve dire quante grafie sono partite: %q", h)
	}
	if strings.Contains(h, " adj ") || strings.Contains(h, "sostituiti da spazio") {
		t.Errorf("l'avviso non deve contenere l'espressione, ne' dire che la punteggiatura e' diventata spazio: %q", h)
	}
}
