// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package mcp

import (
	"context"
	"encoding/json"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/internal/hgj"
	"github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/internal/store"
	"strings"
	"testing"
	"time"
)

func TestHGJInspectionToolsHaveRequiredIDAndHonestMutationHints(t *testing.T) {
	s := server.NewMCPServer("test", "test")
	RegisterTools(s)
	for _, name := range []string{"restaurants_get", "prayer_get"} {
		entry := s.GetTool(name)
		if entry == nil {
			t.Fatal(name)
		}
		raw, _ := json.Marshal(entry.Tool)
		var tool map[string]any
		json.Unmarshal(raw, &tool)
		hints := tool["annotations"].(map[string]any)
		for k, want := range map[string]bool{"readOnlyHint": false, "destructiveHint": false, "openWorldHint": true, "idempotentHint": false} {
			if hints[k] != want {
				t.Fatalf("%s %s=%v", name, k, hints[k])
			}
		}
		required := tool["inputSchema"].(map[string]any)["required"].([]any)
		found := false
		for _, v := range required {
			if v == "id" {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s ID optional", name)
		}
	}
}
func TestHGJSQLDistinguishesSavedDataFromNoMatchingRows(t *testing.T) {
	resetMCPPathEnv(t)
	path, err := mcpDBPath()
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p := hgj.Place{ID: "300739", Kind: hgj.Restaurant, Name: "Fixture restaurant", SourceURL: hgj.Origin + "/restaurant/300739", EvidenceScope: "detail", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if err = hgj.SaveSnapshot(context.Background(), db.DB(), p); err != nil {
		t.Fatal(err)
	}
	db.Close()
	for _, tc := range []struct {
		query string
		count float64
	}{{"SELECT id FROM hgj_detail_snapshots WHERE position=0", 1}, {"SELECT id FROM hgj_detail_snapshots WHERE id='999999'", 0}} {
		result := callMCPSQL(t, context.Background(), tc.query)
		if result.IsError {
			t.Fatal(mcpTextContent(t, result))
		}
		var out map[string]any
		json.Unmarshal([]byte(mcpTextContent(t, result)), &out)
		if out["store_status"] != "ready" || out["count"] != tc.count {
			t.Fatalf("SQL evidence=%v", out)
		}
		if tc.count == 0 {
			next, _ := out["next_step"].(string)
			if !strings.Contains(next, "Saved observations exist") || strings.Contains(next, "store is empty") {
				t.Fatal("no matches mislabeled empty database")
			}
		}
	}
}
