package regionali

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/scraper/htmlparse"
)

// Presidente holds one presidential ticket: candidate + regional list +
// linked provincial lists with their per-list votes, percentage and seats.
type Presidente struct {
	Numero          string             `json:"numero"`
	NomeLista       string             `json:"nome_lista_regionale"`
	Nome            string             `json:"nome"`
	Voti            string             `json:"voti,omitempty"`
	Percentuale     string             `json:"percentuale,omitempty"`
	ListeCollegate  []ListaProvinciale `json:"liste_provinciali_collegate"`
	TotaliVoti      string             `json:"totali_voti,omitempty"`
	TotaliPercent   string             `json:"totali_percentuale,omitempty"`
	TotaliSeggi     string             `json:"totali_seggi,omitempty"`
}

// ListaProvinciale is one provincial party list linked to a presidential
// ticket, with aggregated votes/seats across all provinces.
type ListaProvinciale struct {
	Nome        string `json:"nome"`
	Voti        string `json:"voti,omitempty"`
	Percentuale string `json:"percentuale,omitempty"`
	Seggi       string `json:"seggi,omitempty"`
}

// RiepilogoPresidenti is the full presidential vote report for one year.
type RiepilogoPresidenti struct {
	Anno       int          `json:"anno"`
	Sezioni    string       `json:"sezioni,omitempty"`
	Presidenti []Presidente `json:"presidenti"`
}

var (
	listaNumRe = regexp.MustCompile(`LISTA REGIONALE N\.\s*(\d+)`)
	sezioniRe  = regexp.MustCompile(`n\.\s*(\d+)\s+sezioni\s+su\s+n\.\s*(\d+)\s+sezioni`)
)

// FetchPresidente fetches and parses the rep_7/riepilogoRegionale.html page.
func FetchPresidente(anno int) (*RiepilogoPresidenti, string, error) {
	url := fmt.Sprintf("%s/rep_7/riepilogoRegionale.html", BaseURL(anno))
	body, err := fetch(url)
	if err != nil {
		return nil, url, err
	}
	r, err := parsePresidente(body, anno)
	return r, url, err
}

func parsePresidente(body string, anno int) (*RiepilogoPresidenti, error) {
	out := &RiepilogoPresidenti{Anno: anno}

	if m := sezioniRe.FindStringSubmatch(body); len(m) == 3 {
		out.Sezioni = fmt.Sprintf("%s/%s sezioni", m[1], m[2])
	}

	// Strip the logo+name nested tables so that <tr> matching is not confused.
	body = LogoTableRe.ReplaceAllString(body, "$1")

	rows := htmlparse.TrRe.FindAllStringSubmatch(body, -1)

	// Walk rows with a small state machine. Each Presidente block starts at
	// "LISTA REGIONALE N. <num>" and ends at "Totali ...".
	var current *Presidente
	state := "idle"

	for _, r := range rows {
		cells := htmlparse.TdRe.FindAllStringSubmatch(r[1], -1)
		vals := make([]string, 0, len(cells))
		for _, c := range cells {
			v := htmlparse.WsRe.ReplaceAllString(htmlparse.CleanCell(c[1]), " ")
			vals = append(vals, v)
		}
		if len(vals) == 0 {
			continue
		}
		joined := strings.Join(vals, " | ")

		// New presidential block
		if m := listaNumRe.FindStringSubmatch(joined); len(m) == 2 {
			if current != nil {
				out.Presidenti = append(out.Presidenti, *current)
			}
			current = &Presidente{Numero: m[1]}
			state = "expect_lista_name"
			continue
		}
		if current == nil {
			continue
		}

		switch state {
		case "expect_lista_name":
			// Row with the regional list name (single cell, no "Candidato")
			if !strings.Contains(joined, "Candidato presidente") {
				current.NomeLista = vals[0]
				state = "expect_candidate_header"
			}
		case "expect_candidate_header":
			if strings.Contains(joined, "Candidato presidente") {
				state = "expect_candidate_row"
			}
		case "expect_candidate_row":
			// Pattern: name | voti | %
			if len(vals) >= 3 {
				current.Nome = vals[0]
				current.Voti = vals[1]
				current.Percentuale = vals[2]
				state = "expect_liste_header"
			}
		case "expect_liste_header":
			if strings.Contains(joined, "Liste Provinciali collegate") {
				state = "in_liste"
			}
		case "in_liste":
			// Skip column header rows ("Voti", "%", "Seggi" alone)
			if isHeaderRow(vals) {
				continue
			}
			// Totals row closes the block
			if strings.Contains(joined, "Totali") {
				if len(vals) >= 4 {
					current.TotaliVoti = vals[1]
					current.TotaliPercent = vals[2]
					current.TotaliSeggi = vals[3]
				}
				out.Presidenti = append(out.Presidenti, *current)
				current = nil
				state = "idle"
				continue
			}
			// Provincial list row: nome | voti | % | seggi
			if len(vals) >= 4 {
				current.ListeCollegate = append(current.ListeCollegate, ListaProvinciale{
					Nome:        vals[0],
					Voti:        vals[1],
					Percentuale: vals[2],
					Seggi:       vals[3],
				})
			}
		}
	}
	if current != nil {
		out.Presidenti = append(out.Presidenti, *current)
	}
	return out, nil
}

func isHeaderRow(vals []string) bool {
	for _, v := range vals {
		switch v {
		case "Voti", "%", "Seggi (*)", "Seggi":
			// header label
		default:
			return false
		}
	}
	return true
}
