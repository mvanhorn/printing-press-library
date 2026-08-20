// Package regionali fetches and parses HTML pages for the Sicilian regional
// elections (Assemblea Regionale Siciliana — ARS) from
// www.elezioni.regione.sicilia.it/regionali<YYYY>/.
//
// Pages share structure across years; supported reports:
//   - presidente  → rep_7/riepilogoRegionale.html  (most complete)
//   - affluenza   → rep_6/affluenzaRegionale1.html
//   - seggi       → rep_8/ripartoSeggi.html
//   - listino     → rep_9/listeRegionali.html
//   - candidati   → rep_5/<Province>/votiCandidatiProvincia<XX>.html
package regionali

import (
	"fmt"
	"regexp"

	"github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/internal/scraper"
)

// LogoTableRe collapses nested "<table><tr><td><img></td><td>NAME</td></tr></table>"
// patterns (used for list-with-logo cells) to just NAME, so that the outer
// row parser is not confused by the inner </tr>. Optional <div> wrappers
// cover the rep_7 regional-list-header variant.
var LogoTableRe = regexp.MustCompile(`(?is)<table[^>]*>\s*<tr[^>]*>\s*<td[^>]*>\s*(?:<div[^>]*>\s*)?<img[^>]*>\s*(?:</div>\s*)?</t[dh]>\s*<td[^>]*>\s*(?:<div[^>]*>\s*)?([^<]+?)\s*(?:</div>\s*)?</t[dh]>\s*</tr>\s*</table>`)

// KnownAnni lists the regional election years available on the site.
var KnownAnni = []int{2022, 2017}

// IsKnownAnno reports whether anno is one of the supported years.
func IsKnownAnno(anno int) bool {
	for _, y := range KnownAnni {
		if y == anno {
			return true
		}
	}
	return false
}

// BaseURL returns the root URL for the given regional election year.
func BaseURL(anno int) string {
	return fmt.Sprintf("https://www.elezioni.regione.sicilia.it/regionali%d", anno)
}

// fetch is a thin wrapper around scraper.Fetch to reuse the rate-limited,
// ISO-8859-15-decoding HTTP client.
func fetch(url string) (string, error) {
	return scraper.Fetch(url)
}

// provinceCity maps the two-letter province code to the directory name used
// by rep_5 (votiCandidati): /regionali<YYYY>/rep_5/<City>/votiCandidatiProvincia<XX>.html
var provinceCity = map[string]string{
	"AG": "Agrigento",
	"CL": "Caltanissetta",
	"CT": "Catania",
	"EN": "Enna",
	"ME": "Messina",
	"PA": "Palermo",
	"RG": "Ragusa",
	"SR": "Siracusa",
	"TP": "Trapani",
}

// ProvinceCity returns the directory name for a province code, or empty string
// if the code is not recognised.
func ProvinceCity(prov string) string {
	return provinceCity[prov]
}
