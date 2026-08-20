package icaroclient

import (
	"testing"
)

func TestBuildQuery_EmptyParams(t *testing.T) {
	arc := Archive{
		ID:   "221",
		Slug: "ddl",
		FieldMap: map[string]string{
			"legisl": "LEGISL",
			"anno":   "DDLANN",
		},
	}
	got := BuildQuery(arc, nil, "")
	if got != "all" {
		t.Errorf("BuildQuery(empty) = %q, want %q", got, "all")
	}
}

func TestBuildQuery_SingleField(t *testing.T) {
	arc := Archive{
		ID:   "221",
		Slug: "ddl",
		FieldMap: map[string]string{
			"legisl": "LEGISL",
			"anno":   "DDLANN",
		},
	}
	got := BuildQuery(arc, map[string]string{"legisl": "18"}, "")
	want := "(18.LEGISL)"
	if got != want {
		t.Errorf("BuildQuery(legisl=18) = %q, want %q", got, want)
	}
}

func TestBuildQuery_MultipleFields(t *testing.T) {
	arc := Archive{
		ID:   "221",
		Slug: "ddl",
		FieldMap: map[string]string{
			"legisl": "LEGISL",
			"anno":   "DDLANN",
		},
	}
	got := BuildQuery(arc, map[string]string{"anno": "2024", "legisl": "18"}, "")
	// Keys are sorted: anno < legisl
	want := "(2024.DDLANN E 18.LEGISL)"
	if got != want {
		t.Errorf("BuildQuery(anno=2024,legisl=18) = %q, want %q", got, want)
	}
}

func TestBuildQuery_FreeText(t *testing.T) {
	arc := Archive{
		ID:       "221",
		Slug:     "ddl",
		FieldMap: map[string]string{},
	}
	got := BuildQuery(arc, map[string]string{"testo": "bilancio"}, "")
	want := "(bilancio)"
	if got != want {
		t.Errorf("BuildQuery(testo=bilancio) = %q, want %q", got, want)
	}
}

// --frase cerca la locuzione: le parole devono essere adiacenti e nell'ordine
// dato. È la differenza fra trovare un ddl che parla di "aree idonee" e trovarne
// uno che ha "aree" in un articolo e "idonee" in un altro.
func TestBuildQuery_Frase(t *testing.T) {
	arc := Archive{ID: "221", Slug: "ddl", FieldMap: map[string]string{"legisl": "LEGISL"}}
	casi := []struct {
		nome   string
		params map[string]string
		want   string
	}{
		{"due parole", map[string]string{"frase": "aree idonee"}, "(aree adj idonee)"},
		{"tre parole", map[string]string{"frase": "obiezione di coscienza"}, "(obiezione adj di adj coscienza)"},
		{"una parola sola: nessuna adiacenza", map[string]string{"frase": "rifiuti"}, "(rifiuti)"},
		{"con un campo", map[string]string{"legisl": "18", "frase": "aree idonee"}, "(18.LEGISL) E (aree adj idonee)"},
		{"testo resta in AND", map[string]string{"testo": "aree idonee"}, "(aree E idonee)"},
	}
	for _, c := range casi {
		t.Run(c.nome, func(t *testing.T) {
			if got := BuildQuery(arc, c.params, ""); got != c.want {
				t.Errorf("BuildQuery(%v) = %q, want %q", c.params, got, c.want)
			}
		})
	}
}

// Un valore che contiene già operatori o parentesi è un'espressione scritta da
// chi chiama: va passata intatta, non spezzata in adiacenze.
func TestAdjJoinWords_PassaEspressioni(t *testing.T) {
	for _, in := range []string{"(aree idonee)", "aree E idonee", "aree NOT idonee"} {
		if got := adjJoinWords(in); got != in {
			t.Errorf("adjJoinWords(%q) = %q, atteso invariato", in, got)
		}
	}
}

func TestBuildQuery_ISISRaw(t *testing.T) {
	arc := Archive{
		ID:   "221",
		Slug: "ddl",
		FieldMap: map[string]string{
			"legisl": "LEGISL",
		},
	}
	raw := "18.LEGISL E 1500.DDLNUM"
	got := BuildQuery(arc, map[string]string{"legisl": "99"}, raw)
	if got != raw {
		t.Errorf("BuildQuery with isisRaw = %q, want %q", got, raw)
	}
}

func TestBuildQuery_ValueWithSpaceIsQuoted(t *testing.T) {
	arc := Archive{
		ID:       "221",
		Slug:     "ddl",
		FieldMap: map[string]string{"firmatario": "FIRMAT"},
	}
	got := BuildQuery(arc, map[string]string{"firmatario": "Rossi Mario"}, "")
	want := "((Rossi Mario).FIRMAT)"
	if got != want {
		t.Errorf("BuildQuery(firmatario='Rossi Mario') = %q, want %q", got, want)
	}
}

func TestNeedsQuoting(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"18", false},
		{"bilancio", false},
		{"Rossi Mario", true},
		{"(test)", true},
		{"3.14", true},
	}
	for _, c := range cases {
		got := needsQuoting(c.input)
		if got != c.want {
			t.Errorf("needsQuoting(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestBySlug(t *testing.T) {
	for _, arc := range All {
		got := BySlug(arc.Slug)
		if got == nil {
			t.Errorf("BySlug(%q) returned nil", arc.Slug)
			continue
		}
		if got.Slug != arc.Slug {
			t.Errorf("BySlug(%q).Slug = %q, want %q", arc.Slug, got.Slug, arc.Slug)
		}
	}
}

func TestBySlug_Unknown(t *testing.T) {
	got := BySlug("nonexistent-archive")
	if got != nil {
		t.Errorf("BySlug(unknown) = %v, want nil", got)
	}
}

// TestDataQualifiesOnDatpre covers the --data flag mapping added for
// `deputato profilo --data`: on ddl and the atti-parlamentari archives the
// presentation date is qualified on DATPRE (there is no dedicated data column
// in the short-list, but DATPRE is queryable upstream).
func TestDataQualifiesOnDatpre(t *testing.T) {
	for _, slug := range []string{"ddl", "interrogazioni", "interpellanze", "mozioni", "odg", "risoluzioni"} {
		arc := BySlug(slug)
		if arc == nil {
			t.Fatalf("BySlug(%q) nil", slug)
		}
		if arc.FieldMap["data"] != "DATPRE" {
			t.Errorf("%s: FieldMap[data] = %q, want DATPRE", slug, arc.FieldMap["data"])
		}
	}
}

func TestBuildQuery_Escludi(t *testing.T) {
	arc := Archive{
		ID:       "233",
		Slug:     "interrogazioni",
		FieldMap: map[string]string{"legisl": "LEGISL"},
	}
	got := BuildQuery(arc, map[string]string{"legisl": "18", "testo": "sanità", "escludi": "ospedale"}, "")
	want := "((18.LEGISL) E (sanità)) NOT (ospedale)"
	if got != want {
		t.Errorf("BuildQuery(escludi) = %q, want %q", got, want)
	}
}

func TestBuildQuery_EscludiOnly(t *testing.T) {
	arc := Archive{Slug: "ddl", FieldMap: map[string]string{}}
	got := BuildQuery(arc, map[string]string{"escludi": "regole"}, "")
	want := "(all) NOT (regole)"
	if got != want {
		t.Errorf("BuildQuery(escludi only) = %q, want %q", got, want)
	}
}

func TestBuildQuery_FreeTextAND(t *testing.T) {
	arc := Archive{Slug: "leggi", FieldMap: map[string]string{"legisl": "LEGISL"}}
	got := BuildQuery(arc, map[string]string{"legisl": "18", "testo": "obiezione di coscienza"}, "")
	want := "(18.LEGISL) E (obiezione E di E coscienza)"
	if got != want {
		t.Errorf("BuildQuery(multi-word testo) = %q, want %q", got, want)
	}
}

func TestBuildQuery_FreeTextOperatorPassthrough(t *testing.T) {
	arc := Archive{Slug: "x", FieldMap: map[string]string{}}
	for _, v := range []string{"sanità NOT ospedale", "a OR b", "(a b) c"} {
		got := BuildQuery(arc, map[string]string{"testo": v}, "")
		if got != "("+v+")" {
			t.Errorf("BuildQuery(testo=%q) = %q, want verbatim", v, got)
		}
	}
}

func TestBuildQuery_FreeTextSingleWord(t *testing.T) {
	arc := Archive{Slug: "x", FieldMap: map[string]string{}}
	if got := BuildQuery(arc, map[string]string{"testo": "rifiuti"}, ""); got != "(rifiuti)" {
		t.Errorf("single word = %q, want (rifiuti)", got)
	}
}
