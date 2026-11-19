package cobratree

import (
	"context"
	"encoding/json"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"os"
	"path/filepath"
	"testing"
)

func TestE1LargeWrappedMirrorResumes(t *testing.T) {
	t.Setenv("PP_MCP_MAX_BYTES", "24000")
	dir := t.TempDir()
	payload := `{"meta":{"source":"local"},"outer":"kept","results":{"total_clusters":100,"clusters":` + largeJSONArray(t, 100, 1100) + `}}`
	path := filepath.Join(dir, "payload.json")
	if err := os.WriteFile(path, []byte(payload), 0600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "cli")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\ncat '"+path+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	run, err := RunCLICommand(context.Background(), bin, nil)
	if err != nil || run.Stdout != payload {
		t.Fatalf("stdout capture lost JSON: got %d want %d err=%v", len(run.Stdout), len(payload), err)
	}
	handler := shellOutToCLI(func() (string, error) { return bin, nil }, nil, blockedRootFlags, map[string]bool{"offset": true}, nil, true, nil)
	expected := 0
	for expected < 100 {
		result, err := handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"offset": float64(expected)}}})
		if err != nil || result.IsError {
			t.Fatalf("handler %v %v", result, err)
		}
		text := shelloutMCPTextContent(t, result)
		if len(text) > 24000 {
			t.Fatalf("page over budget: %d", len(text))
		}
		var page struct {
			Meta struct {
				Source string `json:"source"`
			} `json:"meta"`
			Outer   string `json:"outer"`
			Results struct {
				Total    int `json:"total_clusters"`
				Clusters []struct {
					ID int `json:"id"`
				} `json:"clusters"`
				Next     *int `json:"next_offset"`
				Returned int  `json:"returned_count"`
			} `json:"results"`
		}
		if err := json.Unmarshal([]byte(text), &page); err != nil {
			t.Fatal(err)
		}
		if page.Meta.Source != "local" || page.Outer != "kept" || page.Results.Total != 100 || len(page.Results.Clusters) == 0 {
			t.Fatalf("wrapper or siblings lost: %.250s", text)
		}
		for _, row := range page.Results.Clusters {
			if row.ID != expected+1 {
				t.Fatalf("gap: %d want %d", row.ID, expected+1)
			}
			expected++
		}
		if page.Results.Returned != len(page.Results.Clusters) {
			t.Fatal("wrong returned count")
		}
		if expected < 100 && (page.Results.Next == nil || *page.Results.Next != expected) {
			t.Fatalf("next_offset not resumable: %.250s", text)
		}
	}
}

func TestE1WrappedArrayResumes(t *testing.T) {
	payload := `{"meta":{"source":"local"},"results":` + largeJSONArray(t, 100, 1100) + `}`
	var page struct {
		Meta struct {
			Source string `json:"source"`
		} `json:"meta"`
		Results mirrorEnvelope `json:"results"`
	}
	out := paginateMirroredOutput(payload, 2, 24000)
	if err := json.Unmarshal([]byte(out), &page); err != nil {
		t.Fatal(err)
	}
	var first struct {
		ID int `json:"id"`
	}
	if len(page.Results.Results) > 0 {
		_ = json.Unmarshal(page.Results.Results[0], &first)
	}
	if len(out) > 24000 || page.Meta.Source != "local" || first.ID != 3 || page.Results.NextOffset == nil || *page.Results.NextOffset != 2+page.Results.ReturnedCount {
		t.Fatalf("wrapped array not resumed: %.250s", out)
	}
}
