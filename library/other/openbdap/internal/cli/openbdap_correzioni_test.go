// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestRegioneDallaLocalizzazione(t *testing.T) {
	righe := []map[string]any{
		{"Codice Regione": "19", "Descrizione Regione": "SICILIA"},
	}
	if got := regioneDallaLocalizzazione(righe); got != "Sicilia" {
		t.Fatalf("regione = %q, attesa Sicilia", got)
	}
	if got := regioneDallaLocalizzazione(nil); got != "" {
		t.Fatalf("senza righe la regione deve restare vuota, ottenuto %q", got)
	}
	if got := regioneDallaLocalizzazione([]map[string]any{{"Codice Regione": "99"}}); got != "" {
		t.Fatalf("un codice sconosciuto non deve inventare una regione, ottenuto %q", got)
	}
}

func TestAggiungiCodiceISTAT(t *testing.T) {
	riga := map[string]any{"Codice Provincia": "007", "Codice Comune": "058"}
	aggiungiCodiceISTAT(riga)
	if riga["Codice ISTAT Comune"] != "007058" {
		t.Fatalf("codice ISTAT = %v, atteso 007058", riga["Codice ISTAT Comune"])
	}
	parziale := map[string]any{"Codice Provincia": "007"}
	aggiungiCodiceISTAT(parziale)
	if _, presente := parziale["Codice ISTAT Comune"]; presente {
		t.Fatal("senza il codice comune non si deve scrivere un codice ISTAT monco")
	}
}

func TestFamiglieTroncate(t *testing.T) {
	esiti := []esitoFamiglia{
		{Famiglia: "gare", Righe: make([]map[string]any, 50)},
		{Famiglia: "gare", Righe: make([]map[string]any, 50)},
		{Famiglia: "partecipanti", Righe: make([]map[string]any, 3)},
	}
	got := famiglieTroncate(esiti, 50)
	if len(got) != 1 || got[0] != "gare" {
		t.Fatalf("famiglie troncate = %v, attesa solo gare una volta", got)
	}
	if famiglieTroncate(esiti, 0) != nil {
		t.Fatal("senza limite non c'e' troncamento")
	}
}

func TestLettoreUTF8ConvertLatin1(t *testing.T) {
	// "Citt\xe0" in latin-1.
	dati, err := io.ReadAll(nuovoLettoreUTF8(bytes.NewReader([]byte("Comune;Citt\xe0\n"))))
	if err != nil {
		t.Fatal(err)
	}
	if string(dati) != "Comune;Città\n" {
		t.Fatalf("convertito = %q", string(dati))
	}
}

func TestLettoreUTF8LasciaPassareUTF8(t *testing.T) {
	originale := "Comune;Città\nPalermo;sì\n"
	dati, err := io.ReadAll(nuovoLettoreUTF8(strings.NewReader(originale)))
	if err != nil {
		t.Fatal(err)
	}
	if string(dati) != originale {
		t.Fatalf("un flusso gia' UTF-8 non va toccato: %q", string(dati))
	}
}

func TestLettoreUTF8DestinazioneStretta(t *testing.T) {
	lettore := nuovoLettoreUTF8(bytes.NewReader([]byte("a\xe0b")))
	var fuori bytes.Buffer
	buf := make([]byte, 1)
	for {
		n, err := lettore.Read(buf)
		fuori.Write(buf[:n])
		if err != nil {
			break
		}
	}
	if fuori.String() != "aàb" {
		t.Fatalf("letto a un byte per volta = %q", fuori.String())
	}
}

func TestDichiaraUTF8(t *testing.T) {
	if !dichiaraUTF8("text/csv; charset=UTF-8") {
		t.Fatal("un Content-Type che dichiara UTF-8 va riconosciuto")
	}
	if dichiaraUTF8("text/csv") {
		t.Fatal("senza charset non si deve dare per buono UTF-8")
	}
}

func TestLettoreUTF8SequenzaSulTaglioDelBuffer(t *testing.T) {
	// Il primo carattere non ASCII cade negli ultimi byte del blocco che il
	// lettore sbircia: una sequenza tagliata non e' una sequenza invalida, e
	// non deve far scambiare un flusso UTF-8 per latin-1.
	for _, riempimento := range []int{64*1024 - 1, 64*1024 - 2, 64*1024 - 3} {
		originale := strings.Repeat("a", riempimento) + "àbc\n"
		dati, err := io.ReadAll(nuovoLettoreUTF8(strings.NewReader(originale)))
		if err != nil {
			t.Fatal(err)
		}
		if string(dati) != originale {
			t.Fatalf("riempimento %d: flusso UTF-8 alterato (%d byte invece di %d)", riempimento, len(dati), len(originale))
		}
	}
}

func TestLettoreUTF8PrefissoLatin1CheSembraUTF8(t *testing.T) {
	// "Ã¨" in latin-1 e' una sequenza UTF-8 valida per caso: guardare solo il
	// primo carattere alto direbbe UTF-8. Il resto del blocco no, e il blocco
	// e' quello che decide.
	grezzo := []byte("Comune;\xc3\xa8 e \xe0\n")
	dati, err := io.ReadAll(nuovoLettoreUTF8(bytes.NewReader(grezzo)))
	if err != nil {
		t.Fatal(err)
	}
	if string(dati) != "Comune;Ã¨ e à\n" {
		t.Fatalf("convertito = %q", string(dati))
	}
}

func TestLettoreUTF8CodaTroncataAFineFlusso(t *testing.T) {
	// A fine flusso una sequenza incompleta e' davvero incompleta: va trattata
	// come latin-1, non attesa all'infinito.
	dati, err := io.ReadAll(nuovoLettoreUTF8(bytes.NewReader([]byte("ab\xe0"))))
	if err != nil {
		t.Fatal(err)
	}
	if string(dati) != "abà" {
		t.Fatalf("convertito = %q", string(dati))
	}
}

func TestTagliaRuneIncompleta(t *testing.T) {
	casi := []struct {
		nome   string
		blocco []byte
		atteso int
	}{
		{"ascii", []byte("abc"), 3},
		{"utf8 completo", []byte("ab\xc3\xa0"), 4},
		{"utf8 tagliato a due", []byte("ab\xc3"), 2},
		{"utf8 tagliato a tre", []byte("ab\xe0\xa0"), 2},
		{"byte alto isolato", []byte("ab\x80"), 3},
	}
	for _, c := range casi {
		if got := tagliaRuneIncompleta(c.blocco); got != c.atteso {
			t.Errorf("%s: taglio = %d, atteso %d", c.nome, got, c.atteso)
		}
	}
}
