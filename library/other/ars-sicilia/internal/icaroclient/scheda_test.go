package icaroclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Una scheda ridotta ai blocchi che parseScheda legge, con dentro i casi veri
// che le sedute hanno mostrato: un gruppo con le parentesi annidate (leg. XVI),
// un oratore senza gruppo e col punto finale (leg. XIII), chi presiede senza
// ruolo, un ordine del giorno che punta a un altro host e un allegato con URL
// relativo, spazi e accento doppio-codificato (leg. XVIII).
const schedaFixture = `<html><body>
<div class="blocco"><h3>Ordine del giorno</h3>
 <div class="testo_gestionale pad_blocco"><p>
  &bull; <a href="https://w3.ars.sicilia.it/DocumentiEsterni/ODG_PDF/ODG_16.pdf" target="_blank">
      OdG e comunicazioni
  </a>
 </p></div></div>
<div class="blocco"><h3>Allegati al resoconto</h3>
 <div class="testo_gestionale pad_blocco"><p>
  &bull; <a href="/bd/resoconti/file/Allegato A della seduta n. 271di martedÃ¬ 08 settembre 2026.pdf?id=abc&attachid=def">Allegato A</a>
 </p></div></div>
<div class="blocco"><h3>Presidenza</h3>
 <div class="testo_gestionale pad_blocco"><p>
  &bull; <b>Leanza Vincenzo</b>, <i></i>
  <br/>
  &bull; <b>Lo Porto Guido</b>, <i>Presidente ARS</i>
 </p></div></div>
<div class="blocco"><h3>Oratori</h3>
 <div class="testo_gestionale pad_blocco"><p>
  &bull; Falcone Marco (Popolo della Libert&agrave; (PDL) - verso PPE).
  <br/>
  &bull; Savarino Giuseppa.
 </p></div></div>
<a href="/bd/resoconti/file/17_2020_07_23_208_D.pdf?id=6d0da3a8">PDF</a>
<div class="tab"><div class="testo_gestionale"><div class="overflow">
<pre id="textContent">
   Presidenza del vicepresidente Foti

   La seduta &egrave; aperta alle ore 16.23
</pre></div></div></div>
</body></html>`

func TestParseSchedaLeggeIBlocchi(t *testing.T) {
	d := parseScheda(schedaFixture, "https://dati.ars.sicilia.it")
	if d == nil {
		t.Fatal("parseScheda ha restituito nil")
	}
	if want := "https://dati.ars.sicilia.it/bd/resoconti/file/17_2020_07_23_208_D.pdf?id=6d0da3a8"; d.PDFURL != want {
		t.Errorf("PDFURL = %q, voluto %q", d.PDFURL, want)
	}

	// Il gruppo arriva fino all'ultima parentesi: chiudendo alla prima diventava
	// «PDL» e il resto del nome ci finiva dentro.
	if len(d.Oratori) != 2 {
		t.Fatalf("oratori = %+v, volute 2 voci", d.Oratori)
	}
	if d.Oratori[0].Nome != "Falcone Marco" || d.Oratori[0].Gruppo != "Popolo della Libertà (PDL) - verso PPE" {
		t.Errorf("oratore con parentesi annidate letto male: %+v", d.Oratori[0])
	}
	// Sulle sedute vecchie il gruppo non c'è e resta il punto del portale.
	if d.Oratori[1].Nome != "Savarino Giuseppa" || d.Oratori[1].Gruppo != "" {
		t.Errorf("oratore senza gruppo letto male: %+v", d.Oratori[1])
	}

	if len(d.Presidenza) != 2 {
		t.Fatalf("presidenza = %+v, volute 2 voci", d.Presidenza)
	}
	if d.Presidenza[0].Nome != "Leanza Vincenzo" || d.Presidenza[0].Ruolo != "" {
		t.Errorf("presidenza senza ruolo letta male: %+v", d.Presidenza[0])
	}
	if d.Presidenza[1].Ruolo != "Presidente ARS" {
		t.Errorf("ruolo = %q, voluto «Presidente ARS»", d.Presidenza[1].Ruolo)
	}

	// L'ordine del giorno vive su un altro host: l'URL assoluto non si tocca.
	if len(d.ODG) != 1 || d.ODG[0].Titolo != "OdG e comunicazioni" ||
		d.ODG[0].URL != "https://w3.ars.sicilia.it/DocumentiEsterni/ODG_PDF/ODG_16.pdf" {
		t.Errorf("odg = %+v", d.ODG)
	}

	// L'allegato arriva relativo e con spazi: esce assoluto e percent-encodato,
	// altrimenti non si apre da nessuna parte.
	// La versione testuale sta in un <pre> suo: si legge da lì, non dal testo
	// della pagina, che porterebbe dentro menu e blocchi descrittivi.
	if !strings.Contains(d.Testo, "La seduta è aperta alle ore 16.23") ||
		strings.Contains(d.Testo, "Ordine del giorno") {
		t.Errorf("testo letto male: %q", d.Testo)
	}

	if len(d.Allegati) != 1 {
		t.Fatalf("allegati = %+v, voluta 1 voce", d.Allegati)
	}
	got := d.Allegati[0].URL
	want := "https://dati.ars.sicilia.it/bd/resoconti/file/Allegato%20A%20della%20seduta%20n.%20271di%20marted%C3%83%C2%AC%2008%20settembre%202026.pdf?id=abc&attachid=def"
	if got != want {
		t.Errorf("allegato:\n got  %q\n want %q", got, want)
	}
}

// Una scheda senza blocchi descrittivi non deve perdere il PDF: quello viene dal
// sorgente e non dal DOM, ed è la sola cosa che la risposta deve garantire.
func TestParseSchedaSenzaBlocchiTieneIlPDF(t *testing.T) {
	d := parseScheda(`<html><body><a href="/bd/resoconti/file/x.pdf?id=1">PDF</a></body></html>`, "https://dati.ars.sicilia.it")
	if d.PDFURL == "" {
		t.Error("il PDF deve uscire anche senza blocchi")
	}
	if len(d.Oratori) != 0 || len(d.Presidenza) != 0 || len(d.ODG) != 0 || len(d.Allegati) != 0 {
		t.Errorf("blocchi assenti devono restare vuoti: %+v", d)
	}
	// Oltre la frontiera della versione testuale la scheda ha solo il PDF: il
	// campo resta vuoto, e sta al chiamante dirne il motivo.
	if d.Testo != "" {
		t.Errorf("testo = %q, voluto vuoto su una scheda senza <pre>", d.Testo)
	}
}

// Una scheda che non si apre deve tornare come errore, non come dettaglio
// vuoto. Il chiamante, da quando la scheda è il percorso principale di `get`,
// distingue su questo errore la caduta del backend dal documento senza
// allegato: nel primo caso ripiega sull'indice Icaro, nel secondo risponde.
func TestSchedaPropagaIlGuasto(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := clientDiProva(t, srv.URL)
	d, err := c.Scheda(context.Background(), srv.URL+"/bd/resoconti/scheda/17/208")
	if err == nil {
		t.Fatalf("atteso errore, ottenuto dettaglio %+v", d)
	}
	if d != nil {
		t.Errorf("con l'errore il dettaglio deve essere nil, non %+v", d)
	}
}

// URL vuoto non è un guasto: non c'è niente da aprire, e il chiamante lo
// riconosce dal (nil, nil) invece di credere a una scheda senza campi.
func TestSchedaSenzaURLNonEUnGuasto(t *testing.T) {
	c := clientDiProva(t, "http://127.0.0.1:1")
	d, err := c.Scheda(context.Background(), "  ")
	if err != nil || d != nil {
		t.Errorf("atteso (nil, nil), ottenuto (%+v, %v)", d, err)
	}
}
