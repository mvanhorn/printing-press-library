package cli

import (
	"encoding/json"
	"testing"
)

func TestCompactHeadlineItemsKeepsHighGravityFields(t *testing.T) {
	raw, _ := json.Marshal([]map[string]any{{
		"title": "A", "link": "https://x", "published": "2026-01-01",
		"section": "latest", "categories": []string{"a"}, "description": "long",
	}})
	out, err := compactResultItems(raw)
	if err != nil {
		t.Fatal(err)
	}
	var items []map[string]any
	if err := json.Unmarshal(out, &items); err != nil {
		t.Fatal(err)
	}
	if _, ok := items[0]["categories"]; ok {
		t.Fatal("compact should drop categories")
	}
	if items[0]["title"] != "A" || items[0]["link"] != "https://x" {
		t.Fatalf("got %#v", items[0])
	}
}

func TestProcessResultJSONSelectWinsOverCompact(t *testing.T) {
	raw, _ := json.Marshal([]map[string]any{{"title": "A", "description": "d"}})
	flags := &rootFlags{compact: true, selectFields: "description"}
	out, err := processResultJSON(raw, flags)
	if err != nil {
		t.Fatal(err)
	}
	var items []map[string]any
	json.Unmarshal(out, &items)
	if len(items) != 1 || items[0]["description"] != "d" {
		t.Fatalf("select should win: %#v", items)
	}
	if _, ok := items[0]["title"]; ok {
		t.Fatal("select should not include title")
	}
}

func TestResponseEnvelopeShape(t *testing.T) {
	flags := &rootFlags{compact: true}
	meta := map[string]any{"source": "live", "section": "latest"}
	items := []map[string]any{{"title": "Hi", "link": "https://x", "section": "latest"}}
	// Marshal path used by printMachineOutput
	raw, _ := json.Marshal(items)
	processed, _ := processResultJSON(raw, flags)
	var parsed any
	json.Unmarshal(processed, &parsed)
	env := responseEnvelope{Meta: meta, Results: parsed}
	out, _ := json.Marshal(env)
	var decoded map[string]any
	json.Unmarshal(out, &decoded)
	if decoded["meta"] == nil || decoded["results"] == nil {
		t.Fatalf("envelope: %#v", decoded)
	}
}
