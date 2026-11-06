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

// serverTotale serve `totale` documenti da 10 a pagina e conta le richieste. Con
// dichiara false la pagina di sessione non porta il conteggio.
func serverTotale(t *testing.T, totale int, dichiara bool, sessioni, pagine *int) *httptest.Server {
	t.Helper()
	totalPages := (totale + 9) / 10
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "default.jsp") {
			*sessioni++
			if dichiara {
				fmt.Fprintf(w, "<h3><a >Lista Documenti</a>\n(%d)</h3>", totale)
			} else {
				fmt.Fprint(w, "<html>ok</html>")
			}
			return
		}
		*pagine++
		page := 1
		if s := r.URL.Query().Get("setPage"); s != "" {
			page, _ = strconv.Atoi(s)
		}
		var b strings.Builder
		b.WriteString(`<html><body><ul id="shortListTable"><li class="intestazione">h</li>`)
		for i := (page - 1) * 10; i < page*10 && i < totale; i++ {
			fmt.Fprintf(&b, `<li href="javascript: showDoc(%d)"><div>18</div><div>%d</div><div>1.01.2024</div><div><h3>Titolo</h3></div></li>`, i+1, i+1)
		}
		fmt.Fprintf(&b, `</ul><div class="pagination"><span class="pagina_di">Pagina %d di %d</span></div></body></html>`, page, totalPages)
		fmt.Fprint(w, b.String())
	}))
}

// Il totale sta nella pagina che apre la sessione: leggerlo non deve costare né
// una sessione né una pagina in più, ed è per questo che si legge lì.
func TestSearch_TotaleSenzaRichiesteInPiu(t *testing.T) {
	var sessioni, pagine int
	srv := serverTotale(t, 24, true, &sessioni, &pagine)
	defer srv.Close()
	c, _ := New(nil)
	c.BaseURL = srv.URL
	c.limiter = nil

	var totale int
	var troncato bool
	recs, err := c.Search(context.Background(), ddlArchive, SearchOptions{
		Params: map[string]string{"legisl": "18"}, Limit: 10, MaxPages: 1, Totale: &totale, Truncated: &troncato,
	})
	if err != nil {
		t.Fatal(err)
	}
	if totale != 24 || len(recs) != 10 || !troncato {
		t.Errorf("totale=%d righe=%d troncato=%v, attesi 24, 10, true", totale, len(recs), troncato)
	}
	if sessioni != 1 || pagine != 1 {
		t.Errorf("sessioni=%d pagine=%d, attese 1 e 1", sessioni, pagine)
	}
}

// Senza conteggio nella pagina il totale resta ignoto: non si deduce dalle pagine.
func TestSearch_TotaleAssente(t *testing.T) {
	var sessioni, pagine int
	srv := serverTotale(t, 24, false, &sessioni, &pagine)
	defer srv.Close()
	c, _ := New(nil)
	c.BaseURL = srv.URL
	c.limiter = nil

	var totale int
	if _, err := c.Search(context.Background(), ddlArchive, SearchOptions{
		Params: map[string]string{"legisl": "18"}, Limit: 10, MaxPages: 1, Totale: &totale,
	}); err != nil {
		t.Fatal(err)
	}
	if totale != 0 {
		t.Errorf("totale=%d, atteso 0 (ignoto)", totale)
	}
}

// Una ricerca spezzata non ha un totale unico: ogni fetta dichiara il suo, e
// l'ultimo letto spacciato per il totale sarebbe un numero finto.
func TestSearch_TotaleAssenteSeSpezzata(t *testing.T) {
	var viste []string
	srv := serverSpezzato(t, 400, &viste)
	defer srv.Close()
	c, _ := New(nil)
	c.BaseURL = srv.URL

	var totale int
	var spezzato bool
	if _, err := c.Search(context.Background(), ddlArchive, SearchOptions{
		Params: map[string]string{"legisl": "18", "data": "230101/240229"}, MaxPages: 1, Totale: &totale, Spezzato: &spezzato,
	}); err != nil {
		t.Fatal(err)
	}
	if !spezzato {
		t.Fatal("la ricerca doveva spezzarsi")
	}
	if totale != 0 {
		t.Errorf("totale=%d su ricerca spezzata, atteso 0", totale)
	}
}

// Il flag di troncamento ai confini: appena sotto il totale, al totale, oltre, e
// sul bordo di pagina. Misurato dal vivo su 24 atti: 10, 20, 23 sì; 24, 25, 50 no.
func TestSearch_TroncatoAiConfini(t *testing.T) {
	casi := []struct {
		totale, limit, righe int
		troncato             bool
	}{
		{24, 10, 10, true},
		{24, 20, 20, true},
		{24, 23, 23, true},
		{24, 24, 24, false},
		{24, 25, 24, false},
		{24, 50, 24, false},
		{20, 20, 20, false},
	}
	for _, k := range casi {
		var sessioni, pagine int
		srv := serverTotale(t, k.totale, true, &sessioni, &pagine)
		c, _ := New(nil)
		c.BaseURL = srv.URL
		c.limiter = nil
		var troncato bool
		recs, err := c.Search(context.Background(), ddlArchive, SearchOptions{
			Params: map[string]string{"legisl": "18"}, Limit: k.limit, MaxPages: (k.limit + 9) / 10, Truncated: &troncato,
		})
		srv.Close()
		if err != nil {
			t.Fatal(err)
		}
		if len(recs) != k.righe || troncato != k.troncato {
			t.Errorf("totale=%d limit=%d: righe=%d troncato=%v, attesi %d, %v", k.totale, k.limit, len(recs), troncato, k.righe, k.troncato)
		}
	}
}
