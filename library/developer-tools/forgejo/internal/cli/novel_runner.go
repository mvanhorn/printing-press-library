// Copyright 2026 jrimmer. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/forgejo/internal/client"
	"github.com/spf13/cobra"
)

func newNovelRunnerSweepCmd(flags *rootFlags) *cobra.Command {
	var org string
	var allOrgs bool
	var watch bool
	var watchInterval time.Duration
	var offlineOnly bool

	cmd := &cobra.Command{
		Use:   "sweep",
		Short: "See runner status across all orgs and repos in one pass",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeNoop(flags, "dry-run", "dry-run: would sweep runner status across orgs")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			for {
				if err := doRunnerSweep(cmd, c, flags, org, allOrgs, offlineOnly); err != nil {
					return err
				}
				if !watch {
					break
				}
				time.Sleep(watchInterval)
				fmt.Fprint(cmd.OutOrStdout(), "\033[2J\033[H") // clear screen
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Single org to sweep")
	cmd.Flags().BoolVar(&allOrgs, "all-orgs", false, "Sweep all orgs")
	cmd.Flags().BoolVar(&watch, "watch", false, "Poll every --watch-interval")
	cmd.Flags().DurationVar(&watchInterval, "watch-interval", 10*time.Second, "Polling interval for --watch")
	cmd.Flags().BoolVar(&offlineOnly, "offline-only", false, "Only show offline runners")

	return cmd
}

type runnerRow struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Labels     string `json:"labels"`
	Scope      string `json:"scope"`
	LastActive string `json:"last_active"`
}

func doRunnerSweep(cmd *cobra.Command, c *client.Client, flags *rootFlags, orgFilter string, allOrgs bool, offlineOnly bool) error {
	ctx := cmd.Context()

	var rows []runnerRow

	// Fetch user-scoped runners
	userData, err := c.Get(ctx, "/user/actions/runners", map[string]string{"limit": "50"})
	if err == nil {
		rows = append(rows, parseRunnerRows(userData, "user")...)
	}

	if orgFilter != "" {
		orgData, err := c.Get(ctx, fmt.Sprintf("/orgs/%s/actions/runners", orgFilter), map[string]string{"limit": "50"})
		if err == nil {
			rows = append(rows, parseRunnerRows(orgData, "org:"+orgFilter)...)
		}
	} else if allOrgs {
		orgsData, err := c.Get(ctx, "/user/orgs", map[string]string{"limit": "50"})
		if err == nil {
			var orgs []map[string]any
			if json.Unmarshal(orgsData, &orgs) == nil {
				for _, o := range orgs {
					orgName := fmt.Sprintf("%v", o["username"])
					if orgName == "" || orgName == "<nil>" {
						orgName = fmt.Sprintf("%v", o["name"])
					}
					orgData, orgErr := c.Get(ctx, fmt.Sprintf("/orgs/%s/actions/runners", orgName), map[string]string{"limit": "50"})
					if orgErr == nil {
						rows = append(rows, parseRunnerRows(orgData, "org:"+orgName)...)
					}
				}
			}
		}
	}

	if offlineOnly {
		var filtered []runnerRow
		for _, r := range rows {
			if r.Status == "offline" {
				filtered = append(filtered, r)
			}
		}
		rows = filtered
	}

	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
	}

	if len(rows) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No runners found.")
		return nil
	}

	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tSTATUS\tLABELS\tSCOPE\tLAST_ACTIVE")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Name, r.Status, r.Labels, r.Scope, r.LastActive)
	}
	return tw.Flush()
}

func parseRunnerRows(data json.RawMessage, scope string) []runnerRow {
	var runners []map[string]any
	if json.Unmarshal(data, &runners) != nil {
		var env map[string]json.RawMessage
		if json.Unmarshal(data, &env) == nil {
			for _, key := range []string{"runners", "data", "items"} {
				if raw, ok := env[key]; ok {
					if json.Unmarshal(raw, &runners) == nil {
						break
					}
				}
			}
		}
	}

	var rows []runnerRow
	for _, r := range runners {
		name := fmt.Sprintf("%v", r["name"])
		status := fmt.Sprintf("%v", r["status"])

		var labelParts []string
		if labArr, ok := r["labels"].([]any); ok {
			for _, l := range labArr {
				if lm, ok := l.(map[string]any); ok {
					labelParts = append(labelParts, fmt.Sprintf("%v", lm["name"]))
				} else {
					labelParts = append(labelParts, fmt.Sprintf("%v", l))
				}
			}
		}
		labels := ""
		for i, p := range labelParts {
			if i > 0 {
				labels += ","
			}
			labels += p
		}

		lastActive := ""
		if la, ok := r["last_active"].(string); ok && len(la) >= 10 {
			lastActive = la[:10]
		}

		rows = append(rows, runnerRow{
			Name:       name,
			Status:     status,
			Labels:     labels,
			Scope:      scope,
			LastActive: lastActive,
		})
	}
	return rows
}
