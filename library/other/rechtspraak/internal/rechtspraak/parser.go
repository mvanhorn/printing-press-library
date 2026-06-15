// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

package rechtspraak

import (
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

// atomFeed mirrors the Atom feed returned by /uitspraken/zoeken.
type atomFeed struct {
	XMLName  xml.Name    `xml:"feed"`
	Subtitle string      `xml:"subtitle"`
	Updated  string      `xml:"updated"`
	Entries  []atomEntry `xml:"entry"`
}

type atomEntry struct {
	ID      string `xml:"id"`
	Title   string `xml:"title"`
	Summary string `xml:"summary"`
	Updated string `xml:"updated"`
	Deleted string `xml:"deleted,attr"`
	Links   []struct {
		Href string `xml:"href,attr"`
		Rel  string `xml:"rel,attr"`
	} `xml:"link"`
}

// ParseSearchResponse parses an Atom feed and returns entries plus the
// total-count from <subtitle>.
func ParseSearchResponse(r io.Reader) ([]SearchEntry, int, error) {
	var f atomFeed
	dec := xml.NewDecoder(r)
	if err := dec.Decode(&f); err != nil {
		return nil, 0, fmt.Errorf("parse atom: %w", err)
	}
	total := extractTotalFromSubtitle(f.Subtitle)
	entries := make([]SearchEntry, 0, len(f.Entries))
	for _, e := range f.Entries {
		se := SearchEntry{
			ECLI:    e.ID,
			Title:   e.Title,
			Summary: strings.TrimSpace(e.Summary),
			Deleted: e.Deleted,
		}
		if t, err := time.Parse(time.RFC3339, e.Updated); err == nil {
			se.Updated = t
		} else if t, err := time.Parse("2006-01-02", e.Updated); err == nil {
			se.Updated = t
		}
		for _, l := range e.Links {
			if l.Rel == "" || l.Rel == "alternate" {
				se.Link = l.Href
				break
			}
		}
		entries = append(entries, se)
	}
	return entries, total, nil
}

var subtitleRe = regexp.MustCompile(`(?i)\b(\d[\d.,\s]*)$`)

func extractTotalFromSubtitle(s string) int {
	// "Aantal gevonden ECLI's: 3701402"
	m := subtitleRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	digits := strings.NewReplacer(".", "", ",", "", " ", "").Replace(m[1])
	var n int
	if _, err := fmt.Sscanf(digits, "%d", &n); err != nil {
		return 0
	}
	return n
}

// openRechtspraak mirrors the root element of /uitspraken/content responses.
type openRechtspraak struct {
	XMLName          xml.Name  `xml:"open-rechtspraak"`
	RDF              rdfRoot   `xml:"RDF"`
	Inhoudsindicatie *richText `xml:"inhoudsindicatie"`
	Uitspraak        *richText `xml:"uitspraak"`
	Conclusie        *richText `xml:"conclusie"`
}

type rdfRoot struct {
	Descriptions []rdfDescription `xml:"Description"`
}

type rdfDescription struct {
	About        string          `xml:"about,attr"`
	Identifier   string          `xml:"identifier"`
	Format       string          `xml:"format"`
	AccessRights string          `xml:"accessRights"`
	Modified     string          `xml:"modified"`
	Issued       string          `xml:"issued"`
	Publisher    string          `xml:"publisher"`
	Language     string          `xml:"language"`
	Creator      labeledResource `xml:"creator"`
	Date         string          `xml:"date"`
	Type         labeledResource `xml:"type"`
	Procedure    labeledResource `xml:"procedure"`
	Coverage     string          `xml:"coverage"`
	Subject      labeledResource `xml:"subject"`
	Spatial      labeledResource `xml:"spatial"`
	Title        string          `xml:"title"`
	Alternative  string          `xml:"alternative"`
	Zaaknummer   []string        `xml:"zaaknummer"`
	Contributor  []string        `xml:"contributor"`
	Replaces     []string        `xml:"replaces"`
	IsReplacedBy string          `xml:"isReplacedBy"`
	Relations    []relationXML   `xml:"relation"`
	References   []referenceXML  `xml:"references"`
	HasVersion   *hasVersionXML  `xml:"hasVersion"`
}

type labeledResource struct {
	Value    string `xml:",chardata"`
	Resource string `xml:"resourceIdentifier,attr"`
	Label    string `xml:"label,attr"`
}

type relationXML struct {
	Value       string `xml:",chardata"`
	ECLIRef     string `xml:"resourceIdentifier,attr"`
	TypeRelatie string `xml:"typeRelatie,attr"`
	Gevolg      string `xml:"gevolg,attr"`
	Aanleg      string `xml:"aanleg,attr"`
	Label       string `xml:"label,attr"`
}

type referenceXML struct {
	Value    string `xml:",chardata"`
	Label    string `xml:"label,attr"`
	BWB      string `xml:"http://www.example.com/bwb-dl resourceIdentifier,attr"`
	BWBPlain string `xml:"bwb resourceIdentifier,attr"`
	ECLIRef  string `xml:"https://e-justice.europa.eu/ecli resourceIdentifier,attr"`
	CVDR     string `xml:"http://decentrale.regelgeving.overheid.nl/cvdr/ resourceIdentifier,attr"`
	EU       string `xml:"http://publications.europa.eu/celex/ resourceIdentifier,attr"`
}

type hasVersionXML struct {
	List rdfList `xml:"list"`
}

type rdfList struct {
	Items []string `xml:"li"`
}

// richText is the parsed body of an inhoudsindicatie / uitspraak / conclusie
// element. We unmarshal the raw text content; the Docbook structure is
// flattened to whitespace-preserving plain text suitable for FTS5 indexing.
type richText struct {
	InnerXML string `xml:",innerxml"`
}

// PlainText flattens an inner-XML body into plain text by stripping tags and
// collapsing whitespace.
func (r *richText) PlainText() string {
	if r == nil {
		return ""
	}
	return flattenXML(r.InnerXML)
}

var tagRe = regexp.MustCompile(`<[^>]+>`)
var wsRe = regexp.MustCompile(`[ \t]+`)
var nlRe = regexp.MustCompile(`\n\s*\n+`)

func flattenXML(s string) string {
	// Replace block-level closers with newlines so paragraph breaks survive.
	s = strings.ReplaceAll(s, "</para>", "\n\n")
	s = strings.ReplaceAll(s, "</parablock>", "\n\n")
	s = strings.ReplaceAll(s, "</paragroup>", "\n\n")
	s = strings.ReplaceAll(s, "</section>", "\n\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = tagRe.ReplaceAllString(s, "")
	s = wsRe.ReplaceAllString(s, " ")
	s = nlRe.ReplaceAllString(s, "\n\n")
	// Decode common XML entities.
	s = strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
		"&apos;", "'",
		"&#39;", "'",
		"&#160;", " ",
		"&nbsp;", " ",
	).Replace(s)
	return strings.TrimSpace(s)
}

// ParseDecision parses a /uitspraken/content response into a Decision.
func ParseDecision(r io.Reader) (*Decision, error) {
	var or openRechtspraak
	dec := xml.NewDecoder(r)
	dec.Strict = false
	if err := dec.Decode(&or); err != nil {
		return nil, fmt.Errorf("parse content xml: %w", err)
	}
	d := &Decision{}
	// First Description is the ECLI register entry; second (if present) is
	// the text-document. Both contribute fields.
	for _, desc := range or.RDF.Descriptions {
		if desc.Identifier != "" && !strings.HasPrefix(desc.Identifier, "http") {
			d.ECLI = desc.Identifier
		}
		if desc.AccessRights != "" {
			d.AccessRights = desc.AccessRights
		}
		if desc.Modified != "" {
			d.Modified = desc.Modified
		}
		if desc.Issued != "" {
			d.PublicationDate = desc.Issued
		}
		if desc.Language != "" {
			d.Language = desc.Language
		}
		if desc.Creator.Value != "" {
			d.Court = desc.Creator.Value
			d.CourtURI = desc.Creator.Resource
		}
		if desc.Date != "" {
			d.DecisionDate = desc.Date
		}
		if desc.Type.Value != "" {
			d.Type = desc.Type.Value
		}
		if desc.Procedure.Value != "" {
			d.Procedure = desc.Procedure.Value
			d.ProcedureURI = desc.Procedure.Resource
		}
		if desc.Subject.Value != "" {
			d.Subject = desc.Subject.Value
			d.SubjectURI = desc.Subject.Resource
		}
		if desc.Spatial.Value != "" {
			d.Spatial = desc.Spatial.Value
		}
		if desc.Title != "" {
			d.Title = desc.Title
		}
		if desc.Alternative != "" {
			d.Alternative = desc.Alternative
		}
		for _, z := range desc.Zaaknummer {
			if z = strings.TrimSpace(z); z == "" {
				continue
			}
			// Schema allows ";"-separated multi-values inside one element.
			for _, part := range strings.Split(z, ";") {
				if p := strings.TrimSpace(part); p != "" {
					d.Zaaknummer = append(d.Zaaknummer, p)
				}
			}
		}
		for _, c := range desc.Contributor {
			if c = strings.TrimSpace(c); c != "" {
				d.Contributors = append(d.Contributors, c)
			}
		}
		for _, r := range desc.Replaces {
			if r = strings.TrimSpace(r); r != "" {
				d.Replaces = append(d.Replaces, r)
			}
		}
		if desc.IsReplacedBy != "" {
			d.IsReplacedBy = desc.IsReplacedBy
		}
		for _, rel := range desc.Relations {
			d.Relations = append(d.Relations, Relation{
				Target:      rel.ECLIRef,
				TypeRelatie: rel.TypeRelatie,
				Gevolg:      rel.Gevolg,
				Aanleg:      rel.Aanleg,
				Label:       rel.Label,
				Text:        strings.TrimSpace(rel.Value),
			})
		}
		for _, ref := range desc.References {
			ref2 := Reference{
				Text: strings.TrimSpace(ref.Value),
				BWB:  firstNonEmpty(ref.BWB, ref.BWBPlain),
				ECLI: ref.ECLIRef,
				CVDR: ref.CVDR,
				EU:   ref.EU,
			}
			ref2.Kind = classifyReference(ref2)
			d.References = append(d.References, ref2)
		}
		if desc.HasVersion != nil {
			for _, raw := range desc.HasVersion.List.Items {
				if raw = strings.TrimSpace(raw); raw == "" {
					continue
				}
				d.Vindplaatsen = append(d.Vindplaatsen, parseVindplaats(raw))
			}
		}
	}
	d.Summary = or.Inhoudsindicatie.PlainText()
	if or.Uitspraak != nil {
		d.Body = or.Uitspraak.PlainText()
	} else if or.Conclusie != nil {
		d.Body = or.Conclusie.PlainText()
	}
	return d, nil
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}

func classifyReference(r Reference) string {
	switch {
	case r.BWB != "":
		return "bwb"
	case r.CVDR != "":
		return "cvdr"
	case r.EU != "":
		return "eu"
	case r.ECLI != "":
		return "ecli"
	default:
		return "other"
	}
}

var vindplaatsRe = regexp.MustCompile(`^([A-Za-z][A-Za-z\.\-]*)\s+(\d{4})(?:[/,\s]+(\S+))?(?:\s+m\.nt\.\s+(.+))?$`)

func parseVindplaats(s string) Vindplaats {
	v := Vindplaats{Raw: s}
	if m := vindplaatsRe.FindStringSubmatch(s); m != nil {
		v.Journal = m[1]
		v.Year = m[2]
		v.Number = m[3]
		v.Annotator = m[4]
	}
	return v
}

// === Waardelijst parsers ===

type courtsWrapper struct {
	XMLName    xml.Name   `xml:"Instanties"`
	Instanties []courtXML `xml:"Instantie"`
}

type courtXML struct {
	Identifier string `xml:"Identifier"`
	Naam       string `xml:"Naam"`
	Afkorting  string `xml:"Afkorting"`
	Type       string `xml:"Type"`
	BeginDate  string `xml:"BeginDate"`
	EndDate    string `xml:"EndDate"`
}

func ParseCourts(r io.Reader) ([]Court, error) {
	var w courtsWrapper
	if err := xml.NewDecoder(r).Decode(&w); err != nil {
		return nil, fmt.Errorf("parse courts: %w", err)
	}
	out := make([]Court, 0, len(w.Instanties))
	for _, c := range w.Instanties {
		out = append(out, Court{
			Identifier: c.Identifier,
			Name:       strings.TrimSpace(c.Naam),
			Afkorting:  c.Afkorting,
			Type:       c.Type,
			BeginDate:  c.BeginDate,
			EndDate:    c.EndDate,
		})
	}
	return out, nil
}

type subjectsWrapper struct {
	XMLName xml.Name     `xml:"Rechtsgebieden"`
	Top     []subjectXML `xml:"Rechtsgebied"`
}

type subjectXML struct {
	Identifier string       `xml:"Identifier"`
	Naam       string       `xml:"Naam"`
	Children   []subjectXML `xml:"Rechtsgebied"`
}

func ParseSubjects(r io.Reader) ([]Subject, error) {
	var w subjectsWrapper
	if err := xml.NewDecoder(r).Decode(&w); err != nil {
		return nil, fmt.Errorf("parse subjects: %w", err)
	}
	var flat []Subject
	var walk func(parent string, items []subjectXML)
	walk = func(parent string, items []subjectXML) {
		for _, s := range items {
			subj := Subject{
				Identifier: s.Identifier,
				Name:       strings.TrimSpace(s.Naam),
				Parent:     parent,
				Slug:       slugFromIdentifier(s.Identifier),
			}
			flat = append(flat, subj)
			if len(s.Children) > 0 {
				walk(s.Identifier, s.Children)
			}
		}
	}
	walk("", w.Top)
	return flat, nil
}

func slugFromIdentifier(s string) string {
	if i := strings.LastIndex(s, "#"); i >= 0 {
		return s[i+1:]
	}
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}

type proceduresWrapper struct {
	XMLName    xml.Name       `xml:"Proceduresoorten"`
	Procedures []procedureXML `xml:"Proceduresoort"`
}

type procedureXML struct {
	Identifier string `xml:"Identifier"`
	Naam       string `xml:"Naam"`
}

func ParseProcedures(r io.Reader) ([]Procedure, error) {
	var w proceduresWrapper
	if err := xml.NewDecoder(r).Decode(&w); err != nil {
		return nil, fmt.Errorf("parse procedures: %w", err)
	}
	out := make([]Procedure, 0, len(w.Procedures))
	for _, p := range w.Procedures {
		out = append(out, Procedure{
			Identifier: p.Identifier,
			Name:       strings.TrimSpace(p.Naam),
			Slug:       slugFromIdentifier(p.Identifier),
		})
	}
	return out, nil
}

type relationsWrapper struct {
	XMLName   xml.Name         `xml:"FormeleRelaties"`
	Relations []relationDefXML `xml:"FormeleRelatie"`
}

type relationDefXML struct {
	Naam               string        `xml:"Naam"`
	Identifier         string        `xml:"Identifier"`
	LabelEerdereAanleg string        `xml:"LabelEerdereAanleg"`
	LabelLatereAanleg  string        `xml:"LabelLatereAanleg"`
	Rolspelers         rolspelersXML `xml:"Rolspelers"`
	AfhandelingsWijze  outcomesXML   `xml:"AfhandelingsWijze"`
}

type rolspelersXML struct {
	Items []rolspelerXML `xml:"Rolspeler"`
}

type rolspelerXML struct {
	Earlier string `xml:"InstantieEerdereAanleg"`
	Later   string `xml:"InstantieLatereAanleg"`
}

type outcomesXML struct {
	Names []string `xml:"Naam"`
}

func ParseRelations(r io.Reader) ([]RelationDef, error) {
	var w relationsWrapper
	if err := xml.NewDecoder(r).Decode(&w); err != nil {
		return nil, fmt.Errorf("parse relations: %w", err)
	}
	out := make([]RelationDef, 0, len(w.Relations))
	for _, rd := range w.Relations {
		def := RelationDef{
			Name:         strings.TrimSpace(rd.Naam),
			Identifier:   rd.Identifier,
			LabelEarlier: rd.LabelEerdereAanleg,
			LabelLater:   rd.LabelLatereAanleg,
			Outcomes:     rd.AfhandelingsWijze.Names,
		}
		for _, r := range rd.Rolspelers.Items {
			def.Rolspelers = append(def.Rolspelers, RolPair{
				EarlierInstantie: r.Earlier,
				LaterInstantie:   r.Later,
			})
		}
		out = append(out, def)
	}
	return out, nil
}

type foreignDecisionsWrapper struct {
	XMLName  xml.Name             `xml:"NietNederlandseUitspraken"`
	Modified string               `xml:"modified"`
	Entries  []foreignDecisionXML `xml:"entry"`
}

type foreignDecisionXML struct {
	ID  string   `xml:"id"`
	LJN []string `xml:"ljn"`
}

func ParseForeignDecisions(r io.Reader) ([]ForeignDecision, error) {
	var w foreignDecisionsWrapper
	if err := xml.NewDecoder(r).Decode(&w); err != nil {
		return nil, fmt.Errorf("parse foreign decisions: %w", err)
	}
	out := make([]ForeignDecision, 0, len(w.Entries))
	for _, e := range w.Entries {
		out = append(out, ForeignDecision{ECLI: e.ID, LJN: e.LJN})
	}
	return out, nil
}
