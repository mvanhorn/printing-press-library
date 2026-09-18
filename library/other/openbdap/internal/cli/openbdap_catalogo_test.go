// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestDerivaSerie(t *testing.T) {
	casi := []struct {
		titolo                        string
		anno, periodo, regione, serie string
	}{
		{"2024 - Prima Nota di Variazione Approvata Elaborabile Spese Capitolo", "2024", "2024", "", "Prima Nota di Variazione Approvata Elaborabile Spese Capitolo"},
		{"2025/08 - Pagamenti Bilancio dello Stato per Missione Amministrazione", "2025", "2025/08", "", "Pagamenti Bilancio dello Stato per Missione Amministrazione"},
		// Nelle serie SIOPE la regione sta fra l'anno e il nome della serie.
		{"2018 - Emilia-Romagna - SIOPE Movimenti mensili delle disponibilità liquide", "2018", "2018", "Emilia-Romagna", "SIOPE Movimenti mensili delle disponibilità liquide"},
		{"2025/08 - Sicilia - SIOPE Movimenti cumulati mensili di Spesa", "2025", "2025/08", "Sicilia", "SIOPE Movimenti cumulati mensili di Spesa"},
		{"Progetti Opere Pubbliche MOP - Sicilia", "", "", "Sicilia", "Progetti Opere Pubbliche MOP"},
		{"Progetti Opere Pubbliche MOP - Totale", "", "", "Totale", "Progetti Opere Pubbliche MOP"},
		{"Gare Opere Pubbliche MOP - Valle d'Aosta", "", "", "Valle d'Aosta", "Gare Opere Pubbliche MOP"},
	}
	for _, c := range casi {
		anno, periodo, regione, serie := derivaSerie(c.titolo)
		if anno != c.anno || periodo != c.periodo || regione != c.regione || serie != c.serie {
			t.Errorf("derivaSerie(%q) = (%q,%q,%q,%q), atteso (%q,%q,%q,%q)",
				c.titolo, anno, periodo, regione, serie, c.anno, c.periodo, c.regione, c.serie)
		}
	}
}

func TestSlugTitolo(t *testing.T) {
	casi := map[string]string{
		"Progetti Opere Pubbliche MOP - Totale":    "progetti-opere-pubbliche-mop-totale",
		"2025/08 - Pagamenti Bilancio dello Stato": "2025-08-pagamenti-bilancio-dello-stato",
		"": "",
	}
	for titolo, atteso := range casi {
		if got := slugTitolo(titolo); got != atteso {
			t.Errorf("slugTitolo(%q) = %q, atteso %q", titolo, got, atteso)
		}
	}
}

func TestDerivaDatasetRisorse(t *testing.T) {
	raw := map[string]any{
		"id":                "c76e90f7-eea5-4f32-8767-6b60e3505a1d",
		"name":              "spd_mop_gar_mon_reg19_01_9999",
		"title":             "Gare Opere Pubbliche MOP - Sicilia",
		"notes":             " descrizione ",
		"license_id":        "cc-by",
		"metadata_modified": "2026-09-04T12:20:22.000000",
		"groups":            []any{map[string]any{"name": "172_opere-pubbliche"}},
		"tags":              []any{map[string]any{"name": "Opere Pubbliche"}},
		"resources": []any{
			map[string]any{"resource_type": "OData", "id": "49a5930b-d46b-446b-907c-c14988f9c676"},
			map[string]any{"url": "http://bdap-opendata.rgs.mef.gov.it/SpodCkanApi/api/3/datastore/dump/c76e90f7-eea5-4f32-8767-6b60e3505a1d.csv"},
		},
	}
	d := derivaDataset(raw)
	if d.ODataID != "49a5930b-d46b-446b-907c-c14988f9c676" {
		t.Errorf("identificativo OData = %q", d.ODataID)
	}
	if d.Famiglia != "gare" {
		t.Errorf("famiglia = %q, attesa gare", d.Famiglia)
	}
	if d.Regione != "Sicilia" {
		t.Errorf("regione = %q, attesa Sicilia", d.Regione)
	}
	if d.Note != "descrizione" {
		t.Errorf("note = %q", d.Note)
	}
	// Il CSV si costruisce sempre in https: l'indirizzo http pubblicato non risponde.
	atteso := baseSito + pathDumpPrefix + d.ID + ".csv"
	if d.CSVURL != atteso {
		t.Errorf("csv = %q, atteso %q", d.CSVURL, atteso)
	}
}

func TestTrovaDatasetETrovaMOP(t *testing.T) {
	elenco := []dataset{
		{ID: "uuid-1", Nome: "spd_mop_sal_mon_reg19_01_9999", Titolo: "Pagamenti Opere Pubbliche MOP - Sicilia", ODataID: "od-1", Regione: "Sicilia", Famiglia: "pagamenti"},
		{ID: "uuid-2", Nome: "spd_mop_gar_mon_reg01_01_9999", Titolo: "Gare Opere Pubbliche MOP - Piemonte", ODataID: "od-2", Regione: "Piemonte", Famiglia: "gare"},
	}
	if _, ok := trovaDataset(elenco, "od-2"); !ok {
		t.Error("il dataset non e' stato risolto per identificativo OData")
	}
	if _, ok := trovaDataset(elenco, "Pagamenti Opere Pubbliche MOP - Sicilia"); !ok {
		t.Error("il dataset non e' stato risolto per titolo")
	}
	if _, ok := trovaDataset(elenco, "inesistente"); ok {
		t.Error("una chiave inesistente non deve risolvere")
	}
	if got := datasetMOP(elenco, "gare", ""); len(got) != 1 || got[0].ID != "uuid-2" {
		t.Errorf("datasetMOP(gare) = %v", got)
	}
	if got := datasetMOP(elenco, "", "Sicilia"); len(got) != 1 || got[0].ID != "uuid-1" {
		t.Errorf("datasetMOP(Sicilia) = %v", got)
	}
	if got := datasetMOP(elenco, "gare", "Sicilia"); len(got) != 0 {
		t.Errorf("nessun dataset atteso, ottenuti %v", got)
	}
}

// Il portale scrive la stessa regione in due modi: il riconoscimento deve
// ignorare le maiuscole e restituire sempre la forma canonica, altrimenti la
// stessa serie si spezza in due gruppi.
func TestDerivaSerieRegioneMaiuscole(t *testing.T) {
	casi := []string{
		"2024 - Valle D'Aosta - SIOPE Movimenti cumulati mensili di Spesa",
		"2024 - Valle d'Aosta - SIOPE Movimenti cumulati mensili di Spesa",
	}
	for _, titolo := range casi {
		_, _, regione, serie := derivaSerie(titolo)
		if regione != "Valle d'Aosta" {
			t.Errorf("derivaSerie(%q) regione = %q", titolo, regione)
		}
		if serie != "SIOPE Movimenti cumulati mensili di Spesa" {
			t.Errorf("derivaSerie(%q) serie = %q", titolo, serie)
		}
	}
}
