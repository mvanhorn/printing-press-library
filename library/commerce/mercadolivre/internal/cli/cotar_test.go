// Copyright 2026 wandreis and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavior tests for the cotar classification + Markdown rendering, exercised
// on a pure row slice so no live fetch is needed.

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// furadeiraRows returns the three seeded furadeira candidates with sellers and
// delivery windows, mirroring the search_page.html fixture.
func furadeiraRows() []cotacaoRow {
	return []cotacaoRow{
		{CatalogID: "MLB1001", Name: "Furadeira Bosch GSB 550W", Brand: "Bosch", Seller: "Loja Oficial Bosch",
			Price: 349.90, Currency: "BRL", RatingValue: 4.7, DeliveryMinDays: 2, DeliveryMaxDays: 5, HasDelivery: true},
		{CatalogID: "MLB1002", Name: "Furadeira DeWalt 700W", Brand: "DeWalt", Seller: "Ferramentas Vikings",
			Price: 529.00, Currency: "BRL", RatingValue: 4.8, DeliveryMinDays: 1, DeliveryMaxDays: 3, HasDelivery: true},
		{CatalogID: "MLB1003", Name: "Furadeira Bosch GSB 13 RE 650W", Brand: "Bosch", Seller: "Mercado Livre",
			Price: 299.00, Currency: "BRL", RatingValue: 0},
	}
}

func baseOpts() cotacaoOpts {
	return cotacaoOpts{Termo: "furadeira", Data: "2026-07-02 15:04", Currency: "BRL"}
}

// section returns the block of md starting at the "## label" heading up to the
// next "## " heading (or EOF).
func section(md, label string) string {
	start := strings.Index(md, "## "+label)
	if start < 0 {
		return ""
	}
	rest := md[start+len("## "+label):]
	if next := strings.Index(rest, "\n## "); next >= 0 {
		return rest[:next]
	}
	return rest
}

func countRows(block string) int {
	n := 0
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		// Data rows start with "|" and are not the header or separator.
		if strings.HasPrefix(line, "|") && !strings.Contains(line, "Produto") && !strings.HasPrefix(line, "|---") {
			n++
		}
	}
	return n
}

func TestRenderCotacaoPorMarca(t *testing.T) {
	opts := baseOpts()
	opts.GroupBy = []string{"marca"}
	md := renderCotacao(furadeiraRows(), opts)
	t.Logf("\n%s", md)

	if !strings.Contains(md, "## Bosch") {
		t.Fatalf("expected a Bosch section:\n%s", md)
	}
	if !strings.Contains(md, "## DeWalt") {
		t.Fatalf("expected a DeWalt section:\n%s", md)
	}
	bosch := section(md, "Bosch")
	if got := countRows(bosch); got != 2 {
		t.Errorf("Bosch section rows = %d, want 2:\n%s", got, bosch)
	}
	dewalt := section(md, "DeWalt")
	if got := countRows(dewalt); got != 1 {
		t.Errorf("DeWalt section rows = %d, want 1:\n%s", got, dewalt)
	}
	// Price-ascending within Bosch: MLB1003 (299,00) must appear before MLB1001 (349,90).
	i3 := strings.Index(bosch, "MLB1003")
	i1 := strings.Index(bosch, "MLB1001")
	if i3 < 0 || i1 < 0 || i3 > i1 {
		t.Errorf("Bosch rows not price-ascending (MLB1003 should precede MLB1001):\n%s", bosch)
	}
	if !strings.Contains(md, cotarFooter) {
		t.Errorf("footer missing:\n%s", md)
	}
	// Bosch appears before DeWalt (labels sorted).
	if strings.Index(md, "## Bosch") > strings.Index(md, "## DeWalt") {
		t.Errorf("brand sections not sorted (Bosch should precede DeWalt)")
	}
}

func TestRenderCotacaoPorPrazo(t *testing.T) {
	opts := baseOpts()
	opts.GroupBy = []string{"prazo"}
	md := renderCotacao(furadeiraRows(), opts)
	t.Logf("\n%s", md)

	// Ordered by max delivery: MLB1002 (max 3) before MLB1001 (max 5).
	i2 := strings.Index(md, "MLB1002")
	i1 := strings.Index(md, "MLB1001")
	if i2 < 0 || i1 < 0 || i2 > i1 {
		t.Errorf("prazo order wrong: MLB1002 (3d) should precede MLB1001 (5d):\n%s", md)
	}
	// MLB1003 has no consulted prazo -> trailing "prazo não consultado" section.
	if !strings.Contains(md, "## prazo não consultado") {
		t.Errorf("expected 'prazo não consultado' section:\n%s", md)
	}
	naoConsultado := section(md, "prazo não consultado")
	if !strings.Contains(naoConsultado, "MLB1003") {
		t.Errorf("MLB1003 should be in the não-consultado section:\n%s", naoConsultado)
	}
	// A row without prazo shows the em dash in the Prazo column.
	if !strings.Contains(naoConsultado, "—") {
		t.Errorf("row without prazo should show '—':\n%s", naoConsultado)
	}
	if !strings.Contains(md, cotarFooter) {
		t.Errorf("footer missing:\n%s", md)
	}
}

func TestRenderCotacaoPrecoDefault(t *testing.T) {
	opts := baseOpts()
	opts.GroupBy = []string{"preco"}
	md := renderCotacao(furadeiraRows(), opts)
	// Single table, cheapest first: MLB1003 (299) < MLB1001 (349,90) < MLB1002 (529).
	i3 := strings.Index(md, "MLB1003")
	i1 := strings.Index(md, "MLB1001")
	i2 := strings.Index(md, "MLB1002")
	if !(i3 < i1 && i1 < i2) {
		t.Errorf("preco order wrong, want MLB1003<MLB1001<MLB1002, got %d/%d/%d\n%s", i3, i1, i2, md)
	}
	// pt-BR currency formatting.
	if !strings.Contains(md, "R$ 349,90") {
		t.Errorf("expected pt-BR price 'R$ 349,90':\n%s", md)
	}
}

func TestResolveCotarOutPathEmptyWritesTempDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MERCADOLIVRE_DATA_DIR", tmp)

	path, err := resolveCotarOutPath("", "furadeira de impacto", "20260702-150405")
	if err != nil {
		t.Fatalf("resolveCotarOutPath: %v", err)
	}
	if !strings.HasPrefix(path, tmp) {
		t.Errorf("derived path %q not under temp data dir %q", path, tmp)
	}
	want := filepath.Join("cotacoes", "furadeira-de-impacto-20260702-150405.md")
	if !strings.HasSuffix(path, want) {
		t.Errorf("derived path %q does not end with %q", path, want)
	}

	md := renderCotacao(furadeiraRows(), baseOpts())
	if err := writeCotarFile(path, md); err != nil {
		t.Fatalf("writeCotarFile: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file written at %q: %v", path, err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if !strings.Contains(string(body), cotarFooter) {
		t.Errorf("written file missing footer")
	}
}

func TestResolveCotarOutPathExplicit(t *testing.T) {
	explicit := filepath.Join(t.TempDir(), "minha-cotacao.md")
	path, err := resolveCotarOutPath(explicit, "furadeira", "20260702-150405")
	if err != nil {
		t.Fatalf("resolveCotarOutPath explicit: %v", err)
	}
	if path != explicit {
		t.Errorf("explicit --out not honored: got %q, want %q", path, explicit)
	}
}

func TestParseCotarPor(t *testing.T) {
	cases := []struct {
		in      string
		want    []string
		wantErr bool
	}{
		{"", []string{"preco"}, false},
		{"marca", []string{"marca"}, false},
		{"marca,fornecedor", []string{"marca", "fornecedor"}, false},
		{"PRAZO", []string{"prazo"}, false},
		{"bogus", nil, true},
	}
	for _, c := range cases {
		got, err := parseCotarPor(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseCotarPor(%q) expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseCotarPor(%q) error: %v", c.in, err)
			continue
		}
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("parseCotarPor(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
