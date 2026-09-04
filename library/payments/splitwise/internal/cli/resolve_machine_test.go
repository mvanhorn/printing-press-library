package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestResolveNonTTYDefaultsToJSON(t *testing.T) {
	p := seedSyntheticStore(t, map[string]map[string]string{"get-groups": {"7": `{"id":7,"name":"Group Alpha"}`}})
	cmd := newResolveCmd(&rootFlags{})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"Group Alpha", "--type", "group", "--db", p})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v raw=%s", err, out.String())
	}
	if got["id"] != "7" {
		t.Fatalf("got=%v", got)
	}
}
