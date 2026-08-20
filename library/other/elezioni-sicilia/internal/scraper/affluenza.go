package scraper

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/scraper/htmlparse"
)

// AffluenzaRilevamento holds voter turnout data for one time slot.
type AffluenzaRilevamento struct {
	Orario      string `json:"orario"`
	Votanti     string `json:"votanti"`
	Percentuale string `json:"percentuale"`
	PrecPercent string `json:"perc_precedenti,omitempty"`
	Differenza  string `json:"diff_perc,omitempty"`
}

// AffluenzaComune holds voter turnout data for one municipality.
type AffluenzaComune struct {
	Comune      string                 `json:"comune"`
	Provincia   string                 `json:"provincia"`
	Elettori    string                 `json:"elettori"`
	Rilevamenti []AffluenzaRilevamento `json:"rilevamenti"`
}

// Aliases for the shared htmlparse symbols, kept short for call-site brevity.
var (
	cleanCell = htmlparse.CleanCell
	trRe      = htmlparse.TrRe
	tdRe      = htmlparse.TdRe
)

var provRe = regexp.MustCompile(`(?i)Provincia di ([A-Z]+)\s*\(([A-Z]+)\)`)

// FetchAffluenza fetches and parses the regional affluenza table for a given year.
func FetchAffluenza(anno int) ([]AffluenzaComune, string, error) {
	base := ArchiveURL(anno)
	url := fmt.Sprintf("%s/ReportTabellaAffluenza.html", base)

	body, err := Fetch(url)
	if err != nil {
		return nil, url, err
	}
	if IsUnavailable(body) {
		return nil, url, fmt.Errorf("dati non disponibili per l'anno %d", anno)
	}

	records, err := parseAffluenza(body)
	return records, url, err
}

func parseAffluenza(body string) ([]AffluenzaComune, error) {
	// Extract all <tr> rows and collect cells
	rows := trRe.FindAllStringSubmatch(body, -1)

	// Find the header row (contains "Comune" and "Elettori")
	headerRow := -1
	var orari []string
	for i, r := range rows {
		rowText := cleanCell(r[1])
		if strings.Contains(rowText, "Elettori") && strings.Contains(rowText, "Votanti") {
			// Extract orari from the header — only from "Votanti" cells
			cells := tdRe.FindAllStringSubmatch(r[1], -1)
			for _, c := range cells {
				txt := cleanCell(c[1])
				if strings.HasPrefix(txt, "Votanti") && strings.Contains(txt, "ore") {
					orario := extractDateOrario(txt)
					if orario != "" {
						orari = append(orari, orario)
					}
				}
			}
			headerRow = i
			break
		}
	}

	if headerRow < 0 {
		return nil, fmt.Errorf("intestazione affluenza non trovata")
	}

	var result []AffluenzaComune
	currentProv := ""

	for _, r := range rows[headerRow+1:] {
		cells := tdRe.FindAllStringSubmatch(r[1], -1)
		if len(cells) == 0 {
			continue
		}

		// Check if it's a province header row
		rowText := cleanCell(r[1])
		if m := provRe.FindStringSubmatch(rowText); len(m) > 0 {
			currentProv = m[2]
			continue
		}

		// Expect data rows: Comune | Elettori | Votanti% x4 | Comune
		vals := make([]string, 0, len(cells))
		for _, c := range cells {
			vals = append(vals, cleanCell(c[1]))
		}

		// Skip rows with too few cells or empty comune
		if len(vals) < 3 || vals[0] == "" || vals[0] == "Comune" {
			continue
		}

		// Skip province subtotal rows (they don't have "Comune" in first cell after the province header)
		nome := vals[0]
		if nome == "" || strings.Contains(strings.ToLower(nome), "totale") ||
			strings.Contains(strings.ToLower(nome), "provincia") {
			continue
		}

		ac := AffluenzaComune{
			Comune:    nome,
			Provincia: currentProv,
		}

		// Second cell is Elettori
		if len(vals) > 1 {
			ac.Elettori = vals[1]
		}

		// Then groups of 4 cells per rilevamento: Votanti, %, PrecPrec, Diff
		colStart := 2
		for i, orario := range orari {
			base := colStart + i*4
			if base+3 >= len(vals) {
				break
			}
			ril := AffluenzaRilevamento{
				Orario:      orario,
				Votanti:     vals[base],
				Percentuale: vals[base+1],
				PrecPercent: vals[base+2],
				Differenza:  vals[base+3],
			}
			ac.Rilevamenti = append(ac.Rilevamenti, ril)
		}

		result = append(result, ac)
	}

	return result, nil
}

// extractOrario extracts "HH:MM" from a string like "Votanti 24/5/2026 ore 12:00".
func extractOrario(s string) string {
	idx := strings.Index(s, "ore ")
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(s[idx+4:])
	if len(rest) >= 5 {
		return rest[:5]
	}
	return rest
}

// extractDateOrario extracts "24/5 12:00" from "Votanti 24/5/2026 ore 12:00".
var dateOrarioRe = regexp.MustCompile(`(\d+/\d+)/\d{4}\s+ore\s+(\d{2}:\d{2})`)

func extractDateOrario(s string) string {
	m := dateOrarioRe.FindStringSubmatch(s)
	if len(m) >= 3 {
		return m[1] + " " + m[2]
	}
	return extractOrario(s)
}
