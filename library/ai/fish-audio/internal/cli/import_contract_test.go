package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestImportRejectsASRJSONL(t *testing.T) {
	_, _, err := runCLI(t, "import", "asr", "--input", "-", "--no-learn")
	if err == nil || !strings.Contains(err.Error(), "requires a multipart audio upload") || !strings.Contains(err.Error(), "asr transcribe --audio") {
		t.Fatalf("import asr error = %v, want multipart guidance", err)
	}
}

func TestDecodeImportRecordKeepsExactNumbersAndOneObject(t *testing.T) {
	body, err := decodeImportRecord(`{"id":9007199254740993}`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	if got := string(encoded); got != `{"id":9007199254740993}` {
		t.Fatalf("re-encoded body = %s, integer precision changed", got)
	}
	for _, input := range []string{`null`, `[]`, `{"ok":true} {"extra":true}`, `{"ok":true} garbage`} {
		if _, err := decodeImportRecord(input); err == nil {
			t.Errorf("decodeImportRecord(%q) accepted non-record input", input)
		}
	}
}
