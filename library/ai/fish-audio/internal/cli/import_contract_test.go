package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/client"
)

func TestImportRejectsUnsupportedResourcesBeforeClientOrRequest(t *testing.T) {
	previousHooks := clientHooks
	t.Cleanup(func() { clientHooks = previousHooks })
	clients := 0
	clientHooks = append(clientHooks, func(*client.Client) error {
		clients++
		return nil
	})
	for _, tc := range []struct{ resource, guidance string }{
		{"asr", "asr transcribe --audio"},
		{"model", "voice clone --title"},
		{"tts", "tts render --text"},
		{"voice-design", "voice design --instruction"},
	} {
		t.Run(tc.resource, func(t *testing.T) {
			_, _, err := runCLI(t, "import", tc.resource, "--input", "-", "--no-learn")
			if err == nil || !strings.Contains(err.Error(), tc.guidance) {
				t.Fatalf("import %s error = %v, want %q guidance", tc.resource, err, tc.guidance)
			}
			if clients != 0 {
				t.Fatalf("import %s constructed %d client(s) before refusal", tc.resource, clients)
			}
		})
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
