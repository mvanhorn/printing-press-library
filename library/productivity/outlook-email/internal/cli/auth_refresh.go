// Hand-built (Phase 3): explicit refresh-token rotation. The client also
// auto-refreshes on 401, so this command exists mainly for diagnostics and
// scripting.

package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/outlook-email/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/outlook-email/internal/oauth"

	"github.com/spf13/cobra"
)

func newAuthRefreshCmd(flags *rootFlags) *cobra.Command {
	var clientID string
	var authority string
	var scopes string

	cmd := &cobra.Command{
		Use:                   "refresh",
		Short:                 "Force-refresh the access token using the stored refresh token",
		Example:               "  outlook-email-pp-cli auth refresh --json",
		Annotations:           map[string]string{"mcp:read-only": "true"},
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			if cfg.RefreshToken == "" {
				return authErr(fmt.Errorf("no refresh token stored. Run 'auth login --device-code' first"))
			}

			if clientID == "" {
				clientID = os.Getenv("OUTLOOK_EMAIL_CLIENT_ID")
				if clientID == "" {
					clientID = cfg.ClientID
				}
			}
			scopeList := oauth.DefaultScopes
			if s := strings.TrimSpace(scopes); s != "" {
				scopeList = strings.Fields(strings.ReplaceAll(s, ",", " "))
			}

			oc := oauth.NewClient(authority, clientID, scopeList)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			tr, err := oc.RefreshToken(ctx, cfg.RefreshToken)
			if err != nil {
				return authErr(err)
			}

			if err := cfg.SaveTokens(oc.ClientID, "", tr.AccessToken, tr.RefreshToken, tr.ExpiryTime); err != nil {
				return configErr(err)
			}

			w := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(w, map[string]any{
					"refreshed":  true,
					"expires_at": tr.ExpiryTime.UTC().Format(time.RFC3339),
					"scopes":     strings.Fields(tr.Scope),
				}, flags)
			}
			fmt.Fprintln(w, green("Token refreshed."))
			fmt.Fprintf(w, "  Expires: %s\n", tr.ExpiryTime.Format(time.RFC1123))
			return nil
		},
	}

	cmd.Flags().StringVar(&clientID, "client-id", "", "Azure AD app client id (defaults to the saved login)")
	cmd.Flags().StringVar(&authority, "authority", oauth.DefaultAuthority, "OAuth authority URL")
	cmd.Flags().StringVar(&scopes, "scopes", "", "Space- or comma-separated OAuth scopes")

	return cmd
}
