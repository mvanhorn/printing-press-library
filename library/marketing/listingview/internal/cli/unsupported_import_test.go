// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestRootDoesNotExposeUnsupportedImport(t *testing.T) {
	for _, cmd := range RootCmd().Commands() {
		if cmd.Name() == "import" {
			t.Fatal("root command exposes unsupported generic import route")
		}
	}
}
