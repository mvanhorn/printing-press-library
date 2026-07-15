// Package gaclient is a hand-written client for the public "Decisioni e Pareri"
// search of the Italian administrative justice portal (giustizia-amministrativa.it).
//
// The site is a Liferay portal: searching requires a session handshake
// (a CSRF p_auth token + affinity cookies obtained from the form page) and the
// results come back as HTML, not JSON. This package performs that handshake,
// replays the search over plain HTTP, parses the result rows, and fetches the
// full text of a single provvedimento. No browser is needed at runtime.
package gaclient

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/giustizia-amministrativa/internal/cliutil"
)

// Provvedimento is a single ruling/opinion (sentenza, ordinanza, decreto, parere).
// JSON tags are snake_case to match the generated store schema.
type Provvedimento struct {
	Idprovv      string `json:"idprovv"`
	Ecli         string `json:"ecli"`
	Tipo         string `json:"tipo"`
	Sede         string `json:"sede"`
	Schema       string `json:"schema"`
	Sezione      string `json:"sezione"`
	Numero       int    `json:"numero"`
	Anno         int    `json:"anno"`
	Nrg          string `json:"nrg"`
	DataDeposito string `json:"data_deposito"`
	Snippet      string `json:"snippet"`
	Formato      string `json:"formato"`
	NomeFile     string `json:"nome_file"`
	URL          string `json:"url"`
	FullText     string `json:"full_text,omitempty"`
}

var (
	reArticle  = regexp.MustCompile(`(?s)<article class="ricerca--item">(.*?)</article>`)
	reIdprovv  = regexp.MustCompile(`data-idprovv="([^"]*)"`)
	reDataSede = regexp.MustCompile(`data-sede="([^"]*)"`)
	reDataNrg  = regexp.MustCompile(`data-nrg="([^"]*)"`)
	reNomeFile = regexp.MustCompile(`nomeFile=([^"&]+)`)
	reHrefDoc  = regexp.MustCompile(`href="(https://mdp\.giustizia-amministrativa\.it/visualizza[^"]*)"`)
	reEcli     = regexp.MustCompile(`ECLI:[A-Z0-9:.\-]+`)
	reNumRic   = regexp.MustCompile(`(?s)Numero ricorso:\s*<b>([^<]+)</b>`)
	reSnippet  = regexp.MustCompile(`(?s)<div class="col-sm-12 snippet">(.*?)</div>`)
	// "<b>SENTENZA</b> sede di <b>ROMA</b>, sezione <b>SEZIONE 5T</b>, numero provv.: <b>202611307</b>"
	reDescr = regexp.MustCompile(`(?s)<b>(SENTENZA|ORDINANZA|DECRETO|PARERE|[A-ZÀ-Ù ]+)</b>\s*sede di\s*<b>([^<]*)</b>,\s*sezione\s*<b>([^<]*)</b>,\s*numero provv\.:\s*<b>([^<]*)</b>`)
	// "...202611307 (ROMA, SEZIONE 5T) html" inside the highlighted link text
	reFormato = regexp.MustCompile(`\)\s*(html|pdf)\s*<`)
	reTotal   = regexp.MustCompile(`Trovati\s+([0-9]+)\s+risultati`)
	reRange   = regexp.MustCompile(`Risultati da\s+([0-9]+)\s+a\s+([0-9]+)\s+di\s+([0-9]+)`)
	reTags    = regexp.MustCompile(`<[^>]+>`)
)

// ParseTotal returns the total number of results reported by the search page.
func ParseTotal(htmlBody string) int {
	if m := reTotal.FindStringSubmatch(htmlBody); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	if m := reRange.FindStringSubmatch(htmlBody); m != nil {
		n, _ := strconv.Atoi(m[3])
		return n
	}
	return 0
}

// ParseResults extracts the provvedimenti rows from a search results HTML page.
func ParseResults(htmlBody string) []Provvedimento {
	var out []Provvedimento
	for _, m := range reArticle.FindAllStringSubmatch(htmlBody, -1) {
		block := m[1]
		// Skip the pagination footer article (no result row markers).
		if !strings.Contains(block, "data-idprovv") {
			continue
		}
		p := parseItem(block)
		if p.Ecli != "" || p.Idprovv != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseItem(block string) Provvedimento {
	var p Provvedimento
	if m := reIdprovv.FindStringSubmatch(block); m != nil {
		p.Idprovv = m[1]
	}
	if m := reDataSede.FindStringSubmatch(block); m != nil {
		p.Schema = m[1]
	}
	if m := reDataNrg.FindStringSubmatch(block); m != nil {
		p.Nrg = m[1]
	}
	if m := reNomeFile.FindStringSubmatch(block); m != nil {
		p.NomeFile = m[1]
	}
	if m := reHrefDoc.FindStringSubmatch(block); m != nil {
		// Normalize to the highlighted full-text endpoint for a stable public URL.
		p.URL = normalizeDocURL(m[1])
	}
	if m := reEcli.FindString(block); m != "" {
		p.Ecli = strings.TrimSpace(m)
	}
	if m := reDescr.FindStringSubmatch(block); m != nil {
		p.Tipo = normalizeTipo(strings.TrimSpace(m[1]))
		p.Sede = cliutil.CleanText(m[2])
		p.Sezione = cliutil.CleanText(m[3])
		numero := strings.TrimSpace(m[4])
		p.Anno, p.Numero = splitAnnoNumero(numero)
	}
	if p.Anno == 0 && p.Nrg != "" {
		// Fall back to deriving the year from the NRG (YYYYNNNNN).
		p.Anno, _ = splitAnnoNumero(p.Nrg)
	}
	if m := reFormato.FindStringSubmatch(block); m != nil {
		p.Formato = m[1]
	}
	if m := reSnippet.FindStringSubmatch(block); m != nil {
		p.Snippet = cliutil.CleanText(reTags.ReplaceAllString(m[1], ""))
	}
	if m := reNumRic.FindStringSubmatch(block); m != nil && p.Nrg == "" {
		p.Nrg = strings.TrimSpace(m[1])
	}
	return p
}

// normalizeDocURL rewrites the printable "/visualizza/" variant to the
// "/visualizzah2/" highlighted full-text endpoint, which both serve the same
// public document.
func normalizeDocURL(u string) string {
	return strings.Replace(u, "/visualizza/?", "/visualizzah2/?", 1)
}

func normalizeTipo(t string) string {
	t = strings.ToUpper(strings.TrimSpace(t))
	switch t {
	case "SENTENZA":
		return "Sentenza"
	case "ORDINANZA":
		return "Ordinanza"
	case "DECRETO":
		return "Decreto"
	case "PARERE":
		return "Parere"
	}
	// Adunanza Plenaria / Generale and others: title-case-ish.
	return strings.Title(strings.ToLower(t)) //nolint:staticcheck // simple display title for Italian label
}

// splitAnnoNumero splits a portal number string "YYYYNNNNN" into year and
// sequential number (e.g. "202611307" -> 2026, 11307).
func splitAnnoNumero(s string) (anno int, numero int) {
	s = strings.TrimSpace(s)
	if len(s) < 5 {
		n, _ := strconv.Atoi(s)
		return 0, n
	}
	anno, _ = strconv.Atoi(s[:4])
	numero, _ = strconv.Atoi(strings.TrimLeft(s[4:], "0"))
	return anno, numero
}
