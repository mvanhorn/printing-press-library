package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/giustizia-amministrativa/internal/gaclient"
)

// notaDataDepositoAssente spiega un campo che nei risultati di ricerca e'
// sempre vuoto.
//
// Il portale non pubblica la data di deposito nell'indice di ricerca: nel
// blocco HTML di ciascun risultato non compare in alcun formato, e la CLI la
// ricava solo dal documento (ExtractDataDeposito). Un campo vuoto senza
// spiegazione si legge come "questo provvedimento non ha data" invece che come
// un limite dell'endpoint, e chi vuole ordinare per data finisce a scaricare un
// documento intero per riga per scoprire che bastava chiederlo altrove.
func notaDataDepositoAssente(items []gaclient.Provvedimento) string {
	for _, p := range items {
		if p.DataDeposito != "" {
			return ""
		}
	}
	if len(items) == 0 {
		return ""
	}
	return "la data di deposito non compare nei risultati di ricerca: il portale non la espone a questo endpoint. Si ottiene con `get` sul singolo provvedimento o con `corpus build`, che la scrivono nel front-matter e nel manifest."
}

// avvisoGruppoTroncato descrive un campione che si e' fermato prima del totale
// dichiarato.
//
// Con --by sede la troncatura non rimpicciolisce la distribuzione: la deforma.
// Il portale ordina i risultati per sede, quindi le prime si riempiono e le
// ultime mancano del tutto — misurato su "accesso civico generalizzato" 2026,
// Roma risulta 29 contro le 65 dichiarate. Su altre dimensioni lo sweep non
// c'entra e nominarlo sarebbe rumore.
func avvisoGruppoTroncato(campione, totale int, gruppo string, dims []string) string {
	msg := fmt.Sprintf(
		"il campione (%d) si e' esaurito prima del totale dichiarato (%d): il gruppo %q e' tagliato a meta' e i gruppi successivi nell'ordine del portale mancano del tutto. I numeri qui sopra non sono una distribuzione completa. Alza --limit oppure interroga un gruppo per volta.",
		campione, totale, gruppo)
	// Solo per --by sede esatto: stats legge i totali dichiarati per sede
	// unicamente su quella dimensione singola (vedi TotalsBySede in stats.go).
	// Su --by sede,anno lo sweep non cambia il modo di contare, e prometterlo
	// manderebbe il lettore a rifare la query per ottenere gli stessi numeri.
	if len(dims) == 1 && strings.EqualFold(dims[0], "sede") {
		msg += " Raggruppando per sede il taglio non e' solo parziale ma distorto, perche' l'ordine del portale e' proprio quello delle sedi: --sede-sweep legge il totale dichiarato da ciascuna invece di contare le righe del campione."
	}
	return msg
}
