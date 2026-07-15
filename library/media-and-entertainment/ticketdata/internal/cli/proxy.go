// pp:data-source live
// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newProxyCmd(flags *rootFlags) *cobra.Command {
	var rawParams []string

	cmd := &cobra.Command{
		Use:         "proxy <method> <path>",
		Short:       "Issue a raw read-only GET through the TicketData API client",
		Example:     "  ticketdata-pp-cli proxy GET /search --param venue_slug=lumen-field",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && commandNFlag(cmd) == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				if dryRunOK(flags) {
					fmt.Fprintln(cmd.OutOrStdout(), "would GET /")
					return nil
				}
				return usageErr(fmt.Errorf("method and path are required"))
			}
			method := strings.ToUpper(strings.TrimSpace(args[0]))
			if method != "GET" {
				return usageErr(fmt.Errorf("proxy supports GET only"))
			}
			path := normalizeProxyPath(args[1])
			params, err := parseProxyParams(rawParams)
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would GET %s%s\n", c.RequestBaseURL(), formatGETPreview(path, params))
				return nil
			}
			raw, err := c.Get(cmd.Context(), path, params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringArrayVar(&rawParams, "param", nil, "Query parameter as k=v; repeatable")
	return cmd
}

func normalizeProxyPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}

func parseProxyParams(rawParams []string) (map[string]string, error) {
	params := make(map[string]string, len(rawParams))
	for _, raw := range rawParams {
		key, value, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, fmt.Errorf("--param must be k=v")
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("--param key cannot be empty")
		}
		params[key] = strings.TrimSpace(value)
	}
	return params, nil
}
