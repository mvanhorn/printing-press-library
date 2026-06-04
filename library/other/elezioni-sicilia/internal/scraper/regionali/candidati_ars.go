package regionali

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/scraper/htmlparse"
)

// CandidatoARS is one candidate for the ARS (regional assembly) with the
// preference votes received in a given province.
type CandidatoARS struct {
	Ordine     string `json:"ordine"`
	Nome       string `json:"nome"`
	LuogoData  string `json:"luogo_data_nascita"`
	Preferenze string `json:"preferenze"`
}

// ListaProvincialeARS is one provincial party list with its candidates and
// preference votes in a single province.
type ListaProvincialeARS struct {
	Numero    string         `json:"numero"`
	Nome      string         `json:"nome"`
	Candidati []CandidatoARS `json:"candidati"`
}

// CandidatiARSProvincia is the full preference-vote report for one province.
type CandidatiARSProvincia struct {
	Anno      int                   `json:"anno"`
	Provincia string                `json:"provincia"`
	Liste     []ListaProvincialeARS `json:"liste"`
}

// FetchCandidatiARS fetches and parses
// rep_5/<Provincia>/votiCandidatiProvincia<XX>.html for the given province code.
func FetchCandidatiARS(anno int, provincia string) (*CandidatiARSProvincia, string, error) {
	city := ProvinceCity(provincia)
	if city == "" {
		return nil, "", fmt.Errorf("provincia sconosciuta: %s", provincia)
	}
	url := fmt.Sprintf("%s/rep_5/%s/votiCandidatiProvincia%s.html", BaseURL(anno), city, provincia)
	body, err := fetch(url)
	if err != nil {
		return nil, url, err
	}
	out := &CandidatiARSProvincia{Anno: anno, Provincia: provincia}

	rows := htmlparse.TrRe.FindAllStringSubmatch(body, -1)
	var current *ListaProvincialeARS
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

		// LISTA N. <num> <nome> header (single cell typically)
		headerFound := false
		for _, v := range vals {
			if m := listaHeaderRe.FindStringSubmatch(v); len(m) == 3 {
				if current != nil {
					out.Liste = append(out.Liste, *current)
				}
				current = &ListaProvincialeARS{Numero: m[1], Nome: m[2]}
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

		// Skip column header rows
		if len(vals) >= 2 && (vals[0] == "Ordine" || vals[1] == "Cognome Nome") {
			continue
		}

		// Data row: ordine | nome | luogo,data | preferenze
		if len(vals) == 4 {
			current.Candidati = append(current.Candidati, CandidatoARS{
				Ordine:     vals[0],
				Nome:       vals[1],
				LuogoData:  vals[2],
				Preferenze: vals[3],
			})
		}
	}
	if current != nil {
		out.Liste = append(out.Liste, *current)
	}
	return out, url, nil
}
