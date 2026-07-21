package namethatui

import (
	"encoding/json"
	"strings"
	"testing"
)

func push(t *testing.T, value string) string {
	t.Helper()
	b, err := json.Marshal([]any{1, value})
	if err != nil {
		t.Fatal(err)
	}
	return `self.__next_f.push(` + string(b) + `)`
}

func TestParseComponents(t *testing.T) {
	entries := `[{"slug":"button","platform":"react","name":"Button","api":[{"framework":"react","symbol":"Button"}],"debugPrompt":"debug it","parts":[{"id":"label","name":"Label","api":"ButtonLabel"}]},{"slug":"card","platform":"web","name":"Card","api":[],"parts":[]}]`
	for _, tc := range []struct {
		name    string
		page    []byte
		want    int
		wantErr bool
	}{
		{"entries", []byte("<script>" + push(t, "ignored") + push(t, `prefix {"entries":`+entries+`} suffix`) + "</script>"), 2, false},
		{"unrelated", []byte("<html><body>nothing</body></html>"), 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseComponents(tc.page, "https://namethatui.com")
			if (err != nil) != tc.wantErr || len(got) != tc.want {
				t.Fatalf("got %#v, %v", got, err)
			}
			if tc.want > 0 && (got[0].Name == "" || got[0].SourceURL == "" || len(got[0].API) == 0 || len(got[0].Parts) == 0) {
				t.Fatalf("incomplete component: %#v", got[0])
			}
			if tc.want > 0 && (got[0].DebugPrompt != "debug it" || got[0].Parts[0].API != "ButtonLabel") {
				t.Fatalf("schema fields were not preserved: %#v", got[0])
			}
		})
	}
}

func TestParseStyles(t *testing.T) {
	index := []byte(`<script type="application/ld+json">{"@type":"ItemList","itemListElement":[{"name":"Brutalist","url":"/styles/brutalist"}]}</script>`)
	styles, err := ParseStylesIndex(index, "https://namethatui.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(styles) != 1 || styles[0].Slug != "brutalist" {
		t.Fatalf("bad styles: %#v", styles)
	}
	page := []byte("<h1>Brutalist</h1><h2>Accessibility</h2><p>Strong contrast helps.</p><script>" + push(t, `{"signals":[{"id":"contrast","name":"Contrast"}]}`) + "</script>")
	style, err := ParseStylePage(page, styles[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(style.Signals) != 1 {
		t.Fatalf("signals: %#v", style.Signals)
	}
	if len(style.Sections) < 2 || style.Sections[1].Heading != "accessibility" || style.Sections[1].ContentHash == "" {
		t.Fatalf("sections: %#v", style.Sections)
	}
	empty, err := ParseStylePage([]byte("<h1>Plain</h1><script>"+push(t, `{"signals":[]}`)+"</script>"), styles[0])
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(empty)
	if strings.Contains(string(b), `"signals":null`) {
		t.Fatalf("null signals: %s", b)
	}
}
