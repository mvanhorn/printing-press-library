// Copyright 2026 Luigi Pastore and contributors. Licensed under Apache-2.0. See LICENSE.

package norma

import (
	"sort"
	"strings"
	"unicode"
)

// Fonte is a well-known act that can be called by name instead of by URN.
type Fonte struct {
	Key           string   `json:"key"`
	Nome          string   `json:"nome"`
	Estremi       string   `json:"estremi"`
	URN           string   `json:"urn"`
	Allegato      string   `json:"allegato,omitempty"`
	Alias         []string `json:"alias"`
	Abbreviazioni []string `json:"abbreviazioni,omitempty"`
}

// Base returns the parsed act URN with the annex set, ready for WithArticle.
func (f Fonte) Base() URN {
	u, _ := ParseURN(f.URN)
	u.Allegato = f.Allegato
	return u
}

// Fonti is the embedded table of well-known acts. URNs were verified against
// the live Normattiva service; codici keep their articles in an annex.
var Fonti = []Fonte{
	{Key: "costituzione", Nome: "Costituzione della Repubblica italiana", Estremi: "Costituzione 27 dicembre 1947", URN: "urn:nir:stato:costituzione:1947-12-27",
		Alias: []string{"costituzione", "costituzione italiana", "carta costituzionale", "constitution"}, Abbreviazioni: []string{"cost.", "cost"}},
	{Key: "codice-civile", Nome: "Codice civile", Estremi: "R.D. 16 marzo 1942, n. 262", URN: "urn:nir:stato:regio.decreto:1942-03-16;262", Allegato: "2",
		Alias: []string{"codice civile", "cod. civ.", "civil code"}, Abbreviazioni: []string{"c.c.", "cc", "cod.civ."}},
	{Key: "codice-penale", Nome: "Codice penale", Estremi: "R.D. 19 ottobre 1930, n. 1398", URN: "urn:nir:stato:regio.decreto:1930-10-19;1398", Allegato: "1",
		Alias: []string{"codice penale", "cod. pen.", "criminal code"}, Abbreviazioni: []string{"c.p.", "cp", "cod.pen."}},
	{Key: "codice-procedura-civile", Nome: "Codice di procedura civile", Estremi: "R.D. 28 ottobre 1940, n. 1443", URN: "urn:nir:stato:regio.decreto:1940-10-28;1443", Allegato: "1",
		Alias: []string{"codice di procedura civile", "codice procedura civile"}, Abbreviazioni: []string{"c.p.c.", "cpc"}},
	{Key: "codice-procedura-penale", Nome: "Codice di procedura penale", Estremi: "D.P.R. 22 settembre 1988, n. 447", URN: "urn:nir:stato:decreto.del.presidente.della.repubblica:1988-09-22;447",
		Alias: []string{"codice di procedura penale", "codice procedura penale"}, Abbreviazioni: []string{"c.p.p.", "cpp"}},
	{Key: "codice-privacy", Nome: "Codice in materia di protezione dei dati personali", Estremi: "D.Lgs. 30 giugno 2003, n. 196", URN: "urn:nir:stato:decreto.legislativo:2003-06-30;196",
		Alias: []string{"codice privacy", "codice della privacy", "privacy", "gdpr", "gdpr nazionale", "protezione dei dati personali"}},
	{Key: "cad", Nome: "Codice dell'amministrazione digitale", Estremi: "D.Lgs. 7 marzo 2005, n. 82", URN: "urn:nir:stato:decreto.legislativo:2005-03-07;82",
		Alias: []string{"cad", "codice dell'amministrazione digitale", "codice amministrazione digitale"}},
	{Key: "codice-appalti", Nome: "Codice dei contratti pubblici", Estremi: "D.Lgs. 31 marzo 2023, n. 36", URN: "urn:nir:stato:decreto.legislativo:2023-03-31;36",
		Alias: []string{"codice appalti", "codice dei contratti pubblici", "codice contratti pubblici", "nuovo codice appalti"}},
	{Key: "procedimento-amministrativo", Nome: "Legge sul procedimento amministrativo", Estremi: "L. 7 agosto 1990, n. 241", URN: "urn:nir:stato:legge:1990-08-07;241",
		Alias: []string{"legge 241", "procedimento amministrativo", "legge sul procedimento amministrativo", "accesso agli atti"}},
	{Key: "tuel", Nome: "Testo unico degli enti locali", Estremi: "D.Lgs. 18 agosto 2000, n. 267", URN: "urn:nir:stato:decreto.legislativo:2000-08-18;267",
		Alias: []string{"tuel", "testo unico enti locali", "testo unico degli enti locali"}},
	{Key: "sicurezza-lavoro", Nome: "Testo unico sulla salute e sicurezza sul lavoro", Estremi: "D.Lgs. 9 aprile 2008, n. 81", URN: "urn:nir:stato:decreto.legislativo:2008-04-09;81",
		Alias: []string{"testo unico sicurezza", "sicurezza sul lavoro", "tusl", "81/2008"}},
	{Key: "responsabilita-enti", Nome: "Responsabilità amministrativa degli enti", Estremi: "D.Lgs. 8 giugno 2001, n. 231", URN: "urn:nir:stato:decreto.legislativo:2001-06-08;231",
		Alias: []string{"231", "decreto 231", "responsabilità amministrativa degli enti", "responsabilita amministrativa enti"}},
	{Key: "codice-strada", Nome: "Codice della strada", Estremi: "D.Lgs. 30 aprile 1992, n. 285", URN: "urn:nir:stato:decreto.legislativo:1992-04-30;285",
		Alias: []string{"codice della strada", "codice strada", "cds"}},
	{Key: "codice-consumo", Nome: "Codice del consumo", Estremi: "D.Lgs. 6 settembre 2005, n. 206", URN: "urn:nir:stato:decreto.legislativo:2005-09-06;206",
		Alias: []string{"codice del consumo", "codice consumo"}},
	{Key: "tuir", Nome: "Testo unico delle imposte sui redditi", Estremi: "D.P.R. 22 dicembre 1986, n. 917", URN: "urn:nir:stato:decreto.del.presidente.della.repubblica:1986-12-22;917",
		Alias: []string{"tuir", "testo unico imposte sui redditi"}},
	{Key: "testo-unico-edilizia", Nome: "Testo unico dell'edilizia", Estremi: "D.P.R. 6 giugno 2001, n. 380", URN: "urn:nir:stato:decreto.del.presidente.della.repubblica:2001-06-06;380",
		Alias: []string{"testo unico edilizia", "testo unico dell'edilizia", "tue"}},
	{Key: "statuto-lavoratori", Nome: "Statuto dei lavoratori", Estremi: "L. 20 maggio 1970, n. 300", URN: "urn:nir:stato:legge:1970-05-20;300",
		Alias: []string{"statuto dei lavoratori", "statuto lavoratori"}},
	{Key: "codice-ambiente", Nome: "Codice dell'ambiente", Estremi: "D.Lgs. 3 aprile 2006, n. 152", URN: "urn:nir:stato:decreto.legislativo:2006-04-03;152",
		Alias: []string{"codice dell'ambiente", "codice ambiente", "testo unico ambientale", "tua"}},
	{Key: "trasparenza", Nome: "Decreto trasparenza", Estremi: "D.Lgs. 14 marzo 2013, n. 33", URN: "urn:nir:stato:decreto.legislativo:2013-03-14;33",
		Alias: []string{"decreto trasparenza", "trasparenza", "foia"}},
	{Key: "testo-unico-pubblico-impiego", Nome: "Testo unico sul pubblico impiego", Estremi: "D.Lgs. 30 marzo 2001, n. 165", URN: "urn:nir:stato:decreto.legislativo:2001-03-30;165",
		Alias: []string{"testo unico pubblico impiego", "tupi", "165/2001"}},
}

func normKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '\'' || r == '/':
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// LookupFonte resolves a name, alias or abbreviation to a well-known act.
func LookupFonte(name string) (Fonte, bool) {
	k := normKey(name)
	if k == "" {
		return Fonte{}, false
	}
	for _, f := range Fonti {
		if normKey(f.Key) == k || normKey(f.Nome) == k {
			return f, true
		}
		for _, a := range append(append([]string{}, f.Alias...), f.Abbreviazioni...) {
			if normKey(a) == k {
				return f, true
			}
		}
	}
	return Fonte{}, false
}

// SearchFonti returns the acts whose name, key or aliases contain every word
// of query, sorted by key. An empty query returns the whole table.
func SearchFonti(query string) []Fonte {
	words := strings.Fields(normKey(query))
	out := make([]Fonte, 0, len(Fonti))
	for _, f := range Fonti {
		hay := normKey(strings.Join(append(append([]string{f.Key, f.Nome, f.Estremi}, f.Alias...), f.Abbreviazioni...), " "))
		ok := true
		for _, w := range words {
			if !strings.Contains(hay, w) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
