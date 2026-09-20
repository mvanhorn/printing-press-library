// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// See .printing-press-patches/nepra-licence-register.json.
//
// The accordion shaper for NEPRA's generation licence register.
//
// WHY NOT extractNepraTable / nepraparse.BuildGrid. The four Excel-export
// sheets this CLI already reads are ONE table with a header band. A licence
// page is 1..122 SEPARATE accordions, each an <li class="accordion"> holding
// an <h6> with the entity's name and its own <table> of two-cell key/value
// rows. BuildGrid counts <table> but never partitions on it: it never resets
// Rows, the pending-rowspan map or Width at a table boundary, and it only
// reads <tr>/<td>/<th> — so the 122 tables of Generation IPPs RE 2006.php
// collapse into one flat 708-row grid and the 122 <h6> names, which are the
// only entity identity there is, are never read at all. Entity identity is
// unrecoverable from the grid, so this file does its own splitting.
// nepraparse.Decode is still used: these pages are the same windows-1252
// family with no charset in the HTTP header.

package cli

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// licenceEntity is one accordion: one licensee.
type licenceEntity struct {
	// Name is the <h6> text, VERBATIM. It is NOT a primary key: 'Hamza Sugar
	// Mills Limited' appears three times in the register and 'Lucky Cement
	// Limited' and 'Artistic Solar Energy (Private) Limited' twice each, so
	// 335 accordions carry 331 distinct names. Nothing here dedupes on it.
	Name string `json:"name"`
	// NameNote records a defect in the name itself. The K-Electric accordion
	// header reads 'K-Elecric' while the same page's <title> and <h2> spell
	// it correctly, so the misspelling sits in exactly the field a name
	// lookup uses.
	NameNote  string `json:"name_note,omitempty"`
	Surface   string `json:"surface"`
	SourceURL string `json:"source_url"`
	// Pairs is every published key/value row in order, verbatim. It is the
	// evidence; the typed fields below are an interpretation of it.
	Pairs []licencePair `json:"pairs"`

	// The interpreted core fields. Each is "" when the entity publishes no
	// such key — 98 of 335 entities publish no fuel key at all, and two
	// publish no capacity key.
	LicenceNo   string `json:"licence_no"`
	PlantType   string `json:"plant_type"`
	PlantDetail string `json:"plant_detail"`
	Fuel        string `json:"fuel"`
	// GrossCapacityRaw is the capacity cell as published. GrossCapacityMW is
	// nil unless that cell parses as a plain "<number> MW"; the reason is in
	// CapacityNote. A capacity this build could not read is NEVER guessed.
	GrossCapacityRaw string `json:"gross_capacity_raw"`
	// CapacityValue and CapacityUnit are what the register actually says,
	// with the unit read rather than assumed. GrossCapacityMW is the
	// converted figure and is nil whenever the published unit is not
	// convertible to MW — notably MWp, a solar peak DC rating, which is a
	// different physical quantity with no NEPRA-published conversion.
	CapacityValue   *float64 `json:"capacity_value"`
	CapacityUnit    string   `json:"capacity_unit"`
	GrossCapacityMW *float64 `json:"gross_capacity_mw"`
	CapacityNote    string   `json:"capacity_note,omitempty"`
	// Modifications is every modification/revocation/determination row,
	// collected across the EIGHT-PLUS spellings the register uses for that
	// key.
	Modifications []licencePair `json:"modifications,omitempty"`
	// Transposed reports that this entity's key/value pairs are swapped
	// upstream. Fauji Kabirwala publishes Plant Type -> '170 MW' and Gross
	// Capacity -> 'Thermal (Combined Cycle)'. It is FLAGGED, never repaired:
	// the 170 MW is real but published under the wrong key, and silently
	// moving it would be this CLI inventing an attribution.
	Transposed     bool   `json:"transposed"`
	TransposedNote string `json:"transposed_note,omitempty"`
}

type licencePair struct {
	Key string `json:"key"`
	// KeyRaw is kept when normalisation changed the key, so a typo is
	// visible rather than silently corrected.
	KeyRaw string `json:"key_raw,omitempty"`
	Value  string `json:"value"`
	// DocumentURL is the href of the anchor in the VALUE cell, resolved
	// against the page.
	//
	// Without it the modification trail is hollow: almost every modification
	// row's visible text is the single word "View", so a reader learns that
	// a modification exists and has no way to reach it. The register's whole
	// modification trail is links.
	DocumentURL string `json:"document_url,omitempty"`
}

// licenceCapacityRE matches a number followed by a UNIT, and the unit is
// captured rather than assumed.
//
// READING THE UNIT IS NOT PEDANTRY — IT IS A 1000x SAFETY PROPERTY. The
// register publishes four different units in this one cell, measured across
// all 379 entities:
//
//	MW    338 entities   megawatts, the common case
//	MWe     5 entities   megawatts ELECTRICAL: the same quantity as MW, used
//	                     by the five nuclear plants (KANUPP 137 MWe, CHASNUPP
//	                     1-4 at 325/340/340/340 MWe)
//	kW      5 entities   Atlas Energy's five sites, 501.60 to 995.60 kW. A
//	                     parser that read the number and assumed MW would
//	                     report 858.80 MW for an 858.80 kW rooftop array —
//	                     out by a factor of a thousand, and larger than
//	                     Pakistan's biggest hydel station.
//	MWp    13 entities   megawatts PEAK: a solar DC nameplate rating, which
//	                     is NOT the same quantity as MW AC and has no fixed
//	                     conversion to it. It is parsed and labelled, and
//	                     deliberately NOT folded into the MW figure.
//
// A cell whose unit this does not recognise is refused outright rather than
// having its leading number taken. That is what keeps '12:00 MW' from
// becoming 12, '02:00 MW' from becoming 2, and '3s 6 MW' from becoming 3 —
// and the last is the sharpest case, because Almoiz's own plant-detail row
// reads '1x 16MW + 1x20 MW', i.e. 36 MW, so a first-number-wins reading is
// out by 12x. The research probe's published 47,559.97 MW over 331 entities
// is reproducible ONLY by making exactly those two guesses.
var licenceCapacityRE = regexp.MustCompile(`(?i)^([0-9]{1,3}(?:,[0-9]{3})*(?:\.[0-9]+)?|[0-9]+(?:\.[0-9]+)?)\s*(MWp|MWe|MW|kW)$`)

// licenceCapacityUnits maps a recognised unit onto its factor in MW, or
// reports that it is a different quantity that must not be converted.
//
// MWp is present with convertible=false on purpose: a peak DC rating and an
// AC megawatt are different physical quantities, the ratio between them is a
// property of the installation, and NEPRA publishes no conversion. Inventing
// one would be exactly the kind of fabrication this CLI refuses everywhere.
var licenceCapacityUnits = map[string]struct {
	factor      float64
	convertible bool
	note        string
}{
	"mw":  {1, true, ""},
	"mwe": {1, true, "published as MWe (megawatts electrical), which is the same quantity as MW"},
	"kw":  {0.001, true, "published in kW and converted at 1000 kW = 1 MW; the source figure is in capacity_raw"},
	"mwp": {0, false, "published as MWp, a solar PEAK DC nameplate rating. That is NOT the same quantity as " +
		"MW AC and NEPRA publishes no conversion between them, so no MW figure is emitted for this entity. " +
		"capacity_value and capacity_unit carry what the register actually says."},
}

// licenceModificationKeyRE matches the modification family by NORMALISED
// PREFIX rather than by a literal alias list.
//
// A literal map cannot cover it: the register uses at least eight spellings —
// 'Modification-I' (45), 'Licence Modification-I' (36), 'Licence Modification'
// (6), 'Licence Modification -I' (6), 'Modification - I' (3), 'Modification
// -I' (2), 'Licence Modification -II' (2) — and one of them,
// 'Licence Modification (10-01-2025)' (4 occurrences), embeds a DATE.
var licenceModificationKeyRE = regexp.MustCompile(`(?i)^(licence\s+)?(modification|revocation|determination)\b`)

// licenceKeyAliases maps a normalised published key onto the field it means.
//
// Only the UPSTREAM TYPOS are aliased here, and each is recorded with its
// measured frequency so nobody widens the map on a guess. Exact matching alone
// silently drops WAPDA Hydel's 17,367.96 MW, which is 36.5% of the register's
// readable capacity, because its key cell reads 'Gross Capacityy'.
var licenceKeyAliases = map[string]string{
	"gross capacity":  "gross_capacity",
	"gross capacityy": "gross_capacity", // 1 of 335: WAPDA Hydel
	"licence no.":     "licence_no",
	"licence no":      "licence_no",
	"license no.":     "licence_no",
	"plant type":      "plant_type",
	"plant detail":    "plant_detail",
	"plant detaill":   "plant_detail", // 6 of 335
	"plant details":   "plant_detail",
	"fuel type":       "fuel",
	"fuel":            "fuel", // 21 of 335 use the short form
	"licence details": "licence_details",
	"license details": "licence_details",
}

// licenceNormaliseKey folds a published key cell for lookup.
//
// It collapses the whitespace the source splits keys across — the WAPDA Hydel
// capacity key is literally "Gross \n\t\t\t\t\t\tCapacityy" — folds NBSP, and
// lowercases. It does NOT strip punctuation beyond trailing colons: 'Licence
// No.' and 'Licence No' are both real and both aliased explicitly rather than
// folded together by a rule that might collide something else.
func licenceNormaliseKey(s string) string {
	s = strings.ReplaceAll(s, " ", " ")
	s = strings.Join(strings.Fields(s), " ")
	s = strings.TrimSuffix(strings.TrimSpace(s), ":")
	return strings.ToLower(s)
}

// licenceParsePage splits one register page into its accordions.
func licenceParsePage(raw []byte, surface, sourceURL string) ([]licenceEntity, error) {
	text, _, _, err := nepraparse.Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("decoding %s: %w", surface, err)
	}
	doc, err := html.Parse(strings.NewReader(text))
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", surface, err)
	}
	var out []licenceEntity
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "li" && licenceHasClass(n, "accordion") {
			if e, ok := licenceParseAccordion(n, surface, sourceURL); ok {
				out = append(out, e)
			}
			// An accordion never nests another; do not descend.
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return out, nil
}

func licenceHasClass(n *html.Node, want string) bool {
	for _, a := range n.Attr {
		if a.Key != "class" {
			continue
		}
		for _, f := range strings.Fields(a.Val) {
			if f == want {
				return true
			}
		}
	}
	return false
}

// licenceParseAccordion reads one <li class="accordion">.
func licenceParseAccordion(li *html.Node, surface, sourceURL string) (licenceEntity, bool) {
	e := licenceEntity{Surface: surface, SourceURL: sourceURL}

	// The name is the <h6> inside the header div. It is the ONLY place the
	// entity is identified.
	var findName func(*html.Node)
	findName = func(n *html.Node) {
		if e.Name != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "h6" {
			e.Name = licenceText(n)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findName(c)
		}
	}
	findName(li)
	if e.Name == "" {
		return e, false
	}
	if strings.Contains(e.Name, "Elecric") {
		e.NameNote = "the accordion header misspells the entity; the same page's <title> and <h2> spell it correctly. " +
			"Reproduced verbatim because it is what the register publishes in the one field a name lookup reads."
	}

	// Every <tr> with at least two <td> is a key/value row.
	var rows func(*html.Node)
	rows = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			var cells []string
			var hrefs []string
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
					cells = append(cells, licenceText(c))
					hrefs = append(hrefs, licenceFirstHref(c))
				}
			}
			if len(cells) >= 2 {
				keyRaw := cells[0]
				p := licencePair{Key: strings.Join(strings.Fields(keyRaw), " "), Value: cells[1]}
				if p.Key != keyRaw {
					p.KeyRaw = keyRaw
				}
				if len(hrefs) >= 2 {
					p.DocumentURL = licenceResolveHref(hrefs[1], sourceURL)
				}
				e.Pairs = append(e.Pairs, p)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			rows(c)
		}
	}
	rows(li)

	licenceAssign(&e)
	return e, true
}

// licenceAssign interprets the published pairs into the typed fields.
func licenceAssign(e *licenceEntity) {
	for _, p := range e.Pairs {
		norm := licenceNormaliseKey(p.Key)
		if licenceModificationKeyRE.MatchString(norm) {
			e.Modifications = append(e.Modifications, p)
			continue
		}
		switch licenceKeyAliases[norm] {
		case "gross_capacity":
			e.GrossCapacityRaw = p.Value
		case "licence_no":
			e.LicenceNo = p.Value
		case "plant_type":
			e.PlantType = p.Value
		case "plant_detail":
			e.PlantDetail = p.Value
		case "fuel":
			e.Fuel = p.Value
		}
	}

	// Transposition detection BEFORE parsing the capacity, because on a
	// transposed entity the capacity cell holds a technology string and the
	// plant-type cell holds the megawatts.
	capLooksLikeMW := licenceCapacityRE.MatchString(strings.Join(strings.Fields(e.GrossCapacityRaw), " "))
	typeLooksLikeMW := licenceCapacityRE.MatchString(strings.Join(strings.Fields(e.PlantType), " "))
	_ = capLooksLikeMW
	if !capLooksLikeMW && typeLooksLikeMW && e.GrossCapacityRaw != "" {
		e.Transposed = true
		e.TransposedNote = fmt.Sprintf(
			"the key/value pairs are TRANSPOSED upstream: the gross-capacity cell publishes %q while the "+
				"plant-type cell publishes %q. Both values are real and neither is moved: the megawatts are "+
				"published under the wrong key, and relocating them would be this tool inventing an attribution. "+
				"Read pairs[] for what the page actually says.",
			e.GrossCapacityRaw, e.PlantType)
	}

	if e.GrossCapacityRaw == "" {
		e.CapacityNote = "this entity publishes no gross-capacity key at all"
		return
	}
	collapsed := strings.Join(strings.Fields(e.GrossCapacityRaw), " ")
	m := licenceCapacityRE.FindStringSubmatch(collapsed)
	if m == nil {
		e.CapacityNote = fmt.Sprintf(
			"the published capacity cell %q does not parse as a bare <number> followed by one unit "+
				"(MW, MWe, kW or MWp), so no figure is emitted. NOTE that most such cells DO carry a "+
				"unit — the register contains \"12:00 MW\", \"02:00 MW\", \"3s 6 MW\", "+
				"\"110 MW Gross ISO\" and \"8.5 MW (based on alternators coupled with S.Ts)\" — and it "+
				"is the NUMBER that cannot be read, not the unit. Taking the leading digits would invent "+
				"a figure: the \"3s 6 MW\" entity's own plant-detail row reads \"1x 16MW + 1x20 MW\", "+
				"i.e. 36 MW, so that guess is out by 12x.",
			collapsed)
		return
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", ""), 64)
	if err != nil {
		e.CapacityNote = fmt.Sprintf("the published capacity cell %q did not convert: %v", collapsed, err)
		return
	}
	unit := strings.ToLower(m[2])
	spec, known := licenceCapacityUnits[unit]
	if !known {
		e.CapacityNote = fmt.Sprintf("the published capacity cell %q uses an unmapped unit %q", collapsed, m[2])
		return
	}
	e.CapacityValue = &v
	e.CapacityUnit = m[2]
	if spec.note != "" {
		e.CapacityNote = spec.note
	}
	if !spec.convertible {
		return
	}
	mw := v * spec.factor
	e.GrossCapacityMW = &mw
}

// licenceText extracts an element's visible text, HTML-unescaped, NBSP-folded
// and whitespace-collapsed. It keeps <b> content, which matters: several
// modification keys are whole sentences carrying bold runs.
func licenceText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	s := strings.ReplaceAll(b.String(), " ", " ")
	return strings.Join(strings.Fields(s), " ")
}

// licenceFuelTokens splits a published fuel cell into its component fuels.
//
// The register's fuel vocabulary is 61 distinct raw strings across the 237
// entities that publish one, including upstream misspellings ('HFOO',
// 'Imported Caol') and punctuation variants of one fuel ('(Bagasse)',
// 'Bagasse', 'Bagasse & F.O', 'Bagasse / F.O', 'Bagasse/F.O', 'Bagasse / FO',
// 'Bagasse/Furnace Oil'). Splitting on the separators the source actually uses
// is what makes an exact match possible at all.
func licenceFuelTokens(s string) []string {
	f := func(r rune) bool {
		switch r {
		case '/', '&', '+', ',':
			return true
		}
		return false
	}
	var out []string
	for _, part := range strings.FieldsFunc(s, f) {
		part = strings.Trim(strings.TrimSpace(part), "()")
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// licenceFuelMatch reports whether a published fuel cell carries `want` as one
// of its own tokens, matched WHOLE and case-insensitively.
//
// IT IS NOT A SUBSTRING MATCH, and that is measured rather than stylistic:
// `--fuel Coal` as a naive case-insensitive substring matches 26 entities and
// over-collects 'Coal Water Slurry', 'Biomass/Local Coal', 'Coal/Biomass',
// 'Coke Oven Gas / Blast Furnace Gas /Coal tar' and 'Natural Gas/ Residual
// Furnace Oil (RFO)/Coal'. Some of those a caller would want and some they
// would not, and this tool cannot know which — so the exact-token matches are
// returned and the near misses are REPORTED separately rather than being
// silently included or silently dropped.
func licenceFuelMatch(published, want string) bool {
	for _, tok := range licenceFuelTokens(published) {
		if strings.EqualFold(tok, want) {
			return true
		}
	}
	return false
}

// licenceFuelNearMiss reports a cell that CONTAINS the term without carrying
// it as a token, so the caller can widen the query themselves.
func licenceFuelNearMiss(published, want string) bool {
	if licenceFuelMatch(published, want) {
		return false
	}
	return strings.Contains(strings.ToLower(published), strings.ToLower(want))
}

// licenceFuelVocabulary lists the distinct published fuel cells, sorted.
func licenceFuelVocabulary(entities []licenceEntity) []string {
	seen := map[string]struct{}{}
	for _, e := range entities {
		if e.Fuel != "" {
			seen[e.Fuel] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// licenceFirstHref returns the first anchor href inside an element, verbatim.
func licenceFirstHref(n *html.Node) string {
	var out string
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if out != "" {
			return
		}
		if x.Type == html.ElementNode && x.Data == "a" {
			for _, a := range x.Attr {
				if a.Key == "href" {
					out = strings.TrimSpace(a.Val)
					return
				}
			}
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

// licenceResolveHref turns a scraped href into an absolute URL against the
// page it was found on.
//
// IT APPLIES THE SAME SCHEME ALLOWLIST as the determination feed: only http
// and https name a document a caller can fetch, so a javascript:, data: or
// file: href is dropped rather than travelling out as a "document URL". The
// ROW is always kept — the modification is real even when its link is not.
func licenceResolveHref(href, pageURL string) string {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") {
		return ""
	}
	lower := strings.ToLower(href)
	switch {
	case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"):
		return href
	case strings.Contains(lower[:min(len(lower), 24)], ":"):
		// Some other scheme: not fetchable, so not a document URL.
		return ""
	}
	base, err := url.Parse(pageURL)
	if err != nil {
		return ""
	}
	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}
	return base.ResolveReference(ref).String()
}
