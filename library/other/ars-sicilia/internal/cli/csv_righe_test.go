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

// Una sezione vuota non deve far perdere la forma annidata: `novita` ha quasi
// sempre un archivio senza novità, e il CSV usciva con `[map[...]]` in colonna.
func TestCSVConSezioneVuota(t *testing.T) {
	data := json.RawMessage(`{"archivi":[{"archivio":"ddl","nuovi":[{"numero":"1"},{"numero":"2"}]},{"archivio":"pareri","nota":"non databile","nuovi":[]},{"archivio":"leggi","nuovi":null}]}`)
	var buf bytes.Buffer
	if err := printCSV(&buf, data); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(buf.String()), "archivio,numero\nddl,1\nddl,2"; got != want {
		t.Fatalf("csv = %q, want %q", got, want)
	}
	_, vuote := righeDaOggetto(data)
	got := strings.Join(avvisiSezioniVuote(vuote), "|")
	if want := "hint: --csv: pareri non ha righe: non databile|hint: --csv: leggi non ha righe"; got != want {
		t.Fatalf("avvisi = %q, want %q", got, want)
	}
}

// Tutte vuote: non c'è una sotto-lista da riconoscere, resta la tabella delle
// sezioni, che porta le note.
func TestCSVConTutteLeSezioniVuote(t *testing.T) {
	righe, vuote := righeDaOggetto(json.RawMessage(`{"archivi":[{"archivio":"ddl","nuovi":[]},{"archivio":"pareri","nuovi":[]}]}`))
	if len(righe) != 2 || vuote != nil {
		t.Fatalf("righe = %v, vuote = %v", righe, vuote)
	}
}

// Righe con una lista di forma diversa da quella delle altre: non è la forma
// annidata e non si appiattisce.
func TestCSVNonAppiattisceFormeDiverse(t *testing.T) {
	data := json.RawMessage(`{"righe":[{"n":1,"a":[{"x":1}]},{"n":2,"b":[{"y":1}]}]}`)
	righe, vuote := righeDaOggetto(data)
	if len(righe) != 2 || vuote != nil || righe[0]["n"] != float64(1) {
		t.Fatalf("righe = %v, vuote = %v", righe, vuote)
	}
	senzaChiave := json.RawMessage(`{"righe":[{"n":1,"a":[{"x":1}]},{"n":2}]}`)
	if righe, _ := righeDaOggetto(senzaChiave); len(righe) != 2 || righe[1]["n"] != float64(2) {
		t.Fatalf("riga senza lista persa: %v", righe)
	}
}

func TestCSVNonIndovinaQuandoLeListeSonoDue(t *testing.T) {
	// Con due liste di oggetti in radice non è deducibile quale sia quella da
	// mettere in tabella: sceglierne una produrrebbe un CSV plausibile e
	// sbagliato, quindi si torna al JSON (e printCSV lo dice su stderr).
	data := json.RawMessage(`{"uno":[{"a":1}],"due":[{"b":2}]}`)
	if righe, _ := righeDaOggetto(data); righe != nil {
		t.Errorf("ha scelto una lista fra due: %v", righe)
	}
	// E un oggetto senza liste resta JSON.
	if righe, _ := righeDaOggetto(json.RawMessage(`{"a":1,"b":"x"}`)); righe != nil {
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

// Un oggetto o una lista in una cella escono in JSON, non come `map[...]`.
func TestCSVCellaAnnidataInJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := printCSV(&buf, json.RawMessage(`[{"entry":{"command":"ddl"},"score":3,"tag":["a"]}]`)); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	want := "entry,score,tag\n\"{\"\"command\"\":\"\"ddl\"\"}\",3,\"[\"\"a\"\"]\""
	if got != want {
		t.Fatalf("csv = %q, want %q", got, want)
	}
}
