package cli

import "testing"

// Le stringhe sono riferimenti reali del portale, raccolti il 2026-08-01 sulle
// legislature XVII e XVIII. Sono il contratto del parser: i formati non sono
// uniformi nemmeno dentro la stessa legislatura.
func TestParseStralcioRef(t *testing.T) {
	casi := []struct {
		nome       string
		text       string
		numeroRiga int
		vuoleOK    bool
		vuoleBasi  []int
		vuoleVar   string
		vuoleEtich string
		vuoleAuto  bool
	}{
		// --- XVIII legislatura, famiglia del ddl 1030 ---
		{
			nome: "ordinale romano", text: "ddl n. 1030/A Stralcio IV del 27 gennaio 2026",
			numeroRiga: 6030, vuoleOK: true, vuoleBasi: []int{1030}, vuoleVar: "A",
			vuoleEtich: "Stralcio IV",
		},
		{
			nome: "ordinale con bis su barra", text: "ddl n. 1030/A Stralcio IV bis/BIL 15 aprile 2026",
			numeroRiga: 6031, vuoleOK: true, vuoleBasi: []int{1030}, vuoleVar: "A",
			vuoleEtich: "Stralcio IV bis/BIL",
		},
		{
			nome: "n. senza spazio", text: "ddl n.1030/A Stralcio V bis/BIL del 18 febbraio 2026 XVIII Legislatura",
			numeroRiga: 7031, vuoleOK: true, vuoleBasi: []int{1030}, vuoleVar: "A",
			vuoleEtich: "Stralcio V bis/BIL",
		},
		{
			nome: "senza n.", text: "ddl 1030/A Stralcio I bis del 10 febbraio 2026 XVIII Legislatura",
			numeroRiga: 3031, vuoleOK: true, vuoleBasi: []int{1030}, vuoleVar: "A",
			vuoleEtich: "Stralcio I bis",
		},
		{
			nome: "variante V senza ordinale", text: "ddl 1030/V Stralcio del 27 gennaio 2026 XVIII Legislatura",
			numeroRiga: 7030, vuoleOK: true, vuoleBasi: []int{1030}, vuoleVar: "V",
			vuoleEtich: "Stralcio",
		},
		{
			nome: "minuscolo", text: "ddl 1030/A stralcio III del 27 gennaio 2026 XVIII Legislatura",
			numeroRiga: 5030, vuoleOK: true, vuoleBasi: []int{1030}, vuoleVar: "A",
			vuoleEtich: "stralcio III",
		},
		{
			nome: "qualificatore Comm troncato", text: "ddl n. 1030/A Stralcio VI Comm",
			numeroRiga: 8030, vuoleOK: true, vuoleBasi: []int{1030}, vuoleVar: "A",
			vuoleEtich: "Stralcio VI Comm",
		},
		{
			nome: "barra senza variante", text: "ddl n. 738/ stralcio I del 17 Luglio 2024 XVIII Legislatura",
			numeroRiga: 7381, vuoleOK: true, vuoleBasi: []int{738}, vuoleVar: "",
			vuoleEtich: "stralcio I",
		},
		// --- XVII legislatura: gli stessi dati, formati diversi ---
		{
			nome: "due basi abbinate, ordinale prima", text: "ddl nn. 824-810 - I Stralcio del 15 dicembre 2020 XVII Legislatura",
			numeroRiga: 8241, vuoleOK: true, vuoleBasi: []int{824, 810}, vuoleVar: "",
			vuoleEtich: "I Stralcio",
		},
		{
			nome: "tre qualificatori", text: "ddl n. 733/A Stralcio I COMM ter del 20 maggio 2020 XVII Legislatura",
			numeroRiga: 7332, vuoleOK: true, vuoleBasi: []int{733}, vuoleVar: "A",
			vuoleEtich: "Stralcio I COMM ter",
		},
		{
			nome: "base regolare leg 17", text: "ddl n. 979/A Stralcio I del 29 luglio 2021 XVII Legislatura",
			numeroRiga: 9791, vuoleOK: true, vuoleBasi: []int{979}, vuoleVar: "A",
			vuoleEtich: "Stralcio I",
		},
		// --- autoriferimento: il portale scrive l'id della riga, non la base ---
		{
			nome: "autoriferimento con bis", text: "ddl n. 8931/A stralcio 1 bis del 30 dicembre 2020 XVII Legislatura",
			numeroRiga: 8931, vuoleOK: true, vuoleBasi: nil, vuoleEtich: "stralcio 1 bis",
			vuoleAuto: true,
		},
		{
			nome: "autoriferimento ordinale arabo", text: "ddl n. 8934/A stralcio 4 del 20 gennaio 2021 XVII Legislatura",
			numeroRiga: 8934, vuoleOK: true, vuoleBasi: nil, vuoleEtich: "stralcio 4",
			vuoleAuto: true,
		},
		{
			nome: "autoriferimento senza ordinale", text: "ddl n. 8321 stralcio del 29 settembre 2020 XVII Legislatura",
			numeroRiga: 8321, vuoleOK: true, vuoleBasi: nil, vuoleEtich: "stralcio",
			vuoleAuto: true,
		},
		// --- il campo Riferimenti ha un prefisso prima di "ddl" ---
		{
			nome: "riferimenti completo", text: "XVIII Legislatura Numero 6030 del 27.01.26 ddl n. 1030/A Stralcio IV del 27 gennaio 2026",
			numeroRiga: 6030, vuoleOK: true, vuoleBasi: []int{1030}, vuoleVar: "A",
			vuoleEtich: "Stralcio IV",
		},
		// --- non stralci ---
		{
			nome: "ddl base", text: "ddl n. 1030 del 05 novembre 2025 XVIII Legislatura",
			numeroRiga: 1030, vuoleOK: false,
		},
		{
			nome: "riferimenti del ddl base", text: "XVIII Legislatura Numero 738 del 19.04.24",
			numeroRiga: 738, vuoleOK: false,
		},
	}

	for _, c := range casi {
		t.Run(c.nome, func(t *testing.T) {
			ref, ok := parseStralcioRef(c.text, c.numeroRiga)
			if ok != c.vuoleOK {
				t.Fatalf("ok = %v, atteso %v (testo %q)", ok, c.vuoleOK, c.text)
			}
			if !c.vuoleOK {
				return
			}
			if len(ref.Basi) != len(c.vuoleBasi) {
				t.Fatalf("basi = %+v, attese %v", ref.Basi, c.vuoleBasi)
			}
			for i, b := range ref.Basi {
				if b.Numero != c.vuoleBasi[i] {
					t.Errorf("basi[%d].Numero = %d, atteso %d", i, b.Numero, c.vuoleBasi[i])
				}
				if b.Variante != c.vuoleVar {
					t.Errorf("basi[%d].Variante = %q, attesa %q", i, b.Variante, c.vuoleVar)
				}
			}
			if ref.Etichetta != c.vuoleEtich {
				t.Errorf("etichetta = %q, attesa %q", ref.Etichetta, c.vuoleEtich)
			}
			if ref.Autoriferito != c.vuoleAuto {
				t.Errorf("autoriferito = %v, atteso %v", ref.Autoriferito, c.vuoleAuto)
			}
		})
	}
}

// Senza il numero della riga l'autoriferimento non è riconoscibile: il parser
// deve restituire il numero citato come base, non scartarlo.
func TestParseStralcioRefSenzaNumeroRiga(t *testing.T) {
	ref, ok := parseStralcioRef("ddl n. 8931/A stralcio 1 bis del 30 dicembre 2020", 0)
	if !ok {
		t.Fatal("atteso riconoscimento dello stralcio")
	}
	if len(ref.Basi) != 1 || ref.Basi[0].Numero != 8931 {
		t.Fatalf("basi = %+v, attesa [8931]", ref.Basi)
	}
	if ref.Autoriferito {
		t.Error("autoriferito = true senza numero riga: non è determinabile")
	}
}
