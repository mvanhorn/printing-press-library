// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.

package icaroclient

import "testing"

// TestParseShortList_LastColumnHoldsTitle covers the shortList row parsing:
// the last column's Fields entry (named after the column label, e.g.
// "Titolo") must hold the title itself, not the raw div text with the title
// stripped out — that leftover is the excerpt, and it used to masquerade as
// the title in JSON/CSV output and in biblioteca's sync ID derivation.
func TestParseShortList_LastColumnHoldsTitle(t *testing.T) {
	arc := Archive{
		ID:   "221",
		Slug: "ddl",
		FieldMap: map[string]string{
			"legisl": "LEGISL",
			"numero": "NUMDDL",
		},
		Columns: []string{"Legisl.", "Numero", "Data", "Firmatari", "Titolo"},
	}
	body := `<html><body><ul id="shortListTable">
		<li class="intestazione">header</li>
		<li href="javascript: showDoc(2)">
			<div>18</div>
			<div>6381</div>
			<div>9.01.24</div>
			<div>Rossi Mario</div>
			<div><h3>Disposizioni varie e finanziarie</h3><p>ddl n. 638/A Stralcio del 09 gennaio 2024 XVIII Legislatura</p></div>
		</li>
	</ul></body></html>`

	recs, _, err := ParseShortList(body, arc, "https://dati.ars.sicilia.it")
	if err != nil {
		t.Fatalf("ParseShortList error: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("want 1 record, got %d: %+v", len(recs), recs)
	}
	r := recs[0]
	if r.Title != "Disposizioni varie e finanziarie" {
		t.Fatalf("Title = %q, want %q", r.Title, "Disposizioni varie e finanziarie")
	}
	if r.Excerpt != "ddl n. 638/A Stralcio del 09 gennaio 2024 XVIII Legislatura" {
		t.Fatalf("Excerpt = %q, want the portal designation", r.Excerpt)
	}
	if got := r.Fields["Titolo"]; got != r.Title {
		t.Fatalf(`Fields["Titolo"] = %q, want it to match Title %q (not the excerpt)`, got, r.Title)
	}
}

// TestParseDoc_TitleIsValueNotLabel covers the doc detail page: every field,
// including the title, is rendered as a ".blocchi_info" block — a ".title"
// <h3> LABEL followed by a ".testo_gestionale" VALUE sibling. Doc.Title used
// to be the first bare <h3> on the page, which is that label ("Titolo"), not
// the value.
func TestParseDoc_TitleIsValueNotLabel(t *testing.T) {
	arc := Archive{ID: "221", Slug: "ddl"}
	body := `<html><body>
		<div class="colonna_2">
			<div class="blocchi_info">
				<div class="title"><h3 class="">Titolo</h3></div>
				<div class="testo_gestionale">Costituzione osservatorio per l'intelligenza artificiale in Sicilia</div>
			</div>
			<div class="blocchi_info">
				<div class="title"><h3 class="">Iter</h3></div>
				<div class="testo_gestionale"><p>Attuale 08 lug 2026 Esaminato in commissione</p></div>
			</div>
		</div>
	</body></html>`

	doc, err := ParseDoc(body, arc, 1)
	if err != nil {
		t.Fatalf("ParseDoc error: %v", err)
	}
	want := "Costituzione osservatorio per l'intelligenza artificiale in Sicilia"
	if doc.Title != want {
		t.Fatalf("Title = %q, want %q", doc.Title, want)
	}
}

// Il DocID è la posizione nella short list della sessione, non l'identità del
// documento: con un'altra query lo stesso icaDocId apre un altro atto, e fuori
// sessione l'URL risponde 302. L'identificatore stabile è il `docno(N)` che la
// pagina usa per il proprio permalink, e sta dentro uno <script>: va letto
// dall'HTML grezzo, non dall'albero dei nodi.
func TestParseDoc_DocNoDalPermalink(t *testing.T) {
	arc := Archive{ID: "221", Slug: "ddl"}
	body := `<html><body>
		<h3 id="permalink">Link diretto al documento</h3>
		<script>
			document.getElementById("permalink").addEventListener("click", function() {
				const url = "https://dati.ars.sicilia.it/icaro/default.jsp?icaDB=221&icaQuery=docno(9513)"
			});
		</script>
	</body></html>`

	doc, err := ParseDoc(body, arc, 1)
	if err != nil {
		t.Fatalf("ParseDoc error: %v", err)
	}
	if doc.DocNo != 9513 {
		t.Fatalf("DocNo = %d, want 9513", doc.DocNo)
	}
	// Una pagina senza permalink non deve inventarsi uno zero: `omitempty` lo
	// tiene fuori dall'output invece di far credere che il documento sia il #0.
	senza, err := ParseDoc(`<html><body><p>niente</p></body></html>`, arc, 1)
	if err != nil {
		t.Fatalf("ParseDoc error: %v", err)
	}
	if senza.DocNo != 0 {
		t.Fatalf("DocNo = %d, want 0 quando il permalink non c'è", senza.DocNo)
	}
}

// Il permalink deve essere quello che il portale mette dietro il proprio
// bottone: cambiando forma smette di riaprire il documento.
func TestPermalinkURL(t *testing.T) {
	got := PermalinkURL("https://dati.ars.sicilia.it", "221", 9513)
	want := "https://dati.ars.sicilia.it/icaro/default.jsp?icaDB=221&icaQuery=docno%289513%29"
	if got != want {
		t.Fatalf("PermalinkURL = %q, want %q", got, want)
	}
}
