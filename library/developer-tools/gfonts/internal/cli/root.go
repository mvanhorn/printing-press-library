package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version string

// NewRootCommand constructs the complete gfonts command tree.
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "gfonts-pp-cli",
		Short:         "Search, browse, and download Google Fonts",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			_ = cmd.Help()
		},
		Version: version,
	}
	rootCmd.SetVersionTemplate("gfonts {{.Version}}\n")
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		if cmd == rootCmd {
			printUsage()
			return
		}
		printCommandHelp(cmd.Name())
	})
	rootCmd.AddCommand(newSearchCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newInfoCmd())
	rootCmd.AddCommand(newDownloadCmd())
	rootCmd.AddCommand(newTrendingCmd())
	rootCmd.AddCommand(newCategoriesCmd())
	rootCmd.AddCommand(newRandomCmd())
	rootCmd.AddCommand(newAgentContextCmd())
	rootCmd.AddCommand(newVersionCmd())
	return rootCmd
}

// Execute runs the CLI and returns a process exit code.
func Execute(binaryVersion string) int {
	version = binaryVersion
	if err := NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the gfonts version",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("gfonts %s\n", version)
		},
	}
}
