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
