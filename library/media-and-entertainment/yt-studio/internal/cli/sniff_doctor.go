package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/internal/ytstudio"
)

func newSniffDoctorCmd(flags *rootFlags) *cobra.Command {
	var sessionPath string
	cmd := &cobra.Command{
		Use:         "sniff-doctor",
		Short:       "Verify the Studio Innertube session and response schema (drift detection)",
		Long:        "Loads the Studio session file and probes the analytics/get_screen endpoint. Returns typed exit 4 on auth failure, exit 5 on schema drift, exit 0 on healthy.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			session, err := ytstudio.Load(sessionPath)
			if err != nil {
				return authErr(fmt.Errorf("no Studio session loaded; run `yt-studio-pp-cli login --studio`: %w", err))
			}
			client := ytstudio.New(session)
			if err := client.CheckHealth(cmd.Context()); err != nil {
				var stE *ytstudio.Error
				if errors.As(err, &stE) {
					switch stE.Kind {
					case ytstudio.KindAuth:
						return authErr(fmt.Errorf("studio session rejected (HTTP %d); re-login: %w", stE.StatusCode, err))
					case ytstudio.KindRateLimit:
						return apiErr(fmt.Errorf("studio rate-limited (exit 7): %w", err))
					case ytstudio.KindSchemaDrift:
						return apiErr(fmt.Errorf("studio schema drift detected (exit 5): %w", err))
					}
				}
				return err
			}
			res := map[string]any{
				"healthy":        true,
				"session_path":   sessionPath,
				"client_name":    session.EffectiveClientName(),
				"client_version": session.EffectiveClientVersion(),
				"cookies":        len(session.Cookies),
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, res)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Studio session: healthy (%d cookies, clientName=%s, clientVersion=%s)\n",
				len(session.Cookies), session.EffectiveClientName(), session.EffectiveClientVersion())
			return nil
		},
	}
	cmd.Flags().StringVar(&sessionPath, "session", "", "Path to Studio session JSON (default: ~/.openclaw/state/yt-studio/studio-session.json)")
	return cmd
}
