package icaroclient

import (
	"fmt"
	"strings"
	"time"
)

// I parametri che possono portare un range di date AAMMGG/AAMMGG. `anno` c'è
// perché su ddl non esiste un campo anno: --anno è compilato in un range DATPRE
// 1 gen - 31 dic (vedi normalizeParams), quindi non è l'alternativa sicura a
// --data che sembra — è la stessa cosa, con estremi fissi.
var chiaviRange = []string{"data", "anno"}

// chiaveRange trova il parametro che porta un range di date spezzabile.
// Ritorna la chiave e i due estremi.
func chiaveRange(params map[string]string) (chiave, lo, hi string, ok bool) {
	for _, k := range chiaviRange {
		v := strings.TrimSpace(params[k])
		a, b, isRange := strings.Cut(v, "/")
		if !isRange || !isAAMMGGRange(a, b) {
			continue
		}
		return k, a, b, true
	}
	return "", "", "", false
}

// isAAMMGGRange dice se i due estremi sono un range di date valido.
//
// L'ordine va giudicato sulle date, non sulle stringhe: dopo che daAAMMGG ha
// smesso di prefissare il secolo a occhio, il confronto lessicografico non
// concorda piu' con quello cronologico. `990101/001231` e' il 1999-2000, un
// range legittimo, ma "990101" > "001231" e finiva scartato — e uno scarto qui
// non e' silenzioso a meta': fa tornare al chiamante il rifiuto del portale
// invece della risposta a fette, che e' il difetto che questo file chiude.
func isAAMMGGRange(a, b string) bool {
	lo, err := daAAMMGG(a)
	if err != nil {
		return false
	}
	hi, err := daAAMMGG(b)
	return err == nil && lo.Before(hi)
}

func soloCifre(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// daAAMMGG espande la data a sei cifre in una time.Time per farci aritmetica.
//
// Il secolo non c'è e va scelto, e la scelta NON può essere quella di
// `time.Parse("060102", …)`: il layout `06` di Go perna sul 68/69, quindi
// leggerebbe `470101` come 2047 e `690101` come 1969. Su un range storico che
// attraversa quel perno gli estremi escono invertiti, `spezzaPerAnno` lo
// scarta, e il rifiuto del portale torna al chiamante invece di essere
// riprovato a fette — cioè il difetto che questo file esiste per chiudere,
// ricomparso su un intervallo diverso.
//
// La finestra giusta è già stabilita altrove nella CLI (`bd.go`, `data_iso.go`)
// ed è fondata sull'archivio, non su una convenzione generica: il documento più
// antico dell'ARS è la seduta inaugurale del 25/05/1947, nel 1946 non c'è
// nulla. Quindi da 47 in su è Novecento, sotto è Duemila. Il prezzo, identico
// a quello che paga `bd.go`, è che AAMMGG non arriva al 2047: là si scrive la
// data per esteso, che non è ambigua.
func daAAMMGG(s string) (time.Time, error) {
	if len(s) != 6 || !soloCifre(s) {
		return time.Time{}, fmt.Errorf("data AAMMGG non valida %q", s)
	}
	secolo := "20"
	if s[:2] >= "47" {
		secolo = "19"
	}
	t, err := time.Parse("20060102", secolo+s)
	if err != nil {
		return time.Time{}, fmt.Errorf("data AAMMGG non valida %q: %w", s, err)
	}
	return t, nil
}

func aAAMMGG(t time.Time) string { return t.Format("060102") }

// spezzaPerAnno taglia il range sui confini di anno solare. Ritorna le fette
// dalla più recente alla più vecchia: le fette sono blocchi cronologici, quindi
// concatenarle un ordine glielo impone comunque, e questo è quello che gli altri
// comandi della CLI già danno (il più recente prima).
//
// Ritorna nil se il range sta dentro un anno solo: lì per tagliare serve
// spezzaAMeta.
func spezzaPerAnno(lo, hi string) []string {
	a, err := daAAMMGG(lo)
	if err != nil {
		return nil
	}
	b, err := daAAMMGG(hi)
	if err != nil || !a.Before(b) || a.Year() == b.Year() {
		return nil
	}
	var fette []string
	for anno := a.Year(); anno <= b.Year(); anno++ {
		inizio := time.Date(anno, 1, 1, 0, 0, 0, 0, time.UTC)
		fine := time.Date(anno, 12, 31, 0, 0, 0, 0, time.UTC)
		if inizio.Before(a) {
			inizio = a
		}
		if fine.After(b) {
			fine = b
		}
		fette = append(fette, aAAMMGG(inizio)+"/"+aAAMMGG(fine))
	}
	return inverti(fette)
}

// spezzaAMeta taglia il range in due sul giorno di mezzo. Serve quando una fetta
// annuale cede ancora: un anno solare non è una garanzia, il motore cede sul
// NUMERO di documenti e la densità cambia da archivio ad archivio.
//
// Ritorna nil sui range di un giorno solo, dove non c'è più niente da tagliare.
func spezzaAMeta(lo, hi string) []string {
	a, err := daAAMMGG(lo)
	if err != nil {
		return nil
	}
	b, err := daAAMMGG(hi)
	if err != nil || !a.Before(b) {
		return nil
	}
	mezzo := a.Add(b.Sub(a) / 2)
	if !mezzo.Before(b) || mezzo.Before(a) {
		return nil
	}
	return []string{
		aAAMMGG(mezzo.AddDate(0, 0, 1)) + "/" + aAAMMGG(b),
		aAAMMGG(a) + "/" + aAAMMGG(mezzo),
	}
}

func inverti(s []string) []string {
	out := make([]string, len(s))
	for i, v := range s {
		out[len(s)-1-i] = v
	}
	return out
}
