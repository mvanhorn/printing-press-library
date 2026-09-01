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

// La congiunzione italiana dentro una locuzione non è un operatore: «coesione
// e crescita» è il titolo di una manovra, non un AND. Prima la si vedeva e si
// usciva verbatim, cioè il flag prometteva una locuzione e consegnava un AND.
func TestBuildQuery_FraseConCongiunzione(t *testing.T) {
	arc := Archive{ID: "221", Slug: "ddl", FieldMap: map[string]string{"legisl": "LEGISL"}}
	casi := []struct {
		nome     string
		frase    string
		want     string
		scartati []string
	}{
		{"una congiunzione", "coesione e crescita", "coesione adj2 crescita", []string{"e"}},
		{"due di fila", "sanità e o ambiente", "sanità adj3 ambiente", []string{"e", "o"}},
		{"due congiunzioni separate", "case e cura o salute", "case adj2 cura adj2 salute", []string{"e", "o"}},
		{"in testa", "e crescita", "crescita", []string{"e"}},
		{"nessuna collisione", "aree idonee", "aree adj idonee", nil},
	}
	for _, c := range casi {
		t.Run(c.nome, func(t *testing.T) {
			expr, scartati, collisioni := FraseDegradata(c.frase)
			if len(collisioni) > 0 {
				t.Fatalf("collisioni inattese: %v", collisioni)
			}
			if expr != c.want {
				t.Errorf("FraseDegradata(%q) = %q, want %q", c.frase, expr, c.want)
			}
			if len(scartati) != len(c.scartati) {
				t.Fatalf("scartati = %v, want %v", scartati, c.scartati)
			}
			for i := range scartati {
				if scartati[i] != c.scartati[i] {
					t.Errorf("scartati[%d] = %q, want %q", i, scartati[i], c.scartati[i])
				}
			}
			if got, want := BuildQuery(arc, map[string]string{"frase": c.frase}, ""), "("+c.want+")"; got != want {
				t.Errorf("BuildQuery(frase=%q) = %q, want %q", c.frase, got, want)
			}
		})
	}
}

// La maiuscola è il segnale che distingue chi scrive un'espressione booleana
// da chi cita un titolo: allentare la guardia sul minuscolo non deve toccare
// il primo. Una frase di sole congiunzioni non ha nulla da unire e resta com'è.
func TestFraseDegradata_EspressioniIntatte(t *testing.T) {
	for _, in := range []string{"(aree idonee)", "aree E idonee", "aree NOT idonee", "aree ADJ2 idonee", "e o"} {
		expr, scartati, _ := FraseDegradata(in)
		if expr != in || scartati != nil {
			t.Errorf("FraseDegradata(%q) = (%q, %v), atteso invariato e senza scarti", in, expr, scartati)
		}
	}
}

// Un titolo copiato in stampatello non porta il segnale della maiuscola: la
// congiunzione va scartata come in minuscolo, altrimenti l'AND muto torna
// proprio sul caso che il fix esiste per coprire.
func TestFraseDegradata_TuttoMaiuscolo(t *testing.T) {
	expr, scartati, _ := FraseDegradata("SVILUPPO E COESIONE")
	if expr != "SVILUPPO adj2 COESIONE" {
		t.Errorf("FraseDegradata(maiuscolo) = %q, want %q", expr, "SVILUPPO adj2 COESIONE")
	}
	if len(scartati) != 1 || scartati[0] != "E" {
		t.Errorf("scartati = %v, want [E]", scartati)
	}
}

// Il vocabolario ISIS contiene parole piene dell'italiano — «seguito»,
// «vicino», «meno», «no», «escluso». Scartarle come si scarta una congiunzione
// non attenua la ricerca, la falsifica: «aree meno idonee» diventerebbe «aree
// idonee», il contrario di quel che si cerca. Lì la frase esce intatta e il
// chiamante riceve la collisione da dichiarare.
func TestFraseDegradata_ParolePieneNonSiScartano(t *testing.T) {
	casi := []struct {
		frase string
		quale string
	}{
		{"in seguito alla", "seguito"},
		{"aree meno idonee", "meno"},
		{"zone vicino al mare", "vicino"},
		{"personale escluso dal ruolo", "escluso"},
	}
	for _, c := range casi {
		t.Run(c.frase, func(t *testing.T) {
			expr, scartati, collisioni := FraseDegradata(c.frase)
			if expr != c.frase {
				t.Errorf("FraseDegradata(%q) = %q, atteso invariato", c.frase, expr)
			}
			if scartati != nil {
				t.Errorf("scartati = %v, atteso nessuno: una parola piena non si toglie", scartati)
			}
			if len(collisioni) != 1 || collisioni[0] != c.quale {
				t.Errorf("collisioni = %v, want [%s]", collisioni, c.quale)
			}
		})
	}
}
