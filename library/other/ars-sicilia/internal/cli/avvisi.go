// pp:helper
// I campi che dicono quanto è attendibile una risposta non sono campi come gli
// altri: --select non deve poterli togliere.

package cli

import "encoding/json"

// campiAvviso sono le chiavi di radice che qualificano la risposta invece di
// farne parte: dicono che è tagliata, quanti record ha per archivio, da dove
// arriva, cosa c'è da sapere per leggerla.
var campiAvviso = map[string]bool{
	"troncato":  true,
	"conteggio": true,
	"nota":      true,
	// `iterReport` scrive la stessa cosa in inglese (`note`): senza questa
	// riga l'avviso di incoerenza seduta↔data spariva proprio sotto --select,
	// che è il modo in cui l'iter si legge di solito.
	"note": true,
	"hint": true,
	"meta": true,
}

// preservaAvvisi rimette in cima all'output i campi di avviso che --select ha
// tolto.
//
// Il filtro fa la cosa giusta per i dati — il chiamante ha nominato i campi che
// vuole — ma i campi di avviso non sono dati fra cui scegliere: sono ciò che
// dice se i dati sono tutti. Su `deputato profilo … --select tipo,data` il
// payload perdeva `troncato: ["interrogazioni"]` e `conteggio`, cioè proprio
// l'avviso che la risposta copre 46 atti su 84; e --select è il flag che si usa
// per risparmiare contesto, quindi l'avviso spariva esattamente nel modo d'uso
// in cui serve di più.
//
// Agisce solo alla radice: nelle righe di un array `nota` o `hint` sono campi
// come gli altri, e il filtro deve poterli togliere. Se il chiamante ha già
// chiesto un campo di avviso, quello che c'è resta com'è.
func preservaAvvisi(originale, filtrato json.RawMessage) json.RawMessage {
	var orig map[string]json.RawMessage
	if err := json.Unmarshal(originale, &orig); err != nil || orig == nil {
		return filtrato
	}
	var filt map[string]json.RawMessage
	if err := json.Unmarshal(filtrato, &filt); err != nil || filt == nil {
		return filtrato
	}
	cambiato := false
	for k, v := range orig {
		if !campiAvviso[k] {
			continue
		}
		if _, gia := filt[k]; gia {
			continue
		}
		filt[k] = v
		cambiato = true
	}
	if !cambiato {
		return filtrato
	}
	out, err := json.Marshal(filt)
	if err != nil {
		return filtrato
	}
	return out
}
