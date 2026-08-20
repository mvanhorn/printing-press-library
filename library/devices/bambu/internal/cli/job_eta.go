package cli

// pp:data-source auto
// pp:client-call

import (
	"github.com/spf13/cobra"
	"strings"
)

func newNovelJobEtaCmd(flags *rootFlags) *cobra.Command {
	var host string
	cmd := &cobra.Command{Use: "eta", Short: "Correct the printer finish estimate using prior runs of the same job", Example: strings.Trim(`
  bambu-pp-cli job eta --agent`, "\n"), Annotations: map[string]string{"mcp:local-write": "true"}, RunE: func(cmd *cobra.Command, _ []string) error { return runBambuETA(cmd, flags, host) }}
	addHostFlag(cmd, &host)
	return cmd
}
