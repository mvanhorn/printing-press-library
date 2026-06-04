package regionali

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/scraper/htmlparse"
)

// AffluenzaRilevamento is a single (date + hour) snapshot of turnout for one province,
// with the comparison values from the previous regional election.
type AffluenzaRilevamento struct {
	Orario       string `json:"orario"`
	Elettori     string `json:"elettori,omitempty"`
	Votanti      string `json:"votanti,omitempty"`
	Percentuale  string `json:"percentuale,omitempty"`
	ElettoriPrec string `json:"elettori_precedenti,omitempty"`
	VotantiPrec  string `json:"votanti_precedenti,omitempty"`
	PercPrec     string `json:"percentuale_precedenti,omitempty"`
}

// AffluenzaProvincia groups all turnout time slots for one province.
type AffluenzaProvincia struct {
	Provincia    string                 `json:"provincia"`
	Rilevamenti  []AffluenzaRilevamento `json:"rilevamenti"`
}

// AffluenzaRegionale is the full turnout report for one regional election year.
type AffluenzaRegionale struct {
	Anno     int                  `json:"anno"`
	Province []AffluenzaProvincia `json:"province"`
}

var orarioHeaderRe = regexp.MustCompile(`headertabella"[^>]*colspan="3">([^<]+)<`)

// FetchAffluenza fetches all rep_6/affluenzaRegionaleN.html snapshots (1..3)
// and returns them grouped by province.
func FetchAffluenza(anno int) (*AffluenzaRegionale, []string, error) {
	out := &AffluenzaRegionale{Anno: anno}
	urls := []string{}

	// keep insertion order: AG, CL, CT, EN, ME, PA, RG, SR, TP
	provOrder := []string{"AG", "CL", "CT", "EN", "ME", "PA", "RG", "SR", "TP"}
	provByName := map[string]string{
		"Agrigento": "AG", "Caltanissetta": "CL", "Catania": "CT", "Enna": "EN",
		"Messina": "ME", "Palermo": "PA", "Ragusa": "RG", "Siracusa": "SR", "Trapani": "TP",
	}
	byProv := map[string]*AffluenzaProvincia{}
	for _, p := range provOrder {
		byProv[p] = &AffluenzaProvincia{Provincia: p}
	}

	for slot := 1; slot <= 3; slot++ {
		url := fmt.Sprintf("%s/rep_6/affluenzaRegionale%d.html", BaseURL(anno), slot)
		urls = append(urls, url)
		body, err := fetch(url)
		if err != nil {
			return nil, urls, err
		}
		orario := ""
		if m := orarioHeaderRe.FindStringSubmatch(body); len(m) == 2 {
			orario = strings.TrimSpace(m[1])
		}
		parseAffluenzaSlot(body, orario, provByName, byProv)
	}

	for _, p := range provOrder {
		out.Province = append(out.Province, *byProv[p])
	}
	return out, urls, nil
}

func parseAffluenzaSlot(body, orario string, provByName map[string]string, byProv map[string]*AffluenzaProvincia) {
	rows := htmlparse.TrRe.FindAllStringSubmatch(body, -1)
	for _, r := range rows {
		cells := htmlparse.TdRe.FindAllStringSubmatch(r[1], -1)
		vals := make([]string, 0, len(cells))
		for _, c := range cells {
			vals = append(vals, htmlparse.CleanCell(c[1]))
		}
		if len(vals) != 7 {
			continue
		}
		code := provByName[vals[0]]
		if code == "" {
			continue
		}
		byProv[code].Rilevamenti = append(byProv[code].Rilevamenti, AffluenzaRilevamento{
			Orario:       orario,
			Elettori:     vals[1],
			Votanti:      vals[2],
			Percentuale:  vals[3],
			ElettoriPrec: vals[4],
			VotantiPrec:  vals[5],
			PercPrec:     vals[6],
		})
	}
}
