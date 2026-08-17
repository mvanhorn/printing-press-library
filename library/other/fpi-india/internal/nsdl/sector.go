package nsdl

import (
	"regexp"
	"strings"
)

// SectorPeriod is one fortnightly report period discovered from the sector
// landing page's dropdown. NSDL renders sector data as one static HTML file
// per fortnight (FPI_Fortnightly_Selection.aspx is only the period picker),
// so syncing history means enumerating this list, not a single GET.
type SectorPeriod struct {
	Label string // e.g. "JUNE 30, 2026"
	Path  string // relative path, e.g. "/web/StaticReports/.../FIIInvestSector_June302026.html"
}

var sectorOptionRE = regexp.MustCompile(`(?is)<option[^>]*value=["']([^"']*)["'][^>]*>([^<]*)</option>`)

// ParseSectorPeriods extracts the fortnightly period list from the landing
// page's <select id="ddlfortnighly"> dropdown.
func ParseSectorPeriods(body []byte) []SectorPeriod {
	matches := sectorOptionRE.FindAllStringSubmatch(string(body), -1)
	out := make([]SectorPeriod, 0, len(matches))
	for _, m := range matches {
		val := strings.TrimSpace(m[1])
		label := strings.TrimSpace(m[2])
		if val == "" || label == "" {
			continue
		}
		path := val
		path = strings.TrimPrefix(path, "~")
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		// The dropdown's raw value is site-root-relative ("~/StaticReports/...")
		// but every actual report path on this host lives under "/web/"
		// ("/web/StaticReports/...", confirmed against the live page). Without
		// this prefix the request 404s and NSDL's error handler redirects in a
		// loop the HTTP client eventually aborts.
		if !strings.HasPrefix(path, "/web/") {
			path = "/web" + path
		}
		out = append(out, SectorPeriod{Label: label, Path: path})
	}
	return out
}

// ParseSectorSnapshot extracts per-sector rows from one dated fortnightly
// report page. Reuses the generic largest-table extractor: the sector page's
// header is a four-level colspan grid (period group / currency / asset
// class / leaf), so composite naming is the practical path rather than a
// bespoke positional parser for every historical layout revision.
func ParseSectorSnapshot(body []byte) ([]map[string]string, error) {
	return ParseGenericRecords(body)
}
