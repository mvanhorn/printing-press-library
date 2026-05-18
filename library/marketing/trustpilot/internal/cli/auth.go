package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/trustpilot/internal/cliutil"
	tpkg "github.com/mvanhorn/printing-press-library/library/marketing/trustpilot/internal/trustpilot"
)

func newAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Harvest and inspect the Trustpilot WAF cookie / Next.js build ids",
		Long: `Trustpilot is fronted by AWS WAF, so plain HTTP returns 403.
This command shells out to agent-browser (a Chrome wrapper) to load a Trustpilot
page once, harvest the aws-waf-token cookie + the Next.js build ids, and persist
them locally so subsequent commands can replay via plain HTTP for ~5-15 minutes.`,
	}
	cmd.AddCommand(newAuthLoginCmd(flags))
	cmd.AddCommand(newAuthStatusCmd(flags))
	cmd.AddCommand(newAuthLogoutCmd(flags))
	return cmd
}

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var useChrome bool
	cmd := &cobra.Command{
		Use:         "login",
		Short:       "Harvest a fresh aws-waf-token cookie via Chrome",
		Example:     `  trustpilot-pp-cli auth login`,
		Annotations: map[string]string{"mcp:hidden": "true"}, // interactive Chrome harvest
		RunE: func(cmd *cobra.Command, args []string) error {
			// PATCH: Chrome harvest is the default; --chrome remains a compatibility no-op.
			_ = useChrome
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would harvest Trustpilot aws-waf-token cookie via agent-browser")
				return nil
			}
			if flags.dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "would harvest Trustpilot aws-waf-token cookie via agent-browser")
				return nil
			}
			ctx := cmd.Context()
			db, err := openTPStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			fmt.Fprintln(os.Stderr, "Launching Chrome via agent-browser (about 10 seconds)...")
			s, err := tpkg.HarvestSession(ctx, tpkg.HarvestOptions{})
			if err != nil {
				return fmt.Errorf("harvest: %w", err)
			}
			if err := tpkg.SaveSession(ctx, db, s); err != nil {
				return err
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"awsWafToken":    redact(s.AWSWAFToken),
					"reviewsBuildId": s.ReviewsBuildID,
					"searchBuildId":  s.SearchBuildID,
					"harvestedAt":    s.HarvestedAt,
					// PATCH(greptile P1 PR#588): 15 * 60 evaluated as time.Duration is 900ns, not 15 minutes; use 15 * time.Minute.
					"freshUntil":     s.HarvestedAt.Add(15 * time.Minute),
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Harvested cookie ok (token=%s..., reviewsBuild=%s, searchBuild=%s)\n",
				safePrefix(s.AWSWAFToken, 12), s.ReviewsBuildID, s.SearchBuildID)
			return nil
		},
	}
	cmd.Flags().BoolVar(&useChrome, "chrome", false, "Launch a one-shot headless Chrome via agent-browser to harvest the cookie")
	return cmd
}

func newAuthStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "status",
		Short:       "Show the persisted Trustpilot session (without leaking the token)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			db, err := openTPStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			s, err := tpkg.LoadSession(ctx, db)
			if err != nil {
				return err
			}
			payload := map[string]any{
				"hasSession":     s.AWSWAFToken != "",
				"isFresh":        s.IsFresh(),
				"reviewsBuildId": s.ReviewsBuildID,
				"searchBuildId":  s.SearchBuildID,
				"harvestedAt":    s.HarvestedAt,
				"tokenPrefix":    safePrefix(s.AWSWAFToken, 8),
			}
			if flags.asJSON {
				return flags.printJSON(cmd, payload)
			}
			if s.AWSWAFToken == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "No Trustpilot session. Run: trustpilot-pp-cli auth login")
				return nil
			}
			fresh := "stale"
			if s.IsFresh() {
				fresh = "fresh"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Trustpilot session: %s (harvested %s)\nreviewsBuildId: %s\nsearchBuildId: %s\n",
				fresh, s.HarvestedAt.Format("2006-01-02 15:04:05Z07:00"), s.ReviewsBuildID, s.SearchBuildID)
			return nil
		},
	}
}

func newAuthLogoutCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "logout",
		Short:       "Clear the persisted Trustpilot session",
		Annotations: map[string]string{"mcp:read-only": "true"}, // mutates local cache only
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			db, err := openTPStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			if _, err := db.ExecContext(ctx, `DELETE FROM tp_session`); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Cleared Trustpilot session.")
			return nil
		},
	}
}

func redact(s string) string {
	if len(s) <= 12 {
		return "REDACTED"
	}
	return s[:8] + "...REDACTED"
}

func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
