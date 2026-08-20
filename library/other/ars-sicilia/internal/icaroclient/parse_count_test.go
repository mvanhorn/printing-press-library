package icaroclient

import "testing"

// Il totale sta nella pagina che apre la sessione, ma quel blocco è la cronologia
// della sessione: una voce per ogni ricerca fatta, la più recente in fondo.
// Leggendo la prima, tre conteggi diversi tornavano tutti col numero del primo.
func TestParseResultCount_PrendeLUltimaRicerca(t *testing.T) {
	body := `<ul id="resultsList">
		<li><p>1. Disegni di Legge</p><h3 class="arrowDiv arrow"><a>Lista Documenti</a>(302)</h3></li>
		<li><p>2. Disegni di Legge</p><h3 class="arrowDiv arrow"><a>Lista Documenti</a>	(7)	</h3></li>
	</ul>`
	n, ok := ParseResultCount(body)
	if !ok || n != 7 {
		t.Fatalf("ParseResultCount = %d, %v; atteso 7, true", n, ok)
	}
}

// Zero documenti e totale assente sono due cose diverse: senza il secondo valore
// un parsing fallito diventerebbe "archivio vuoto".
func TestParseResultCount_AssenteNonEZero(t *testing.T) {
	if n, ok := ParseResultCount("<html><body>nessun blocco</body></html>"); ok || n != 0 {
		t.Fatalf("totale assente: atteso 0,false; ottenuto %d,%v", n, ok)
	}
	if n, ok := ParseResultCount(`<h3><a>Lista Documenti</a>(0)</h3>`); !ok || n != 0 {
		t.Fatalf("zero dichiarato: atteso 0,true; ottenuto %d,%v", n, ok)
	}
}
