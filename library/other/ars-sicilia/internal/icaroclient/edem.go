package icaroclient

// Client per le viste pre-aggregate /edem/ del portale ARS ("eDemocracy").
// A differenza di /icaro/ e /bd/ (che rispondono a query di ricerca), i canali
// /edem/channel.jsp?channel=N sono classifiche/aggregazioni dei Disegni di Legge
// GIÀ CALCOLATE dal portale: una singola GET restituisce l'intera vista, senza
// sessione né paginazione. Sono usate per fornire `analytics --group-by
// proponente|gruppo` con UNA sola richiesta (vs le ~90 di --group-by oratore o il
// deep-sync per i cofirmatari).
//
// Nota di scope: /edem/ aggrega SOLO la legislatura corrente (le classifiche non
// sono parametrizzabili per legislatura). Il conteggio è per primo firmatario
// (proponente): ch1 per anno, ch6 per proponente e ch7 per gruppo sommano tutti
// allo stesso totale, quindi ogni DDL è contato una volta.

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// Canali /edem/ usati per le classifiche DDL. La mappa completa dei canali è in
// docs/bd-migration (ch1 anno, ch2 recenti, ch3 argomento, ch4 in commissione,
// ch5 in aula, ch8/ch9 iniziativa). Qui servono solo le due classifiche.
const (
	EdemChannelProponente = 6 // DDL per deputato proponente (primo firmatario)
	EdemChannelGruppo     = 7 // DDL per gruppo parlamentare
)

// DDLRankItem è una riga di una classifica /edem/: nome (deputato o gruppo) e
// numero di DDL attribuiti.
type DDLRankItem struct {
	Name  string `json:"nome"`
	Count int    `json:"ddl"`
}

// DDLRanking scarica la classifica pre-aggregata del canale /edem/ indicato e
// ritorna le righe (nome, conteggio) ordinate per conteggio decrescente. Una sola
// GET, senza sessione: la vista non è paginata.
func (c *Client) DDLRanking(ctx context.Context, channel int) ([]DDLRankItem, error) {
	edemURL := fmt.Sprintf("%s/edem/channel.jsp?channel=%d", c.BaseURL, channel)
	body, err := c.get(ctx, edemURL)
	if err != nil {
		return nil, fmt.Errorf("edem channel %d: %w", channel, err)
	}
	rows, err := parseEdemRanking(body)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Count > rows[j].Count })
	return rows, nil
}

// parseEdemRanking estrae le righe (nome, conteggio) dalla `ul.tabella` di una
// pagina /edem/channel.jsp. Ogni <li> di dato ha due colonne: la prima con il
// link `<a href="javascript: showDDL(...)">Nome</a>` (preceduto dal link "+"
// `class="goto"`, da ignorare), la seconda con `<span class="simobile">N° DDL</span>`
// seguito dal conteggio.
func parseEdemRanking(body string) ([]DDLRankItem, error) {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parsing /edem/ HTML: %w", err)
	}
	var ul *html.Node
	walk(root, func(n *html.Node) {
		if ul == nil && n.Type == html.ElementNode && n.Data == "ul" && hasClass(n, "tabella") {
			ul = n
		}
	})
	if ul == nil {
		return nil, nil
	}
	var rows []DDLRankItem
	for li := ul.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != html.ElementNode || li.Data != "li" {
			continue
		}
		if hasClass(li, "intestazione") {
			continue // riga di intestazione (Titolo / Numero totale DDL)
		}
		name := edemRowName(li)
		count, ok := edemRowCount(li)
		if name == "" || !ok {
			continue
		}
		rows = append(rows, DDLRankItem{Name: name, Count: count})
	}
	return rows, nil
}

// edemRowName ritorna il testo dell'anchor della classifica: quello il cui href
// richiama showDDL(...). Il primo <a> della riga è il pulsante "+"
// (`class="goto"`, href setItem), da saltare.
func edemRowName(li *html.Node) string {
	var name string
	walk(li, func(n *html.Node) {
		if name != "" || n.Type != html.ElementNode || n.Data != "a" {
			return
		}
		if strings.Contains(attr(n, "href"), "showDDL") {
			name = collapseSpaces(textContent(n))
		}
	})
	return name
}

// edemRowCount ritorna il conteggio DDL della riga, letto dalla colonna con la
// label simobile "N° DDL" (il numero segue la label nello stesso div).
func edemRowCount(li *html.Node) (int, bool) {
	for div := li.FirstChild; div != nil; div = div.NextSibling {
		if div.Type != html.ElementNode || div.Data != "div" || !hasClass(div, "intesta") {
			continue
		}
		label := strings.TrimSpace(findSimobileLabel(div))
		if label == "" || !strings.Contains(label, "DDL") {
			continue
		}
		val := strings.TrimSpace(stripSimobileLabel(textContent(div), label))
		if n, err := strconv.Atoi(val); err == nil {
			return n, true
		}
	}
	return 0, false
}
