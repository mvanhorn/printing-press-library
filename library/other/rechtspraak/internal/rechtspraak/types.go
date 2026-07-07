// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

package rechtspraak

import "time"

// SearchEntry is one row from the Atom feed at /uitspraken/zoeken.
type SearchEntry struct {
	ECLI    string    `json:"ecli"`
	Title   string    `json:"title"`
	Summary string    `json:"summary,omitempty"`
	Updated time.Time `json:"updated"`
	Link    string    `json:"link,omitempty"`
	// Deleted carries the Atom entry's "deleted" attribute: "doc" means the
	// body was withdrawn but the ECLI still exists; "ecli" means the ECLI
	// itself was superseded (treat as a delete signal during sync).
	Deleted string `json:"deleted,omitempty"`
}

// Relation is one dcterms:relation edge on a Decision.
type Relation struct {
	// Target is the related ECLI (from ecli:resourceIdentifier).
	Target string `json:"target"`
	// TypeRelatie is the relation type URI from psi:typeRelatie, e.g.
	// http://psi.rechtspraak.nl/cassatie or http://psi.rechtspraak.nl/conclusie.
	TypeRelatie string `json:"type_relatie,omitempty"`
	// Gevolg is the disposition outcome URI from psi:gevolg, e.g.
	// http://psi.rechtspraak.nl/bekrachtiging.
	Gevolg string `json:"gevolg,omitempty"`
	// Aanleg is the direction URI from psi:aanleg, e.g.
	// http://psi.rechtspraak.nl/eerdereAanleg or .../latereAanleg.
	Aanleg string `json:"aanleg,omitempty"`
	// Label is the human-readable rdfs:label.
	Label string `json:"label,omitempty"`
	// Text is the inline text node ("Conclusie: ECLI:NL:PHR:...").
	Text string `json:"text,omitempty"`
}

// Reference is a body-level dcterms:references entry.
type Reference struct {
	Text string `json:"text,omitempty"`
	// One of these resource identifiers is typically populated.
	BWB  string `json:"bwb,omitempty"`  // Dutch statute (BWB) URI
	CVDR string `json:"cvdr,omitempty"` // Local-regulation CVDR URI
	EU   string `json:"eu,omitempty"`   // CELEX EU document URI
	ECLI string `json:"ecli,omitempty"` // Cross-jurisdiction ECLI
	Kind string `json:"kind,omitempty"` // "bwb" | "cvdr" | "eu" | "ecli" | "other"
}

// Vindplaats is one published citation of a decision (e.g., RvdW 2024/125).
type Vindplaats struct {
	Raw       string `json:"raw"`
	Journal   string `json:"journal,omitempty"`
	Year      string `json:"year,omitempty"`
	Number    string `json:"number,omitempty"`
	Page      string `json:"page,omitempty"`
	Annotator string `json:"annotator,omitempty"`
}

// Decision is the parsed view of one ECLI's /uitspraken/content response.
type Decision struct {
	ECLI            string       `json:"ecli"`
	Court           string       `json:"court,omitempty"`
	CourtURI        string       `json:"court_uri,omitempty"`
	DecisionDate    string       `json:"decision_date,omitempty"`    // YYYY-MM-DD
	PublicationDate string       `json:"publication_date,omitempty"` // YYYY-MM-DD
	Modified        string       `json:"modified,omitempty"`         // ISO8601
	Subject         string       `json:"subject,omitempty"`
	SubjectURI      string       `json:"subject_uri,omitempty"`
	Procedure       string       `json:"procedure,omitempty"`
	ProcedureURI    string       `json:"procedure_uri,omitempty"`
	Type            string       `json:"type,omitempty"` // Uitspraak | Conclusie
	Zaaknummer      []string     `json:"zaaknummer,omitempty"`
	Title           string       `json:"title,omitempty"`
	Alternative     string       `json:"alternative_title,omitempty"`
	Summary         string       `json:"summary,omitempty"` // inhoudsindicatie plain text
	Body            string       `json:"body,omitempty"`    // uitspraak/conclusie plain text
	Contributors    []string     `json:"contributors,omitempty"`
	Spatial         string       `json:"spatial,omitempty"` // zittingsplaats
	Relations       []Relation   `json:"relations,omitempty"`
	References      []Reference  `json:"references,omitempty"`
	Vindplaatsen    []Vindplaats `json:"vindplaatsen,omitempty"`
	Replaces        []string     `json:"replaces,omitempty"` // old LJN codes
	IsReplacedBy    string       `json:"is_replaced_by,omitempty"`
	Language        string       `json:"language,omitempty"`
	AccessRights    string       `json:"access_rights,omitempty"`
}

// Court is one entry from /Waardelijst/Instanties.
type Court struct {
	Identifier string `json:"identifier"` // PSI URI
	Name       string `json:"name"`
	Afkorting  string `json:"afkorting,omitempty"` // short code used in ECLI (e.g. "HR", "RBAMS")
	Type       string `json:"type,omitempty"`      // Hoge Raad, Rechtbank, Gerechtshof, etc.
	BeginDate  string `json:"begin_date,omitempty"`
	EndDate    string `json:"end_date,omitempty"`
}

// Subject is one entry from /Waardelijst/Rechtsgebieden.
type Subject struct {
	Identifier string `json:"identifier"` // PSI URI
	Name       string `json:"name"`
	Slug       string `json:"slug,omitempty"`   // last fragment after '#'
	Parent     string `json:"parent,omitempty"` // PSI URI of parent
}

// Procedure is one entry from /Waardelijst/Proceduresoorten.
type Procedure struct {
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
	Slug       string `json:"slug,omitempty"`
}

// RelationDef is one entry from /Waardelijst/FormeleRelaties.
type RelationDef struct {
	Name         string    `json:"name"`       // e.g. "Hoger beroep"
	Identifier   string    `json:"identifier"` // PSI URI
	LabelEarlier string    `json:"label_earlier,omitempty"`
	LabelLater   string    `json:"label_later,omitempty"`
	Outcomes     []string  `json:"outcomes,omitempty"` // AfhandelingsWijze names
	Rolspelers   []RolPair `json:"rolspelers,omitempty"`
}

// RolPair captures a from/to court-tier pair for a relation.
type RolPair struct {
	EarlierInstantie string `json:"earlier"`
	LaterInstantie   string `json:"later"`
}

// ForeignDecision maps a foreign ECLI to one or more old Dutch LJN codes.
type ForeignDecision struct {
	ECLI string   `json:"ecli"`
	LJN  []string `json:"ljn"`
}
