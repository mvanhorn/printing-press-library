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

var statuteCiteRe = regexp.MustCompile(`(?i)\b(?:Cal\.\s*)?(?:(Code\s+Civ\.\s*Proc\.|Civ\.\s*Proc\.\s*Code)|(Lab\.|Labor|Gov\.|Government|Civ\.|Civil|Evid\.|Evidence|Pen\.|Penal|Prob\.|Probate|Fam\.|Family|Bus\.\s*(?:&|and)\s*Prof\.|Health\s*(?:&|and)\s*Saf\.|Ins\.|Insurance|Veh\.|Vehicle|Corp\.|Corporations)\s*Code),?\s*(?:§§?|sections?)\s*(\d+(?:\.\d+)?[a-z]?)`)

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
	for _, m := range statuteCiteRe.FindAllStringSubmatch(text, -1) {
		raw := m[1]
		if raw == "" {
			raw = m[2]
		}
		key := nonAlnum.ReplaceAllString(strings.ToLower(raw), "")
		key = strings.TrimSuffix(key, "code")
		name, ok := codeNames[key]
		if !ok {
			continue
		}
		add(fmt.Sprintf("Cal %s Code %s", name, m[3]), "statute")
	}
	out := make([]briefCite, 0, len(order))
	for _, k := range order {
		out = append(out, *seen[k])
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

func briefCites(path string) ([]briefCite, error) {
	var text string
	var err error
	if path == "-" {
		b, e := io.ReadAll(os.Stdin)
		text, err = string(b), e
	} else {
		text, err = readDocText(path)
	}
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
