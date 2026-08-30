package icaroclient

import "testing"

// I tre corpi che il portale sa rispondere, ridotti ai blocchi che li
// distinguono. Catturati il 2026-08-29 sull'archivio ddl, stessa sessione,
// variando solo l'estremo destro del range DATPRE.
const (
	corpoPieno = `<div class="pad_form"><div>Caricamento in corso...</div></div>
		<ul id="resultsList"><li><p>1. Disegni di Legge</p>
		<h3 class="arrowDiv arrow"><a>Lista Documenti</a>(460)</h3></li></ul>`

	corpoVuotoVero = `<div class="pad_form"><div>Caricamento in corso...</div></div>
		<ul id="resultsList"><li><p>777. Disegni di Legge</p>
		<h3 class="arrowDiv arrow"><a>Lista Documenti</a>(0)</h3>
		<p>Non esistono documenti corrispondenti alla ricerca formulata.</p></li></ul>`

	corpoErrore = `<div class="pad_form">
		<div style="margin: 50px 0 40px 0; text-align: center; font-size: 22px;">
			Si è verificato un errore durante l'esecuzione<br/>di un'operazione con il Server
		</div>
		<div class="message ko">
			 (QR997)
		</div>
	</div>
	<ul id="resultsList"></ul>`
)

// Il difetto che questo test chiude: il corpo d'errore e il vuoto vero
// uscivano entrambi come lista vuota, e solo uno dei due è una risposta.
func TestDetectQueryError_DistingueErroreDaVuotoVero(t *testing.T) {
	code, failed := DetectQueryError(corpoErrore)
	if !failed {
		t.Fatal("corpo d'errore: DetectQueryError non l'ha riconosciuto")
	}
	if code != "QR997" {
		t.Errorf("codice = %q, atteso QR997", code)
	}

	if _, failed := DetectQueryError(corpoVuotoVero); failed {
		t.Error("vuoto vero scambiato per errore: zero documenti è una risposta, non un fallimento")
	}
	if _, failed := DetectQueryError(corpoPieno); failed {
		t.Error("corpo con risultati scambiato per errore")
	}
}

// Il codice può mancare: quello che conta è che la pagina sia d'errore, non che
// il portale abbia scritto una sigla dentro il blocco.
func TestDetectQueryError_SenzaCodice(t *testing.T) {
	_, failed := DetectQueryError(`<div class="message ko"> </div>`)
	if !failed {
		t.Fatal("blocco message ko senza codice: atteso riconoscimento comunque")
	}
}

// Count leggeva il conteggio dallo stesso corpo: sulla pagina d'errore il blocco
// del totale non c'è, e senza questo controllo ripiegava sulle pagine ritornando
// 0 — cioè lo stesso numero del vuoto vero.
func TestParseResultCount_AssenteSulCorpoDErrore(t *testing.T) {
	if _, ok := ParseResultCount(corpoErrore); ok {
		t.Fatal("corpo d'errore: il totale non deve risultare presente")
	}
}
