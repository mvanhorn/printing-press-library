// Hand-authored novel feature: typed exit codes per terminal run status.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var typedExitForStatus = map[string]int{
	"FAILED":         20,
	"CRASHED":        21,
	"SYSTEM_FAILURE": 22,
	"CANCELED":       23,
}

func newRunsGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <runId>",
		Short: "Retrieve a run with typed exit codes per terminal status (0/20/21/22/23/3/4)",
		Long: `Retrieve a run by ID. Unlike 'runs retrieve-v1', this command sets the process
exit code from the run's terminal status — agents and shell loops can branch
on $? without parsing JSON.

Exit codes:
  0   COMPLETED (or in-flight)
  3   run not found
  4   authentication error
  20  FAILED
  21  CRASHED
  22  SYSTEM_FAILURE
  23  CANCELED`,
		Example: "  trigger-dev-pp-cli runs get run_abc123\n  trigger-dev-pp-cli runs get run_abc123 --json --select 'status,error.message'",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,3,4,20,21,22,23",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := replacePathParam("/api/v3/runs/{runId}", "runId", args[0])
			data, err := c.Get(path, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var run struct {
				Status string `json:"status"`
			}
			if err := json.Unmarshal(extractResponseData(data), &run); err != nil {
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}
			if err := printOutputWithFlags(cmd.OutOrStdout(), data, flags); err != nil {
				return err
			}
			status := strings.ToUpper(strings.TrimSpace(run.Status))
			if code, ok := typedExitForStatus[status]; ok {
				return &cliError{code: code, err: fmt.Errorf("run terminal status: %s", status)}
			}
			return nil
		},
	}
	return cmd
}
