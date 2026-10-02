package main

import (
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
	if want := "gfonts " + version + "\n"; string(got) != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
}
