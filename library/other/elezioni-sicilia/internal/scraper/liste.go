package scraper

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/scraper/htmlparse"
)

// Lista represents a party list linked to a mayoral candidate.
type Lista struct {
	Numero    string `json:"numero"`
	Nome      string `json:"nome"`
	Candidati string `json:"candidati,omitempty"`
	Voti      string `json:"voti,omitempty"`
	Percento  string `json:"percentuale,omitempty"`
	Seggi     string `json:"seggi,omitempty"`
}

// CandidatoConListe holds a candidate and their linked party lists.
type CandidatoConListe struct {
	Numero   string  `json:"numero"`
	Nome     string  `json:"nome"`
	Voti     string  `json:"voti,omitempty"`
	Percento string  `json:"percentuale,omitempty"`
	Liste    []Lista `json:"liste"`
}

// RiportoListe holds the full vote-by-list report for a comune.
type RiportoListe struct {
	Comune    string              `json:"comune"`
	Provincia string              `json:"provincia"`
	Anno      int                 `json:"anno"`
	Stato     ScrutinioState      `json:"stato_scrutinio"`
	Dettaglio string              `json:"dettaglio_scrutinio,omitempty"`
	Candidati []CandidatoConListe `json:"candidati"`
}

var (
	tableRe = htmlparse.TableRe
	wsRe    = htmlparse.WsRe
)

// FetchListe fetches and parses the liste page for a comune.
func FetchListe(comune *Comune, anno int) (*RiportoListe, string, error) {
	base := ArchiveURL(anno)
	url := fmt.Sprintf("%s/%s/ReportCandidatiListe%s%s.html", base, comune.Provincia, comune.Provincia, comune.Codice)

	body, err := Fetch(url)
	if err != nil {
		return nil, url, err
	}
	if IsUnavailable(body) {
		return nil, url, fmt.Errorf("dati non disponibili per %s %d", comune.Nome, anno)
	}

	r, err := parseListe(body, comune, anno)
	return r, url, err
}

// parseListe uses the alternating table structure:
// odd tables = candidate info, even tables = liste for that candidate.
func parseListe(body string, comune *Comune, anno int) (*RiportoListe, error) {
	stato, dettaglio := ExtractScrutinioState(body)

	result := &RiportoListe{
		Comune:    comune.Nome,
		Provincia: comune.Provincia,
		Anno:      anno,
		Stato:     stato,
		Dettaglio: dettaglio,
	}

	tables := tableRe.FindAllStringSubmatch(body, -1)

	// Find the first candidate table (contains "Candidato Sindaco" or "Sindaco Eletto").
	// The "Sindaco Eletto" cell uses <br> between the two words, so whitespace must be
	// collapsed before the check.
	startIdx := -1
	for i, t := range tables {
		if !strings.Contains(t[1], "Sindaco") && !strings.Contains(t[1], "Candidato") {
			continue
		}
		rows := trRe.FindAllStringSubmatch(t[1], -1)
		for _, r := range rows {
			normalized := wsRe.ReplaceAllString(cleanCell(r[1]), " ")
			if strings.Contains(normalized, "Candidato Sindaco") || strings.Contains(normalized, "Sindaco Eletto") {
				startIdx = i
				break
			}
		}
		if startIdx >= 0 {
			break
		}
	}

	if startIdx < 0 || startIdx+1 >= len(tables) {
		return result, nil
	}

	// Process alternating pairs: [candTable, listeTable, candTable, listeTable...]
	for i := startIdx; i+1 < len(tables); i += 2 {
		candTable := tables[i][1]
		listeTable := tables[i+1][1]

		cand := parseCandidatoTable(candTable)
		if cand == nil {
			break
		}
		cand.Liste = parseListeTable(listeTable)
		result.Candidati = append(result.Candidati, *cand)
	}

	return result, nil
}

// parseCandidatoTable extracts candidate info from a table like:
// N° | 1 | Candidato Sindaco | NOME COGNOME | VOTI | 1234 | % | 20,5%
func parseCandidatoTable(tableContent string) *CandidatoConListe {
	rows := trRe.FindAllStringSubmatch(tableContent, -1)
	for _, r := range rows {
		cells := tdRe.FindAllStringSubmatch(r[1], -1)
		vals := make([]string, 0, len(cells))
		for _, c := range cells {
			v := cleanCell(c[1])
			if v != "" {
				vals = append(vals, v)
			}
		}
		// Pattern: N° | numero | Candidato Sindaco/Sindaco Eletto | NOME | VOTI | voti | % | perc
		if len(vals) >= 4 && vals[0] == "N°" {
			cand := &CandidatoConListe{}
			if len(vals) > 1 {
				cand.Numero = vals[1]
			}
			// vals[2] is "Candidato Sindaco" or "Sindaco Eletto"
			if len(vals) > 3 {
				cand.Nome = vals[3]
			}
			if len(vals) > 5 && vals[4] == "VOTI" {
				cand.Voti = vals[5]
			}
			if len(vals) > 7 && vals[6] == "%" {
				cand.Percento = vals[7]
			}
			return cand
		}
	}
	return nil
}

// parseListeTable extracts liste from a table.
// Data rows have: N° | (empty img cell) | Nome Lista | Candidati | Voti | % [| Seggi]
func parseListeTable(tableContent string) []Lista {
	rows := trRe.FindAllStringSubmatch(tableContent, -1)
	var liste []Lista
	headerSeen := false

	for _, r := range rows {
		cells := tdRe.FindAllStringSubmatch(r[1], -1)
		// Collect ALL cell values (including empty, to preserve column positions)
		allVals := make([]string, 0, len(cells))
		for _, c := range cells {
			allVals = append(allVals, cleanCell(c[1]))
		}
		// Non-empty values only
		vals := make([]string, 0, len(allVals))
		for _, v := range allVals {
			if v != "" {
				vals = append(vals, v)
			}
		}

		if len(vals) == 0 {
			continue
		}

		// Skip header row
		if vals[0] == "N°" {
			headerSeen = true
			continue
		}
		if !headerSeen {
			continue
		}
		// Skip total/separator rows
		if vals[0] == "Totale" || vals[0] == "-" {
			continue
		}

		// Data row: first non-empty = numero, next = nome, then candidati, voti, %, [seggi]
		if len(vals) >= 2 {
			l := Lista{
				Numero: vals[0],
				Nome:   vals[1],
			}
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

func formatLista(l Lista) string {
	return fmt.Sprintf("[%s] %s — voti: %s (%s%%)", l.Numero, l.Nome, l.Voti, strings.TrimSuffix(l.Percento, "%"))
}
