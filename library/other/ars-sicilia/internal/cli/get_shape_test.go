package cli

import (
	"encoding/json"
	"testing"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
)

// `<archivio> get` aveva due forme: la scheda Icaro teneva numero e data dentro
// `fields`, la scheda /bd/ in radice. Lo stesso `--select numero,data_iso`
// rendeva su una seduta recente e tornava `{}` su una storica, con exit 0.
// Le coordinate devono stare in radice su entrambi i rami, con gli stessi nomi
// e tipi della scheda /bd/.
func TestGetOutPortaLeCoordinateInRadice(t *testing.T) {
	doc := icaro.Doc{
		Fields: map[string]string{
			"Numero": "147",
			"Data":   "18.12.24",
			"Titolo": "Ordine del giorno della seduta successiva",
		},
	}
	raw, err := json.Marshal(getOut{
		Doc:    doc,
		Legisl: 18,
		Numero: 147,
		Data:   doc.Fields["Data"],
		Titolo: titoloDoc(doc),
		Fonte:  "icaro",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// numero e legisl sono numeri, come sulla scheda /bd/: non la stringa che
	// sta in fields. Vengono dagli argomenti del comando, che sono autorevoli.
	if n, _ := got["numero"].(float64); n != 147 {
		t.Errorf("numero = %v (%T), voluto 147 come numero", got["numero"], got["numero"])
	}
	if l, _ := got["legisl"].(float64); l != 18 {
		t.Errorf("legisl = %v, voluto 18", got["legisl"])
	}
	if got["titolo"] != "Ordine del giorno della seduta successiva" {
		t.Errorf("titolo = %v, atteso quello del campo Titolo", got["titolo"])
	}
	if got["data"] != "18.12.24" {
		t.Errorf("data = %v: deve uscire grezza, cosi' iniettaDataISO le affianca data_iso", got["data"])
	}
	if got["fonte"] != "icaro" {
		t.Errorf("fonte = %v, voluto icaro", got["fonte"])
	}
	// `fields` resta dov'e': questa e' un'aggiunta, non uno spostamento, e chi
	// legge fields.Numero deve continuare a funzionare.
	f, _ := got["fields"].(map[string]any)
	if f["Numero"] != "147" {
		t.Errorf("fields.Numero = %v: il percorso vecchio non va rotto", f["Numero"])
	}
}

// La data grezza deve poi diventare data_iso per la stessa via di tutti gli
// altri payload, senza un percorso dedicato al `get`.
func TestGetOutDataDiventaISO(t *testing.T) {
	raw, err := json.Marshal(getOut{Numero: 147, Data: "18.12.24"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(iniettaDataISO(raw), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["data_iso"] != "2024-12-18" {
		t.Errorf("data_iso = %v, voluto 2024-12-18", got["data_iso"])
	}
}

// La fonte mette il titolo in due posti diversi a seconda dell'archivio: sui
// resoconti la scheda ha titolo vuoto e il campo pieno, sui ddl il contrario.
func TestTitoloDocPescaDoveLaFonteLoMette(t *testing.T) {
	casi := []struct {
		nome  string
		doc   icaro.Doc
		vuole string
	}{
		{"titolo nella scheda", icaro.Doc{Title: "Rendiconto generale"}, "Rendiconto generale"},
		{"titolo solo nel campo", icaro.Doc{Fields: map[string]string{"Titolo": "Ordine del giorno"}}, "Ordine del giorno"},
		{"scheda vince sul campo", icaro.Doc{Title: "Scheda", Fields: map[string]string{"Titolo": "Campo"}}, "Scheda"},
		{"nessuno dei due", icaro.Doc{}, ""},
	}
	for _, c := range casi {
		if got := titoloDoc(c.doc); got != c.vuole {
			t.Errorf("%s: titoloDoc = %q, voluto %q", c.nome, got, c.vuole)
		}
	}
}
