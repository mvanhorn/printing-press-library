// Copyright 2026 Luigi Pastore and contributors. Licensed under Apache-2.0. See LICENSE.

// Package norma holds the offline logic behind the Normattiva commands:
// URN:NIR identifiers, citation parsing, the table of well-known acts,
// article HTML rendering and text diffs. Nothing here touches the network.
package norma

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// URN is a parsed URN:NIR identifier for a State act, optionally narrowed to
// an article and pinned to a version.
//
//	urn:nir:stato:<tipo>:<data>;<numero>[:<allegato>][~art<articolo>][@originale | !vig=<YYYY-MM-DD>]
type URN struct {
	Tipo      string // e.g. "legge", "decreto.legislativo", "costituzione"
	Data      string // "YYYY-MM-DD" or "YYYY"
	Numero    string // act number; empty for the Constitution
	Allegato  string // annex holding the articles (codici), e.g. "2" for the codice civile
	Articolo  string // article number with suffix, e.g. "3", "21octies"
	Originale bool   // @originale: text as first published
	Vigenza   string // !vig=YYYY-MM-DD: text in force on that date
}

var urnRe = regexp.MustCompile(`^urn:nir:stato:([a-z.]+):(\d{4}(?:-\d{2}-\d{2})?)(?:;([0-9a-z]+))?(?::([0-9a-z]+))?(?:~art([0-9]+[a-z]*))?(?:(@originale)|!vig=(\d{4}-\d{2}-\d{2}))?$`)

// ParseURN parses a URN:NIR string. Matching is case-insensitive.
func ParseURN(s string) (URN, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	m := urnRe.FindStringSubmatch(s)
	if m == nil {
		return URN{}, fmt.Errorf("not a valid URN:NIR for a State act: %q (expected urn:nir:stato:<tipo>:<data>;<numero>[~artN])", s)
	}
	return URN{Tipo: m[1], Data: m[2], Numero: m[3], Allegato: m[4], Articolo: m[5], Originale: m[6] != "", Vigenza: m[7]}, nil
}

// IsURN reports whether s looks like a URN:NIR string.
func IsURN(s string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(s)), "urn:nir:")
}

// String renders the URN back to its canonical form.
func (u URN) String() string {
	var b strings.Builder
	b.WriteString("urn:nir:stato:")
	b.WriteString(u.Tipo)
	b.WriteString(":")
	b.WriteString(u.Data)
	if u.Numero != "" {
		b.WriteString(";")
		b.WriteString(u.Numero)
	}
	if u.Allegato != "" && u.Articolo != "" {
		b.WriteString(":")
		b.WriteString(u.Allegato)
	}
	if u.Articolo != "" {
		b.WriteString("~art")
		b.WriteString(u.Articolo)
	}
	switch {
	case u.Originale:
		b.WriteString("@originale")
	case u.Vigenza != "":
		b.WriteString("!vig=")
		b.WriteString(u.Vigenza)
	}
	return b.String()
}

// Act returns the URN without article and version.
func (u URN) Act() URN {
	u.Articolo, u.Originale, u.Vigenza = "", false, ""
	return u
}

// WithArticle returns a copy narrowed to the given article ("2043", "21-octies", "3 bis").
func (u URN) WithArticle(art string) URN {
	u.Articolo = NormalizeArticle(art)
	return u
}

// WithVigenza pins the URN to the text in force on date (YYYY-MM-DD). An empty
// date clears the pin.
func (u URN) WithVigenza(date string) URN {
	u.Vigenza, u.Originale = date, false
	return u
}

// WithOriginale pins the URN to the originally published text.
func (u URN) WithOriginale() URN {
	u.Vigenza, u.Originale = "", true
	return u
}

// WebURL is the public normattiva.it permalink for this URN.
func (u URN) WebURL() string {
	return "https://www.normattiva.it/uri-res/N2Ls?" + u.String()
}

var articleRe = regexp.MustCompile(`^(\d+)\s*[-.]?\s*([a-z]*)$`)

// NormalizeArticle turns "21-octies", "21 octies", "Art. 3", "3-BIS" into the
// URN article form ("21octies", "3", "3bis"). Unrecognized input is returned
// lowercased with spaces removed.
func NormalizeArticle(art string) string {
	a := strings.ToLower(strings.TrimSpace(art))
	for _, p := range []string{"articoli", "articolo", "artt.", "artt"} {
		a = strings.TrimPrefix(a, p)
	}
	a = strings.TrimPrefix(a, "art.")
	a = strings.TrimPrefix(a, "art")
	a = strings.TrimSpace(a)
	if m := articleRe.FindStringSubmatch(a); m != nil {
		return m[1] + m[2]
	}
	return strings.ReplaceAll(a, " ", "")
}

// DisplayArticle renders a URN article ("21octies") for humans ("21-octies").
func DisplayArticle(art string) string {
	if m := articleRe.FindStringSubmatch(art); m != nil && m[2] != "" {
		return m[1] + "-" + m[2]
	}
	return art
}

// ArticleSuffixes are the Latin ordinal suffixes used for inserted articles,
// in order.
var ArticleSuffixes = []string{"bis", "ter", "quater", "quinquies", "sexies", "septies", "octies", "novies", "decies", "undecies", "duodecies", "terdecies", "quaterdecies", "quinquiesdecies", "sexiesdecies", "septiesdecies", "duodevicies", "undevicies", "vicies"}

// TipoLabel maps a URN act type to the label Normattiva uses in search
// results and act titles.
var TipoLabel = map[string]string{
	"legge":               "LEGGE",
	"decreto.legislativo": "DECRETO LEGISLATIVO",
	"decreto.legge":       "DECRETO-LEGGE",
	"decreto.del.presidente.della.repubblica": "DECRETO DEL PRESIDENTE DELLA REPUBBLICA",
	"regio.decreto": "REGIO DECRETO",
	"costituzione":  "COSTITUZIONE",
	"decreto":       "DECRETO",
	"decreto.del.presidente.del.consiglio.dei.ministri": "DECRETO DEL PRESIDENTE DEL CONSIGLIO DEI MINISTRI",
	"legge.costituzionale":                              "LEGGE COSTITUZIONALE",
	"decreto.legislativo.luogotenenziale":               "DECRETO LEGISLATIVO LUOGOTENENZIALE",
	"regio.decreto.legge":                               "REGIO DECRETO-LEGGE",
}

// TipoFromLabel is the inverse of TipoLabel ("DECRETO-LEGGE" -> "decreto.legge").
func TipoFromLabel(label string) string {
	l := strings.ToUpper(strings.TrimSpace(label))
	for k, v := range TipoLabel {
		if v == l {
			return k
		}
	}
	return ""
}

// ParseVigenzaDate parses the API's YYYYMMDD vigenza dates. "99999999" means
// open-ended and returns ok=false with open=true.
func ParseVigenzaDate(s string) (t time.Time, open bool, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false, false
	}
	if s == "99999999" {
		return time.Time{}, true, false
	}
	t, err := time.Parse("20060102", s)
	if err != nil {
		return time.Time{}, false, false
	}
	return t, false, true
}

// FormatVigenzaDate renders YYYYMMDD as YYYY-MM-DD; open-ended becomes "".
func FormatVigenzaDate(s string) string {
	t, _, ok := ParseVigenzaDate(s)
	if !ok {
		return ""
	}
	return t.Format("2006-01-02")
}

// ParseDate accepts YYYY-MM-DD, DD/MM/YYYY, DD-MM-YYYY, "oggi"/"today".
func ParseDate(s string, now time.Time) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "", "oggi", "today", "vigente":
		return now.Format("2006-01-02"), nil
	}
	for _, layout := range []string{"2006-01-02", "02/01/2006", "02-01-2006", "2/1/2006", "20060102"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("unrecognized date %q (use YYYY-MM-DD, DD/MM/YYYY or 'oggi')", s)
}
