package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestUnescapeJSONHTML(t *testing.T) {
	cases := map[string]string{
		`{"t":"Black \u0026 White"}`:   `{"t":"Black & White"}`,
		`{"t":"a \u003cb\u003E"}`:      `{"t":"a <b>"}`,
		`{"t":"keep \\u0026 literal"}`: `{"t":"keep \\u0026 literal"}`,
		`{"t":"caf\u00e9 \"q\""}`:      `{"t":"caf\u00e9 \"q\""}`,
		`{"n":1}`:                      `{"n":1}`,
	}
	for in, want := range cases {
		got := string(unescapeJSONHTML([]byte(in)))
		if got != want {
			t.Errorf("unescapeJSONHTML(%s) = %s, want %s", in, got, want)
		}
		if !json.Valid([]byte(got)) {
			t.Errorf("output is not valid JSON: %s", got)
		}
	}
}

func TestWriteEncodedJSONKeepsAmpersand(t *testing.T) {
	var buf bytes.Buffer
	if err := writeEncodedJSON(&buf, map[string]string{"title": "Tchibo Black & White"}, "", ""); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Black & White")) {
		t.Fatalf("ampersand was escaped: %s", buf.String())
	}
}
