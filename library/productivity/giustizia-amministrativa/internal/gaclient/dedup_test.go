package gaclient

import "testing"

func TestDedupBySnippet(t *testing.T) {
	items := []Provvedimento{
		{Ecli: "A", Snippet: "...abbia ridotto eccessivamente i margini di utile che le imprese"},
		{Ecli: "B", Snippet: "contenuto unico sulla responsabilita' precontrattuale dell'appaltatore"},
		{Ecli: "C", Snippet: "...abbia ridotto eccessivamente i margini di utile che le imprese"},
		{Ecli: "D", Snippet: "...abbia ridotto eccessivamente i margini di utile che le imprese"},
		{Ecli: "E", Snippet: "altro contenuto distinto sull'accesso civico generalizzato"},
	}

	out, grouped := dedupBySnippet(items)

	if grouped != 2 {
		t.Fatalf("grouped = %d, want 2", grouped)
	}
	if len(out) != 3 {
		t.Fatalf("len(out) = %d, want 3", len(out))
	}
	if out[0].Ecli != "A" || out[0].Duplicati != 3 {
		t.Errorf("rep = %s duplicati=%d, want A/3", out[0].Ecli, out[0].Duplicati)
	}
	if out[1].Duplicati != 0 {
		t.Errorf("unique item B has duplicati=%d, want 0", out[1].Duplicati)
	}
	if out[2].Duplicati != 0 {
		t.Errorf("unique item E has duplicati=%d, want 0", out[2].Duplicati)
	}
}

func TestDedupBySnippetShortSnippetNotGrouped(t *testing.T) {
	items := []Provvedimento{
		{Ecli: "A", Snippet: "corto"},
		{Ecli: "B", Snippet: "corto"},
	}
	out, grouped := dedupBySnippet(items)
	if grouped != 0 {
		t.Fatalf("grouped = %d, want 0 (snippet too short)", grouped)
	}
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2", len(out))
	}
}

func TestDedupBySnippetWhitespaceNormalized(t *testing.T) {
	items := []Provvedimento{
		{Ecli: "A", Snippet: "  ...stesso   snippet\tcon\r\nspazi  variabili  "},
		{Ecli: "B", Snippet: "...stesso snippet con spazi variabili"},
	}
	out, grouped := dedupBySnippet(items)
	if grouped != 1 {
		t.Fatalf("grouped = %d, want 1", grouped)
	}
	if out[0].Duplicati != 2 {
		t.Errorf("duplicati = %d, want 2", out[0].Duplicati)
	}
}

func TestDedupBySnippetEmpty(t *testing.T) {
	out, grouped := dedupBySnippet(nil)
	if grouped != 0 || len(out) != 0 {
		t.Fatalf("empty input: out=%d grouped=%d, want 0/0", len(out), grouped)
	}
}

func TestApplySnippetDedupAddsWarning(t *testing.T) {
	res := &SearchResult{
		Items: []Provvedimento{
			{Ecli: "A", Snippet: "...snippet identico abbastanza lungo da superare la soglia minima"},
			{Ecli: "B", Snippet: "...snippet identico abbastanza lungo da superare la soglia minima"},
		},
		Total: 2,
	}
	out := applySnippetDedup(res)
	if len(out.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(out.Items))
	}
	if len(out.Warnings) != 1 {
		t.Fatalf("warnings = %d, want 1", len(out.Warnings))
	}
	if out.Items[0].Duplicati != 2 {
		t.Errorf("duplicati = %d, want 2", out.Items[0].Duplicati)
	}
}

func TestApplySnippetDedupNilSafe(t *testing.T) {
	out := applySnippetDedup(nil)
	if out != nil {
		t.Fatalf("nil input should return nil")
	}
}
