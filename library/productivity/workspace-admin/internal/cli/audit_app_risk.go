// Copyright 2026 RyanGravetteIDLA and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature: audit app-risk.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/workspace-admin/internal/cliutil"
)

type appRiskRow struct {
	ClientID    string   `json:"client_id"`
	DisplayText string   `json:"display_text"`
	Tier        string   `json:"risk_tier"`
	FullDrive   bool     `json:"full_drive_access"`
	UserCount   int      `json:"user_count"`
	Users       []string `json:"users"`
	Scopes      []string `json:"scopes"`
}

type appRiskView struct {
	Apps         []appRiskRow `json:"apps"`
	ScannedUsers int          `json:"scanned_users"`
	Note         string       `json:"note,omitempty"`
}

type tokenRow struct {
	ClientID    string   `json:"clientId"`
	DisplayText string   `json:"displayText"`
	Scopes      []string `json:"scopes"`
	UserKey     string   `json:"userKey"`
}

func newNovelAuditAppRiskCmd(flags *rootFlags) *cobra.Command {
	var flagMinRisk string
	var flagUser string
	var flagMaxUsers int

	cmd := &cobra.Command{
		Use:   "app-risk",
		Short: "Rank third-party OAuth apps by a curated scope-to-risk tier with users-per-app counts.",
		Long: "Roll up third-party OAuth apps authorized in the domain by a curated scope-to-risk tier\n" +
			"(Low/Moderate/High), flag apps with full Drive access, and count users-per-app.\n\n" +
			"OAuth grants are per-user in the Directory API, so without --user this command scans users\n" +
			"(up to --max-users) and aggregates their authorized apps.",
		Example:     "  workspace-admin-pp-cli audit app-risk --min-risk high --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would aggregate OAuth app scope-risk across users")
				return nil
			}
			if flagMinRisk != "" {
				switch strings.ToLower(flagMinRisk) {
				case "low", "moderate", "medium", "high":
				default:
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--min-risk must be one of: low, moderate, high"))
				}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Resolve the set of users whose tokens we will inspect.
			var users []string
			if flagUser != "" {
				users = []string{flagUser}
			} else {
				maxUsers := flagMaxUsers
				if cliutil.IsDogfoodEnv() && maxUsers > 50 {
					maxUsers = 50
				}
				pageToken := ""
				for len(users) < maxUsers {
					params := map[string]string{
						"customer":   "my_customer",
						"maxResults": "100",
						"fields":     "nextPageToken,users(primaryEmail)",
					}
					if pageToken != "" {
						params["pageToken"] = pageToken
					}
					data, err := c.Get(ctx, wsDirectoryBase+"/users", params)
					if err != nil {
						return classifyAPIError(err, flags)
					}
					var env struct {
						NextPageToken string `json:"nextPageToken"`
						Users         []struct {
							PrimaryEmail string `json:"primaryEmail"`
						} `json:"users"`
					}
					if err := json.Unmarshal(data, &env); err != nil {
						return fmt.Errorf("decoding users response: %w", err)
					}
					for _, u := range env.Users {
						if u.PrimaryEmail != "" {
							users = append(users, u.PrimaryEmail)
						}
					}
					if env.NextPageToken == "" || len(env.Users) == 0 {
						break
					}
					pageToken = env.NextPageToken
				}
				if len(users) > maxUsers {
					users = users[:maxUsers]
				}
			}

			type acc struct {
				display string
				scopes  map[string]bool
				users   map[string]bool
			}
			apps := map[string]*acc{}
			scanned := 0
			for _, u := range users {
				scanned++
				data, err := c.Get(ctx, wsDirectoryBase+"/users/"+u+"/tokens", nil)
				if err != nil {
					// A single user's token read failing should not abort the whole audit.
					continue
				}
				var env struct {
					Items []tokenRow `json:"items"`
				}
				if json.Unmarshal(data, &env) != nil {
					continue
				}
				for _, t := range env.Items {
					a := apps[t.ClientID]
					if a == nil {
						a = &acc{display: t.DisplayText, scopes: map[string]bool{}, users: map[string]bool{}}
						apps[t.ClientID] = a
					}
					for _, s := range t.Scopes {
						a.scopes[s] = true
					}
					a.users[u] = true
				}
			}

			minRank := riskRank(flagMinRisk)
			rows := make([]appRiskRow, 0, len(apps))
			for clientID, a := range apps {
				scopes := make([]string, 0, len(a.scopes))
				for s := range a.scopes {
					scopes = append(scopes, s)
				}
				sort.Strings(scopes)
				tier, fullDrive := scopeRiskTier(scopes)
				if riskRank(tier) < minRank {
					continue
				}
				usersList := make([]string, 0, len(a.users))
				for u := range a.users {
					usersList = append(usersList, u)
				}
				sort.Strings(usersList)
				rows = append(rows, appRiskRow{
					ClientID:    clientID,
					DisplayText: a.display,
					Tier:        tier,
					FullDrive:   fullDrive,
					UserCount:   len(usersList),
					Users:       usersList,
					Scopes:      scopes,
				})
			}
			sort.Slice(rows, func(i, j int) bool {
				ri, rj := riskRank(rows[i].Tier), riskRank(rows[j].Tier)
				if ri != rj {
					return ri > rj
				}
				if rows[i].UserCount != rows[j].UserCount {
					return rows[i].UserCount > rows[j].UserCount
				}
				return rows[i].DisplayText < rows[j].DisplayText
			})

			view := appRiskView{Apps: rows, ScannedUsers: scanned}
			if len(rows) == 0 {
				view.Note = "no OAuth apps matched; widen scope with --max-users or relax --min-risk"
			}
			return emitAudit(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagMinRisk, "min-risk", "", "Only show apps at or above this tier: low, moderate, high")
	cmd.Flags().StringVar(&flagUser, "user", "", "Audit a single user's authorized apps instead of scanning the domain")
	cmd.Flags().IntVar(&flagMaxUsers, "max-users", 200, "Maximum users to scan when aggregating domain-wide")
	return cmd
}
