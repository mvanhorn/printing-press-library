package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestLayersSearchRejectsMisleadingLiveMode(t *testing.T) {
	cmd := newSearchCmd(&rootFlags{dataSource: "live"})
	cmd.SetArgs([]string{"semester"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "no unified live search endpoint") {
		t.Fatalf("Execute() error = %v, want explicit live-search rejection", err)
	}
}

func TestLayersSearchUsesPrivateLocalIndex(t *testing.T) {
	flags := &rootFlags{dataSource: "local"}
	cmd := newSearchCmd(flags)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"semester", "--db", filepath.Join(t.TempDir(), "data.db")})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("search output is not JSON: %v\n%s", err, stdout.String())
	}
	if source := result["meta"].(map[string]any)["source"]; source != "local" {
		t.Fatalf("meta.source = %v, want local", source)
	}
	if got := cmd.Annotations["pp:happy-args"]; got == "" {
		t.Fatal("search command must provide a safe Printing Press happy-path fixture")
	}
}
