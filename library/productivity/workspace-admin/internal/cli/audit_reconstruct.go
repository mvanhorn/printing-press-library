// Copyright 2026 RyanGravetteIDLA and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature: audit reconstruct.
// pp:data-source live

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/workspace-admin/internal/cliutil"
)

type reconstructView struct {
	User     string          `json:"user"`
	Timeline []activityEvent `json:"timeline"`
	Note     string          `json:"note,omitempty"`
}

// reconstructApps are the Reports applications interleaved into the timeline.
var reconstructApps = []string{"login", "admin", "drive", "token"}

func newNovelAuditReconstructCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "reconstruct <user>",
		Short: "Merge one user's login, admin, Drive, and token activity into a single ordered timeline.",
		Long: "Merge one user's login, admin, Drive, and token activity from the Reports API into a single\n" +
			"chronological timeline for post-compromise review.\n\n" +
			"For domain-wide login anomalies (failures, locations, dormant accounts) use 'audit logins' instead.",
		Example:     "  workspace-admin-pp-cli audit reconstruct user@yourdomain.com --since 7d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would interleave login/admin/drive/token activity into one timeline")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a user email is required"))
			}
			user := args[0]
			if !validUserKey(user) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("%q is not a valid user email or directory ID", user))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
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

			var all []activityEvent
			for _, app := range reconstructApps {
				data, gerr := c.Get(ctx, wsReportsBase+"/activity/users/"+user+"/applications/"+app, params)
				if gerr != nil {
					// Some applications may be inaccessible for a given user/scope; skip rather than abort.
					continue
				}
				evs, perr := parseReportsActivities(data)
				if perr != nil {
					continue
				}
				all = append(all, evs...)
			}
			all = mergeTimeline(all)
			if flagLimit > 0 && len(all) > flagLimit {
				all = all[len(all)-flagLimit:] // keep the most recent N
			}
			if all == nil {
				all = []activityEvent{}
			}
			view := reconstructView{User: user, Timeline: all}
			if len(all) == 0 {
				view.Note = "no activity found for this user in the window; widen --since or verify Reports scope access"
			}
			return emitAudit(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Look back this far (e.g. 24h, 7d, 4w)")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Keep only the most recent N events (0 = all)")
	return cmd
}
