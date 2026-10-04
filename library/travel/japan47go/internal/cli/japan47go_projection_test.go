// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"bytes"
	"encoding/json"
	"github.com/spf13/cobra"
	"testing"
)

func TestServiceProjectionKeepsEnvelopeAndRequestedFacts(t *testing.T) {
	record := map[string]any{"id": "example", "name_ja": "local guide", "request": map[string]any{"status": "known", "lead_times": []map[string]any{{"value": 10, "unit": "days"}}}, "description_evidence": "verbose text", "durations": []map[string]any{{"min_minutes": 30}}}
	for _, key := range []string{"service", "candidates", "comparisons"} {
		t.Run(key, func(t *testing.T) {
			var value any = record
			if key == "candidates" {
				value = []any{record}
			} else if key == "comparisons" {
				value = []any{map[string]any{"service": record, "notice": map[string]any{"state": "excluded"}}}
			}
			out := bytes.Buffer{}
			cmd := &cobra.Command{}
			cmd.SetOut(&out)
			f := &rootFlags{asJSON: true, agent: true, compact: true, selectFields: "id,name_ja,request"}
			e := serviceOutput(cmd, f, map[string]any{key: value, "coverage": map[string]any{"complete": false}, "source_boundary": "published facts"})
			if e != nil {
				t.Fatal(e)
			}
			var root map[string]json.RawMessage
			if e = json.Unmarshal(out.Bytes(), &root); e != nil {
				t.Fatal(e)
			}
			var result map[string]json.RawMessage
			if e = json.Unmarshal(root["results"], &result); e != nil {
				t.Fatal(e)
			}
			if result["coverage"] == nil || result["source_boundary"] == nil {
				t.Fatal("lost provenance")
			}
			raw := result[key]
			if key != "service" {
				var xs []json.RawMessage
				json.Unmarshal(raw, &xs)
				raw = xs[0]
			}
			if key == "comparisons" {
				var c map[string]json.RawMessage
				json.Unmarshal(raw, &c)
				if c["notice"] == nil {
					t.Fatal("lost notice state")
				}
				raw = c["service"]
			}
			var got map[string]json.RawMessage
			json.Unmarshal(raw, &got)
			if len(got) != 3 || got["id"] == nil || got["name_ja"] == nil || got["request"] == nil {
				t.Fatalf("bad projection %s", raw)
			}
			if !bytes.Contains(got["request"], []byte("10")) {
				t.Fatal("nested selected object lost")
			}
		})
	}
}
