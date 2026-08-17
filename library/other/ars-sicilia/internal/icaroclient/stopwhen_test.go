// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.

package icaroclient

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// leggiArchive riproduce l'archivio 201, indicizzato PER ARTICOLO: è il caso
// per cui StopWhen esiste, perché lì il limite che l'utente esprime (leggi) non
// coincide con l'unità che il portale pagina (righe-articolo).
var leggiArchive = Archive{
	ID:       "201",
	Slug:     "leggi",
	FieldMap: map[string]string{"legisl": "LEGISL", "anno": "ANNO"},
	Columns:  []string{"Legisl.", "Atto", "Docum.", "Data", "Titolo"},
}

// paginaLeggi costruisce una pagina di shortList da 10 righe-articolo. Le prime
// due leggi sono lunghe 25 articoli come le finanziarie vere, così la finestra
// da 100 righe che il codice usava non basta a vederne più di quattro.
func paginaLeggi(page, totalPages int, righe []string) string {
	var b strings.Builder
	b.WriteString(`<html><body><ul id="shortListTable"><li class="intestazione">header</li>`)
	for i, atto := range righe {
		fmt.Fprintf(&b, `<li href="javascript: showDoc(%d)">
			<div>18</div><div>%s</div><div>Art. %d</div><div>1.01.2025</div>
			<div><h3>Titolo di %s</h3><p>estratto</p></div>
		</li>`, page*100+i, atto, i+1, atto)
	}
	fmt.Fprintf(&b, `</ul><div class="pagination"><span class="pagina_di">Pagina %d di %d</span></div></body></html>`, page, totalPages)
	return b.String()
}

// serverLeggi serve 40 pagine da 10 righe, con leggi da 25 articoli l'una come
// le finanziarie: 100 righe valgono 4 leggi, e per vederne 10 servono 250 righe
// — cioè 25 pagine, dieci in più di quante la stima a priori ne scaricasse.
func serverLeggi(t *testing.T, pagineViste *[]int) *httptest.Server {
	t.Helper()
	const totalPages = 40
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "default.jsp") {
			fmt.Fprint(w, "<html>ok</html>")
			return
		}
		page := 1
		if s := r.URL.Query().Get("setPage"); s != "" {
			page, _ = strconv.Atoi(s)
		}
		*pagineViste = append(*pagineViste, page)
		righe := make([]string, 0, 10)
		for i := range 10 {
			n := (page-1)*10 + i // indice globale della riga
			righe = append(righe, fmt.Sprintf("L.R. %d", n/25+1))
		}
		fmt.Fprint(w, paginaLeggi(page, totalPages, righe))
	}))
}

// Il predicato deve fermare la paginazione sull'unità chiesta dall'utente. Con
// la stima a priori (10 leggi ≈ 100 righe) la risposta erano 4 leggi su 10:
// è il difetto misurato su `leggi cerca --legisl 18 --anno 2025`, che rendeva
// 4 leggi mentre l'anno ne conta 31.
func TestSearchStopWhenContaLeggiNonRighe(t *testing.T) {
	var pagine []int
	srv := serverLeggi(t, &pagine)
	defer srv.Close()

	c, err := New(nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.BaseURL = srv.URL
	c.limiter = nil

	const vogliamo = 10
	conta := func(recs []Record) int {
		visti := map[string]bool{}
		for _, r := range recs {
			visti[r.Fields["Atto"]] = true
		}
		return len(visti)
	}
	var truncated bool
	recs, err := c.Search(context.Background(), leggiArchive, SearchOptions{
		Limit:     500,
		MaxPages:  50,
		Truncated: &truncated,
		StopWhen:  func(all []Record) bool { return conta(all) >= vogliamo },
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if got := conta(recs); got < vogliamo {
		t.Errorf("leggi raccolte = %d, ne servivano %d: la paginazione si è fermata troppo presto", got, vogliamo)
	}
	// La prova che il vecchio comportamento era insufficiente: con le sole 100
	// righe della stima si sarebbero viste 4 leggi.
	if got := conta(recs[:100]); got >= vogliamo {
		t.Fatalf("la fixture non riproduce il caso reale: 100 righe danno già %d leggi", got)
	}
	// E che non pagina all'infinito: si ferma appena le leggi ci sono.
	if len(pagine) >= 40 {
		t.Errorf("scaricate %d pagine su 40: il predicato non ha fermato nulla", len(pagine))
	}
	if !truncated {
		t.Error("truncated = false: l'archivio ha altre pagine non lette, va detto")
	}
}

// Senza predicato il comportamento non cambia: si pagina fino a Limit o
// all'ultima pagina, come prima.
func TestSearchSenzaStopWhenInvariato(t *testing.T) {
	var pagine []int
	srv := serverLeggi(t, &pagine)
	defer srv.Close()

	c, err := New(nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.BaseURL = srv.URL
	c.limiter = nil

	recs, err := c.Search(context.Background(), leggiArchive, SearchOptions{Limit: 30, MaxPages: 50})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(recs) != 30 {
		t.Errorf("righe = %d, attese 30 (Limit conta righe quando non c'è il predicato)", len(recs))
	}
}
