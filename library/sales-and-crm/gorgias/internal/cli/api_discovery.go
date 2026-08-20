// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// promotedLeafResources lists the root-level visible commands that map to a
// single Gorgias endpoint each. They're not hidden parent groups (which is
// the api browser's normal filter), so they need explicit inclusion or the
// browser's count drifts below the 108-endpoint claim. Reference: counted
// against codeOrchEndpoints in internal/mcp/code_orch.go.
var promotedLeafResources = map[string]struct{}{
	"messages":      {}, // messages.list
	"pickups":       {}, // pickups.delete
	"reporting":     {}, // reporting.stats
	"ticket-search": {}, // ticket-search.query
}

// promotedMethodName returns the synthetic method name for a promoted leaf
// command, derived from its `pp:endpoint` annotation (e.g. "messages.list"
// → "list"). Falls back to the cobra command name itself when the
// annotation is missing — the count is still 1, just less informative.
func promotedMethodName(cmd *cobra.Command) string {
	if cmd.Annotations != nil {
		if epID := cmd.Annotations["pp:endpoint"]; epID != "" {
			if idx := strings.Index(epID, "."); idx >= 0 {
				return epID[idx+1:]
			}
		}
	}
	return cmd.Name()
}

func newAPICmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "api [interface]",
		Short:       "Browse all API endpoints by interface name",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # List all available interfaces
  gorgias-pp-cli api

  # Show methods for a specific interface
  gorgias-pp-cli api <interface-name>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()

			if len(args) > 0 {
				target := strings.ToLower(args[0])
				_, isPromoted := promotedLeafResources[target]
				for _, child := range root.Commands() {
					if strings.ToLower(child.Name()) != target {
						continue
					}
					if !child.Hidden && !isPromoted {
						continue
					}
					methods := child.Commands()
					// Promoted single-endpoint resources have no sub-commands
					// in cobra (they ARE the endpoint). For the `api` browser
					// to surface them as countable methods — keeping the
					// per-resource method tally consistent with the headline
					// 108-endpoint claim — synthesize a single self-method.
					// The method name is derived from `pp:endpoint` (`x.y` →
					// "y") so a reader can map `messages list` etc. back to
					// the spec.
					if isPromoted && len(methods) == 0 {
						promotedMethod := promotedMethodName(child)
						if flags.asJSON {
							return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
								"interface": child.Name(),
								"short":     child.Short,
								"methods": []map[string]any{{
									"name":  promotedMethod,
									"short": child.Short,
								}},
							}, flags)
						}
						fmt.Fprintf(cmd.OutOrStdout(), "%s — %s\n\nMethods:\n", child.Name(), child.Short)
						fmt.Fprintf(cmd.OutOrStdout(), "  %-50s %s\n", child.Name()+" "+promotedMethod, child.Short)
						fmt.Fprintf(cmd.OutOrStdout(), "\nUse '%s-pp-cli %s --help' for details.\n", "gorgias", child.Name())
						return nil
					}
					// JSON envelope: {interface, short, methods: [{name, short}, ...]}.
					if flags.asJSON {
						methodList := make([]map[string]any, 0, len(methods))
						for _, method := range methods {
							methodList = append(methodList, map[string]any{
								"name":  method.Name(),
								"short": method.Short,
							})
						}
						return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
							"interface": child.Name(),
							"short":     child.Short,
							"methods":   methodList,
						}, flags)
					}
					if len(methods) == 0 {
						return child.Help()
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%s — %s\n\nMethods:\n", child.Name(), child.Short)
					for _, method := range methods {
						fmt.Fprintf(cmd.OutOrStdout(), "  %-50s %s\n", child.Name()+" "+method.Name(), method.Short)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "\nUse '%s-pp-cli %s <method> --help' for details.\n", "gorgias", child.Name())
					return nil
				}
				return fmt.Errorf("interface %q not found. Run '%s-pp-cli api' to list all interfaces", args[0], "gorgias")
			}

			// Pre-formatting human strings ahead of time would block the JSON
			// path from emitting clean field values; build the typed slice and
			// derive human format on print.
			type ifaceEntry struct {
				Name  string `json:"name"`
				Short string `json:"short"`
			}
			var ifaces []ifaceEntry
			for _, child := range root.Commands() {
				if child.Hidden {
					ifaces = append(ifaces, ifaceEntry{Name: child.Name(), Short: child.Short})
				}
			}
			// Promoted single-endpoint resources sit as visible leaf commands
			// at the root (messages / pickups / reporting / ticket-search) so
			// the `api` browser would otherwise hide them. Include them by
			// name — they're each one endpoint, so the methods view is just
			// the leaf command itself.
			for _, child := range root.Commands() {
				if child.Hidden {
					continue
				}
				if _, ok := promotedLeafResources[child.Name()]; ok {
					ifaces = append(ifaces, ifaceEntry{Name: child.Name(), Short: child.Short})
				}
			}
			sort.Slice(ifaces, func(i, j int) bool { return ifaces[i].Name < ifaces[j].Name })

			// JSON envelope: {interfaces: [...], note?: "..."}.
			if flags.asJSON {
				out := map[string]any{"interfaces": ifaces}
				if len(ifaces) == 0 {
					out["interfaces"] = []ifaceEntry{}
					out["note"] = "No hidden API interfaces found."
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			if len(ifaces) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No hidden API interfaces found.")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Available API interfaces (%d):\n\n", len(ifaces))
			for _, e := range ifaces {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-45s %s\n", e.Name, e.Short)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nUse '%s-pp-cli api <interface>' to see methods.\n", "gorgias")
			return nil
		},
	}

	return cmd
}
