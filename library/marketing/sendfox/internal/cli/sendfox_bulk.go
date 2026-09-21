package cli

import (
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/cliutil"
	"github.com/spf13/cobra"
	"strconv"
	"time"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, f *rootFlags) {
		p, _, _ := root.Find([]string{"contacts"})
		p.AddCommand(newSendfoxBulkWait(f))
	})
}
func newSendfoxBulkWait(f *rootFlags) *cobra.Command {
	var id, interval string
	var maxPolls int
	cmd := &cobra.Command{Use: "bulk-wait", Short: "Observe an asynchronous bulk action with a bounded request budget", Example: "  sendfox-pp-cli contacts bulk-wait --id <bulk-action-id> --max-polls 5 --interval 2s --json", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--id=1;--max-polls=1;--dry-run"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "contacts bulk-wait")
		}
		n, e := strconv.Atoi(id)
		if e != nil || n < 1 {
			return usageErr(fmt.Errorf("--id must be a positive bulk action ID"))
		}
		if maxPolls < 1 || maxPolls > 60 {
			return usageErr(fmt.Errorf("--max-polls must be 1..60; all processes share the account's 60/min limit"))
		}
		delay, e := cliutil.ParseDurationLoose(interval)
		if e != nil || delay < time.Second {
			return usageErr(fmt.Errorf("--interval must be at least 1s"))
		}
		if cliutil.IsDogfoodEnv() {
			maxPolls = 1
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		c, e := f.newClient()
		if e != nil {
			return e
		}
		var result map[string]any
		for i := 1; i <= maxPolls; i++ {
			raw, e := c.Get(ctx, "/contacts/bulk-actions/"+id, nil)
			if e != nil {
				return e
			}
			if e = json.Unmarshal(raw, &result); e != nil {
				return e
			}
			if inner, ok := result["data"].(map[string]any); ok {
				result = inner
			}
			state, _ := result["status"].(string)
			view := map[string]any{"action": result, "requests_used": i, "max_requests": maxPolls, "finished": state == "completed" || state == "failed"}
			if state == "completed" || state == "failed" || i == maxPolls {
				if state != "completed" && state != "failed" {
					view["note"] = "Budget exhausted; re-run with the same ID to continue observation, never resubmit the mutation."
				}
				if err := f.printJSON(cmd, view); err != nil {
					return err
				}
				if state == "failed" {
					return fmt.Errorf("bulk action failed; inspect returned server error and processed_count before any new action")
				}
				return nil
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		return nil
	}}
	cmd.Flags().StringVar(&id, "id", "", "Existing asynchronous bulk action ID to observe")
	cmd.Flags().IntVar(&maxPolls, "max-polls", 5, "Maximum API requests before returning a resumable progress report")
	cmd.Flags().StringVar(&interval, "interval", "2s", "Delay between requests; timeout bounds the whole polling operation")
	return cmd
}
