// Hand-written: citation extraction and .docx text helpers (no network, no state).

package cli

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var caseCiteRe = regexp.MustCompile(`\b(\d{1,3})\s+(Cal\.\s?(?:App\.\s?)?(?:Supp\.\s?)?(?:[2-5](?:d|th)\s?)?|Cal\.\s?Rptr\.\s?(?:[23]d\s?)?|P\.\s?(?:[23]d\s?)?)(\d{1,4})\b`)

// statuteCiteRe captures the code name and the section marker. Group 3 is the marker: "§§" and
// "sections" introduce a list ("§§ 1102.5, 1194, and 1198.5", "§§ 1102.5-1102.6"), which
// statuteSectionsRe then reads in full so no section of the list is dropped.
var statuteCiteRe = regexp.MustCompile(`(?i)\b(?:Cal\.\s*)?(?:(Code\s+Civ\.\s*Proc\.|Civ\.\s*Proc\.\s*Code)|(Lab\.|Labor|Gov\.|Government|Civ\.|Civil|Evid\.|Evidence|Pen\.|Penal|Prob\.|Probate|Fam\.|Family|Bus\.\s*(?:&|and)\s*Prof\.|Health\s*(?:&|and)\s*Saf\.|Ins\.|Insurance|Veh\.|Vehicle|Corp\.|Corporations)\s*Code),?\s*(§§|§|sections|section)\s*(\d+(?:\.\d+)?[a-z]?)`)

// statuteListTailRe reads the rest of a plural section list after its first section: an optional
// subdivision "(a)", then ", 1194", ", and 1198.5", " & 1199", "-1102.6", or "–1102.6".
var statuteListTailRe = regexp.MustCompile(`^(?:\([A-Za-z0-9]+\))*(?:\s*,\s*(?:and\s+|or\s+|&\s*)?|\s+and\s+|\s+or\s+|\s*&\s*|\s*[-\x{2013}]\s*)(\d+(?:\.\d+)?[a-z]?)`)

// statuteProseRe spots a quantity right after a number ("30 days", "50 percent", "$5"), which
// means the number is prose and the section list has ended. Anything else after a number
// ("1194 applies", "1194.") leaves it counted as a section.
var statuteProseRe = regexp.MustCompile(`(?i)^\s*(?:%|(?:calendar\s+|business\s+|working\s+)?(?:days?|weeks?|months?|years?|hours?|minutes?|percent|dollars?|cents?|times|employees?|workers?|people|persons?|per)\b)`)

// reporterAheadRe spots a case reporter right after a number, so "§§ 1102.5, 98 Cal.App.4th 1"
// is not read as sections 1102.5 and 98.
var reporterAheadRe = regexp.MustCompile(`^\s*(?:Cal\.|P\.|Cal\s|F\.|U\.S\.|S\.\s?Ct\.)`)

var codeNames = map[string]string{
	"codecivproc": "Civ Proc", "civproc": "Civ Proc", "lab": "Lab", "labor": "Lab", "gov": "Gov", "government": "Gov",
	"civ": "Civ", "civil": "Civ", "evid": "Evid", "evidence": "Evid", "pen": "Penal", "penal": "Penal", "prob": "Prob", "probate": "Prob",
	"fam": "Fam", "family": "Fam", "busprof": "Bus & Prof", "businessprof": "Bus & Prof", "healthsaf": "Health & Safety",
	"ins": "Ins", "insurance": "Ins", "veh": "Veh", "vehicle": "Veh", "corp": "Corp", "corporations": "Corp",
}

type briefCite struct {
	Cite string `json:"cite"`
	Kind string `json:"kind"`
	Seen int    `json:"seen"`
}

func extractCites(text string) []briefCite {
	text = strings.Join(strings.Fields(text), " ")
	seen := map[string]*briefCite{}
	var order []string
	add := func(cite, kind string) {
		k := citeKey(cite)
		if c, ok := seen[k]; ok {
			c.Seen++
			return
		}
		seen[k] = &briefCite{Cite: cite, Kind: kind, Seen: 1}
		order = append(order, k)
	}
	for _, m := range caseCiteRe.FindAllStringSubmatch(text, -1) {
		rep := strings.Join(strings.Fields(m[2]), " ")
		add(fmt.Sprintf("%s %s %s", m[1], rep, m[3]), "case")
	}
	for _, m := range statuteCiteRe.FindAllStringSubmatchIndex(text, -1) {
		raw := group(text, m, 1)
		if raw == "" {
			raw = group(text, m, 2)
		}
		key := nonAlnum.ReplaceAllString(strings.ToLower(raw), "")
		key = strings.TrimSuffix(key, "code")
		name, ok := codeNames[key]
		if !ok {
			continue
		}
		for _, sp := range statuteSections(text, m) {
			add(fmt.Sprintf("Cal %s Code %s", name, sp.Sec), "statute")
		}
	}
	out := make([]briefCite, 0, len(order))
	for _, k := range order {
		out = append(out, *seen[k])
	}
	return out
}

func group(text string, m []int, g int) string {
	if 2*g+1 >= len(m) || m[2*g] < 0 {
		return ""
	}
	return text[m[2*g]:m[2*g+1]]
}

// secSpan is one section a citation names and where its number sits in the text.
type secSpan struct {
	Sec        string
	Start, End int
}

// statuteSections returns the sections one statuteCiteRe match names: one for "§ 1102.5", and
// every listed section for a plural marker ("§§ 1102.5, 1194" gives 1102.5 and 1194). A range
// ("§§ 1102.5-1102.6") gives both ends, which is what a brief cites as authority. Each section
// carries the offsets of its number in text, so callers never search the text for it again.
func statuteSections(text string, m []int) []secSpan {
	out := []secSpan{{Sec: group(text, m, 4), Start: m[0], End: m[1]}}
	marker := strings.ToLower(group(text, m, 3))
	if marker != "§§" && marker != "sections" {
		return out
	}
	base := m[1]
	for {
		rest := text[base:]
		t := statuteListTailRe.FindStringSubmatchIndex(rest)
		if t == nil {
			break
		}
		after := rest[t[1]:]
		if reporterAheadRe.MatchString(after) || statuteProseRe.MatchString(after) {
			break // the number starts a case citation, or is prose ("30 days"), not another section
		}
		out = append(out, secSpan{Sec: rest[t[2]:t[3]], Start: base + t[2], End: base + t[3]})
		base += t[1]
	}
	return out
}

// statuteSpans maps each statute citation in text (as extractCites names it) to where it is cited.
// text must already be whitespace-collapsed, as extractCites collapses it.
func statuteSpans(text string) map[string][][2]int {
	out := map[string][][2]int{}
	for _, m := range statuteCiteRe.FindAllStringSubmatchIndex(text, -1) {
		raw := group(text, m, 1)
		if raw == "" {
			raw = group(text, m, 2)
		}
		key := strings.TrimSuffix(nonAlnum.ReplaceAllString(strings.ToLower(raw), ""), "code")
		name, ok := codeNames[key]
		if !ok {
			continue
		}
		for _, sp := range statuteSections(text, m) {
			cite := fmt.Sprintf("Cal %s Code %s", name, sp.Sec)
			out[cite] = append(out[cite], [2]int{sp.Start, sp.End})
		}
	}
	return out
}

func readDocText(path string) (string, error) {
	if strings.EqualFold(filepath.Ext(path), ".docx") {
		zr, err := zip.OpenReader(path)
		if err != nil {
			return "", err
		}
		defer zr.Close()
		for _, f := range zr.File {
			if f.Name == "word/document.xml" {
				rc, err := f.Open()
				if err != nil {
					return "", err
				}
				defer rc.Close()
				b, err := io.ReadAll(rc)
				if err != nil {
					return "", err
				}
				x := string(b)
				x = strings.NewReplacer("</w:p>", "\n", "<w:tab/>", " ", "<w:br/>", "\n").Replace(x)
				x = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(x, "")
				return htmlUnescape(x), nil
			}
		}
		return "", fmt.Errorf("%s has no word/document.xml", path)
	}
	b, err := os.ReadFile(path)
	return string(b), err
}

func htmlUnescape(s string) string {
	return strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", "\"", "&apos;", "'", "&#xA7;", "§").Replace(s)
}

// briefCites lists the citations in a brief. It reads the brief the same way `citecheck packet`
// does (readBriefText), so citations that appear only in footnotes are included.
func briefCites(path string) ([]briefCite, error) {
	text, err := readBriefText(path, os.Stdin)
	if err != nil {
		return nil, err
	}
	return extractCites(text), nil
}

func lineSet(t string) map[string]bool {
	m := map[string]bool{}
	for _, l := range strings.Split(t, "\n") {
		l = strings.Join(strings.Fields(l), " ")
		if len(l) > 20 {
			m[l] = true
		}
	}
	return m
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
