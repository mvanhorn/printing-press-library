package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/goose/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/commerce/goose/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/goose/internal/config"

	"github.com/spf13/cobra"
)

// newAuthLoginCmd implements `goose-pp-cli auth login` — the Cognito
// bootstrap. The user pastes their refresh token (or passes it via
// --refresh-token); we immediately mint an access token to validate, then
// persist both plus the auto-detected facility list to the config.
//
// Why pasting rather than driving Chrome from inside the CLI: every reliable
// path (CDP, Chrome's LevelDB file, AppleScript) brings runtime dependencies
// or platform-specific code that breaks the cross-platform contract. The paste
// path is one-line copy-paste from DevTools and works on any OS.
func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var refreshToken string
	var chromeFlag bool
	var facilityOverride string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Sign in by importing your goose.pet Cognito session",
		Long: strings.TrimSpace(`
Sign in by importing your goose.pet Cognito refresh token.

To find your refresh token:
  1. Open https://app.goose.pet in Chrome (or any browser) and sign in.
  2. Open DevTools → Application → Local Storage → https://app.goose.pet.
  3. Copy the value of the key ending in '.refreshToken'.
     (Full key: CognitoIdentityServiceProvider.4qv4b8pvtsqigsontd3vfmf6kf.<your-email>.refreshToken)
  4. Run 'goose-pp-cli auth login' and paste the value when prompted, or pass it
     with '--refresh-token <value>'.

After this one-time setup the CLI auto-refreshes access tokens behind the scenes.
`),
		Example: "  goose-pp-cli auth login\n  goose-pp-cli auth login --refresh-token <token>",
		Annotations: map[string]string{
			"mcp:hidden": "true", // requires human-in-the-loop paste
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would prompt for refresh token and call Cognito InitiateAuth (verify mode short-circuit)")
				return nil
			}

			if refreshToken == "" {
				refreshToken = strings.TrimSpace(os.Getenv("GOOSE_REFRESH_TOKEN"))
			}
			if refreshToken == "" {
				if chromeFlag {
					fmt.Fprintln(cmd.OutOrStdout(), "Open Chrome → DevTools → Application → Local Storage → https://app.goose.pet.")
					fmt.Fprintln(cmd.OutOrStdout(), "Find the key ending in '.refreshToken' and copy its value.")
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "See `goose-pp-cli auth login --help` for where to find your refresh token.")
				}
				fmt.Fprint(cmd.OutOrStdout(), "Paste refresh token: ")
				reader := bufio.NewReader(os.Stdin)
				line, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("reading refresh token from stdin: %w", err)
				}
				refreshToken = strings.TrimSpace(line)
			}
			if refreshToken == "" {
				return fmt.Errorf("no refresh token provided")
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Validating refresh token against Cognito…")
			res, err := auth.Refresh(refreshToken)
			if err != nil {
				return err
			}

			// Detect facilities from the access token's groups claim.
			facilities, _ := auth.ExtractFacilities(res.AccessToken)
			facility := facilityOverride
			if facility == "" && len(facilities) > 0 {
				facility = facilities[0]
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			cfg.RefreshToken = refreshToken
			cfg.AccessToken = res.AccessToken
			cfg.TokenExpiry = res.ExpiresAt
			cfg.ClientID = auth.CognitoClientID
			if cfg.TemplateVars == nil {
				cfg.TemplateVars = map[string]string{}
			}
			if facility != "" {
				cfg.TemplateVars["facility"] = facility
			}
			// Refresh the AuthHeaderVal so subsequent commands work without env vars.
			cfg.AuthHeaderVal = "Bearer " + res.AccessToken
			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), green("Signed in to Goose."))
			fmt.Fprintf(cmd.OutOrStdout(), "  Config:    %s\n", cfg.Path)
			fmt.Fprintf(cmd.OutOrStdout(), "  Facility:  %s\n", facility)
			if len(facilities) > 1 {
				fmt.Fprintf(cmd.OutOrStdout(), "  Available: %s (override with --facility or GOOSE_FACILITY)\n", strings.Join(facilities, ", "))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  Expires:   %s (auto-refreshes)\n", res.ExpiresAt.Format(time.RFC3339))
			return nil
		},
	}

	cmd.Flags().StringVar(&refreshToken, "refresh-token", "", "Refresh token value (skip the paste prompt)")
	cmd.Flags().BoolVar(&chromeFlag, "chrome", false, "Show Chrome-specific instructions for finding the refresh token")
	cmd.Flags().StringVar(&facilityOverride, "facility", "", "Override the default facility slug (e.g., your-facility)")

	return cmd
}
