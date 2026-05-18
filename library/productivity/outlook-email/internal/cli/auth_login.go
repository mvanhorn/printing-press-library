// Hand-built (Phase 3): OAuth 2.0 device-code login for personal Microsoft 365 accounts.

package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/outlook-email/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/outlook-email/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/outlook-email/internal/oauth"

	"github.com/spf13/cobra"
)

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var deviceCode bool
	var clientID string
	var authority string
	var scopes string
	var launch bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate against Microsoft Identity (device-code flow for personal accounts)",
		Long: `Authenticate against Microsoft Identity using the OAuth 2.0 device-code flow.
The flow targets the /common authority by default, so personal Microsoft 365
accounts (Outlook.com, Hotmail, Live, MSA) sign in alongside work/school
accounts. Refresh tokens are persisted so subsequent commands run
non-interactively.`,
		Example: strings.Trim(`
  # Default: device-code flow against /common with the Microsoft Graph
  # PowerShell client id (works with personal Microsoft accounts out of the box)
  outlook-email-pp-cli auth login --device-code

  # Bring your own Azure AD app registration (recommended for production)
  outlook-email-pp-cli auth login --device-code \
    --client-id 11111111-2222-3333-4444-555555555555

  # Override the authority (e.g. /consumers for personal-only)
  outlook-email-pp-cli auth login --device-code \
    --authority https://login.microsoftonline.com/consumers
`, "\n"),
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// auth login is a top-level interactive flow that prints a verification
			// URL and waits for the user. Side-effect convention: short-circuit
			// under verify so test runners don't hang.
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would start device-code flow against", oauth.DefaultAuthority)
				return nil
			}

			if !deviceCode {
				return usageErr(fmt.Errorf("device-code is the only supported flow for personal Microsoft accounts; pass --device-code"))
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			if clientID == "" {
				clientID = os.Getenv("OUTLOOK_EMAIL_CLIENT_ID")
			}
			scopeList := oauth.DefaultScopes
			if s := strings.TrimSpace(scopes); s != "" {
				scopeList = strings.Fields(strings.ReplaceAll(s, ",", " "))
			}

			oc := oauth.NewClient(authority, clientID, scopeList)

			ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Minute)
			defer cancel()

			device, err := oc.RequestDeviceCode(ctx)
			if err != nil {
				return apiErr(err)
			}

			w := cmd.OutOrStdout()
			if flags.asJSON {
				envelope := map[string]any{
					"verification_uri": device.VerificationURI,
					"user_code":        device.UserCode,
					"expires_in":       device.ExpiresIn,
					"interval":         device.Interval,
					"message":          device.Message,
					"client_id":        oc.ClientID,
					"authority":        oc.Authority,
					"scopes":           oc.Scopes,
				}
				if err := printJSONFiltered(w, envelope, flags); err != nil {
					return err
				}
			} else {
				fmt.Fprintln(w, device.Message)
				fmt.Fprintln(w, "")
				fmt.Fprintln(w, "    URL:  ", device.VerificationURI)
				fmt.Fprintln(w, "    Code: ", device.UserCode)
				fmt.Fprintln(w, "")
				fmt.Fprintln(w, "Waiting for authorization (this window will close itself once you complete sign-in)...")
			}

			if launch {
				openInBrowser(device.VerificationURI)
			}

			tr, err := oc.PollToken(ctx, device)
			if err != nil {
				return apiErr(err)
			}

			if err := cfg.SaveTokens(oc.ClientID, "", tr.AccessToken, tr.RefreshToken, tr.ExpiryTime); err != nil {
				return configErr(err)
			}

			if flags.asJSON {
				out := map[string]any{
					"authenticated": true,
					"client_id":     oc.ClientID,
					"authority":     oc.Authority,
					"scopes":        strings.Fields(tr.Scope),
					"expires_at":    tr.ExpiryTime.UTC().Format(time.RFC3339),
				}
				return printJSONFiltered(w, out, flags)
			}
			fmt.Fprintln(w, green("Authentication successful."))
			fmt.Fprintf(w, "  Access token expires: %s\n", tr.ExpiryTime.Format(time.RFC1123))
			fmt.Fprintf(w, "  Config: %s\n", cfg.Path)
			return nil
		},
	}

	cmd.Flags().BoolVar(&deviceCode, "device-code", true, "Use the OAuth 2.0 device-code flow (the only flow that works for personal Microsoft accounts)")
	cmd.Flags().StringVar(&clientID, "client-id", "", "Azure AD app registration client id (default: Microsoft Graph PowerShell client; supports personal accounts)")
	cmd.Flags().StringVar(&authority, "authority", oauth.DefaultAuthority, "OAuth authority URL (default https://login.microsoftonline.com/common — accepts personal and work/school)")
	cmd.Flags().StringVar(&scopes, "scopes", "", "Space- or comma-separated OAuth scopes (default: Mail.ReadWrite Mail.Send MailboxSettings.ReadWrite User.Read offline_access)")
	cmd.Flags().BoolVar(&launch, "launch", false, "Open the verification URL in the system browser (default: print URL and wait)")

	return cmd
}

func openInBrowser(rawURL string) {
	// Best-effort. We never fail the login on a missing browser.
	candidates := [][]string{
		{"open", rawURL},
		{"xdg-open", rawURL},
		{"cmd", "/c", "start", rawURL},
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c[0]); err == nil {
			_ = exec.Command(c[0], c[1:]...).Start()
			return
		}
	}
}
