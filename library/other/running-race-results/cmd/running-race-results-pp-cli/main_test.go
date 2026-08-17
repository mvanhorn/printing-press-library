// cmd/running-race-results-pp-cli/main_test.go
package main

import "testing"

func TestVersionString(t *testing.T) {
	if version == "" {
		t.Fatal("version must be set")
	}
}
