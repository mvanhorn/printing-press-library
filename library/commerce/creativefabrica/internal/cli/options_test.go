package cli

import (
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestNovelCommandsRejectInvalidLimitsBeforeNetwork(t *testing.T) {
	cases := []struct {
		name string
		cmd  func(*rootFlags) *cobra.Command
		args []string
		want string
	}{
		{name: "deals zero limit", cmd: newNovelDealsCmd, args: []string{"--limit", "0"}, want: "--limit"},
		{name: "deals negative pages", cmd: newNovelDealsCmd, args: []string{"--max-scan-pages", "-1"}, want: "--max-scan-pages"},
		{name: "tags negative sample", cmd: newNovelTagsCmd, args: []string{"query", "--sample", "-1"}, want: "--sample"},
		{name: "new-since below minimum", cmd: newNovelNewSinceCmd, args: []string{"query", "--limit", "1"}, want: "--limit must be between 20 and 100"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			flags := &rootFlags{dryRun: true}
			cmd := tc.cmd(flags)
			cmd.SetArgs(tc.args)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want validation mentioning %s", err, tc.want)
			}
		})
	}
}
