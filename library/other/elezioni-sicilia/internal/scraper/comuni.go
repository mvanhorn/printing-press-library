package scraper

import (
	"fmt"
	"regexp"
	"strings"
)

// Comune represents a municipality in the Sicilian election system.
type Comune struct {
	Nome      string `json:"nome"`
	Provincia string `json:"provincia"`
	Codice    string `json:"codice"`
}

// optionRe matches <option value="ReportDatiListaXX123.html">Name</option>
var optionRe = regexp.MustCompile(`(?i)<option[^>]+value=["']?Report\w+([A-Z]{2})(\d+)\.html["']?[^>]*>\s*([^<]+)\s*</option>`)

// FetchComuni retrieves the list of comuni for a province and year.
func FetchComuni(provincia string, anno int) ([]Comune, error) {
	base := ArchiveURL(anno)
	url := fmt.Sprintf("%s/%s/ReportDatiLista%s.html", base, provincia, provincia)

	body, err := Fetch(url)
	if err != nil {
		return nil, err
	}
	if IsUnavailable(body) {
		return nil, fmt.Errorf("dati non disponibili per l'anno %d", anno)
	}

	return parseComuni(body, provincia), nil
}

func parseComuni(body, provincia string) []Comune {
	matches := optionRe.FindAllStringSubmatch(body, -1)
	var result []Comune
	seen := map[string]bool{}
	for _, m := range matches {
		if len(m) < 4 {
			continue
		}
		prov := m[1]
		codice := m[2]
		nome := strings.TrimSpace(m[3])
		if prov != provincia || nome == "" || seen[codice] {
			continue
		}
		seen[codice] = true
		result = append(result, Comune{Nome: nome, Provincia: prov, Codice: codice})
	}
	return result
}

// FindComune resolves a comune name (case-insensitive, partial match) to a Comune
// from the given province, fetching the list if needed.
func FindComune(query, provincia string, anno int) (*Comune, error) {
	comuni, err := FetchComuni(provincia, anno)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	for _, c := range comuni {
		if strings.ToLower(c.Nome) == q {
			return &c, nil
		}
	}
	// Partial match
	for _, c := range comuni {
		if strings.Contains(strings.ToLower(c.Nome), q) {
			return &c, nil
		}
	}
	return nil, fmt.Errorf("comune %q non trovato in provincia %s per l'anno %d", query, provincia, anno)
}

// ResolveComune finds a comune by name across all provinces (if provincia is "").
func ResolveComune(query, provincia string, anno int) (*Comune, error) {
	if provincia != "" {
		return FindComune(query, strings.ToUpper(provincia), anno)
	}
	// Search all provinces
	for _, prov := range Province {
		c, err := FindComune(query, prov, anno)
		if err == nil {
			return c, nil
		}
	}
	return nil, fmt.Errorf("comune %q non trovato in nessuna provincia per l'anno %d", query, anno)
}
