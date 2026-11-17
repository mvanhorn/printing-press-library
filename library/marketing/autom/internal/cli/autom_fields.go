// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.
// Root-level Autom hooks: the global `fields` parameter (server-side
// projection, filtered out of each generated command by the generator and
// restored here as one root flag) and the current-command marker used by the
// credit ledger.

package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/client"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if root.PersistentFlags().Lookup("fields") == nil {
			root.PersistentFlags().String("fields", "", "Ask Autom to return only these response fields (comma-separated, dot notation for nested fields, for example organic_results.link,organic_results.position). Server-side and smaller than --select. Turns the response cache off for the call. Ignored by commands that read full results: rank, serp, engines/geo compare, ground, batch run, news watch, credits and jobs get")
		}
		fieldsValue := func() string {
			v, err := root.PersistentFlags().GetString("fields")
			if err != nil {
				return ""
			}
			return strings.TrimSpace(v)
		}
		client.ExtraParams = func(path string) map[string]string {
			if v := fieldsValue(); v != "" {
				return map[string]string{"fields": v}
			}
			return nil
		}
		prev := root.PersistentPreRunE
		root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			currentCommandPath = strings.TrimSpace(strings.TrimPrefix(cmd.CommandPath(), root.Name()))
			if prev != nil {
				if err := prev(cmd, args); err != nil {
					return err
				}
			}
			// A projected body must never be cached under the key of a full
			// call (the cache key is computed before extras are merged).
			if fieldsValue() != "" {
				flags.noCache = true
			}
			return nil
		}
	})
}

// fullResponse returns params with the global --fields projection disabled.
// Hand-written commands that parse the response need every field. It works
// because explicit params win over extras and the client drops empty GET
// parameters, so `fields=""` removes the projection from the request.
func fullResponse(params map[string]string) map[string]string {
	out := make(map[string]string, len(params)+1)
	for k, v := range params {
		out[k] = v
	}
	out["fields"] = ""
	return out
}
