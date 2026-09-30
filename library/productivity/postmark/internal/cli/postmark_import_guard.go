// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
)

// import POSTs one record per JSONL line. Data removals permanently erase a
// recipient's data, so they are refused here and requested one at a time with
// `data-removals create`, which an agent reaches only through a confirmed
// call. Email sends are already refused by the client transport.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		importCmd, _, err := root.Find([]string{"import"})
		if err != nil || importCmd == root || importCmd.Name() != "import" || importCmd.RunE == nil {
			return
		}
		inner := importCmd.RunE
		importCmd.RunE = func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && postmarkIsDataRemovalResource(args[0]) {
				return usageErr(errors.New("import does not create data removals: each one permanently erases a recipient's data, so request them one at a time with 'postmark-pp-cli data-removals create'"))
			}
			return inner(cmd, args)
		}
	})
}

func postmarkIsDataRemovalResource(resource string) bool {
	r := strings.ToLower(strings.TrimSpace(resource))
	r = strings.ReplaceAll(r, "-", "_")
	return r == "data_removals" || r == "data_removal"
}
