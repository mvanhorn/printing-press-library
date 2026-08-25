// Copyright 2026 roberto-bissanti. Licensed under Apache-2.0. See LICENSE.

// Package comuni embeds a snapshot of the matteocontrini/comuni-json
// dataset (canonical Italian comuni list with codice belfiore, CAP, and
// province metadata, derived from ISTAT and ANCI) and exposes resolvers
// that map human-friendly inputs to the codice belfiore used by AdE
// and the ondata Parquet dataset.
//
// Source: https://github.com/matteocontrini/comuni-json
package comuni

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode"
)

//go:embed comuni.json
var raw []byte

// Comune is one entry in the embedded dataset. JSON tags match the
// upstream comuni-json shape so we can json.Unmarshal directly.
type Comune struct {
	Nome            string   `json:"nome"`
	Codice          string   `json:"codice"` // ISTAT code (zero-padded)
	Zona            zonaT    `json:"zona"`
	Regione         regioneT `json:"regione"`
	Provincia       provT    `json:"provincia"`
	Sigla           string   `json:"sigla"`           // two-letter province (RM, MI)
	CodiceCatastale string   `json:"codiceCatastale"` // belfiore (H501)
	CAP             []string `json:"cap"`
	Popolazione     int      `json:"popolazione"`
}

type zonaT struct {
	Codice string `json:"codice"`
	Nome   string `json:"nome"`
}
type regioneT struct {
	Codice string `json:"codice"`
	Nome   string `json:"nome"`
}
type provT struct {
	Codice string `json:"codice"`
	Nome   string `json:"nome"`
}

// Errors returned by resolvers.
var (
	ErrNotFound    = errors.New("comune not found")
	ErrAmbiguous   = errors.New("multiple comuni match")
	ErrInvalidCAP  = errors.New("invalid CAP (Italian CAPs are 5 digits)")
	ErrInvalidCode = errors.New("invalid codice belfiore (expected 4 chars: 1 letter + 3 alphanumeric)")
)

var (
	loadOnce sync.Once
	loadErr  error

	all []Comune

	byBelfiore map[string]*Comune       // belfiore code → comune
	byCAP      map[string][]*Comune     // CAP → comuni (CAPs are not unique)
	byNorm     map[string][]*Comune     // normalized "name|province-sigla" → comune
	byNameOnly map[string][]*Comune     // normalized "name" alone → comune(s)
)

// Load parses the embedded dataset once. Subsequent calls are no-ops.
// Callers do not need to call this explicitly; the resolvers call it
// implicitly. Exposed for tests and for early failure detection.
func Load() error {
	loadOnce.Do(func() {
		if err := json.Unmarshal(raw, &all); err != nil {
			loadErr = fmt.Errorf("parsing embedded comuni.json: %w", err)
			return
		}
		byBelfiore = make(map[string]*Comune, len(all))
		byCAP = make(map[string][]*Comune, len(all)*2)
		byNorm = make(map[string][]*Comune, len(all))
		byNameOnly = make(map[string][]*Comune, len(all))
		for i := range all {
			c := &all[i]
			if c.CodiceCatastale != "" {
				byBelfiore[strings.ToUpper(c.CodiceCatastale)] = c
			}
			for _, cap := range c.CAP {
				byCAP[cap] = append(byCAP[cap], c)
			}
			nname := normalize(c.Nome)
			byNameOnly[nname] = append(byNameOnly[nname], c)
			byNorm[nname+"|"+strings.ToUpper(c.Sigla)] = append(byNorm[nname+"|"+strings.ToUpper(c.Sigla)], c)
		}
	})
	return loadErr
}

// ResolveByBelfiore looks up a comune by its codice catastale (case-insensitive).
func ResolveByBelfiore(code string) (*Comune, error) {
	if err := Load(); err != nil {
		return nil, err
	}
	code = strings.ToUpper(strings.TrimSpace(code))
	if !isBelfioreShape(code) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidCode, code)
	}
	if c, ok := byBelfiore[code]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("%w: codice belfiore %q", ErrNotFound, code)
}

// ResolveByName looks up a comune by its human name, optionally
// disambiguated with a 2-letter province sigla (e.g. "MI") or a full
// province name (e.g. "Milano"). Comparison is accent-insensitive
// and case-insensitive.
//
// If multiple comuni share the same name and no province is given,
// ErrAmbiguous is returned along with the candidate list inside the
// error message.
func ResolveByName(name, province string) (*Comune, error) {
	if err := Load(); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: name is empty", ErrNotFound)
	}
	nname := normalize(name)
	prov := strings.TrimSpace(province)
	if prov != "" {
		// Match against sigla OR full province name.
		sigla := strings.ToUpper(prov)
		key := nname + "|" + sigla
		if hits := byNorm[key]; len(hits) == 1 {
			return hits[0], nil
		}
		// Fall back to scanning name-only matches and filtering by
		// province name (so the caller can pass "Milano" not just "MI").
		var matched []*Comune
		nprov := normalize(prov)
		for _, c := range byNameOnly[nname] {
			if strings.EqualFold(c.Sigla, prov) || normalize(c.Provincia.Nome) == nprov {
				matched = append(matched, c)
			}
		}
		if len(matched) == 1 {
			return matched[0], nil
		}
		if len(matched) == 0 {
			return nil, fmt.Errorf("%w: name=%q province=%q", ErrNotFound, name, prov)
		}
		return nil, ambiguous(matched, fmt.Sprintf("name=%q province=%q", name, prov))
	}
	hits := byNameOnly[nname]
	switch len(hits) {
	case 0:
		return nil, fmt.Errorf("%w: name=%q", ErrNotFound, name)
	case 1:
		return hits[0], nil
	default:
		return nil, ambiguous(hits, fmt.Sprintf("name=%q", name))
	}
}

// ResolveByCAP returns every comune that lists the given CAP. Italian
// CAPs are not unique — large cities have several CAPs and small CAPs
// can cover multiple comuni — so callers must handle the multi-match
// case. The returned slice is never empty on a nil error.
func ResolveByCAP(cap string) ([]*Comune, error) {
	if err := Load(); err != nil {
		return nil, err
	}
	cap = strings.TrimSpace(cap)
	if !isCAPShape(cap) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidCAP, cap)
	}
	hits := byCAP[cap]
	if len(hits) == 0 {
		return nil, fmt.Errorf("%w: cap=%q", ErrNotFound, cap)
	}
	return hits, nil
}

// All returns the full slice of embedded comuni for callers that want
// to iterate (read-only; do not mutate).
func All() ([]Comune, error) {
	if err := Load(); err != nil {
		return nil, err
	}
	return all, nil
}

func ambiguous(matched []*Comune, why string) error {
	names := make([]string, 0, len(matched))
	for _, c := range matched {
		names = append(names, fmt.Sprintf("%s (%s, %s)", c.Nome, c.Sigla, c.CodiceCatastale))
	}
	return fmt.Errorf("%w: %s → %d candidates: %s", ErrAmbiguous, why, len(matched), strings.Join(names, "; "))
}

func isBelfioreShape(s string) bool {
	if len(s) != 4 {
		return false
	}
	if !(s[0] >= 'A' && s[0] <= 'Z') {
		return false
	}
	for i := 1; i < 4; i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}
	return true
}

func isCAPShape(s string) bool {
	if len(s) != 5 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// normalize lowercases, strips diacritics, collapses whitespace, and
// removes punctuation so "Sant'Agata di Militello" and "sant agata di
// militello" compare equal.
func normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := true
	for _, r := range strings.ToLower(s) {
		r = stripDiacritic(r)
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevSpace = false
		} else {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
		}
	}
	return strings.TrimSpace(b.String())
}

// stripDiacritic returns the base ASCII rune for the most common Italian
// accented letters. Full Unicode NFD would be cleaner but pulls in
// golang.org/x/text; this covers à è é ì ò ù plus capitals.
func stripDiacritic(r rune) rune {
	switch r {
	case 'à', 'á', 'â', 'ä', 'ã':
		return 'a'
	case 'è', 'é', 'ê', 'ë':
		return 'e'
	case 'ì', 'í', 'î', 'ï':
		return 'i'
	case 'ò', 'ó', 'ô', 'ö', 'õ':
		return 'o'
	case 'ù', 'ú', 'û', 'ü':
		return 'u'
	case 'ñ':
		return 'n'
	case 'ç':
		return 'c'
	}
	return r
}
