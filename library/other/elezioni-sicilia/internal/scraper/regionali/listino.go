package regionali

import (
	"fmt"
	"regexp"

	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/scraper/htmlparse"
)

// CandidatoListino is one candidate of a regional party list. The candidate
// at position 1 is the presidential candidate (capolista).
type CandidatoListino struct {
	Numero      string `json:"numero"` // "1" for capolista, "2", "3" ...
	Nome        string `json:"nome"`
	LuogoData   string `json:"luogo_data_nascita"`
	Capolista   bool   `json:"capolista,omitempty"`
}

// ListaRegionale is one regional party list with all its candidates.
type ListaRegionale struct {
	Numero     string             `json:"numero"`
	Nome       string             `json:"nome"`
	Candidati  []CandidatoListino `json:"candidati"`
}

// ListinoRegionale is the full list of regional candidates for one year.
type ListinoRegionale struct {
	Anno  int              `json:"anno"`
	Liste []ListaRegionale `json:"liste"`
}

var listaHeaderRe = regexp.MustCompile(`(?i)LISTA N\.\s*(\d+)\s+(.+)`)
var capolistaRe = regexp.MustCompile(`(?i)CAPOLISTA|PRESIDENTE`)

// FetchListino fetches and parses the rep_9/listeRegionali.html page.
func FetchListino(anno int) (*ListinoRegionale, string, error) {
	url := fmt.Sprintf("%s/rep_9/listeRegionali.html", BaseURL(anno))
	body, err := fetch(url)
	if err != nil {
		return nil, url, err
	}
	out := &ListinoRegionale{Anno: anno}

	rows := htmlparse.TrRe.FindAllStringSubmatch(body, -1)

	var current *ListaRegionale
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

		// Check for "LISTA N. <num> <name>" header in any cell
		headerFound := false
		for _, v := range vals {
			if m := listaHeaderRe.FindStringSubmatch(v); len(m) == 3 {
				if current != nil {
					out.Liste = append(out.Liste, *current)
				}
				current = &ListaRegionale{Numero: m[1], Nome: m[2]}
				headerFound = true
				break
			}
		}
		if headerFound {
			continue
		}
		if current == nil {
			continue
		}

		// Skip header rows of the candidate table
		if len(vals) >= 3 && (vals[0] == "Numero d'ordine" || vals[1] == "Cognome Nome") {
			continue
		}

		// Data row: numero | nome | luogo, data
		if len(vals) == 3 {
			cand := CandidatoListino{
				Numero:    vals[0],
				Nome:      vals[1],
				LuogoData: vals[2],
			}
			// "1" is replaced by the CAPOLISTA marker in the source; detect via
			// the marker text in vals[0] or the "Detta/o" pattern after the name.
			if vals[0] == "" || isCapolistaMarker(vals[0]) {
				cand.Numero = "1"
				cand.Capolista = true
			}
			current.Candidati = append(current.Candidati, cand)
		}
	}
	if current != nil {
		out.Liste = append(out.Liste, *current)
	}
	return out, url, nil
}

func isCapolistaMarker(s string) bool {
	return capolistaRe.MatchString(s)
}
