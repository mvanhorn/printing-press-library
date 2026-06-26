package cli

import (
	"github.com/spf13/cobra"
	"profilepress-pp-cli/internal/linkedin"
)

func newPrivacyCheckCmd() *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use:     "privacy-check",
		Short:   "Verify whether LinkedIn profile-update broadcast status is safe before writes.",
		Example: "profilepress privacy-check --status disabled",
		RunE: func(cmd *cobra.Command, args []string) error {
			result := linkedin.EvaluatePrivacy(status)
			return printJSON(map[string]any{"privacy_status": result, "safe_to_apply": result == linkedin.PrivacyPassed})
		},
	}
	cmd.Flags().StringVar(&status, "status", "unknown", "observed LinkedIn broadcast setting: disabled, enabled, or unknown")
	return cmd
}
