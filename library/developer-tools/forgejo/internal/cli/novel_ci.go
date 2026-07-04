// Copyright 2026 jrimmer. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func newNovelCIStatusCmd(flags *rootFlags) *cobra.Command {
	var owner, repo, branch string
	var watch bool
	var watchInterval time.Duration
	var allRepos bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Pass/fail status of CI runs per repo",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeNoop(flags, "dry-run", "dry-run: would fetch CI status")
			}

			var targets [][2]string // [owner, repo] pairs
			if allRepos {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				repoData, err := c.Get(cmd.Context(), "/repos/search", map[string]string{"limit": "50"})
				if err == nil {
					var repoList []map[string]any
					if json.Unmarshal(repoData, &repoList) == nil {
						for _, r := range repoList {
							fn := fmt.Sprintf("%v", r["full_name"])
							if parts := strings.SplitN(fn, "/", 2); len(parts) == 2 {
								targets = append(targets, [2]string{parts[0], parts[1]})
							}
						}
					}
				}
				if len(targets) == 0 {
					if flags.asJSON {
						fmt.Fprintln(cmd.OutOrStdout(), `{"status":"empty","reason":"no repos found"}`)
						return nil
					}
					fmt.Fprintln(cmd.OutOrStdout(), "No repos found.")
					return nil
				}
			} else if owner == "" || repo == "" {
				return fmt.Errorf("--owner and --repo are required (or use --all-repos); e.g. fj ci status --owner acme --repo myrepo")
			} else {
				targets = [][2]string{{owner, repo}}
			}

			for {
				for _, t := range targets {
					if err := doCIStatus(cmd, flags, t[0], t[1], branch); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s/%s: %v\n", t[0], t[1], err)
					}
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

	cmd.Flags().StringVar(&owner, "owner", "", "Repository owner (required when --all-repos is not set)")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository name (required when --all-repos is not set)")
	cmd.Flags().StringVar(&branch, "branch", "main", "Branch to check CI status for")
	cmd.Flags().BoolVar(&watch, "watch", false, "Poll every --watch-interval")
	cmd.Flags().DurationVar(&watchInterval, "watch-interval", 10*time.Second, "Polling interval for --watch")
	cmd.Flags().BoolVar(&allRepos, "all-repos", false, "Check CI status across all accessible repos")

	return cmd
}

type ciStatusRow struct {
	Context     string `json:"context"`
	State       string `json:"state"`
	Description string `json:"description"`
	Created     string `json:"created"`
}

func doCIStatus(cmd *cobra.Command, flags *rootFlags, owner, repo, branch string) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	ctx := cmd.Context()

	// Get latest commit on branch
	commitData, err := c.Get(ctx, fmt.Sprintf("/repos/%s/%s/commits", owner, repo), map[string]string{
		"sha":   branch,
		"limit": "1",
	})
	if err != nil {
		return classifyAPIError(fmt.Errorf("getting commits: %w", err), flags)
	}

	var commits []map[string]any
	if err := json.Unmarshal(commitData, &commits); err != nil || len(commits) == 0 {
		return fmt.Errorf("no commits found on branch %q", branch)
	}
	sha := fmt.Sprintf("%v", commits[0]["sha"])
	if sha == "" || sha == "<nil>" {
		return fmt.Errorf("could not determine commit SHA for branch %q", branch)
	}

	// Get commit statuses
	statusData, err := c.Get(ctx, fmt.Sprintf("/repos/%s/%s/commits/%s/statuses", owner, repo, sha), map[string]string{
		"limit": "50",
	})
	if err != nil {
		return classifyAPIError(fmt.Errorf("getting statuses: %w", err), flags)
	}

	var statuses []map[string]any
	if json.Unmarshal(statusData, &statuses) != nil {
		statuses = nil
	}

	var rows []ciStatusRow
	for _, s := range statuses {
		context := fmt.Sprintf("%v", s["context"])
		state := fmt.Sprintf("%v", s["state"])
		description := truncate(fmt.Sprintf("%v", s["description"]), 50)
		created := ""
		if cr, ok := s["created_at"].(string); ok && len(cr) >= 10 {
			created = cr[:10]
		}
		rows = append(rows, ciStatusRow{
			Context:     context,
			State:       state,
			Description: description,
			Created:     created,
		})
	}

	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "CI status for %s/%s @ %s (%.8s)\n\n", owner, repo, branch, sha)

	if len(rows) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No CI statuses found.")
		return nil
	}

	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "CONTEXT\tSTATE\tDESCRIPTION\tCREATED")
	for _, r := range rows {
		state := r.State
		if colorEnabled() {
			switch strings.ToLower(r.State) {
			case "success":
				state = green(r.State)
			case "failure", "error":
				state = red(r.State)
			case "pending":
				state = yellow(r.State)
			}
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.Context, state, r.Description, r.Created)
	}
	return tw.Flush()
}
