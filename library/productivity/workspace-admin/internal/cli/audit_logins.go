// Copyright 2026 RyanGravetteIDLA and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature: audit logins.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/workspace-admin/internal/cliutil"
)

type dormantUser struct {
	Email         string `json:"email"`
	LastLoginTime string `json:"last_login_time"`
}

type loginsView struct {
	Logins       []activityEvent `json:"logins"`
	DormantUsers []dormantUser   `json:"dormant_users,omitempty"`
	Note         string          `json:"note,omitempty"`
}

func newNovelAuditLoginsCmd(flags *rootFlags) *cobra.Command {
	var flagFailures bool
	var flagGeo bool
	var flagSince string
	var flagUser string
	var flagDormant bool
	var flagDormantDays int
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "logins",
		Short: "Surface failed-login bursts, login locations, and (with --dormant) dormant accounts.",
		Long: "Query login activity for failed-login bursts and sign-in locations, and optionally cross-reference\n" +
			"users to find dormant accounts (no login in N days).\n\n" +
			"For a single user's full forensic action timeline use 'audit reconstruct' instead.",
		Example:     "  workspace-admin-pp-cli audit logins --failures --since 7d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would query login activity and optional dormant accounts")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			userKey := "all"
			if flagUser != "" {
				userKey = flagUser
			}
			params := map[string]string{"maxResults": "1000"}
			if flagSince != "" {
				dur, derr := cliutil.ParseDurationLoose(flagSince)
				if derr != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, derr))
				}
				params["startTime"] = time.Now().Add(-dur).UTC().Format(time.RFC3339)
			}
			data, err := c.Get(ctx, wsReportsBase+"/activity/users/"+userKey+"/applications/login", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			events, err := parseReportsActivities(data)
			if err != nil {
				return fmt.Errorf("parsing login activity: %w", err)
			}

			filtered := make([]activityEvent, 0, len(events))
			for _, e := range events {
				if flagFailures && !isLoginFailure(e.Name) {
					continue
				}
				if flagGeo && e.IP == "" {
					continue
				}
				filtered = append(filtered, e)
				if flagLimit > 0 && len(filtered) >= flagLimit {
					break
				}
			}
			filtered = mergeTimeline(filtered)

			view := loginsView{Logins: filtered}

			if flagDormant {
				cutoff := time.Now().AddDate(0, 0, -flagDormantDays)
				pageToken := ""
				maxUsers := 2000
				if cliutil.IsDogfoodEnv() {
					maxUsers = 100
				}
				scanned := 0
				for scanned < maxUsers {
					up := map[string]string{
						"customer":   "my_customer",
						"maxResults": "200",
						"fields":     "nextPageToken,users(primaryEmail,lastLoginTime)",
					}
					if pageToken != "" {
						up["pageToken"] = pageToken
					}
					ud, uerr := c.Get(ctx, wsDirectoryBase+"/users", up)
					if uerr != nil {
						return classifyAPIError(uerr, flags)
					}
					var env struct {
						NextPageToken string `json:"nextPageToken"`
						Users         []struct {
							PrimaryEmail  string `json:"primaryEmail"`
							LastLoginTime string `json:"lastLoginTime"`
						} `json:"users"`
					}
					if json.Unmarshal(ud, &env) != nil {
						break
					}
					for _, u := range env.Users {
						scanned++
						// lastLoginTime of "1970-01-01T00:00:00.000Z" means never logged in.
						// The Directory API returns millisecond precision, so parse with
						// RFC3339Nano; plain RFC3339 rejects the fractional seconds and would
						// mark every user dormant.
						t, perr := time.Parse(time.RFC3339Nano, u.LastLoginTime)
						if perr != nil || t.Before(cutoff) {
							view.DormantUsers = append(view.DormantUsers, dormantUser{Email: u.PrimaryEmail, LastLoginTime: u.LastLoginTime})
						}
					}
					if env.NextPageToken == "" || len(env.Users) == 0 {
						break
					}
					pageToken = env.NextPageToken
				}
			}

			if len(filtered) == 0 && len(view.DormantUsers) == 0 {
				view.Note = "no matching login events; widen --since or drop --failures"
			}
			return emitAudit(cmd, flags, view)
		},
	}
	cmd.Flags().BoolVar(&flagFailures, "failures", false, "Only show failed/blocked/suspicious sign-in events")
	cmd.Flags().BoolVar(&flagGeo, "geo", false, "Only show events that carry a source IP/location")
	cmd.Flags().StringVar(&flagSince, "since", "", "Look back this far (e.g. 24h, 7d, 4w)")
	cmd.Flags().StringVar(&flagUser, "user", "", "Limit to a single user's login events")
	cmd.Flags().BoolVar(&flagDormant, "dormant", false, "Also list dormant accounts (no login in --dormant-days)")
	cmd.Flags().IntVar(&flagDormantDays, "dormant-days", 90, "Days of inactivity that mark an account dormant")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum login events to return (0 = no limit)")
	return cmd
}
