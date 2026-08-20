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
	// Columns names the short-list table columns POSITIONALLY: entry i names
	// the i-th <div> of a result row, and len(Columns) tells the parser how
	// many divs to read. Names become the record's Fields keys (and hence
	// JSON/CSV keys), so they are normalized here rather than copied verbatim
	// from the portal's own headers: every archive's last column is "Titolo"
	// (it is the title block — <h3> title plus <p> excerpt), and the id/date
	// slots are "Numero"/"Data" whatever the portal calls them (N.Ord.,
	// N.Seduta, N.Foglio, Data Pres., ...). "" marks a real column that the
	// portal renders empty; it is a placeholder to keep later entries at the
	// right index, and is never emitted.
	//
	// Getting these wrong silently mislabels data (a sitting number surfacing
	// as "data", an author as "anno"), so they are verified against the live
	// portal headers — see the per-archive notes below.
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
		// Portal header: Legisl. | Atto | Docum. | Data | Titolo e Testo della Legge
		Columns: []string{"Legisl.", "Atto", "Docum.", "Data", "Titolo"},
	},
	{
		ID: "217", Slug: "resoconti",
		Description: "Resoconti delle Sedute d'Aula",
		FieldMap: map[string]string{
			"legisl": "LEGISL",
			"anno":   "ANNSED",
			"numero": "NUMSED",
			"data":   "DATSED",
		},
		// Portal header: Legisl. | N.Seduta | Data | <blank> | Argomento e Oratori
		// The previous 4-entry version undercounted the real 5 divs, so the
		// title block landed on an unnamed index and surfaced as "col5".
		Columns: []string{"Legisl.", "Numero", "Data", "", "Titolo"},
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
			// ddl has no year field of its own (unlike leggi.LEGANN or
			// resoconti.ANNSED): "anno" is qualified on DATPRE, the
			// presentation date, as a Jan-1..Dec-31 range. See
			// normalizeParams' "anno"/ddl case, which builds that range —
			// without it, --anno was silently unqualified and matched the
			// year as free text anywhere in the document.
			"anno": "DATPRE",
			// --data qualifies a specific presentation date (or range) on the
			// same DATPRE field; used by `deputato profilo --data` to bound a
			// deputy's acts to a period without guessing a large --limit.
			"data": "DATPRE",
		},
		// Portal header: Legisl. | Numero | Data | <blank> | Titolo e Identificazione del DDL
		// Index 3 was declared "Firmatari", but the portal renders no
		// signatory column here — it is the blank one. The name only ever
		// produced an always-empty "firmatari" CSV column; the real list
		// lives in the document (see --con-firmatari).
		Columns: []string{"Legisl.", "Numero", "Data", "", "Titolo"},
	},
	{
		ID: "226", Slug: "pareri",
		Description: "Pareri richiesti dal Governo Regionale",
		FieldMap: map[string]string{
			"legisl":      "LEGISL",
			"commissione": "COMMIS",
			"numero":      "NUMISC",
		},
		// Portal header: Legisl. | N.Isc. | Data Pres. | <blank> | Titolo e Primo Firmatario
		// The older 4-entry version undercounted the real 5 divs, so index 2
		// ("Commissione" back then) actually held the presentation date and
		// there was no "Data" field at all.
		Columns: []string{"Legisl.", "Numero", "Data", "", "Titolo"},
	},
	{
		ID: "229", Slug: "convocazioni",
		Description: "Convocazioni delle Commissioni",
		FieldMap: map[string]string{
			"legisl":      "LEGISL",
			"codcom":      "CODCOM",
			"commissione": "COMMIS",
			"numero":      "NUMINT",
			"data":        "DATSED",
		},
		// Portal header: Legisl. | Data | N.Foglio | <blank> | Ordine del Giorno e Commissione
		// Note the date comes BEFORE the number here, unlike every other
		// archive. The older version had both at the wrong index, so "data"
		// held the N.Foglio sitting number. There is no commissione column
		// at all — its name only appears in the title block's excerpt.
		Columns: []string{"Legisl.", "Data", "Numero", "", "Titolo"},
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
			"data":        "DATSED",
		},
		// Portal header: Legisl. | N.Seduta | Data | <blank> | Ordine del Giorno e Commissione
		// "Data" was already at the right index by coincidence; "Commissione"
		// was not — it held the seduta number, and the declared "Numero"
		// landed on the always-blank 4th column.
		Columns: []string{"Legisl.", "Numero", "Data", "", "Titolo"},
	},
	{
		ID: "233", Slug: "interrogazioni",
		Description: "Interrogazioni Parlamentari",
		FieldMap: map[string]string{
			"legisl":     "LEGISL",
			"numero":     "NUMORD",
			"firmatario": "FIRMAT",
			"rubrica":    "RUBRIC",
			"data":       "DATPRE",
		},
		// Portal header: Legisl. | N.Ord. | Data Pres. | <blank> | Titolo e Primo Firmatario
		// Index 3 was declared "Firmatari": the portal renders that column
		// empty and puts only the FIRST signatory in the title block's
		// excerpt. The full list lives in the document (--con-firmatari).
		Columns: []string{"Legisl.", "Numero", "Data", "", "Titolo"},
	},
	{
		ID: "234", Slug: "interpellanze",
		Description: "Interpellanze Parlamentari",
		FieldMap: map[string]string{
			"legisl":     "LEGISL",
			"numero":     "NUMORD",
			"firmatario": "FIRMAT",
			"rubrica":    "RUBRIC",
			"data":       "DATPRE",
		},
		// Portal header: Legisl. | N.Ord. | Data Pres. | <blank> | Titolo e Primo Firmatario
		// Index 3 was declared "Firmatari": the portal renders that column
		// empty and puts only the FIRST signatory in the title block's
		// excerpt. The full list lives in the document (--con-firmatari).
		Columns: []string{"Legisl.", "Numero", "Data", "", "Titolo"},
	},
	{
		ID: "235", Slug: "mozioni",
		Description: "Mozioni Parlamentari",
		FieldMap: map[string]string{
			"legisl":     "LEGISL",
			"numero":     "NUMORD",
			"firmatario": "FIRMAT",
			"rubrica":    "RUBRIC",
			"data":       "DATPRE",
		},
		// Portal header: Legisl. | N.Ord. | Data Pres. | <blank> | Titolo e Primo Firmatario
		// Index 3 was declared "Firmatari": the portal renders that column
		// empty and puts only the FIRST signatory in the title block's
		// excerpt. The full list lives in the document (--con-firmatari).
		Columns: []string{"Legisl.", "Numero", "Data", "", "Titolo"},
	},
	{
		ID: "236", Slug: "odg",
		Description: "Ordini del Giorno",
		FieldMap: map[string]string{
			"legisl":     "LEGISL",
			"numero":     "NUMORD",
			"firmatario": "FIRMAT",
			"rubrica":    "RUBRIC",
			"data":       "DATPRE",
		},
		// Portal header: Legisl. | N.Ord. | Data Pres. | <blank> | Titolo e Primo Firmatario
		// Index 3 was declared "Firmatari": the portal renders that column
		// empty and puts only the FIRST signatory in the title block's
		// excerpt. The full list lives in the document (--con-firmatari).
		Columns: []string{"Legisl.", "Numero", "Data", "", "Titolo"},
	},
	{
		ID: "238", Slug: "risoluzioni",
		Description: "Risoluzioni Parlamentari",
		FieldMap: map[string]string{
			"legisl":      "LEGISL",
			"numero":      "NUMORD",
			"firmatario":  "FIRMAT",
			"commissione": "COMMIS",
			"data":        "DATPRE",
		},
		// Portal header: Legisl. | N.Ord. | Data Pres. | <blank> | Titolo e Primo Firmatario
		// Index 3 was declared "Firmatari": the portal renders that column
		// empty and puts only the FIRST signatory in the title block's
		// excerpt. The full list lives in the document (--con-firmatari).
		Columns: []string{"Legisl.", "Numero", "Data", "", "Titolo"},
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
		// Portal header: - | - | Autore | <blank> | Titolo e Note Tipografiche
		// The first two columns are placeholders the portal renders empty.
		// The old 3-entry version was shifted two slots left, so the author
		// surfaced under "anno" and the title under an unnamed "col5"; there
		// is no year column at all. This also fixes biblioteca's sync ID,
		// which is derived from autore+titolo.
		Columns: []string{"", "", "Autore", "", "Titolo"},
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
