// Copyright 2026 alon-auto and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written raw OData escape hatch: anything the API can express that no
// typed command covers, with auth and throttling still handled.

package cli

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

func newQueryRawCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query <path-and-query>",
		Short: "Raw OData escape hatch — GET any path with any query options",
		Long: strings.Trim(`
Sends a GET to the service root plus your path. Query options are parsed and
re-encoded properly, so you can paste unencoded OData ($filter with spaces and
quotes) straight from the docs. Auth, rate limiting, and typed exit codes are
still applied.`, "\n"),
		Example: strings.Trim(`
  priority-pp-cli query "ORDERS?$filter=CUSTNAME eq '1011'&$expand=ORDERITEMS_SUBFORM"
  priority-pp-cli query "FAMILY_LOG('001')/FAMILY_LOGPART_SUBFORM"
  priority-pp-cli query "GetPriorityVersion()"`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "path-and-query=GetODataVersion()"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would GET the raw OData path")
				return nil
			}
			if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("path is required, e.g. \"ORDERS?$top=5\""))
			}
			raw := strings.TrimPrefix(strings.TrimSpace(args[0]), "/")
			path := raw
			params := map[string]string{}
			if idx := strings.Index(raw, "?"); idx >= 0 {
				path = raw[:idx]
				vals, err := url.ParseQuery(raw[idx+1:])
				if err != nil {
					return usageErr(fmt.Errorf("parsing query options: %w", err))
				}
				for k, v := range vals {
					if len(v) > 0 {
						params[k] = v[0]
					}
				}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(ctx, "/"+path, params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.selectFields != "" {
				data = filterFields(data, flags.selectFields)
			} else if flags.compact {
				data = compactFields(data)
			}
			return printOutput(cmd.OutOrStdout(), data, true)
		},
	}
	return cmd
}
