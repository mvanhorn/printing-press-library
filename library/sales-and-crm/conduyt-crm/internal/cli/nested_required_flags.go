package cli

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// PATCH(nested-required-flags-637): the Printing Press emits a property that is required INSIDE an optional nested body
// object (settings update --calling-window-*, conduyt-auth logout --screen-share-*, screen-share delete-session-code
// --marks-*, ...) as an always-required flag, so the bare command (which the API accepts: the object is optional) is
// refused locally. The generated check now fires only when a flag of that object's group was passed, i.e. when the
// object will be sent. A reprint must keep this (pinned by nested_required_flags_test.go).
func nestedFlagGroupChanged(cmd *cobra.Command, group string) bool {
	changed := false
	cmd.Flags().Visit(func(f *pflag.Flag) {
		if strings.HasPrefix(f.Name, group+"-") {
			changed = true
		}
	})
	return changed
}
