// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/config"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"runtime"
)

func newAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication for Gorgias",
		Long: "Manage the credentials used for Gorgias API calls.\n\n" +
			"Gorgias uses HTTP Basic auth: your account email is the username and the\n" +
			"API key is the password. The CLI accepts credentials from (in priority order):\n\n" +
			"  1. GORGIAS_USERNAME + GORGIAS_API_KEY env vars\n" +
			"  2. $XDG_CONFIG_HOME/gorgias-pp-cli/config.toml (default ~/.config/...)\n\n" +
			"Sub-commands:\n" +
			"  * setup       — print steps for minting a new API key in the Gorgias UI\n" +
			"  * status      — report whether credentials are configured (no API call)\n" +
			"  * set-token   — save email + API key to the config file (non-interactive)\n" +
			"  * logout      — clear stored credentials\n\n" +
			"For an end-to-end auth probe (calls /account), use `gorgias-pp-cli doctor`.",
	}

	cmd.AddCommand(newAuthSetupCmd(flags))
	cmd.AddCommand(newAuthStatusCmd(flags))
	cmd.AddCommand(newAuthSetTokenCmd(flags))
	cmd.AddCommand(newAuthLogoutCmd(flags))

	return cmd
}

// newAuthSetupCmd prints concrete steps for getting a credential. Side-effect
// rule: print by default, --launch opt-in to open the URL, short-circuit when
// the verifier is running this in a sandboxed subprocess.
func newAuthSetupCmd(_ *rootFlags) *cobra.Command {
	var launch bool
	cmd := &cobra.Command{
		Use:     "setup",
		Short:   "Print steps for obtaining a credential (use --launch to open the URL)",
		Example: "  gorgias-pp-cli auth setup\n  gorgias-pp-cli auth setup --launch",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			fmt.Fprintln(w, "Get an API key at: https://docs.gorgias.com/en-US/rest-api-208286")
			fmt.Fprintln(w, "  Settings → REST API → Create API key.")
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "Gorgias uses HTTP Basic auth — your account email is the username, the")
			fmt.Fprintln(w, "API key is the password. Set both:")
			fmt.Fprintln(w, "  export GORGIAS_USERNAME=\"account-email-placeholder\"")
			fmt.Fprintln(w, "  export GORGIAS_API_KEY=\"<your-api-key>\"")
			fmt.Fprintln(w, "  export GORGIAS_BASE_URL=\"https://<tenant>.gorgias.com/api\"")
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "Or save persistently:")
			fmt.Fprintln(w, "  gorgias-pp-cli auth set-token <email> <api-key>")
			if !launch {
				return nil
			}
			launchURL := "https://docs.gorgias.com/en-US/rest-api-208286"
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(w, "would launch: %s\n", launchURL)
				return nil
			}
			if err := openSetupURL(launchURL); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "could not open browser automatically: %v\nopen this URL manually: %s\n", err, launchURL)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&launch, "launch", false, "Open the setup URL in your default browser")
	return cmd
}

// openSetupURL opens url in the OS default browser. Per the side-effect rule,
// the caller short-circuits with cliutil.IsVerifyEnv() before this is reached.
func openSetupURL(url string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", url)
	case "linux":
		c = exec.Command("xdg-open", url)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	return c.Start()
}

func newAuthStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Short:   "Show authentication status",
		Example: "  gorgias-pp-cli auth status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			w := cmd.OutOrStdout()
			header := cfg.AuthHeader()
			authed := header != ""
			// JSON envelope: {authenticated, verified, source, config}. When not
			// authenticated, write the envelope first then return authErr
			// so exit code carries the auth-failure signal.
			if flags.asJSON {
				out := map[string]any{
					"authenticated": authed,
					"verified":      false,
					"source":        cfg.AuthSource,
					"config":        cfg.Path,
				}
				if !authed {
					// Embed the error inside the same JSON document the user
					// is reading — silenceEmission keeps the exit code but
					// stops writeExecuteError from emitting a second envelope.
					out["error"] = "no credentials configured"
					out["hint"] = "export GORGIAS_USERNAME=<email> GORGIAS_API_KEY=<key>, or run: gorgias-pp-cli auth set-token <email> <api-key>"
				}
				if printErr := printJSONFiltered(w, out, flags); printErr != nil {
					return printErr
				}
				if !authed {
					return silenceEmission(authErr(fmt.Errorf("no credentials configured")))
				}
				return nil
			}
			if !authed {
				fmt.Fprintln(w, red("Not authenticated"))
				fmt.Fprintln(w, "")
				fmt.Fprintln(w, "Set your credentials:")
				fmt.Fprintln(w, "  export GORGIAS_USERNAME=\"account-email-placeholder\"")
				fmt.Fprintln(w, "  export GORGIAS_API_KEY=\"<your-api-key>\"")
				fmt.Fprintf(w, "  gorgias-pp-cli auth set-token <email> <api-key>\n")
				return authErr(fmt.Errorf("no credentials configured"))
			}

			fmt.Fprintln(w, green("Credentials present (not verified)"))
			fmt.Fprintf(w, "  Source: %s\n", cfg.AuthSource)
			fmt.Fprintf(w, "  Config: %s\n", cfg.Path)
			return nil
		},
	}
}

func newAuthSetTokenCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "set-token <email> <api-key>",
		Short:   "Save Gorgias Basic-auth credentials (email + API key) to the config file",
		Example: "  gorgias-pp-cli auth set-token account-email-placeholder gor_xxxxxxxxxxxx",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			// Clear any legacy auth_header value so AuthHeader() falls through
			// to the email/api-key pair we're about to save. Without this, a
			// pre-existing auth_header from an older config silently shadows
			// the newly-saved credentials.
			cfg.AuthHeaderVal = ""
			if err := cfg.SaveCredentials(args[0], args[1]); err != nil {
				return configErr(fmt.Errorf("saving credentials: %w", err))
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"saved":       true,
					"config_path": cfg.Path,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Credentials saved to %s\n", cfg.Path)
			return nil
		},
	}
}

func newAuthLogoutCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "logout",
		Short:   "Clear stored credentials",
		Example: "  gorgias-pp-cli auth logout",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			if err := cfg.ClearTokens(); err != nil {
				return configErr(fmt.Errorf("clearing tokens: %w", err))
			}

			// Identify which (if any) auth env var is still exported so the
			// JSON envelope and the human prose can both surface it.
			envStillSet := ""
			if envStillSet == "" && os.Getenv("GORGIAS_USERNAME") != "" {
				envStillSet = "GORGIAS_USERNAME"
			}
			if envStillSet == "" && os.Getenv("GORGIAS_API_KEY") != "" {
				envStillSet = "GORGIAS_API_KEY"
			}

			// JSON envelope: {cleared: true, note?: "<env_var> env var is still set"}.
			if flags.asJSON {
				out := map[string]any{"cleared": true}
				if envStillSet != "" {
					out["note"] = envStillSet + " env var is still set"
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			if envStillSet != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Config cleared. Note: %s env var is still set.\n", envStillSet)
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Logged out. Credentials cleared.")
			return nil
		},
	}
}
