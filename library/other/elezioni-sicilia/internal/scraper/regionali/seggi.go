package regionali

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/scraper/htmlparse"
)

// SeggiPerLista holds the seat allocation for one provincial list across all
// 9 provinces plus the regional total. A dash ("-") is used by the source when
// the list did not run in a given province.
type SeggiPerLista struct {
	Lista string            `json:"lista"`
	Seggi map[string]string `json:"seggi"` // keys: AG, CL, CT, EN, ME, PA, RG, SR, TP, TOT
}

// RipartoSeggi is the full seat allocation report for one regional election year.
type RipartoSeggi struct {
	Anno   int             `json:"anno"`
	Liste  []SeggiPerLista `json:"liste"`
}

// FetchSeggi fetches and parses the rep_8/ripartoSeggi.html page.
func FetchSeggi(anno int) (*RipartoSeggi, string, error) {
	url := fmt.Sprintf("%s/rep_8/ripartoSeggi.html", BaseURL(anno))
	body, err := fetch(url)
	if err != nil {
		return nil, url, err
	}
	out := &RipartoSeggi{Anno: anno}

	body = LogoTableRe.ReplaceAllString(body, "$1")
	rows := htmlparse.TrRe.FindAllStringSubmatch(body, -1)

	cols := []string{"AG", "CL", "CT", "EN", "ME", "PA", "RG", "SR", "TP", "TOT"}

	for _, r := range rows {
		cells := htmlparse.TdRe.FindAllStringSubmatch(r[1], -1)
		if len(cells) != 11 {
			continue // not a data row (we expect 1 name + 9 provinces + total)
		}
		vals := make([]string, 0, len(cells))
		for _, c := range cells {
			vals = append(vals, htmlparse.CleanCell(c[1]))
		}
		if vals[0] == "" || vals[0] == "Totali" {
			continue
		}
		entry := SeggiPerLista{Lista: vals[0], Seggi: make(map[string]string, len(cols))}
		for i, code := range cols {
			entry.Seggi[code] = vals[i+1]
		}
		out.Liste = append(out.Liste, entry)
	}
	return out, url, nil
}
