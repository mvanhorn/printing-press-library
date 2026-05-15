package cli

import "github.com/spf13/cobra"

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func versionInfo() map[string]string {
	return map[string]string{"version": version, "commit": commit, "date": date}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{Use: "version", Short: "Print version information", RunE: func(cmd *cobra.Command, args []string) error {
		return emit(versionInfo())
	}}
}
