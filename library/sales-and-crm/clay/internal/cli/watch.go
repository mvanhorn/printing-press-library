// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/clay/internal/cliutil"
	"github.com/spf13/cobra"
)

type watchResult struct {
	TableID  string         `json:"tableId"`
	Settled  bool           `json:"settled"`
	Polls    int            `json:"polls"`
	Statuses map[string]int `json:"statuses"`
	Pending  int            `json:"pending"`
}

// pendingStatuses mean work is still in flight.
var pendingStatuses = map[string]bool{"QUEUED": true, "RUNNING": true, "PENDING": true, "RETRY": true, "RATE_LIMITED": true}

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var flagWorkspace string
	var flagInterval time.Duration
	var flagMaxPolls int

	cmd := &cobra.Command{
		Use:   "watch <tableId>",
		Short: "Block until every column's enrichment runs settle, reporting per-column status counts.",
		Long: "Use this command in a script after triggering enrichment, instead of polling the Clay UI.\n" +
			"Do NOT use it to start a run; it only observes.",
		Example: "  clay-pp-cli watch t_abc123 --workspace 1234567 --interval 10s --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "<tableId>=t_example;--workspace=1;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "watch")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<tableId> is required"))
			}
			ws, err := resolveWorkspace(flagWorkspace)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			maxPolls := flagMaxPolls
			interval := flagInterval
			// Live dogfood runs under a flat per-command timeout; curtail work.
			if cliutil.IsDogfoodEnv() {
				maxPolls = 1
				interval = time.Second
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			res := watchResult{TableID: args[0], Statuses: map[string]int{}}
			for poll := 1; poll <= maxPolls; poll++ {
				rs, rErr := fetchRunStatus(ctx, c, ws, args[0])
				if rErr != nil {
					return classifyAPIError(cmd.OutOrStdout(), rErr, flags)
				}
				res.Polls = poll
				res.Statuses = map[string]int{}
				res.Pending = 0
				for _, counts := range rs.StatusCountsByField {
					for _, sc := range counts {
						res.Statuses[sc.Status] += sc.Count
						if pendingStatuses[sc.Status] {
							res.Pending += sc.Count
						}
					}
				}
				if res.Pending == 0 {
					res.Settled = true
					break
				}
				if poll == maxPolls {
					break
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(interval):
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if res.Settled {
				fmt.Fprintf(cmd.OutOrStdout(), "settled after %d poll(s): %v\n", res.Polls, res.Statuses)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "still pending (%d) after %d poll(s): %v\n", res.Pending, res.Polls, res.Statuses)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagWorkspace, "workspace", "", "Clay workspace id (or set CLAY_WORKSPACE_ID)")
	cmd.Flags().DurationVar(&flagInterval, "interval", 10*time.Second, "Delay between polls")
	cmd.Flags().IntVar(&flagMaxPolls, "max-polls", 30, "Maximum polls before giving up")
	return cmd
}
