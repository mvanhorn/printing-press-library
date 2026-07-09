// Copyright 2026 RyanGravetteIDLA and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature: audit user360.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type user360Security struct {
	Suspended     bool   `json:"suspended"`
	IsAdmin       bool   `json:"is_admin"`
	Enrolled2SV   bool   `json:"enrolled_2sv"`
	Enforced2SV   bool   `json:"enforced_2sv"`
	OrgUnitPath   string `json:"org_unit_path"`
	LastLoginTime string `json:"last_login_time"`
	CreationTime  string `json:"creation_time"`
}

type user360App struct {
	DisplayText string `json:"display_text"`
	ClientID    string `json:"client_id"`
	RiskTier    string `json:"risk_tier"`
	FullDrive   bool   `json:"full_drive_access"`
}

type user360Group struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type user360View struct {
	User     string          `json:"user"`
	Security user360Security `json:"security"`
	Apps     []user360App    `json:"apps"`
	Groups   []user360Group  `json:"groups"`
	Note     string          `json:"note,omitempty"`
}

func newNovelAuditUser360Cmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user360 <user>",
		Short: "One per-user posture report joining security, OAuth apps, and group memberships.",
		Long: "One per-user posture report joining the user's security posture (2SV, suspended, admin, OU),\n" +
			"authorized OAuth apps with risk tiers, and group memberships — a cross-API view no single\n" +
			"Google endpoint returns.",
		Example:     "  workspace-admin-pp-cli audit user360 user@yourdomain.com --agent --select security,apps",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would join user security, apps, and groups into one report")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a user email is required"))
			}
			user := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			view := user360View{User: user, Apps: []user360App{}, Groups: []user360Group{}}

			// 1. User security posture.
			udata, err := c.Get(ctx, wsDirectoryBase+"/users/"+user, map[string]string{
				"fields": "primaryEmail,suspended,isAdmin,isEnrolledIn2Sv,isEnforcedIn2Sv,orgUnitPath,lastLoginTime,creationTime",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var u struct {
				Suspended       bool   `json:"suspended"`
				IsAdmin         bool   `json:"isAdmin"`
				IsEnrolledIn2Sv bool   `json:"isEnrolledIn2Sv"`
				IsEnforcedIn2Sv bool   `json:"isEnforcedIn2Sv"`
				OrgUnitPath     string `json:"orgUnitPath"`
				LastLoginTime   string `json:"lastLoginTime"`
				CreationTime    string `json:"creationTime"`
			}
			if err := json.Unmarshal(udata, &u); err != nil {
				return fmt.Errorf("decoding user: %w", err)
			}
			view.Security = user360Security{
				Suspended:     u.Suspended,
				IsAdmin:       u.IsAdmin,
				Enrolled2SV:   u.IsEnrolledIn2Sv,
				Enforced2SV:   u.IsEnforcedIn2Sv,
				OrgUnitPath:   u.OrgUnitPath,
				LastLoginTime: u.LastLoginTime,
				CreationTime:  u.CreationTime,
			}

			// 2. Authorized OAuth apps with risk tiers (non-fatal on error).
			if tdata, terr := c.Get(ctx, wsDirectoryBase+"/users/"+user+"/tokens", nil); terr == nil {
				var env struct {
					Items []tokenRow `json:"items"`
				}
				if json.Unmarshal(tdata, &env) == nil {
					for _, t := range env.Items {
						tier, fullDrive := scopeRiskTier(t.Scopes)
						view.Apps = append(view.Apps, user360App{
							DisplayText: t.DisplayText,
							ClientID:    t.ClientID,
							RiskTier:    tier,
							FullDrive:   fullDrive,
						})
					}
					sort.Slice(view.Apps, func(i, j int) bool {
						return riskRank(view.Apps[i].RiskTier) > riskRank(view.Apps[j].RiskTier)
					})
				}
			}

			// 3. Group memberships (non-fatal on error).
			if gdata, gerr := c.Get(ctx, wsDirectoryBase+"/groups", map[string]string{
				"userKey": user,
				"fields":  "groups(email,name)",
			}); gerr == nil {
				var env struct {
					Groups []user360Group `json:"groups"`
				}
				if json.Unmarshal(gdata, &env) == nil {
					view.Groups = append(view.Groups, env.Groups...)
				}
			}

			return emitAudit(cmd, flags, view)
		},
	}
	return cmd
}
