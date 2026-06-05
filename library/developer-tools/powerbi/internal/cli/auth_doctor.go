// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/powerbi/internal/config"

	"github.com/spf13/cobra"
)

type jwtClaims struct {
	Aud   string   `json:"aud"`
	Iss   string   `json:"iss"`
	AppID string   `json:"appid"`
	TID   string   `json:"tid"`
	OID   string   `json:"oid"`
	UPN   string   `json:"upn"`
	Email string   `json:"email"`
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
	Scp   string   `json:"scp"`
	Exp   int64    `json:"exp"`
	Iat   int64    `json:"iat"`
	IDtyp string   `json:"idtyp"`
}

func newAuthDoctorCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose Power BI auth: decode token, probe API, explain failures",
		Long: `Decode the bearer token currently in use, surface its tenant / appId /
scopes / expiry, and probe /groups to confirm it actually works against Power BI.

Power BI has five common auth failure modes. When the probe fails, this command
explains which one is biting:

  1. Token expired             — re-run 'auth login' or refresh POWERBI_TOKEN
  2. Wrong scope               — token was minted for Graph, not Power BI
  3. Tenant setting disabled   — 'Allow service principals to use Power BI APIs' is off
  4. SP not added to workspace — Power BI workspace-level access required
  5. RLS blocking the SP       — service principals can't query RLS datasets

Run this first when you get unexpected 401/403 from any other command.`,
		Example: "  powerbi-pp-cli auth doctor\n  powerbi-pp-cli auth doctor --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			w := cmd.OutOrStdout()

			report := map[string]any{
				"authenticated": false,
				"source":        cfg.AuthSource,
				"config_path":   cfg.Path,
			}

			// Find the raw token wherever it lives.
			rawToken := ""
			header := cfg.AuthHeader()
			if strings.HasPrefix(header, "Bearer ") {
				rawToken = strings.TrimPrefix(header, "Bearer ")
				report["authenticated"] = true
			}

			if rawToken == "" {
				report["diagnosis"] = "no_credential"
				report["fix"] = "Run 'powerbi-pp-cli auth login' or set POWERBI_TOKEN."
				if flags.asJSON {
					return printJSONFiltered(w, report, flags)
				}
				fmt.Fprintln(w, red("[no auth]")+" No bearer token found.")
				fmt.Fprintln(w, "Fix: run 'powerbi-pp-cli auth login' (device-code flow) or 'export POWERBI_TOKEN=...'")
				return authErr(fmt.Errorf("no credential"))
			}

			// Decode the JWT payload (middle segment, base64url, no padding).
			parts := strings.Split(rawToken, ".")
			if len(parts) != 3 {
				report["diagnosis"] = "malformed_token"
				report["fix"] = "Token does not look like a JWT (expected 3 dot-separated segments). Re-run 'auth login'."
				if flags.asJSON {
					return printJSONFiltered(w, report, flags)
				}
				fmt.Fprintln(w, red("[malformed]")+" Bearer token is not a JWT.")
				return authErr(fmt.Errorf("malformed token"))
			}
			payload, err := base64.RawURLEncoding.DecodeString(parts[1])
			if err != nil {
				// Some AAD tokens use standard padding; retry with that.
				payload, err = base64.StdEncoding.DecodeString(parts[1])
				if err != nil {
					report["diagnosis"] = "undecodable_jwt"
					if flags.asJSON {
						return printJSONFiltered(w, report, flags)
					}
					fmt.Fprintln(w, red("[undecodable]")+" Could not base64-decode the JWT payload.")
					return authErr(fmt.Errorf("undecodable jwt"))
				}
			}
			var claims jwtClaims
			if err := json.Unmarshal(payload, &claims); err != nil {
				report["diagnosis"] = "unparseable_claims"
				if flags.asJSON {
					return printJSONFiltered(w, report, flags)
				}
				fmt.Fprintln(w, red("[unparseable]")+" JWT payload is not valid JSON.")
				return authErr(fmt.Errorf("unparseable claims"))
			}

			expiresAt := time.Unix(claims.Exp, 0)
			ttl := time.Until(expiresAt)
			expired := ttl < 0

			report["audience"] = claims.Aud
			report["issuer"] = claims.Iss
			report["tenant"] = claims.TID
			report["app_id"] = claims.AppID
			report["upn"] = claims.UPN
			report["scopes"] = claims.Scp
			report["roles"] = claims.Roles
			report["expires_at"] = expiresAt.Format(time.RFC3339)
			report["expires_in_seconds"] = int(ttl.Seconds())
			if claims.IDtyp == "app" || (claims.AppID != "" && claims.UPN == "") {
				report["identity_type"] = "service_principal"
			} else {
				report["identity_type"] = "user"
			}

			// Check audience matches Power BI.
			audOK := strings.Contains(claims.Aud, "analysis.windows.net/powerbi") ||
				strings.Contains(claims.Aud, "00000009-0000-0000-c000-000000000000") // PBI service principal
			if !audOK {
				report["diagnosis"] = "wrong_audience"
				report["fix"] = fmt.Sprintf("Token audience is %q. Power BI tokens must have aud=https://analysis.windows.net/powerbi/api. Re-mint with that scope.", claims.Aud)
			} else if expired {
				report["diagnosis"] = "expired"
				report["fix"] = "Run 'powerbi-pp-cli auth login' to refresh."
			}

			// Probe /groups to confirm the token actually works.
			if report["diagnosis"] == nil {
				c, err := flags.newClient()
				if err == nil {
					code, _ := c.ProbeGet("/groups")
					report["probe_status"] = code
					switch {
					case code >= 200 && code < 300:
						report["diagnosis"] = "ok"
					case code == 401:
						report["diagnosis"] = "unauthorized"
						report["fix"] = "Token rejected. Re-run 'auth login' — token may be revoked, or scope did not include Power BI."
					case code == 403:
						if report["identity_type"] == "service_principal" {
							report["diagnosis"] = "sp_forbidden"
							report["fix"] = "Service principals are blocked by default. Have an admin enable the tenant setting 'Allow service principals to use Power BI APIs' AND add the SP to each workspace as a member."
						} else {
							report["diagnosis"] = "forbidden"
							report["fix"] = "User does not have access to any workspaces, OR the 'Allow service principals' check is required for this token type. Try logging in as a user with workspace access."
						}
					case code == 429:
						report["diagnosis"] = "throttled"
						report["fix"] = "120 queries/min/user cap hit. Back off and retry."
					default:
						report["diagnosis"] = "probe_failed"
						report["fix"] = fmt.Sprintf("Unexpected probe status %d. Check network and tenant URL.", code)
					}
				}
			}

			if flags.asJSON {
				return printJSONFiltered(w, report, flags)
			}

			// Human-friendly output.
			fmt.Fprintln(w, bold("Power BI auth doctor"))
			fmt.Fprintf(w, "  source:        %s\n", cfg.AuthSource)
			fmt.Fprintf(w, "  identity:      %s", report["identity_type"])
			if claims.UPN != "" {
				fmt.Fprintf(w, " (%s)", claims.UPN)
			} else if claims.AppID != "" {
				fmt.Fprintf(w, " (app %s)", claims.AppID)
			}
			fmt.Fprintln(w)
			fmt.Fprintf(w, "  tenant:        %s\n", claims.TID)
			fmt.Fprintf(w, "  audience:      %s\n", claims.Aud)
			fmt.Fprintf(w, "  scopes:        %s\n", claims.Scp)
			if expired {
				fmt.Fprintf(w, "  expires_at:    %s %s\n", expiresAt.Format(time.RFC3339), red("(EXPIRED)"))
			} else {
				fmt.Fprintf(w, "  expires_at:    %s (in %s)\n", expiresAt.Format(time.RFC3339), ttl.Round(time.Second))
			}
			if v, ok := report["probe_status"].(int); ok {
				fmt.Fprintf(w, "  probe GET /groups: HTTP %d\n", v)
			}
			diag, _ := report["diagnosis"].(string)
			fix, _ := report["fix"].(string)
			switch diag {
			case "ok":
				fmt.Fprintln(w, green("[ok]")+" Token is valid and Power BI is reachable.")
			default:
				fmt.Fprintf(w, "%s %s\n", yellow("[issue]"), diag)
				if fix != "" {
					fmt.Fprintf(w, "  fix: %s\n", fix)
				}
				return authErr(fmt.Errorf("auth doctor: %s", diag))
			}
			return nil
		},
	}
}
