package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestCSVDaOggettoCheAvvolgeUnaLista(t *testing.T) {
	// La forma di `deputato profilo`: le righe stanno in `atti`, accanto a
	// `conteggio` (oggetto) e `troncato` (lista di stringhe), che non sono
	// righe e non devono essere scambiate per tali.
	data := json.RawMessage(`{"atti":[{"tipo":"ddl","data":"3.08.26"},{"tipo":"mozioni","data":"1.07.26"}],"conteggio":{"ddl":1},"troncato":["interrogazioni"],"deputato":"Tal dei Tali"}`)
	var buf bytes.Buffer
	if err := printCSV(&buf, data); err != nil {
		t.Fatal(err)
	}
	righe := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(righe) != 3 {
		t.Fatalf("attese intestazione + 2 righe, ottenuto:\n%s", buf.String())
	}
	if righe[0] != "data,tipo" {
		t.Errorf("intestazione = %q", righe[0])
	}
	if righe[1] != "3.08.26,ddl" {
		t.Errorf("prima riga = %q", righe[1])
	}
}

func TestCSVDaSezioniAnnidate(t *testing.T) {
	// La forma di `commissione dossier`: ogni sezione avvolge le sue righe, e
	// i campi scalari della sezione devono restare su ogni riga, altrimenti si
	// perde a quale sezione apparteneva.
	data := json.RawMessage(`{"sezioni":[{"tipo":"convocazioni","archivio":"229","risultati":[{"numero":"152"}]},{"tipo":"sommari","archivio":"230","risultati":[{"numero":"270"},{"numero":"269"}]}],"commissione":"SESTA"}`)
	var buf bytes.Buffer
	if err := printCSV(&buf, data); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	righe := strings.Split(strings.TrimSpace(out), "\n")
	if len(righe) != 4 {
		t.Fatalf("attese intestazione + 3 righe, ottenuto:\n%s", out)
	}
	if righe[0] != "archivio,numero,tipo" {
		t.Errorf("intestazione = %q", righe[0])
	}
	if !strings.Contains(out, "229,152,convocazioni") || !strings.Contains(out, "230,269,sommari") {
		t.Errorf("le righe non portano la sezione di provenienza:\n%s", out)
	}
}

func TestCSVNonIndovinaQuandoLeListeSonoDue(t *testing.T) {
	// Con due liste di oggetti in radice non è deducibile quale sia quella da
	// mettere in tabella: sceglierne una produrrebbe un CSV plausibile e
	// sbagliato, quindi si torna al JSON (e printCSV lo dice su stderr).
	data := json.RawMessage(`{"uno":[{"a":1}],"due":[{"b":2}]}`)
	if righe := righeDaOggetto(data); righe != nil {
		t.Errorf("ha scelto una lista fra due: %v", righe)
	}
	// E un oggetto senza liste resta JSON.
	if righe := righeDaOggetto(json.RawMessage(`{"a":1,"b":"x"}`)); righe != nil {
		t.Errorf("righe inventate da un oggetto piatto: %v", righe)
	}
}

func TestCSVArrayInRadiceNonCambia(t *testing.T) {
	data := json.RawMessage(`[{"numero":"1"},{"numero":"2"}]`)
	var buf bytes.Buffer
	if err := printCSV(&buf, data); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != "numero\n1\n2" {
		t.Errorf("output = %q", got)
	}
}
