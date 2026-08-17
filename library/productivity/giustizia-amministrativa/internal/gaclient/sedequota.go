package gaclient

import (
	"fmt"
	"sort"
	"strings"
)

// Come una ricerca su tutte le sedi spende --limit.
const (
	// SedeQuotaProporzionale pesa ogni sede per il totale che il portale
	// dichiara: il campione rispecchia dove la giurisprudenza sta davvero.
	SedeQuotaProporzionale = "proporzionale"
	// SedeQuotaUguale da' a ogni sede la stessa fetta. Risponde a una domanda
	// diversa - "esiste qualcosa, da qualche parte, su questo tema?" - e per
	// quella e' la scelta giusta, perche' non lascia fuori le sedi piccole.
	SedeQuotaUguale = "uguale"
)

var sedeQuotaValidi = []string{SedeQuotaProporzionale, SedeQuotaUguale}

// normalizeSedeQuota accetta la stringa dell'utente e restituisce il modo, o un
// errore esplicito: un valore non riconosciuto non viene ignorato in silenzio,
// come per sede e tipo.
func normalizeSedeQuota(raw string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return SedeQuotaProporzionale, nil
	}
	for _, ok := range sedeQuotaValidi {
		if v == ok {
			return v, nil
		}
	}
	return "", fmt.Errorf("sede-quota %q non riconosciuta: usa %s", raw, strings.Join(sedeQuotaValidi, " oppure "))
}

// allocaProporzionale ripartisce limit fra le sedi in proporzione ai totali che
// il portale dichiara, con il metodo dei resti piu' grandi: le quote intere
// prima, poi i posti che avanzano alle sedi con il resto maggiore. Nessuna sede
// riceve piu' provvedimenti di quanti ne abbia, e i posti che si liberano per
// quel tetto vengono ridistribuiti.
//
// Restituisce solo le sedi con quota > 0. Una sede con un totale piccolo che
// non arriva a un posto resta fuori, ed e' il comportamento voluto: con
// --limit 100 su 2187 provvedimenti, una sede che ne ha uno vale lo 0,05% e non
// merita l'1% del campione.
func allocaProporzionale(totali map[string]int, ordine []string, limit int) map[string]int {
	out := map[string]int{}
	if limit <= 0 || len(totali) == 0 {
		return out
	}

	somma := 0
	for _, s := range ordine {
		if n := totali[s]; n > 0 {
			somma += n
		}
	}
	if somma == 0 {
		return out
	}

	type resto struct {
		sede string
		frac float64
	}
	var resti []resto
	assegnati := 0

	for _, s := range ordine {
		n := totali[s]
		if n <= 0 {
			continue
		}
		esatto := float64(limit) * float64(n) / float64(somma)
		base := int(esatto)
		if base > n {
			base = n
		}
		out[s] = base
		assegnati += base
		if base < n {
			resti = append(resti, resto{sede: s, frac: esatto - float64(int(esatto))})
		}
	}

	// I posti rimasti vanno alle sedi col resto piu' grande. A parita' di resto
	// decide il totale, poi il nome: l'esito deve essere deterministico, o due
	// esecuzioni identiche restituiscono campioni diversi.
	sort.SliceStable(resti, func(i, j int) bool {
		if resti[i].frac != resti[j].frac {
			return resti[i].frac > resti[j].frac
		}
		if totali[resti[i].sede] != totali[resti[j].sede] {
			return totali[resti[i].sede] > totali[resti[j].sede]
		}
		return resti[i].sede < resti[j].sede
	})

	// Piu' giri: una sede che raggiunge il proprio tetto libera un posto per le
	// altre, quindi non basta una passata sola.
	for assegnati < limit {
		progresso := false
		for _, r := range resti {
			if assegnati >= limit {
				break
			}
			if out[r.sede] >= totali[r.sede] {
				continue
			}
			out[r.sede]++
			assegnati++
			progresso = true
		}
		if !progresso {
			break
		}
	}

	for s, n := range out {
		if n == 0 {
			delete(out, s)
		}
	}
	return out
}
