// pp:data-source local

package cli

import "github.com/spf13/cobra"

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, path := range [][]string{{"feedback", "list"}, {"profile", "list"}} {
			cmd, _, err := root.Find(path)
			if err != nil || cmd.Name() != path[1] || cmd.RunE == nil {
				continue
			}
			tbHonourDryRun(cmd, flags, path[0]+" "+path[1])
		}
	})
}

func tbHonourDryRun(cmd *cobra.Command, flags *rootFlags, action string) {
	run := cmd.RunE
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(c.OutOrStdout(), flags, action)
		}
		return run(c, args)
	}
}
