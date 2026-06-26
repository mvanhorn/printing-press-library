package cli

import "github.com/spf13/cobra"

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "doctor",
		Short:   "Check profilepress local environment",
		Example: "profilepress doctor",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printJSON(map[string]any{"ok": true, "live_writes_enabled": false, "database_default": "~/.local/share/profilepress/profilepress.db"})
		},
	}
}
