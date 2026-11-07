// Copyright 2026 Luigi Pastore and contributors. Licensed under Apache-2.0. See LICENSE.

package norma

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Citation is one reference to an Italian State act found in text, e.g.
// "art. 2043 c.c." or "L. 7 agosto 1990, n. 241, art. 3".
type Citation struct {
	Raw      string `json:"raw"`
	Start    int    `json:"-"`
	End      int    `json:"-"`
	Fonte    string `json:"fonte,omitempty"` // key of a well-known act, when matched
	Tipo     string `json:"tipo,omitempty"`  // URN act type
	Data     string `json:"data,omitempty"`  // YYYY-MM-DD or YYYY
	Numero   string `json:"numero,omitempty"`
	Articolo string `json:"articolo,omitempty"`
	Comma    string `json:"comma,omitempty"`
}

// URN builds the URN:NIR for the citation (article-level when an article
// was cited).
func (c Citation) URN() URN {
	var u URN
	if c.Fonte != "" {
		for _, f := range Fonti {
			if f.Key == c.Fonte {
				u = f.Base()
				break
			}
		}
	} else {
		u = URN{Tipo: c.Tipo, Data: c.Data, Numero: c.Numero}
		// A numbered citation of a well-known act ("R.D. 262/1942") gets the
		// verified URN, including the annex that holds a code's articles.
		if f, ok := fonteFor(c.Tipo, c.Numero, c.Data); ok {
			u = f.Base()
		}
	}
	if c.Articolo != "" {
		u = u.WithArticle(c.Articolo)
	}
	return u
}

// Label is a compact human rendering, e.g. "art. 2043 c.c." or "art. 3 L. 241/1990".
func (c Citation) Label() string {
	act := ""
	if c.Fonte != "" {
		for _, f := range Fonti {
			if f.Key == c.Fonte {
				act = f.Nome
				if len(f.Abbreviazioni) > 0 {
					act = f.Abbreviazioni[0]
				}
			}
		}
	} else {
		year := c.Data
		if len(year) > 4 {
			year = year[:4]
		}
		act = fmt.Sprintf("%s %s/%s", tipoAbbrev(c.Tipo), c.Numero, year)
	}
	if c.Articolo == "" {
		return act
	}
	return fmt.Sprintf("art. %s %s", DisplayArticle(c.Articolo), act)
}

func tipoAbbrev(tipo string) string {
	switch tipo {
	case "legge":
		return "L."
	case "decreto.legislativo":
		return "D.Lgs."
	case "decreto.legge":
		return "D.L."
	case "decreto.del.presidente.della.repubblica":
		return "D.P.R."
	case "regio.decreto":
		return "R.D."
	case "legge.costituzionale":
		return "L. cost."
	}
	return strings.ToUpper(tipo)
}

var mesi = map[string]string{"gennaio": "01", "febbraio": "02", "marzo": "03", "aprile": "04", "maggio": "05", "giugno": "06",
	"luglio": "07", "agosto": "08", "settembre": "09", "ottobre": "10", "novembre": "11", "dicembre": "12"}

const tipoPattern = `(l\.\s*cost\.|legge\s+costituzionale|d\.\s?lgs\.?|dlgs\.?|decreto\s+legislativo|d\.\s?l\.|decreto[- ]legge|d\.\s?p\.\s?r\.?|dpr|decreto\s+del\s+presidente\s+della\s+repubblica|r\.\s?d\.|regio\s+decreto|l\.|legge)`

const suffixPattern = `bis|ter|quater|quinquies|sexies|septies|octies|novies|nonies|decies|undecies|duodecies|terdecies|quaterdecies|quinquiesdecies|sexiesdecies|septiesdecies|duodevicies|undevicies|vicies`

var (
	actRe = regexp.MustCompile(`(?i)\b` + tipoPattern + `\s*(?:` +
		`(\d{1,2})\s+(gennaio|febbraio|marzo|aprile|maggio|giugno|luglio|agosto|settembre|ottobre|novembre|dicembre)\s+(\d{4}),?\s*n\.?\s*(\d+)` +
		`|n\.?\s*(\d+)\s*(?:/|,?\s*del\s+)(\d{4})` +
		`|(\d+)\s*/\s*(\d{4})` +
		`)`)
	artRe  = regexp.MustCompile(`(?i)\bart(?:icol[oi])?\.?\s*(\d+)(?:\s*[-\s]\s*(` + suffixPattern + `)\b|(` + suffixPattern + `)\b)?(?:\s*,?\s*comm[ai]\s*(\d+(?:[-\s]?(?:` + suffixPattern + `))?))?`)
	codeRe *regexp.Regexp

	numSufPattern = `\d+(?:\s*-?\s*(?:` + suffixPattern + `)\b)?`
	artListRe     = regexp.MustCompile(`(?i)\b(?:artt\.?|articoli)\s*(` + numSufPattern + `(?:\s*(?:,|\be\b|\bed\b)\s*` + numSufPattern + `)*)`)
	artListSepRe  = regexp.MustCompile(`(?i)\s*(?:,|\be\b|\bed\b)\s*`)

	leadingArticleRe = regexp.MustCompile(`^(\d+(?:-?(?:` + suffixPattern + `))?)\s+(\D.*)$`)
)

func init() {
	var alts []string
	seen := map[string]bool{}
	for _, f := range Fonti {
		for _, a := range append(append([]string{}, f.Abbreviazioni...), f.Alias...) {
			// Bare words like "privacy" or "231" are too ambiguous inside free text.
			if seen[a] || !(strings.ContainsAny(a, ". ") || (len(a) >= 10 && !strings.ContainsAny(a, "0123456789"))) {
				continue
			}
			seen[a] = true
			alts = append(alts, regexp.QuoteMeta(a))
		}
	}
	sort.Slice(alts, func(i, j int) bool { return len(alts[i]) > len(alts[j]) })
	codeRe = regexp.MustCompile(`(?i)(?:^|[^\pL])(` + strings.Join(alts, "|") + `)(?:[^\pL]|$)`)
}

func tipoFromToken(tok string) string {
	t := strings.ToLower(strings.Join(strings.Fields(tok), ""))
	t = strings.ReplaceAll(t, "-", "")
	switch {
	case strings.HasPrefix(t, "l.cost") || strings.HasPrefix(t, "leggecostituzionale"):
		return "legge.costituzionale"
	case strings.HasPrefix(t, "d.lgs") || strings.HasPrefix(t, "dlgs") || strings.HasPrefix(t, "decretolegislativo"):
		return "decreto.legislativo"
	case t == "d.l." || strings.HasPrefix(t, "decretolegge"):
		return "decreto.legge"
	case strings.HasPrefix(t, "d.p.r") || t == "dpr" || strings.HasPrefix(t, "decretodelpresidente"):
		return "decreto.del.presidente.della.repubblica"
	case t == "r.d." || strings.HasPrefix(t, "regiodecreto"):
		return "regio.decreto"
	case t == "l." || t == "legge":
		return "legge"
	}
	return ""
}

type span struct {
	start, end int
	cit        Citation
}

func findActs(text string) []span {
	var out []span
	for _, m := range actRe.FindAllStringSubmatchIndex(text, -1) {
		g := func(i int) string {
			if m[2*i] < 0 {
				return ""
			}
			return text[m[2*i]:m[2*i+1]]
		}
		c := Citation{Tipo: tipoFromToken(g(1))}
		switch {
		case g(5) != "":
			day, _ := strconv.Atoi(g(2))
			c.Data = fmt.Sprintf("%s-%s-%02d", g(4), mesi[strings.ToLower(g(3))], day)
			c.Numero = g(5)
		case g(6) != "":
			c.Numero, c.Data = g(6), g(7)
		default:
			c.Numero, c.Data = g(8), g(9)
		}
		if c.Tipo == "" {
			continue
		}
		c.Numero = strings.TrimLeft(c.Numero, "0")
		out = append(out, span{m[0], m[1], c})
	}
	for _, m := range codeRe.FindAllStringSubmatchIndex(text, -1) {
		tok := text[m[2]:m[3]]
		f, ok := LookupFonte(tok)
		if !ok {
			continue
		}
		overlap := false
		for _, a := range out {
			if m[2] < a.end && m[3] > a.start {
				overlap = true
			}
		}
		if !overlap {
			out = append(out, span{m[2], m[3], Citation{Fonte: f.Key}})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].start < out[j].start })
	return out
}

// ExtractCitations finds every act and article citation in free text. An
// article is attached to the act right after it ("art. 3 L. 241/1990",
// "art. 2043 del codice civile") or, failing that, right before it
// ("D.Lgs. 196/2003, art. 13"). Acts cited without an article are returned
// as act-level citations.
func ExtractCitations(text string) []Citation {
	acts := findActs(text)
	used := make([]bool, len(acts))
	var out []Citation
	type artMatch struct {
		start, end int
		arts       []string
		comma      string
	}
	var arts []artMatch
	for _, m := range artListRe.FindAllStringSubmatchIndex(text, -1) {
		var list []string
		for _, n := range artListSepRe.Split(text[m[2]:m[3]], -1) {
			if n = strings.TrimSpace(n); n != "" {
				list = append(list, n)
			}
		}
		arts = append(arts, artMatch{start: m[0], end: m[1], arts: list})
	}
	for _, m := range artRe.FindAllStringSubmatchIndex(text, -1) {
		overlap := false
		for _, a := range arts {
			if m[0] < a.end && m[1] > a.start {
				overlap = true
			}
		}
		if overlap {
			continue
		}
		g := func(k int) string {
			if m[2*k] < 0 {
				return ""
			}
			return text[m[2*k]:m[2*k+1]]
		}
		arts = append(arts, artMatch{start: m[0], end: m[1], arts: []string{g(1) + strings.ToLower(g(2)+g(3))}, comma: g(4)})
	}
	sort.Slice(arts, func(i, j int) bool { return arts[i].start < arts[j].start })
	for i, am := range arts {
		m := []int{am.start, am.end}
		nextArt := len(text)
		if i+1 < len(arts) {
			nextArt = arts[i+1].start
		}
		best := -1
		// Forward: act starting shortly after the article, before the next article.
		for j, a := range acts {
			if a.start >= m[1] && a.start-m[1] <= 40 && a.start < nextArt {
				gap := strings.ToLower(text[m[1]:a.start])
				if !strings.ContainsAny(gap, ".;\n") {
					best = j
					break
				}
			}
		}
		if best < 0 {
			// Backward: closest act ending shortly before the article.
			for j := len(acts) - 1; j >= 0; j-- {
				a := acts[j]
				if a.end <= m[0] && m[0]-a.end <= 25 && !strings.ContainsAny(text[a.end:m[0]], ";\n") {
					best = j
					break
				}
			}
		}
		if best < 0 {
			continue
		}
		used[best] = true
		start, end := m[0], acts[best].end
		if acts[best].start < m[0] {
			start, end = acts[best].start, m[1]
		}
		for _, art := range am.arts {
			c := acts[best].cit
			c.Articolo, c.Comma = NormalizeArticle(art), am.comma
			c.Raw, c.Start, c.End = strings.TrimSpace(text[start:end]), start, end
			out = append(out, c)
		}
	}
	for j, a := range acts {
		if used[j] {
			continue
		}
		c := a.cit
		c.Raw, c.Start, c.End = strings.TrimSpace(text[a.start:a.end]), a.start, a.end
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}

// ParseCitation parses one reference typed by a person: a URN, a well-known
// act name ("codice civile"), or a citation like "d.lgs. 196/2003 art. 13".
// article overrides any article found in s when non-empty.
func ParseCitation(s, article string) (Citation, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Citation{}, fmt.Errorf("empty act reference")
	}
	if IsURN(s) {
		u, err := ParseURN(s)
		if err != nil {
			return Citation{}, err
		}
		c := Citation{Raw: s, Tipo: u.Tipo, Data: u.Data, Numero: u.Numero, Articolo: u.Articolo}
		for _, f := range Fonti {
			if f.URN == u.Act().String() {
				c.Fonte = f.Key
			}
		}
		if article != "" {
			c.Articolo = NormalizeArticle(article)
		}
		return c, nil
	}
	if f, ok := LookupFonte(s); ok {
		c := Citation{Raw: s, Fonte: f.Key}
		if article != "" {
			c.Articolo = NormalizeArticle(article)
		}
		return c, nil
	}
	// "2043 c.c." without the "art." prefix.
	if m := leadingArticleRe.FindStringSubmatch(strings.ToLower(s)); m != nil {
		if c, err := ParseCitation(m[2], m[1]); err == nil {
			if article != "" {
				c.Articolo = NormalizeArticle(article)
			}
			c.Raw = s
			return c, nil
		}
	}
	cits := ExtractCitations(s)
	if len(cits) == 0 {
		return Citation{}, fmt.Errorf("could not recognize an act in %q; try a URN, a name like \"codice civile\" (see 'fonti'), or \"d.lgs. 196/2003\"", s)
	}
	c := cits[0]
	c.Raw = s
	if article != "" {
		c.Articolo = NormalizeArticle(article)
	}
	return c, nil
}

func fonteFor(tipo, numero, data string) (Fonte, bool) {
	if tipo == "" || numero == "" || len(data) < 4 {
		return Fonte{}, false
	}
	for _, f := range Fonti {
		u, err := ParseURN(f.URN)
		if err != nil || u.Tipo != tipo || u.Numero != numero || u.Data[:4] != data[:4] {
			continue
		}
		if len(data) == 10 && u.Data != data {
			continue
		}
		return f, true
	}
	return Fonte{}, false
}
