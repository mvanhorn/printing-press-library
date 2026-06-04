package scraper

import (
	"fmt"
	"strings"
)

// RisultatoComune holds the final election results for a comune.
type RisultatoComune struct {
	Comune      string            `json:"comune"`
	Provincia   string            `json:"provincia"`
	Anno        int               `json:"anno"`
	PopLegale   string            `json:"pop_legale,omitempty"`
	Sezioni     string            `json:"sezioni,omitempty"`
	Elettori    string            `json:"elettori,omitempty"`
	Seggi       string            `json:"seggi,omitempty"`
	Votanti     string            `json:"votanti,omitempty"`
	VotantiPerc string            `json:"votanti_perc,omitempty"`
	Stato       ScrutinioState    `json:"stato_scrutinio"`
	Dettaglio   string            `json:"dettaglio_scrutinio,omitempty"`
	Candidati   []CandidatoRisult `json:"candidati"`
}

// CandidatoRisult holds result data for one candidate.
type CandidatoRisult struct {
	Numero   string  `json:"numero"`
	Eletto   bool    `json:"eletto"`
	Nome     string  `json:"nome"`
	Voti     string  `json:"voti,omitempty"`
	Percento string  `json:"percentuale,omitempty"`
	Liste    []Lista `json:"liste,omitempty"`
}

// FetchRisultati fetches and parses the risultati page for a comune.
func FetchRisultati(comune *Comune, anno int) (*RisultatoComune, string, error) {
	base := ArchiveURL(anno)
	url := fmt.Sprintf("%s/%s/ReportRisultati%s%s.html", base, comune.Provincia, comune.Provincia, comune.Codice)

	body, err := Fetch(url)
	if err != nil {
		return nil, url, err
	}
	if IsUnavailable(body) {
		return nil, url, fmt.Errorf("dati non disponibili per %s %d", comune.Nome, anno)
	}

	r, err := parseRisultati(body, comune, anno)
	return r, url, err
}

// FetchSeggi fetches and parses the seggi page for a comune.
func FetchSeggi(comune *Comune, anno int) (*RisultatoComune, string, error) {
	base := ArchiveURL(anno)
	url := fmt.Sprintf("%s/%s/ReportSeggi%s%s.html", base, comune.Provincia, comune.Provincia, comune.Codice)

	body, err := Fetch(url)
	if err != nil {
		return nil, url, err
	}
	if IsUnavailable(body) {
		return nil, url, fmt.Errorf("dati non disponibili per %s %d", comune.Nome, anno)
	}

	r, err := parseRisultati(body, comune, anno)
	if err == nil {
		r.Anno = anno
	}
	return r, url, err
}

func parseRisultati(body string, comune *Comune, anno int) (*RisultatoComune, error) {
	stato, dettaglio := ExtractScrutinioState(body)

	result := &RisultatoComune{
		Comune:    comune.Nome,
		Provincia: comune.Provincia,
		Anno:      anno,
		Stato:     stato,
		Dettaglio: dettaglio,
	}

	if stato == ScrutinioInCorso {
		return result, nil
	}

	tables := tableRe.FindAllStringSubmatch(body, -1)

	for _, t := range tables {
		tableContent := t[1]
		rows := trRe.FindAllStringSubmatch(tableContent, -1)
		if len(rows) == 0 {
			continue
		}

		// Collect row values
		rowVals := make([][]string, 0, len(rows))
		for _, r := range rows {
			cells := tdRe.FindAllStringSubmatch(r[1], -1)
			vals := make([]string, 0, len(cells))
			for _, c := range cells {
				v := cleanCell(c[1])
				if v != "" {
					vals = append(vals, v)
				}
			}
			if len(vals) > 0 {
				rowVals = append(rowVals, vals)
			}
		}

		if len(rowVals) == 0 {
			continue
		}

		firstRow := rowVals[0]

		// Summary stats table: header starts with "Sezioni"
		// Row 0: Sezioni | Elettori | Seggi | Votanti | ...
		// Row 1: 5 | 4.051 | 10 | Totali | % | ...  (actual sezioni/elettori/seggi)
		// Row 2: 1.713 | 42,29% | ...                (votanti)
		if len(firstRow) >= 3 && firstRow[0] == "Sezioni" {
			if len(rowVals) >= 2 {
				row1 := rowVals[1]
				// row1[0]=Sezioni, row1[1]=Elettori, row1[2]=Seggi
				if len(row1) >= 1 && isNumber(row1[0]) {
					result.Sezioni = row1[0]
				}
				if len(row1) >= 2 && isNumber(row1[1]) {
					result.Elettori = row1[1]
				}
				if len(row1) >= 3 && isNumber(row1[2]) {
					result.Seggi = row1[2]
				}
			}
			// Votanti from the data row (first row where [0] is a number and [1] is a percentage)
			for _, rv := range rowVals[2:] {
				if len(rv) >= 2 && isNumber(rv[0]) && isPerc(rv[1]) {
					result.Votanti = rv[0]
					result.VotantiPerc = rv[1]
					break
				}
			}
			continue
		}

		// Comune summary: "Comune | NOME | Provincia | XX | ..."
		if len(firstRow) >= 2 && firstRow[0] == "Comune" {
			for _, rv := range rowVals {
				switch rv[0] {
				case "Pop.Legale", "Pop. Legale":
					if len(rv) > 1 {
						result.PopLegale = rv[1]
					}
				case "Sezioni":
					if len(rv) > 1 {
						result.Sezioni = rv[1]
					}
				case "Elettori":
					if len(rv) > 1 {
						result.Elettori = rv[1]
					}
				case "Seggi":
					if len(rv) > 1 && result.Seggi == "" {
						result.Seggi = rv[1]
					}
				}
			}
			continue
		}

		// Candidate table: first row starts with N°, numero, tipo, nome, VOTI, voti, %, perc
		if len(firstRow) >= 4 && firstRow[0] == "N°" && firstRow[3] != "Lista/e Collegata/e" {
			cand := CandidatoRisult{
				Numero: firstRow[1],
				Eletto: strings.Contains(firstRow[2], "Eletto"),
				Nome:   firstRow[3],
			}
			if len(firstRow) > 5 && firstRow[4] == "VOTI" {
				cand.Voti = firstRow[5]
			}
			if len(firstRow) > 7 && firstRow[6] == "%" {
				cand.Percento = firstRow[7]
			}

			// Parse remaining rows as liste
			cand.Liste = parseListeRows(rowVals[1:])
			result.Candidati = append(result.Candidati, cand)
			continue
		}
	}

	// Also extract Elettori from the "Sezioni/Elettori/Seggi" table structure
	// by looking at the text before the candidate tables
	if result.Elettori == "" {
		result.Elettori = extractElettori(body)
	}

	return result, nil
}

// parseListeRows parses liste rows from a table starting with the header row.
func parseListeRows(rows [][]string) []Lista {
	var liste []Lista
	headerSeen := false
	for _, vals := range rows {
		if len(vals) == 0 {
			continue
		}
		if vals[0] == "N°" {
			headerSeen = true
			continue
		}
		if !headerSeen {
			continue
		}
		if vals[0] == "Totale" || vals[0] == "-" {
			continue
		}
		if len(vals) >= 2 {
			l := Lista{Numero: vals[0], Nome: vals[1]}
			if len(vals) > 2 {
				l.Candidati = vals[2]
			}
			if len(vals) > 3 {
				l.Voti = vals[3]
			}
			if len(vals) > 4 {
				l.Percento = vals[4]
			}
			if len(vals) > 5 {
				l.Seggi = vals[5]
			}
			liste = append(liste, l)
		}
	}
	return liste
}

// extractElettori finds the Elettori value from the summary stats area.
var elettoriRe = parzRe // reuse regex approach

func extractElettori(body string) string {
	tables := tableRe.FindAllStringSubmatch(body, -1)
	for _, t := range tables {
		rows := trRe.FindAllStringSubmatch(t[1], -1)
		for _, r := range rows {
			cells := tdRe.FindAllStringSubmatch(r[1], -1)
			vals := make([]string, 0, len(cells))
			for _, c := range cells {
				v := cleanCell(c[1])
				if v != "" {
					vals = append(vals, v)
				}
			}
			// Look for "Elettori" label followed by a number
			for i, v := range vals {
				if v == "Elettori" && i+1 < len(vals) && isNumber(vals[i+1]) {
					return vals[i+1]
				}
			}
		}
	}
	return ""
}

func isNumber(s string) bool {
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", "")
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func isPerc(s string) bool {
	return strings.Contains(s, ",") || strings.Contains(s, "%")
}
