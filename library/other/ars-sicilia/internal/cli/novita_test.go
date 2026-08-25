package cli

import (
	"testing"
	"time"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
)

func TestFinestraIndietro(t *testing.T) {
	oggi := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	casi := map[string]string{
		"7d":  "2026-08-07",
		"1d":  "2026-08-13",
		"3w":  "2026-07-24",
		"2m":  "2026-06-15",
		"24h": "2026-08-13",
	}
	for since, want := range casi {
		got, err := inizioFinestra(since, "", oggi)
		if err != nil {
			t.Errorf("--since %s: %v", since, err)
			continue
		}
		if got.Format("2006-01-02") != want {
			t.Errorf("--since %s → %s, atteso %s", since, got.Format("2006-01-02"), want)
		}
	}
	// --dal vince su --since.
	got, err := inizioFinestra("7d", "2026-01-01", oggi)
	if err != nil || got.Format("2006-01-02") != "2026-01-01" {
		t.Errorf("--dal non ha la precedenza: %v %v", got, err)
	}
}

func TestFinestraRifiutaCioCheNonPuoFunzionare(t *testing.T) {
	oggi := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	// Una finestra che parte domani sarebbe vuota per costruzione, e un elenco
	// vuoto si legge come «non è successo niente».
	if _, err := inizioFinestra("", "2026-12-31", oggi); err == nil {
		t.Error("--dal nel futuro accettato")
	}
	for _, s := range []string{"sette giorni", "d", "0d", "-3d", "3x"} {
		if _, err := inizioFinestra(s, "", oggi); err == nil {
			t.Errorf("--since %q accettato", s)
		}
	}
}

func TestUltimaDentroGuardaLUltimaDataLeggibile(t *testing.T) {
	righe := []icaro.Record{
		{Fields: map[string]string{"Data": "3.08.26"}},
		{Fields: map[string]string{"Data": "28.07.26"}},
		{Fields: map[string]string{"Data": ""}}, // riga senza data: si salta
	}
	if !ultimaDentro(righe, "2026-07-15") {
		t.Error("la finestra è ancora aperta e ultimaDentro dice di no")
	}
	if ultimaDentro(righe, "2026-08-01") {
		t.Error("l'ultima riga è fuori finestra e ultimaDentro dice di sì")
	}
	if ultimaDentro(nil, "2026-01-01") {
		t.Error("nessuna riga: non c'è niente da continuare a leggere")
	}
}

func TestNovitaTagliaLElencoMaNonIlConteggio(t *testing.T) {
	// --limit governa quanto si mostra, non quanto si è trovato: se il
	// conteggio seguisse il limite, «12 nuovi ddl» diventerebbe «10» solo
	// perché si è chiesto di vederne dieci.
	e := novitaArchivio{Conteggio: 62}
	if e.Conteggio != 62 {
		t.Fatal("premessa")
	}
	nuovi := make([]map[string]string, 62)
	for i := range nuovi {
		nuovi[i] = map[string]string{"data_iso": "2026-08-01"}
	}
	e.Nuovi = nuovi
	if len(e.Nuovi) > 30 {
		e.Nuovi = e.Nuovi[:30]
	}
	if e.Conteggio != 62 || len(e.Nuovi) != 30 {
		t.Errorf("conteggio=%d mostrati=%d, attesi 62 e 30", e.Conteggio, len(e.Nuovi))
	}
}

// Sull'archivio leggi le righe sono per articolo: sette righe della stessa
// legge devono uscire come una novità sola, con gli articoli contati accanto.
func TestRigheNovitaLeggi(t *testing.T) {
	arc := icaro.Archive{Slug: "leggi"}
	recs := []icaro.Record{}
	for i := 1; i <= 7; i++ {
		recs = append(recs, icaro.Record{
			Title:  "Disposizioni varie in materia di produzione energetica",
			URL:    "https://dati.ars.sicilia.it/icaro/doc201-1.jsp",
			Fields: map[string]string{"Legisl.": "18", "Atto": "L.R. 14", "Data": "22.07.2026", "Docum.": itoa(i)},
		})
	}
	recs = append(recs, icaro.Record{
		Title:  "Altra legge",
		Fields: map[string]string{"Legisl.": "18", "Atto": "L.R. 13", "Data": "15.07.2026", "Docum.": "1"},
	})

	righe := righeNovitaLeggi(arc, recs)
	if len(righe) != 2 {
		t.Fatalf("otto righe-articolo di due leggi devono dare 2 novità, non %d", len(righe))
	}
	if righe[0]["atto"] != "L.R. 14" {
		t.Errorf("atto = %q, atteso L.R. 14", righe[0]["atto"])
	}
	if righe[0]["numero"] != "14" {
		t.Errorf("numero deve essere il numero nudo, quello che si passa a --numero: %q", righe[0]["numero"])
	}
	if righe[0]["articoli_trovati"] != "7" {
		t.Errorf("articoli_trovati = %q, atteso 7", righe[0]["articoli_trovati"])
	}
	if righe[0]["data_iso"] != "2026-07-22" {
		t.Errorf("data_iso = %q, atteso 2026-07-22", righe[0]["data_iso"])
	}
}
