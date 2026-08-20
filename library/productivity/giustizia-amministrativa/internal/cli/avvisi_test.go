package cli

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/giustizia-amministrativa/internal/gaclient"
)

func TestNotaDataDepositoAssente(t *testing.T) {
	// Il portale non espone la data di deposito nell'indice di ricerca:
	// verificato sull'HTML reale dei risultati, dove non compare in nessun
	// formato. Senza dirlo, il campo vuoto si legge come "questo provvedimento
	// non ha data" invece che come un limite dell'endpoint.
	vuoti := []gaclient.Provvedimento{{Ecli: "ECLI:A"}, {Ecli: "ECLI:B"}}
	nota := notaDataDepositoAssente(vuoti)
	if nota == "" {
		t.Fatal("con tutte le righe senza data la nota deve esserci")
	}
	if !strings.Contains(nota, "get") || !strings.Contains(nota, "corpus build") {
		t.Errorf("la nota deve dire da dove si ottiene la data, ottenuto: %q", nota)
	}

	pieni := []gaclient.Provvedimento{{Ecli: "ECLI:A", DataDeposito: "14/07/2026"}}
	if nota := notaDataDepositoAssente(pieni); nota != "" {
		t.Errorf("con la data presente la nota non serve, ottenuto: %q", nota)
	}

	if nota := notaDataDepositoAssente(nil); nota != "" {
		t.Errorf("senza risultati non c'e' nulla da segnalare, ottenuto: %q", nota)
	}
}

func TestAvvisoGruppoTroncatoPerSede(t *testing.T) {
	// Con --by sede la troncatura non rimpicciolisce la distribuzione: la
	// deforma. Il portale ordina per sede, quindi le prime si riempiono e le
	// ultime mancano del tutto — misurato, Roma 29 contro le 65 reali.
	perSede := avvisoGruppoTroncato(119, 167, "BOLOGNA", []string{"sede"})
	if !strings.Contains(perSede, "--sede-sweep") {
		t.Errorf("con --by sede l'avviso deve indicare --sede-sweep, ottenuto: %q", perSede)
	}

	// Su piu' dimensioni lo sweep non cambia il modo di contare: stats legge i
	// totali dichiarati solo per --by sede esatto, quindi prometterlo su
	// sede,anno manderebbe a rifare la query per gli stessi numeri.
	multi := avvisoGruppoTroncato(119, 167, "ROMA/2026", []string{"sede", "anno"})
	if strings.Contains(multi, "--sede-sweep") {
		t.Errorf("su --by sede,anno lo sweep non va promesso, ottenuto: %q", multi)
	}

	perTipo := avvisoGruppoTroncato(119, 167, "Sentenza", []string{"tipo"})
	if strings.Contains(perTipo, "--sede-sweep") {
		t.Errorf("su altre dimensioni --sede-sweep non c'entra, ottenuto: %q", perTipo)
	}
	if !strings.Contains(perTipo, "Sentenza") || !strings.Contains(perTipo, "167") {
		t.Errorf("l'avviso deve nominare il gruppo tagliato e il totale, ottenuto: %q", perTipo)
	}
}
