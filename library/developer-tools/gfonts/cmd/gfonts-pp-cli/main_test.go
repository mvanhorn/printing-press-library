package main

import (
	"encoding/json"
	"io"
	"os"
	"testing"
)

func TestSubcommandVersionFlagKeepsOriginalBehavior(t *testing.T) {
	previousArgs, previousStdout := os.Args, os.Stdout
	t.Cleanup(func() {
		os.Args, os.Stdout = previousArgs, previousStdout
	})
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	os.Args = []string{"gfonts-pp-cli", "search", "--version"}
	os.Stdout = writer
	main()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if want := "gfonts " + cliVersion() + "\n"; string(got) != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
}

func TestSourceBuildUsesReleaseLedgerVersion(t *testing.T) {
	previousVersion := version
	version = ""
	t.Cleanup(func() { version = previousVersion })
	data, err := os.ReadFile("../../.printing-press-release.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if got := cliVersion(); got == "0.0.0-dev" || got != manifest.Version {
		t.Fatalf("source-build version = %q, want catalog release %q", got, manifest.Version)
	}
}
