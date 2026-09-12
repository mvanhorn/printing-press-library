package cli

import (
	"errors"
	"strings"
	"testing"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
)

// Sugli archivi /bd/ la scheda è il percorso principale di `get`, non un
// ripiego per le sedute che l'indice Icaro non ha: Icaro tiene i resoconti
// spezzati per punto dell'ordine del giorno e `get` apriva il primo frammento
// come se fosse la seduta. Se questo elenco cambia, cambia anche quali `get`
// interrogano /bd/ per primo.
func TestResocontiSonoServitiDaBD(t *testing.T) {
	if !icaro.IsBDArchive("resoconti") {
		t.Fatal("resoconti deve restare un archivio /bd/: è da lì che `get` prende la scheda con pdf_url")
	}
	if icaro.IsBDArchive("ddl") {
		t.Error("ddl non è servito da /bd/: `get` deve restare sul percorso Icaro")
	}
}

// Il ramo Icaro superstite consegna un frammento, e la nota deve dire sia che
// lo è sia perché ci si è finiti: con /bd/ muto basta riprovare, con il record
// assente da /bd/ no.
func TestNotaRamoIcaroDistingueLeDueCause(t *testing.T) {
	muto := notaRamoIcaro(errors.New("stream error INTERNAL_ERROR"))
	assente := notaRamoIcaro(nil)
	for _, n := range []string{muto, assente} {
		if !strings.Contains(n, "frammenti per punto dell'ordine del giorno") {
			t.Errorf("la nota deve dire che `body` è un frammento: %q", n)
		}
		if !strings.Contains(n, "un'altra seduta") {
			t.Errorf("la nota deve avvertire che il frammento può parlare di un'altra seduta: %q", n)
		}
	}
	if !strings.Contains(muto, "non ha risposto") || !strings.Contains(muto, "Riprova") {
		t.Errorf("backend muto: la nota deve dirlo e invitare a riprovare: %q", muto)
	}
	// Con bdErr nil il backend ha risposto, ma una sua risposta troncata torna
	// come zero righe: la nota non deve affermare che il record non esiste.
	if strings.Contains(assente, "non ha risposto") {
		t.Errorf("con /bd/ che ha risposto la nota non deve dire il contrario: %q", assente)
	}
	if !strings.Contains(assente, "ha risposto vuoto") {
		t.Errorf("la nota deve ammettere anche la risposta vuota, che su /bd/ capita: %q", assente)
	}
}
