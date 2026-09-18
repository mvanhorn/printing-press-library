// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

func colonneDiProva() []colonna {
	return []colonna{
		{Nome: "Codice CUP", NomeFisico: "ccodice_cup", ID: "Cccodice_cup_1267962549", Tipo: "STRING"},
		{Nome: "Descrizione Titolare", NomeFisico: "cdescrizione_titolare", ID: "Ccdescrizione_ti177583083", Tipo: "STRING"},
		{Nome: "Costo Lavori Previsto", NomeFisico: "ccosto_lavori_previsto", ID: "Cccosto_lavori_1978461874", Tipo: "NUMERIC"},
	}
}

func TestRisolviColonna(t *testing.T) {
	colonne := colonneDiProva()
	casi := map[string]string{
		"Codice CUP":              "Cccodice_cup_1267962549",
		"codice cup":              "Cccodice_cup_1267962549",
		"ccodice_cup":             "Cccodice_cup_1267962549",
		"Cccodice_cup_1267962549": "Cccodice_cup_1267962549",
		"titolare":                "Ccdescrizione_ti177583083",
	}
	for chiave, atteso := range casi {
		col, ok := risolviColonna(colonne, chiave)
		if !ok || col.ID != atteso {
			t.Errorf("risolviColonna(%q) = (%q,%v), atteso %q", chiave, col.ID, ok, atteso)
		}
	}
	if _, ok := risolviColonna(colonne, "colonna inesistente"); ok {
		t.Error("una colonna inesistente non deve risolvere")
	}
}

func TestCostruisciFiltro(t *testing.T) {
	colonne := colonneDiProva()

	filtro, err := costruisciFiltro(colonne, []string{"Codice CUP=I77H11000120009"})
	if err != nil {
		t.Fatalf("errore inatteso: %v", err)
	}
	if filtro != "Cccodice_cup_1267962549 eq 'I77H11000120009'" {
		t.Errorf("filtro = %q", filtro)
	}

	filtro, err = costruisciFiltro(colonne, []string{"Descrizione Titolare~COMUNE DI PALERMO", "Codice CUP=ABC"})
	if err != nil {
		t.Fatalf("errore inatteso: %v", err)
	}
	atteso := "substringof('COMUNE DI PALERMO',Ccdescrizione_ti177583083) and Cccodice_cup_1267962549 eq 'ABC'"
	if filtro != atteso {
		t.Errorf("filtro = %q, atteso %q", filtro, atteso)
	}

	// L'apostrofo nel valore va raddoppiato, altrimenti chiude la stringa OData.
	filtro, err = costruisciFiltro(colonne, []string{"Descrizione Titolare=VALLE D'AOSTA"})
	if err != nil {
		t.Fatalf("errore inatteso: %v", err)
	}
	if filtro != "Ccdescrizione_ti177583083 eq 'VALLE D''AOSTA'" {
		t.Errorf("filtro = %q", filtro)
	}

	if _, err := costruisciFiltro(colonne, []string{"Colonna Inventata=1"}); err == nil {
		t.Error("una colonna inesistente deve produrre un errore")
	}
	if _, err := costruisciFiltro(colonne, []string{"senza separatore"}); err == nil {
		t.Error("una condizione senza separatore deve produrre un errore")
	}
	if filtro, err := costruisciFiltro(colonne, nil); err != nil || filtro != "" {
		t.Errorf("nessuna condizione deve dare filtro vuoto, ottenuto (%q,%v)", filtro, err)
	}
}

func TestOdataPath(t *testing.T) {
	got := odataPath("bda1676b", "/DataRows")
	if got != "/ODataProxy/MdData('bda1676b@rgs')/DataRows" {
		t.Errorf("odataPath = %q", got)
	}
}

// Una chiave vuota non deve risolvere: la ricerca parziale la troverebbe in
// qualunque nome e filtrerebbe sulla prima colonna del dataset.
func TestRisolviColonnaChiaveVuota(t *testing.T) {
	for _, chiave := range []string{"", "   "} {
		if col, ok := risolviColonna(colonneDiProva(), chiave); ok {
			t.Errorf("risolviColonna(%q) ha risolto su %q", chiave, col.ID)
		}
	}
}

func TestCostruisciFiltroColonnaVuota(t *testing.T) {
	if _, err := costruisciFiltro(colonneDiProva(), []string{"=I39B05000060005"}); err == nil {
		t.Error("una condizione senza nome di colonna deve produrre un errore")
	}
}

func TestScartaVirgolette(t *testing.T) {
	casi := map[string]string{
		`"valore"`: "valore",
		"'valore'": "valore",
		"valore":   "valore",
		"VALLE D'": "VALLE D'", // l'apostrofo finale non e' una virgoletta di apertura
		`"`:        `"`,
	}
	for dato, atteso := range casi {
		if got := scartaVirgolette(dato); got != atteso {
			t.Errorf("scartaVirgolette(%q) = %q, atteso %q", dato, got, atteso)
		}
	}
}

func TestFiltroUguale(t *testing.T) {
	if got := filtroUguale("Cc1", "VALLE D'AOSTA"); got != "Cc1 eq 'VALLE D''AOSTA'" {
		t.Errorf("filtroUguale = %q", got)
	}
}

func TestControllaCodice(t *testing.T) {
	validi := map[string]string{"cup": "I77H11000120009", "cig": "57106934F1"}
	for etichetta, codice := range validi {
		if err := controllaCodice(codice, etichetta); err != nil {
			t.Errorf("controllaCodice(%q,%q) = %v", codice, etichetta, err)
		}
	}
	invalidi := map[string]string{"cup": "PIPPO", "cig": "123"}
	for etichetta, codice := range invalidi {
		if err := controllaCodice(codice, etichetta); err == nil {
			t.Errorf("controllaCodice(%q,%q) doveva fallire", codice, etichetta)
		}
	}
	// Un'etichetta sconosciuta non deve bloccare: nessun formato da imporre.
	if err := controllaCodice("qualsiasi", "altro"); err != nil {
		t.Errorf("etichetta sconosciuta = %v", err)
	}
}

// Una chiave parziale che corrisponde a piu' colonne non deve risolvere:
// sceglierne una in silenzio filtrerebbe sul campo sbagliato.
func TestRisolviColonnaAmbigua(t *testing.T) {
	colonne := []colonna{
		{Nome: "Codice CUP", ID: "Cc1"},
		{Nome: "Codice CIG", ID: "Cc2"},
		{Nome: "Descrizione Titolare", ID: "Cc3"},
	}
	if col, ok := risolviColonna(colonne, "codice"); ok {
		t.Errorf("la chiave ambigua ha risolto su %q", col.ID)
	}
	if col, ok := risolviColonna(colonne, "titolare"); !ok || col.ID != "Cc3" {
		t.Errorf("la chiave con una sola corrispondenza deve risolvere, ottenuto (%q,%v)", col.ID, ok)
	}
	// Il nome esatto resta risolvibile anche quando e' prefisso di altri.
	if col, ok := risolviColonna(colonne, "Codice CUP"); !ok || col.ID != "Cc1" {
		t.Errorf("il nome esatto deve risolvere, ottenuto (%q,%v)", col.ID, ok)
	}
	ambigue := colonneAmbigue(colonne, "codice")
	if len(ambigue) != 2 {
		t.Errorf("colonneAmbigue = %v, attese 2 voci", ambigue)
	}
	if colonneAmbigue(colonne, "titolare") != nil {
		t.Error("una sola corrispondenza non e' ambigua")
	}
}

func TestCostruisciFiltroColonnaAmbigua(t *testing.T) {
	colonne := []colonna{{Nome: "Codice CUP", ID: "Cc1"}, {Nome: "Codice CIG", ID: "Cc2"}}
	_, err := costruisciFiltro(colonne, []string{"codice=X"})
	if err == nil {
		t.Fatal("una colonna ambigua deve produrre un errore")
	}
	if !strings.Contains(err.Error(), "ambigua") {
		t.Errorf("l'errore deve spiegare l'ambiguita', ottenuto: %v", err)
	}
}

func TestCompattaRighe(t *testing.T) {
	righe := []map[string]any{{
		"Codice CUP":                  "I77H11000120009",
		"Costo Lavori Previsto":       "1076000.00",
		"Costo Lavori Effettivo":      "0.00",
		"Fine esecuzione prevista":    "",
		"Oneri Investimento Previsti": float64(0),
	}}
	// Senza --compact la riga resta intera: i campi vuoti sono un'informazione.
	if got := compattaRighe(righe, false); len(got[0]) != 5 {
		t.Errorf("senza compattazione = %d campi, attesi 5", len(got[0]))
	}
	// Gli zeri restano: "Costo Lavori Effettivo: 0.00" e' un importo
	// registrato, non un campo mancante. Sparisce solo la stringa vuota.
	compatte := compattaRighe(righe, true)
	if len(compatte[0]) != 4 {
		t.Errorf("con compattazione = %v, attesi i quattro campi con valore", compatte[0])
	}
	if _, presente := compatte[0]["Costo Lavori Effettivo"]; !presente {
		t.Error("uno zero registrato deve restare")
	}
	if _, presente := compatte[0]["Fine esecuzione prevista"]; presente {
		t.Error("il campo vuoto doveva sparire")
	}
	if _, presente := compatte[0]["Codice CUP"]; !presente {
		t.Error("il CUP deve restare")
	}
}

func TestCampoVuoto(t *testing.T) {
	vuoti := []any{nil, "", "   "}
	for _, v := range vuoti {
		if !campoVuoto(v) {
			t.Errorf("campoVuoto(%#v) = false", v)
		}
	}
	pieni := []any{"A", "0.01", "0.00", "0", float64(0), float64(1), "2011-04-11", "00107730079"}
	for _, v := range pieni {
		if campoVuoto(v) {
			t.Errorf("campoVuoto(%#v) = true", v)
		}
	}
}
