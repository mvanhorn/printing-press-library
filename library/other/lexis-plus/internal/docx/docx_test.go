package docx

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteProducesValidPackage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "memo.docx")
	paras := []Para{
		{Style: "Title", Runs: []Run{{Text: "Shepard's Red Flag Memo"}}},
		{Style: "Normal", Runs: []Run{{Text: "Case: ", Bold: true}, {Text: "Royal Globe v. Superior Court", Italic: true}}},
		{Style: "Quote", Runs: []Run{{Text: "“less < than & more > than” \x01control"}}},
	}
	if err := Write(path, paras); err != nil {
		t.Fatalf("Write: %v", err)
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("not a zip: %v", err)
	}
	defer zr.Close()
	want := map[string]bool{"[Content_Types].xml": false, "_rels/.rels": false, "word/_rels/document.xml.rels": false, "word/styles.xml": false, "word/document.xml": false}
	var docXML string
	for _, f := range zr.File {
		if _, ok := want[f.Name]; !ok {
			continue
		}
		want[f.Name] = true
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		// every part must be well-formed XML
		dec := xml.NewDecoder(strings.NewReader(string(b)))
		for {
			if _, err := dec.Token(); err == io.EOF {
				break
			} else if err != nil {
				t.Fatalf("%s is not well-formed XML: %v", f.Name, err)
			}
		}
		if f.Name == "word/document.xml" {
			docXML = string(b)
		}
	}
	for name, ok := range want {
		if !ok {
			t.Errorf("missing part %s", name)
		}
	}
	for _, s := range []string{"Royal Globe v. Superior Court", `<w:pStyle w:val="Quote"/>`, "<w:b/>", "<w:i/>", "less &lt; than &amp; more &gt; than"} {
		if !strings.Contains(docXML, s) {
			t.Errorf("document.xml missing %q", s)
		}
	}
	if strings.Contains(docXML, "\x01") {
		t.Errorf("control character was not stripped")
	}
}
