// pp:helper
// Normalizzazione delle date della fonte in ISO, per l'ordinamento e per il
// campo `data_iso` emesso in output.

package cli

import (
	"encoding/json"
	"strings"
)

// dataISO converte in "YYYY-MM-DD" le quattro forme in cui le date arrivano dal
// portale, e torna "" quando non ne riconosce nessuna:
//
//	28.07.26     flusso Icaro, anno a due cifre, giorno non zero-padded
//	5.01.2026    flusso Icaro sull'archivio leggi, anno a quattro cifre
//	05/08/2026   pagine del backend /bd/ (resoconti, sommari, convocazioni)
//	17 giu 2026  blocco di stato dentro il documento di un DDL
//
// Il "" per l'irriconoscibile non è un dettaglio: il confronto lessicografico
// fra una data normalizzata e una stringa grezza dà l'ordine sbagliato senza
// dirlo — "28/07/2026" batte "2026-08-03" perché "28" > "20", ed è così che nel
// profilo di un deputato un resoconto finiva in testa a interrogazioni più
// recenti. Chi ordina deve poter distinguere "data che non so leggere" da una
// data vera, invece di riceverla travestita.
func dataISO(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// "17 giu 2026": giorno, mese abbreviato o esteso, anno.
	if parts := strings.Fields(s); len(parts) == 3 {
		if mm, ok := itaMonths[strings.ToLower(parts[1])[:min3(len(parts[1]))]]; ok {
			return isoDa(parts[0], mm, parts[2])
		}
	}

	// Le tre forme numeriche differiscono solo per il separatore.
	sep := "."
	if strings.Contains(s, "/") {
		sep = "/"
	}
	parts := strings.Split(s, sep)
	if len(parts) != 3 {
		return ""
	}
	return isoDa(parts[0], parts[1], parts[2])
}

// isoDa compone la data ISO da giorno, mese e anno già separati, zero-paddando
// giorno e mese ed espandendo l'anno a due cifre. Torna "" se un pezzo non è
// plausibile.
func isoDa(dd, mm, yy string) string {
	dd, mm, yy = strings.TrimSpace(dd), strings.TrimSpace(mm), strings.TrimSpace(yy)
	if len(yy) == 2 {
		// Perno di secolo grezzo: gli atti anteriori al 2000 sono rari sul portale.
		if yy[0] >= '0' && yy[0] <= '4' {
			yy = "20" + yy
		} else {
			yy = "19" + yy
		}
	}
	if len(yy) != 4 || len(mm) == 0 || len(mm) > 2 || len(dd) == 0 || len(dd) > 2 {
		return ""
	}
	if !soloCifre(yy) || !soloCifre(mm) || !soloCifre(dd) {
		return ""
	}
	if len(mm) == 1 {
		mm = "0" + mm
	}
	if len(dd) == 1 {
		dd = "0" + dd
	}
	return yy + "-" + mm + "-" + dd
}

func soloCifre(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// iniettaDataISO percorre il payload e affianca a ogni campo `data` leggibile
// un campo `data_iso` in forma "YYYY-MM-DD".
//
// La fonte scrive le date in quattro dialetti diversi, due dei quali convivono
// nello stesso payload di `ddl iter`, e nessuno è ordinabile come stringa né
// riconosciuto da un foglio di calcolo. I filtri d'ingresso invece vogliono
// ISO: senza questo campo l'output non si può reimmettere in una query che
// usa gli stessi criteri con cui è stato prodotto.
//
// Non tocca gli oggetti che un `data_iso` ce l'hanno già, né le date che non
// riesce a leggere: un campo assente dice "non l'ho saputa normalizzare", ed è
// un'informazione, mentre una stringa vuota si legge come "data mancante".
// Nemmeno tocca i valori che non sono date della fonte — `deputato profilo`
// riporta in radice il range chiesto (`2026-06-01:2026-08-14`), che infatti
// resta senza `data_iso`.
func iniettaDataISO(data json.RawMessage) json.RawMessage {
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err == nil && arr != nil {
		for i, el := range arr {
			arr[i] = iniettaDataISO(el)
		}
		if out, err := json.Marshal(arr); err == nil {
			return out
		}
		return data
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil || obj == nil {
		return data
	}
	for k, v := range obj {
		obj[k] = iniettaDataISO(v)
	}
	if _, gia := obj["data_iso"]; !gia {
		if grezza, ok := obj["data"]; ok {
			var s string
			if json.Unmarshal(grezza, &s) == nil {
				if iso := dataISO(s); iso != "" {
					if enc, err := json.Marshal(iso); err == nil {
						obj["data_iso"] = enc
					}
				}
			}
		}
	}
	if out, err := json.Marshal(obj); err == nil {
		return out
	}
	return data
}

// chiaveData torna la chiave di ordinamento di una data della fonte. Le date
// illeggibili tornano una chiave che le manda in fondo all'ordine decrescente,
// invece di lasciarle competere con stringhe grezze.
func chiaveData(s string) string {
	if iso := dataISO(s); iso != "" {
		return iso
	}
	return ""
}
