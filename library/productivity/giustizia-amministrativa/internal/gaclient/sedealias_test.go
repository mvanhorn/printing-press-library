package gaclient

import "testing"

func TestResolveSedeAccettaLeGrafieDelPortale(t *testing.T) {
	cases := map[string]string{
		"TARLAZ":            "Roma", // il codice dentro l'ECLI che emettiamo noi
		"tarmi":             "Milano",
		"TRGABZ":            "Bolzano",
		"tar_lazio":         "Roma", // nome della regione
		"TAR Sicilia":       "Palermo",
		"sicilia-catania":   "Catania", // sede staccata
		"lombardia_brescia": "Brescia",
		"TARLOM":            "Milano",   // codice del portale pre-2026
		"l'aquila":          "L'Aquila", // grafia con apostrofo
		"Reggio Calabria":   "Reggio Calabria",
		"roma":              "Roma",
		"cds":               "Consiglio di Stato",
	}
	for in, want := range cases {
		if !validSede(in) {
			t.Errorf("%q rifiutata", in)
			continue
		}
		if got := mapSede(in); got != want {
			t.Errorf("%q -> %q, atteso %q", in, got, want)
		}
	}
	// TARBOL e TARABR restano fuori: come abbreviazioni puntano a Bologna e
	// all'Aquila, e una sede sbagliata in silenzio e' peggio di un rifiuto.
	for _, in := range []string{"TARBOL", "TARABR", "pippo", "tar-atlantide"} {
		if validSede(in) {
			t.Errorf("%q accettata: %q", in, mapSede(in))
		}
	}
}

func TestSedeAliasWarningSoloPerLeRegioniConDueSedi(t *testing.T) {
	if nota := sedeAliasWarning("sicilia"); nota == "" {
		t.Error("nessun avviso per la regione con sede staccata")
	}
	for _, in := range []string{"palermo", "TARPA", "veneto", "toscana", ""} {
		if nota := sedeAliasWarning(in); nota != "" {
			t.Errorf("%q: avviso non atteso: %s", in, nota)
		}
	}
}
