package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlayersSearchCSV(t *testing.T) {
	csvPath := writeSampleCSV(t)
	var out bytes.Buffer
	err := run([]string{"players", "search", "negreanu", "--file", csvPath, "--json", "--compact"}, &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Daniel Negreanu") {
		t.Fatalf("expected player in output, got %s", out.String())
	}
}

func TestImportAndDoctor(t *testing.T) {
	csvPath := writeSampleCSV(t)
	dir := t.TempDir()
	outPath := filepath.Join(dir, "pokerdb.local.json")
	var out bytes.Buffer
	err := run([]string{"import", csvPath, "--out", outPath}, &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	err = run([]string{"doctor", "--file", outPath}, &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "POKERDB_API_KEY") || !strings.Contains(out.String(), `"api": "none"`) {
		t.Fatalf("doctor should be local-only/no-api, got %s", out.String())
	}
}

func TestImportRequiresOut(t *testing.T) {
	csvPath := writeSampleCSV(t)
	err := run([]string{"import", csvPath}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "import requires --out") {
		t.Fatalf("expected --out usage error, got %v", err)
	}
}

func TestImportKeepsExistingOutOnLoadError(t *testing.T) {
	dir := t.TempDir()
	badJSON := filepath.Join(dir, "bad.json")
	outPath := filepath.Join(dir, "pokerdb.local.json")
	if err := os.WriteFile(badJSON, []byte(`{"rows": [`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outPath, []byte("keep-me"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := run([]string{"import", badJSON, "--out", outPath}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error")
	}
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep-me" {
		t.Fatalf("existing output was changed: %q", got)
	}
}

func writeSampleCSV(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sample-results.csv")
	data := "player,country,earnings,rank,event,date,place,venue,city,source_url\n" +
		"Daniel Negreanu,Canada,\"$54,000,000\",1,WSOP Paradise Super Main Event,2025-12-18,12,Atlantis Paradise Island,Nassau,local-export://players/daniel-negreanu\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
