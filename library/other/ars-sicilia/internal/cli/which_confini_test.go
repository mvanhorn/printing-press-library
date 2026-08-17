package cli

import (
	"strings"
	"testing"
)

func lower(s string) string { return strings.ToLower(s) }

func TestWhichNonAggancaInMezzoAllaParola(t *testing.T) {
	// «chi» prendeva 2 punti da «ri*chi*esti» nella descrizione di
	// `commissione dossier` e 1 da «cross-ar*chi*vio» nel gruppo. Ora aggancia
	// solo dove «chi» è una parola vera — le classifiche di persone, la cui
	// descrizione comincia con «Chi parla di più», «Chi propone più leggi» —
	// e mai un comando che con le persone non c'entra.
	for _, m := range rankWhich(whichIndex, "chi", 5) {
		if !contienePrefissoDiParola(lower(m.Entry.Description)+" "+lower(m.Entry.Group), "chi") {
			t.Errorf("«chi» aggancia %q senza che la parola compaia: %s", m.Entry.Command, m.Entry.Description)
		}
	}
	if contienePrefissoDiParola("pareri richiesti al governo, archivio 226", "chi") {
		t.Error("«chi» entra ancora dentro «richiesti»/«archivio»")
	}
}

func TestWhichRispondeAlleDomandeOrdinarie(t *testing.T) {
	// Le domande poste come le pone una persona, con il comando che deve
	// uscire primo. Tre di queste — rifiuti in aula, novità, chi parla di più
	// — prima non agganciavano nulla: le capacità c'erano, la voce d'indice
	// no.
	casi := []struct{ query, want string }{
		{"chi ha parlato di rifiuti in aula", "resoconti cerca"},
		{"chi parla di piu in aula", "analytics --type resoconti --group-by oratore"},
		{"novita degli ultimi sette giorni", "novita --since 7d"},
		{"atti nuovi di questa settimana", "novita --since 7d"},
		{"cerca una interrogazione per numero", "interrogazioni cerca"},
		{"cosa si discute in commissione la prossima settimana", "commissioni convocazioni"},
		{"a che punto e' il disegno di legge sulla sanita", "legge cronologia"},
		{"quanto e' aggiornato il portale", "sync coverage"},
		{"quali materie esistono per classificare i ddl", "ddl materie"},
	}
	for _, c := range casi {
		got := rankWhich(whichIndex, c.query, 3)
		if len(got) == 0 {
			t.Errorf("%q non aggancia nulla", c.query)
			continue
		}
		if got[0].Entry.Command != c.want {
			t.Errorf("%q → %q, atteso %q (tutti: %v)", c.query, got[0].Entry.Command, c.want, primiComandi(got))
		}
	}
}

func TestWhichTaceSuCioCheNonEsiste(t *testing.T) {
	// Le domande a cui la risposta è «il portale non lo pubblica» non devono
	// pescare un comando plausibile: la risposta sta in non_coperto.
	for _, q := range []string{"quanto spende l'assemblea", "quanto costa un vitalizio"} {
		if got := rankWhich(whichIndex, q, 3); len(got) != 0 {
			t.Errorf("%q aggancia %v, e non dovrebbe", q, primiComandi(got))
		}
	}
}

func TestWhichSpiegaCioCheLaFonteNonPubblica(t *testing.T) {
	casi := map[string]string{
		"come ha votato un deputato in aula":               "voti nominali",
		"chi era assente alla seduta":                      "presenze",
		"quali emendamenti sono stati presentati a un ddl": "emendamenti",
		"quanto spende l'assemblea":                        "spese",
	}
	for q := range casi {
		if got := assentiPerQuery(q); len(got) == 0 {
			t.Errorf("%q non produce nessuna spiegazione di non-copertura", q)
		}
	}
	// E una domanda coperta non deve inventarsi una non-copertura.
	if got := assentiPerQuery("cerca una interrogazione per numero"); len(got) != 0 {
		t.Errorf("non-copertura inventata su una domanda coperta: %v", got)
	}
}

func TestWhichIgnoraLeParoleVuote(t *testing.T) {
	// «chi ha parlato di rifiuti in aula» pescava `sync` e `search` per via
	// del «in» dentro il gruppo «Dati in locale».
	got := rankWhich(whichIndex, "chi ha parlato di rifiuti in aula", 10)
	for _, m := range got {
		if m.Entry.Command == "sync" || m.Entry.Command == "search" {
			t.Errorf("una parola vuota aggancia ancora %q", m.Entry.Command)
		}
	}
	if len(tokenUtili([]string{"quali", "emendamenti", "sono", "stati", "presentati"})) != 2 {
		t.Errorf("tokenUtili non filtra gli ausiliari: %v", tokenUtili([]string{"quali", "emendamenti", "sono", "stati", "presentati"}))
	}
}

func TestWhichReggeLaMorfologiaItaliana(t *testing.T) {
	// L'indice è al singolare, le domande arrivano al plurale.
	if !contieneParole("cronologia completa di un disegno di legge", "disegni di legge") {
		t.Error("«disegni di legge» non aggancia più «disegno di legge»")
	}
	if !contienePrefissoDiParola("lavori di commissione", "commissioni") {
		t.Error("«commissioni» non aggancia più «commissione»")
	}
	if contieneParole("cronologia completa di un disegno di legge", "disegni di regolamento") {
		t.Error("una sequenza che nel testo non c'è viene agganciata lo stesso")
	}
}

func TestWhichIndicePuntaSoloAComandiVeri(t *testing.T) {
	// Il patto dell'indice: ogni Command è invocabile così com'è scritto.
	// Qui si controlla la forma — nome non vuoto, descrizione non vuota,
	// nessun duplicato — perché un doppione significa due voci che si
	// contendono la stessa domanda.
	visti := map[string]bool{}
	for i, e := range whichIndex {
		if e.Command == "" || e.Description == "" {
			t.Errorf("whichIndex[%d] incompleto: %+v", i, e)
		}
		if visti[e.Command] {
			t.Errorf("whichIndex[%d]: comando duplicato %q", i, e.Command)
		}
		visti[e.Command] = true
	}
	if len(whichIndex) < 20 {
		t.Errorf("l'indice è tornato alle sole hero feature: %d voci", len(whichIndex))
	}
}

func primiComandi(m []whichMatch) []string {
	out := make([]string, 0, len(m))
	for _, x := range m {
		out = append(out, x.Entry.Command)
	}
	return out
}
