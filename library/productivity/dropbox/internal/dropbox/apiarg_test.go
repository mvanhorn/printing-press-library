package dropbox

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEncodeAPIArg(t *testing.T) {
	input := map[string]string{"path": "/Fotos/Ñandú 📷.jpg"}
	got, err := EncodeAPIArg(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`\u00d1`, `\u00fa`, `\ud83d\udcf7`} {
		if !strings.Contains(got, want) {
			t.Errorf("%q missing %q", got, want)
		}
	}
	for _, r := range got {
		if r > 0x7e {
			t.Errorf("non-ASCII rune %U", r)
		}
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["path"] != input["path"] {
		t.Fatalf("round trip = %q", decoded["path"])
	}
	ascii, err := EncodeAPIArg(map[string]string{"path": "/Photos/a.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	if ascii != `{"path":"/Photos/a.jpg"}` {
		t.Fatalf("ASCII changed: %s", ascii)
	}
}
