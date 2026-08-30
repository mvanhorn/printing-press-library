package gaclient

import "testing"

func TestNormalizeSedeQuota(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "", want: SedeQuotaProporzionale},
		{in: "proporzionale", want: SedeQuotaProporzionale},
		{in: "  UGUALE ", want: SedeQuotaUguale},
		{in: "equal", wantErr: true},
		{in: "proporzionali", wantErr: true},
	} {
		got, err := normalizeSedeQuota(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("normalizeSedeQuota(%q): atteso errore, nessuno", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("normalizeSedeQuota(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("normalizeSedeQuota(%q) = %q, atteso %q", tc.in, got, tc.want)
		}
	}
}

// I numeri sono quelli veri di "appalto" 2026, misurati sul portale: e' il caso
// che ha motivato il cambio di semantica.
func TestAllocaProporzionaleRispecchiaLaDistribuzioneReale(t *testing.T) {
	totali := map[string]int{
		"ROMA": 373, "CONSIGLIO DI STATO": 339, "NAPOLI": 282, "MILANO": 141,
		"VENEZIA": 93, "PERUGIA": 13, "PESCARA": 3, "AOSTA": 1,
	}
	ordine := []string{"ROMA", "CONSIGLIO DI STATO", "NAPOLI", "MILANO", "VENEZIA", "PERUGIA", "PESCARA", "AOSTA"}

	got := allocaProporzionale(totali, ordine, 100)

	somma := 0
	for _, n := range got {
		somma += n
	}
	if somma != 100 {
		t.Fatalf("le quote sommano a %d, atteso esattamente il limite 100: %v", somma, got)
	}
	// Roma vale il 29% di questo sottoinsieme (373/1245): con quota uguale
	// avrebbe avuto 12 posti su 8 sedi, qui deve averne molti di piu'.
	if got["ROMA"] < 25 {
		t.Errorf("ROMA ha quota %d, troppo bassa per 373 provvedimenti su 1245", got["ROMA"])
	}
	// Aosta ne ha uno solo: non puo' riceverne di piu' di quanti ne esistano.
	if got["AOSTA"] > 1 {
		t.Errorf("AOSTA ha quota %d ma solo 1 provvedimento", got["AOSTA"])
	}
	// L'ordine delle quote deve seguire l'ordine dei totali.
	if got["ROMA"] < got["CONSIGLIO DI STATO"] || got["CONSIGLIO DI STATO"] < got["NAPOLI"] {
		t.Errorf("le quote non seguono i totali: %v", got)
	}
}

func TestAllocaProporzionaleNonSuperaMaiIlTotaleDiUnaSede(t *testing.T) {
	// Limite piu' alto della somma: ogni sede deve fermarsi al proprio totale
	// invece di ricevere posti per provvedimenti che non esistono.
	totali := map[string]int{"A": 3, "B": 2}
	ordine := []string{"A", "B"}
	got := allocaProporzionale(totali, ordine, 100)
	if got["A"] != 3 || got["B"] != 2 {
		t.Errorf("atteso A=3 B=2 (i totali reali), ottenuto %v", got)
	}
}

func TestAllocaProporzionaleCasiLimite(t *testing.T) {
	ordine := []string{"A", "B"}
	if got := allocaProporzionale(map[string]int{"A": 5}, ordine, 0); len(got) != 0 {
		t.Errorf("limit 0 deve dare quote vuote, ottenuto %v", got)
	}
	if got := allocaProporzionale(map[string]int{"A": 0, "B": 0}, ordine, 10); len(got) != 0 {
		t.Errorf("totali a zero devono dare quote vuote, ottenuto %v", got)
	}
	if got := allocaProporzionale(map[string]int{}, nil, 10); len(got) != 0 {
		t.Errorf("nessuna sede deve dare quote vuote, ottenuto %v", got)
	}
	// Sedi minuscole e limite basso: chi non arriva a un posto resta fuori, ma
	// il limite va speso comunque.
	got := allocaProporzionale(map[string]int{"GRANDE": 999, "PICCOLA": 1}, []string{"GRANDE", "PICCOLA"}, 2)
	somma := got["GRANDE"] + got["PICCOLA"]
	if somma != 2 {
		t.Errorf("le quote sommano a %d, atteso 2: %v", somma, got)
	}
	if got["GRANDE"] < got["PICCOLA"] {
		t.Errorf("con 999 contro 1, GRANDE non puo' avere meno di PICCOLA: %v", got)
	}
}

func TestAllocaProporzionaleEDeterministica(t *testing.T) {
	// Resti identici fra sedi diverse: senza un criterio di parita' stabile,
	// due esecuzioni della stessa ricerca restituirebbero campioni diversi.
	totali := map[string]int{"A": 10, "B": 10, "C": 10}
	ordine := []string{"A", "B", "C"}
	primo := allocaProporzionale(totali, ordine, 10)
	for i := 0; i < 20; i++ {
		got := allocaProporzionale(totali, ordine, 10)
		for _, s := range ordine {
			if got[s] != primo[s] {
				t.Fatalf("esecuzione %d diversa dalla prima: %v contro %v", i, got, primo)
			}
		}
	}
}
