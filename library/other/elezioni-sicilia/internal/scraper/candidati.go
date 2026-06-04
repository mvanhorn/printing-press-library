package scraper

import (
	"fmt"
	"strings"
)

// CandidatoSindaco represents a mayoral candidate with their vote count.
type CandidatoSindaco struct {
	Numero   string `json:"numero"`
	Nome     string `json:"nome"`
	Voti     string `json:"voti,omitempty"`
	Percento string `json:"percentuale,omitempty"`
}

// RiportoCandidati holds candidati data for one comune.
type RiportoCandidati struct {
	Comune    string             `json:"comune"`
	Provincia string             `json:"provincia"`
	Anno      int                `json:"anno"`
	Stato     ScrutinioState     `json:"stato_scrutinio"`
	Dettaglio string             `json:"dettaglio_scrutinio,omitempty"`
	Candidati []CandidatoSindaco `json:"candidati"`
}

// FetchCandidati fetches and parses the candidati page for a comune.
func FetchCandidati(comune *Comune, anno int) (*RiportoCandidati, string, error) {
	base := ArchiveURL(anno)
	url := fmt.Sprintf("%s/%s/ReportCandidati%s%s.html", base, comune.Provincia, comune.Provincia, comune.Codice)

	body, err := Fetch(url)
	if err != nil {
		return nil, url, err
	}
	if IsUnavailable(body) {
		return nil, url, fmt.Errorf("dati non disponibili per %s %d", comune.Nome, anno)
	}

	r, err := parseCandidati(body, comune, anno)
	return r, url, err
}

func parseCandidati(body string, comune *Comune, anno int) (*RiportoCandidati, error) {
	stato, dettaglio := ExtractScrutinioState(body)

	result := &RiportoCandidati{
		Comune:    comune.Nome,
		Provincia: comune.Provincia,
		Anno:      anno,
		Stato:     stato,
		Dettaglio: dettaglio,
	}

	// Extract candidates: look for "N° X Candidato Sindaco NAME VOTI N % P%"
	// The page has each candidate's block scattered in the HTML
	rows := trRe.FindAllStringSubmatch(body, -1)

	var current CandidatoSindaco
	for _, r := range rows {
		cells := tdRe.FindAllStringSubmatch(r[1], -1)
		vals := make([]string, 0, len(cells))
		for _, c := range cells {
			v := cleanCell(c[1])
			if v != "" {
				vals = append(vals, v)
			}
		}

		if len(vals) == 0 {
			continue
		}

		// Look for "N°" marker
		for i, v := range vals {
			if v == "N°" && i+1 < len(vals) {
				// Next might be the number
				if len(vals) > i+2 {
					label := strings.ToLower(vals[i+2])
					if strings.Contains(label, "candidat") || strings.Contains(label, "sindac") {
						// Save previous candidate if any
						if current.Nome != "" {
							result.Candidati = append(result.Candidati, current)
						}
						current = CandidatoSindaco{
							Numero: vals[i+1],
						}
						if i+3 < len(vals) {
							current.Nome = vals[i+3]
						}
					}
				}
			}
			// Look for VOTI label
			if v == "VOTI" && i+1 < len(vals) && current.Nome != "" {
				current.Voti = vals[i+1]
			}
			if v == "%" && i+1 < len(vals) && current.Nome != "" && current.Voti != "" {
				current.Percento = vals[i+1]
			}
		}
	}
	// Add last candidate
	if current.Nome != "" {
		result.Candidati = append(result.Candidati, current)
	}

	return result, nil
}
