package cli

import "github.com/spf13/cobra"

func newAuthstatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Short:   "Report profilepress auth posture",
		Example: "profilepress auth status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printJSON(map[string]any{"auth": "user-controlled browser/session", "collects_credentials": false, "live_writes_enabled": false})
		},
	}
}
