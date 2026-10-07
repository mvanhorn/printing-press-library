// Hand-authored test for PATCH(clickup-alias-examples-use-alias-path).

package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// Every alias Example must invoke the alias path (`docs <verb>` / `chat <verb>`),
// not the spec-derived `workspaces <group> <verb>` path, which has no such verb.
func TestAliasExamplesUseAliasPath(t *testing.T) {
	flags := &rootFlags{}
	for _, group := range []struct {
		name string
		cmd  *cobra.Command
	}{
		{name: "docs", cmd: newDocsAliasCmd(flags)},
		{name: "chat", cmd: newChatAliasCmd(flags)},
	} {
		for _, sub := range group.cmd.Commands() {
			if sub.Example == "" {
				continue
			}
			verb := strings.Fields(sub.Use)[0]
			want := "clickup-pp-cli " + group.name + " " + verb + " "
			for _, line := range strings.Split(strings.TrimSpace(sub.Example), "\n") {
				line = strings.TrimSpace(line)
				if !strings.HasPrefix(line+" ", want) {
					t.Errorf("%s %s: example %q does not start with %q", group.name, verb, line, want)
				}
			}
		}
	}
}
