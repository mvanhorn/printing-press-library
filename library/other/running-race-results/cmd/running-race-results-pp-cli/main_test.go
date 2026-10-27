// cmd/running-race-results-pp-cli/main_test.go
package main

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/provider"
)

func TestVersionString(t *testing.T) {
	root := cli.NewRoot(provider.NewRegistry())
	if root.Version == "" {
		t.Fatal("version must be set")
	}
}
