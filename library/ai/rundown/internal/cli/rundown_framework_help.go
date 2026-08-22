// Copyright 2026 Abdelrahman Shaaban and contributors. Licensed under Apache-2.0. See LICENSE.

// Help-text repairs for generated framework commands.
//
// The generated `feedback` command ships without an Examples block, which its
// own help-quality probe reports as a failure. Editing internal/cli/feedback.go
// directly would be undone by `generate --force`, so the examples are attached
// through the novel-command hook instead, which regeneration preserves.

package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, _ *rootFlags) {
		cmd, _, err := root.Find([]string{"feedback"})
		if err != nil || cmd == nil || cmd.Name() != "feedback" {
			return
		}
		if strings.TrimSpace(cmd.Example) != "" {
			// A future generator version supplies its own examples; defer.
			return
		}
		cmd.Example = strings.Trim(`
  rundown-pp-cli feedback "top --since 7d is the command I reach for most"
  rundown-pp-cli feedback "stack should accept a display name, not just a slug"
  rundown-pp-cli feedback --stdin
  rundown-pp-cli feedback list
`, "\n")
	})
}
