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
