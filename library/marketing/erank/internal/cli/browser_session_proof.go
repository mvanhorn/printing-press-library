package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/marketing/erank/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/erank/internal/config"

	"github.com/spf13/cobra"
)

func newBrowserSessionProofCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "browser-session-proof",
		Short:   "Verify the saved eRank browser-session proof",
		Example: "  erank-pp-cli browser-session-proof --json",
		Annotations: map[string]string{
			"mcp:read-only":    "true",
			"pp:requires-tier": "browser-session-proof",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"browser_session_proof": "dry-run",
				}, flags)
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"browser_session_proof": "valid",
					"detail":                "mock verifier browser-session proof",
				}, flags)
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			ok, detail := browserSessionProofStatusForAuth(cfg, cfg.AuthHeader())
			if !ok && cliutil.IsDogfoodEnv() {
				if err := validateAndWriteBrowserSessionProof(cmd.Context(), cfg, flags); err == nil {
					ok, detail = browserSessionProofStatusForAuth(cfg, cfg.AuthHeader())
				}
			}
			result := map[string]any{
				"browser_session_proof": "valid",
				"detail":                detail,
			}
			if !ok {
				result["browser_session_proof"] = "missing-or-invalid"
				_ = printJSONFiltered(cmd.OutOrStdout(), result, flags)
				return authErr(fmt.Errorf("%s", detail))
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
}
