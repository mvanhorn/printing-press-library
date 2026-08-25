// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.
//
// Parsing delle pagine gruppi parlamentari di www.ars.sicilia.it.
//
// Selettori verificati sulle pagine vive del 16/08/2026 e certificati dai
// test su testdata/ (docs/anagrafiche-www-ars.md li descrive):
//
//   - elenco (?idLeg=16|17|18): ogni gruppo è una card div.dep che contiene
//     un a.underline con nome e href /gruppi-parlamentari/<slug>
//   - dettaglio (/gruppi-parlamentari/<slug>): h1 «Gruppo <nome>», mailto
//     del gruppo nel blocco «Scrivi al gruppo», componenti in
//     ul.elenco_nomi_dett_gruppo > li:
//       nome   div.dep > p > a (title «On.Nome», href = scheda /deputati/<slug>)
//       ruolo  span.gruppo — facoltativo (Presidente, Vice-Presidente,
//              Segretario, Tesoriere Gruppo Parlamentare)
//       collegio div.collegio > p («Palermo», «Agrigento», «Regionale»)
//       email  mailto dentro div.email
//
// Un'estrazione vuota è errore, non risultato: il portale non pubblica
// elenchi di gruppi vuoti, quindi zero righe significa selettore rotto.

package wwwclient

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
)

// Gruppo è una riga dell'elenco dei gruppi di una legislatura.
type Gruppo struct {
	Legisl int    `json:"legislatura"`
	Nome   string `json:"gruppo"`
	Slug   string `json:"slug"`
	URL    string `json:"url"`
}

// Componente è un deputato iscritto a un gruppo.
type Componente struct {
	Deputato string `json:"deputato"`
	// Ruolo è facoltativo: solo presidente, vice, segretario e tesoriere
	// del gruppo lo hanno. omitempty, non empty string, in JSON.
	Ruolo    string `json:"ruolo,omitempty"`
	Collegio string `json:"collegio,omitempty"`
	Email    string `json:"email,omitempty"`
	// Scheda è l'URL della pagina del deputato su www.ars.sicilia.it.
	Scheda string `json:"scheda"`
}

// DettaglioGruppo è la composizione completa di un gruppo.
type DettaglioGruppo struct {
	Legisl     int          `json:"legislatura"`
	Gruppo     string       `json:"gruppo"`
	Slug       string       `json:"slug"`
	URL        string       `json:"url"`
	Email      string       `json:"email,omitempty"`
	Componenti []Componente `json:"componenti"`
}

// GruppiPath è il path dell'elenco per legislatura.
const GruppiPath = "/gruppi-parlamentari"

// legislatureDaSlug ricava la legislatura dal prefisso dello slug
// («XVIII-misto» → 18). Il numero va mappato, non calcolato: gli slug
// portano il numero romano, le API della CLI quello arabo.
func legislatureDaSlug(slug string) int {
	rom := slug
	if i := strings.Index(slug, "-"); i > 0 {
		rom = slug[:i]
	}
	switch rom {
	case "XVIII":
		return 18
	case "XVII":
		return 17
	case "XVI":
		return 16
	}
	return 0
}

// GruppiElencoReq e GruppoDettaglioReq dicono path e parametri delle due
// richieste, e le usano sia il client sia l'anteprima --dry-run della CLI.
// Prima l'anteprima li ricomponeva per conto suo a partire da GruppiPath: oggi
// coincidevano, ma nulla li teneva agganciati, ed e' la forma da cui nascono le
// anteprime che descrivono una richiesta diversa da quella che parte.
func GruppiElencoReq(legisl int) (path string, params map[string]string) {
	return GruppiPath, map[string]string{"idLeg": strconv.Itoa(legisl)}
}

// GruppoDettaglioReq: la pagina del gruppo non porta parametri, il gruppo sta
// nel path.
func GruppoDettaglioReq(slug string) (path string, params map[string]string) {
	return GruppiPath + "/" + slug, nil
}

// GruppiElenco scarica e parsa l'elenco dei gruppi di una legislatura.
// legislature valide: 16, 17, 18 (XVIII accetta anche 71 sul portale, ma
// qui si normalizza subito al numero canonico).
func (c *Client) GruppiElenco(ctx context.Context, legisl int) ([]Gruppo, error) {
	path, params := GruppiElencoReq(legisl)
	body, err := c.Get(ctx, path, params)
	if err != nil {
		return nil, err
	}
	return ParseGruppiElenco(body, legisl)
}

// GruppoDettaglio scarica e parsa la pagina di un gruppo.
func (c *Client) GruppoDettaglio(ctx context.Context, slug string) (*DettaglioGruppo, error) {
	path, params := GruppoDettaglioReq(slug)
	body, err := c.Get(ctx, path, params)
	if err != nil {
		return nil, err
	}
	return ParseGruppoDettaglio(body, slug)
}

// ParseGruppiElenco estrae i gruppi dalla pagina d'elenco.
func ParseGruppiElenco(body []byte, legisl int) ([]Gruppo, error) {
	root, err := xhtml.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parsing HTML elenco gruppi: %w", err)
	}
	out := []Gruppo{}
	seen := map[string]bool{}
	var walk func(n *xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && n.Data == "div" && hasClass(n, "dep") {
			if a := findDescendant(n, "a", "underline"); a != nil {
				name := strings.TrimSpace(textContent(a))
				href := attrValue(a, "href")
				if name != "" && strings.HasPrefix(href, GruppiPath+"/") {
					slug := strings.TrimPrefix(href, GruppiPath+"/")
					if q := strings.IndexAny(slug, "?#"); q >= 0 {
						slug = slug[:q]
					}
					if !seen[slug] {
						seen[slug] = true
						out = append(out, Gruppo{
							Legisl: legisl,
							Nome:   name,
							Slug:   slug,
							URL:    DefaultBaseURL + "/" + "gruppi-parlamentari/" + slug,
						})
					}
				}
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(root)
	if len(out) == 0 {
		return nil, fmt.Errorf("nessun gruppo estratto dalla pagina d'elenco (legisl %d): selettore rotto o pagina cambiata", legisl)
	}
	return out, nil
}

// ParseGruppoDettaglio estrae la composizione dalla pagina di un gruppo.
func ParseGruppoDettaglio(body []byte, slug string) (*DettaglioGruppo, error) {
	root, err := xhtml.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parsing HTML gruppo %s: %w", slug, err)
	}

	det := &DettaglioGruppo{
		Legisl:     legislatureDaSlug(slug),
		Slug:       slug,
		URL:        DefaultBaseURL + GruppiPath + "/" + slug,
		Componenti: []Componente{},
	}

	// h1 = «Gruppo <nome>»: il prefisso si toglie per riavere il nome
	// canonical, lo stesso dell'elenco e delle firme sugli atti.
	if h1 := findFirst(root, "h1"); h1 != nil {
		nome := strings.TrimSpace(textContent(h1))
		det.Gruppo = strings.TrimPrefix(nome, "Gruppo ")
	}

	// Email del gruppo: il primo mailto fuori dall'elenco dei componenti
	// (il blocco «Scrivi al gruppo» sta sopra la ul; le email dei singoli
	// deputati stanno dentro i li).
	var ul *xhtml.Node
	var findUL func(n *xhtml.Node)
	findUL = func(n *xhtml.Node) {
		if ul != nil {
			return
		}
		if n.Type == xhtml.ElementNode && n.Data == "ul" && hasClass(n, "elenco_nomi_dett_gruppo") {
			ul = n
			return
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			findUL(ch)
		}
	}
	findUL(root)

	var findMailto func(n *xhtml.Node) string
	findMailto = func(n *xhtml.Node) string {
		if n.Type == xhtml.ElementNode && n.Data == "a" {
			if href := attrValue(n, "href"); strings.HasPrefix(href, "mailto:") {
				return strings.TrimPrefix(href, "mailto:")
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			if m := findMailto(ch); m != "" {
				return m
			}
		}
		return ""
	}
	// L'email del gruppo sta in un antenato che non è la ul dei componenti.
	var walkMailto func(n *xhtml.Node, inUL bool)
	walkMailto = func(n *xhtml.Node, inUL bool) {
		if det.Email != "" {
			return
		}
		if n.Type == xhtml.ElementNode && n.Data == "a" && !inUL {
			if href := attrValue(n, "href"); strings.HasPrefix(href, "mailto:") {
				det.Email = strings.TrimPrefix(href, "mailto:")
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walkMailto(ch, inUL || n == ul)
		}
	}
	walkMailto(root, false)

	if ul == nil {
		return nil, fmt.Errorf("elenco componenti non trovato nella pagina del gruppo %s: selettore rotto o pagina cambiata", slug)
	}

	for li := ul.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != xhtml.ElementNode || li.Data != "li" || hasClass(li, "intestazione") {
			continue
		}
		comp := Componente{}
		if a := findDepLink(li); a != nil {
			// Il title («On.Catanzaro Michele») è la forma pulita; il testo
			// del link è lo stesso. Si preferisce il title perché è l'attributo
			// che il portale usa per il tooltip, ma fallback sul testo.
			name := strings.TrimSpace(attrValue(a, "title"))
			if name == "" {
				name = strings.TrimSpace(textContent(a))
			}
			comp.Deputato = name
			href := attrValue(a, "href")
			if strings.HasPrefix(href, "/") {
				comp.Scheda = DefaultBaseURL + href
			} else {
				comp.Scheda = href
			}
		}
		if span := findDescendant(li, "span", "gruppo"); span != nil {
			comp.Ruolo = strings.TrimSpace(textContent(span))
		}
		if div := findDescendant(li, "div", "collegio"); div != nil {
			comp.Collegio = strings.TrimSpace(textContent(div))
		}
		if m := findMailto(li); m != "" {
			comp.Email = m
		}
		if comp.Deputato != "" {
			det.Componenti = append(det.Componenti, comp)
		}
	}
	if len(det.Componenti) == 0 {
		return nil, fmt.Errorf("nessun componente estratto dalla pagina del gruppo %s: selettore rotto o pagina cambiata", slug)
	}
	return det, nil
}

// ---- helper HTML (stesso stile di icaroclient/parse.go) ----

func hasClass(n *xhtml.Node, class string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" {
			for _, c := range strings.Fields(a.Val) {
				if c == class {
					return true
				}
			}
		}
	}
	return false
}

func attrValue(n *xhtml.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// textContent raccoglie il testo dei discendenti, normalizzando gli spazi.
// Il parser x/net/html decodifica già le entità («&#039;» → «'»).
func textContent(n *xhtml.Node) string {
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(x *xhtml.Node) {
		if x.Type == xhtml.TextNode {
			b.WriteString(x.Data)
		}
		for ch := x.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

// findDiscendant generico: primo elemento tag con classe cls ("" = qualsiasi
// classe). Cerca in profondità, figli primi.
func findDescendant(n *xhtml.Node, tag, cls string) *xhtml.Node {
	var found *xhtml.Node
	var walk func(*xhtml.Node)
	walk = func(x *xhtml.Node) {
		if found != nil {
			return
		}
		if x.Type == xhtml.ElementNode && x.Data == tag && (cls == "" || hasClass(x, cls)) {
			found = x
			return
		}
		for ch := x.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	return found
}

func findFirst(n *xhtml.Node, tag string) *xhtml.Node {
	return findDescendant(n, tag, "")
}

// findDepLink trova il link alla scheda deputato dentro un li componente:
// il primo a il cui href punta a /deputati/.
func findDepLink(li *xhtml.Node) *xhtml.Node {
	var found *xhtml.Node
	var walk func(*xhtml.Node)
	walk = func(x *xhtml.Node) {
		if found != nil {
			return
		}
		if x.Type == xhtml.ElementNode && x.Data == "a" {
			if href := attrValue(x, "href"); strings.HasPrefix(href, "/deputati/") {
				found = x
				return
			}
		}
		for ch := x.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(li)
	return found
}
