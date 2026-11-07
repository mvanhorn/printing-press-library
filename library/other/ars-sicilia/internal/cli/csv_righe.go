// pp:helper
// Estrazione delle righe da mettere in CSV quando il payload non è un array
// in radice ma un oggetto che lo avvolge.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
)

// righeDaOggetto trova, dentro un payload che non è un array in radice, le
// righe che il CSV deve rendere. Torna nil quando non ce n'è una sola
// evidente: meglio non rendere niente che rendere la lista sbagliata.
//
// I quattro comandi aggregati — `ddl iter`, `legge cronologia`,
// `deputato profilo`, `commissione dossier` — restituiscono un oggetto che
// avvolge la lista, quindi con --csv finivano nel ramo «oggetto singolo» di
// printCSV, che stampa il JSON. Il formato chiesto non arrivava e nessuno lo
// diceva: sono esattamente i comandi che si esportano per analizzarli altrove.
//
// Due forme, ed entrambe si riconoscono senza sapere quale comando ha
// prodotto il payload:
//
//	{atti: [ {...}, {...} ], conteggio: {...}}          una sola lista di oggetti
//	{sezioni: [ {tipo: "sommari", risultati: [...] } ]} liste annidate di un livello
//
// Nel secondo caso le righe delle sotto-liste vengono concatenate e i campi
// scalari del contenitore (`tipo`, `archivio`) diventano colonne su ogni riga,
// che è l'unico modo di non perdere a quale sezione apparteneva una riga.
// Una sezione con la lista vuota non dà righe e torna in `vuote`.
func righeDaOggetto(data json.RawMessage) (righeTabella, vuote []map[string]any) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil || obj == nil {
		return nil, nil
	}
	nome, righe := unicaListaDiOggetti(obj)
	if nome == "" {
		return nil, nil
	}

	// Livello annidato: ogni riga avvolge a sua volta una sola lista. Il nome
	// della lista viene dalle righe che ne hanno una piena: le altre devono
	// averla vuota sotto lo stesso nome, altrimenti la forma non è questa.
	interni := make([]map[string]json.RawMessage, len(righe))
	sottoNome := ""
	for i, r := range righe {
		grezzo, err := json.Marshal(r)
		if err != nil || json.Unmarshal(grezzo, &interni[i]) != nil {
			return righe, nil
		}
		n, _ := unicaListaDiOggetti(interni[i])
		if n == "" {
			continue
		}
		if sottoNome != "" && n != sottoNome {
			return righe, nil
		}
		sottoNome = n
	}
	if sottoNome == "" {
		return righe, nil
	}

	annidate := make([]map[string]any, 0, len(righe))
	for i, r := range righe {
		var sotto []map[string]any
		if err := json.Unmarshal(interni[i][sottoNome], &sotto); err != nil {
			return righe, nil
		}
		if n, _ := unicaListaDiOggetti(interni[i]); n == "" && len(sotto) > 0 {
			return righe, nil // più di una lista piena in questa riga
		}
		contesto := map[string]any{}
		for k, v := range r {
			if k == sottoNome {
				continue
			}
			if _, lista := v.([]any); lista {
				continue
			}
			if _, mappa := v.(map[string]any); mappa {
				continue
			}
			contesto[k] = v
		}
		if len(sotto) == 0 {
			vuote = append(vuote, contesto)
			continue
		}
		for _, s := range sotto {
			riga := map[string]any{}
			for k, v := range contesto {
				riga[k] = v
			}
			for k, v := range s {
				riga[k] = v
			}
			annidate = append(annidate, riga)
		}
	}
	return annidate, vuote
}

// avvisiSezioniVuote dice quali sezioni non hanno portato righe nella tabella,
// con il loro `errore` o la loro `nota` se ce l'hanno: `novita` ci scrive perché
// un archivio non è databile o non è stato letto, e nel CSV non c'è una riga su
// cui metterlo.
func avvisiSezioniVuote(vuote []map[string]any) []string {
	var out []string
	for _, c := range vuote {
		nome, _ := c["tipo"].(string)
		if nome == "" {
			nome, _ = c["archivio"].(string)
		}
		if nome == "" {
			nome = "una sezione"
		}
		testo := fmt.Sprintf("hint: --csv: %s non ha righe", nome)
		if errore, _ := c["errore"].(string); errore != "" {
			// Una richiesta fallita non è un archivio senza novità.
			testo = fmt.Sprintf("hint: --csv: %s non è stato letto: %s", nome, errore)
		} else if nota, _ := c["nota"].(string); nota != "" {
			testo += ": " + nota
		}
		out = append(out, testo)
	}
	return out
}

// unicaListaDiOggetti torna il nome e il contenuto dell'unica chiave che porta
// una lista di oggetti. Se le liste di oggetti sono zero o più d'una, torna
// nome vuoto: quale sia «quella giusta» non è deducibile, e sceglierne una a
// caso produrrebbe un CSV plausibile e sbagliato. Le liste di scalari
// (`troncato: ["interrogazioni"]`) non contano.
func unicaListaDiOggetti(obj map[string]json.RawMessage) (string, []map[string]any) {
	nomi := make([]string, 0, len(obj))
	for k := range obj {
		nomi = append(nomi, k)
	}
	sort.Strings(nomi) // determinismo: l'ordine delle mappe in Go non lo è

	trovato := ""
	var righe []map[string]any
	for _, k := range nomi {
		var lista []map[string]any
		if err := json.Unmarshal(obj[k], &lista); err != nil || len(lista) == 0 {
			continue
		}
		if trovato != "" {
			return "", nil
		}
		trovato, righe = k, lista
	}
	return trovato, righe
}
