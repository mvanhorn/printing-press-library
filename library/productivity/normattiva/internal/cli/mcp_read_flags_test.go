// Copyright 2026 Luigi Pastore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/mcp/cobratree"
)

func TestFileFlagIsMCPReadPath(t *testing.T) {
	root := RootCmd()
	for _, name := range []string{"verifica", "fascicolo"} {
		cmd, _, err := root.Find([]string{name})
		if err != nil || cmd == nil || cmd.Name() != name {
			t.Fatalf("find %s: cmd=%v err=%v", name, cmd, err)
		}
		if !cobratree.InputFlagBlocked(cmd, "file") {
			t.Fatalf("%s --file is not declared as an MCP read path", name)
		}
		if cobratree.InputFlagBlocked(cmd, "al") {
			t.Fatalf("%s --al was treated as a filesystem read", name)
		}
	}
}
