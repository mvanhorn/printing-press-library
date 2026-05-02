package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/phgql"

	"github.com/spf13/cobra"
)

func newWhoamiCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Show the authenticated Product Hunt user (full data; reports remaining complexity-budget)",
		Long: strings.Trim(`
Calls Product Hunt's GraphQL viewer query and prints the authenticated user
(yourself) plus the current rate-limit budget. Returns null when the auth mode
is OAuth client_credentials (public scope has no user context) — in that case
the output explains how to switch to a developer token.
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli whoami
  producthunt-pp-cli whoami --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := phgql.New(cfg)
			var resp phgql.ViewerResponse
			rate, qerr := c.Query(cmd.Context(), phgql.ViewerQuery, nil, &resp)
			if qerr != nil {
				return qerr
			}
			out := whoamiOut{
				Authenticated:  resp.Viewer.User != nil,
				AuthMode:       authModeName(cfg),
				RateLimit:      rate,
				ResetEpochHint: rate.ResetSecs,
				User:           resp.Viewer.User,
			}
			if resp.Viewer.User == nil {
				out.Hint = "Viewer is null — you are using OAuth client_credentials (public scope). To unlock `whoami`, generate a developer token at https://www.producthunt.com/v2/oauth/applications and run `producthunt-pp-cli auth set-token <token>`."
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

type whoamiOut struct {
	Authenticated  bool              `json:"authenticated"`
	AuthMode       string            `json:"auth_mode"`
	User           *phgql.ViewerUser `json:"user,omitempty"`
	RateLimit      phgql.RateLimit   `json:"rate_limit"`
	ResetEpochHint int               `json:"reset_seconds_from_response"`
	Hint           string            `json:"hint,omitempty"`
}

func authModeName(cfg *config.Config) string {
	if cfg == nil {
		return "none"
	}
	if cfg.ProductHuntToken != "" {
		return "developer_token (env:PRODUCT_HUNT_TOKEN)"
	}
	if cfg.AccessToken != "" || cfg.AuthHeaderVal != "" {
		if cfg.ClientID != "" {
			return "oauth_client_credentials"
		}
		return "developer_token"
	}
	if cfg.ClientID != "" && cfg.ClientSecret != "" {
		return "oauth_client_credentials"
	}
	return "none"
}

// errMessageContains is a tiny helper used by error-shaping in command files.
func errMessageContains(err error, substrs ...string) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, s := range substrs {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// renderJSON is a one-liner used when callers want to dump a struct to a
// writer without going through printJSONFiltered (when --select etc. shouldn't
// apply). Currently unused in this file but kept for command files that need
// it.
func renderJSON(out interface{}) string {
	b, _ := json.MarshalIndent(out, "", "  ")
	return string(b)
}

// fmtError shapes a phgql.Error to a single string for error messages.
func fmtError(err error) string {
	if err == nil {
		return ""
	}
	if pe, ok := err.(*phgql.Error); ok {
		return fmt.Sprintf("[%s] %s", pe.Code, pe.Message)
	}
	return err.Error()
}

// Ensure context import + various unused imports we'll use across files compile.
var _ = context.Background
var _ = renderJSON
var _ = fmtError
var _ = errMessageContains
