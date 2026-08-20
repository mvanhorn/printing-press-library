package cli

import (
	"bytes"
	"encoding/json"
	"sort"
	"testing"
)

func TestDataISOCopreIQuattroDialettiDellaFonte(t *testing.T) {
	casi := []struct {
		in, want string
	}{
		{"28.07.26", "2026-07-28"},    // Icaro, anno a due cifre
		{"3.08.26", "2026-08-03"},     // Icaro, giorno non zero-padded
		{"5.01.2026", "2026-01-05"},   // archivio leggi, anno a quattro cifre
		{"05/08/2026", "2026-08-05"},  // backend /bd/
		{"17 giu 2026", "2026-06-17"}, // blocco di stato dentro un DDL
		{"17 giugno 2026", "2026-06-17"},
		{" 04/08/2026 ", "2026-08-04"},
		{"8.04.25", "2025-04-08"},
		{"31.12.99", "1999-12-31"}, // perno di secolo
	}
	for _, c := range casi {
		if got := dataISO(c.in); got != c.want {
			t.Errorf("dataISO(%q) = %q, atteso %q", c.in, got, c.want)
		}
	}
}

func TestDataISOTaceSuCioCheNonEUnaData(t *testing.T) {
	// La stringa vuota è il modo di dire "non l'ho saputa leggere". Se qui
	// tornasse l'input grezzo, il confronto lessicografico lo metterebbe in
	// competizione con le date vere ed è esattamente il bug che l'ordinamento
	// del profilo aveva.
	for _, in := range []string{
		"", "   ",
		"2026-06-01:2026-08-14", // il range chiesto, non una data
		"17 luglio 2",           // date dell'archivio pareri, tagliate
		"chissà",
		"1.2.3.4",
		"aa/bb/cccc",
	} {
		if got := dataISO(in); got != "" {
			t.Errorf("dataISO(%q) = %q, atteso vuoto", in, got)
		}
	}
}

func TestOrdinamentoNonFaVincereLeDateDelBackendBD(t *testing.T) {
	// "28/07/2026" confrontato come stringa grezza batte "2026-08-03" perché
	// "28" > "20": nel profilo di un deputato il resoconto finiva in testa a
	// interrogazioni più recenti.
	atti := []struct{ data string }{
		{"27.07.26"},
		{"28/07/2026"}, // resoconto, formato /bd/
		{"3.08.26"},
		{"29.07.26"},
	}
	sort.SliceStable(atti, func(i, j int) bool {
		return chiaveData(atti[i].data) > chiaveData(atti[j].data)
	})
	want := []string{"3.08.26", "29.07.26", "28/07/2026", "27.07.26"}
	for i, w := range want {
		if atti[i].data != w {
			t.Fatalf("posizione %d = %q, atteso %q (ordine ottenuto: %v)", i, atti[i].data, w, atti)
		}
	}
}

func TestIniettaDataISOAffiancaSenzaSostituire(t *testing.T) {
	in := json.RawMessage(`{"eventi":[{"data":"16.06.26","fase":"presentazione"},{"data":"17 giu 2026","fase":"commissione"}],"data":"2026-06-01:2026-08-14"}`)
	var got map[string]any
	if err := json.Unmarshal(iniettaDataISO(in), &got); err != nil {
		t.Fatal(err)
	}
	// Il range in radice non è una data della fonte: niente data_iso.
	if _, c := got["data_iso"]; c {
		t.Errorf("data_iso aggiunto al range di ricerca in radice: %v", got["data_iso"])
	}
	eventi, _ := got["eventi"].([]any)
	if len(eventi) != 2 {
		t.Fatalf("eventi persi: %v", got["eventi"])
	}
	want := []string{"2026-06-16", "2026-06-17"}
	for i, e := range eventi {
		m, _ := e.(map[string]any)
		if m["data_iso"] != want[i] {
			t.Errorf("evento %d: data_iso = %v, atteso %s", i, m["data_iso"], want[i])
		}
		if m["data"] == nil {
			t.Errorf("evento %d: la data originale è stata sostituita invece che affiancata", i)
		}
	}
}

func TestIniettaDataISONonTocca(t *testing.T) {
	// Un data_iso già presente non va riscritto, e un payload senza date
	// deve tornare identico.
	in := json.RawMessage(`[{"data":"5.01.2026","data_iso":"scritto-a-mano"},{"titolo":"senza data"}]`)
	var got []map[string]any
	if err := json.Unmarshal(iniettaDataISO(in), &got); err != nil {
		t.Fatal(err)
	}
	if got[0]["data_iso"] != "scritto-a-mano" {
		t.Errorf("data_iso preesistente sovrascritto: %v", got[0]["data_iso"])
	}
	if _, c := got[1]["data_iso"]; c {
		t.Errorf("data_iso inventato su una riga senza data: %v", got[1])
	}
}

func TestBustaEIniezioneDannoLoStessoDataISO(t *testing.T) {
	// La busta di --envelope esce da writeJSON, non da printOutputWithFlags:
	// senza un'iniezione sua le stesse righe avevano data_iso senza
	// --envelope e non l'avevano con --envelope.
	righe := []map[string]string{{"data": "02/09/2026"}, {"data": "28.07.26"}}
	var buf bytes.Buffer
	if err := emitEnvelope(&buf, righe, true, "", &rootFlags{}); err != nil {
		t.Fatal(err)
	}
	var busta struct {
		Risultati []map[string]string `json:"risultati"`
		Troncato  bool                `json:"troncato"`
	}
	if err := json.Unmarshal(buf.Bytes(), &busta); err != nil {
		t.Fatal(err)
	}
	want := []string{"2026-09-02", "2026-07-28"}
	for i, r := range busta.Risultati {
		if r["data_iso"] != want[i] {
			t.Errorf("riga %d: data_iso = %q, atteso %q", i, r["data_iso"], want[i])
		}
	}
	if !busta.Troncato {
		t.Error("la busta ha perso troncato")
	}
}
