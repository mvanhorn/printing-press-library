package journal

import (
	"encoding/json"
	"os"
	"testing"
)

func TestAppendRedactsSecrets(t *testing.T) {
	p := t.TempDir() + "/j"
	j := Journal{Path: p}
	if err := j.Append(map[string]any{"operation": "x", "token": "abc", "nested": map[string]any{"password": "pw", "ok": "yes"}}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["token"]; ok {
		t.Fatal("token leaked")
	}
	if got["operation"] != "x" {
		t.Fatal("record missing")
	}
}
