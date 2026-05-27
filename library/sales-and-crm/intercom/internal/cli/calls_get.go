// PATCH(intercom-calls-v2-14): see calls.go.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newCallsGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "get <id>",
		Short:       "Fetch one call by id",
		Example:     "  intercom-pp-cli calls get 35578587",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			callID := strings.TrimSpace(args[0])
			if callID == "" {
				return usageErr(fmt.Errorf("call id is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GetWithHeaders(cmd.Context(), "/calls/"+callID, nil, callsHeaderOverrides())
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	return cmd
}
