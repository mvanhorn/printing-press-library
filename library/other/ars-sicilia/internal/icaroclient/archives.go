// Package icaroclient handles the multi-step session/search/parse flow against
// the Icaro search engine that powers dati.ars.sicilia.it. The portal exposes
// 12 documentary archives ("banche dati") served by the same backend; this
// package gives the CLI a single typed client to talk to all of them.
package icaroclient

import "strings"

// Archive enumerates the 12 ARS documentary archives. The numeric ID is the
// `icaDB` parameter the Icaro engine expects.
type Archive struct {
	ID          string // numeric ID used as icaDB
	Slug        string // CLI/store slug (e.g. "leggi", "ddl")
	Description string // human-readable label (Italian)
	// FieldMap maps friendly CLI flag names to the upstream ISIS field sigla.
	// Flags missing from this map are not translated automatically and must be
	// supplied via --isis-query.
	FieldMap map[string]string
	// Columns names the short-list table column labels in display order. The
	// parser uses len(Columns) to know how many positional divs to read per
	// row. Title and body extracts always come from the last column's <h3>
	// and trailing <p>.
	Columns []string
}

// All archives supported by the CLI. Order matters only for stable iteration.
var All = []Archive{
	{
		ID: "201", Slug: "leggi",
		Description: "Leggi della Regione Siciliana",
		FieldMap: map[string]string{
			"legisl": "LEGISL",
			"anno":   "LEGANN",
			"numero": "LEGNUM",
		},
		Columns: []string{"Legisl.", "Atto", "Docum.", "Data", "Titolo"},
	},
	{
		ID: "217", Slug: "resoconti",
		Description: "Resoconti delle Sedute d'Aula",
		FieldMap: map[string]string{
			"legisl": "LEGISL",
			"anno":   "ANNSED",
			"numero": "NUMSED",
		},
		Columns: []string{"Legisl.", "Numero", "Data", "Argomenti"},
	},
	{
		ID: "221", Slug: "ddl",
		Description: "Disegni di Legge",
		FieldMap: map[string]string{
			"legisl":     "LEGISL",
			"numero":     "NUMDDL",
			"firmatario": "FIRMAT",
			"materia":    "SETTOR",
			"iter":       "ITERST",
		},
		Columns: []string{"Legisl.", "Numero", "Data", "Firmatari", "Titolo"},
	},
	{
		ID: "226", Slug: "pareri",
		Description: "Pareri richiesti dal Governo Regionale",
		FieldMap: map[string]string{
			"legisl":      "LEGISL",
			"commissione": "COMMIS",
			"numero":      "NUMISC",
		},
		Columns: []string{"Legisl.", "Numero", "Commissione", "Oggetto"},
	},
	{
		ID: "229", Slug: "convocazioni",
		Description: "Convocazioni delle Commissioni",
		FieldMap: map[string]string{
			"legisl":      "LEGISL",
			"codcom":      "CODCOM",
			"commissione": "COMMIS",
			"numero":      "NUMINT",
		},
		Columns: []string{"Legisl.", "Commissione", "Data", "ODG"},
	},
	{
		ID: "230", Slug: "sommari",
		Description: "Sommari Lavori Commissioni",
		FieldMap: map[string]string{
			"legisl":      "LEGISL",
			"codcom":      "CODCOM",
			"commissione": "COMMIS",
			"numero":      "NUMSED",
			"presidente":  "PRESID",
		},
		Columns: []string{"Legisl.", "Commissione", "Data", "Numero", "Argomenti"},
	},
	{
		ID: "233", Slug: "interrogazioni",
		Description: "Interrogazioni Parlamentari",
		FieldMap: map[string]string{
			"legisl":     "LEGISL",
			"numero":     "NUMORD",
			"firmatario": "FIRMAT",
			"rubrica":    "RUBRIC",
		},
		Columns: []string{"Legisl.", "Numero", "Data", "Firmatari", "Titolo"},
	},
	{
		ID: "234", Slug: "interpellanze",
		Description: "Interpellanze Parlamentari",
		FieldMap: map[string]string{
			"legisl":     "LEGISL",
			"numero":     "NUMORD",
			"firmatario": "FIRMAT",
			"rubrica":    "RUBRIC",
		},
		Columns: []string{"Legisl.", "Numero", "Data", "Firmatari", "Titolo"},
	},
	{
		ID: "235", Slug: "mozioni",
		Description: "Mozioni Parlamentari",
		FieldMap: map[string]string{
			"legisl":     "LEGISL",
			"numero":     "NUMORD",
			"firmatario": "FIRMAT",
			"rubrica":    "RUBRIC",
		},
		Columns: []string{"Legisl.", "Numero", "Data", "Firmatari", "Titolo"},
	},
	{
		ID: "236", Slug: "odg",
		Description: "Ordini del Giorno",
		FieldMap: map[string]string{
			"legisl":     "LEGISL",
			"numero":     "NUMORD",
			"firmatario": "FIRMAT",
			"rubrica":    "RUBRIC",
		},
		Columns: []string{"Legisl.", "Numero", "Data", "Firmatari", "Titolo"},
	},
	{
		ID: "238", Slug: "risoluzioni",
		Description: "Risoluzioni Parlamentari",
		FieldMap: map[string]string{
			"legisl":      "LEGISL",
			"numero":      "NUMORD",
			"firmatario":  "FIRMAT",
			"commissione": "COMMIS",
		},
		Columns: []string{"Legisl.", "Numero", "Data", "Firmatari", "Titolo"},
	},
	{
		ID: "205", Slug: "biblioteca",
		Description: "Catalogo Bibliografico",
		FieldMap: map[string]string{
			"autore":   "AUTORE",
			"titolo":   "TITOLO",
			"soggetto": "SOGGET",
			"dewey":    "DEWEY",
			"isbn":     "ISBN",
		},
		Columns: []string{"Autore", "Titolo", "Anno"},
	},
}

// BySlug returns the archive matching slug, or nil when slug is unknown.
func BySlug(slug string) *Archive {
	slug = strings.ToLower(slug)
	for i := range All {
		if All[i].Slug == slug {
			return &All[i]
		}
	}
	return nil
}

// ByID returns the archive matching numeric icaDB id, or nil.
func ByID(id string) *Archive {
	for i := range All {
		if All[i].ID == id {
			return &All[i]
		}
	}
	return nil
}
