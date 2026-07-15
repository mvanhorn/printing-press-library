package cli

// pp:data-source local

import (
	"strings"

	"github.com/spf13/cobra"
)

func newNovelJobRepeatsCmd(flags *rootFlags) *cobra.Command {
	var name string
	cmd := &cobra.Command{Use: "repeats [job-name]", Short: "Compare persisted executions of the same printable job", Example: "  bambu-pp-cli job repeats --dry-run", Annotations: map[string]string{"mcp:local-write": "true", "pp:no-error-path-probe": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if name == "" && len(args) > 0 {
			name = strings.Join(args, " ")
		}
		return runBambuRepeats(cmd, flags, name)
	}}
	cmd.Flags().StringVar(&name, "name", "", "Job-name substring; defaults to the latest persisted job")
	return cmd
}
